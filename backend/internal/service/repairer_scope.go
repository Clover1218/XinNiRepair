// 维修业务员经营范围管理 (V1.5)。
//
// 本服务负责「超管指派业务员可服务哪些客户企业」的读写；
// 数据范围判定（谁能看到哪些数据）在 AccessService，两者职责分离。
//
// 对应需求: 问题 #10 设置可接单企业, 不能所有企业都能查看
//
//	问题 #28 按权限范围内企业查询统计, 防止企业名单泄露
package service

import (
	"context"
	"strings"

	"go.uber.org/zap"

	apperrors "xin-ni-repair/internal/errors"
	"xin-ni-repair/internal/model"
	"xin-ni-repair/internal/repository"
)

// maxScopeEnterprises 单个业务员可指派的企业上限（防御性约束，避免误传超大列表）
const maxScopeEnterprises = 200

// ────────────────────────────────────────────
// 输出结构
// ────────────────────────────────────────────

// ScopeEnterpriseItem 经营范围项（含企业摘要，供后台页面直接渲染）
type ScopeEnterpriseItem struct {
	EnterpriseID   string `json:"enterprise_id"`
	EnterpriseName string `json:"enterprise_name"`
	AssignedAt     string `json:"assigned_at"`
}

// RepairerScopeResult 某业务员的经营范围
type RepairerScopeResult struct {
	RepairerID  string                `json:"repairer_id"`
	Nickname    string                `json:"nickname"`
	Enterprises []ScopeEnterpriseItem `json:"enterprises"`
	Total       int                   `json:"total"`
	// ScopeLimited 为 false 表示超级管理员（理论上不限），此时 Enterprises 仅为展示用
	ScopeLimited bool `json:"scope_limited"`
}

// UpdateRepairerScopeInput 全量覆盖指派入参
type UpdateRepairerScopeInput struct {
	EnterpriseIDs []string `json:"enterprise_ids"`
}

// ScopeRepairerItem 反向查询结果（某企业由哪些业务员服务）
type ScopeRepairerItem struct {
	RepairerID string `json:"repairer_id"`
	Nickname   string `json:"nickname"`
	Phone      string `json:"phone"`
	AvatarURL  string `json:"avatar_url"`
	AssignedAt string `json:"assigned_at"`
}

// ────────────────────────────────────────────
// 服务
// ────────────────────────────────────────────

// RepairerScopeService 经营范围管理服务
type RepairerScopeService struct {
	scopes *repository.RepairerEnterpriseRepository
	orders *repository.OrderRepository
	users  *repository.AuthRepository
	logger *zap.Logger
}

// NewRepairerScopeService 创建 RepairerScopeService
func NewRepairerScopeService(
	scopes *repository.RepairerEnterpriseRepository,
	orders *repository.OrderRepository,
	users *repository.AuthRepository,
	logger *zap.Logger,
) *RepairerScopeService {
	return &RepairerScopeService{scopes: scopes, orders: orders, users: users, logger: logger}
}

// GetScope 某业务员的经营范围
func (s *RepairerScopeService) GetScope(ctx context.Context, repairerID string) (*RepairerScopeResult, error) {
	u, err := s.loadRepairer(ctx, repairerID)
	if err != nil {
		return nil, err
	}
	items, err := s.scopes.ListActiveByRepairer(ctx, repairerID)
	if err != nil {
		return nil, s.dbErr("list repairer scope failed", err)
	}
	out := make([]ScopeEnterpriseItem, 0, len(items))
	for _, it := range items {
		out = append(out, ScopeEnterpriseItem{
			EnterpriseID:   it.EnterpriseID,
			EnterpriseName: it.Enterprise.Name,
			AssignedAt:     it.CreatedAt.Format("2006-01-02 15:04:05"),
		})
	}
	return &RepairerScopeResult{
		RepairerID:   u.ID,
		Nickname:     u.Nickname,
		Enterprises:  out,
		Total:        len(out),
		ScopeLimited: true,
	}, nil
}

