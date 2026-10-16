// 首页待办看板（Dashboard）· V1.7 角色化卡片矩阵
//
// 形态：按角色给出 2-3 张卡片（不是时间流），每卡给出
//   - total  ：该卡当前命中的工单总数
//   - unread ：其中「有他人新动态且我尚未查看」的工单数（仅「提醒」卡启用）
//   - orders ：该卡最近 N 条工单（点进去直达详情）
//
// 卡片矩阵：
//   - 企业审核员：待审核（本单位，审完才消除）+ 进行中（我报修的）+ 提醒（我报修的已完工/已退回/已取消未读）
//   - 维修业务员：处理中（我接的，含待补签）+ 待接单（业务范围内可接）
//   - 普通用户  ：进行中（我报修的）+ 提醒（我报修的已完工/已退回/已取消未读）
//
// 未读判定：order_timeline.created_at > order_reads.last_viewed_at 且该动态非我所为。
// 只有「提醒」卡做未读收敛——进行中/待审核/待接单属于「待办」，不做未读，避免红点噪音。
// 只做聚合与取最近几条，不分页返回全量流水（流水在工单详情页时间轴展示）。
package service

import (
	"context"
	"time"

	"go.uber.org/zap"

	apperrors "xin-ni-repair/internal/errors"
	"xin-ni-repair/internal/model"
	"xin-ni-repair/internal/repository"
)

// dashboardPreviewSize 每张卡片下挂的最近工单条数
const dashboardPreviewSize = 3

// DashboardOrder 看板卡片内的一条工单摘要
type DashboardOrder struct {
	ID             string `json:"id"`
	OrderNo        string `json:"order_no"`
	Description    string `json:"description"`
	EnterpriseName string `json:"enterprise_name"`
	Room           string `json:"room"`
	ReporterName   string `json:"reporter_name"`
	Urgency        string `json:"urgency"`
	UrgencyLabel   string `json:"urgency_label"`
	DisplayTag     string `json:"display_tag"`
	DisplayTagType string `json:"display_tag_type"`
	// Status 原始状态值：前端据此决定「提醒」卡内的工单该落到哪个列表 Tab
	Status    string `json:"status"`
	UpdatedAt string `json:"updated_at"`
	Unread    bool   `json:"unread"`
}

// DashboardCard 一张状态卡
type DashboardCard struct {
	Key      string           `json:"key"`
	Label    string           `json:"label"`
	Tag      string           `json:"tag"`
	Total    int64            `json:"total"`
	Unread   int64            `json:"unread"`
	Personal bool             `json:"personal"`
	Orders   []DashboardOrder `json:"orders"`
}

// DashboardResult 首页看板响应
type DashboardResult struct {
	Cards       []DashboardCard `json:"cards"`
	UnreadTotal int64           `json:"unread_total"`
	UpdatedAt   time.Time       `json:"updated_at"`
}

// cardDef 看板卡定义（内部使用；后端持有稳定英文键，中文文案随角色可微调）
type cardDef struct {
	Key             string
	Label           string
	Tag             string
	Statuses        []string
	SignatureStatus string
	// ReporterMe 限定「我报修的」；RepairerMe 限定「我接单的」
	ReporterMe bool
	RepairerMe bool
	// Sort 排序：board = 状态优先级 + 更新时间倒序（与工单列表「全部」Tab 一致）
	Sort string
	// UnreadOnly 只保留未读工单（提醒卡）
	UnreadOnly bool
	// CountUnread 是否计算未读数
	CountUnread bool
	// Personal 是否「我报修的」个人工单（首页用其归到「我的」分组）
	Personal bool
}

// DashboardService 首页看板服务
type DashboardService struct {
	orders *repository.OrderRepository
	access *AccessService
	logger *zap.Logger
}

// NewDashboardService 创建 DashboardService
func NewDashboardService(orders *repository.OrderRepository, access *AccessService, logger *zap.Logger) *DashboardService {
	return &DashboardService{orders: orders, access: access, logger: logger}
}

func (s *DashboardService) dbErr(msg string, err error) error {
	s.logger.Error(msg, zap.Error(err))
	return apperrors.ErrDatabaseError.WithError(err)
}

