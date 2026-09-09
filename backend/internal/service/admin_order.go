// AdminOrderService 管理后台工单处理业务逻辑 (第五章 5.1-5.16)。
//
// 权限: 本服务内统一按 Operator(用户ID+平台角色) 判定:
//   - 店方操作 (接单/处理/完工/重新打开/收据/对账): 维修业务员(role=1)/超管(role=2)
//   - 单位侧操作 (审核通过 reported→pending_accept、退回 reported): 该单位单位审核员(membership.role=1) 亦可
//
// 具体单位域校验统一走 AccessService, 便于后续甲方细化"业务范围"等约束时集中调整。
package service

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"
	"gorm.io/datatypes"

	apperrors "xin-ni-repair/internal/errors"
	"xin-ni-repair/internal/model"
	"xin-ni-repair/internal/repository"
	"xin-ni-repair/pkg/imagebed"
)

// maxReceiptImages 同一工单收据图上限 (5.7)
const maxReceiptImages = 3

// ────────────────────────────────────────────
// 输出结构
// ────────────────────────────────────────────

// AdminReporter 报修人摘要
type AdminReporter struct {
	ID        string `json:"id"`
	Nickname  string `json:"nickname"`
	AvatarURL string `json:"avatar_url"`
}

// AdminOrderItem 管理端工单列表项 (5.1)
type AdminOrderItem struct {
	ID             string        `json:"id"`
	OrderNo        *string       `json:"order_no"`
	Reporter       AdminReporter `json:"reporter"`
	EnterpriseID   string        `json:"enterprise_id"`
	EnterpriseName string        `json:"enterprise_name"`
	CategoryID     string        `json:"category_id"`
	CategoryName   string        `json:"category_name"`
	PropertyID     string        `json:"property_id"`
	PropertyName   string        `json:"property_name"`
	Description    string        `json:"description"`
	Urgency        string        `json:"urgency"`
	UrgencyLabel   string        `json:"urgency_label"`
	Status         string        `json:"status"`
	StatusLabel    string        `json:"status_label"`
	ImageCount     int64         `json:"image_count"`
	SubmittedAt    *time.Time    `json:"submitted_at"`
	CreatedAt      time.Time     `json:"created_at"`
	// V1.3: 列表卡片需展示 位置 / 联系人 / 故障图缩略图
	Room    string      `json:"room"`
	Contact string      `json:"contact"`
	Images  []ImageItem `json:"images"`
	// V1.4: 列表展示维修操作内容与金额（完工/对账字段；未完工为空/0）
	RepairContent string  `json:"repair_content"`
	Amount        float64 `json:"amount"`
	// 列表卡片按权限就地操作所需的可执行动作（与详情页一致，按状态生成）
	AvailableActions []AdminAction `json:"available_actions"`
}

// AdminOrderList 管理端工单分页结果 (5.1)
type AdminOrderList struct {
	List       []AdminOrderItem `json:"list"`
	Total      int64            `json:"total"`
	Page       int              `json:"page"`
	PageSize   int              `json:"page_size"`
	TotalPages int              `json:"total_pages"`
}

// AdminAction 管理端可执行动作 (5.2)
type AdminAction struct {
	Action          string `json:"action"`
	Label           string `json:"label"`
	ToStatus        string `json:"to_status"`
	RequireReason   bool   `json:"require_reason,omitempty"`
	ReasonMinLength int    `json:"reason_min_length,omitempty"`
	ConfirmMessage  string `json:"confirm_message"`
}

// AdminTimelineItem 管理端时间轴项 (5.2, 含操作人角色与 IP)
type AdminTimelineItem struct {
	ID           string    `json:"id"`
	Action       string    `json:"action"`
	ActionLabel  string    `json:"action_label"`
	OperatorName string    `json:"operator_name"`
	OperatorRole string    `json:"operator_role"`
	FromStatus   *string   `json:"from_status"`
	ToStatus     *string   `json:"to_status"`
	Remark       *string   `json:"remark"`
	IpAddress    string    `json:"ip_address"`
	CreatedAt    time.Time `json:"created_at"`
}

// AdminOrderDetail 管理端工单详情 (5.2)
type AdminOrderDetail struct {
	ID               string                `json:"id"`
	OrderNo          *string               `json:"order_no"`
	EnterpriseID     string                `json:"enterprise_id"`
	EnterpriseName   string                `json:"enterprise_name"`
	Reporter         AdminReporter         `json:"reporter"`
	CategoryID       string                `json:"category_id"`
	CategoryName     string                `json:"category_name"`
	PropertyID       string                `json:"property_id"`
	PropertyName     string                `json:"property_name"`
	Description      string                `json:"description"`
	Urgency          string                `json:"urgency"`
	UrgencyLabel     string                `json:"urgency_label"`
	Room             string                `json:"room"`
	Contact          string                `json:"contact"`
	Status           string                `json:"status"`
	StatusLabel      string                `json:"status_label"`
	RejectReason     string                `json:"reject_reason"`
	RepairContent    string                `json:"repair_content"`
	Quantity         int                   `json:"quantity"`
	UnitPrice        float64               `json:"unit_price"`
	Amount           float64               `json:"amount"`
	Metadata         *model.RepairMetadata `json:"metadata,omitempty"`
	AuditorName      string                `json:"auditor_name,omitempty"`
	AuditorID        string                `json:"auditor_id,omitempty"`
	RepairerName     string                `json:"repairer_name,omitempty"`
	RepairerID       string                `json:"repairer_id,omitempty"`
	Images           []ImageItem           `json:"images"`
	Receipts         []ImageItem           `json:"receipts"`
	Timeline         []AdminTimelineItem   `json:"timeline"`
	AvailableActions []AdminAction         `json:"available_actions"`
	CreatedAt        time.Time             `json:"created_at"`
	SubmittedAt      *time.Time            `json:"submitted_at"`
	AuditedAt        *time.Time            `json:"audited_at,omitempty"`
	AcceptedAt       *time.Time            `json:"accepted_at,omitempty"`
	CompletedAt      *time.Time            `json:"completed_at,omitempty"`
	UpdatedAt        time.Time             `json:"updated_at"`
}

// ────────────────────────────────────────────
// Service
// ────────────────────────────────────────────

// AdminOrderService 管理后台工单处理逻辑
type AdminOrderService struct {
	orders    *repository.OrderRepository
	images    *repository.OrderImageRepository
	timelines *repository.OrderTimelineRepository
	access    *AccessService
	imagebed  *imagebed.Client
	notifier  *OrderNotifier
	logger    *zap.Logger
}

// NewAdminOrderService 创建 AdminOrderService
func NewAdminOrderService(
	orders *repository.OrderRepository,
	images *repository.OrderImageRepository,
	timelines *repository.OrderTimelineRepository,
	access *AccessService,
	imagebed *imagebed.Client,
	notifier *OrderNotifier,
	logger *zap.Logger,
) *AdminOrderService {
	return &AdminOrderService{
		orders:    orders,
		images:    images,
		timelines: timelines,
		access:    access,
		imagebed:  imagebed,
		notifier:  notifier,
		logger:    logger,
	}
}

