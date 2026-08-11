// AdminOrderService 管理后台工单处理业务逻辑 (第五章 5.1-5.7)。
//
// 鉴权由中间件 RequirePlatformAdmin 保证 (仅平台管理员)。
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
	ProjectName    string        `json:"project_name"`
	Description    string        `json:"description"`
	Urgency        string        `json:"urgency"`
	UrgencyLabel   string        `json:"urgency_label"`
	Status         string        `json:"status"`
	StatusLabel    string        `json:"status_label"`
	ImageCount     int64         `json:"image_count"`
	SubmittedAt    *time.Time    `json:"submitted_at"`
	CreatedAt      time.Time     `json:"created_at"`
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
	ID               string               `json:"id"`
	OrderNo          *string              `json:"order_no"`
	EnterpriseID     string               `json:"enterprise_id"`
	EnterpriseName   string               `json:"enterprise_name"`
	ProjectName      string               `json:"project_name"`
	Category         string               `json:"category"`
	CategoryLabel    string               `json:"category_label"`
	Property         string               `json:"property"`
	PropertyLabel    string               `json:"property_label"`
	Description      string               `json:"description"`
	Urgency          string               `json:"urgency"`
	UrgencyLabel     string               `json:"urgency_label"`
	Room             string               `json:"room"`
	Contact          string               `json:"contact"`
	Status           string               `json:"status"`
	StatusLabel      string               `json:"status_label"`
	RejectReason     string               `json:"reject_reason"`
	RepairContent    string               `json:"repair_content"`
	Quantity         int                  `json:"quantity"`
	UnitPrice        float64              `json:"unit_price"`
	Amount           float64              `json:"amount"`
	Metadata         model.RepairMetadata `json:"metadata"`
	Images           []ImageItem          `json:"images"`
	Receipts         []ImageItem          `json:"receipts"`
	Timeline         []AdminTimelineItem  `json:"timeline"`
	AvailableActions []AdminAction        `json:"available_actions"`
	CreatedAt        time.Time            `json:"created_at"`
	SubmittedAt      *time.Time           `json:"submitted_at"`
	ReviewedAt       *time.Time           `json:"reviewed_at,omitempty"`
	AcceptedAt       *time.Time           `json:"accepted_at,omitempty"`
	CompletedAt      *time.Time           `json:"completed_at,omitempty"`
	UpdatedAt        time.Time            `json:"updated_at"`
}

// ────────────────────────────────────────────
// Service
// ────────────────────────────────────────────

// AdminOrderService 管理后台工单处理逻辑
type AdminOrderService struct {
	orders    *repository.OrderRepository
	images    *repository.OrderImageRepository
	timelines *repository.OrderTimelineRepository
	imagebed  *imagebed.Client
	logger    *zap.Logger
}

// NewAdminOrderService 创建 AdminOrderService
func NewAdminOrderService(
	orders *repository.OrderRepository,
	images *repository.OrderImageRepository,
	timelines *repository.OrderTimelineRepository,
	imagebed *imagebed.Client,
	logger *zap.Logger,
) *AdminOrderService {
	return &AdminOrderService{
		orders:    orders,
		images:    images,
		timelines: timelines,
		imagebed:  imagebed,
		logger:    logger,
	}
}

// adminSortFields 5.1 工单列表允许的排序字段白名单 (与 repository.ListForAdmin 保持一致)
var adminSortFields = map[string]bool{
	"order_no":        true,
	"enterprise_name": true,
	"reporter":        true,
	"project_name":    true,
	"urgency":         true,
	"status":          true,
	"submitted_at":    true,
	"created_at":      true,
}

// ListRepairers 维修员(业务员)列表 (5.15): 平台管理员即维修员
func (s *AdminOrderService) ListRepairers(ctx context.Context) ([]model.User, error) {
	return s.orders.ListRepairers(ctx)
}