// buildCardDefs 按角色拼卡片（兼任取并集，最多 3 张）
//
// plainRepairer = 维修业务员（role=1，受经营范围约束）；超管后端算 staff，但小程序端
// 已退化为普通用户能力，故按 plainRepairer=false 处理，出「我报修的」两张卡。
func buildCardDefs(plainRepairer, reviewer bool) []cardDef {
	defs := make([]cardDef, 0, 3)

	// ① 单位审核员：待审核（本单位）。不做未读收敛——审核完才消除。
	if reviewer {
		defs = append(defs, cardDef{
			Key: "pending_review", Label: "待审核", Tag: "pending_review",
			Statuses: []string{string(model.OrderReported)},
			Sort:     repository.SortBoard,
		})
	}

	if plainRepairer {
		// ② 维修业务员：我接的处理中（含待补签，先后由 board 排序决定）+ 业务范围内待接单
		defs = append(defs, cardDef{
			Key: "repairer_processing", Label: "处理中", Tag: "processing",
			Statuses:   []string{string(model.OrderProcessing)},
			RepairerMe: true,
			Sort:       repository.SortBoard,
		})
		defs = append(defs, cardDef{
			Key: "pending_accept", Label: "待接单", Tag: "pending_accept",
			Statuses: []string{string(model.OrderPendingAccept)},
			Sort:     repository.SortBoard,
		})
	} else {
		// ③ 普通用户 / 单位审核员 / 超管：我报修的进行中 + 提醒（已完工·已退回·已取消未读）
		defs = append(defs, cardDef{
			Key: "in_progress", Label: "工单状态", Tag: "in_progress",
			Personal: true,
			Statuses: []string{
				string(model.OrderReported),
				string(model.OrderPendingAccept),
				string(model.OrderProcessing),
			},
			ReporterMe: true,
			Sort:       repository.SortBoard,
		})
		defs = append(defs, cardDef{
			Key: "notice", Label: "提醒", Tag: "",
			Personal: true,
			Statuses: []string{
				string(model.OrderCompleted),
				string(model.OrderRejected),
				string(model.OrderCancelled),
			},
			ReporterMe:  true,
			Sort:        repository.SortBoard,
			UnreadOnly:  true,
			CountUnread: true,
		})
	}
	return defs
}

// Dashboard 按角色产出首页看板
//
// enterpriseID 仅对单位审核员生效（首页多单位切换条选中的单位）；为空则按其所任
// 审核员的全部单位收敛。维修业务员按 V1.5 经营范围收敛；普通用户仅自己的工单。
func (s *DashboardService) Dashboard(ctx context.Context, op Operator, enterpriseID string) (*DashboardResult, error) {
	staff := op.IsStoreStaff()
	var reviewerIDs []string
	if !staff {
		ids, err := s.access.ReviewerEnterpriseIDs(ctx, op.UserID)
		if err != nil {
			return nil, s.dbErr("dashboard: list reviewer enterprises failed", err)
		}
		reviewerIDs = ids
	}
	reviewer := len(reviewerIDs) > 0

	// 基础过滤：数据权限（经营范围 / 审核单位）
	base := repository.OrderAdminFilter{}
	switch {
	case staff:
		if err := s.access.ApplyOrderScope(ctx, op, &base); err != nil {
			return nil, err
		}
	case reviewer:
		if enterpriseID != "" {
			if err := s.access.CanManageEnterprise(ctx, enterpriseID, op.UserID, op.Role); err != nil {
				return nil, err
			}
			base.EnterpriseID = enterpriseID
		} else {
			base.ScopeLimited = true
			base.ScopeEnterpriseIDs = reviewerIDs
		}
	}

	cards := make([]DashboardCard, 0, 3)
	unreadTotal := int64(0)
	for _, d := range buildCardDefs(op.IsPlainRepairer(), reviewer) {
		card, err := s.buildCard(ctx, op, base, d, false)
		if err != nil {
			return nil, err
		}
		// 空卡不返回：前端直接不渲染（避免「暂无」占位占屏）
		if card == nil {
			continue
		}
		unreadTotal += card.Unread
		cards = append(cards, *card)
	}

	return &DashboardResult{Cards: cards, UnreadTotal: unreadTotal, UpdatedAt: time.Now()}, nil
}

