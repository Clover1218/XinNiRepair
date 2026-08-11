package repository

import (
	"context"
	"fmt"
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

// FindByID 按 ID 查询工单 (含企业关联), 不存在时返回 nil
func (r *OrderRepository) FindByID(ctx context.Context, id string) (*model.RepairOrder, error) {
	var order model.RepairOrder
	err := r.db.WithContext(ctx).Preload("Enterprise").Where("id = ?", id).First(&order).Error
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
	EnterpriseID string // 企业精确筛选
	OrderNo      string // 工单号模糊筛选
	ProjectName  string // 项目名称模糊筛选
	DateFrom     *time.Time
	DateTo       *time.Time
	ReporterID   string
	SortBy       string // order_no | enterprise_name | reporter | project_name | urgency | status | submitted_at | created_at
	SortOrder    string // asc | desc
}

// ListForAdmin 管理端分页查询工单 (5.1), 支持多状态/紧急度/关键字/企业/工单号/项目名/时间范围/报修人筛选
func (r *OrderRepository) ListForAdmin(ctx context.Context, f OrderAdminFilter, offset, limit int) ([]model.RepairOrder, int64, error) {
	base := r.db.WithContext(ctx).Model(&model.RepairOrder{}).
		Joins("JOIN users ON users.id = repair_orders.reporter_id").
		Joins("LEFT JOIN enterprises ON enterprises.id = repair_orders.enterprise_id")
	if len(f.Status) > 0 {
		base = base.Where("repair_orders.status IN ?", f.Status)
	}
	if f.Urgency != "" {
		base = base.Where("repair_orders.urgency = ?", f.Urgency)
	}
	if f.Keyword != "" {
		like := "%" + f.Keyword + "%"
		base = base.Where("repair_orders.order_no LIKE ? OR repair_orders.project_name LIKE ? OR users.nickname LIKE ?",
			like, like, like)
	}
	if f.EnterpriseID != "" {
		base = base.Where("repair_orders.enterprise_id = ?", f.EnterpriseID)
	}
	if f.OrderNo != "" {
		base = base.Where("repair_orders.order_no LIKE ?", "%"+f.OrderNo+"%")
	}
	if f.ProjectName != "" {
		base = base.Where("repair_orders.project_name LIKE ?", "%"+f.ProjectName+"%")
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

	var total int64
	if err := base.Session(&gorm.Session{}).Count(&total).Error; err != nil {
		return nil, 0, err
	}

	sortField := "repair_orders.submitted_at" // 默认按提交时间
	switch f.SortBy {
	case "order_no":
		sortField = "repair_orders.order_no"
	case "enterprise_name":
		sortField = "enterprises.name"
	case "reporter":
		sortField = "users.nickname"
	case "project_name":
		sortField = "repair_orders.project_name"
	case "urgency":
		sortField = "repair_orders.urgency"
	case "status":
		sortField = "repair_orders.status"
	case "created_at":
		sortField = "repair_orders.created_at"
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

// ListForExportByRepairer 业务员模式下导出 (5.14): repairer_id 优先, 历史空值回退时间轴 complete 操作人, 完工时间正序
func (r *OrderRepository) ListForExportByRepairer(ctx context.Context, repairerID, status string, from, to time.Time) ([]model.RepairOrder, error) {
	sub := r.db.Model(&model.OrderTimeline{}).
		Select("DISTINCT order_id").
		Where("operator_id = ? AND action = ?", repairerID, string(model.ActionComplete))
	base := r.db.WithContext(ctx).Model(&model.RepairOrder{}).
		Where("(repair_orders.repairer_id = ? OR (repair_orders.repairer_id IS NULL AND repair_orders.id IN (?))) AND repair_orders.completed_at IS NOT NULL", repairerID, sub).
		Where("repair_orders.completed_at >= ? AND repair_orders.completed_at <= ?", from, to)
	if status != "" {
		base = base.Where("status = ?", status)
	}
	var orders []model.RepairOrder
	err := base.Preload("Reporter").
		Preload("Enterprise").
		Preload("Repairer").
		Order("enterprise_id ASC, completed_at ASC").
		Find(&orders).Error
	if err != nil {
		return nil, err
	}
	return orders, nil
}

// ListRepairers 查询全部维修员(平台管理员, users.role=1), 按昵称排序 (5.15)
func (r *OrderRepository) ListRepairers(ctx context.Context) ([]model.User, error) {
	var users []model.User
	err := r.db.WithContext(ctx).
		Where("role = ?", model.PlatformRolePlatformAdmin).
		Order("nickname ASC").
		Find(&users).Error
	if err != nil {
		return nil, err
	}
	return users, nil
}

// GenerateOrderNo 生成当日工单号: XNB-{YYYYMMDD}-{3位当日流水号} (提交上报时调用)
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
