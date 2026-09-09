// OrderService 报修工单业务逻辑 (用户端 4.1-4.9)。
//
// 按《后端接口设计文档 v1.1》/《数据库字段设计文档 V1.4》对齐:
//   - 4.1 options 返回项目字典树 (project_categories → properties/problems, 来自数据库)
//   - 4.3 更新草稿 (category_id/property_id + 名称快照自动回填; 常见问题仅预填描述不落单)
//   - 4.4 提交上报 (严格校验; 提交时生成工单号 XNB-{YYYYMMDD}-{3位序号}, 通知本单位审核员/维修业务员)
//   - 4.8 取消仅限 draft/reported/pending_accept
package service

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"path/filepath"
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

// maxImageSize 图片大小上限 5MB
const maxImageSize = 5 << 20

// maxFaultImages 同一工单未删除 (非 deleted) 故障图上限
const maxFaultImages = 9

// maxDraftCount 用户草稿总数上限 (4.2)
const maxDraftCount = 5

// ────────────────────────────────────────────
// 展示文案映射 (包级, 管理端共用)
// ────────────────────────────────────────────

var urgencyLabels = map[string]string{
	"normal":      "普通",
	"urgent":      "紧急",
	"very_urgent": "非常紧急",
}

var statusLabels = map[string]string{
	"draft":          "草稿",
	"reported":       "已上报",
	"pending_accept": "待接单",
	"processing":     "处理中",
	"rejected":       "已退回",
	"completed":      "已处理",
	"cancelled":      "已取消",
}

var actionLabels = map[string]string{
	"create_draft":   "创建草稿",
	"submit":         "提交上报",
	"audit":          "审核通过",
	"accept":         "接单维修",
	"complete":       "完工",
	"reopen":         "重新打开",
	"reject":         "退回",
	"cancel":         "取消",
	"upload_receipt": "上传收据",
	"update_finance": "修改对账信息",
}

// ────────────────────────────────────────────
// 输出结构 (4.1-4.9)
// ────────────────────────────────────────────

// CategoryOption 项目大类选项 (4.1; V1.4 修订: 常见问题随属性返回)
type CategoryOption struct {
	ID          string           `json:"id"`
	Name        string           `json:"name"`
	Description string           `json:"description"`
	SortOrder   int              `json:"sort_order"`
	Properties  []PropertyOption `json:"properties"`
}

// PropertyOption 项目属性选项 (4.1; 含其下常见问题)
type PropertyOption struct {
	ID       string          `json:"id"`
	Name     string          `json:"name"`
	Problems []ProblemOption `json:"problems,omitempty"`
}

// ProblemOption 常见问题选项 (4.1, 选中后预填描述)
type ProblemOption struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
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
	Categories   []CategoryOption   `json:"categories"`
	UrgentLevels []ValueLabel       `json:"urgent_levels"`
	Enterprises  []EnterpriseOption `json:"enterprises"`
}

