// OrderService 报修工单业务逻辑 (用户端 4.1-4.9)。
//
// 遵循《后端接口设计文档v1.0》第四章最新约定:
//   - 4.2 创建空草稿 (请求体为空, order_no/enterprise_id 均留空)
//   - 4.3 更新草稿 (enterprise_id 在此设置; images 用 status 全量替换)
//   - 4.4 提交上报 (严格校验必填; order_no 接单时生成, 此处保持空)
//   - 4.9 图片上传 (插入 status=temporary 记录)
package service

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"

	apperrors "xin-ni-repair/internal/errors"
	"xin-ni-repair/internal/model"
	"xin-ni-repair/internal/repository"
	"xin-ni-repair/pkg/imagebed"
)

// maxImageSize 图片大小上限 5MB
const maxImageSize = 5 << 20

// maxFaultImages 同一工单未删除 (非 deleted) 故障图上限
const maxFaultImages = 9

// maxDraftCount 用户草稿总数上限 (4.2)
const maxDraftCount = 5

// ────────────────────────────────────────────
// 静态枚举数据 (4.1)
// ────────────────────────────────────────────

var categoryLabels = map[string]string{
	"computer": "电脑维修",
	"network":  "网络故障",
	"printer":  "打印机",
	"other":    "其他设备",
}

var propertyLabels = map[string]string{
	"repair":   "维修",
	"purchase": "采购",
	"replace":  "更换",
	"warranty": "保修",
}

var urgencyLabels = map[string]string{
	"normal":      "普通",
	"urgent":      "紧急",
	"very_urgent": "非常紧急",
}

var statusLabels = map[string]string{
	"draft":      "草稿",
	"reported":   "已上报",
	"reviewed":   "已阅",
	"processing": "处理中",
	"completed":  "已处理",
	"cancelled":  "已取消",
}

var actionLabels = map[string]string{
	"create_draft":   "创建草稿",
	"submit":         "提交报修",
	"review":         "查阅",
	"accept":         "接单维修",
	"complete":       "完工",
	"reject":         "退回",
	"upload_receipt": "上传收据",
	"update_finance": "修改对账信息",
	"cancel":         "取消",
}

// OptionItem 项目大类
type OptionItem struct {
	ID         string   `json:"id"`
	Name       string   `json:"name"`
	Attributes []string `json:"attributes"`
}

// ValueLabel 键值对选项
type ValueLabel struct {
	Value string `json:"value"`
	Label string `json:"label"`
}

// EnterpriseOption 可上报企业
type EnterpriseOption struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// OrderOptions 新建工单可选枚举 (4.1)
type OrderOptions struct {
	ProjectCategories []OptionItem        `json:"project_categories"`
	Properties        []ValueLabel        `json:"properties"`
	CommonIssues      map[string][]string `json:"common_issues"`
	UrgentLevels      []ValueLabel        `json:"urgent_levels"`
	Enterprises       []EnterpriseOption  `json:"enterprises"`
}

// ────────────────────────────────────────────
// 输入 / 输出结构
// ────────────────────────────────────────────

// UpdateOrderInput 更新草稿入参 (4.3, 所有字段可选)
type UpdateOrderInput struct {
	EnterpriseID *string   `json:"enterprise_id"` // 在此设置企业归属 (首次设置时校验成员资格)
	ProjectName  *string   `json:"project_name"`
	Category     *string   `json:"category"`
	Property     *string   `json:"property"`
	Description  *string   `json:"description"`
	Urgency      *string   `json:"urgency"`
	Room         *string   `json:"room"`
	Contact      *string   `json:"contact"`
	Images       *[]string `json:"images"` // 完整的图片 URL 列表 (全量替换)
}

// OrderDraftResult 创建/更新草稿响应 (4.2/4.3)
type OrderDraftResult struct {
	OrderID   string    `json:"order_id"`
	Status    string    `json:"status"`
	CreatedAt time.Time `json:"created_at,omitempty"`
	UpdatedAt time.Time `json:"updated_at,omitempty"`
}