// UpdateScope 全量覆盖某业务员的经营范围（幂等）
func (s *RepairerScopeService) UpdateScope(ctx context.Context, op Operator, repairerID string, in UpdateRepairerScopeInput) (*RepairerScopeResult, error) {
	if !op.IsSuperAdmin() {
		return nil, apperrors.ErrForbidden.WithMessage("仅超级管理员可指派经营范围")
	}
	u, err := s.loadRepairer(ctx, repairerID)
	if err != nil {
		return nil, err
	}

	// 去重 + 去空 + 上限校验
	seen := make(map[string]struct{}, len(in.EnterpriseIDs))
	ids := make([]string, 0, len(in.EnterpriseIDs))
	for _, raw := range in.EnterpriseIDs {
		id := strings.TrimSpace(raw)
		if id == "" {
			continue
		}
		if _, dup := seen[id]; dup {
			continue
		}
		seen[id] = struct{}{}
		ids = append(ids, id)
	}
	if len(ids) > maxScopeEnterprises {
		return nil, apperrors.ErrInvalidParam.WithMessage("单次指派企业数量超出上限")
	}

	// 逐个校验企业存在且有效，避免写入脏 ID
	for _, id := range ids {
		ok, err := s.scopes.ExistsEnterprise(ctx, id)
		if err != nil {
			return nil, s.dbErr("check enterprise failed", err)
		}
		if !ok {
			return nil, apperrors.ErrInvalidParam.WithMessage("企业不存在或已禁用: " + id)
		}
	}

	if err := s.scopes.ReplaceAll(ctx, repairerID, ids, op.UserID); err != nil {
		return nil, s.dbErr("replace repairer scope failed", err)
	}
	if s.logger != nil {
		s.logger.Info("repairer scope updated",
			zap.String("repairer_id", repairerID),
			zap.String("operator", op.UserID),
			zap.Int("count", len(ids)))
	}
	return s.GetScope(ctx, u.ID)
}

// ListByEnterprise 反向查询：服务该企业的业务员
func (s *RepairerScopeService) ListByEnterprise(ctx context.Context, enterpriseID string) ([]ScopeRepairerItem, error) {
	items, err := s.scopes.ListByEnterprise(ctx, enterpriseID)
	if err != nil {
		return nil, s.dbErr("list scope by enterprise failed", err)
	}
	out := make([]ScopeRepairerItem, 0, len(items))
	for _, it := range items {
		out = append(out, ScopeRepairerItem{
			RepairerID: it.RepairerID,
			Nickname:   it.Repairer.Nickname,
			Phone:      it.Repairer.Phone,
			AvatarURL:  it.Repairer.AvatarUrl,
			AssignedAt: it.CreatedAt.Format("2006-01-02 15:04:05"),
		})
	}
	return out, nil
}

// GetMyScope 业务员查询自身可见范围（小程序端筛选用）
func (s *RepairerScopeService) GetMyScope(ctx context.Context, op Operator) (*RepairerScopeResult, error) {
	// 超管不做收敛，返回全部企业便于理解"当前不受限"
	u, err := s.users.FindUserByID(ctx, op.UserID)
	if err != nil {
		return nil, s.dbErr("load operator failed", err)
	}
	items, err := s.scopes.ListActiveByRepairer(ctx, op.UserID)
	if err != nil {
		return nil, s.dbErr("list my scope failed", err)
	}
	out := make([]ScopeEnterpriseItem, 0, len(items))
	for _, it := range items {
		out = append(out, ScopeEnterpriseItem{
			EnterpriseID:   it.EnterpriseID,
			EnterpriseName: it.Enterprise.Name,
			AssignedAt:     it.CreatedAt.Format("2006-01-02 15:04:05"),
		})
	}
	nickname := ""
	if u != nil {
		nickname = u.Nickname
	}
	return &RepairerScopeResult{
		RepairerID:   op.UserID,
		Nickname:     nickname,
		Enterprises:  out,
		Total:        len(out),
		ScopeLimited: op.IsPlainRepairer(),
	}, nil
}

// loadRepairer 加载并校验目标用户确为维修业务员/超管
func (s *RepairerScopeService) loadRepairer(ctx context.Context, repairerID string) (*model.User, error) {
	repairerID = strings.TrimSpace(repairerID)
	if repairerID == "" {
		return nil, apperrors.ErrInvalidParam.WithMessage("repairer_id 不能为空")
	}
	u, err := s.users.FindUserByID(ctx, repairerID)
	if err != nil {
		return nil, s.dbErr("load repairer failed", err)
	}
	if u == nil {
		return nil, apperrors.ErrUserNotFound
	}
	if u.Role < model.PlatformRoleRepairer {
		return nil, apperrors.ErrInvalidParam.WithMessage("目标用户不是维修业务员/超级管理员")
	}
	return u, nil
}

func (s *RepairerScopeService) dbErr(msg string, err error) error {
	return apperrors.ErrDatabaseError.WithError(err).WithMessage(msg)
}