// ListRepairers 维修员(业务员)列表 (5.15): users.role>=1 即维修业务员/超管
func (s *AdminOrderService) ListRepairers(ctx context.Context) ([]model.User, error) {
	return s.orders.ListRepairers(ctx)
}

// ListOrders 工单列表 (5.1)
//
// 鉴权: 店方角色 (role>=1) 可查全部; 单位审核员 (role=0 + membership.role=1)
// 必须传 enterprise_id 且限定于其有权限的单位。
func (s *AdminOrderService) ListOrders(ctx context.Context, op Operator, f repository.OrderAdminFilter, page, pageSize int) (*AdminOrderList, error) {
	for _, st := range f.Status {
		if _, ok := statusLabels[st]; !ok {
			return nil, apperrors.ErrInvalidParam.WithMessage("status 取值: draft/reported/pending_accept/processing/completed/cancelled")
		}
	}
	if f.Urgency != "" {
		if _, ok := urgencyLabels[f.Urgency]; !ok {
			return nil, apperrors.ErrInvalidParam.WithMessage("urgency 取值: normal/urgent/very_urgent")
		}
	}
	if f.SortBy != "" {
		switch f.SortBy {
		case "order_no", "enterprise_name", "reporter", "category_name", "urgency", "status", "submitted_at", "created_at":
		default:
			return nil, apperrors.ErrInvalidParam.WithMessage("sort_by 取值: order_no/enterprise_name/reporter/category_name/urgency/status/submitted_at/created_at")
		}
	}
	if f.SortOrder != "" && !strings.EqualFold(f.SortOrder, "asc") && !strings.EqualFold(f.SortOrder, "desc") {
		return nil, apperrors.ErrInvalidParam.WithMessage("sort_order 取值: asc/desc")
	}

	// 单位审核员限定本单位
	if !op.IsStoreStaff() {
		if f.EnterpriseID == "" {
			return nil, apperrors.ErrWrongEnterprise.WithMessage("单位审核员需指定 enterprise_id 并仅可查看本单位工单")
		}
		if err := s.access.CanManageEnterprise(ctx, f.EnterpriseID, op.UserID, op.Role); err != nil {
			return nil, err
		}
	}

	// 维修业务员（非超管）查看「处理中」时只能看到自己接的工单（repairer_id = 自己）
	if op.IsPlainRepairer() && statusSliceContains(f.Status, string(model.OrderProcessing)) {
		f.RepairerID = op.UserID
	}

	orders, total, err := s.orders.ListForAdmin(ctx, f, (page-1)*pageSize, pageSize)
	if err != nil {
		return nil, s.dbErr("list orders for admin failed", err)
	}

	orderIDs := make([]string, 0, len(orders))
	for _, o := range orders {
		orderIDs = append(orderIDs, o.ID)
	}
	imgCounts, err := s.images.CountActiveByOrders(ctx, orderIDs, string(model.ImageFault))
	if err != nil {
		return nil, s.dbErr("count images failed", err)
	}
	// V1.3: 卡片缩略图（故障图，active）
	imgList, err := s.images.ListActiveByOrders(ctx, orderIDs, string(model.ImageFault))
	if err != nil {
		return nil, s.dbErr("list images failed", err)
	}

	list := make([]AdminOrderItem, 0, len(orders))
	for _, o := range orders {
		list = append(list, AdminOrderItem{
			ID:               o.ID,
			OrderNo:          o.OrderNo,
			Reporter:         AdminReporter{ID: o.Reporter.ID, Nickname: o.Reporter.Nickname, AvatarURL: o.Reporter.AvatarUrl},
			EnterpriseID:     enterpriseIDStr(o.EnterpriseID),
			EnterpriseName:   o.Enterprise.Name,
			CategoryID:       nstr(o.CategoryID),
			CategoryName:     o.CategoryName,
			PropertyID:       nstr(o.PropertyID),
			PropertyName:     o.PropertyName,
			Description:      o.Description,
			Urgency:          o.Urgency,
			UrgencyLabel:     urgencyLabels[o.Urgency],
			Status:           o.Status,
			StatusLabel:      statusLabels[o.Status],
			ImageCount:       imgCounts[o.ID],
			SubmittedAt:      o.SubmittedAt,
			CreatedAt:        o.CreatedAt,
			Room:             o.Room,
			Contact:          o.Contact,
			Images:           buildImageItems(imgList[o.ID]),
			RepairContent:    o.RepairContent,
			Amount:           o.Amount,
			AvailableActions: filterOwnerActions(op, o.RepairerID, adminAvailableActions(o.Status)),
		})
	}

	totalPages := int(total) / pageSize
	if int(total)%pageSize > 0 {
		totalPages++
	}
	return &AdminOrderList{
		List:       list,
		Total:      total,
		Page:       page,
		PageSize:   pageSize,
		TotalPages: totalPages,
	}, nil
}

// ── 5.17 本单位工单汇总统计 (V1.3 C20) ──

// StatsRequest 汇总统计查询参数 (5.17)
type StatsRequest struct {
	EnterpriseID string
	Start        *time.Time // 含；nil=不限
	End          *time.Time // 不含（开区间）；nil=至今
}

// StatusStat 状态分布项
type StatusStat struct {
	Status string `json:"status"`
	Label  string `json:"label"`
	Count  int64  `json:"count"`
}

// CategoryStat 项目大类分布项
type CategoryStat struct {
	CategoryID   string `json:"category_id"`
	CategoryName string `json:"category_name"`
	Count        int64  `json:"count"`
}

// ReporterStat 报修人排行项（按提交账号昵称；C21 起含真实头像）
type ReporterStat struct {
	UserID    string `json:"user_id"`
	Nickname  string `json:"nickname"`
	AvatarURL string `json:"avatar_url"`
	Count     int64  `json:"count"`
}

// OrderToday 今日概况（5.17 data.today，独立于 start/end）
type OrderToday struct {
	PendingReview  int64 `json:"pending_review"`
	SubmittedToday int64 `json:"submitted_today"`
	AuditedToday   int64 `json:"audited_today"`
	RejectedToday  int64 `json:"rejected_today"`
}

// StatsRange 实际生效的时间范围
type StatsRange struct {
	Start *time.Time `json:"start"`
	End   *time.Time `json:"end"`
}

// ReporterOptionView 报修人下拉选项（5.1 列表筛选配套）
type ReporterOptionView struct {
	UserID    string `json:"id"`
	Nickname  string `json:"nickname"`
	AvatarURL string `json:"avatar_url"`
}

// ReporterOptionsResponse 报修人候选响应
type ReporterOptionsResponse struct {
	List []ReporterOptionView `json:"list"`
}

// ListReporterOptions 报修人候选（按企业域 + 昵称关键字；审核员须限定本单位）。
func (s *AdminOrderService) ListReporterOptions(ctx context.Context, op Operator, enterpriseID, keyword string) (*ReporterOptionsResponse, error) {
	if !op.IsStoreStaff() {
		if enterpriseID == "" {
			return nil, apperrors.ErrWrongEnterprise.WithMessage("单位审核员需指定 enterprise_id 查询本单位报修人")
		}
		if err := s.access.CanManageEnterprise(ctx, enterpriseID, op.UserID, op.Role); err != nil {
			return nil, err
		}
	}
	rows, err := s.orders.ReporterOptions(ctx, enterpriseID, keyword, 200)
	if err != nil {
		return nil, s.dbErr("list reporter options failed", err)
	}
	list := make([]ReporterOptionView, 0, len(rows))
	for _, r := range rows {
		list = append(list, ReporterOptionView{UserID: r.UserID, Nickname: r.Nickname, AvatarURL: r.AvatarURL})
	}
	return &ReporterOptionsResponse{List: list}, nil
}

