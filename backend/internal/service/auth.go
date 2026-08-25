package service

import (
	"context"
	"os"
	"regexp"
	"strings"

	"github.com/google/uuid"
	"go.uber.org/zap"
	"golang.org/x/crypto/bcrypt"

	apperrors "xin-ni-repair/internal/errors"
	"xin-ni-repair/internal/model"
	"xin-ni-repair/internal/repository"
)

// phonePattern 11 位手机号 (1 开头)
var phonePattern = regexp.MustCompile(`^1\d{10}$`)

// EnterpriseInfo 用户所属企业信息
type EnterpriseInfo struct {
	EnterpriseID   string `json:"enterprise_id"`
	EnterpriseName string `json:"enterprise_name"`
	Role           string `json:"role"`
	Status         string `json:"status"`
}

// UserProfile 用户信息 (含所属企业)
type UserProfile struct {
	ID          string           `json:"id"`
	Nickname    string           `json:"nickname"`
	AvatarURL   string           `json:"avatar_url"`
	Phone       string           `json:"phone"`
	Role        int              `json:"role"` // 平台角色: 0=普通用户 1=平台管理员
	Enterprises []EnterpriseInfo `json:"enterprises"`
}

// LoginResult 微信登录结果
type LoginResult struct {
	AccessToken string       `json:"access_token"`
	ExpiresIn   int64        `json:"expires_in"`
	User        *UserProfile `json:"user"`
	NeedProfile bool         `json:"need_profile"` // true=需先完善资料(新用户), 由 /auth/register 完成
}

// AuthService 认证业务逻辑
type AuthService struct {
	users  *repository.AuthRepository
	token  *TokenService
	wechat *WechatService
	logger *zap.Logger
}

// NewAuthService 创建 AuthService
func NewAuthService(users *repository.AuthRepository, token *TokenService, wechat *WechatService, logger *zap.Logger) *AuthService {
	return &AuthService{
		users:  users,
		token:  token,
		wechat: wechat,
		logger: logger,
	}
}

// Login 微信静默登录: code 换 openid, 新用户自动注册, 签发 JWT
func (s *AuthService) Login(ctx context.Context, code string) (*LoginResult, error) {
	session, err := s.wechat.Code2Session(ctx, code)
	if err != nil {
		return nil, err
	}

	user, err := s.users.FindUserByOpenid(ctx, session.Openid)
	if err != nil {
		s.logger.Error("find user by openid failed", zap.Error(err))
		return nil, apperrors.ErrDatabaseError.WithError(err)
	}

	// 新用户: 不自动注册, 提示需先完善资料 (头像/昵称/手机号)
	if user == nil {
		return &LoginResult{NeedProfile: true}, nil
	}

	return s.issueToken(ctx, user)
}

// AdminLogin 管理后台密码登录 (2.4): nickname + password, 仅平台管理员
func (s *AuthService) AdminLogin(ctx context.Context, nickname, password string) (*LoginResult, error) {
	user, err := s.users.FindUserByNickname(ctx, nickname)
	if err != nil {
		s.logger.Error("find user by nickname failed", zap.Error(err))
		return nil, apperrors.ErrDatabaseError.WithError(err)
	}
	if user == nil || user.Password == "" {
		return nil, apperrors.ErrInvalidCredentials
	}
	if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(password)); err != nil {
		return nil, apperrors.ErrInvalidCredentials
	}
	if user.Role < model.PlatformRolePlatformAdmin {
		return nil, apperrors.ErrNotAdmin
	}

	return s.issueToken(ctx, user)
}

// Register 新用户资料完善注册 (2.5): 微信 code 换 openid, 手机号二选一来源
// - phoneCode 非空: 微信 getPhoneNumber 授权解密 (优先)
// - phone 非空: 手动输入, 仅格式校验
func (s *AuthService) Register(ctx context.Context, code, nickname, avatarURL, phoneCode, phone string) (*LoginResult, error) {
	session, err := s.wechat.Code2Session(ctx, code)
	if err != nil {
		return nil, err
	}

	// openid 已被占用: 已有用户直接登录
	existing, err := s.users.FindUserByOpenid(ctx, session.Openid)
	if err != nil {
		s.logger.Error("find user by openid failed", zap.Error(err))
		return nil, apperrors.ErrDatabaseError.WithError(err)
	}
	if existing != nil {
		return s.issueToken(ctx, existing)
	}

	// 手机号来源: 优先微信授权解密, 否则手动输入(仅格式校验)
	var phoneNumber string
	switch {
	case phoneCode != "":
		phoneNumber, err = s.wechat.GetPhoneNumber(ctx, phoneCode)
		if err != nil {
			return nil, err
		}
	case phone != "":
		if !phonePattern.MatchString(phone) {
			return nil, apperrors.ErrInvalidParam.WithMessage("手机号格式不正确")
		}
		phoneNumber = phone
	default:
		return nil, apperrors.ErrInvalidParam.WithMessage("缺少手机号，请微信授权获取或手动输入")
	}

	user := &model.User{
		ID:        uuid.New().String(),
		Openid:    session.Openid,
		Unionid:   session.Unionid,
		Nickname:  nickname,
		AvatarUrl: avatarURL,
		Phone:     phoneNumber,
	}
	if err := s.users.CreateUser(ctx, user); err != nil {
		// 唯一索引冲突: phone 已被占用 (PostgreSQL 错误码 23505)
		if isUniqueViolation(err) {
			return nil, apperrors.ErrPhoneAlreadyBound.WithMessage("该手机号已被其他账号绑定")
		}
		s.logger.Error("create user failed", zap.Error(err))
		return nil, apperrors.ErrDatabaseError.WithError(err)
	}
	s.logger.Info("new user registered via profile", zap.String("user_id", user.ID))

	return s.issueToken(ctx, user)
}

