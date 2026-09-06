// 访问控制 (authorization) 抽象层。
//
// V1.4 角色为"双层模型": 全局平台角色 users.role(0普通/1维修业务员/2超管) +
// 单位内成员身份 memberships.role(0普通成员/1单位审核员)。本服务统一提供判定:
//
//   - 店方角色 (维修业务员/超级管理员, role>=1): 可跨单位管理
//   - 单位审核员 (membership.role=1 且 approved): 仅限本单位
//
// 甲方尚未细化的约束 (如"维修业务员仅限本单位/业务范围接单"、审核员可跨单位等)
// 统一在本服务内集中扩展即可, 不影响 handler/service 调用方。
package service

import (
	"context"

	apperrors "xin-ni-repair/internal/errors"
	"xin-ni-repair/internal/model"
	"xin-ni-repair/internal/repository"
)

// Operator 当前操作者 (来自 JWT, handler 统一构造)
type Operator struct {
	UserID string `json:"user_id"`
	Role   int    `json:"role"` // 平台全局角色: 0=普通用户 1=维修业务员 2=超级管理员
}

// IsStoreStaff 是否为店方角色 (维修业务员/超级管理员)
func (op Operator) IsStoreStaff() bool {
	return op.Role >= model.PlatformRoleRepairer
}

// AccessService 访问控制服务
type AccessService struct {
	mems *repository.MembershipRepository
}

// NewAccessService 创建 AccessService
func NewAccessService(mems *repository.MembershipRepository) *AccessService {
	return &AccessService{mems: mems}
}

// StaffOnly 校验操作者为店方角色, 否则返回权限错误
func (a *AccessService) StaffOnly(op Operator) error {
	if !op.IsStoreStaff() {
		return apperrors.ErrNotAdmin.WithMessage("仅维修业务员/超级管理员可执行此操作")
	}
	return nil
}

// SuperOnly 校验操作者为超级管理员
func (a *AccessService) SuperOnly(op Operator) error {
	if op.Role < model.PlatformRoleSuper {
		return apperrors.ErrForbidden.WithMessage("仅超级管理员可执行此操作")
	}
	return nil
}

// ReviewerOfEnterprise 判断用户是否为该单位的"已审批单位审核员"
func (a *AccessService) ReviewerOfEnterprise(ctx context.Context, enterpriseID, userID string) (bool, error) {
	m, err := a.mems.FindByEnterpriseAndUser(ctx, enterpriseID, userID)
	if err != nil {
		return false, err
	}
	return m != nil && m.Status == string(model.MemberApproved) && m.Role == model.EnterpriseRoleReviewer, nil
}

// MemberOfEnterprise 判断用户是否为该单位的"已审批成员" (含单位审核员)
func (a *AccessService) MemberOfEnterprise(ctx context.Context, enterpriseID, userID string) (bool, error) {
	m, err := a.mems.FindByEnterpriseAndUser(ctx, enterpriseID, userID)
	if err != nil {
		return false, err
	}
	return m != nil && m.Status == string(model.MemberApproved), nil
}

// CanManageEnterprise 校验对本单位的管理权限:
// 店方角色(role>=1) 或 该单位单位审核员(approved) 均可; 否则返回错误。
func (a *AccessService) CanManageEnterprise(ctx context.Context, enterpriseID, userID string, role int) error {
	if role >= model.PlatformRoleRepairer {
		return nil
	}
	ok, err := a.ReviewerOfEnterprise(ctx, enterpriseID, userID)
	if err != nil {
		return apperrors.ErrDatabaseError.WithError(err)
	}
	if !ok {
		return apperrors.ErrWrongEnterprise.WithMessage("仅该单位单位审核员或店方角色可执行此操作")
	}
	return nil
}

// CanAccessEnterpriseOrders 校验对本单位工单的查看/处理权限
// (与 CanManageEnterprise 规则一致; 保留独立方法便于甲方后续差异化扩展)
func (a *AccessService) CanAccessEnterpriseOrders(ctx context.Context, enterpriseID, userID string, role int) error {
	return a.CanManageEnterprise(ctx, enterpriseID, userID, role)
}

// CanAccessOrder 校验某工单是否在当前操作者权限范围内:
// 店方角色直接放行; 普通用户需为该工单所属单位的已审批单位审核员。
// orderEnterpriseID 为 nil (空草稿) 时仅店方角色可访问。
func (a *AccessService) CanAccessOrder(ctx context.Context, orderEnterpriseID *string, userID string, role int) error {
	if role >= model.PlatformRoleRepairer {
		return nil
	}
	if orderEnterpriseID == nil {
		return apperrors.ErrWrongEnterprise.WithMessage("工单尚未归属单位，仅店方角色可操作")
	}
	return a.CanManageEnterprise(ctx, *orderEnterpriseID, userID, role)
}