// OrderStats 工单汇总统计响应 (5.17)
type OrderStats struct {
	Today        OrderToday     `json:"today"`
	Total        int64          `json:"total"`
	Range        StatsRange     `json:"range"`
	ByStatus     []StatusStat   `json:"by_status"`
	ByCategory   []CategoryStat `json:"by_category"`
	TopReporters []ReporterStat `json:"top_reporters"`
	UpdatedAt    time.Time      `json:"updated_at"`
}

// EnterpriseStatsRow 企业维度分组聚合项 (5.18)
type EnterpriseStatsRow struct {
	EnterpriseID   string       `json:"enterprise_id"`
	EnterpriseName string       `json:"enterprise_name"`
	Total          int64        `json:"total"`
	ByStatus       []StatusStat `json:"by_status"` // 六态补零（与 5.17 一致）
}

// EnterpriseStats 企业维度分组聚合响应 (5.18)
type EnterpriseStats struct {
	Range     StatsRange           `json:"range"`
	List      []EnterpriseStatsRow `json:"list"`
	UpdatedAt time.Time            `json:"updated_at"`
}

// RepairerStatsRow 维修员业绩聚合项 (5.19)
type RepairerStatsRow struct {
	RepairerID   string `json:"repairer_id"`
	RepairerName string `json:"repairer_name"`
	Assigned     int64  `json:"assigned"`  // 接单/处理量：待接单/处理中/已完工（repairer_id 非空）
	Completed    int64  `json:"completed"` // 完工量：completed_at 非空
}

// RepairerStats 维修员业绩聚合响应 (5.19)
type RepairerStats struct {
	Range     StatsRange         `json:"range"`
	List      []RepairerStatsRow `json:"list"`
	UpdatedAt time.Time          `json:"updated_at"`
}

// OrderMetrics 区间运营指标响应 (5.20)
type OrderMetrics struct {
	PendingReview int64      `json:"pending_review"` // 当前待审核存量（reported，不计时间）
	Submitted     int64      `json:"submitted"`      // 区间上报
	Audited       int64      `json:"audited"`        // 区间审核通过
	Rejected      int64      `json:"rejected"`       // 区间退回
	Range         StatsRange `json:"range"`
	UpdatedAt     time.Time  `json:"updated_at"`
}

// RepairerSummaryRowView 维修员区间汇总项 (5.21)
type RepairerSummaryRowView struct {
	RepairerID   string `json:"repairer_id"`
	RepairerName string `json:"repairer_name"`
	Accepted     int64  `json:"accepted"`  // 区间接单（accepted_at∈窗口）
	Completed    int64  `json:"completed"` // 区间完工（completed_at∈窗口）
}

// RepairerSummary 维修员区间汇总响应 (5.21)
type RepairerSummary struct {
	GlobalPendingAccept int64                    `json:"global_pending_accept"` // 全局待接单存量（不受所选业务员影响）
	List                []RepairerSummaryRowView `json:"list"`
	Range               StatsRange               `json:"range"`
	UpdatedAt           time.Time                `json:"updated_at"`
}

// RepairerOverview 维修员个人汇总响应 (5.22，小程序「处理工单」统计卡：累计 + 今日)
type RepairerOverview struct {
	PendingAccept  int64     `json:"pending_accept"`  // 可接单存量
	MyAccepted     int64     `json:"my_accepted"`     // 我的累计接单
	MyProcessing   int64     `json:"my_processing"`   // 处理中
	MyCompleted    int64     `json:"my_completed"`    // 累计完工
	TodayAccepted  int64     `json:"today_accepted"`  // 今日接单
	TodayCompleted int64     `json:"today_completed"` // 今日完工
	UpdatedAt      time.Time `json:"updated_at"`
}

// statsStatusOrder 状态分布固定展示顺序（零填充用）
var statsStatusOrder = []string{
	string(model.OrderReported),
	string(model.OrderPendingAccept),
	string(model.OrderProcessing),
	string(model.OrderCompleted),
	string(model.OrderCancelled),
	string(model.OrderRejected),
}

// Stats 工单汇总统计 (5.17)。
//
// 鉴权: 店方角色 (role>=1) 可不传 enterprise_id（全部可见单位）或指定单单位;
// 单位审核员 (role=0 + membership.role=1) 必须传 enterprise_id 且为其担任审核员的单位。
func (s *AdminOrderService) Stats(ctx context.Context, op Operator, req StatsRequest) (*OrderStats, error) {
	if req.Start != nil && req.End != nil && req.End.Before(*req.Start) {
		return nil, apperrors.ErrInvalidParam.WithMessage("end 不能早于 start")
	}
	if !op.IsStoreStaff() {
		if req.EnterpriseID == "" {
			return nil, apperrors.ErrWrongEnterprise.WithMessage("单位审核员需指定 enterprise_id 并仅可统计本单位工单")
		}
		if err := s.access.CanManageEnterprise(ctx, req.EnterpriseID, op.UserID, op.Role); err != nil {
			return nil, err
		}
	}

	agg, err := s.orders.StatsForAdmin(ctx, req.EnterpriseID, req.Start, req.End)
	if err != nil {
		return nil, s.dbErr("stats for admin failed", err)
	}

	countByStatus := make(map[string]int64, len(agg.ByStatus))
	for _, st := range agg.ByStatus {
		countByStatus[st.Status] = st.Count
	}
	byStatus := make([]StatusStat, 0, len(statsStatusOrder))
	for _, code := range statsStatusOrder {
		byStatus = append(byStatus, StatusStat{Status: code, Label: statusLabels[code], Count: countByStatus[code]})
	}

	byCategory := make([]CategoryStat, 0, len(agg.ByCategory))
	for _, c := range agg.ByCategory {
		byCategory = append(byCategory, CategoryStat{CategoryID: c.CategoryID, CategoryName: c.CategoryName, Count: c.Count})
	}

	topReporters := make([]ReporterStat, 0, len(agg.TopReporters))
	for _, r := range agg.TopReporters {
		topReporters = append(topReporters, ReporterStat{UserID: r.UserID, Nickname: r.Nickname, AvatarURL: r.AvatarURL, Count: r.Count})
	}

	// 今日概况（独立于 start/end，按服务器本地日 0 点 ~ 次日 0 点）
	now := time.Now()
	dayStart := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	dayEnd := dayStart.Add(24 * time.Hour)
	todayAgg, err := s.orders.StatsToday(ctx, req.EnterpriseID, dayStart, dayEnd)
	if err != nil {
		return nil, s.dbErr("stats today failed", err)
	}

	return &OrderStats{
		Today: OrderToday{
			PendingReview:  todayAgg.PendingReview,
			SubmittedToday: todayAgg.SubmittedToday,
			AuditedToday:   todayAgg.AuditedToday,
			RejectedToday:  todayAgg.RejectedToday,
		},
		Total:        agg.Total,
		Range:        StatsRange{Start: req.Start, End: req.End},
		ByStatus:     byStatus,
		ByCategory:   byCategory,
		TopReporters: topReporters,
		UpdatedAt:    now,
	}, nil
}

