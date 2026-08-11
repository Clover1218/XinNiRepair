package repository

import (
	"context"

	"gorm.io/gorm"

	"xin-ni-repair/internal/model"
)

// ────────────────────────────────────────────
// 企业数据访问
// ────────────────────────────────────────────

// EnterpriseRepository 企业数据访问
type EnterpriseRepository struct {
	db *gorm.DB
}

// NewEnterpriseRepository 创建 EnterpriseRepository
func NewEnterpriseRepository(db *gorm.DB) *EnterpriseRepository {
	return &EnterpriseRepository{db: db}
}

// Create 创建企业
func (r *EnterpriseRepository) Create(ctx context.Context, ent *model.Enterprise) error {
	return r.db.WithContext(ctx).Create(ent).Error
}

// FindByID 按 ID 查询企业, 不存在时返回 nil
func (r *EnterpriseRepository) FindByID(ctx context.Context, id string) (*model.Enterprise, error) {
	var ent model.Enterprise
	err := r.db.WithContext(ctx).Where("id = ?", id).First(&ent).Error
	if err == gorm.ErrRecordNotFound {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &ent, nil
}

// FindByName 按名称查询企业, 不存在时返回 nil
func (r *EnterpriseRepository) FindByName(ctx context.Context, name string) (*model.Enterprise, error) {
	var ent model.Enterprise
	err := r.db.WithContext(ctx).Where("name = ?", name).First(&ent).Error
	if err == gorm.ErrRecordNotFound {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &ent, nil
}

// FindByInviteCode 按邀请码查询企业, 不存在时返回 nil
func (r *EnterpriseRepository) FindByInviteCode(ctx context.Context, inviteCode string) (*model.Enterprise, error) {
	var ent model.Enterprise
	err := r.db.WithContext(ctx).Where("invite_code = ?", inviteCode).First(&ent).Error
	if err == gorm.ErrRecordNotFound {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &ent, nil
}

// ListForAdmin 管理端分页查询企业 (5.8), keyword 企业名模糊, status 为 1=active/0=inactive
func (r *EnterpriseRepository) ListForAdmin(ctx context.Context, keyword string, status *int, offset, limit int) ([]model.Enterprise, int64, error) {
	base := r.db.WithContext(ctx).Model(&model.Enterprise{})
	if keyword != "" {
		base = base.Where("name LIKE ?", "%"+keyword+"%")
	}
	if status != nil {
		base = base.Where("status = ?", *status)
	}

	var total int64
	if err := base.Session(&gorm.Session{}).Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var enterprises []model.Enterprise
	err := base.Order("created_at DESC").
		Offset(offset).
		Limit(limit).
		Find(&enterprises).Error
	if err != nil {
		return nil, 0, err
	}
	return enterprises, total, nil
}

// EnterpriseStats 企业统计 (5.8/5.9)
type EnterpriseStats struct {
	MemberCount  int64
	OrderCount   int64
	PendingCount int64
}

// CountStats 批量统计企业的成员数/工单数/待审核申请数
func (r *EnterpriseRepository) CountStats(ctx context.Context, enterpriseIDs []string) (map[string]EnterpriseStats, error) {
	result := make(map[string]EnterpriseStats, len(enterpriseIDs))
	if len(enterpriseIDs) == 0 {
		return result, nil
	}
	for _, id := range enterpriseIDs {
		result[id] = EnterpriseStats{}
	}

	type countRow struct {
		EnterpriseID string
		Cnt          int64
	}

	// 已通过成员数
	var mRows []countRow
	if err := r.db.WithContext(ctx).Table("memberships").
		Select("enterprise_id, COUNT(*) AS cnt").
		Where("enterprise_id IN ? AND status = ?", enterpriseIDs, model.MemberApproved).
		Group("enterprise_id").
		Scan(&mRows).Error; err != nil {
		return nil, err
	}
	for _, row := range mRows {
		s := result[row.EnterpriseID]
		s.MemberCount = row.Cnt
		result[row.EnterpriseID] = s
	}

	// 工单数
	var oRows []countRow
	if err := r.db.WithContext(ctx).Table("repair_orders").
		Select("enterprise_id, COUNT(*) AS cnt").
		Where("enterprise_id IN ?", enterpriseIDs).
		Group("enterprise_id").
		Scan(&oRows).Error; err != nil {
		return nil, err
	}
	for _, row := range oRows {
		s := result[row.EnterpriseID]
		s.OrderCount = row.Cnt
		result[row.EnterpriseID] = s
	}

	// 待审核申请数
	var pRows []countRow
	if err := r.db.WithContext(ctx).Table("memberships").
		Select("enterprise_id, COUNT(*) AS cnt").
		Where("enterprise_id IN ? AND status = ?", enterpriseIDs, model.MemberPending).
		Group("enterprise_id").
		Scan(&pRows).Error; err != nil {
		return nil, err
	}
	for _, row := range pRows {
		s := result[row.EnterpriseID]
		s.PendingCount = row.Cnt
		result[row.EnterpriseID] = s
	}

	return result, nil
}

// ExistsInviteCode 判断邀请码是否已被占用 (excludeID 用于排除自身)
func (r *EnterpriseRepository) ExistsInviteCode(ctx context.Context, code, excludeID string) (bool, error) {
	q := r.db.WithContext(ctx).Model(&model.Enterprise{}).Where("invite_code = ?", code)
	if excludeID != "" {
		q = q.Where("id <> ?", excludeID)
	}
	var count int64
	if err := q.Count(&count).Error; err != nil {
		return false, err
	}
	return count > 0, nil
}

// Update 更新企业全部可变字段
func (r *EnterpriseRepository) Update(ctx context.Context, ent *model.Enterprise) error {
	return r.db.WithContext(ctx).Save(ent).Error
}

// ────────────────────────────────────────────
// 成员关系数据访问
// ────────────────────────────────────────────

// MembershipRepository 成员关系数据访问
type MembershipRepository struct {
	db *gorm.DB
}

// NewMembershipRepository 创建 MembershipRepository
func NewMembershipRepository(db *gorm.DB) *MembershipRepository {
	return &MembershipRepository{db: db}
}

// Create 创建成员关系
func (r *MembershipRepository) Create(ctx context.Context, m *model.Membership) error {
	return r.db.WithContext(ctx).Create(m).Error
}

// FindByEnterpriseAndUser 查询企业内指定用户的成员关系, 不存在时返回 nil
func (r *MembershipRepository) FindByEnterpriseAndUser(ctx context.Context, enterpriseID, userID string) (*model.Membership, error) {
	var m model.Membership
	err := r.db.WithContext(ctx).
		Where("enterprise_id = ? AND user_id = ?", enterpriseID, userID).
		First(&m).Error
	if err == gorm.ErrRecordNotFound {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &m, nil
}

// FindApprovedByUser 查询用户已通过审核的成员关系 (含企业信息)
func (r *MembershipRepository) FindApprovedByUser(ctx context.Context, userID string) ([]model.Membership, error) {
	var memberships []model.Membership
	err := r.db.WithContext(ctx).
		Where("user_id = ? AND status = ?", userID, model.MemberApproved).
		Preload("Enterprise").
		Order("created_at ASC").
		Find(&memberships).Error
	if err != nil {
		return nil, err
	}
	return memberships, nil
}

// FindByEnterpriseAndUsers 查询企业内多个用户的成员关系
func (r *MembershipRepository) FindByEnterpriseAndUsers(ctx context.Context, enterpriseID string, userIDs []string) ([]model.Membership, error) {
	var memberships []model.Membership
	err := r.db.WithContext(ctx).
		Where("enterprise_id = ? AND user_id IN ?", enterpriseID, userIDs).
		Find(&memberships).Error
	if err != nil {
		return nil, err
	}
	return memberships, nil
}

// Update 更新成员关系
func (r *MembershipRepository) Update(ctx context.Context, m *model.Membership) error {
	return r.db.WithContext(ctx).Save(m).Error
}

// CountApproved 统计企业内已通过成员数
func (r *MembershipRepository) CountApproved(ctx context.Context, enterpriseID string) (int64, error) {
	var count int64
	err := r.db.WithContext(ctx).Model(&model.Membership{}).
		Where("enterprise_id = ? AND status = ?", enterpriseID, model.MemberApproved).
		Count(&count).Error
	return count, err
}

// ListByEnterprise 分页查询企业成员, 支持状态/角色筛选与昵称/手机号模糊搜索
func (r *MembershipRepository) ListByEnterprise(ctx context.Context, enterpriseID, status, role string, keyword string, offset, limit int) ([]model.Membership, int64, error) {
	base := r.db.WithContext(ctx).
		Table("memberships").
		Joins("JOIN users ON users.id = memberships.user_id").
		Where("memberships.enterprise_id = ?", enterpriseID)
	if status != "" {
		base = base.Where("memberships.status = ?", status)
	}
	if role != "" {
		roleVal := model.EnterpriseRoleMember // member 默认 0
		if role == "admin" {
			roleVal = model.EnterpriseRoleAdmin
		}
		base = base.Where("memberships.role = ?", roleVal)
	}
	if keyword != "" {
		like := "%" + keyword + "%"
		base = base.Where("users.nickname LIKE ? OR users.phone LIKE ?", like, like)
	}

	var total int64
	if err := base.Session(&gorm.Session{}).Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var memberships []model.Membership
	err := base.Preload("User").
		Order("memberships.created_at ASC").
		Offset(offset).
		Limit(limit).
		Find(&memberships).Error
	if err != nil {
		return nil, 0, err
	}
	return memberships, total, nil
}

// orderCountRow 报修单统计行
type orderCountRow struct {
	ReporterID string
	Cnt        int64
}

// CountOrdersByReporter 统计用户在指定企业下的报修单数
func (r *MembershipRepository) CountOrdersByReporter(ctx context.Context, enterpriseID string, userIDs []string) (map[string]int64, error) {
	result := make(map[string]int64, len(userIDs))
	if len(userIDs) == 0 {
		return result, nil
	}
	var rows []orderCountRow
	err := r.db.WithContext(ctx).Table("repair_orders").
		Select("reporter_id, COUNT(*) AS cnt").
		Where("enterprise_id = ? AND reporter_id IN ?", enterpriseID, userIDs).
		Group("reporter_id").
		Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	for _, row := range rows {
		result[row.ReporterID] = row.Cnt
	}
	return result, nil
}
