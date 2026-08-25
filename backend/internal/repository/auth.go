package repository

import (
	"context"

	"gorm.io/gorm"

	"xin-ni-repair/internal/model"
)

// AuthRepository 用户与成员关系数据访问
type AuthRepository struct {
	db *gorm.DB
}

// NewAuthRepository 创建 AuthRepository
func NewAuthRepository(db *gorm.DB) *AuthRepository {
	return &AuthRepository{db: db}
}

// FindUserByOpenid 按 openid 查询用户, 不存在时返回 nil
func (r *AuthRepository) FindUserByOpenid(ctx context.Context, openid string) (*model.User, error) {
	var user model.User
	err := r.db.WithContext(ctx).Where("openid = ?", openid).First(&user).Error
	if err == gorm.ErrRecordNotFound {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &user, nil
}

// FindUserByNickname 按昵称查询用户, 不存在时返回 nil (管理后台登录用)
func (r *AuthRepository) FindUserByNickname(ctx context.Context, nickname string) (*model.User, error) {
	var user model.User
	err := r.db.WithContext(ctx).Where("nickname = ?", nickname).First(&user).Error
	if err == gorm.ErrRecordNotFound {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &user, nil
}

// CreateUser 创建用户
func (r *AuthRepository) CreateUser(ctx context.Context, user *model.User) error {
	return r.db.WithContext(ctx).Create(user).Error
}

// FindUserByID 按 ID 查询用户, 不存在时返回 nil
func (r *AuthRepository) FindUserByID(ctx context.Context, id string) (*model.User, error) {
	var user model.User
	err := r.db.WithContext(ctx).Where("id = ?", id).First(&user).Error
	if err == gorm.ErrRecordNotFound {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &user, nil
}

// UpdateUser 更新用户字段
func (r *AuthRepository) UpdateUser(ctx context.Context, user *model.User) error {
	return r.db.WithContext(ctx).Save(user).Error
}

// FindMembershipsWithEnterprise 查询用户的成员关系及其关联企业信息
func (r *AuthRepository) FindMembershipsWithEnterprise(ctx context.Context, userID string) ([]model.Membership, error) {
	var memberships []model.Membership
	err := r.db.WithContext(ctx).
		Where("user_id = ?", userID).
		Preload("Enterprise").
		Order("created_at ASC").
		Find(&memberships).Error
	if err != nil {
		return nil, err
	}
	return memberships, nil
}

// ListUsers 分页查询用户列表, 支持按昵称/手机号模糊搜索和角色筛选
func (r *AuthRepository) ListUsers(ctx context.Context, keyword string, role *int, offset, limit int) ([]model.User, int64, error) {
	query := r.db.WithContext(ctx).Model(&model.User{})
	if keyword != "" {
		like := "%" + keyword + "%"
		query = query.Where("nickname ILIKE ? OR phone ILIKE ?", like, like)
	}
	if role != nil {
		query = query.Where("role = ?", *role)
	}
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var users []model.User
	if err := query.Order("created_at DESC").Offset(offset).Limit(limit).Find(&users).Error; err != nil {
		return nil, 0, err
	}
	return users, total, nil
}

// FindApprovedMembershipsByUserIDs 批量查询用户已加入企业的成员关系
func (r *AuthRepository) FindApprovedMembershipsByUserIDs(ctx context.Context, userIDs []string) (map[string]model.Membership, error) {
	var memberships []model.Membership
	err := r.db.WithContext(ctx).
		Where("user_id IN ?", userIDs).
		Where("status = ?", "approved").
		Preload("Enterprise").
		Find(&memberships).Error
	if err != nil {
		return nil, err
	}
	m := make(map[string]model.Membership, len(memberships))
	for _, mem := range memberships {
		m[mem.UserID] = mem
	}
	return m, nil
}