// validateStatsRange 校验统计时间区间合法性
func validateStatsRange(req StatsRequest) error {
	if req.Start != nil && req.End != nil && req.End.Before(*req.Start) {
		return apperrors.ErrInvalidParam.WithMessage("end 不能早于 start")
	}
	return nil
}

// StatsByEnterprise 企业维度分组聚合 (5.18)。仅店方/超管（单位审核员不可见）。
// StatsMetrics 5.20 区间运营指标。鉴权同 5.17：店方可全域或指定企业；单位审核员必带本单位。
func (s *AdminOrderService) StatsMetrics(ctx context.Context, op Operator, req StatsRequest) (*OrderMetrics, error) {
	if err := validateStatsRange(req); err != nil {
		return nil, err
	}
	if !op.IsStoreStaff() {
		if req.EnterpriseID == "" {
			return nil, apperrors.ErrWrongEnterprise.WithMessage("单位审核员需指定 enterprise_id 并仅可统计本单位工单")
		}
		if err := s.access.CanManageEnterprise(ctx, req.EnterpriseID, op.UserID, op.Role); err != nil {
			return nil, err
		}
	}
	agg, err := s.orders.StatsMetrics(ctx, req.EnterpriseID, req.Start, req.End)
	if err != nil {
		return nil, s.dbErr("stats metrics failed", err)
	}
	return &OrderMetrics{
		PendingReview: agg.PendingReview,
		Submitted:     agg.Submitted,
		Audited:       agg.Audited,
		Rejected:      agg.Rejected,
		Range:         StatsRange{Start: req.Start, End: req.End},
		UpdatedAt:     time.Now(),
	}, nil
}

// StatsRepairerSummary 5.21 维修员区间汇总。仅店方/超管。
func (s *AdminOrderService) StatsRepairerSummary(ctx context.Context, op Operator, repairerID string, req StatsRequest) (*RepairerSummary, error) {
	if !op.IsStoreStaff() {
		return nil, apperrors.ErrForbidden.WithMessage("业务员统计仅维修业务员/超级管理员可见")
	}
	// 越权防护: 维修业务员(非超管)仅可查看本人区间汇总, 忽略传入的 repairer_id
	if op.IsPlainRepairer() {
		repairerID = op.UserID
	}
	if err := validateStatsRange(req); err != nil {
		return nil, err
	}
	agg, err := s.orders.StatsRepairerSummary(ctx, repairerID, req.Start, req.End)
	if err != nil {
		return nil, s.dbErr("stats repairer summary failed", err)
	}
	list := make([]RepairerSummaryRowView, 0, len(agg.Rows))
	for _, row := range agg.Rows {
		list = append(list, RepairerSummaryRowView{
			RepairerID:   row.RepairerID,
			RepairerName: row.RepairerName,
			Accepted:     row.Accepted,
			Completed:    row.Completed,
		})
	}
	return &RepairerSummary{
		GlobalPendingAccept: agg.GlobalPendingAccept,
		List:                list,
		Range:               StatsRange{Start: req.Start, End: req.End},
		UpdatedAt:           time.Now(),
	}, nil
}

// StatsRepairerOverview 5.22 维修员个人汇总（小程序「处理工单」统计卡：累计 + 今日双行）。
// 仅店方/超管；repairerID 为空时统计当前操作者本人；enterpriseID 空串=全部企业域。
func (s *AdminOrderService) StatsRepairerOverview(ctx context.Context, op Operator, repairerID, enterpriseID string) (*RepairerOverview, error) {
	if !op.IsStoreStaff() {
		return nil, apperrors.ErrForbidden.WithMessage("仅维修业务员/超级管理员可查看处理统计")
	}
	// 越权防护: 维修业务员(非超管)仅可查看本人汇总, 忽略传入的 repairer_id
	if op.IsPlainRepairer() || repairerID == "" {
		repairerID = op.UserID
	}
	now := time.Now()
	dayStart := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	dayEnd := dayStart.Add(24 * time.Hour)
	agg, err := s.orders.StatsRepairerOverview(ctx, repairerID, enterpriseID, dayStart, dayEnd)
	if err != nil {
		return nil, s.dbErr("stats repairer overview failed", err)
	}
	return &RepairerOverview{
		PendingAccept:  agg.PendingAccept,
		MyAccepted:     agg.MyAccepted,
		MyProcessing:   agg.MyProcessing,
		MyCompleted:    agg.MyCompleted,
		TodayAccepted:  agg.TodayAccepted,
		TodayCompleted: agg.TodayCompleted,
		UpdatedAt:      now,
	}, nil
}

func (s *AdminOrderService) StatsByEnterprise(ctx context.Context, op Operator, req StatsRequest) (*EnterpriseStats, error) {
	if !op.IsStoreStaff() {
		return nil, apperrors.ErrForbidden.WithMessage("企业对比仅维修业务员/超级管理员可见")
	}
	if err := validateStatsRange(req); err != nil {
		return nil, err
	}
	rows, err := s.orders.StatsByEnterprise(ctx, req.EnterpriseID, req.Start, req.End)
	if err != nil {
		return nil, s.dbErr("stats by enterprise failed", err)
	}
	list := make([]EnterpriseStatsRow, 0, len(rows))
	for _, row := range rows {
		countByStatus := make(map[string]int64, len(row.ByStatus))
		for _, st := range row.ByStatus {
			countByStatus[st.Status] = st.Count
		}
		byStatus := make([]StatusStat, 0, len(statsStatusOrder))
		for _, code := range statsStatusOrder {
			byStatus = append(byStatus, StatusStat{Status: code, Label: statusLabels[code], Count: countByStatus[code]})
		}
		list = append(list, EnterpriseStatsRow{
			EnterpriseID:   row.EnterpriseID,
			EnterpriseName: row.EnterpriseName,
			Total:          row.Total,
			ByStatus:       byStatus,
		})
	}
	return &EnterpriseStats{
		Range:     StatsRange{Start: req.Start, End: req.End},
		List:      list,
		UpdatedAt: time.Now(),
	}, nil
}

// StatsByRepairer 维修员维度聚合 (5.19)。仅店方/超管（单位审核员不可见）。
func (s *AdminOrderService) StatsByRepairer(ctx context.Context, op Operator, req StatsRequest) (*RepairerStats, error) {
	if !op.IsStoreStaff() {
		return nil, apperrors.ErrForbidden.WithMessage("维修员业绩仅维修业务员/超级管理员可见")
	}
	if err := validateStatsRange(req); err != nil {
		return nil, err
	}
	rows, err := s.orders.StatsByRepairer(ctx, req.EnterpriseID, req.Start, req.End)
	if err != nil {
		return nil, s.dbErr("stats by repairer failed", err)
	}
	list := make([]RepairerStatsRow, 0, len(rows))
	for _, row := range rows {
		list = append(list, RepairerStatsRow{
			RepairerID:   row.RepairerID,
			RepairerName: row.RepairerName,
			Assigned:     row.Assigned,
			Completed:    row.Completed,
		})
	}
	return &RepairerStats{
		Range:     StatsRange{Start: req.Start, End: req.End},
		List:      list,
		UpdatedAt: time.Now(),
	}, nil
}