// SubmitResult 提交上报响应 (4.4)
type SubmitResult struct {
	ID          string    `json:"id"`
	OrderNo     *string   `json:"order_no"` // 接单时生成, 提交阶段仍为空
	Status      string    `json:"status"`
	SubmittedAt time.Time `json:"submitted_at"`
}

// Pagination 分页信息
type Pagination struct {
	Total      int64 `json:"total"`
	Page       int   `json:"page"`
	PageSize   int   `json:"page_size"`
	TotalPages int   `json:"total_pages"`
}

// OrderListItem 我的工单列表项 (4.6)
type OrderListItem struct {
	ID             string     `json:"id"`
	OrderNo        *string    `json:"order_no"`
	ProjectName    string     `json:"project_name"`
	Category       string     `json:"category"`
	CategoryLabel  string     `json:"category_label"`
	Description    string     `json:"description"`
	EnterpriseID   string     `json:"enterprise_id"`
	EnterpriseName string     `json:"enterprise_name"`
	Urgency        string     `json:"urgency"`
	UrgencyLabel   string     `json:"urgency_label"`
	Status         string     `json:"status"`
	StatusLabel    string     `json:"status_label"`
	CreatedAt      time.Time  `json:"created_at"`
	SubmittedAt    *time.Time `json:"submitted_at"`
}

// OrderListResult 我的工单列表 (4.6)
type OrderListResult struct {
	List       []OrderListItem `json:"list"`
	Pagination Pagination      `json:"pagination"`
}

// ImageItem 图片信息
type ImageItem struct {
	ID        string `json:"id"`
	URL       string `json:"url"`
	SortOrder int    `json:"sort_order"`
	FileSize  int    `json:"file_size"`
}

// TimelineItem 时间轴项
type TimelineItem struct {
	ID           string    `json:"id"`
	Action       string    `json:"action"`
	ActionLabel  string    `json:"action_label"`
	OperatorName string    `json:"operator_name"`
	FromStatus   string    `json:"from_status"`
	ToStatus     string    `json:"to_status"`
	Remark       string    `json:"remark"`
	CreatedAt    time.Time `json:"created_at"`
}

// OrderDetail 工单详情 (4.7)
type OrderDetail struct {
	ID               string         `json:"id"`
	OrderNo          *string        `json:"order_no"`
	EnterpriseID     string         `json:"enterprise_id"`
	EnterpriseName   string         `json:"enterprise_name"`
	ProjectName      string         `json:"project_name"`
	Category         string         `json:"category"`
	CategoryLabel    string         `json:"category_label"`
	Property         string         `json:"property"`
	PropertyLabel    string         `json:"property_label"`
	Description      string         `json:"description"`
	Urgency          string         `json:"urgency"`
	UrgencyLabel     string         `json:"urgency_label"`
	Room             string         `json:"room"`
	Contact          string         `json:"contact"`
	Status           string         `json:"status"`
	StatusLabel      string         `json:"status_label"`
	RejectReason     string         `json:"reject_reason"`
	RepairContent    string         `json:"repair_content"`
	Amount           float64        `json:"amount"`
	Images           []ImageItem    `json:"images"`
	Receipts         []ImageItem    `json:"receipts"`
	Timeline         []TimelineItem `json:"timeline"`
	AvailableActions []string       `json:"available_actions"`
	CreatedAt        time.Time      `json:"created_at"`
	SubmittedAt      *time.Time     `json:"submitted_at"`
	UpdatedAt        time.Time      `json:"updated_at"`
}

// UploadImageResult 图片上传响应 (4.9)
type UploadImageResult struct {
	ID        string `json:"id"`
	URL       string `json:"url"`
	SortOrder int    `json:"sort_order"`
	FileSize  int    `json:"file_size"`
}

// ────────────────────────────────────────────
// Service
// ────────────────────────────────────────────

