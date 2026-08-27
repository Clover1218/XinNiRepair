// OrderExportService 工单导出 (5.14)。
//
// 支持企业对账单(enterprise)与业务员汇总(repairer)两种模式,
// 使用 excelize/v2 在内存中生成 xlsx 文件流。
package service

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/xuri/excelize/v2"
	"go.uber.org/zap"

	apperrors "xin-ni-repair/internal/errors"
	"xin-ni-repair/internal/model"
	"xin-ni-repair/internal/repository"
)

// 导出模式
const (
	ExportModeEnterprise = "enterprise" // 企业对账单
	ExportModeRepairer   = "repairer"   // 业务员汇总
)

// 可用导出字段定义: 字段标识 → 列名
var exportFieldColumns = map[string]string{
	"order_no":        "工单号",
	"time":            "日期",
	"content":         "内容",
	"quantity":        "数量",
	"unit_price":      "单价",
	"amount":          "金额",
	"remark":          "备注",
	"repair_result":   "维修结果",
	"repair_method":   "维修方式",
	"warranty_period": "保修期",
	"repair_duration": "维修时长",
	"reporter":        "报修人",
	"repairer":        "维修员",
	"room":            "位置",
	"contact":         "联系方式",
	"enterprise_name": "客户名称",
}

// 默认导出字段 (不传 fields 时)
var defaultExportFields = []string{"order_no", "time", "content", "quantity", "unit_price", "amount", "remark"}

// OrderExportService 工单导出业务逻辑
type OrderExportService struct {
	orders    *repository.OrderRepository
	timelines *repository.OrderTimelineRepository
	shopName  string
	logger    *zap.Logger
}

// NewOrderExportService 创建 OrderExportService
func NewOrderExportService(
	orders *repository.OrderRepository,
	timelines *repository.OrderTimelineRepository,
	shopName string,
	logger *zap.Logger,
) *OrderExportService {
	return &OrderExportService{
		orders:    orders,
		timelines: timelines,
		shopName:  shopName,
		logger:    logger,
	}
}

// ExportResult 导出结果 (文件流与文件名)
type ExportResult struct {
	Filename    string
	ContentType string
	Content     *bytes.Buffer
}

// ExportRequest 导出请求入参 (5.14)
type ExportRequest struct {
	Mode         string
	EnterpriseID string
	RepairerID   string
	DateFrom     time.Time
	DateTo       time.Time
	Fields       []string
	Status       string
}

// Export 导出工单 Excel (5.14)
func (s *OrderExportService) Export(ctx context.Context, req ExportRequest) (*ExportResult, error) {
	// 模式校验
	switch req.Mode {
	case ExportModeEnterprise:
		if req.EnterpriseID == "" {
			return nil, apperrors.ErrInvalidParam.WithMessage("mode=enterprise 时 enterprise_id 必填")
		}
	case ExportModeRepairer:
		if req.RepairerID == "" {
			return nil, apperrors.ErrInvalidParam.WithMessage("mode=repairer 时 repairer_id 必填")
		}
	default:
		return nil, apperrors.ErrInvalidParam.WithMessage("mode 取值: enterprise/repairer")
	}

	// 状态默认 completed (文档: 默认只导出已完成的工单)
	if req.Status == "" {
		req.Status = string(model.OrderCompleted)
	}

	// 字段校验
	fields, err := resolveExportFields(req.Fields)
	if err != nil {
		return nil, err
	}

	// 查询工单
	var orders []model.RepairOrder
	if req.Mode == ExportModeEnterprise {
		orders, err = s.orders.ListForExportByEnterprise(ctx, req.EnterpriseID, req.Status, req.DateFrom, req.DateTo)
	} else {
		orders, err = s.orders.ListForExportByRepairer(ctx, req.RepairerID, req.Status, req.DateFrom, req.DateTo)
	}
	if err != nil {
		s.logger.Error("export orders query failed", zap.Error(err))
		return nil, apperrors.ErrExportFailed.WithError(err)
	}
	if len(orders) == 0 {
		return nil, apperrors.ErrNoExportData
	}

	// 批量查维修员
	orderIDs := make([]string, 0, len(orders))
	for _, o := range orders {
		orderIDs = append(orderIDs, o.ID)
	}
	repairerNames, err := s.timelines.CompleteOperatorsByOrders(ctx, orderIDs)
	if err != nil {
		s.logger.Error("export repairer names query failed", zap.Error(err))
		return nil, apperrors.ErrExportFailed.WithError(err)
	}

	return s.buildExcel(req, orders, repairerNames, fields)
}