// Detail 工单详情 (5.2)
//
// 鉴权: 店方角色; 或该工单所属单位的单位审核员。
func (s *AdminOrderService) Detail(ctx context.Context, op Operator, orderID string) (*AdminOrderDetail, error) {
	order, err := s.loadOrder(ctx, orderID)
	if err != nil {
		return nil, err
	}
	if err := s.access.CanAccessOrder(ctx, order.EnterpriseID, op.UserID, op.Role); err != nil {
		return nil, err
	}

	images, err := s.images.ListActiveByOrder(ctx, orderID, string(model.ImageFault))
	if err != nil {
		return nil, s.dbErr("list images failed", err)
	}
	receipts, err := s.images.ListActiveByOrder(ctx, orderID, string(model.ImageReceipt))
	if err != nil {
		return nil, s.dbErr("list receipts failed", err)
	}
	timelines, err := s.timelines.ListByOrder(ctx, orderID)
	if err != nil {
		return nil, s.dbErr("list timelines failed", err)
	}

	return &AdminOrderDetail{
		ID:               order.ID,
		OrderNo:          order.OrderNo,
		EnterpriseID:     enterpriseIDStr(order.EnterpriseID),
		EnterpriseName:   order.Enterprise.Name,
		Reporter:         AdminReporter{ID: order.Reporter.ID, Nickname: order.Reporter.Nickname, AvatarURL: order.Reporter.AvatarUrl},
		CategoryID:       nstr(order.CategoryID),
		CategoryName:     order.CategoryName,
		PropertyID:       nstr(order.PropertyID),
		PropertyName:     order.PropertyName,
		Description:      order.Description,
		Urgency:          order.Urgency,
		UrgencyLabel:     urgencyLabels[order.Urgency],
		Room:             order.Room,
		Contact:          order.Contact,
		Status:           order.Status,
		StatusLabel:      statusLabels[order.Status],
		RejectReason:     order.RejectReason,
		RepairContent:    order.RepairContent,
		Quantity:         order.Quantity,
		UnitPrice:        order.UnitPrice,
		Amount:           order.Amount,
		Metadata:         parseOrderMetadata(order.Metadata),
		AuditorName:      order.Auditor.Nickname,
		AuditorID:        nstr(order.AuditedBy),
		RepairerName:     order.Repairer.Nickname,
		RepairerID:       nstr(order.RepairerID),
		Images:           buildImageItems(images),
		Receipts:         buildImageItems(receipts),
		Timeline:         buildAdminTimelineItems(timelines),
		AvailableActions: filterOwnerActions(op, order.RepairerID, adminAvailableActions(order.Status)),
		CreatedAt:        order.CreatedAt,
		SubmittedAt:      order.SubmittedAt,
		AuditedAt:        order.AuditedAt,
		AcceptedAt:       order.AcceptedAt,
		CompletedAt:      order.CompletedAt,
		UpdatedAt:        order.UpdatedAt,
	}, nil
}

// Audit 审核通过工单 (5.3): reported → pending_accept
//
// 鉴权: 该单位单位审核员或店方角色。
func (s *AdminOrderService) Audit(ctx context.Context, op Operator, orderID, remark, ip string) error {
	if err := validateLength("remark", remark, 0, 100); err != nil {
		return err
	}
	order, err := s.loadOrder(ctx, orderID)
	if err != nil {
		return err
	}
	if order.Status != string(model.OrderReported) {
		return apperrors.ErrOrderCannotEdit.WithMessage("仅 reported 状态的工单可审核通过")
	}
	if err := s.access.CanAccessOrder(ctx, order.EnterpriseID, op.UserID, op.Role); err != nil {
		return err
	}

	now := time.Now()
	from := order.Status
	order.Status = string(model.OrderPendingAccept)
	order.AuditedAt = &now
	order.AuditedBy = &op.UserID
	if err := s.orders.Update(ctx, order); err != nil {
		return s.dbErr("update order failed", err)
	}
	if err := s.appendAdminTimeline(ctx, order, op.UserID, string(model.ActionAudit), from, string(model.OrderPendingAccept), remark, ip); err != nil {
		return err
	}
	// 通知报修人: 已通过审核, 等待业务员接单
	if s.notifier != nil {
		s.notifier.NotifyOrderAudited(ctx, order)
	}
	s.logger.Info("order audited",
		zap.String("order_id", order.ID), zap.String("operator_id", op.UserID), zap.String("role", fmt.Sprint(op.Role)))
	return nil
}

// Accept 接单维修 (5.4): pending_accept → processing
//
// 鉴权: 店方角色 (维修业务员/超管)。
func (s *AdminOrderService) Accept(ctx context.Context, op Operator, orderID, remark, ip string) error {
	if err := s.access.StaffOnly(op); err != nil {
		return err
	}
	if err := validateLength("remark", remark, 0, 100); err != nil {
		return err
	}
	order, err := s.loadOrder(ctx, orderID)
	if err != nil {
		return err
	}
	if order.Status != string(model.OrderPendingAccept) {
		return apperrors.ErrOrderCannotEdit.WithMessage("仅 pending_accept 状态的工单可接单")
	}

	now := time.Now()
	from := order.Status
	order.Status = string(model.OrderProcessing)
	order.AcceptedAt = &now
	order.RepairerID = &op.UserID // 维修业务员（接单人）: accept 时写入
	if err := s.orders.Update(ctx, order); err != nil {
		return s.dbErr("update order failed", err)
	}
	if s.notifier != nil {
		s.notifier.NotifyOrderProcessing(ctx, order)
	}
	return s.appendAdminTimeline(ctx, order, op.UserID, string(model.ActionAccept), from, string(model.OrderProcessing), remark, ip)
}

// Reject 退回工单 (5.5): reported/pending_accept/processing → draft
//
// 鉴权: reported 由该单位单位审核员或店方角色退回;
// pending_accept/processing 仅店方角色退回。
func (s *AdminOrderService) Reject(ctx context.Context, op Operator, orderID, reason, ip string) error {
	reason = strings.TrimSpace(reason)
	n := len([]rune(reason))
	if n < 10 || n > 200 {
		return apperrors.ErrRejectReasonTooShort.WithMessage("退回原因需为10-200字")
	}

	order, err := s.loadOrder(ctx, orderID)
	if err != nil {
		return err
	}
	switch order.Status {
	case string(model.OrderReported):
		// 单位审核员或店方角色可退回已上报工单
		if err := s.access.CanAccessOrder(ctx, order.EnterpriseID, op.UserID, op.Role); err != nil {
			return err
		}
	case string(model.OrderPendingAccept), string(model.OrderProcessing):
		if err := s.access.StaffOnly(op); err != nil {
			return apperrors.ErrNotAdmin.WithMessage("仅维修业务员/超级管理员可退回待接单或处理中的工单")
		}
		// 归属校验：接单后的工单仅接单人本人或超级管理员可退回
		if err := ensureOrderOwner(op, order); err != nil {
			return err
		}
	default:
		return apperrors.ErrOrderCannotEdit.WithMessage("仅 reported/pending_accept/processing 状态的工单可退回")
	}

	from := order.Status
	order.Status = string(model.OrderRejected)
	order.RejectReason = reason
	// 退回派生规则: 清空流转时间戳/责任人 (按适用情况), 保留 reject_reason 供"已退回"展示
	order.SubmittedAt = nil
	order.AuditedAt = nil
	order.AuditedBy = nil
	order.AcceptedAt = nil
	order.RepairerID = nil
	if err := s.orders.Update(ctx, order); err != nil {
		return s.dbErr("update order failed", err)
	}
	if s.notifier != nil {
		s.notifier.NotifyOrderReject(ctx, order, reason)
	}
	return s.appendAdminTimeline(ctx, order, op.UserID, string(model.ActionReject), from, string(model.OrderRejected), reason, ip)
}