// OrderService 工单业务逻辑
type OrderService struct {
	orders    *repository.OrderRepository
	images    *repository.OrderImageRepository
	timelines *repository.OrderTimelineRepository
	mems      *repository.MembershipRepository
	imagebed  *imagebed.Client
	notifier  *OrderNotifier
	logger    *zap.Logger
}

// NewOrderService 创建 OrderService
func NewOrderService(
	orders *repository.OrderRepository,
	images *repository.OrderImageRepository,
	timelines *repository.OrderTimelineRepository,
	mems *repository.MembershipRepository,
	imagebed *imagebed.Client,
	notifier *OrderNotifier,
	logger *zap.Logger,
) *OrderService {
	return &OrderService{
		orders:    orders,
		images:    images,
		timelines: timelines,
		mems:      mems,
		imagebed:  imagebed,
		notifier:  notifier,
		logger:    logger,
	}
}

// Options 查询新建工单可选枚举 (4.1)
func (s *OrderService) Options(ctx context.Context, userID string) (*OrderOptions, error) {
	memberships, err := s.mems.FindApprovedByUser(ctx, userID)
	if err != nil {
		return nil, s.dbErr("find approved memberships failed", err)
	}

	enterprises := make([]EnterpriseOption, 0, len(memberships))
	for _, m := range memberships {
		enterprises = append(enterprises, EnterpriseOption{ID: m.EnterpriseID, Name: m.Enterprise.Name})
	}

	return &OrderOptions{
		ProjectCategories: []OptionItem{
			{ID: "computer", Name: "电脑维修", Attributes: []string{}},
			{ID: "network", Name: "网络故障", Attributes: []string{}},
			{ID: "printer", Name: "打印机", Attributes: []string{}},
			{ID: "other", Name: "其他设备", Attributes: []string{}},
		},
		Properties: []ValueLabel{
			{Value: "repair", Label: "维修"},
			{Value: "purchase", Label: "采购"},
			{Value: "replace", Label: "更换"},
			{Value: "warranty", Label: "保修"},
		},
		CommonIssues: map[string][]string{
			"computer": {"无法开机", "蓝屏/死机", "显示器无信号", "电脑运行缓慢", "软件安装请求"},
			"network":  {"无法上网", "WiFi连接失败", "网速慢", "网络频繁断开"},
			"printer":  {"打印机卡纸", "打印空白", "无法识别墨盒", "打印模糊"},
			"other":    {"设备无法通电", "设备异响", "按键失灵", "其他"},
		},
		UrgentLevels: []ValueLabel{
			{Value: "normal", Label: "普通"},
			{Value: "urgent", Label: "紧急"},
			{Value: "very_urgent", Label: "非常紧急"},
		},
		Enterprises: enterprises,
	}, nil
}

// Create 创建空草稿 (4.2)
//
// 请求体为空。创建的空草稿: order_no 为空、enterprise_id 为空、其余字段为空,
// 企业归属在 4.3 更新草稿时设置。
func (s *OrderService) Create(ctx context.Context, userID string) (*OrderDraftResult, error) {
	// 草稿数限制: 用户全部 draft 工单不超过 5 个
	count, err := s.orders.CountDraftsByUser(ctx, userID)
	if err != nil {
		return nil, s.dbErr("count drafts failed", err)
	}
	if count >= maxDraftCount {
		return nil, apperrors.ErrDraftLimit
	}

	order := &model.RepairOrder{
		ID:         uuid.New().String(),
		ReporterID: userID,
		Status:     string(model.OrderDraft),
		// OrderNo / EnterpriseID 及业务字段留空
	}
	if err := s.orders.Create(ctx, order); err != nil {
		return nil, s.dbErr("create order failed", err)
	}
	s.logger.Info("order draft created", zap.String("order_id", order.ID))

	if err := s.timelines.Create(ctx, &model.OrderTimeline{
		ID:         uuid.New().String(),
		OrderID:    order.ID,
		OperatorID: userID,
		Action:     string(model.ActionCreateDraft),
		ToStatus:   string(model.OrderDraft),
	}); err != nil {
		return nil, s.dbErr("create timeline failed", err)
	}

	return &OrderDraftResult{
		OrderID:   order.ID,
		Status:    order.Status,
		CreatedAt: order.CreatedAt,
	}, nil
}