// UpdateOrderInput 更新草稿入参 (4.3, 所有字段可选)
type UpdateOrderInput struct {
	EnterpriseID *string   `json:"enterprise_id"` // 报修单位 id (草稿可为空, 提交时必填)
	CategoryID   *string   `json:"category_id"`   // 项目大类 ID
	PropertyID   *string   `json:"property_id"`   // 项目属性 ID (须属于所选大类)
	Description  *string   `json:"description"`   // 报修描述 (可含常见问题预填文本)
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
	OrderNo     *string   `json:"order_no"` // 提交时生成 XNB-{YYYYMMDD}-{3位序号}
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
	CategoryID     string     `json:"category_id"`
	CategoryName   string     `json:"category_name"`
	PropertyID     string     `json:"property_id"`
	PropertyName   string     `json:"property_name"`
	Description    string     `json:"description"`
	EnterpriseID   string     `json:"enterprise_id"`
	EnterpriseName string     `json:"enterprise_name"`
	Urgency        string     `json:"urgency"`
	UrgencyLabel   string     `json:"urgency_label"`
	Status         string     `json:"status"`
	StatusLabel    string     `json:"status_label"`
	CreatedAt      time.Time  `json:"created_at"`
	SubmittedAt    *time.Time `json:"submitted_at"`
	/* ── V1.3 统一工单卡片所需字段（与 AdminOrderItem 对齐，见开发文档 V1.3 5.4） ── */
	/** 报修位置/房间号 */
	Room string `json:"room"`
	/** 联系人及电话（"王五 12345678910"，待后端拆分为 contact_name/contact_phone） */
	Contact string      `json:"contact"`
	Images  []ImageItem `json:"images"`
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

// OrderReporter 报修人摘要 (4.7)
type OrderReporter struct {
	ID        string `json:"id"`
	Nickname  string `json:"nickname"`
	AvatarURL string `json:"avatar_url"`
}

// OrderDetail 工单详情 (4.7)
type OrderDetail struct {
	ID               string                `json:"id"`
	OrderNo          *string               `json:"order_no"`
	EnterpriseID     string                `json:"enterprise_id"`
	EnterpriseName   string                `json:"enterprise_name"`
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
	Reporter         OrderReporter         `json:"reporter"`
	RepairContent    string                `json:"repair_content"`
	Quantity         int                   `json:"quantity"`
	UnitPrice        float64               `json:"unit_price"`
	Amount           float64               `json:"amount"`
	Metadata         *model.RepairMetadata `json:"metadata,omitempty"`
	AuditorName      string                `json:"auditor_name,omitempty"`
	RepairerName     string                `json:"repairer_name,omitempty"`
	Images           []ImageItem           `json:"images"`
	Receipts         []ImageItem           `json:"receipts"`
	Timeline         []TimelineItem        `json:"timeline"`
	AvailableActions []string              `json:"available_actions"`
	CreatedAt        time.Time             `json:"created_at"`
	SubmittedAt      *time.Time            `json:"submitted_at"`
	UpdatedAt        time.Time             `json:"updated_at"`
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
	projects  *repository.ProjectRepository
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
	projects *repository.ProjectRepository,
	imagebed *imagebed.Client,
	notifier *OrderNotifier,
	logger *zap.Logger,
) *OrderService {
	return &OrderService{
		orders:    orders,
		images:    images,
		timelines: timelines,
		mems:      mems,
		projects:  projects,
		imagebed:  imagebed,
		notifier:  notifier,
		logger:    logger,
	}
}

// Options 查询新建工单可选枚举 (4.1): 项目字典树 + 紧急程度 + 可上报单位
func (s *OrderService) Options(ctx context.Context, userID string) (*OrderOptions, error) {
	memberships, err := s.mems.FindApprovedByUser(ctx, userID)
	if err != nil {
		return nil, s.dbErr("find approved memberships failed", err)
	}

	cats, err := s.projects.ListCategoriesWithChildren(ctx)
	if err != nil {
		return nil, s.dbErr("list project categories failed", err)
	}

	enterprises := make([]EnterpriseOption, 0, len(memberships))
	for _, m := range memberships {
		enterprises = append(enterprises, EnterpriseOption{ID: m.EnterpriseID, Name: m.Enterprise.Name})
	}

	categoryOptions := make([]CategoryOption, 0, len(cats))
	for _, c := range cats {
		props := make([]PropertyOption, 0, len(c.Properties))
		for _, p := range c.Properties {
			problems := make([]ProblemOption, 0, len(p.Problems))
			for _, q := range p.Problems {
				problems = append(problems, ProblemOption{
					ID:          q.ID,
					Name:        q.Name,
					Description: q.Description,
				})
			}
			props = append(props, PropertyOption{ID: p.ID, Name: p.Name, Problems: problems})
		}
		opt := CategoryOption{
			ID:          c.ID,
			Name:        c.Name,
			Description: c.Description,
			SortOrder:   c.SortOrder,
			Properties:  props,
		}
		categoryOptions = append(categoryOptions, opt)
	}

	return &OrderOptions{
		Categories: categoryOptions,
		UrgentLevels: []ValueLabel{
			{Value: string(model.UrgencyNormal), Label: "普通"},
			{Value: string(model.UrgencyUrgent), Label: "紧急"},
			{Value: string(model.UrgencyVeryUrgent), Label: "非常紧急"},
		},
		Enterprises: enterprises,
	}, nil
}

// Create 创建空草稿 (4.2)
//
// 请求体为空。创建的空草稿: order_no 为空、enterprise_id/category_id/property_id 为空,
// 企业归属与项目大类/属性在 4.3 更新草稿时设置。
func (s *OrderService) Create(ctx context.Context, userID string) (*OrderDraftResult, error) {
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
// 所有字段可选; category_id/property_id 名称快照由后端自动回填;
// 常见问题不作为工单字段 (前端选中后预填 description);
// images 传完整列表, 按 status 全量替换。
func (s *OrderService) Update(ctx context.Context, userID, orderID string, in UpdateOrderInput) (*OrderDraftResult, error) {
	order, err := s.loadOwnedEditable(ctx, userID, orderID)
	if err != nil {
		return nil, err
	}

	if in.EnterpriseID != nil {
		v := strings.TrimSpace(*in.EnterpriseID)
		if v == "" {
			return nil, apperrors.ErrInvalidParam.WithMessage("enterprise_id 不能为空")
		}
		if err := s.verifyMembership(ctx, v, userID); err != nil {
			return nil, err
		}
		order.EnterpriseID = &v
	}

	// 大类变更: 校验有效 → 回填名称快照; 大类变了旧属性作废需重选
	if in.CategoryID != nil {
		v := strings.TrimSpace(*in.CategoryID)
		cat, err := s.projects.FindActiveCategoryByID(ctx, v)
		if err != nil {
			return nil, s.dbErr("find category failed", err)
		}
		if cat == nil {
			return nil, apperrors.ErrInvalidParam.WithMessage("项目大类不存在或已删除")
		}
		order.CategoryID = &v
		order.CategoryName = cat.Name
		// 原属性不属于新大类时清空, 提示重新选择属性
		if order.PropertyID != nil {
			prop, err := s.projects.FindActivePropertyByID(ctx, *order.PropertyID)
			if err != nil {
				return nil, s.dbErr("find property failed", err)
			}
			if prop == nil || prop.CategoryID != v {
				order.PropertyID = nil
				order.PropertyName = ""
			}
		}
	}

	// 属性变更: 校验有效 + 必须属于当前(或本次)大类
	if in.PropertyID != nil {
		v := strings.TrimSpace(*in.PropertyID)
		prop, err := s.projects.FindActivePropertyByID(ctx, v)
		if err != nil {
			return nil, s.dbErr("find property failed", err)
		}
		if prop == nil {
			return nil, apperrors.ErrInvalidParam.WithMessage("项目属性不存在或已删除")
		}
		catID := ""
		if order.CategoryID != nil {
			catID = *order.CategoryID
		}
		if catID == "" || prop.CategoryID != catID {
			return nil, apperrors.ErrInvalidParam.WithMessage("project_property 必须属于所选项目大类，请先选择大类")
		}
		order.PropertyID = &v
		order.PropertyName = prop.Name
	}

	if in.Description != nil {
		v := strings.TrimSpace(*in.Description)
		if err := validateLength("description", v, 1, 500); err != nil {
			return nil, err
		}
		order.Description = v
	}
	if in.Urgency != nil {
		if _, ok := urgencyLabels[*in.Urgency]; !ok {
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
// 严格校验必填项完整性与字典有效性后置为 reported; 提交时生成工单号
// XNB-{YYYYMMDD}-{3位序号} 并写入 submitted_at; 时间轴 submit。
func (s *OrderService) Submit(ctx context.Context, userID, orderID string) (*SubmitResult, error) {
	order, err := s.loadOwnedEditable(ctx, userID, orderID)
	if err != nil {
		return nil, err
	}

	if err := s.validateSubmitFields(ctx, order); err != nil {
		return nil, err
	}
	if err := s.verifyMembership(ctx, enterpriseIDStr(order.EnterpriseID), userID); err != nil {
		return nil, err
	}

	now := time.Now()
	order.Status = string(model.OrderReported)
	order.SubmittedAt = &now

	orderNo, err := s.orders.GenerateOrderNo(ctx, now)
	if err != nil {
		return nil, s.dbErr("generate order no failed", err)
	}
	order.OrderNo = &orderNo

	if err := s.orders.Update(ctx, order); err != nil {
		return nil, s.dbErr("update order failed", err)
	}

	if err := s.appendTimeline(ctx, order, userID, string(model.ActionSubmit), order.Status, string(model.OrderReported), ""); err != nil {
		return nil, err
	}
	s.logger.Info("order submitted", zap.String("order_id", order.ID), zap.String("order_no", orderNo))

	// 新单提醒: 该单位单位审核员与业务范围内维修业务员 (WebSocket/站内暂未实现, 预留通知钩子)
	s.notifyReviewersOnSubmit(ctx, order)

	return &SubmitResult{
		ID:          order.ID,
		OrderNo:     order.OrderNo,
		Status:      order.Status,
		SubmittedAt: now,
	}, nil
}

// notifyReviewersOnSubmit 新单提醒钩子。
// v1.1 约定: 提交上报后本单位单位审核员与维修业务员收到新单提醒。
// WebSocket/站内通知中心接口尚未排期实现, 此处仅记录日志, 便于后续接入。
func (s *OrderService) notifyReviewersOnSubmit(ctx context.Context, order *model.RepairOrder) {
	s.logger.Info("notify reviewers on submit (hook)",
		zap.String("order_id", order.ID),
		zap.String("order_no", orderNo(order)))
}

// Delete 删除草稿 (4.5): 关联故障图软删除 (status=deleted) + 工单状态置为 cancelled (保留追溯)
func (s *OrderService) Delete(ctx context.Context, userID, orderID string) error {
	order, err := s.loadOwnedEditable(ctx, userID, orderID)
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
// status 支持逗号分隔的多状态筛选, 如 "reported,pending_accept,processing"
func (s *OrderService) List(ctx context.Context, userID, enterpriseID, status string, page, pageSize int) (*OrderListResult, error) {
	var statuses []string
	if status != "" {
		for _, v := range strings.Split(status, ",") {
			v = strings.TrimSpace(v)
			if v == "" {
				continue
			}
			if _, ok := statusLabels[v]; !ok {
				return nil, apperrors.ErrInvalidParam.WithMessage("status 取值: draft/reported/pending_accept/processing/rejected/completed/cancelled, 多个用逗号分隔")
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

	// V1.3: 统一卡片缩略图（故障图，active），批量查询避免 N+1
	orderIDs := make([]string, 0, len(orders))
	for _, o := range orders {
		orderIDs = append(orderIDs, o.ID)
	}
	imgList, err := s.images.ListActiveByOrders(ctx, orderIDs, string(model.ImageFault))
	if err != nil {
		return nil, s.dbErr("list images failed", err)
	}

	list := make([]OrderListItem, 0, len(orders))
	for _, o := range orders {
		list = append(list, OrderListItem{
			ID:             o.ID,
			OrderNo:        o.OrderNo,
			CategoryID:     nstr(o.CategoryID),
			CategoryName:   o.CategoryName,
			PropertyID:     nstr(o.PropertyID),
			PropertyName:   o.PropertyName,
			Description:    o.Description,
			EnterpriseID:   enterpriseIDStr(o.EnterpriseID),
			EnterpriseName: o.Enterprise.Name,
			Urgency:        o.Urgency,
			UrgencyLabel:   urgencyLabels[o.Urgency],
			Status:         o.Status,
			StatusLabel:    statusLabels[o.Status],
			CreatedAt:      o.CreatedAt,
			SubmittedAt:    o.SubmittedAt,
			Room:           o.Room,
			Contact:        o.Contact,
			Images:         buildImageItems(imgList[o.ID]),
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

// Detail 工单详情 (4.7): 仅报修人可查看
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
		Reporter:         OrderReporter{ID: order.Reporter.ID, Nickname: order.Reporter.Nickname, AvatarURL: order.Reporter.AvatarUrl},
		RepairContent:    order.RepairContent,
		Quantity:         order.Quantity,
		UnitPrice:        order.UnitPrice,
		Amount:           order.Amount,
		Metadata:         parseOrderMetadata(order.Metadata),
		AuditorName:      order.Auditor.Nickname,
		RepairerName:     order.Repairer.Nickname,
		Images:           buildImageItems(images),
		Receipts:         buildImageItems(receipts),
		Timeline:         buildTimelineItems(timelines),
		AvailableActions: availableActions(order.Status),
		CreatedAt:        order.CreatedAt,
		SubmittedAt:      order.SubmittedAt,
		UpdatedAt:        order.UpdatedAt,
	}, nil
}

// Cancel 取消工单 (4.8): 仅 draft/reported/pending_accept, 报修人可取消
func (s *OrderService) Cancel(ctx context.Context, userID, orderID, reason string) error {
	order, err := s.loadOwnedOrder(ctx, userID, orderID)
	if err != nil {
		return err
	}
	if !model.IsCancelableStatus(model.OrderStatus(order.Status)) {
		return apperrors.ErrOrderCannotEdit.WithMessage("仅 draft/reported/pending_accept 状态的工单可取消")
	}
	if err := validateLength("reason", reason, 0, 200); err != nil {
		return err
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
// sort_order 默认 -1 (由 4.3 更新草稿统一设置)。格式支持 jpg/png/webp (V1.4)。
func (s *OrderService) UploadImage(ctx context.Context, userID, orderID, filename string, size int64, content io.Reader, sortOrder int) (*UploadImageResult, error) {
	order, err := s.loadOwnedOrder(ctx, userID, orderID)
	if err != nil {
		return nil, err
	}
	if order.Status != string(model.OrderDraft) && order.Status != string(model.OrderRejected) {
		return nil, apperrors.ErrOrderCannotEdit
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

	count, err := s.images.CountNotDeleted(ctx, orderID, string(model.ImageFault))
	if err != nil {
		return nil, s.dbErr("count images failed", err)
	}
	if count >= maxFaultImages {
		return nil, apperrors.ErrImageTooMany
	}

	upload, err := s.imagebed.Upload(ctx, filename, io.MultiReader(bytes.NewReader(head), content))
	if err != nil {
		s.logger.Error("image bed upload failed", zap.Error(err))
		return nil, apperrors.ErrOSSUpload.WithError(err)
	}

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

// loadOwnedEditable 加载当前用户拥有的工单并校验为可编辑状态 (draft 或 rejected)。
// rejected 为「已退回」草稿：报修人需修改后重新提交，与 draft 同享编辑/提交/删除能力。
func (s *OrderService) loadOwnedEditable(ctx context.Context, userID, orderID string) (*model.RepairOrder, error) {
	order, err := s.loadOwnedOrder(ctx, userID, orderID)
	if err != nil {
		return nil, err
	}
	if order.Status != string(model.OrderDraft) && order.Status != string(model.OrderRejected) {
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

// verifyMembership 校验用户是该企业已审批成员 (单位审核员/普通成员均可)
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

// validateSubmitFields 4.4 提交校验: 必填字段完整性与字典有效性
func (s *OrderService) validateSubmitFields(ctx context.Context, order *model.RepairOrder) error {
	if enterpriseIDStr(order.EnterpriseID) == "" {
		return apperrors.ErrDraftNotSubmittable.WithMessage("缺少 enterprise_id，请先在草稿中设置报修单位")
	}
	if nstr(order.CategoryID) == "" {
		return apperrors.ErrDraftNotSubmittable.WithMessage("缺少 category_id（项目大类为必填项）")
	}
	if nstr(order.PropertyID) == "" {
		return apperrors.ErrDraftNotSubmittable.WithMessage("缺少 property_id（项目属性为必填项）")
	}
	// 字典有效性: 大类/属性需存在且属性必须属于该大类 (防止字典被删后仍可提交)
	cat, err := s.projects.FindActiveCategoryByID(ctx, *order.CategoryID)
	if err != nil {
		return s.dbErr("find category failed", err)
	}
	if cat == nil {
		return apperrors.ErrDraftNotSubmittable.WithMessage("所选项目大类不存在或已删除，请重新选择")
	}
	prop, err := s.projects.FindActivePropertyByID(ctx, *order.PropertyID)
	if err != nil {
		return s.dbErr("find property failed", err)
	}
	if prop == nil {
		return apperrors.ErrDraftNotSubmittable.WithMessage("所选项目属性不存在或已删除，请重新选择")
	}
	if prop.CategoryID != *order.CategoryID {
		return apperrors.ErrDraftNotSubmittable.WithMessage("所选项目属性不属于所选项目大类，请重新选择")
	}
	if err := validateLength("description", order.Description, 1, 500); err != nil {
		return apperrors.ErrDraftNotSubmittable.WithMessage(err.Error())
	}
	if _, ok := urgencyLabels[order.Urgency]; !ok {
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

// replaceImages 图片全量替换 (status 流程, 供 4.3 故障图 / 5.6 收据图共用)
func (s *OrderService) replaceImages(ctx context.Context, orderID, imageType string, urls []string) error {
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

// availableActions 按状态计算用户端可执行操作
func availableActions(status string) []string {
	switch model.OrderStatus(status) {
	case model.OrderDraft, model.OrderRejected:
		return []string{"submit", "cancel"}
	case model.OrderReported, model.OrderPendingAccept:
		return []string{"cancel"}
	default:
		return []string{}
	}
}

// nstr 解引用字符串指针, nil 返回空串
func nstr(p *string) string {
	if p == nil {
		return ""
	}
	return *p
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

// validImageFormat 校验图片格式 (后缀 + 魔数; 支持 jpg/png/webp)
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

// buildTimelineItems 组装时间轴列表 (用户端)
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

// parseOrderMetadata 解析 metadata JSONB 为 RepairMetadata 指针, 空/全空返回 nil
func parseOrderMetadata(raw datatypes.JSON) *model.RepairMetadata {
	if len(raw) == 0 {
		return nil
	}
	var meta model.RepairMetadata
	if err := json.Unmarshal(raw, &meta); err != nil {
		return nil
	}
	if meta.RepairResult == "" && meta.RepairMethod == "" && meta.WarrantyPeriod == "" && meta.ExtraRemark == "" && meta.RepairDuration == 0 {
		return nil
	}
	return &meta
}