// CompleteOrderInput 完工请求入参 (5.6)
type CompleteOrderInput struct {
	Remark        string
	Receipts      []string
	Quantity      int
	UnitPrice     float64
	RepairContent string
	Metadata      model.RepairMetadata
}

// UpdateFinanceInput 修改对账信息入参 (5.6.1, 全部可选, 至少一项)
type UpdateFinanceInput struct {
	Quantity      *int
	UnitPrice     *float64
	RepairContent *string
	Metadata      *model.RepairMetadata
}

// Complete 完工 (5.6): processing → completed, 收据全量替换, 记录对账字段
//
// 鉴权: 店方角色。
func (s *AdminOrderService) Complete(ctx context.Context, op Operator, orderID, ip string, in CompleteOrderInput) error {
	if err := s.access.StaffOnly(op); err != nil {
		return err
	}
	remark := strings.TrimSpace(in.Remark)
	if err := validateLength("remark", remark, 1, 200); err != nil {
		return apperrors.ErrInvalidParam.WithMessage("完工备注必填且不超过200字")
	}
	if len(in.Receipts) == 0 {
		return apperrors.ErrInvalidParam.WithMessage("receipts 不能为空")
	}
	if len(in.Receipts) > maxReceiptImages {
		return apperrors.ErrImageTooMany.WithMessage("收据最多 3 张")
	}
	if in.Quantity < 0 {
		return apperrors.ErrInvalidParam.WithMessage("quantity 需 ≥0")
	}
	if in.UnitPrice < 0 {
		return apperrors.ErrInvalidParam.WithMessage("unit_price 需 ≥0")
	}
	if strings.TrimSpace(in.RepairContent) == "" {
		return apperrors.ErrInvalidParam.WithMessage("repair_content 必填")
	}

	order, err := s.loadOrder(ctx, orderID)
	if err != nil {
		return err
	}
	if order.Status != string(model.OrderProcessing) {
		return apperrors.ErrOrderCannotEdit.WithMessage("仅 processing 状态的工单可完工")
	}
	// 归属校验：仅接单人本人或超级管理员可完工
	if err := ensureOrderOwner(op, order); err != nil {
		return err
	}

	now := time.Now()
	from := order.Status
	order.Status = string(model.OrderCompleted)
	order.CompletedAt = &now
	order.RepairContent = strings.TrimSpace(in.RepairContent)
	order.Quantity = in.Quantity
	order.UnitPrice = in.UnitPrice
	if err := s.setOrderMetadata(ctx, order, in.Metadata); err != nil {
		return err
	}
	if err := s.orders.Update(ctx, order); err != nil {
		return s.dbErr("update order failed", err)
	}

	if err := s.replaceImages(ctx, order.ID, string(model.ImageReceipt), in.Receipts); err != nil {
		return err
	}

	s.logger.Info("order completed",
		zap.String("order_id", order.ID), zap.String("operator_id", op.UserID))

	if s.notifier != nil {
		s.notifier.NotifyOrderComplete(ctx, order, op.UserID)
	}

	return s.appendAdminTimeline(ctx, order, op.UserID, string(model.ActionComplete), from, string(model.OrderCompleted), remark, ip)
}

// UpdateFinance 修改对账信息 (5.6.1): 仅 completed 状态, 至少传一项; 鉴权: 店方角色
func (s *AdminOrderService) UpdateFinance(ctx context.Context, op Operator, orderID, ip string, in UpdateFinanceInput) error {
	if err := s.access.StaffOnly(op); err != nil {
		return err
	}
	order, err := s.loadOrder(ctx, orderID)
	if err != nil {
		return err
	}
	if order.Status != string(model.OrderCompleted) {
		return apperrors.ErrOrderCannotEdit.WithMessage("仅 completed 状态的工单可修改对账信息")
	}
	// 归属校验：仅接单人本人或超级管理员可修改对账
	if err := ensureOrderOwner(op, order); err != nil {
		return err
	}
	if in.Quantity == nil && in.UnitPrice == nil && in.RepairContent == nil && in.Metadata == nil {
		return apperrors.ErrInvalidParam.WithMessage("至少需要传入一个字段")
	}

	beforeQuantity := order.Quantity
	beforeUnitPrice := order.UnitPrice
	beforeRepairContent := order.RepairContent
	beforeMeta := parseMetadata(order.Metadata)

	if in.Quantity != nil {
		if *in.Quantity < 0 {
			return apperrors.ErrInvalidParam.WithMessage("quantity 需 ≥0")
		}
		order.Quantity = *in.Quantity
	}
	if in.UnitPrice != nil {
		if *in.UnitPrice < 0 {
			return apperrors.ErrInvalidParam.WithMessage("unit_price 需 ≥0")
		}
		order.UnitPrice = *in.UnitPrice
	}
	if in.RepairContent != nil {
		order.RepairContent = strings.TrimSpace(*in.RepairContent)
	}
	if in.Metadata != nil {
		if err := s.setOrderMetadata(ctx, order, *in.Metadata); err != nil {
			return err
		}
	}

	if err := s.orders.Update(ctx, order); err != nil {
		return s.dbErr("update order failed", err)
	}

	diff := financeIncrementDiff(beforeQuantity, beforeUnitPrice, beforeRepairContent, beforeMeta, order)
	return s.appendAdminTimeline(ctx, order, op.UserID, string(model.ActionUpdateFinance), order.Status, order.Status, diff, ip)
}

// Reopen 重新打开工单 (5.16): completed → processing (重新维修, 清空完工信息)
//
// 鉴权: 店方角色。
func (s *AdminOrderService) Reopen(ctx context.Context, op Operator, orderID, remark, ip string) error {
	if err := s.access.StaffOnly(op); err != nil {
		return err
	}
	if err := validateLength("remark", remark, 0, 200); err != nil {
		return err
	}
	order, err := s.loadOrder(ctx, orderID)
	if err != nil {
		return err
	}
	if order.Status != string(model.OrderCompleted) {
		return apperrors.ErrOrderCannotEdit.WithMessage("仅 completed 状态的工单可重新打开")
	}
	// 归属校验：仅接单人本人或超级管理员可重新打开
	if err := ensureOrderOwner(op, order); err != nil {
		return err
	}

	from := order.Status
	order.Status = string(model.OrderProcessing)
	order.CompletedAt = nil // 再次完工后重新登记
	if err := s.orders.Update(ctx, order); err != nil {
		return s.dbErr("update order failed", err)
	}
	if err := s.appendAdminTimeline(ctx, order, op.UserID, string(model.ActionReopen), from, string(model.OrderProcessing), remark, ip); err != nil {
		return err
	}
	// 通知报修人: 工单已重新处理
	if s.notifier != nil {
		s.notifier.NotifyOrderReopened(ctx, order)
	}
	s.logger.Info("order reopened",
		zap.String("order_id", order.ID), zap.String("operator_id", op.UserID))
	return nil
}

