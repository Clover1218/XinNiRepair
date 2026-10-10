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

// IsSuperAdmin 是否为超级管理员（可跨接单人操作他人已接工单）
func (op Operator) IsSuperAdmin() bool {
	return op.Role >= model.PlatformRoleSuper
}

// IsPlainRepairer 是否为维修业务员（非超管；仅可查看/操作自己接的工单）
func (op Operator) IsPlainRepairer() bool {
	return op.IsStoreStaff() && !op.IsSuperAdmin()
}

// AccessService 访问控制服务
type AccessService struct {
	mems   *repository.MembershipRepository
	scopes *repository.RepairerEnterpriseRepository
}

// NewAccessService 创建 AccessService
func NewAccessService(
	mems *repository.MembershipRepository,
	scopes *repository.RepairerEnterpriseRepository,
) *AccessService {
	return &AccessService{mems: mems, scopes: scopes}
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

// ────────────────────────────────────────────
// 维修业务员经营范围（V1.5 数据权限）
// ────────────────────────────────────────────

// ScopeView 数据范围视图，描述某操作者在"能看到哪些企业的数据"上的边界。
type ScopeView struct {
	// Scoped=false 表示不做范围收敛（超级管理员不限；单位审核员沿用既有"仅限本单位"校验）
	Scoped bool
	// EnterpriseIDs 仅在 Scoped=true 时有意义；为空表示该业务员当前无任何可见企业
	EnterpriseIDs []string
}

// OrderScope 计算操作者的工单数据范围。
//
// 规则：
//   - 超级管理员 (role>=2)：不限
//   - 维修业务员 (role==1)：仅限生效经营范围内的企业
//   - 其余（普通用户 / 单位审核员）：不在此收敛，由既有 CanManageEnterprise 校验本单位
func (a *AccessService) OrderScope(ctx context.Context, op Operator) (*ScopeView, error) {
	if !op.IsPlainRepairer() {
		return &ScopeView{Scoped: false}, nil
	}
	ids, err := a.scopes.ActiveEnterpriseIDsByRepairer(ctx, op.UserID)
	if err != nil {
		return nil, apperrors.ErrDatabaseError.WithError(err)
	}
	return &ScopeView{Scoped: true, EnterpriseIDs: ids}, nil
}

// ScopeItemsByRepairer 批量取各业务员的生效经营范围项（含企业名称，供列表页直接展示）
func (a *AccessService) ScopeItemsByRepairer(ctx context.Context) (map[string][]ScopeEnterpriseItem, error) {
	grouped, err := a.scopes.ListActiveGroupedByRepairer(ctx)
	if err != nil {
		return nil, apperrors.ErrDatabaseError.WithError(err)
	}
	out := make(map[string][]ScopeEnterpriseItem, len(grouped))
	for rid, items := range grouped {
		list := make([]ScopeEnterpriseItem, 0, len(items))
		for _, it := range items {
			list = append(list, ScopeEnterpriseItem{
				EnterpriseID:   it.EnterpriseID,
				EnterpriseName: it.Enterprise.Name,
				AssignedAt:     it.CreatedAt.Format("2006-01-02 15:04:05"),
			})
		}
		out[rid] = list
	}
	return out, nil
}

// ApplyOrderScope 将经营范围收敛条件写入查询过滤器。
//
// OwnerOrRepairerID 为「已接单兜底」：即便某企业的指派被事后撤销，
// 业务员已接下的工单仍必须可见可操作，否则工单会永久卡在处理中。
func (a *AccessService) ApplyOrderScope(ctx context.Context, op Operator, f *repository.OrderAdminFilter) error {
	v, err := a.OrderScope(ctx, op)
	if err != nil {
		return err
	}
	if !v.Scoped {
		return nil // 超管不限；单位审核员沿用既有本单位校验
	}
	f.ScopeLimited = true
	f.ScopeEnterpriseIDs = v.EnterpriseIDs
	f.OwnerOrRepairerID = op.UserID
	return nil
}

// AssertStatsScope 统计 / 筛选类接口的经营范围校验（V1.5）。
//
// 维修业务员必须显式指定 enterprise_id 且该企业须在其经营范围内；
// 超级管理员不受此约束；单位审核员由既有 CanManageEnterprise 校验本单位。
//
// 之所以要求显式指定而非自动聚合全量：这些 repository 查询按单个 enterprise_id 聚合，
// 不做此约束会让业务员拿到跨范围的汇总数据。
// AssertCanAdministerEnterprise 企业信息维护与成员管理的**写操作**权限（V1.5）。
//
// 仅超级管理员或该单位已审批的单位审核员。
// 维修业务员**不在其列**：他与客户企业是服务关系而非管理关系，
// 不应能改名、改 auto_approve、刷新邀请码或审批成员。
func (a *AccessService) AssertCanAdministerEnterprise(ctx context.Context, op Operator, enterpriseID string) error {
	if enterpriseID == "" {
		return apperrors.ErrInvalidParam.WithMessage("enterprise_id 不能为空")
	}
	if op.IsSuperAdmin() {
		return nil
	}
	ok, err := a.ReviewerOfEnterprise(ctx, enterpriseID, op.UserID)
	if err != nil {
		return apperrors.ErrDatabaseError.WithError(err)
	}
	if !ok {
		return apperrors.ErrForbidden.WithMessage("仅超级管理员或该单位单位审核员可维护企业信息")
	}
	return nil
}

// AssertCanViewEnterprise 企业查看的**读操作**权限（V1.5）。
//
//   - 超级管理员：不限
//   - 维修业务员：仅限其经营范围内的企业
//   - 单位审核员：仅限本单位
func (a *AccessService) AssertCanViewEnterprise(ctx context.Context, op Operator, enterpriseID string) error {
	if enterpriseID == "" {
		return apperrors.ErrInvalidParam.WithMessage("enterprise_id 不能为空")
	}
	if op.IsSuperAdmin() {
		return nil
	}
	if op.IsPlainRepairer() {
		ok, err := a.scopes.ExistsActive(ctx, op.UserID, enterpriseID)
		if err != nil {
			return apperrors.ErrDatabaseError.WithError(err)
		}
		if !ok {
			return apperrors.ErrForbidden.WithMessage("该企业不在您的服务范围内，请联系管理员分配")
		}
		return nil
	}
	ok, err := a.ReviewerOfEnterprise(ctx, enterpriseID, op.UserID)
	if err != nil {
		return apperrors.ErrDatabaseError.WithError(err)
	}
	if !ok {
		return apperrors.ErrWrongEnterprise.WithMessage("仅该单位单位审核员可查看该单位信息")
	}
	return nil
}

// AssertOrderInScope 校验单张工单是否在操作者的经营范围内（V1.5）。
//
// 已接单兜底：RepairerID 为自己的工单永远放行。若不做此兜底，
// 超管撤销某企业指派后，业务员已接下的工单会永久卡在处理中——谁也动不了。
func (a *AccessService) AssertOrderInScope(ctx context.Context, op Operator, o *model.RepairOrder) error {
	if o == nil {
		return nil
	}
	if op.IsPlainRepairer() && o.RepairerID != nil && *o.RepairerID == op.UserID {
		return nil
	}
	if o.EnterpriseID == nil {
		return nil // 尚未归属单位的空草稿，不受经营范围约束
	}
	return a.AssertEnterpriseInScope(ctx, op, *o.EnterpriseID)
}

func (a *AccessService) AssertStatsScope(ctx context.Context, op Operator, enterpriseID string) error {
	if !op.IsPlainRepairer() {
		return nil
	}
	if enterpriseID == "" {
		return apperrors.ErrWrongEnterprise.WithMessage("请先选择要查看的企业")
	}
	ok, err := a.scopes.ExistsActive(ctx, op.UserID, enterpriseID)
	if err != nil {
		return apperrors.ErrDatabaseError.WithError(err)
	}
	if !ok {
		return apperrors.ErrForbidden.WithMessage("该企业不在您的服务范围内，请联系管理员分配")
	}
	return nil
}

// InScope 判断某企业是否在指定范围视图内（Scoped=false 时恒为真）
func (v *ScopeView) InScope(enterpriseID string) bool {
	if !v.Scoped {
		return true
	}
	for _, id := range v.EnterpriseIDs {
		if id == enterpriseID {
			return true
		}
	}
	return false
}

// AssertEnterpriseInScope 校验某企业对操作者是否在经营范围内（接单等写操作前置校验）。
// 超管与单位审核员不受经营范围约束，始终放行（后者另有本单位校验）。
func (a *AccessService) AssertEnterpriseInScope(ctx context.Context, op Operator, enterpriseID string) error {
	if !op.IsPlainRepairer() || enterpriseID == "" {
		return nil
	}
	ok, err := a.scopes.ExistsActive(ctx, op.UserID, enterpriseID)
	if err != nil {
		return apperrors.ErrDatabaseError.WithError(err)
	}
	if !ok {
		return apperrors.ErrForbidden.WithMessage("该企业不在您的服务范围内，请联系管理员分配")
	}
	return nil
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