// ListOrders 工单列表 (5.1)
func (s *AdminOrderService) ListOrders(ctx context.Context, f repository.OrderAdminFilter, page, pageSize int) (*AdminOrderList, error) {
	for _, st := range f.Status {
		if _, ok := statusLabels[st]; !ok {
			return nil, apperrors.ErrInvalidParam.WithMessage("status 取值: draft/reported/reviewed/processing/completed/cancelled")
		}
	}
	if f.Urgency != "" {
		if _, ok := urgencyLabels[f.Urgency]; !ok {
			return nil, apperrors.ErrInvalidParam.WithMessage("urgency 取值: normal/urgent/very_urgent")
		}
	}
	if f.SortBy != "" && !adminSortFields[f.SortBy] {
		return nil, apperrors.ErrInvalidParam.WithMessage("sort_by 取值: order_no/enterprise_name/reporter/project_name/urgency/status/submitted_at/created_at")
	}
	if f.SortOrder != "" && !strings.EqualFold(f.SortOrder, "asc") && !strings.EqualFold(f.SortOrder, "desc") {
		return nil, apperrors.ErrInvalidParam.WithMessage("sort_order 取值: asc/desc")
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

	list := make([]AdminOrderItem, 0, len(orders))
	for _, o := range orders {
		list = append(list, AdminOrderItem{
			ID:             o.ID,
			OrderNo:        o.OrderNo,
			Reporter:       AdminReporter{ID: o.Reporter.ID, Nickname: o.Reporter.Nickname, AvatarURL: o.Reporter.AvatarUrl},
			EnterpriseID:   enterpriseIDStr(o.EnterpriseID),
			EnterpriseName: o.Enterprise.Name,
			ProjectName:    o.ProjectName,
			Description:    o.Description,
			Urgency:        o.Urgency,
			UrgencyLabel:   urgencyLabels[o.Urgency],
			Status:         o.Status,
			StatusLabel:    statusLabels[o.Status],
			ImageCount:     imgCounts[o.ID],
			SubmittedAt:    o.SubmittedAt,
			CreatedAt:      o.CreatedAt,
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

// Detail 工单详情 (5.2)
func (s *AdminOrderService) Detail(ctx context.Context, orderID string) (*AdminOrderDetail, error) {
	order, err := s.loadOrder(ctx, orderID)
	if err != nil {
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
		Quantity:         order.Quantity,
		UnitPrice:        order.UnitPrice,
		Amount:           order.Amount,
		Metadata:         parseOrderMetadata(order.Metadata),
		Images:           buildImageItems(images),
		Receipts:         buildImageItems(receipts),
		Timeline:         buildAdminTimelineItems(timelines),
		AvailableActions: adminAvailableActions(order.Status),
		CreatedAt:        order.CreatedAt,
		SubmittedAt:      order.SubmittedAt,
		ReviewedAt:       order.ReviewedAt,
		AcceptedAt:       order.AcceptedAt,
		CompletedAt:      order.CompletedAt,
		UpdatedAt:        order.UpdatedAt,
	}, nil
}

// Review 查阅工单 (5.3): reported → reviewed
func (s *AdminOrderService) Review(ctx context.Context, adminID, orderID, remark, ip string) error {
	if err := validateLength("remark", remark, 0, 100); err != nil {
		return err
	}
	order, err := s.loadOrder(ctx, orderID)
	if err != nil {
		return err
	}
	if order.Status != string(model.OrderReported) {
		return apperrors.ErrOrderCannotEdit.WithMessage("仅 reported 状态的工单可查阅")
	}

	now := time.Now()
	from := order.Status
	order.Status = string(model.OrderReviewed)
	order.ReviewedAt = &now
	if err := s.orders.Update(ctx, order); err != nil {
		return s.dbErr("update order failed", err)
	}
	return s.appendAdminTimeline(ctx, order, adminID, string(model.ActionReview), from, string(model.OrderReviewed), remark, ip)
}

// Accept 接单维修 (5.4): reviewed → processing
func (s *AdminOrderService) Accept(ctx context.Context, adminID, orderID, remark, ip string) error {
	if err := validateLength("remark", remark, 0, 100); err != nil {
		return err
	}
	order, err := s.loadOrder(ctx, orderID)
	if err != nil {
		return err
	}
	if order.Status != string(model.OrderReviewed) {
		return apperrors.ErrOrderCannotEdit.WithMessage("仅 reviewed 状态的工单可接单")
	}

	now := time.Now()
	from := order.Status
	order.Status = string(model.OrderProcessing)
	order.AcceptedAt = &now
	order.RepairerID = &adminID // 接单即绑定维修员 (5.4)
	if err := s.orders.Update(ctx, order); err != nil {
		return s.dbErr("update order failed", err)
	}
	return s.appendAdminTimeline(ctx, order, adminID, string(model.ActionAccept), from, string(model.OrderProcessing), remark, ip)
}

// Reject 退回工单 (5.5): reported/reviewed/processing → draft
func (s *AdminOrderService) Reject(ctx context.Context, adminID, orderID, reason, ip string) error {
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
	case string(model.OrderReported), string(model.OrderReviewed), string(model.OrderProcessing):
	default:
		return apperrors.ErrOrderCannotEdit.WithMessage("仅 reported/reviewed/processing 状态的工单可退回")
	}

	from := order.Status
	order.Status = string(model.OrderDraft)
	order.RejectReason = reason
	order.SubmittedAt = nil
	order.ReviewedAt = nil
	order.AcceptedAt = nil
	if err := s.orders.Update(ctx, order); err != nil {
		return s.dbErr("update order failed", err)
	}
	return s.appendAdminTimeline(ctx, order, adminID, string(model.ActionReject), from, string(model.OrderDraft), reason, ip)
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
func (s *AdminOrderService) Complete(ctx context.Context, adminID, orderID, ip string, in CompleteOrderInput) error {
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

	// 收据全量替换 (status 流程)
	if err := s.replaceImages(ctx, order.ID, string(model.ImageReceipt), in.Receipts); err != nil {
		return err
	}

	// TODO: 触发公众号模板消息通知报修人"维修已完成" (通知能力待接入)
	s.logger.Info("order completed",
		zap.String("order_id", order.ID), zap.String("operator_id", adminID))

	return s.appendAdminTimeline(ctx, order, adminID, string(model.ActionComplete), from, string(model.OrderCompleted), remark, ip)
}

// UpdateFinance 修改对账信息 (5.6.1): 仅 completed 状态, 至少传一项
func (s *AdminOrderService) UpdateFinance(ctx context.Context, adminID, orderID, ip string, in UpdateFinanceInput) error {
	order, err := s.loadOrder(ctx, orderID)
	if err != nil {
		return err
	}
	if order.Status != string(model.OrderCompleted) {
		return apperrors.ErrOrderCannotEdit.WithMessage("仅 completed 状态的工单可修改对账信息")
	}
	if in.Quantity == nil && in.UnitPrice == nil && in.RepairContent == nil && in.Metadata == nil {
		return apperrors.ErrInvalidParam.WithMessage("至少需要传入一个字段")
	}

	// 记录修改前快照, 用于计算差异
	before := &model.RepairOrder{
		Quantity:      order.Quantity,
		UnitPrice:     order.UnitPrice,
		RepairContent: order.RepairContent,
		Metadata:      order.Metadata,
	}

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

	// 时间轴差异日志: 只记录实际变化的字段
	diff := computeFinanceDiff(before, order)
	return s.appendAdminTimeline(ctx, order, adminID, string(model.ActionUpdateFinance), order.Status, order.Status, diff, ip)
}

// setOrderMetadata 校验并写入 metadata JSONB (为空时填入 RepairMetadata 默认值)
func (s *AdminOrderService) setOrderMetadata(ctx context.Context, order *model.RepairOrder, meta model.RepairMetadata) error {
	raw, err := json.Marshal(meta)
	if err != nil {
		return apperrors.ErrInvalidParam.WithMessage("metadata 格式错误")
	}
	order.Metadata = raw
	return nil
}

// parseOrderMetadata 将数据库 metadata JSONB 字段解析为 RepairMetadata (解析失败返回零值)
func parseOrderMetadata(raw datatypes.JSON) model.RepairMetadata {
	var meta model.RepairMetadata
	if len(raw) > 0 {
		_ = json.Unmarshal(raw, &meta)
	}
	return meta
}

// financeDiffText 对账字段摘要文本 (用于 5.6.1 时间轴差异日志)
func financeDiffText(order *model.RepairOrder) string {
	meta := model.RepairMetadata{}
	if len(order.Metadata) > 0 {
		_ = json.Unmarshal(order.Metadata, &meta)
	}
	return fmt.Sprintf("数量:%d 单价:%.2f 维修内容:%s 维修结果:%s 维修方式:%s 保修期:%s 额外备注:%s 维修时长:%d分钟",
		order.Quantity, order.UnitPrice, order.RepairContent,
		meta.RepairResult, meta.RepairMethod, meta.WarrantyPeriod, meta.ExtraRemark, meta.RepairDuration)
}

// computeFinanceDiff 计算修改前后的差异文本, 只记录有变化的字段 (5.6.1)
func computeFinanceDiff(before, after *model.RepairOrder) string {
	var parts []string

	if before.Quantity != after.Quantity {
		parts = append(parts, fmt.Sprintf("数量:%d→%d", before.Quantity, after.Quantity))
	}
	if before.UnitPrice != after.UnitPrice {
		parts = append(parts, fmt.Sprintf("单价:%.2f→%.2f", before.UnitPrice, after.UnitPrice))
	}
	if before.RepairContent != after.RepairContent {
		parts = append(parts, fmt.Sprintf("维修内容:%s→%s", before.RepairContent, after.RepairContent))
	}

	beforeMeta := parseOrderMetadata(before.Metadata)
	afterMeta := parseOrderMetadata(after.Metadata)

	if beforeMeta.RepairResult != afterMeta.RepairResult {
		parts = append(parts, fmt.Sprintf("维修结果:%s→%s", beforeMeta.RepairResult, afterMeta.RepairResult))
	}
	if beforeMeta.RepairMethod != afterMeta.RepairMethod {
		parts = append(parts, fmt.Sprintf("维修方式:%s→%s", beforeMeta.RepairMethod, afterMeta.RepairMethod))
	}
	if beforeMeta.WarrantyPeriod != afterMeta.WarrantyPeriod {
		parts = append(parts, fmt.Sprintf("保修期:%s→%s", beforeMeta.WarrantyPeriod, afterMeta.WarrantyPeriod))
	}
	if beforeMeta.ExtraRemark != afterMeta.ExtraRemark {
		parts = append(parts, fmt.Sprintf("额外备注:%s→%s", beforeMeta.ExtraRemark, afterMeta.ExtraRemark))
	}
	if beforeMeta.RepairDuration != afterMeta.RepairDuration {
		parts = append(parts, fmt.Sprintf("维修时长:%d→%d分钟", beforeMeta.RepairDuration, afterMeta.RepairDuration))
	}

	return strings.Join(parts, "; ")
}

// UploadReceipt 上传收据图片 (5.7): 仅 processing, status=temporary, sort_order=-1
func (s *AdminOrderService) UploadReceipt(ctx context.Context, adminID, orderID, filename string, size int64, content io.Reader, ip string) (*UploadImageResult, error) {
	order, err := s.loadOrder(ctx, orderID)
	if err != nil {
		return nil, err
	}
	if order.Status != string(model.OrderProcessing) {
		return nil, apperrors.ErrOrderCannotEdit.WithMessage("仅 processing 状态的工单可上传收据")
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

	// 数量限制: 同一工单未删除收据 ≤ 3
	count, err := s.images.CountNotDeleted(ctx, orderID, string(model.ImageReceipt))
	if err != nil {
		return nil, s.dbErr("count receipts failed", err)
	}
	if count >= maxReceiptImages {
		return nil, apperrors.ErrImageTooMany.WithMessage("收据最多 3 张")
	}

	// 图床上传
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

	if err := s.appendAdminTimeline(ctx, order, adminID, string(model.ActionUploadReceipt), order.Status, order.Status, "", ip); err != nil {
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
func adminAvailableActions(status string) []AdminAction {
	switch status {
	case string(model.OrderReported):
		return []AdminAction{
			{Action: "review", Label: "查阅", ToStatus: string(model.OrderReviewed), ConfirmMessage: "确认已查阅该工单？"},
			{Action: "reject", Label: "退回", ToStatus: string(model.OrderDraft), RequireReason: true, ReasonMinLength: 10, ConfirmMessage: "退回后报修人可修改重新提交，确认退回？"},
		}
	case string(model.OrderReviewed):
		return []AdminAction{
			{Action: "accept", Label: "接单维修", ToStatus: string(model.OrderProcessing), ConfirmMessage: "确认接单维修该工单？"},
			{Action: "reject", Label: "退回", ToStatus: string(model.OrderDraft), RequireReason: true, ReasonMinLength: 10, ConfirmMessage: "退回后报修人可修改重新提交，确认退回？"},
		}
	case string(model.OrderProcessing):
		return []AdminAction{
			{Action: "complete", Label: "完工", ToStatus: string(model.OrderCompleted), ConfirmMessage: "确认完工该工单？"},
			{Action: "reject", Label: "退回", ToStatus: string(model.OrderDraft), RequireReason: true, ReasonMinLength: 10, ConfirmMessage: "退回后报修人可修改重新提交，确认退回？"},
		}
	case string(model.OrderCompleted):
		return []AdminAction{
			// 已完工工单仍可修改对账信息, 状态保持 completed 不变
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
			OperatorRole: operatorRoleName(tl.Operator.Role),
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

// operatorRoleName 操作人角色名 (1=管理员, 其余为报修人)
func operatorRoleName(role int) string {
	if role == 1 {
		return "管理员"
	}
	return "报修人"
}

// dbErr 数据库错误包装
func (s *AdminOrderService) dbErr(msg string, err error) error {
	s.logger.Error(msg, zap.Error(err))
	return apperrors.ErrDatabaseError.WithError(err)
}
