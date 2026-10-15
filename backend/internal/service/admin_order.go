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

// pendingSignFilterValue 工单列表「待补签」筛选伪状态值（非状态机状态，实为按签字状态 pending_sign 筛选）
const pendingSignFilterValue = "pending_sign"

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
	// V1.5 本质标签（后端派生，前端只渲染）
	DisplayTag      string `json:"display_tag"`
	DisplayTagType  string `json:"display_tag_type"`
	ReceiptUploaded bool   `json:"receipt_uploaded"`
	SignatureStatus string `json:"signature_status"`
	FinishOutcome   string `json:"finish_outcome"`
}

// AdminOrderList 管理端工单分页结果 (5.1)
type AdminOrderList struct {
	List       []AdminOrderItem `json:"list"`
	Total      int64            `json:"total"`
	Page       int              `json:"page"`
	PageSize   int              `json:"page_size"`
	TotalPages int              `json:"total_pages"`
	// StatusCounts 各状态实时计数（角标用）：key 为状态值（reported/pending_accept/...），
	// 另含聚合键 "unfinished"（待审核+待接单+处理中）。不含 f.Status 自身，角标互不影响。
	StatusCounts map[string]int64 `json:"status_counts"`
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
	Remark           string                `json:"remark"`
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
	// V1.5 本质标签（后端派生，前端只渲染）
	DisplayTag      string `json:"display_tag"`
	DisplayTagType  string `json:"display_tag_type"`
	ReceiptUploaded bool   `json:"receipt_uploaded"`
	SignatureStatus string `json:"signature_status"`
	FinishOutcome   string `json:"finish_outcome"`
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
	settings  *SettingsService
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
	settings *SettingsService,
	logger *zap.Logger,
) *AdminOrderService {
	return &AdminOrderService{
		orders:    orders,
		images:    images,
		timelines: timelines,
		access:    access,
		imagebed:  imagebed,
		notifier:  notifier,
		settings:  settings,
		logger:    logger,
	}
}

// RepairerWithScope 维修员列表项 (5.15)，V1.5 附带经营范围明细便于后台分配
type RepairerWithScope struct {
	ID         string `json:"id"`
	Nickname   string `json:"nickname"`
	AvatarURL  string `json:"avatar_url"`
	Role       int    `json:"role"`
	ScopeCount int64  `json:"scope_count"`
	// Enterprises 已分配企业明细：列表页需直接看到"这个人管哪几个企业"，
	// 只给数量无法判断分配是否正确
	Enterprises []ScopeEnterpriseItem `json:"enterprises"`
}

// ListRepairers 维修员(业务员)列表 (5.15): users.role>=1 即维修业务员/超管
func (s *AdminOrderService) ListRepairers(ctx context.Context) ([]RepairerWithScope, error) {
	users, err := s.orders.ListRepairers(ctx)
	if err != nil {
		return nil, s.dbErr("list repairers failed", err)
	}
	items, err := s.access.ScopeItemsByRepairer(ctx)
	if err != nil {
		return nil, err
	}
	list := make([]RepairerWithScope, 0, len(users))
	for _, u := range users {
		ent := items[u.ID]
		list = append(list, RepairerWithScope{
			ID:          u.ID,
			Nickname:    u.Nickname,
			AvatarURL:   u.AvatarUrl,
			Role:        u.Role,
			ScopeCount:  int64(len(ent)),
			Enterprises: ent,
		})
	}
	return list, nil
}