// resolveExportFields 校验并解析导出字段 (默认导出 defaultExportFields)
func resolveExportFields(fields []string) ([]string, error) {
	if len(fields) == 0 {
		return append([]string(nil), defaultExportFields...), nil
	}
	seen := make(map[string]bool, len(fields))
	resolved := make([]string, 0, len(fields))
	for _, f := range fields {
		f = strings.TrimSpace(f)
		if f == "" {
			continue
		}
		if _, ok := exportFieldColumns[f]; !ok {
			return nil, apperrors.ErrInvalidParam.WithMessage("fields 含未知字段: " + f)
		}
		if seen[f] {
			continue
		}
		seen[f] = true
		resolved = append(resolved, f)
	}
	if len(resolved) == 0 {
		return append([]string(nil), defaultExportFields...), nil
	}
	return resolved, nil
}

// buildExcel 构建 xlsx 文件
func (s *OrderExportService) buildExcel(req ExportRequest, orders []model.RepairOrder, repairerNames map[string]string, fields []string) (*ExportResult, error) {
	f := excelize.NewFile()
	defer f.Close()

	sheet := "Sheet1"
	f.SetSheetName("Sheet1", sheet)

	periodText := fmt.Sprintf("%s-%s", req.DateFrom.Format("2006年1月2日"), req.DateTo.Format("2006年1月2日"))
	now := time.Now()
	totalAmount := 0.0
	totalCount := 0

	// ── 表头区域 ──
	// 行1: 店铺名 + 单据名 + 期间
	title := "维修对账单"
	if req.Mode == ExportModeRepairer {
		title = "业务员维修汇总表"
	}
	f.SetCellValue(sheet, "A1", s.shopName+" "+title)
	_ = f.SetCellValue(sheet, "F1", periodText)

	// 行2: 客户名称 / 维修员
	var infoLabel, infoValue string
	if req.Mode == ExportModeEnterprise {
		for _, o := range orders {
			if o.Enterprise.Name != "" {
				infoLabel, infoValue = "客户名称", o.Enterprise.Name
				break
			}
		}
	} else {
		infoLabel, infoValue = "维修员", repairerNames[orders[0].ID]
		if infoValue == "" {
			for _, o := range orders {
				if v := repairerNames[o.ID]; v != "" {
					infoValue = v
					break
				}
			}
		}
	}
	if infoLabel != "" {
		f.SetCellValue(sheet, "A2", infoLabel+"："+infoValue)
	}
	_ = f.SetCellValue(sheet, "F2", "制表时间："+now.Format("2006-01-02 15:04"))

	// 列头 (行3): 固定"序号"列 + 用户字段
	headerRow := 3
	f.SetCellValue(sheet, "A"+fmt.Sprint(headerRow), "序号")
	for i, field := range fields {
		f.SetCellValue(sheet, cellName(i+2)+fmt.Sprint(headerRow), exportFieldColumns[field])
	}

	// 金额列位置（小计/合计落在此列）
	amountCol := -1
	for i, field := range fields {
		if field == "amount" {
			amountCol = i + 2
			break
		}
	}

	// ── 样式: 表头加粗 ──
	style, err := f.NewStyle(&excelize.Style{Font: &excelize.Font{Bold: true}})
	if err == nil && style > 0 {
		lastCol := cellName(len(fields) + 1)
		_ = f.SetCellStyle(sheet, "A"+fmt.Sprint(headerRow), lastCol+fmt.Sprint(headerRow), style)
	}

	// ── 数据行: 业务员模式按企业分组（组标题 + 组内数据 + 组小计） ──
	row := headerRow + 1
	groups := s.groupByEnterprise(req, orders)
	for _, g := range groups {
		if req.Mode == ExportModeRepairer {
			// 分组标题行
			f.SetCellValue(sheet, "A"+fmt.Sprint(row), "【"+g.Name+"】")
			if style > 0 {
				_ = f.SetCellStyle(sheet, "A"+fmt.Sprint(row), cellName(len(fields)+1)+fmt.Sprint(row), style)
			}
			row++
		}
		groupAmount := 0.0
		groupCount := 0
		for idx, o := range g.Orders {
			amount := amountOf(o)
			content := o.RepairContent
			if strings.TrimSpace(content) == "" {
				content = o.ProjectName
			}
			metadata := s.parseMetadata(o.Metadata)

			f.SetCellValue(sheet, "A"+fmt.Sprint(row), idx+1)
			col := 2
			for _, field := range fields {
				f.SetCellValue(sheet, cellName(col)+fmt.Sprint(row), s.fieldValue(field, o, amount, content, metadata, repairerNames[o.ID]))
				col++
			}
			groupAmount += amount
			groupCount++
			row++
		}
		// 组小计（业务员模式，每组一个小计行）
		if req.Mode == ExportModeRepairer {
			f.SetCellValue(sheet, "A"+fmt.Sprint(row), "小计")
			if amountCol > 0 {
				f.SetCellValue(sheet, cellName(amountCol)+fmt.Sprint(row), fmt.Sprintf("%.2f", groupAmount))
			}
			f.SetCellValue(sheet, cellName(len(fields)+1)+fmt.Sprint(row), fmt.Sprintf("共 %d 单", groupCount))
			row++
		}
		totalAmount += groupAmount
		totalCount += groupCount
	}

	// ── 合计行 ──
	sumRow := row
	f.SetCellValue(sheet, "A"+fmt.Sprint(sumRow), "合计")
	if amountCol > 0 {
		f.SetCellValue(sheet, cellName(amountCol)+fmt.Sprint(sumRow), fmt.Sprintf("%.2f", totalAmount))
	}
	f.SetCellValue(sheet, cellName(len(fields)+1)+fmt.Sprint(sumRow), fmt.Sprintf("共 %d 单", totalCount))
	if style > 0 {
		lastCol := cellName(len(fields) + 1)
		_ = f.SetCellStyle(sheet, "A"+fmt.Sprint(sumRow), lastCol+fmt.Sprint(sumRow), style)
	}

	// 写入缓冲区
	buf := new(bytes.Buffer)
	if err := f.Write(buf); err != nil {
		s.logger.Error("export excel write failed", zap.Error(err))
		return nil, apperrors.ErrExportFailed.WithError(err)
	}

	// 文件名
	monthText := req.DateTo.Format("2006年1月")
	filename := fmt.Sprintf("对账单_%s_%s.xlsx", infoValue, monthText)
	if req.Mode == ExportModeRepairer {
		filename = fmt.Sprintf("维修汇总表_%s_%s.xlsx", infoValue, monthText)
	}

	return &ExportResult{
		Filename:    filename,
		ContentType: "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet",
		Content:     buf,
	}, nil
}

