package service

import (
	"context"
	"time"

	"go.uber.org/zap"
	"golang.org/x/crypto/bcrypt"

	apperrors "xin-ni-repair/internal/errors"
	"xin-ni-repair/internal/model"
	"xin-ni-repair/internal/repository"
)

// UserAdminService 超级管理员用户管理逻辑 (第六章)
type UserAdminService struct {
	users  *repository.AuthRepository
	logger *zap.Logger
}

// NewUserAdminService 创建 UserAdminService
func NewUserAdminService(users *repository.AuthRepository, logger *zap.Logger) *UserAdminService {
	return &UserAdminService{users: users, logger: logger}
}

// AdminUserItem 用户列表项
type AdminUserItem struct {
	ID             string    `json:"id"`
	Openid         string    `json:"openid"`
	Nickname       string    `json:"nickname"`
	AvatarURL      string    `json:"avatar_url"`
	Phone          string    `json:"phone"`
	Role           int       `json:"role"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
	EnterpriseName *string   `json:"enterprise_name"`
	MemberRole     *int      `json:"member_role"`
	MemberStatus   *string   `json:"member_status"`
}

// AdminUserList 用户列表分页结果
type AdminUserList struct {
	List     []AdminUserItem `json:"list"`
	Total    int64           `json:"total"`
	Page     int             `json:"page"`
	PageSize int             `json:"page_size"`
}

// AdminUserDetail 用户详情 (含成员关系)
type AdminUserDetail struct {
	ID          string            `json:"id"`
	Openid      string            `json:"openid"`
	Unionid     string            `json:"unionid"`
	Nickname    string            `json:"nickname"`
	AvatarURL   string            `json:"avatar_url"`
	Phone       string            `json:"phone"`
	Role        int               `json:"role"`
	CreatedAt   time.Time         `json:"created_at"`
	UpdatedAt   time.Time         `json:"updated_at"`
	Memberships []AdminMembership `json:"memberships"`
}

// AdminMembership 成员关系 (详情用)
type AdminMembership struct {
	ID             string     `json:"id"`
	EnterpriseID   string     `json:"enterprise_id"`
	EnterpriseName string     `json:"enterprise_name"`
	Role           int        `json:"role"`
	Status         string     `json:"status"`
	JoinedAt       *time.Time `json:"joined_at"`
	CreatedAt      time.Time  `json:"created_at"`
}

// ListUsers 用户列表 (6.1)
func (s *UserAdminService) ListUsers(ctx context.Context, keyword string, role *int, page, pageSize int) (*AdminUserList, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}

	users, total, err := s.users.ListUsers(ctx, keyword, role, (page-1)*pageSize, pageSize)
	if err != nil {
		return nil, s.dbErr("list users failed", err)
	}

	// 批量查询企业信息
	userIDs := make([]string, 0, len(users))
	for _, u := range users {
		userIDs = append(userIDs, u.ID)
	}
	memberMap, err := s.users.FindApprovedMembershipsByUserIDs(ctx, userIDs)
	if err != nil {
		return nil, s.dbErr("find memberships for users failed", err)
	}

	items := make([]AdminUserItem, 0, len(users))
	for _, u := range users {
		item := AdminUserItem{
			ID:        u.ID,
			Openid:    u.Openid,
			Nickname:  u.Nickname,
			AvatarURL: u.AvatarUrl,
			Phone:     u.Phone,
			Role:      u.Role,
			CreatedAt: u.CreatedAt,
			UpdatedAt: u.UpdatedAt,
		}
		if mem, ok := memberMap[u.ID]; ok {
			name := mem.Enterprise.Name
			role := mem.Role
			status := string(mem.Status)
			item.EnterpriseName = &name
			item.MemberRole = &role
			item.MemberStatus = &status
		}
		items = append(items, item)
	}

	return &AdminUserList{
		List:     items,
		Total:    total,
		Page:     page,
		PageSize: pageSize,
	}, nil
}

// GetUser 用户详情 (6.2)
func (s *UserAdminService) GetUser(ctx context.Context, userID string) (*AdminUserDetail, error) {
	user, err := s.users.FindUserByID(ctx, userID)
	if err != nil {
		return nil, s.dbErr("find user failed", err)
	}
	if user == nil {
		return nil, apperrors.ErrUserNotFound
	}

	memberships, err := s.users.FindMembershipsWithEnterprise(ctx, userID)
	if err != nil {
		return nil, s.dbErr("find memberships failed", err)
	}

	adminMems := make([]AdminMembership, 0, len(memberships))
	for _, m := range memberships {
		adminMems = append(adminMems, AdminMembership{
			ID:             m.ID,
			EnterpriseID:   m.EnterpriseID,
			EnterpriseName: m.Enterprise.Name,
			Role:           m.Role,
			Status:         m.Status,
			JoinedAt:       m.JoinedAt,
			CreatedAt:      m.CreatedAt,
		})
	}

	return &AdminUserDetail{
		ID:          user.ID,
		Openid:      user.Openid,
		Unionid:     user.Unionid,
		Nickname:    user.Nickname,
		AvatarURL:   user.AvatarUrl,
		Phone:       user.Phone,
		Role:        user.Role,
		CreatedAt:   user.CreatedAt,
		UpdatedAt:   user.UpdatedAt,
		Memberships: adminMems,
	}, nil
}

// UpdateUserInput 更新用户参数
type UpdateUserInput struct {
	Nickname *string
	Role     *int
	Phone    *string
}

// UpdateUser 更新用户属性 (6.3)
func (s *UserAdminService) UpdateUser(ctx context.Context, userID, operatorID string, input UpdateUserInput) (*AdminUserDetail, error) {
	user, err := s.users.FindUserByID(ctx, userID)
	if err != nil {
		return nil, s.dbErr("find user failed", err)
	}
	if user == nil {
		return nil, apperrors.ErrUserNotFound
	}

	// 不可修改自身 role
	if input.Role != nil && userID == operatorID {
		return nil, apperrors.ErrInvalidParam.WithMessage("不能修改自身角色")
	}

	// 不可设为超级管理员
	if input.Role != nil && *input.Role == model.PlatformRoleSuperAdmin {
		return nil, apperrors.ErrInvalidParam.WithMessage("不允许设置超级管理员角色")
	}

	if input.Nickname != nil {
		if len(*input.Nickname) < 1 || len(*input.Nickname) > 32 {
			return nil, apperrors.ErrInvalidParam.WithMessage("昵称长度需 1-32 字符")
		}
		user.Nickname = *input.Nickname
	}
	if input.Role != nil {
		user.Role = *input.Role
	}
	if input.Phone != nil {
		user.Phone = *input.Phone
	}

	if err := s.users.UpdateUser(ctx, user); err != nil {
		return nil, s.dbErr("update user failed", err)
	}

	return s.GetUser(ctx, userID)
}

// ResetPassword 重置用户密码 (6.4)
func (s *UserAdminService) ResetPassword(ctx context.Context, userID, newPassword string) error {
	user, err := s.users.FindUserByID(ctx, userID)
	if err != nil {
		return s.dbErr("find user failed", err)
	}
	if user == nil {
		return apperrors.ErrUserNotFound
	}

	// 普通微信用户无登录密码
	if user.Role < model.PlatformRolePlatformAdmin {
		return apperrors.ErrInvalidParam.WithMessage("该用户无登录密码（微信用户）")
	}

	if len(newPassword) < 6 || len(newPassword) > 32 {
		return apperrors.ErrInvalidParam.WithMessage("密码长度需 6-32 位")
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(newPassword), bcrypt.DefaultCost)
	if err != nil {
		s.logger.Error("reset password: bcrypt hash failed", zap.Error(err))
		return apperrors.ErrInternal
	}
	user.Password = string(hash)

	if err := s.users.UpdateUser(ctx, user); err != nil {
		return s.dbErr("update user password failed", err)
	}
	return nil
}

// dbErr 记录数据库错误并返回统一错误
func (s *UserAdminService) dbErr(msg string, err error) error {
	s.logger.Error(msg, zap.Error(err))
	return apperrors.ErrDatabaseError.WithError(err)
}