// Update 更新草稿 (4.3)
//
// 所有字段可选; enterprise_id 在此设置企业归属 (首次设置时校验成员资格);
// images 传完整列表, 按 status 全量替换。
func (s *OrderService) Update(ctx context.Context, userID, orderID string, in UpdateOrderInput) (*OrderDraftResult, error) {
	order, err := s.loadOwnedDraft(ctx, userID, orderID)
	if err != nil {
		return nil, err
	}

	if in.EnterpriseID != nil {
		v := strings.TrimSpace(*in.EnterpriseID)
		if err := s.verifyMembership(ctx, v, userID); err != nil {
			return nil, err
		}
		order.EnterpriseID = &v
	}
	if in.ProjectName != nil {
		v := strings.TrimSpace(*in.ProjectName)
		if err := validateLength("project_name", v, 1, 20); err != nil {
			return nil, err
		}
		order.ProjectName = v
	}
	if in.Category != nil {
		if !mapContains(categoryLabels, *in.Category) {
			return nil, apperrors.ErrInvalidParam.WithMessage("category 取值: computer/network/printer/other")
		}
		order.Category = *in.Category
	}
	if in.Property != nil {
		if !mapContains(propertyLabels, *in.Property) {
			return nil, apperrors.ErrInvalidParam.WithMessage("property 取值: repair/purchase/replace/warranty")
		}
		order.Property = *in.Property
	}
	if in.Description != nil {
		v := strings.TrimSpace(*in.Description)
		if err := validateLength("description", v, 1, 500); err != nil {
			return nil, err
		}
		order.Description = v
	}
	if in.Urgency != nil {
		if !mapContains(urgencyLabels, *in.Urgency) {
			return nil, apperrors.ErrInvalidParam.WithMessage("urgency 取值: normal/urgent/very_urgent")
		}
		order.Urgency = *in.Urgency
	}
	if in.Room != nil {
		v := strings.TrimSpace(*in.Room)
		if err := validateLength("room", v, 1, 20); err != nil {
			return nil, err
		}
		order.Room = v
	}
	if in.Contact != nil {
		if err := validateLength("contact", strings.TrimSpace(*in.Contact), 0, 40); err != nil {
			return nil, err
		}
		order.Contact = strings.TrimSpace(*in.Contact)
	}

	if err := s.orders.Update(ctx, order); err != nil {
		return nil, s.dbErr("update order failed", err)
	}

	// 图片全量替换 (status 流程)
	if in.Images != nil {
		if len(*in.Images) > maxFaultImages {
			return nil, apperrors.ErrImageTooMany
		}
		if err := s.replaceImages(ctx, order.ID, string(model.ImageFault), *in.Images); err != nil {
			return nil, err
		}
	}

	return &OrderDraftResult{
		OrderID:   order.ID,
		Status:    order.Status,
		UpdatedAt: order.UpdatedAt,
	}, nil
}