// exportGroup 按企业分组后的一个区块
type exportGroup struct {
	Name   string
	Orders []model.RepairOrder
}

// groupByEnterprise 按企业分组。
// 企业对账单(enterprise)模式固定单组；业务员汇总(repairer)模式按 EnterpriseID 分组，
// 组序按工单在结果中的首次出现顺序，保证同一企业的工单连续展示。
func (s *OrderExportService) groupByEnterprise(req ExportRequest, orders []model.RepairOrder) []exportGroup {
	if req.Mode == ExportModeEnterprise {
		name := ""
		for _, o := range orders {
			if o.Enterprise.Name != "" {
				name = o.Enterprise.Name
				break
			}
		}
		return []exportGroup{{Name: name, Orders: orders}}
	}

	groups := make([]exportGroup, 0)
	index := make(map[string]int)
	for _, o := range orders {
		id := ""
		if o.EnterpriseID != nil {
			id = *o.EnterpriseID
		}
		name := o.Enterprise.Name
		if name == "" {
			name = "未分组"
		}
		if gi, ok := index[id]; ok {
			groups[gi].Orders = append(groups[gi].Orders, o)
		} else {
			index[id] = len(groups)
			groups = append(groups, exportGroup{Name: name, Orders: []model.RepairOrder{o}})
		}
	}
	return groups
}

// fieldValue 按字段标识取值
func (s *OrderExportService) fieldValue(field string, o model.RepairOrder, amount float64, content string, m *model.RepairMetadata, repairerName string) interface{} {
	switch field {
	case "order_no":
		if o.OrderNo != nil {
			return *o.OrderNo
		}
		return ""
	case "time":
		if o.CompletedAt != nil {
			return o.CompletedAt.Format("2006-01-02")
		}
		return ""
	case "content":
		return content
	case "quantity":
		return o.Quantity
	case "unit_price":
		return o.UnitPrice
	case "amount":
		return amount
	case "remark":
		return m.ExtraRemark
	case "repair_result":
		return m.RepairResult
	case "repair_method":
		return m.RepairMethod
	case "warranty_period":
		return m.WarrantyPeriod
	case "repair_duration":
		return m.RepairDuration
	case "reporter":
		return o.Reporter.Nickname
	case "repairer":
		return repairerName
	case "room":
		return o.Room
	case "contact":
		return o.Contact
	case "enterprise_name":
		return o.Enterprise.Name
	default:
		return ""
	}
}

// amountOf 计算工单金额: 优先取数据库生成列 amount, 为空/为 0 时用 quantity*unit_price 兜底
func amountOf(o model.RepairOrder) float64 {
	if o.Amount > 0 {
		return o.Amount
	}
	if o.Quantity == 0 {
		return 0
	}
	return o.UnitPrice * float64(o.Quantity)
}

// parseMetadata 解析 metadata JSONB, 失败时返回空结构
func (s *OrderExportService) parseMetadata(raw []byte) *model.RepairMetadata {
	m := &model.RepairMetadata{}
	if len(raw) == 0 {
		return m
	}
	if err := json.Unmarshal(raw, m); err != nil {
		return &model.RepairMetadata{}
	}
	return m
}

// cellName 列号转 Excel 列名 (1→A, 2→B, ...)
func cellName(col int) string {
	s := ""
	for col > 0 {
		col--
		s = string(rune('A'+col%26)) + s
		col /= 26
	}
	return s
}
