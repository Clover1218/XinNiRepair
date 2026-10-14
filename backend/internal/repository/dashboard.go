package repository

import (
	"context"
	"time"

	"gorm.io/gorm"

	"xin-ni-repair/internal/model"
)

// ────────────────────────────────────────────
// 首页待办看板（按状态聚合 + 工单级未读）
// ────────────────────────────────────────────

// DashboardCardStat 单张看板卡的聚合结果
type DashboardCardStat struct {
	Total  int64 `json:"total"`
	Unread int64 `json:"unread"`
}

// unreadCond 工单级未读判定：
// 存在一条晚于「该用户对该工单最后查看时间」的动态，且该动态不是自己产生的。
//
// 排除自己操作是必须的——否则用户自己提交报修 / 自己接单后会立刻看到红点。
// 从未查看过（order_reads 无记录）时以工单创建时间为基线。
const unreadCond = `EXISTS (
	SELECT 1 FROM order_timeline t
	WHERE t.order_id = repair_orders.id
		AND t.operator_id <> ?
		AND t.created_at > COALESCE(
			(SELECT r.last_viewed_at FROM order_reads r
			 WHERE r.order_id = repair_orders.id AND r.user_id = ?),
			repair_orders.created_at
		)
)`

// applyCardFilter 复用管理端筛选语义的必要子集（状态 / 单位 / 报修人 / 接单人 /
// 签字子状态 / 终态成因 / 经营范围）。刻意不 JOIN users：避免报修人被软删时连带丢失工单。
func applyCardFilter(q *gorm.DB, f OrderAdminFilter) *gorm.DB {
	if len(f.Status) > 0 {
		q = q.Where("repair_orders.status IN ?", f.Status)
	}
	if f.EnterpriseID != "" {
		q = q.Where("repair_orders.enterprise_id = ?", f.EnterpriseID)
	}
	if f.ReporterID != "" {
		q = q.Where("repair_orders.reporter_id = ?", f.ReporterID)
	}
	if f.RepairerID != "" {
		q = q.Where("repair_orders.repairer_id = ?", f.RepairerID)
	}
	if f.SignatureStatus != "" {
		q = q.Where("repair_orders.signature_status = ?", f.SignatureStatus)
	}
	if f.FinishOutcome != "" {
		q = q.Where("repair_orders.finish_outcome = ?", f.FinishOutcome)
	}
	return q
}

// DashboardCardOptions 单张看板卡的查询选项（V1.7）
type DashboardCardOptions struct {
	// Limit 卡内最多返回的工单条数
	Limit int
	// Sort 排序："" = updated_at DESC；board = 状态优先级 + updated_at DESC
	Sort string
	// UnreadOnly 只统计/返回「他人有新增动态且我未读」的工单（提醒卡用）
	UnreadOnly bool
	// CountUnread 是否额外计算未读数（进行中/待审核等卡片不需要，省一次 COUNT）
	CountUnread bool
}

// ListDashboardCard 查询单张看板卡：总数 / 未读数 / 最近 limit 条工单。
//
// 只做聚合与取最近几条——**不分页返回全量流水**，流水展示在工单详情页的时间轴里。
func (r *OrderRepository) ListDashboardCard(
	ctx context.Context, f OrderAdminFilter, userID string, opts DashboardCardOptions,
) (DashboardCardStat, []model.RepairOrder, error) {
	scopeCond, scopeArgs, visible := applyScope(f)
	if !visible {
		// 无任何可见范围（如业务员尚未分配经营范围）→ 返回空卡，绝不退化成全量
		return DashboardCardStat{}, []model.RepairOrder{}, nil
	}
	base := r.db.WithContext(ctx).Model(&model.RepairOrder{})
	if scopeCond != "" {
		base = base.Where(scopeCond, scopeArgs...)
	}
	base = applyCardFilter(base, f)
	// 提醒卡：基数即未读集合（已读的不再出现）
	if opts.UnreadOnly {
		base = base.Where(unreadCond, userID, userID)
	}

	var stat DashboardCardStat
	if err := base.Session(&gorm.Session{}).Count(&stat.Total).Error; err != nil {
		return stat, nil, err
	}
	if stat.Total == 0 {
		return stat, []model.RepairOrder{}, nil
	}
	if opts.CountUnread {
		if err := base.Session(&gorm.Session{}).
			Where(unreadCond, userID, userID).
			Count(&stat.Unread).Error; err != nil {
			return stat, nil, err
		}
	}
	if opts.UnreadOnly {
		// 基数已是未读集合，未读数即总数
		stat.Unread = stat.Total
	}

	orderSQL := "repair_orders.updated_at DESC"
	if opts.Sort == SortBoard {
		orderSQL = boardOrderSQL
	}

	var orders []model.RepairOrder
	q := base.Preload("Enterprise").Preload("Reporter").Preload("Repairer").
		Order(orderSQL)
	// Limit<=0 表示「全量」（首页待办「查看全部」展开用），不附加 LIMIT 子句。
	if opts.Limit > 0 {
		q = q.Limit(opts.Limit)
	}
	err := q.Find(&orders).Error
	if err != nil {
		return stat, nil, err
	}
	return stat, orders, nil
}

// UnreadFlags 批量判定给定工单对指定用户是否未读（供看板卡片内每条工单打小红点）。
func (r *OrderRepository) UnreadFlags(ctx context.Context, userID string, orderIDs []string) (map[string]bool, error) {
	out := make(map[string]bool, len(orderIDs))
	if len(orderIDs) == 0 {
		return out, nil
	}

	var reads []model.OrderRead
	if err := r.db.WithContext(ctx).
		Where("user_id = ? AND order_id IN ?", userID, orderIDs).
		Find(&reads).Error; err != nil {
		return nil, err
	}
	readAt := make(map[string]time.Time, len(reads))
	for _, v := range reads {
		readAt[v.OrderID] = v.LastViewedAt
	}

	type lastRow struct {
		OrderID string
		MaxAt   time.Time
	}
	var rows []lastRow
	if err := r.db.WithContext(ctx).Model(&model.OrderTimeline{}).
		Select("order_id, MAX(created_at) AS max_at").
		Where("order_id IN ? AND operator_id <> ?", orderIDs, userID).
		Group("order_id").Scan(&rows).Error; err != nil {
		return nil, err
	}
	lastAct := make(map[string]time.Time, len(rows))
	for _, v := range rows {
		lastAct[v.OrderID] = v.MaxAt
	}

	for _, id := range orderIDs {
		at, hasAct := lastAct[id]
		if !hasAct {
			// 没有任何「他人操作」→ 谈不上未读（例如自己刚提交的单）
			out[id] = false
			continue
		}
		lv, seen := readAt[id]
		if !seen {
			out[id] = true
			continue
		}
		out[id] = at.After(lv)
	}
	return out, nil
}

// MarkRead 写入/刷新「某用户对某工单」的已读时间（幂等 upsert）。
func (r *OrderRepository) MarkRead(ctx context.Context, orderID, userID string) error {
	return r.db.WithContext(ctx).Exec(
		`INSERT INTO order_reads (order_id, user_id, last_viewed_at)
		 VALUES (?, ?, now())
		 ON CONFLICT (order_id, user_id) DO UPDATE SET last_viewed_at = now()`,
		orderID, userID,
	).Error
}
