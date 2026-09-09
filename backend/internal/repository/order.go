package repository

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"gorm.io/gorm"

	"xin-ni-repair/internal/model"
)

// ────────────────────────────────────────────
// 工单数据访问
// ────────────────────────────────────────────

// OrderRepository 工单数据访问
type OrderRepository struct {
	db *gorm.DB
}

// NewOrderRepository 创建 OrderRepository
func NewOrderRepository(db *gorm.DB) *OrderRepository {
	return &OrderRepository{db: db}
}

// Create 创建工单
func (r *OrderRepository) Create(ctx context.Context, order *model.RepairOrder) error {
	return r.db.WithContext(ctx).Create(order).Error
}

// FindByID 按 ID 查询工单 (含关联人/单位/审核人/维修员), 不存在时返回 nil
func (r *OrderRepository) FindByID(ctx context.Context, id string) (*model.RepairOrder, error) {
	var order model.RepairOrder
	err := r.db.WithContext(ctx).
		Preload("Enterprise").
		Preload("Reporter").
		Preload("Auditor").
		Preload("Repairer").
		Where("id = ?", id).First(&order).Error
	if err == gorm.ErrRecordNotFound {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &order, nil
}

// Update 更新工单
func (r *OrderRepository) Update(ctx context.Context, order *model.RepairOrder) error {
	return r.db.WithContext(ctx).Save(order).Error
}

// CountDraftsByUser 统计用户所有处于 draft 状态的工单数
func (r *OrderRepository) CountDraftsByUser(ctx context.Context, userID string) (int64, error) {
	var count int64
	err := r.db.WithContext(ctx).Model(&model.RepairOrder{}).
		Where("reporter_id = ? AND status = ?", userID, model.OrderDraft).
		Count(&count).Error
	return count, err
}

// ListByReporter 分页查询用户自己的工单, 支持企业与多状态筛选
func (r *OrderRepository) ListByReporter(ctx context.Context, reporterID, enterpriseID string, statuses []string, offset, limit int) ([]model.RepairOrder, int64, error) {
	base := r.db.WithContext(ctx).Model(&model.RepairOrder{}).
		Where("reporter_id = ?", reporterID)
	if enterpriseID != "" {
		base = base.Where("enterprise_id = ?", enterpriseID)
	}
	if len(statuses) > 0 {
		base = base.Where("status IN ?", statuses)
	}

	var total int64
	if err := base.Session(&gorm.Session{}).Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var orders []model.RepairOrder
	err := base.Preload("Enterprise").
		Order("created_at DESC").
		Offset(offset).
		Limit(limit).
		Find(&orders).Error
	if err != nil {
		return nil, 0, err
	}
	return orders, total, nil
}

// OrderAdminFilter 管理端工单筛选条件 (5.1)
type OrderAdminFilter struct {
	Status       []string
	Urgency      string
	Keyword      string
	EnterpriseID string // 企业精确筛选 (前端以企业下拉形式使用; 单位审核员必传且限定本单位)
	CategoryID   string // 项目大类精确筛选
	PropertyID   string // 项目属性(问题类型)精确筛选
	DateFrom     *time.Time
	DateTo       *time.Time
	ReporterID   string // 报修人精确筛选
	RepairerID   string // 接单人(维修业务员)精确限定：业务员查看「处理中」仅看自己接的单
	SortBy       string // order_no | enterprise_name | reporter | category_name | urgency | status | submitted_at | created_at
	SortOrder    string // asc | desc
}

// ListForAdmin 管理端分页查询工单 (5.1), 支持多状态/紧急度/关键字/单位/大类/时间范围/报修人筛选
func (r *OrderRepository) ListForAdmin(ctx context.Context, f OrderAdminFilter, offset, limit int) ([]model.RepairOrder, int64, error) {
	base := r.db.WithContext(ctx).Model(&model.RepairOrder{}).
		Joins("JOIN users ON users.id = repair_orders.reporter_id")
	if len(f.Status) > 0 {
		base = base.Where("repair_orders.status IN ?", f.Status)
	}
	if f.Urgency != "" {
		base = base.Where("repair_orders.urgency = ?", f.Urgency)
	}
	if f.Keyword != "" {
		like := "%" + f.Keyword + "%"
		// 工单号检索（前端搜索框仅作工单号搜索；项目/报修人等已有独立下拉筛选）
		base = base.Where("repair_orders.order_no LIKE ?", like)
	}
	if f.EnterpriseID != "" {
		base = base.Where("repair_orders.enterprise_id = ?", f.EnterpriseID)
	}
	if f.CategoryID != "" {
		base = base.Where("repair_orders.category_id = ?", f.CategoryID)
	}
	if f.PropertyID != "" {
		base = base.Where("repair_orders.property_id = ?", f.PropertyID)
	}
	if f.DateFrom != nil {
		base = base.Where("repair_orders.submitted_at >= ?", *f.DateFrom)
	}
	if f.DateTo != nil {
		base = base.Where("repair_orders.submitted_at <= ?", *f.DateTo)
	}
	if f.ReporterID != "" {
		base = base.Where("repair_orders.reporter_id = ?", f.ReporterID)
	}
	if f.RepairerID != "" {
		base = base.Where("repair_orders.repairer_id = ?", f.RepairerID)
	}

	var total int64
	if err := base.Session(&gorm.Session{}).Count(&total).Error; err != nil {
		return nil, 0, err
	}

	sortField, sErr := adminOrderSortField(f.SortBy)
	if sErr != nil {
		return nil, 0, sErr
	}
	if strings.HasPrefix(sortField, "enterprises.") {
		base = base.Joins("JOIN enterprises ON enterprises.id = repair_orders.enterprise_id")
	}
	dir := "DESC"
	if strings.EqualFold(f.SortOrder, "asc") {
		dir = "ASC"
	}

	var orders []model.RepairOrder
	err := base.Preload("Reporter").
		Preload("Enterprise").
		Order(sortField + " " + dir).
		Offset(offset).
		Limit(limit).
		Find(&orders).Error
	if err != nil {
		return nil, 0, err
	}
	return orders, total, nil
}

// ── 5.17 本单位工单汇总统计 (V1.3 C20) ──

// OrderStatsAggregate 汇总统计的行级聚合结果（service 层再补零/组装配 DTO）
type OrderStatsAggregate struct {
	Total        int64
	ByStatus     []OrderStatusCount
	ByCategory   []OrderCategoryCount
	TopReporters []ReporterRank
}

type OrderStatusCount struct {
	Status string
	Count  int64
}

type OrderCategoryCount struct {
	CategoryID   string
	CategoryName string
	Count        int64
}

type ReporterRank struct {
	UserID    string
	Nickname  string
	AvatarURL string
	Count     int64
}

// StatsForAdmin 聚合统计 (5.17)。
// 仅统计六种正式状态 (reported/pending_accept/processing/completed/cancelled/rejected)，
// 草稿 (draft, submitted_at 为空) 天然不计；时间区间作用于 submitted_at（重提会刷新）。
func (r *OrderRepository) StatsForAdmin(ctx context.Context, enterpriseID string, start, end *time.Time) (*OrderStatsAggregate, error) {
	formal := []string{
		string(model.OrderReported),
		string(model.OrderPendingAccept),
		string(model.OrderProcessing),
		string(model.OrderCompleted),
		string(model.OrderCancelled),
		string(model.OrderRejected),
	}
	scope := func(q *gorm.DB) *gorm.DB {
		q = q.Model(&model.RepairOrder{}).Where("status IN ?", formal)
		if enterpriseID != "" {
			q = q.Where("enterprise_id = ?", enterpriseID)
		}
		if start != nil {
			q = q.Where("submitted_at >= ?", *start)
		}
		if end != nil {
			q = q.Where("submitted_at < ?", *end) // 结束时间为开区间
		}
		return q
	}

	agg := &OrderStatsAggregate{}
	if err := scope(r.db.WithContext(ctx)).Count(&agg.Total).Error; err != nil {
		return nil, err
	}

	var byStatus []OrderStatusCount
	if err := scope(r.db.WithContext(ctx)).
		Select("status, COUNT(*) AS count").
		Group("status").
		Order("count DESC").
		Scan(&byStatus).Error; err != nil {
		return nil, err
	}

	// 大类分布按 category_id 归并；大类名取快照（同名历史工单合并在同一类别下）
	var byCategory []OrderCategoryCount
	if err := scope(r.db.WithContext(ctx)).
		Select("category_id, MAX(category_name) AS category_name, COUNT(*) AS count").
		Group("category_id").
		Order("count DESC").
		Scan(&byCategory).Error; err != nil {
		return nil, err
	}

	var topReporters []ReporterRank
	if err := scope(r.db.WithContext(ctx)).
		Joins("JOIN users ON users.id = repair_orders.reporter_id").
		Select("repair_orders.reporter_id AS user_id, COALESCE(NULLIF(users.nickname, ''), '用户') AS nickname, COALESCE(users.avatar_url, '') AS avatar_url, COUNT(*) AS count").
		Group("repair_orders.reporter_id, users.nickname, users.avatar_url").
		Order("count DESC").
		Limit(5).
		Scan(&topReporters).Error; err != nil {
		return nil, err
	}

	agg.ByStatus = byStatus
	agg.ByCategory = byCategory
	agg.TopReporters = topReporters
	return agg, nil
}

// OrderTodayAggregate 今日概况聚合（5.17 data.today）
type OrderTodayAggregate struct {
	PendingReview  int64 // 当前待审核存量（reported，无时间过滤）
	SubmittedToday int64 // 今日新增上报（submitted_at ∈ [dayStart,dayEnd)）
	AuditedToday   int64 // 今日审核通过（timeline action=audit）
	RejectedToday  int64 // 今日退回（timeline action=reject）
}

// StatsToday 今日概况（5.17 data.today）。按服务器本地日 [dayStart,dayEnd) 计；
// 企业域随 enterpriseID（店方空串=全部可见单位）；与 StatsForAdmin 的 start/end 无关。
func (r *OrderRepository) StatsToday(ctx context.Context, enterpriseID string, dayStart, dayEnd time.Time) (*OrderTodayAggregate, error) {
	domain := func(q *gorm.DB) *gorm.DB {
		if enterpriseID != "" {
			q = q.Where("enterprise_id = ?", enterpriseID)
		}
		return q
	}
	agg := &OrderTodayAggregate{}

	if err := domain(r.db.WithContext(ctx).Model(&model.RepairOrder{}).
		Where("status = ?", string(model.OrderReported))).
		Count(&agg.PendingReview).Error; err != nil {
		return nil, err
	}
	if err := domain(r.db.WithContext(ctx).Model(&model.RepairOrder{}).
		Where("submitted_at >= ? AND submitted_at < ?", dayStart, dayEnd)).
		Count(&agg.SubmittedToday).Error; err != nil {
		return nil, err
	}

	type actionCount struct {
		Action string
		Count  int64
	}
	var acts []actionCount
	q := r.db.WithContext(ctx).Model(&model.OrderTimeline{}).
		Joins("JOIN repair_orders ON repair_orders.id = order_timeline.order_id").
		Where("order_timeline.action IN ?", []string{string(model.ActionAudit), string(model.ActionReject)}).
		Where("order_timeline.created_at >= ? AND order_timeline.created_at < ?", dayStart, dayEnd)
	if enterpriseID != "" {
		q = q.Where("repair_orders.enterprise_id = ?", enterpriseID)
	}
	if err := q.Select("order_timeline.action AS action, COUNT(*) AS count").
		Group("order_timeline.action").
		Scan(&acts).Error; err != nil {
		return nil, err
	}
	for _, a := range acts {
		switch a.Action {
		case string(model.ActionAudit):
			agg.AuditedToday = a.Count
		case string(model.ActionReject):
			agg.RejectedToday = a.Count
		}
	}
	return agg, nil
}

// ── 5.20/5.21 统计页二期「纵向双区卡」聚合 (V1.4, 方案 A) ──

// OrderMetricsAggregate 区间运营指标 (5.20 行级)
type OrderMetricsAggregate struct {
	PendingReview int64 // 当前待审核存量（reported，不计时间）
	Submitted     int64 // 区间上报（submitted_at∈窗口，重提刷新）
	Audited       int64 // 区间审核通过（timeline action=audit∈窗口）
	Rejected      int64 // 区间退回（timeline action=reject∈窗口）
}

// StatsMetrics 5.20：待审核存量 + 窗口内 上报/审核通过/退回。
// 时间窗口 start/end（end 开区间，nil=不限）；企业域随 enterpriseID（空串=全部）。
func (r *OrderRepository) StatsMetrics(ctx context.Context, enterpriseID string, start, end *time.Time) (*OrderMetricsAggregate, error) {
	domain := func(q *gorm.DB) *gorm.DB {
		if enterpriseID != "" {
			q = q.Where("enterprise_id = ?", enterpriseID)
		}
		return q
	}
	agg := &OrderMetricsAggregate{}

	// 当前待审核存量（reported，不随时间窗口过滤）
	if err := domain(r.db.WithContext(ctx).Model(&model.RepairOrder{}).
		Where("status = ?", string(model.OrderReported))).
		Count(&agg.PendingReview).Error; err != nil {
		return nil, err
	}

	// 窗口上报：submitted_at∈窗口（草稿无 submitted_at 天然不计）
	sub := domain(r.db.WithContext(ctx).Model(&model.RepairOrder{}))
	if start != nil {
		sub = sub.Where("submitted_at >= ?", *start)
	}
	if end != nil {
		sub = sub.Where("submitted_at < ?", *end)
	}
	if err := sub.Count(&agg.Submitted).Error; err != nil {
		return nil, err
	}

	// 窗口审核通过/退回：timeline action∈{audit,reject} 且 created_at∈窗口（企业域经 repair_orders 联表）
	acts := domain(r.db.WithContext(ctx).Model(&model.OrderTimeline{}).
		Joins("JOIN repair_orders ON repair_orders.id = order_timeline.order_id").
		Where("order_timeline.action IN ?", []string{string(model.ActionAudit), string(model.ActionReject)}))
	if start != nil {
		acts = acts.Where("order_timeline.created_at >= ?", *start)
	}
	if end != nil {
		acts = acts.Where("order_timeline.created_at < ?", *end)
	}
	type actionCount struct {
		Action string
		Count  int64
	}
	var rows []actionCount
	if err := acts.Select("order_timeline.action AS action, COUNT(*) AS count").
		Group("order_timeline.action").
		Scan(&rows).Error; err != nil {
		return nil, err
	}
	for _, a := range rows {
		switch a.Action {
		case string(model.ActionAudit):
			agg.Audited = a.Count
		case string(model.ActionReject):
			agg.Rejected = a.Count
		}
	}
	return agg, nil
}

// RepairerSummaryRow 维修员区间汇总项 (5.21)
type RepairerSummaryRow struct {
	RepairerID   string
	RepairerName string
	Accepted     int64 // 区间接单（accepted_at∈窗口）
	Completed    int64 // 区间完工（completed_at∈窗口）
}

// RepairerSummaryAggregate 维修员区间汇总聚合 (5.21)
type RepairerSummaryAggregate struct {
	GlobalPendingAccept int64
	Rows                []RepairerSummaryRow
}

// StatsRepairerSummary 5.21：全局待接单存量 + 各维修员（或指定单人）的区间接单/完工量。
// 时间窗口作用于各自时间戳列（accepted_at / completed_at）；repairer_id 空=全部。
func (r *OrderRepository) StatsRepairerSummary(ctx context.Context, repairerID string, start, end *time.Time) (*RepairerSummaryAggregate, error) {
	agg := &RepairerSummaryAggregate{}
	// 全局待接单存量（不受所选业务员影响）
	if err := r.db.WithContext(ctx).Model(&model.RepairOrder{}).
		Where("status = ?", string(model.OrderPendingAccept)).
		Count(&agg.GlobalPendingAccept).Error; err != nil {
		return nil, err
	}

	scope := func(q *gorm.DB) *gorm.DB {
		// 始终联表 users（SELECT/GROUP BY 引用 users.nickname）；指定业务员时精确过滤，否则取全部有接单记录的维修员
		q = q.Joins("JOIN users ON users.id = repair_orders.repairer_id")
		if repairerID != "" {
			q = q.Where("repair_orders.repairer_id = ?", repairerID)
		} else {
			q = q.Where("repair_orders.repairer_id IS NOT NULL")
		}
		return q
	}
	window := func(q *gorm.DB, col string) *gorm.DB {
		if start != nil {
			q = q.Where("repair_orders."+col+" >= ?", *start)
		}
		if end != nil {
			q = q.Where("repair_orders."+col+" < ?", *end)
		}
		return q
	}

	sel := "repair_orders.repairer_id AS repairer_id, COALESCE(NULLIF(users.nickname, ''), '维修员') AS name, COUNT(*) AS count"
	gby := "repair_orders.repairer_id, users.nickname"
	type cntRow struct {
		RepairerID string
		Name       string
		Count      int64
	}

	accepted := make(map[string]int64)
	completed := make(map[string]int64)
	names := make(map[string]string)
	var aRows []cntRow
	// 接单：accepted_at 必须非空（累计查询无时间窗口时同样成立）
	if err := window(scope(r.db.WithContext(ctx).Model(&model.RepairOrder{}).
		Where("repair_orders.accepted_at IS NOT NULL")), "accepted_at").
		Select(sel).Group(gby).Scan(&aRows).Error; err != nil {
		return nil, err
	}
	var cRows []cntRow
	// 完工：completed_at 必须非空
	if err := window(scope(r.db.WithContext(ctx).Model(&model.RepairOrder{}).
		Where("repair_orders.completed_at IS NOT NULL")), "completed_at").
		Select(sel).Group(gby).Scan(&cRows).Error; err != nil {
		return nil, err
	}
	for _, row := range aRows {
		accepted[row.RepairerID] = row.Count
		names[row.RepairerID] = row.Name
	}
	for _, row := range cRows {
		completed[row.RepairerID] = row.Count
		if _, ok := names[row.RepairerID]; !ok {
			names[row.RepairerID] = row.Name
		}
	}
	rows := make([]RepairerSummaryRow, 0, len(names))
	for id, name := range names {
		rows = append(rows, RepairerSummaryRow{RepairerID: id, RepairerName: name, Accepted: accepted[id], Completed: completed[id]})
	}
	sort.Slice(rows, func(i, j int) bool {
		if rows[i].Accepted != rows[j].Accepted {
			return rows[i].Accepted > rows[j].Accepted
		}
		return rows[i].Completed > rows[j].Completed
	})
	agg.Rows = rows
	return agg, nil
}

// RepairerOverviewAggregate 维修员个人汇总 (5.22，小程序「处理工单」统计卡)
type RepairerOverviewAggregate struct {
	PendingAccept  int64 // 可接单存量（pending_accept，可随企业域）
	MyAccepted     int64 // 我的累计接单（accepted_at 非空）
	MyProcessing   int64 // 处理中（status=processing 且接单人为我）
	MyCompleted    int64 // 累计完工（completed_at 非空）
	TodayAccepted  int64 // 今日接单
	TodayCompleted int64 // 今日完工
}

// StatsRepairerOverview 5.22：维修员个人汇总（累计 + 今日双行）。
// repairerID 为空时由 service 层取当前操作者；enterpriseID 空串=全部企业域。
func (r *OrderRepository) StatsRepairerOverview(ctx context.Context, repairerID, enterpriseID string, dayStart, dayEnd time.Time) (*RepairerOverviewAggregate, error) {
	agg := &RepairerOverviewAggregate{}

	// 可接单存量（待接单；企业域可选）
	pend := r.db.WithContext(ctx).Model(&model.RepairOrder{}).
		Where("status = ?", string(model.OrderPendingAccept))
	if enterpriseID != "" {
		pend = pend.Where("enterprise_id = ?", enterpriseID)
	}
	if err := pend.Count(&agg.PendingAccept).Error; err != nil {
		return nil, err
	}

	// 我的订单基域（接单人=我；企业域可选）
	base := func() *gorm.DB {
		q := r.db.WithContext(ctx).Model(&model.RepairOrder{}).Where("repairer_id = ?", repairerID)
		if enterpriseID != "" {
			q = q.Where("enterprise_id = ?", enterpriseID)
		}
		return q
	}
	if err := base().Where("accepted_at IS NOT NULL").Count(&agg.MyAccepted).Error; err != nil {
		return nil, err
	}
	if err := base().Where("status = ?", string(model.OrderProcessing)).Count(&agg.MyProcessing).Error; err != nil {
		return nil, err
	}
	if err := base().Where("completed_at IS NOT NULL").Count(&agg.MyCompleted).Error; err != nil {
		return nil, err
	}
	if err := base().Where("accepted_at >= ? AND accepted_at < ?", dayStart, dayEnd).Count(&agg.TodayAccepted).Error; err != nil {
		return nil, err
	}
	if err := base().Where("completed_at >= ? AND completed_at < ?", dayStart, dayEnd).Count(&agg.TodayCompleted).Error; err != nil {
		return nil, err
	}
	return agg, nil
}

// ── 5.18/5.19 独立统计页聚合 (V1.3, 仅店方/超管) ──

// formalOrderStatuses 六种正式状态（统计口径：草稿天然不计）。
func formalOrderStatuses() []string {
	return []string{
		string(model.OrderReported),
		string(model.OrderPendingAccept),
		string(model.OrderProcessing),
		string(model.OrderCompleted),
		string(model.OrderCancelled),
		string(model.OrderRejected),
	}
}

// EnterpriseOrderStat 企业维度分组聚合项 (5.18 行级)
type EnterpriseOrderStat struct {
	EnterpriseID   string
	EnterpriseName string
	Total          int64
	ByStatus       []OrderStatusCount // 仅含出现过的状态（service 层补零六态）
}

// StatsByEnterprise 企业维度分组聚合 (5.18)。
// 仅店方/超管使用（调用方已鉴权）；口径同 5.17：六正式态、submitted_at 区间、enterprise 域（空串=全部）。
func (r *OrderRepository) StatsByEnterprise(ctx context.Context, enterpriseID string, start, end *time.Time) ([]EnterpriseOrderStat, error) {
	type entStatusRow struct {
		EnterpriseID   string
		EnterpriseName string
		Status         string
		Count          int64
	}
	base := r.db.WithContext(ctx).Model(&model.RepairOrder{}).
		Joins("JOIN enterprises ON enterprises.id = repair_orders.enterprise_id").
		Where("repair_orders.status IN ?", formalOrderStatuses())
	if enterpriseID != "" {
		base = base.Where("repair_orders.enterprise_id = ?", enterpriseID)
	}
	if start != nil {
		base = base.Where("repair_orders.submitted_at >= ?", *start)
	}
	if end != nil {
		base = base.Where("repair_orders.submitted_at < ?", *end)
	}

	var rows []entStatusRow
	if err := base.
		Select("repair_orders.enterprise_id AS enterprise_id, MAX(enterprises.name) AS enterprise_name, repair_orders.status AS status, COUNT(*) AS count").
		Group("repair_orders.enterprise_id, repair_orders.status").
		Scan(&rows).Error; err != nil {
		return nil, err
	}

	byEnt := make(map[string]*EnterpriseOrderStat, len(rows))
	for _, row := range rows {
		st := byEnt[row.EnterpriseID]
		if st == nil {
			st = &EnterpriseOrderStat{EnterpriseID: row.EnterpriseID, EnterpriseName: row.EnterpriseName}
			byEnt[row.EnterpriseID] = st
		}
		st.Total += row.Count
		st.ByStatus = append(st.ByStatus, OrderStatusCount{Status: row.Status, Count: row.Count})
	}
	list := make([]EnterpriseOrderStat, 0, len(byEnt))
	for _, st := range byEnt {
		list = append(list, *st)
	}
	// 按工单量降序；同量按企业名升序
	sort.Slice(list, func(i, j int) bool {
		if list[i].Total != list[j].Total {
			return list[i].Total > list[j].Total
		}
		return list[i].EnterpriseName < list[j].EnterpriseName
	})
	return list, nil
}

// RepairerOrderStat 维修员业绩聚合项 (5.19 行级)
type RepairerOrderStat struct {
	RepairerID   string
	RepairerName string
	Assigned     int64 // 接单/处理量：repairer_id 非空且 status∈{pending_accept,processing,completed}
	Completed    int64 // 完工量：completed_at 非空
}

// StatsByRepairer 维修员维度聚合 (5.19)。
// 仅店方/超管使用（调用方已鉴权）；口径同 5.17：submitted_at 区间、enterprise 域（空串=全部）。
func (r *OrderRepository) StatsByRepairer(ctx context.Context, enterpriseID string, start, end *time.Time) ([]RepairerOrderStat, error) {
	scope := func(q *gorm.DB) *gorm.DB {
		q = q.Joins("JOIN users ON users.id = repair_orders.repairer_id").
			Where("repair_orders.repairer_id IS NOT NULL")
		if enterpriseID != "" {
			q = q.Where("repair_orders.enterprise_id = ?", enterpriseID)
		}
		if start != nil {
			q = q.Where("repair_orders.submitted_at >= ?", *start)
		}
		if end != nil {
			q = q.Where("repair_orders.submitted_at < ?", *end)
		}
		return q
	}

	type cntRow struct {
		RepairerID string
		Name       string
		Count      int64
	}
	selectCols := "repair_orders.repairer_id AS repairer_id, COALESCE(NULLIF(users.nickname, ''), '维修员') AS name, COUNT(*) AS count"
	groupBy := "repair_orders.repairer_id, users.nickname"

	// 接单/处理量：repairer_id 非空 且 status ∈ {pending_accept, processing, completed}
	var aRows []cntRow
	if err := scope(r.db.WithContext(ctx).Model(&model.RepairOrder{})).
		Where("repair_orders.status IN ?", []string{
			string(model.OrderPendingAccept),
			string(model.OrderProcessing),
			string(model.OrderCompleted),
		}).
		Select(selectCols).
		Group(groupBy).
		Scan(&aRows).Error; err != nil {
		return nil, err
	}
	// 完工量：completed_at 非空
	var cRows []cntRow
	if err := scope(r.db.WithContext(ctx).Model(&model.RepairOrder{})).
		Where("repair_orders.completed_at IS NOT NULL").
		Select(selectCols).
		Group(groupBy).
		Scan(&cRows).Error; err != nil {
		return nil, err
	}

	assigned := make(map[string]int64, len(aRows))
	completed := make(map[string]int64, len(cRows))
	names := make(map[string]string, len(aRows)+len(cRows))
	for _, row := range aRows {
		assigned[row.RepairerID] = row.Count
		names[row.RepairerID] = row.Name
	}
	for _, row := range cRows {
		completed[row.RepairerID] = row.Count
		if _, ok := names[row.RepairerID]; !ok {
			names[row.RepairerID] = row.Name
		}
	}

	list := make([]RepairerOrderStat, 0, len(names))
	for id, name := range names {
		list = append(list, RepairerOrderStat{
			RepairerID:   id,
			RepairerName: name,
			Assigned:     assigned[id],
			Completed:    completed[id],
		})
	}
	// 按接单量降序；同量按完工量降序
	sort.Slice(list, func(i, j int) bool {
		if list[i].Assigned != list[j].Assigned {
			return list[i].Assigned > list[j].Assigned
		}
		return list[i].Completed > list[j].Completed
	})
	return list, nil
}

// ReporterOption 报修人下拉选项（管理列表"按报修人筛选"，来源于历史工单的报修人）
type ReporterOption struct {
	UserID    string
	Nickname  string
	AvatarURL string
}

// ReporterOptions 查询报修人候选（5.1 配套）：正式工单报修人去重，支持企业域与昵称模糊，按昵称升序，上限 limit。
func (r *OrderRepository) ReporterOptions(ctx context.Context, enterpriseID, keyword string, limit int) ([]ReporterOption, error) {
	q := r.db.WithContext(ctx).Model(&model.RepairOrder{}).
		Joins("JOIN users ON users.id = repair_orders.reporter_id").
		Where("repair_orders.status IN ?", formalOrderStatuses())
	if enterpriseID != "" {
		q = q.Where("repair_orders.enterprise_id = ?", enterpriseID)
	}
	if keyword != "" {
		q = q.Where("users.nickname LIKE ?", "%"+keyword+"%")
	}
	if limit <= 0 {
		limit = 200
	}
	var rows []ReporterOption
	if err := q.
		Select("repair_orders.reporter_id AS user_id, COALESCE(NULLIF(users.nickname, ''), '用户') AS nickname, COALESCE(users.avatar_url, '') AS avatar_url").
		Group("repair_orders.reporter_id, users.nickname, users.avatar_url").
		Order("nickname ASC").
		Limit(limit).
		Scan(&rows).Error; err != nil {
		return nil, err
	}
	return rows, nil
}

// adminOrderSortField 管理端排序字段白名单 → SQL 列 (防注入)
func adminOrderSortField(sortBy string) (string, error) {
	switch sortBy {
	case "", "submitted_at":
		return "repair_orders.submitted_at", nil
	case "order_no":
		return "repair_orders.order_no", nil
	case "enterprise_name":
		return "enterprises.name", nil
	case "reporter":
		return "users.nickname", nil
	case "category_name":
		return "repair_orders.category_name", nil
	case "urgency":
		return "repair_orders.urgency", nil
	case "status":
		return "repair_orders.status", nil
	case "created_at":
		return "repair_orders.created_at", nil
	default:
		return "", fmt.Errorf("invalid sort_by: %s", sortBy)
	}
}

// ListForExportByEnterprise 企业模式下导出 (5.14): 按 enterprise_id + 完工时间范围查询, 完工时间正序
func (r *OrderRepository) ListForExportByEnterprise(ctx context.Context, enterpriseID, status string, from, to time.Time) ([]model.RepairOrder, error) {
	base := r.db.WithContext(ctx).Model(&model.RepairOrder{}).
		Where("enterprise_id = ? AND completed_at IS NOT NULL", enterpriseID).
		Where("completed_at >= ? AND completed_at <= ?", from, to)
	if status != "" {
		base = base.Where("status = ?", status)
	}
	var orders []model.RepairOrder
	err := base.Preload("Reporter").
		Preload("Enterprise").
		Order("completed_at ASC").
		Find(&orders).Error
	if err != nil {
		return nil, err
	}
	return orders, nil
}

// ListForExportByRepairer 业务员模式下导出 (5.14): 优先按 repairer_id(接单时绑定) 关联,
// 历史工单 (repairer_id 为空) 回退按时间轴 complete 操作人关联; 完工时间正序
func (r *OrderRepository) ListForExportByRepairer(ctx context.Context, repairerID, status string, from, to time.Time) ([]model.RepairOrder, error) {
	subComplete := r.db.Model(&model.OrderTimeline{}).
		Select("DISTINCT order_id").
		Where("operator_id = ? AND action = ?", repairerID, string(model.ActionComplete))
	base := r.db.WithContext(ctx).Model(&model.RepairOrder{}).
		Where("completed_at IS NOT NULL").
		Where("repairer_id = ? OR id IN (?)", repairerID, subComplete).
		Where("completed_at >= ? AND completed_at <= ?", from, to)
	if status != "" {
		base = base.Where("status = ?", status)
	}
	var orders []model.RepairOrder
	err := base.Preload("Reporter").
		Preload("Enterprise").
		Order("enterprise_id ASC, completed_at ASC").
		Find(&orders).Error
	if err != nil {
		return nil, err
	}
	return orders, nil
}

// ListRepairers 查询全部维修员(平台管理员及超级管理员, role>=1), 按昵称排序 (5.15)
func (r *OrderRepository) ListRepairers(ctx context.Context) ([]model.User, error) {
	var users []model.User
	err := r.db.WithContext(ctx).
		Where("role >= ?", model.PlatformRolePlatformAdmin).
		Order("nickname ASC").
		Find(&users).Error
	if err != nil {
		return nil, err
	}
	return users, nil
}

// GenerateOrderNo 生成当日工单号: XNB-{YYYYMMDD}-{3位当日流水号} (提交上报时调用)
// 示例: XNB-20260827-001; 草稿阶段 order_no 为空; 每日从 001 重新开始计数
func (r *OrderRepository) GenerateOrderNo(ctx context.Context, now time.Time) (string, error) {
	prefix := fmt.Sprintf("XNB-%s-", now.Format("20060102"))

	var maxNo string
	err := r.db.WithContext(ctx).Model(&model.RepairOrder{}).
		Where("order_no LIKE ?", prefix+"%").
		Order("order_no DESC").
		Limit(1).
		Pluck("order_no", &maxNo).Error
	if err != nil {
		return "", err
	}

	seq := 1
	if len(maxNo) == len(prefix)+3 {
		if _, err := fmt.Sscanf(maxNo[len(prefix):], "%03d", &seq); err == nil {
			seq++
		}
	}
	return fmt.Sprintf("%s%03d", prefix, seq), nil
}

// ────────────────────────────────────────────
// 工单图片数据访问
// ────────────────────────────────────────────

// OrderImageRepository 工单图片数据访问
type OrderImageRepository struct {
	db *gorm.DB
}

// NewOrderImageRepository 创建 OrderImageRepository
func NewOrderImageRepository(db *gorm.DB) *OrderImageRepository {
	return &OrderImageRepository{db: db}
}

// Create 创建图片记录 (status=temporary)
func (r *OrderImageRepository) Create(ctx context.Context, img *model.OrderImage) error {
	return r.db.WithContext(ctx).Create(img).Error
}

// FindByOrderAndURL 按工单与 URL 查询图片记录 (含已删除), 不存在时返回 nil
func (r *OrderImageRepository) FindByOrderAndURL(ctx context.Context, orderID, url string) (*model.OrderImage, error) {
	var img model.OrderImage
	err := r.db.WithContext(ctx).
		Where("order_id = ? AND image_url = ?", orderID, url).
		First(&img).Error
	if err == gorm.ErrRecordNotFound {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &img, nil
}

// ListActiveByOrder 查询工单下指定类型的 active 图片 (按 sort_order 升序)
func (r *OrderImageRepository) ListActiveByOrder(ctx context.Context, orderID, imageType string) ([]model.OrderImage, error) {
	var images []model.OrderImage
	err := r.db.WithContext(ctx).
		Where("order_id = ? AND image_type = ? AND status = ?", orderID, imageType, model.ImageActive).
		Order("sort_order ASC").
		Find(&images).Error
	if err != nil {
		return nil, err
	}
	return images, nil
}

// ListAllByOrder 查询工单下指定类型的全部图片 (含 temporary/deleted, 供全量替换比对)
func (r *OrderImageRepository) ListAllByOrder(ctx context.Context, orderID, imageType string) ([]model.OrderImage, error) {
	var images []model.OrderImage
	err := r.db.WithContext(ctx).
		Where("order_id = ? AND image_type = ?", orderID, imageType).
		Find(&images).Error
	if err != nil {
		return nil, err
	}
	return images, nil
}

// Update 更新图片记录
func (r *OrderImageRepository) Update(ctx context.Context, img *model.OrderImage) error {
	return r.db.WithContext(ctx).Save(img).Error
}

// CountNotDeleted 统计工单下指定类型未删除 (status != deleted) 的图片数
func (r *OrderImageRepository) CountNotDeleted(ctx context.Context, orderID, imageType string) (int64, error) {
	var count int64
	err := r.db.WithContext(ctx).Model(&model.OrderImage{}).
		Where("order_id = ? AND image_type = ? AND status <> ?", orderID, imageType, model.ImageDeleted).
		Count(&count).Error
	return count, err
}

// MarkAllDeleted 将工单下指定类型未删除的图片全部标记为 deleted
func (r *OrderImageRepository) MarkAllDeleted(ctx context.Context, orderID, imageType string) error {
	return r.db.WithContext(ctx).Model(&model.OrderImage{}).
		Where("order_id = ? AND image_type = ? AND status <> ?", orderID, imageType, model.ImageDeleted).
		Update("status", model.ImageDeleted).Error
}

// CountActiveByOrders 统计多个工单下指定类型的 active 图片数, 返回 map[orderID]count (5.1 image_count)
func (r *OrderImageRepository) CountActiveByOrders(ctx context.Context, orderIDs []string, imageType string) (map[string]int64, error) {
	result := make(map[string]int64, len(orderIDs))
	if len(orderIDs) == 0 {
		return result, nil
	}
	var rows []struct {
		OrderID string
		Cnt     int64
	}
	err := r.db.WithContext(ctx).Table("order_images").
		Select("order_id, COUNT(*) AS cnt").
		Where("order_id IN ? AND image_type = ? AND status = ?", orderIDs, imageType, model.ImageActive).
		Group("order_id").
		Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	for _, row := range rows {
		result[row.OrderID] = row.Cnt
	}
	return result, nil
}

// ListActiveByOrders 批量查询多个工单的 active 图片 (按 sort_order 升序), 返回 map[orderID][]OrderImage
// V1.3: 供管理端列表卡片展示故障图缩略图
func (r *OrderImageRepository) ListActiveByOrders(ctx context.Context, orderIDs []string, imageType string) (map[string][]model.OrderImage, error) {
	result := make(map[string][]model.OrderImage, len(orderIDs))
	if len(orderIDs) == 0 {
		return result, nil
	}
	var images []model.OrderImage
	err := r.db.WithContext(ctx).
		Where("order_id IN ? AND image_type = ? AND status = ?", orderIDs, imageType, model.ImageActive).
		Order("order_id ASC, sort_order ASC").
		Find(&images).Error
	if err != nil {
		return nil, err
	}
	for _, img := range images {
		result[img.OrderID] = append(result[img.OrderID], img)
	}
	return result, nil
}

// ────────────────────────────────────────────
// 工单时间轴数据访问
// ────────────────────────────────────────────

// OrderTimelineRepository 工单时间轴数据访问
type OrderTimelineRepository struct {
	db *gorm.DB
}

// NewOrderTimelineRepository 创建 OrderTimelineRepository
func NewOrderTimelineRepository(db *gorm.DB) *OrderTimelineRepository {
	return &OrderTimelineRepository{db: db}
}

// Create 写入时间轴记录
func (r *OrderTimelineRepository) Create(ctx context.Context, tl *model.OrderTimeline) error {
	return r.db.WithContext(ctx).Create(tl).Error
}

// ListByOrder 查询工单时间轴 (按时间正序), 预加载操作人
func (r *OrderTimelineRepository) ListByOrder(ctx context.Context, orderID string) ([]model.OrderTimeline, error) {
	var timelines []model.OrderTimeline
	err := r.db.WithContext(ctx).
		Where("order_id = ?", orderID).
		Preload("Operator").
		Order("created_at ASC").
		Find(&timelines).Error
	if err != nil {
		return nil, err
	}
	return timelines, nil
}

// CompleteOperatorsByOrders 批量查询多个工单的完工(complete)操作人昵称, 返回 map[orderID]nickname (5.14 维修员列)
func (r *OrderTimelineRepository) CompleteOperatorsByOrders(ctx context.Context, orderIDs []string) (map[string]string, error) {
	result := make(map[string]string, len(orderIDs))
	if len(orderIDs) == 0 {
		return result, nil
	}
	var rows []struct {
		OrderID  string
		Nickname string
	}
	err := r.db.WithContext(ctx).
		Table("order_timeline").
		Select("order_timeline.order_id, users.nickname").
		Joins("JOIN users ON users.id = order_timeline.operator_id").
		Where("order_timeline.order_id IN ? AND order_timeline.action = ?", orderIDs, string(model.ActionComplete)).
		Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	for _, row := range rows {
		if _, ok := result[row.OrderID]; !ok {
			result[row.OrderID] = row.Nickname
		}
	}
	return result, nil
}