// Submit 提交上报 (4.4)
//
// 严格校验必填字段与成员资格后置为 reported; order_no 保持为空,
// 由接单时生成或手动填入 (4.2 约定)。
func (s *OrderService) Submit(ctx context.Context, userID, orderID string) (*SubmitResult, error) {
	order, err := s.loadOwnedDraft(ctx, userID, orderID)
	if err != nil {
		return nil, err
	}

	// 必填字段严格校验 (4.4 校验表)
	if err := s.validateSubmitFields(order); err != nil {
		return nil, err
	}
	// 企业成员资格校验
	if err := s.verifyMembership(ctx, enterpriseIDStr(order.EnterpriseID), userID); err != nil {
		return nil, err
	}

	now := time.Now()
	order.Status = string(model.OrderReported)
	order.SubmittedAt = &now

	// 提交时生成工单号: XNB-{YYYYMMDD}-{3位当日流水号} (4.4 业务规则)
	orderNo, err := s.orders.GenerateOrderNo(ctx, now)
	if err != nil {
		return nil, s.dbErr("generate order no failed", err)
	}
	order.OrderNo = &orderNo

	if err := s.orders.Update(ctx, order); err != nil {
		return nil, s.dbErr("update order failed", err)
	}

	if err := s.appendTimeline(ctx, order, userID, string(model.ActionSubmit), string(model.OrderDraft), string(model.OrderReported), ""); err != nil {
		return nil, err
	}
	s.logger.Info("order submitted", zap.String("order_id", order.ID), zap.String("order_no", orderNo))

	return &SubmitResult{
		ID:          order.ID,
		OrderNo:     order.OrderNo,
		Status:      order.Status,
		SubmittedAt: now,
	}, nil
}

// Delete 删除草稿 (4.5): 关联故障图软删除 (status=deleted) + 工单状态置为 cancelled
func (s *OrderService) Delete(ctx context.Context, userID, orderID string) error {
	order, err := s.loadOwnedDraft(ctx, userID, orderID)
	if err != nil {
		return err
	}

	if err := s.images.MarkAllDeleted(ctx, orderID, string(model.ImageFault)); err != nil {
		return s.dbErr("soft delete images failed", err)
	}
	order.Status = string(model.OrderCancelled)
	if err := s.orders.Update(ctx, order); err != nil {
		return s.dbErr("update order failed", err)
	}
	s.logger.Info("order draft deleted", zap.String("order_id", order.ID))
	return nil
}

// List 我的工单列表 (4.6)
// status 支持逗号分隔的多状态筛选, 如 "reported,reviewed,processing"
func (s *OrderService) List(ctx context.Context, userID, enterpriseID, status string, page, pageSize int) (*OrderListResult, error) {
	var statuses []string
	if status != "" {
		for _, v := range strings.Split(status, ",") {
			v = strings.TrimSpace(v)
			if v == "" {
				continue
			}
			if _, ok := statusLabels[v]; !ok {
				return nil, apperrors.ErrInvalidParam.WithMessage("status 取值: draft/reported/reviewed/processing/completed/cancelled, 多个用逗号分隔")
			}
			statuses = append(statuses, v)
		}
	}
	if enterpriseID != "" {
		if err := s.verifyMembership(ctx, enterpriseID, userID); err != nil {
			return nil, err
		}
	}

	orders, total, err := s.orders.ListByReporter(ctx, userID, enterpriseID, statuses, (page-1)*pageSize, pageSize)
	if err != nil {
		return nil, s.dbErr("list orders failed", err)
	}

	list := make([]OrderListItem, 0, len(orders))
	for _, o := range orders {
		list = append(list, OrderListItem{
			ID:             o.ID,
			OrderNo:        o.OrderNo,
			ProjectName:    o.ProjectName,
			Category:       o.Category,
			CategoryLabel:  categoryLabels[o.Category],
			Description:    o.Description,
			EnterpriseID:   enterpriseIDStr(o.EnterpriseID),
			EnterpriseName: o.Enterprise.Name,
			Urgency:        o.Urgency,
			UrgencyLabel:   urgencyLabels[o.Urgency],
			Status:         o.Status,
			StatusLabel:    statusLabels[o.Status],
			CreatedAt:      o.CreatedAt,
			SubmittedAt:    o.SubmittedAt,
		})
	}

	totalPages := int(total) / pageSize
	if int(total)%pageSize > 0 {
		totalPages++
	}
	return &OrderListResult{
		List: list,
		Pagination: Pagination{
			Total:      total,
			Page:       page,
			PageSize:   pageSize,
			TotalPages: totalPages,
		},
	}, nil
}