// issueToken 生成 JWT 并组装登录结果
func (s *AuthService) issueToken(ctx context.Context, user *model.User) (*LoginResult, error) {
	tokenStr, ttl, err := s.token.GenerateAccessToken(user.ID, user.Role)
	if err != nil {
		s.logger.Error("generate access token failed", zap.Error(err))
		return nil, apperrors.ErrInternal.WithError(err)
	}
	profile, err := s.buildProfile(ctx, user)
	if err != nil {
		return nil, err
	}
	return &LoginResult{
		AccessToken: tokenStr,
		ExpiresIn:   int64(ttl.Seconds()),
		User:        profile,
	}, nil
}

// isUniqueViolation 判断是否为唯一约束冲突 (PostgreSQL 错误码 23505)
func isUniqueViolation(err error) bool {
	return err != nil && strings.Contains(err.Error(), "23505")
}

// SeedAdminUser 启动时初始化店主账号 (幂等)。
// - openid 占位 "admin-bootstrap", 非微信来源标记
// - nickname 固定 "admin", 对应 /auth/admin-login 入口
// - 密码默认 "123456", 可通过环境变量 ADMIN_PASSWORD 覆盖
// 已存在则跳过 (不改密码), 仅在不存在时创建。
func (s *AuthService) SeedAdminUser(ctx context.Context) error {
	const (
		adminOpenid   = "admin-bootstrap"
		adminNickname = "admin"
		defaultPwd    = "123456"
	)

	// 已存在则跳过
	existing, err := s.users.FindUserByOpenid(ctx, adminOpenid)
	if err != nil {
		s.logger.Error("seed: check admin existence failed", zap.Error(err))
		return err
	}
	if existing != nil {
		return nil
	}

	// 密码: 优先环境变量, 默认 123456
	password := os.Getenv("ADMIN_PASSWORD")
	if password == "" {
		password = defaultPwd
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		s.logger.Error("seed: bcrypt hash failed", zap.Error(err))
		return err
	}

	user := &model.User{
		ID:       uuid.New().String(),
		Openid:   adminOpenid,
		Nickname: adminNickname,
		Password: string(hash),
		Role:     model.PlatformRolePlatformAdmin,
	}
	if err := s.users.CreateUser(ctx, user); err != nil {
		s.logger.Error("seed: create admin failed", zap.Error(err))
		return err
	}
	s.logger.Info("admin user seeded",
		zap.String("nickname", adminNickname),
		zap.String("login", "POST /api/v1/auth/admin-login"),
	)
	return nil
}

// Me 获取当前用户信息
func (s *AuthService) Me(ctx context.Context, userID string) (*UserProfile, error) {
	user, err := s.users.FindUserByID(ctx, userID)
	if err != nil {
		s.logger.Error("find user by id failed", zap.Error(err))
		return nil, apperrors.ErrDatabaseError.WithError(err)
	}
	if user == nil {
		return nil, apperrors.ErrUserNotFound
	}
	return s.buildProfile(ctx, user)
}

// BindPhone 绑定手机号 (仅当未绑定时可绑定, 绑定后不可修改)
func (s *AuthService) BindPhone(ctx context.Context, userID, phone string) error {
	if !phonePattern.MatchString(phone) {
		return apperrors.ErrInvalidParam.WithMessage("手机号格式不正确")
	}

	user, err := s.users.FindUserByID(ctx, userID)
	if err != nil {
		s.logger.Error("find user by id failed", zap.Error(err))
		return apperrors.ErrDatabaseError.WithError(err)
	}
	if user == nil {
		return apperrors.ErrUserNotFound
	}
	if user.Phone != "" {
		return apperrors.ErrPhoneAlreadyBound
	}

	user.Phone = phone
	if err := s.users.UpdateUser(ctx, user); err != nil {
		s.logger.Error("update user failed", zap.Error(err))
		return apperrors.ErrDatabaseError.WithError(err)
	}
	s.logger.Info("phone bound", zap.String("user_id", user.ID))
	return nil
}

// buildProfile 组装用户资料及所属企业列表
func (s *AuthService) buildProfile(ctx context.Context, user *model.User) (*UserProfile, error) {
	memberships, err := s.users.FindMembershipsWithEnterprise(ctx, user.ID)
	if err != nil {
		s.logger.Error("find memberships failed", zap.Error(err))
		return nil, apperrors.ErrDatabaseError.WithError(err)
	}

	profile := &UserProfile{
		ID:          user.ID,
		Nickname:    user.Nickname,
		AvatarURL:   user.AvatarUrl,
		Phone:       user.Phone,
		Role:        user.Role,
		Enterprises: make([]EnterpriseInfo, 0, len(memberships)),
	}
	for _, m := range memberships {
		profile.Enterprises = append(profile.Enterprises, EnterpriseInfo{
			EnterpriseID:   m.EnterpriseID,
			EnterpriseName: m.Enterprise.Name,
			Role:           memberRoleName(m.Role),
			Status:         m.Status,
		})
	}
	return profile, nil
}