// UploadReceipt 上传收据图片 (5.7): processing(完工时) 或 completed(完工后补充)
//
// 鉴权: 店方角色。
func (s *AdminOrderService) UploadReceipt(ctx context.Context, op Operator, orderID, filename string, size int64, content io.Reader, ip string) (*UploadImageResult, error) {
	if err := s.access.StaffOnly(op); err != nil {
		return nil, err
	}
	order, err := s.loadOrder(ctx, orderID)
	if err != nil {
		return nil, err
	}
	switch order.Status {
	case string(model.OrderProcessing), string(model.OrderCompleted):
	default:
		return nil, apperrors.ErrOrderCannotEdit.WithMessage("仅 processing/completed 状态的工单可上传收据")
	}
	// 归属校验：仅接单人本人或超级管理员可上传该工单收据
	if err := ensureOrderOwner(op, order); err != nil {
		return nil, err
	}

	if size <= 0 || size > maxImageSize {
		return nil, apperrors.ErrImageInvalid.WithMessage("图片大小需不超过 5MB")
	}
	head := make([]byte, 512)
	n, err := io.ReadFull(io.LimitReader(content, int64(len(head))), head)
	if err != nil && err != io.EOF && err != io.ErrUnexpectedEOF {
		return nil, apperrors.ErrImageInvalid
	}
	head = head[:n]
	if !validImageFormat(head, filename) {
		return nil, apperrors.ErrImageInvalid.WithMessage("仅支持 jpg/png/webp 格式")
	}

	count, err := s.images.CountNotDeleted(ctx, orderID, string(model.ImageReceipt))
	if err != nil {
		return nil, s.dbErr("count receipts failed", err)
	}
	if count >= maxReceiptImages {
		return nil, apperrors.ErrImageTooMany.WithMessage("收据最多 3 张")
	}

	upload, err := s.imagebed.Upload(ctx, filename, io.MultiReader(bytes.NewReader(head), content))
	if err != nil {
		s.logger.Error("image bed upload failed", zap.Error(err))
		return nil, apperrors.ErrOSSUpload.WithError(err)
	}

	img := &model.OrderImage{
		ID:        uuid.New().String(),
		OrderID:   orderID,
		ImageUrl:  upload.URL,
		ImageType: string(model.ImageReceipt),
		Status:    string(model.ImageTemporary),
		SortOrder: -1, // 由 5.6 完工接口统一设置
		FileSize:  int(size),
	}
	if err := s.images.Create(ctx, img); err != nil {
		return nil, s.dbErr("create receipt record failed", err)
	}

	if err := s.appendAdminTimeline(ctx, order, op.UserID, string(model.ActionUploadReceipt), order.Status, order.Status, "", ip); err != nil {
		return nil, err
	}

	return &UploadImageResult{
		ID:        img.ID,
		URL:       img.ImageUrl,
		SortOrder: img.SortOrder,
		FileSize:  img.FileSize,
	}, nil
}

// ────────────────────────────────────────────
// 内部辅助
// ────────────────────────────────────────────

// setOrderMetadata 校验并写入 metadata JSONB (为空时填入 RepairMetadata 默认值)
func (s *AdminOrderService) setOrderMetadata(ctx context.Context, order *model.RepairOrder, meta model.RepairMetadata) error {
	raw, err := json.Marshal(meta)
	if err != nil {
		return apperrors.ErrInvalidParam.WithMessage("metadata 格式错误")
	}
	order.Metadata = raw
	return nil
}

// parseMetadata 解析 metadata JSONB, 解析失败返回零值
func parseMetadata(raw datatypes.JSON) model.RepairMetadata {
	meta := model.RepairMetadata{}
	if len(raw) > 0 {
		_ = json.Unmarshal(raw, &meta)
	}
	return meta
}

// appendChange 往 diff 片段追加 "label: old -> new"
func appendChange(diff, label, old, new string) string {
	if old == new {
		return diff
	}
	fragment := fmt.Sprintf("%s: %s -> %s", label, old, new)
	if diff == "" {
		return fragment
	}
	return diff + "; " + fragment
}

// appendChangeInt 追加整型差异
func appendChangeInt(diff, label string, old, new int) string {
	return appendChange(diff, label, fmt.Sprintf("%d", old), fmt.Sprintf("%d", new))
}

// appendChangePrice 追加价格差异（保留两位小数）
func appendChangePrice(diff, label string, old, new float64) string {
	return appendChange(diff, label, fmt.Sprintf("%.2f", old), fmt.Sprintf("%.2f", new))
}

// financeIncrementDiff 生成修改对账信息的增量 diff 文案, 未变化字段跳过
func financeIncrementDiff(
	beforeQuantity int, beforeUnitPrice float64, beforeRepairContent string,
	oldMeta model.RepairMetadata, newOrder *model.RepairOrder,
) string {
	newMeta := parseMetadata(newOrder.Metadata)
	diff := ""
	diff = appendChangeInt(diff, "数量", beforeQuantity, newOrder.Quantity)
	diff = appendChangePrice(diff, "单价", beforeUnitPrice, newOrder.UnitPrice)
	diff = appendChange(diff, "维修内容", beforeRepairContent, newOrder.RepairContent)
	diff = appendChange(diff, "维修结果", oldMeta.RepairResult, newMeta.RepairResult)
	diff = appendChange(diff, "维修方式", oldMeta.RepairMethod, newMeta.RepairMethod)
	diff = appendChange(diff, "保修期", oldMeta.WarrantyPeriod, newMeta.WarrantyPeriod)
	diff = appendChange(diff, "额外备注", oldMeta.ExtraRemark, newMeta.ExtraRemark)
	diff = appendChangeInt(diff, "维修时长(分钟)", oldMeta.RepairDuration, newMeta.RepairDuration)
	return diff
}

// loadOrder 查询工单, 不存在返回错误
func (s *AdminOrderService) loadOrder(ctx context.Context, orderID string) (*model.RepairOrder, error) {
	order, err := s.orders.FindByID(ctx, orderID)
	if err != nil {
		return nil, s.dbErr("find order failed", err)
	}
	if order == nil {
		return nil, apperrors.ErrOrderNotFound
	}
	return order, nil
}