// Detail 工单详情 (4.7)
func (s *OrderService) Detail(ctx context.Context, userID, orderID string) (*OrderDetail, error) {
	order, err := s.orders.FindByID(ctx, orderID)
	if err != nil {
		return nil, s.dbErr("find order failed", err)
	}
	if order == nil {
		return nil, apperrors.ErrOrderNotFound
	}
	if order.ReporterID != userID {
		return nil, apperrors.ErrForbidden
	}

	// 仅展示 active 图片 (temporary 未确认, deleted 已删除, 均不展示)
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

	return &OrderDetail{
		ID:               order.ID,
		OrderNo:          order.OrderNo,
		EnterpriseID:     enterpriseIDStr(order.EnterpriseID),
		EnterpriseName:   order.Enterprise.Name,
		ProjectName:      order.ProjectName,
		Category:         order.Category,
		CategoryLabel:    categoryLabels[order.Category],
		Property:         order.Property,
		PropertyLabel:    propertyLabels[order.Property],
		Description:      order.Description,
		Urgency:          order.Urgency,
		UrgencyLabel:     urgencyLabels[order.Urgency],
		Room:             order.Room,
		Contact:          order.Contact,
		Status:           order.Status,
		StatusLabel:      statusLabels[order.Status],
		RejectReason:     order.RejectReason,
		RepairContent:    order.RepairContent,
		Amount:           order.Amount,
		Images:           buildImageItems(images),
		Receipts:         buildImageItems(receipts),
		Timeline:         buildTimelineItems(timelines),
		AvailableActions: availableActions(order.Status),
		CreatedAt:        order.CreatedAt,
		SubmittedAt:      order.SubmittedAt,
		UpdatedAt:        order.UpdatedAt,
	}, nil
}

// Cancel 取消工单 (4.8)
func (s *OrderService) Cancel(ctx context.Context, userID, orderID, reason string) error {
	order, err := s.loadOwnedOrder(ctx, userID, orderID)
	if err != nil {
		return err
	}
	switch order.Status {
	case string(model.OrderDraft), string(model.OrderReported), string(model.OrderReviewed):
	default:
		return apperrors.ErrOrderCannotEdit
	}

	from := order.Status
	order.Status = string(model.OrderCancelled)
	if err := s.orders.Update(ctx, order); err != nil {
		return s.dbErr("update order failed", err)
	}
	if err := s.appendTimeline(ctx, order, userID, string(model.ActionCancel), from, string(model.OrderCancelled), reason); err != nil {
		return err
	}
	s.logger.Info("order cancelled", zap.String("order_id", order.ID))
	return nil
}