// buildCard 构造单张看板卡。full=true 时突破每卡 3 条预览上限（首页「查看全部」展开用，
// 提醒卡仍仅返回未读集合——其 def 的 UnreadOnly 已保证）。空卡返回 nil。
func (s *DashboardService) buildCard(
	ctx context.Context, op Operator, base repository.OrderAdminFilter, d cardDef, full bool,
) (*DashboardCard, error) {
	f := base
	f.Status = d.Statuses
	f.SignatureStatus = d.SignatureStatus
	// 「我报修的」始终可见：不受经营范围 / 审核单位收敛影响，
	// 否则业务员自己报的单会因为该企业不在其服务范围内而消失。
	if d.ReporterMe {
		f.ScopeLimited = false
		f.ScopeEnterpriseIDs = nil
		f.OwnerOrRepairerID = ""
		f.EnterpriseID = ""
		f.ReporterID = op.UserID
	}
	if d.RepairerMe {
		f.RepairerID = op.UserID
	}

	limit := dashboardPreviewSize
	if full {
		limit = 0 // 全量（仓库侧 Limit<=0 表示不限）
	}

	stat, orders, err := s.orders.ListDashboardCard(ctx, f, op.UserID, repository.DashboardCardOptions{
		Limit:       limit,
		Sort:        d.Sort,
		UnreadOnly:  d.UnreadOnly,
		CountUnread: d.CountUnread,
	})
	if err != nil {
		return nil, s.dbErr("dashboard: list card failed", err)
	}
	if stat.Total == 0 {
		return nil, nil
	}

	// 未读角标：只有启用未读统计的卡才需要逐条判定
	flags := map[string]bool{}
	if d.CountUnread {
		ids := make([]string, 0, len(orders))
		for i := range orders {
			ids = append(ids, orders[i].ID)
		}
		flags, err = s.orders.UnreadFlags(ctx, op.UserID, ids)
		if err != nil {
			return nil, s.dbErr("dashboard: unread flags failed", err)
		}
	}

	list := make([]DashboardOrder, 0, len(orders))
	for i := range orders {
		o := orders[i]
		tag, tagType := ComputeDisplayTag(&o, false)
		orderNo := ""
		if o.OrderNo != nil {
			orderNo = *o.OrderNo
		}
		urgencyLabel := o.Urgency
		if v, ok := urgencyLabels[o.Urgency]; ok {
			urgencyLabel = v
		}
		list = append(list, DashboardOrder{
			ID:             o.ID,
			OrderNo:        orderNo,
			Description:    o.Description,
			EnterpriseName: o.Enterprise.Name,
			Room:           o.Room,
			ReporterName:   o.Reporter.Nickname,
			Urgency:        o.Urgency,
			UrgencyLabel:   urgencyLabel,
			DisplayTag:     tag,
			DisplayTagType: tagType,
			Status:         o.Status,
			UpdatedAt:      o.UpdatedAt.Format("2006-01-02 15:04:05"),
			Unread:         d.CountUnread && flags[o.ID],
		})
	}

	return &DashboardCard{
		Key:      d.Key,
		Label:    d.Label,
		Tag:      d.Tag,
		Total:    stat.Total,
		Unread:   stat.Unread,
		Personal: d.Personal,
		Orders:   list,
	}, nil
}

// DashboardCardDetail 单张看板卡的全量内容（突破每卡预览上限），
// 供首页「查看全部」展开。提醒卡因 def.UnreadOnly 为真，仍只返回未读工单。
func (s *DashboardService) DashboardCardDetail(
	ctx context.Context, op Operator, enterpriseID, cardKey string,
) (*DashboardCard, error) {
	staff := op.IsStoreStaff()
	var reviewerIDs []string
	if !staff {
		ids, err := s.access.ReviewerEnterpriseIDs(ctx, op.UserID)
		if err != nil {
			return nil, s.dbErr("dashboard: list reviewer enterprises failed", err)
		}
		reviewerIDs = ids
	}
	reviewer := len(reviewerIDs) > 0

	// 基础过滤：数据权限（经营范围 / 审核单位），与 Dashboard 完全一致
	base := repository.OrderAdminFilter{}
	switch {
	case staff:
		if err := s.access.ApplyOrderScope(ctx, op, &base); err != nil {
			return nil, err
		}
	case reviewer:
		if enterpriseID != "" {
			if err := s.access.CanManageEnterprise(ctx, enterpriseID, op.UserID, op.Role); err != nil {
				return nil, err
			}
			base.EnterpriseID = enterpriseID
		} else {
			base.ScopeLimited = true
			base.ScopeEnterpriseIDs = reviewerIDs
		}
	}

	defs := buildCardDefs(op.IsPlainRepairer(), reviewer)
	var target *cardDef
	for i := range defs {
		if defs[i].Key == cardKey {
			target = &defs[i]
			break
		}
	}
	if target == nil {
		return nil, apperrors.ErrInvalidParam.WithMessage("未知的卡片类型")
	}
	return s.buildCard(ctx, op, base, *target, true)
}

// MarkRead 标记某工单对当前用户已读（进详情页时调用，红点随之消失）。
//
// 权限：报修人本人 / 接单人 / 审核人 / 店方角色 / 本单位审核员；越权返回 ErrForbidden。
func (s *DashboardService) MarkRead(ctx context.Context, op Operator, orderID string) error {
	o, err := s.orders.FindByID(ctx, orderID)
	if err != nil {
		return s.dbErr("dashboard: find order failed", err)
	}
	if o == nil {
		return apperrors.ErrNotFound.WithMessage("工单不存在")
	}

	allowed := op.IsStoreStaff() || o.ReporterID == op.UserID ||
		(o.RepairerID != nil && *o.RepairerID == op.UserID) ||
		(o.AuditedBy != nil && *o.AuditedBy == op.UserID)
	if !allowed && o.EnterpriseID != nil {
		ok, rErr := s.access.ReviewerOfEnterprise(ctx, *o.EnterpriseID, op.UserID)
		if rErr != nil {
			return s.dbErr("dashboard: check reviewer failed", rErr)
		}
		allowed = ok
	}
	if !allowed {
		return apperrors.ErrForbidden.WithMessage("无权查看该工单")
	}

	if err := s.orders.MarkRead(ctx, orderID, op.UserID); err != nil {
		return s.dbErr("dashboard: mark read failed", err)
	}
	return nil
}