// ListOrders 工单列表 (5.1)
//
// 鉴权: 店方角色 (role>=1) 可查全部; 单位审核员 (role=0 + membership.role=1)
// 必须传 enterprise_id 且限定于其有权限的单位。
func (s *AdminOrderService) ListOrders(ctx context.Context, op Operator, f repository.OrderAdminFilter, page, pageSize int) (*AdminOrderList, error) {
	// 「待补签」是跨状态的派生筛选（按客户签字状态 = pending_sign），并非状态机状态。
	// 必须在 status 合法性校验之前抽离，否则会被当作非法状态拒绝。
	pendingSignFilter := false
	realStatuses := make([]string, 0, len(f.Status))
	for _, st := range f.Status {
		if st == pendingSignFilterValue {
			pendingSignFilter = true
			continue
		}
		realStatuses = append(realStatuses, st)
	}
	f.Status = realStatuses
	if pendingSignFilter {
		f.SignatureStatus = string(model.SignaturePendingSign)
	}

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
		// V1.7：board = 状态优先级 + 更新时间（首页看板与列表「全部」Tab 共用）
		case repository.SortBoard:
		default:
			return nil, apperrors.ErrInvalidParam.WithMessage("sort_by 取值: order_no/enterprise_name/reporter/category_name/urgency/status/submitted_at/created_at/board")
		}
	}
	if f.SortOrder != "" && !strings.EqualFold(f.SortOrder, "asc") && !strings.EqualFold(f.SortOrder, "desc") {
		return nil, apperrors.ErrInvalidParam.WithMessage("sort_order 取值: asc/desc")
	}

	// 单位审核员数据权限：
	// - 指定 enterprise_id → 校验为该单位审核员（仅看本单位）
	// - 未指定（选择「全部企业」）→ 按其所担任审核员的所有单位做 IN 收敛，不信任客户端传入清单
	if !op.IsStoreStaff() {
		if f.EnterpriseID != "" {
			if err := s.access.CanManageEnterprise(ctx, f.EnterpriseID, op.UserID, op.Role); err != nil {
				return nil, err
			}
		} else {
			ids, err := s.access.ReviewerEnterpriseIDs(ctx, op.UserID)
			if err != nil {
				return nil, err
			}
			// 空切片 → 无可看单位，applyScope 返回 visible=false，列表/计数返回空结果（不泄漏数据）
			f.ScopeLimited = true
			f.ScopeEnterpriseIDs = ids
		}
	}

	// V1.5 经营范围收敛：维修业务员仅可见指派范围内的客户企业数据。
	// OwnerOrRepairerID 保证已接工单在范围被撤销后仍可见（不卡死在处理中）。
	if err := s.access.ApplyOrderScope(ctx, op, &f); err != nil {
		return nil, err
	}
	if op.IsPlainRepairer() && f.EnterpriseID != "" {
		scope, err := s.access.OrderScope(ctx, op)
		if err != nil {
			return nil, err
		}
		if !scope.InScope(f.EnterpriseID) {
			return nil, apperrors.ErrForbidden.WithMessage("该企业不在您的服务范围内，请联系管理员分配")
		}
	}

	// 维修业务员（非超管）查看「处理中」时只能看到自己接的工单（repairer_id = 自己）。
	// 仅在把 processing 作为唯一筛选项时收敛：若调用方显式请求了包含 processing 的多个状态
	// （例如「未完成」= 待审核+待接单+处理中），此时再强制 repairer_id 会把其余状态的工单
	// 静默吞掉，导致聚合页看起来"少数据"。
	// 维修业务员查看「处理中」：仅当本请求已显式要求「我的」(repairer_id=me，handler 已翻译为 op.UserID)
	// 才收敛为本人接的单；审核员在 review 模式(未传 repairer_id)保持本单位全范围视角，不强行收窄。
	if op.IsPlainRepairer() && f.RepairerID == op.UserID && len(f.Status) == 1 && f.Status[0] == string(model.OrderProcessing) {
		f.RepairerID = op.UserID
	}

	// 状态 Tab 实时角标：按"其它筛选条件"分组计数（忽略当前 Tab 状态 + 忽略 processing 专属 repairer 特例）
	countFilter := f
	countFilter.Status = nil
	countFilter.RepairerID = ""
	countFilter.SignatureStatus = "" // 角标反映各状态自身数量，不受「待补签」派生筛选影响
	byStatusCounts, cErr := s.orders.CountByStatusForAdmin(ctx, countFilter)
	if cErr != nil {
		return nil, s.dbErr("count orders by status failed", cErr)
	}
	// V1.8：维修业务员「我的」视角(repairer_id=me)角标隔离：
	// processing / pending_sign 仅统计本人接的单，避免从全经营范围角标泄漏他人工单量；
	// 审核员 review 视角(未传 repairer_id)保持本单位全范围角标（不收敛）。
	if op.IsPlainRepairer() && f.RepairerID == op.UserID {
		meFilter := f
		meFilter.Status = []string{string(model.OrderProcessing)}
		meFilter.RepairerID = op.UserID
		meFilter.SignatureStatus = "" // pending_sign 派生计数按本人统计（跨 processing/completed）
		meCounts, mcErr := s.orders.CountByStatusForAdmin(ctx, meFilter)
		if mcErr != nil {
			return nil, s.dbErr("count repairer orders by status failed", mcErr)
		}
		byStatusCounts[string(model.OrderProcessing)] = meCounts[string(model.OrderProcessing)]
		byStatusCounts[pendingSignFilterValue] = meCounts[pendingSignFilterValue]
	}
	unfinishedCount := byStatusCounts[string(model.OrderReported)] +
		byStatusCounts[string(model.OrderPendingAccept)] +
		byStatusCounts[string(model.OrderProcessing)]
	// "all"：忽略状态筛选（保留其它筛选）的工单总数，供「全部」角标使用，
	// 避免「全部」角标随当前选中状态而变成与某状态相同的数字。
	allCount := int64(0)
	for k, c := range byStatusCounts {
		if k == pendingSignFilterValue { // 派生计数，不计入「全部」
			continue
		}
		allCount += c
	}
	statusCounts := map[string]int64{
		"all":                            allCount,
		string(model.OrderDraft):         byStatusCounts[string(model.OrderDraft)],
		string(model.OrderReported):      byStatusCounts[string(model.OrderReported)],
		string(model.OrderPendingAccept): byStatusCounts[string(model.OrderPendingAccept)],
		string(model.OrderProcessing):    byStatusCounts[string(model.OrderProcessing)],
		string(model.OrderCompleted):     byStatusCounts[string(model.OrderCompleted)],
		string(model.OrderCancelled):     byStatusCounts[string(model.OrderCancelled)],
		string(model.OrderRejected):      byStatusCounts[string(model.OrderRejected)],
		"unfinished":                     unfinishedCount,
		"pending_sign":                   byStatusCounts[pendingSignFilterValue],
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
	receiptCounts, err := s.images.CountActiveByOrders(ctx, orderIDs, string(model.ImageReceipt))
	if err != nil {
		return nil, s.dbErr("count receipts failed", err)
	}

	list := make([]AdminOrderItem, 0, len(orders))
	for _, o := range orders {
		receiptUploaded := receiptCounts[o.ID] > 0
		tag, tagType := ComputeDisplayTag(&o, receiptUploaded)
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
			AvailableActions: filterOwnerActions(op, o.RepairerID, adminAvailableActions(o.Status, o.SignatureStatus)),
			DisplayTag:       tag,
			DisplayTagType:   tagType,
			ReceiptUploaded:  receiptUploaded,
			SignatureStatus:  o.SignatureStatus,
			FinishOutcome:    o.FinishOutcome,
		})
	}

	totalPages := int(total) / pageSize
	if int(total)%pageSize > 0 {
		totalPages++
	}
	return &AdminOrderList{
		List:         list,
		Total:        total,
		Page:         page,
		PageSize:     pageSize,
		TotalPages:   totalPages,
		StatusCounts: statusCounts,
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
	// V1.5 经营范围收敛
	if err := s.access.AssertStatsScope(ctx, op, enterpriseID); err != nil {
		return nil, err
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
	// V1.5 经营范围收敛
	if err := s.access.AssertStatsScope(ctx, op, req.EnterpriseID); err != nil {
		return nil, err
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
	// V1.5 经营范围收敛
	if err := s.access.AssertStatsScope(ctx, op, req.EnterpriseID); err != nil {
		return nil, err
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
	// V1.5 经营范围：维修业务员的企业对比仅限其服务范围内的客户
	scope, err := s.access.OrderScope(ctx, op)
	if err != nil {
		return nil, err
	}
	if scope.Scoped && req.EnterpriseID != "" && !scope.InScope(req.EnterpriseID) {
		return nil, apperrors.ErrForbidden.WithMessage("该企业不在您的服务范围内，请联系管理员分配")
	}
	rows, err := s.orders.StatsByEnterprise(ctx, req.EnterpriseID, req.Start, req.End)
	if err != nil {
		return nil, s.dbErr("stats by enterprise failed", err)
	}
	list := make([]EnterpriseStatsRow, 0, len(rows))
	for _, row := range rows {
		if !scope.InScope(row.EnterpriseID) {
			continue
		}
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
	// V1.5 经营范围：详情读取同样收敛（含已接单兜底）
	if err := s.access.AssertOrderInScope(ctx, op, order); err != nil {
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

	tagD, tagTypeD := ComputeDisplayTag(order, len(receipts) > 0)
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
		Remark:           order.Remark,
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
		AvailableActions: filterOwnerActions(op, order.RepairerID, adminAvailableActions(order.Status, order.SignatureStatus)),
		CreatedAt:        order.CreatedAt,
		SubmittedAt:      order.SubmittedAt,
		AuditedAt:        order.AuditedAt,
		AcceptedAt:       order.AcceptedAt,
		CompletedAt:      order.CompletedAt,
		UpdatedAt:        order.UpdatedAt,
		DisplayTag:       tagD,
		DisplayTagType:   tagTypeD,
		ReceiptUploaded:  len(receipts) > 0,
		SignatureStatus:  order.SignatureStatus,
		FinishOutcome:    order.FinishOutcome,
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

// Unaudit 撤回审核 (V1.5, 问题 #1/#22): pending_accept → reported
//
// 业务背景：审核通过后、业务员接单前，报修人发现填错了需要改，
// 审核员可撤回审核让其重新编辑后再提交。
//
// 前置条件：
//  1. 当前状态为 pending_accept
//  2. 尚未有人接单（repairer_id 为空）——已接单则不可撤回，避免与在修工单冲突
//
// 幂等：已处于 reported 时重复调用返回业务错误，而非静默成功。
func (s *AdminOrderService) Unaudit(ctx context.Context, op Operator, orderID, remark, ip string) error {
	if err := validateLength("remark", remark, 0, 100); err != nil {
		return err
	}
	order, err := s.loadOrder(ctx, orderID)
	if err != nil {
		return err
	}
	if order.Status == string(model.OrderReported) {
		return apperrors.ErrOrderCannotEdit.WithMessage("该工单已处于待审核状态，无需重复撤回")
	}
	if order.Status != string(model.OrderPendingAccept) {
		return apperrors.ErrOrderCannotEdit.WithMessage("仅 pending_accept（已审核待接单）状态的工单可撤回审核")
	}
	if order.RepairerID != nil && *order.RepairerID != "" {
		return apperrors.ErrOrderCannotEdit.WithMessage("业务员已接单，无法撤回审核")
	}
	if err := s.access.CanAccessOrder(ctx, order.EnterpriseID, op.UserID, op.Role); err != nil {
		return err
	}

	from := order.Status
	order.Status = string(model.OrderReported)
	order.AuditedAt = nil
	order.AuditedBy = nil
	order.AcceptedAt = nil
	if err := s.orders.Update(ctx, order); err != nil {
		return s.dbErr("update order failed", err)
	}
	if err := s.appendAdminTimeline(ctx, order, op.UserID, string(model.ActionUnaudit), from, string(model.OrderReported), remark, ip); err != nil {
		return err
	}
	s.logger.Info("order unaudited",
		zap.String("order_id", order.ID), zap.String("operator_id", op.UserID))
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
	// V1.5 经营范围前置校验：接单属于写操作，服务端必须强制拦截，
	// 否则"只能看范围内的单"会形同虚设（可直接按工单号越权接单）。
	if order.EnterpriseID != nil {
		if err := s.access.AssertEnterpriseInScope(ctx, op, *order.EnterpriseID); err != nil {
			return err
		}
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
	// 最短 2 字（原为 10 字，按 2026-09-12 问题汇总 #24 放宽）
	if n < 2 || n > 200 {
		return apperrors.ErrRejectReasonTooShort.WithMessage("退回原因需为2-200字")
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
	case string(model.OrderPendingAccept):
		// V1.5: 审核通过后、业务员接单前，单位审核员(或店方)也可直接打回给申报人重新修改。
		// 此时尚未接单(RepairerID 为空)，无需归属校验；复用与 reported 一致的 CanAccessOrder 鉴权。
		if err := s.access.CanAccessOrder(ctx, order.EnterpriseID, op.UserID, op.Role); err != nil {
			return err
		}
	case string(model.OrderProcessing):
		if err := s.access.StaffOnly(op); err != nil {
			return apperrors.ErrNotAdmin.WithMessage("仅维修业务员/超级管理员可退回处理中的工单")
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

// CompleteOrderInput 完工登记入参 (5.6, req②/④)
type CompleteOrderInput struct {
	Remark        string
	ReceiptIDs    []string
	Quantity      int
	UnitPrice     float64
	RepairContent string
	Metadata      model.RepairMetadata
	Result        string // ""/"complete"(默认) | "customer_cancelled" | "unrepairable"
	SignMode      string // ""/"signed"(默认) | "pending_sign"，仅 result=complete 生效
}

// 完工登记三结果 (req②): 完成 / 客户原因取消 / 不能修
const (
	FinishResultComplete          = "complete"           // 完成: processing → completed
	FinishResultCustomerCancelled = "customer_cancelled" // 客户原因取消: processing → cancelled
	FinishResultUnrepairable      = "unrepairable"       // 不能修: processing → cancelled
)

// UpdateFinanceInput 修改对账信息入参 (5.6.1, 全部可选, 至少一项)
type UpdateFinanceInput struct {
	Quantity      *int
	UnitPrice     *float64
	RepairContent *string
	Metadata      *model.RepairMetadata
	Remark        *string
}

// Complete 完工登记 (5.6, req②): processing → completed(完成) 或 cancelled(客户原因取消/不能修)。
// 三结果由 in.Result 决定: complete(默认) / customer_cancelled / unrepairable。
// 完成需收据+对账字段；两取消终态不要求收据/金额，仅记 finish_outcome 细分。
//
// 鉴权: 店方角色。
func (s *AdminOrderService) Complete(ctx context.Context, op Operator, orderID, ip string, in CompleteOrderInput) error {
	if err := s.access.StaffOnly(op); err != nil {
		return err
	}
	remark := strings.TrimSpace(in.Remark)
	// 维修备注长度上限校验（≤200）；是否必填按结单结果区分（见下方 cancel/unrepairable 分支）。
	if len(remark) > 200 {
		return apperrors.ErrInvalidParam.WithMessage("维修备注不能超过200字")
	}

	result := strings.TrimSpace(in.Result)
	if result == "" {
		result = FinishResultComplete
	}
	switch result {
	case FinishResultComplete, FinishResultCustomerCancelled, FinishResultUnrepairable:
	default:
		return apperrors.ErrInvalidParam.WithMessage("result 取值非法（应为 complete / customer_cancelled / unrepairable）")
	}

	order, err := s.loadOrder(ctx, orderID)
	if err != nil {
		return err
	}
	if order.Status != string(model.OrderProcessing) {
		return apperrors.ErrOrderCannotEdit.WithMessage("仅 processing 状态的工单可结单")
	}
	// 归属校验：仅接单人本人或超级管理员可结单
	if err := ensureOrderOwner(op, order); err != nil {
		return err
	}

	// 维修备注（结单原因/说明）持久化到工单，便于后续补签/查看时回显
	order.Remark = remark

	// 业务规则：客户原因取消 / 不能修 场景下，维修备注（原因说明）必填；
	// 完成场景（含稍后补签）维修备注选填，避免与元数据"额外备注"语义冲突。
	if (result == FinishResultCustomerCancelled || result == FinishResultUnrepairable) && strings.TrimSpace(in.Remark) == "" {
		return apperrors.ErrInvalidParam.WithMessage("维修备注必填：请说明客户取消/无法维修的原因")
	}

	from := order.Status
	now := time.Now()

	// ── 完成分支: processing → completed(已签字) 或 维持 processing(稍后补签) ──
	if result == FinishResultComplete {
		signMode := strings.TrimSpace(in.SignMode)
		if signMode == "" {
			signMode = model.SignModeSigned
		}
		switch signMode {
		case model.SignModeSigned:
			// 客户已当场签字：收据/对账必填，写 completed_at，进入时限闸门
		if len(in.ReceiptIDs) == 0 {
			return apperrors.ErrInvalidParam.WithMessage("receipt_ids 不能为空")
		}
		if len(in.ReceiptIDs) > maxReceiptImages {
				return apperrors.ErrImageTooMany.WithMessage("收据最多 3 张")
			}
			if in.Quantity < 1 {
				in.Quantity = 1 // 数量默认 1（需求③）
			}
			if in.UnitPrice < 0 {
				return apperrors.ErrInvalidParam.WithMessage("unit_price 需 ≥0")
			}
			if strings.TrimSpace(in.RepairContent) == "" {
				return apperrors.ErrInvalidParam.WithMessage("repair_content 必填")
			}

			order.Status = string(model.OrderCompleted)
			order.CompletedAt = &now
			order.FinishOutcome = ""
			order.SignatureStatus = string(model.SignatureSigned)
			order.RepairContent = strings.TrimSpace(in.RepairContent)
			order.Quantity = in.Quantity
			order.UnitPrice = in.UnitPrice
			if err := s.setOrderMetadata(ctx, order, in.Metadata); err != nil {
				return err
			}
		if err := s.orders.Update(ctx, order); err != nil {
			return s.dbErr("update order failed", err)
		}
		if err := s.associateReceiptImages(ctx, order.ID, in.ReceiptIDs); err != nil {
			return err
		}
		s.logger.Info("order completed with signature",
				zap.String("order_id", order.ID), zap.String("operator_id", op.UserID))
			if s.notifier != nil {
				s.notifier.NotifyOrderComplete(ctx, order, op.UserID)
			}
			return s.appendAdminTimeline(ctx, order, op.UserID, string(model.ActionComplete), from, string(model.OrderCompleted), remark, ip)

		case model.SignModePendingSign:
			// 稍后补签：保持 processing、不写 completed_at、金额后续可改；
			// 收据/对账选填，仅记录签字状态为待补签。
			if in.Quantity < 1 {
				in.Quantity = 1
			}
			if in.UnitPrice < 0 {
				return apperrors.ErrInvalidParam.WithMessage("unit_price 需 ≥0")
			}
			order.Status = string(model.OrderProcessing) // 维持处理中
			order.CompletedAt = nil
			order.FinishOutcome = ""
			order.SignatureStatus = string(model.SignaturePendingSign)
			if strings.TrimSpace(in.RepairContent) != "" {
				order.RepairContent = strings.TrimSpace(in.RepairContent)
			}
			order.Quantity = in.Quantity
			order.UnitPrice = in.UnitPrice
			if err := s.setOrderMetadata(ctx, order, in.Metadata); err != nil {
				return err
			}
			if err := s.orders.Update(ctx, order); err != nil {
				return s.dbErr("update order failed", err)
			}
			// 收据选填：传了就激活(即时上传的 temporary 经 diff 变 active), 没传保持原样
			if len(in.ReceiptIDs) > 0 {
				if len(in.ReceiptIDs) > maxReceiptImages {
					return apperrors.ErrImageTooMany.WithMessage("收据最多 3 张")
				}
				if err := s.associateReceiptImages(ctx, order.ID, in.ReceiptIDs); err != nil {
					return err
				}
			}
			s.logger.Info("order deferred for signature",
				zap.String("order_id", order.ID), zap.String("operator_id", op.UserID))
			return s.appendAdminTimeline(ctx, order, op.UserID, string(model.ActionDeferSign), from, string(model.OrderProcessing), remark, ip)

		default:
			return apperrors.ErrInvalidParam.WithMessage("sign_mode 取值非法（应为 signed / pending_sign）")
		}
	}

	// ── 客户原因取消 / 不能修: processing → cancelled(终态), 记 finish_outcome 细分 ──
	order.Status = string(model.OrderCancelled)
	order.CompletedAt = nil
	order.SignatureStatus = string(model.SignatureNone)
	order.FinishOutcome = result // 直接复用常量值 unrepairable / customer_cancelled
	if err := s.orders.Update(ctx, order); err != nil {
		return s.dbErr("update order failed", err)
	}
	s.logger.Info("order finished as cancelled outcome",
		zap.String("order_id", order.ID), zap.String("outcome", order.FinishOutcome), zap.String("operator_id", op.UserID))
	return s.appendAdminTimeline(ctx, order, op.UserID, string(model.ActionCancel), from, string(model.OrderCancelled), remark, ip)
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
	// 时限闸门：超期后仅超级管理员可操作，其余拦截提示
	overdue, err := s.isCompletedOverdue(ctx, order)
	if err != nil {
		return err
	}
	if overdue && !op.IsSuperAdmin() {
		return apperrors.ErrForbidden.WithMessage("该工单已超出完工后二次修改时限，请联系超级管理员处理")
	}
	// 归属校验：仅接单人本人或超级管理员可修改对账
	if err := ensureOrderOwner(op, order); err != nil {
		return err
	}
	if in.Quantity == nil && in.UnitPrice == nil && in.RepairContent == nil && in.Metadata == nil && in.Remark == nil {
		return apperrors.ErrInvalidParam.WithMessage("至少需要传入一个字段")
	}

	beforeQuantity := order.Quantity
	beforeUnitPrice := order.UnitPrice
	beforeRepairContent := order.RepairContent
	beforeRemark := order.Remark
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
	if in.Remark != nil {
		trimmed := strings.TrimSpace(*in.Remark)
		if len(trimmed) > 200 {
			return apperrors.ErrInvalidParam.WithMessage("维修备注不能超过200字")
		}
		order.Remark = trimmed
	}
	if in.Metadata != nil {
		if err := s.setOrderMetadata(ctx, order, *in.Metadata); err != nil {
			return err
		}
	}

	if err := s.orders.Update(ctx, order); err != nil {
		return s.dbErr("update order failed", err)
	}

	diff := financeIncrementDiff(beforeQuantity, beforeUnitPrice, beforeRepairContent, beforeRemark, beforeMeta, order)
	if overdue && op.IsSuperAdmin() {
		diff = diff + "（超期操作）"
	}
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
	// 时限闸门：超期后仅超级管理员可操作，其余拦截提示
	overdue, err := s.isCompletedOverdue(ctx, order)
	if err != nil {
		return err
	}
	if overdue && !op.IsSuperAdmin() {
		return apperrors.ErrForbidden.WithMessage("该工单已超出完工后二次修改时限，请联系超级管理员处理")
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
	reopenRemark := strings.TrimSpace(remark)
	if overdue && op.IsSuperAdmin() {
		reopenRemark = reopenRemark + "（超期操作）"
	}
	if err := s.appendAdminTimeline(ctx, order, op.UserID, string(model.ActionReopen), from, string(model.OrderProcessing), reopenRemark, ip); err != nil {
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

	count, err := s.images.CountActive(ctx, orderID, string(model.ImageReceipt))
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

// SignCompleteInput 补签正式完工入参 (req④)
type SignCompleteInput struct {
	ReceiptIDs    []string
	Quantity      int
	UnitPrice     float64
	RepairContent string
	Metadata      model.RepairMetadata
	Remark        string
}

// SignComplete 补签正式完工 (req④): 仅 processing 且 signature_status=pending_sign 的工单。
// 校验通过后 → completed + 写 completed_at + signature_status=signed；
// 可补登金额/收据（收据选填），写 ActionSign 时间轴，进入时限闸门。
//
// 鉴权: 店方角色。
func (s *AdminOrderService) SignComplete(ctx context.Context, op Operator, orderID, ip string, in SignCompleteInput) error {
	if err := s.access.StaffOnly(op); err != nil {
		return err
	}
	order, err := s.loadOrder(ctx, orderID)
	if err != nil {
		return err
	}
	if order.Status != string(model.OrderProcessing) || order.SignatureStatus != string(model.SignaturePendingSign) {
		return apperrors.ErrOrderCannotEdit.WithMessage("仅「待补签」状态的工单可补签正式完工")
	}
	if err := ensureOrderOwner(op, order); err != nil {
		return err
	}

	remark := strings.TrimSpace(in.Remark)
	if err := validateLength("remark", remark, 0, 200); err != nil {
		return err
	}

	from := order.Status
	now := time.Now()

	if in.Quantity < 1 {
		in.Quantity = 1
	}
	if in.UnitPrice < 0 {
		return apperrors.ErrInvalidParam.WithMessage("unit_price 需 ≥0")
	}

	order.Status = string(model.OrderCompleted)
	order.CompletedAt = &now
	order.SignatureStatus = string(model.SignatureSigned)
	if strings.TrimSpace(in.RepairContent) != "" {
		order.RepairContent = strings.TrimSpace(in.RepairContent)
	}
	order.Quantity = in.Quantity
	order.UnitPrice = in.UnitPrice
	order.Remark = remark
	if err := s.setOrderMetadata(ctx, order, in.Metadata); err != nil {
		return err
	}
	if err := s.orders.Update(ctx, order); err != nil {
		return s.dbErr("update order failed", err)
	}
	// 收据选填：传了就激活(即时上传的 temporary 经 diff 变 active), 没传保持原样
	if len(in.ReceiptIDs) > 0 {
		if len(in.ReceiptIDs) > maxReceiptImages {
			return apperrors.ErrImageTooMany.WithMessage("收据最多 3 张")
		}
		if err := s.associateReceiptImages(ctx, order.ID, in.ReceiptIDs); err != nil {
			return err
		}
	}
	s.logger.Info("order signed and completed",
		zap.String("order_id", order.ID), zap.String("operator_id", op.UserID))
	if s.notifier != nil {
		s.notifier.NotifyOrderComplete(ctx, order, op.UserID)
	}
	return s.appendAdminTimeline(ctx, order, op.UserID, string(model.ActionSign), from, string(model.OrderCompleted), remark, ip)
}

// associateReceiptImages 提交/补签时收据图关联 (status 流程, 与故障图 OrderService.AssociateImages 一致):
// 上传阶段 UploadReceipt 仅产生 temporary 记录; 此处做 diff: 现表 receiptIDs → active(回填 sort_order),
// 原表 active 但不在 receiptIDs 中的收据 → deleted。仅 active 计入工单收据数。
func (s *AdminOrderService) associateReceiptImages(ctx context.Context, orderID string, receiptIDs []string) error {
	if len(receiptIDs) > maxReceiptImages {
		return apperrors.ErrImageTooMany.WithMessage("收据最多 3 张")
	}
	kept := make([]string, 0, len(receiptIDs))
	for idx, id := range receiptIDs {
		id = strings.TrimSpace(id)
		if id == "" {
			continue
		}
		if err := s.images.ActivateOne(ctx, orderID, string(model.ImageReceipt), id, idx); err != nil {
			return s.dbErr("activate receipt failed", err)
		}
		kept = append(kept, id)
	}
	if err := s.images.SoftDeleteActiveExcept(ctx, orderID, string(model.ImageReceipt), kept); err != nil {
		return s.dbErr("soft delete receipts failed", err)
	}
	return nil
}

// ────────────────────────────────────────────
// 内部辅助
// ────────────────────────────────────────────

// isCompletedOverdue 判断已完工工单是否已超出「二次修改时限」窗口 (仅 completed 态有意义)。
// 返回 true 表示已超期；非 completed 或窗口内返回 false。读取 system_settings 阈值，缺失回退默认 24h。
func (s *AdminOrderService) isCompletedOverdue(ctx context.Context, order *model.RepairOrder) (bool, error) {
	if order.Status != string(model.OrderCompleted) || order.CompletedAt == nil {
		return false, nil
	}
	window := s.settings.GetCompletedEditWindowHours(ctx)
	if time.Since(*order.CompletedAt) > time.Duration(window)*time.Hour {
		return true, nil
	}
	return false, nil
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
	beforeQuantity int, beforeUnitPrice float64, beforeRepairContent string, beforeRemark string,
	oldMeta model.RepairMetadata, newOrder *model.RepairOrder,
) string {
	newMeta := parseMetadata(newOrder.Metadata)
	diff := ""
	diff = appendChangeInt(diff, "数量", beforeQuantity, newOrder.Quantity)
	diff = appendChangePrice(diff, "单价", beforeUnitPrice, newOrder.UnitPrice)
	diff = appendChange(diff, "维修内容", beforeRepairContent, newOrder.RepairContent)
	diff = appendChange(diff, "维修备注", beforeRemark, newOrder.Remark)
	diff = appendChange(diff, "维修结果", oldMeta.RepairResult, newMeta.RepairResult)
	diff = appendChange(diff, "维修方式", oldMeta.RepairMethod, newMeta.RepairMethod)
	diff = appendChange(diff, "保修期", oldMeta.WarrantyPeriod, newMeta.WarrantyPeriod)
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
	string(model.ActionSign):          true,
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

func adminAvailableActions(status, signatureStatus string) []AdminAction {
	orderSignaturePending := signatureStatus == string(model.SignaturePendingSign)
	switch model.OrderStatus(status) {
	case model.OrderReported:
		return []AdminAction{
			{Action: string(model.ActionAudit), Label: "审核通过", ToStatus: string(model.OrderPendingAccept), ConfirmMessage: "审核通过后工单进入待接单并上报维修业务员，确认？"},
			{Action: string(model.ActionReject), Label: "退回", ToStatus: string(model.OrderRejected), RequireReason: true, ReasonMinLength: 2, ConfirmMessage: "退回后工单进入「已退回」，报修人可修改重新提交，确认退回？"},
		}
	case model.OrderPendingAccept:
		return []AdminAction{
			{Action: string(model.ActionAccept), Label: "接单维修", ToStatus: string(model.OrderProcessing), ConfirmMessage: "确认接单维修该工单？"},
			{Action: string(model.ActionReject), Label: "退回", ToStatus: string(model.OrderRejected), RequireReason: true, ReasonMinLength: 2, ConfirmMessage: "退回后工单进入「已退回」，报修人可修改重新提交，确认退回？"},
		}
	case model.OrderProcessing:
		// 待补签：仅展示「上传签字单，正式完工」+ 退回（不再展示普通完工）
		if status == string(model.OrderProcessing) && orderSignaturePending {
			return []AdminAction{
				{Action: string(model.ActionSign), Label: "上传签字单，正式完工", ToStatus: string(model.OrderCompleted), ConfirmMessage: "确认上传签字单并正式完工该工单？"},
				{Action: string(model.ActionReject), Label: "退回", ToStatus: string(model.OrderRejected), RequireReason: true, ReasonMinLength: 2, ConfirmMessage: "退回后工单进入「已退回」，报修人可修改重新提交，确认退回？"},
			}
		}
		return []AdminAction{
			{Action: string(model.ActionComplete), Label: "完工", ToStatus: string(model.OrderCompleted), ConfirmMessage: "确认完工该工单？"},
			{Action: string(model.ActionReject), Label: "退回", ToStatus: string(model.OrderRejected), RequireReason: true, ReasonMinLength: 2, ConfirmMessage: "退回后工单进入「已退回」，报修人可修改重新提交，确认退回？"},
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