// UploadImage 图片上传 (4.9)
//
// 上传成功后插入 order_images 记录: status=temporary, image_type=fault;
// sort_order 默认 -1 (由 4.3 更新草稿统一设置)。
func (s *OrderService) UploadImage(ctx context.Context, userID, orderID, filename string, size int64, content io.Reader, sortOrder int) (*UploadImageResult, error) {
	order, err := s.loadOwnedOrder(ctx, userID, orderID)
	if err != nil {
		return nil, err
	}
	if order.Status != string(model.OrderDraft) {
		return nil, apperrors.ErrOrderCannotEdit
	}

	// 大小与格式校验
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

	// 数量限制: 同一工单未删除 (status != deleted) 故障图 ≤ 9
	count, err := s.images.CountNotDeleted(ctx, orderID, string(model.ImageFault))
	if err != nil {
		return nil, s.dbErr("count images failed", err)
	}
	if count >= maxFaultImages {
		return nil, apperrors.ErrImageTooMany
	}

	// 图床上传
	upload, err := s.imagebed.Upload(ctx, filename, io.MultiReader(bytes.NewReader(head), content))
	if err != nil {
		s.logger.Error("image bed upload failed", zap.Error(err))
		return nil, apperrors.ErrOSSUpload.WithError(err)
	}

	// 排序值: 不传则默认 -1, 由更新草稿接口统一设置; 传入则使用传入值
	if sortOrder <= 0 {
		sortOrder = -1
	}

	img := &model.OrderImage{
		ID:        uuid.New().String(),
		OrderID:   orderID,
		ImageUrl:  upload.URL,
		ImageType: string(model.ImageFault),
		Status:    string(model.ImageTemporary),
		SortOrder: sortOrder,
		FileSize:  int(size),
	}
	if err := s.images.Create(ctx, img); err != nil {
		return nil, s.dbErr("create image record failed", err)
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

// loadOwnedDraft 加载当前用户拥有的工单并校验为 draft 状态
func (s *OrderService) loadOwnedDraft(ctx context.Context, userID, orderID string) (*model.RepairOrder, error) {
	order, err := s.loadOwnedOrder(ctx, userID, orderID)
	if err != nil {
		return nil, err
	}
	if order.Status != string(model.OrderDraft) {
		return nil, apperrors.ErrOrderCannotEdit
	}
	return order, nil
}

// loadOwnedOrder 查询当前用户拥有的工单
func (s *OrderService) loadOwnedOrder(ctx context.Context, userID, orderID string) (*model.RepairOrder, error) {
	order, err := s.orders.FindByID(ctx, orderID)
	if err != nil {
		return nil, s.dbErr("find order failed", err)
	}
	if order == nil {
		return nil, apperrors.ErrOrderNotFound
	}
	if order.ReporterID != userID {
		return nil, apperrors.ErrForbidden
	}
	return order, nil
}

// verifyMembership 校验用户是该企业已审批成员
func (s *OrderService) verifyMembership(ctx context.Context, enterpriseID, userID string) error {
	m, err := s.mems.FindByEnterpriseAndUser(ctx, enterpriseID, userID)
	if err != nil {
		return s.dbErr("find membership failed", err)
	}
	if m == nil || m.Status != string(model.MemberApproved) {
		return apperrors.ErrWrongEnterprise
	}
	return nil
}

// validateSubmitFields 4.4 提交校验: 必填字段完整性与枚举合法性
func (s *OrderService) validateSubmitFields(order *model.RepairOrder) error {
	if enterpriseIDStr(order.EnterpriseID) == "" {
		return apperrors.ErrDraftNotSubmittable.WithMessage("缺少 enterprise_id，请先在草稿中设置企业")
	}
	if err := validateLength("project_name", order.ProjectName, 1, 20); err != nil {
		return apperrors.ErrDraftNotSubmittable.WithMessage(err.Error())
	}
	if !mapContains(categoryLabels, order.Category) {
		return apperrors.ErrDraftNotSubmittable.WithMessage("category 必须为 computer/network/printer/other")
	}
	if !mapContains(propertyLabels, order.Property) {
		return apperrors.ErrDraftNotSubmittable.WithMessage("property 必须为 repair/purchase/replace/warranty")
	}
	if err := validateLength("description", order.Description, 1, 500); err != nil {
		return apperrors.ErrDraftNotSubmittable.WithMessage(err.Error())
	}
	if !mapContains(urgencyLabels, order.Urgency) {
		return apperrors.ErrDraftNotSubmittable.WithMessage("urgency 必须为 normal/urgent/very_urgent")
	}
	if err := validateLength("room", order.Room, 1, 20); err != nil {
		return apperrors.ErrDraftNotSubmittable.WithMessage(err.Error())
	}
	if err := validateLength("contact", order.Contact, 0, 40); err != nil {
		return apperrors.ErrDraftNotSubmittable.WithMessage(err.Error())
	}
	if order.Contact == "" {
		return apperrors.ErrDraftNotSubmittable.WithMessage("缺少 contact，联系人及电话为必填")
	}
	return nil
}

// replaceImages 图片全量替换 (status 流程, 供 4.3 故障图 / 5.6 收据图共用):
//  1. 将工单下指定类型的图全部标记为 deleted
//  2. 按传入列表顺序, 将对应 URL 行的 sort_order 与 status 置为 active (不存在则创建)
func (s *OrderService) replaceImages(ctx context.Context, orderID, imageType string, urls []string) error {
	// 1. 全部置 deleted
	if err := s.images.MarkAllDeleted(ctx, orderID, imageType); err != nil {
		return s.dbErr("mark images deleted failed", err)
	}

	// 2. 按列表激活
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

// appendTimeline 追加时间轴记录
func (s *OrderService) appendTimeline(ctx context.Context, order *model.RepairOrder, operatorID, action, from, to, remark string) error {
	err := s.timelines.Create(ctx, &model.OrderTimeline{
		ID:         uuid.New().String(),
		OrderID:    order.ID,
		OrderNo:    order.OrderNo,
		OperatorID: operatorID,
		Action:     action,
		FromStatus: from,
		ToStatus:   to,
		Remark:     remark,
	})
	if err != nil {
		return s.dbErr("create timeline failed", err)
	}
	return nil
}

// dbErr 数据库错误包装
func (s *OrderService) dbErr(msg string, err error) error {
	s.logger.Error(msg, zap.Error(err))
	return apperrors.ErrDatabaseError.WithError(err)
}

// mapContains 判断 key 是否存在于 map
func mapContains(m map[string]string, key string) bool {
	_, ok := m[key]
	return ok
}

// enterpriseIDStr 解引用企业 ID 指针, nil 时返回空串
func enterpriseIDStr(e *string) string {
	if e == nil {
		return ""
	}
	return *e
}

// validateLength 校验字符串长度 (min 为 0 表示允许为空)
func validateLength(field, v string, min, max int) error {
	n := len([]rune(v))
	if n < min || n > max {
		if min == 0 {
			return apperrors.ErrInvalidParam.WithMessage(fmt.Sprintf("%s 长度不能超过 %d 字符", field, max))
		}
		return apperrors.ErrInvalidParam.WithMessage(fmt.Sprintf("%s 需为 %d-%d 字符", field, min, max))
	}
	return nil
}

// availableActions 按状态计算用户端可执行操作
func availableActions(status string) []string {
	switch status {
	case string(model.OrderDraft):
		return []string{"submit", "cancel"}
	case string(model.OrderReported), string(model.OrderReviewed):
		return []string{"cancel"}
	default:
		return []string{}
	}
}

// validImageFormat 校验图片格式 (后缀 + 魔数)
func validImageFormat(head []byte, filename string) bool {
	switch strings.ToLower(filepath.Ext(filename)) {
	case ".jpg", ".jpeg", ".png", ".webp":
	default:
		return false
	}
	switch {
	case len(head) >= 3 && head[0] == 0xFF && head[1] == 0xD8 && head[2] == 0xFF:
		return true // JPEG
	case len(head) >= 8 && bytes.HasPrefix(head, []byte{0x89, 'P', 'N', 'G', 0x0D, 0x0A, 0x1A, 0x0A}):
		return true // PNG
	case len(head) >= 12 && bytes.HasPrefix(head, []byte("RIFF")) && bytes.HasPrefix(head[8:], []byte("WEBP")):
		return true // WEBP
	default:
		return false
	}
}

// buildImageItems 组装图片列表
func buildImageItems(images []model.OrderImage) []ImageItem {
	items := make([]ImageItem, 0, len(images))
	for _, img := range images {
		items = append(items, ImageItem{
			ID:        img.ID,
			URL:       img.ImageUrl,
			SortOrder: img.SortOrder,
			FileSize:  img.FileSize,
		})
	}
	return items
}

// buildTimelineItems 组装时间轴列表
func buildTimelineItems(timelines []model.OrderTimeline) []TimelineItem {
	items := make([]TimelineItem, 0, len(timelines))
	for _, tl := range timelines {
		items = append(items, TimelineItem{
			ID:           tl.ID,
			Action:       tl.Action,
			ActionLabel:  actionLabels[tl.Action],
			OperatorName: tl.Operator.Nickname,
			FromStatus:   tl.FromStatus,
			ToStatus:     tl.ToStatus,
			Remark:       tl.Remark,
			CreatedAt:    tl.CreatedAt,
		})
	}
	return items
}