// replaceImages 图片全量替换 (status 流程, 复用于收据图)
func (s *AdminOrderService) replaceImages(ctx context.Context, orderID, imageType string, urls []string) error {
	if err := s.images.MarkAllDeleted(ctx, orderID, imageType); err != nil {
		return s.dbErr("mark images deleted failed", err)
	}

	for idx, raw := range urls {
		u := strings.TrimSpace(raw)
		if u == "" {
			continue
		}
		img, err := s.images.FindByOrderAndURL(ctx, orderID, u)
		if err != nil {
			return s.dbErr("find image failed", err)
		}
		if img == nil {
			if err := s.images.Create(ctx, &model.OrderImage{
				ID:        uuid.New().String(),
				OrderID:   orderID,
				ImageUrl:  u,
				ImageType: imageType,
				Status:    string(model.ImageActive),
				SortOrder: idx,
			}); err != nil {
				return s.dbErr("create image failed", err)
			}
			continue
		}
		if img.Status != string(model.ImageActive) || img.SortOrder != idx {
			img.Status = string(model.ImageActive)
			img.SortOrder = idx
			if err := s.images.Update(ctx, img); err != nil {
				return s.dbErr("update image failed", err)
			}
		}
	}
	return nil
}

// appendAdminTimeline 追加管理端操作时间轴
func (s *AdminOrderService) appendAdminTimeline(ctx context.Context, order *model.RepairOrder, operatorID, action, from, to, remark, ip string) error {
	err := s.timelines.Create(ctx, &model.OrderTimeline{
		ID:         uuid.New().String(),
		OrderID:    order.ID,
		OrderNo:    order.OrderNo,
		OperatorID: operatorID,
		Action:     action,
		FromStatus: from,
		ToStatus:   to,
		Remark:     remark,
		IpAddress:  ip,
	})
	if err != nil {
		return s.dbErr("create timeline failed", err)
	}
	return nil
}

// adminAvailableActions 按状态返回管理端可执行动作 (5.2)
// ownerOnlyActions 仅接单人本人（或超级管理员）可执行的动作；
// 维修业务员（role=1，非超管）面对"他人接单"的工单时，这些动作从 available_actions 中移除，避免前端展示不可用按钮。
var ownerOnlyActions = map[string]bool{
	string(model.ActionComplete):      true,
	string(model.ActionUpdateFinance): true,
	string(model.ActionReopen):        true,
	string(model.ActionUploadReceipt): true,
	string(model.ActionReject):        true,
}

// statusSliceContains 状态筛选列表是否包含指定状态
func statusSliceContains(list []string, target string) bool {
	for _, s := range list {
		if s == target {
			return true
		}
	}
	return false
}

// filterOwnerActions 维修业务员（非超管）对他人接单的工单不展示"接单后动作"
func filterOwnerActions(op Operator, repairerID *string, actions []AdminAction) []AdminAction {
	if !op.IsPlainRepairer() || repairerID == nil || *repairerID == op.UserID {
		return actions
	}
	out := make([]AdminAction, 0, len(actions))
	for _, a := range actions {
		if ownerOnlyActions[a.Action] {
			continue
		}
		out = append(out, a)
	}
	return out
}

// ensureOrderOwner 校验接单人归属：超级管理员不受限；
// 已接单（repairer_id 非空）的工单，维修业务员仅可操作自己接的单（未接单时无归属，不拦截）。
func ensureOrderOwner(op Operator, order *model.RepairOrder) error {
	if op.IsSuperAdmin() || order.RepairerID == nil {
		return nil
	}
	if *order.RepairerID != op.UserID {
		return apperrors.ErrForbidden.WithMessage("仅接单的维修业务员本人或超级管理员可操作该工单")
	}
	return nil
}

func adminAvailableActions(status string) []AdminAction {
	switch model.OrderStatus(status) {
	case model.OrderReported:
		return []AdminAction{
			{Action: string(model.ActionAudit), Label: "审核通过", ToStatus: string(model.OrderPendingAccept), ConfirmMessage: "审核通过后工单进入待接单并上报维修业务员，确认？"},
			{Action: string(model.ActionReject), Label: "退回", ToStatus: string(model.OrderRejected), RequireReason: true, ReasonMinLength: 10, ConfirmMessage: "退回后工单进入「已退回」，报修人可修改重新提交，确认退回？"},
		}
	case model.OrderPendingAccept:
		return []AdminAction{
			{Action: string(model.ActionAccept), Label: "接单维修", ToStatus: string(model.OrderProcessing), ConfirmMessage: "确认接单维修该工单？"},
			{Action: string(model.ActionReject), Label: "退回", ToStatus: string(model.OrderRejected), RequireReason: true, ReasonMinLength: 10, ConfirmMessage: "退回后工单进入「已退回」，报修人可修改重新提交，确认退回？"},
		}
	case model.OrderProcessing:
		return []AdminAction{
			{Action: string(model.ActionComplete), Label: "完工", ToStatus: string(model.OrderCompleted), ConfirmMessage: "确认完工该工单？"},
			{Action: string(model.ActionReject), Label: "退回", ToStatus: string(model.OrderRejected), RequireReason: true, ReasonMinLength: 10, ConfirmMessage: "退回后工单进入「已退回」，报修人可修改重新提交，确认退回？"},
		}
	case model.OrderCompleted:
		return []AdminAction{
			{Action: string(model.ActionReopen), Label: "重新打开", ToStatus: string(model.OrderProcessing), ConfirmMessage: "重新打开后工单回到处理中，可再次完工登记，确认？"},
			{Action: string(model.ActionUpdateFinance), Label: "修改对账信息", ToStatus: string(model.OrderCompleted), ConfirmMessage: "确认修改该工单的对账信息？"},
		}
	default:
		return []AdminAction{}
	}
}

// buildAdminTimelineItems 组装管理端时间轴 (含操作人角色与 IP)
func buildAdminTimelineItems(timelines []model.OrderTimeline) []AdminTimelineItem {
	items := make([]AdminTimelineItem, 0, len(timelines))
	for _, tl := range timelines {
		items = append(items, AdminTimelineItem{
			ID:           tl.ID,
			Action:       tl.Action,
			ActionLabel:  actionLabels[tl.Action],
			OperatorName: tl.Operator.Nickname,
			OperatorRole: operatorRoleName(tl.Operator.Role, tl.Action),
			FromStatus:   strPtr(tl.FromStatus),
			ToStatus:     strPtr(tl.ToStatus),
			Remark:       strPtr(tl.Remark),
			IpAddress:    tl.IpAddress,
			CreatedAt:    tl.CreatedAt,
		})
	}
	return items
}

// strPtr 空字符串转 nil 指针 (时间轴可空字段)
func strPtr(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

// operatorRoleName 操作人角色展示名
// 平台角色 >=2=超级管理员, >=1=维修业务员; audit 动作且为普通用户时为单位审核员; 其余为报修人。
func operatorRoleName(role int, action string) string {
	if role >= model.PlatformRoleSuper {
		return "超级管理员"
	}
	if role >= model.PlatformRoleRepairer {
		return "维修业务员"
	}
	if action == string(model.ActionAudit) || action == string(model.ActionReject) {
		return "单位审核员"
	}
	return "报修人"
}

// dbErr 数据库错误包装
func (s *AdminOrderService) dbErr(msg string, err error) error {
	s.logger.Error(msg, zap.Error(err))
	return apperrors.ErrDatabaseError.WithError(err)
}
