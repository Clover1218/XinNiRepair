// EnterpriseService 企业管理业务逻辑。
package service

import (
	"context"
	"crypto/rand"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"

	apperrors "xin-ni-repair/internal/errors"
	"xin-ni-repair/internal/model"
	"xin-ni-repair/internal/repository"
)

// inviteCodeChars 邀请码字符集 (去除易混淆字符)
const inviteCodeChars = "ABCDEFGHJKLMNPQRSTUVWXYZ23456789"

// EnterpriseDetail 企业详情
type EnterpriseDetail struct {
	ID                  string     `json:"id"`
	Name                string     `json:"name"`
	InviteCode          string     `json:"invite_code"`
	AutoApprove         bool       `json:"auto_approve"`
	MemberCount         int64      `json:"member_count,omitempty"`
	MyRole              string     `json:"my_role,omitempty"`
	CreatedAt           time.Time  `json:"created_at"`
	InviteCodeExpiresAt *time.Time `json:"invite_code_expires_at"`
}

// JoinResult 加入企业结果
type JoinResult struct {
	Status         string     `json:"status"`
	EnterpriseName string     `json:"enterprise_name"`
	JoinedAt       *time.Time `json:"joined_at,omitempty"`
	Tip            string     `json:"tip,omitempty"`
}

// MemberItem 成员列表项
type MemberItem struct {
	MembershipID string     `json:"membership_id"`
	UserID       string     `json:"user_id"`
	Nickname     string     `json:"nickname"`
	AvatarURL    string     `json:"avatar_url"`
	Phone        string     `json:"phone"`
	Role         string     `json:"role"`
	RoleLabel    string     `json:"role_label"`
	Status       string     `json:"status"`
	StatusLabel  string     `json:"status_label"`
	OrderCount   int64      `json:"order_count"`
	JoinedAt     *time.Time `json:"joined_at"`
}

// MemberList 成员分页结果
type MemberList struct {
	List       []MemberItem `json:"list"`
	Total      int64        `json:"total"`
	Page       int          `json:"page"`
	PageSize   int          `json:"page_size"`
	TotalPages int          `json:"total_pages"`
}

// BatchResult 批量审批结果
type BatchResult struct {
	ApprovedCount int `json:"approved_count,omitempty"`
	SuccessCount  int `json:"success_count,omitempty"`
	FailedCount   int `json:"failed_count"`
}

// InviteCodeResult 刷新邀请码结果
type InviteCodeResult struct {
	InviteCode string     `json:"invite_code"`
	ExpiresAt  *time.Time `json:"expires_at"`
}

// memberStatusWhitelist 成员列表允许的状态筛选
var memberStatusWhitelist = map[string]bool{
	string(model.MemberPending):  true,
	string(model.MemberApproved): true,
	string(model.MemberRejected): true,
	string(model.MemberRemoved):  true,
}

// EnterpriseService 企业管理业务逻辑
type EnterpriseService struct {
	ents   *repository.EnterpriseRepository
	mems   *repository.MembershipRepository
	logger *zap.Logger
}

// NewEnterpriseService 创建 EnterpriseService
func NewEnterpriseService(ents *repository.EnterpriseRepository, mems *repository.MembershipRepository, logger *zap.Logger) *EnterpriseService {
	return &EnterpriseService{
		ents:   ents,
		mems:   mems,
		logger: logger,
	}
}

// Create 创建企业 (仅平台管理员)
func (s *EnterpriseService) Create(ctx context.Context, name string) (*EnterpriseDetail, error) {
	name = strings.TrimSpace(name)
	if n := len([]rune(name)); n < 2 || n > 50 {
		return nil, apperrors.ErrInvalidParam.WithMessage("企业名称需为2-50字符")
	}

	exist, err := s.ents.FindByName(ctx, name)
	if err != nil {
		return nil, s.dbErr("find enterprise by name failed", err)
	}
	if exist != nil {
		return nil, apperrors.ErrEnterpriseNameExists
	}

	code, err := s.generateUniqueInviteCode(ctx, "")
	if err != nil {
		return nil, err
	}

	ent := &model.Enterprise{
		ID:         uuid.New().String(),
		Name:       name,
		InviteCode: code,
	}
	if err := s.ents.Create(ctx, ent); err != nil {
		return nil, s.dbErr("create enterprise failed", err)
	}
	s.logger.Info("enterprise created", zap.String("enterprise_id", ent.ID), zap.String("name", ent.Name))

	return &EnterpriseDetail{
		ID:                  ent.ID,
		Name:                ent.Name,
		InviteCode:          ent.InviteCode,
		AutoApprove:         ent.AutoApprove,
		CreatedAt:           ent.CreatedAt,
		InviteCodeExpiresAt: ent.InviteCodeExpiresAt,
	}, nil
}

// Get 获取企业信息 (企业成员或平台管理员)
func (s *EnterpriseService) Get(ctx context.Context, userID string, role int, enterpriseID string) (*EnterpriseDetail, error) {
	ent, err := s.ents.FindByID(ctx, enterpriseID)
	if err != nil {
		return nil, s.dbErr("find enterprise by id failed", err)
	}
	if ent == nil {
		return nil, apperrors.ErrEnterpriseNotFound
	}

	isPlatformAdmin := role >= model.PlatformRolePlatformAdmin
	m, err := s.mems.FindByEnterpriseAndUser(ctx, enterpriseID, userID)
	if err != nil {
		return nil, s.dbErr("find membership failed", err)
	}
	if !isPlatformAdmin && (m == nil || m.Status != string(model.MemberApproved)) {
		return nil, apperrors.ErrWrongEnterprise
	}

	memberCount, err := s.mems.CountApproved(ctx, enterpriseID)
	if err != nil {
		return nil, s.dbErr("count members failed", err)
	}

	detail := &EnterpriseDetail{
		ID:                  ent.ID,
		Name:                ent.Name,
		InviteCode:          ent.InviteCode,
		AutoApprove:         ent.AutoApprove,
		MemberCount:         memberCount,
		CreatedAt:           ent.CreatedAt,
		InviteCodeExpiresAt: ent.InviteCodeExpiresAt,
	}
	if m != nil {
		detail.MyRole = memberRoleName(m.Role)
	}
	return detail, nil
}

// Update 更新企业设置 (仅平台管理员)
func (s *EnterpriseService) Update(ctx context.Context, enterpriseID string, name *string, autoApprove *bool) (*EnterpriseDetail, error) {
	ent, err := s.ents.FindByID(ctx, enterpriseID)
	if err != nil {
		return nil, s.dbErr("find enterprise by id failed", err)
	}
	if ent == nil {
		return nil, apperrors.ErrEnterpriseNotFound
	}

	if name != nil {
		n := strings.TrimSpace(*name)
		if len([]rune(n)) < 2 || len([]rune(n)) > 50 {
			return nil, apperrors.ErrInvalidParam.WithMessage("企业名称需为2-50字符")
		}
		if n != ent.Name {
			exist, err := s.ents.FindByName(ctx, n)
			if err != nil {
				return nil, s.dbErr("find enterprise by name failed", err)
			}
			if exist != nil {
				return nil, apperrors.ErrEnterpriseNameExists
			}
			ent.Name = n
		}
	}
	if autoApprove != nil {
		ent.AutoApprove = *autoApprove
	}

	if err := s.ents.Update(ctx, ent); err != nil {
		return nil, s.dbErr("update enterprise failed", err)
	}
	s.logger.Info("enterprise updated", zap.String("enterprise_id", ent.ID))

	return &EnterpriseDetail{
		ID:                  ent.ID,
		Name:                ent.Name,
		InviteCode:          ent.InviteCode,
		AutoApprove:         ent.AutoApprove,
		CreatedAt:           ent.CreatedAt,
		InviteCodeExpiresAt: ent.InviteCodeExpiresAt,
	}, nil
}

// Join 加入企业 (扫码/邀请码)
// Join 加入企业 (3.4): 仅凭邀请码加入, 无需企业 ID
func (s *EnterpriseService) Join(ctx context.Context, userID, inviteCode string) (*JoinResult, error) {
	ent, err := s.ents.FindByInviteCode(ctx, inviteCode)
	if err != nil {
		return nil, s.dbErr("find enterprise by invite code failed", err)
	}
	if ent == nil || ent.Status != int(model.EnterpriseActive) {
		return nil, apperrors.ErrInviteCodeInvalid
	}
	if ent.InviteCodeExpiresAt != nil && time.Now().After(*ent.InviteCodeExpiresAt) {
		return nil, apperrors.ErrInviteCodeExpired
	}

	m, err := s.mems.FindByEnterpriseAndUser(ctx, ent.ID, userID)
	if err != nil {
		return nil, s.dbErr("find membership failed", err)
	}

	now := time.Now()
	if m == nil {
		m = &model.Membership{
			ID:           uuid.New().String(),
			EnterpriseID: ent.ID,
			UserID:       userID,
			Role:         model.EnterpriseRoleMember,
			Status:       string(model.MemberPending),
		}
		if ent.AutoApprove {
			m.Status = string(model.MemberApproved)
			m.JoinedAt = &now
		}
		if err := s.mems.Create(ctx, m); err != nil {
			return nil, s.dbErr("create membership failed", err)
		}
	} else {
		switch m.Status {
		case string(model.MemberPending), string(model.MemberApproved):
			return nil, apperrors.ErrAlreadyJoined
		case string(model.MemberRejected), string(model.MemberRemoved):
			// 被拒绝/移除的用户可重新申请
			m.Status = string(model.MemberPending)
			m.RemovedAt = nil
			if ent.AutoApprove {
				m.Status = string(model.MemberApproved)
				m.JoinedAt = &now
			}
			if err := s.mems.Update(ctx, m); err != nil {
				return nil, s.dbErr("update membership failed", err)
			}
		}
	}

	if m.Status == string(model.MemberApproved) {
		return &JoinResult{Status: string(model.MemberApproved), EnterpriseName: ent.Name, JoinedAt: &now}, nil
	}
	return &JoinResult{
		Status:         string(model.MemberPending),
		EnterpriseName: ent.Name,
		Tip:            "申请已提交，请等待管理员审核",
	}, nil
}

// AdminEnterpriseItem 管理端企业列表项 (5.8)
type AdminEnterpriseItem struct {
	ID           string    `json:"id"`
	Name         string    `json:"name"`
	MemberCount  int64     `json:"member_count"`
	OrderCount   int64     `json:"order_count"`
	PendingCount int64     `json:"pending_count"`
	Status       string    `json:"status"` // active / inactive
	CreatedAt    time.Time `json:"created_at"`
}

// AdminEnterpriseList 管理端企业分页结果 (5.8)
type AdminEnterpriseList struct {
	List       []AdminEnterpriseItem `json:"list"`
	Total      int64                 `json:"total"`
	Page       int                   `json:"page"`
	PageSize   int                   `json:"page_size"`
	TotalPages int                   `json:"total_pages"`
}

// AdminEnterpriseDetail 管理端企业详情 (5.9)
type AdminEnterpriseDetail struct {
	ID                  string     `json:"id"`
	Name                string     `json:"name"`
	InviteCode          string     `json:"invite_code"`
	InviteCodeExpiresAt *time.Time `json:"invite_code_expires_at"`
	MemberCount         int64      `json:"member_count"`
	OrderCount          int64      `json:"order_count"`
	Status              string     `json:"status"`
	CreatedAt           time.Time  `json:"created_at"`
}

// AdminListEnterprises 企业列表 (5.8, 仅平台管理员), status 取值 active/inactive, 默认 active
func (s *EnterpriseService) AdminListEnterprises(ctx context.Context, page, pageSize int, keyword, status string) (*AdminEnterpriseList, error) {
	var statusVal *int
	switch status {
	case "", "active":
		v := int(model.EnterpriseActive)
		statusVal = &v
	case "inactive":
		v := int(model.EnterpriseDeleted)
		statusVal = &v
	default:
		return nil, apperrors.ErrInvalidParam.WithMessage("status 取值: active/inactive")
	}

	enterprises, total, err := s.ents.ListForAdmin(ctx, strings.TrimSpace(keyword), statusVal, (page-1)*pageSize, pageSize)
	if err != nil {
		return nil, s.dbErr("list enterprises failed", err)
	}

	entIDs := make([]string, 0, len(enterprises))
	for _, e := range enterprises {
		entIDs = append(entIDs, e.ID)
	}
	stats, err := s.ents.CountStats(ctx, entIDs)
	if err != nil {
		return nil, s.dbErr("count enterprise stats failed", err)
	}

	list := make([]AdminEnterpriseItem, 0, len(enterprises))
	for _, e := range enterprises {
		st := stats[e.ID]
		list = append(list, AdminEnterpriseItem{
			ID:           e.ID,
			Name:         e.Name,
			MemberCount:  st.MemberCount,
			OrderCount:   st.OrderCount,
			PendingCount: st.PendingCount,
			Status:       enterpriseStatusName(e.Status),
			CreatedAt:    e.CreatedAt,
		})
	}

	totalPages := int(total) / pageSize
	if int(total)%pageSize > 0 {
		totalPages++
	}
	return &AdminEnterpriseList{
		List:       list,
		Total:      total,
		Page:       page,
		PageSize:   pageSize,
		TotalPages: totalPages,
	}, nil
}

// AdminEnterpriseDetailByID 企业详情 (5.9, 仅平台管理员)
func (s *EnterpriseService) AdminEnterpriseDetailByID(ctx context.Context, enterpriseID string) (*AdminEnterpriseDetail, error) {
	ent, err := s.ents.FindByID(ctx, enterpriseID)
	if err != nil {
		return nil, s.dbErr("find enterprise failed", err)
	}
	if ent == nil {
		return nil, apperrors.ErrEnterpriseNotFound
	}

	stats, err := s.ents.CountStats(ctx, []string{ent.ID})
	if err != nil {
		return nil, s.dbErr("count enterprise stats failed", err)
	}
	st := stats[ent.ID]

	return &AdminEnterpriseDetail{
		ID:                  ent.ID,
		Name:                ent.Name,
		InviteCode:          ent.InviteCode,
		InviteCodeExpiresAt: ent.InviteCodeExpiresAt,
		MemberCount:         st.MemberCount,
		OrderCount:          st.OrderCount,
		Status:              enterpriseStatusName(ent.Status),
		CreatedAt:           ent.CreatedAt,
	}, nil
}

// enterpriseStatusName 企业状态名
func enterpriseStatusName(status int) string {
	if status == int(model.EnterpriseDeleted) {
		return "inactive"
	}
	return "active"
}

// ListMembers 成员列表 (5.10 / 3.5, 仅平台管理员), role 取值 admin/member
func (s *EnterpriseService) ListMembers(ctx context.Context, enterpriseID string, page, pageSize int, status, role, keyword string) (*MemberList, error) {
	if status != "" && !memberStatusWhitelist[status] {
		return nil, apperrors.ErrInvalidParam.WithMessage("status 取值: pending/approved/rejected/removed")
	}
	if role != "" && role != "admin" && role != "member" {
		return nil, apperrors.ErrInvalidParam.WithMessage("role 取值: admin/member")
	}

	memberships, total, err := s.mems.ListByEnterprise(ctx, enterpriseID, status, role, keyword, (page-1)*pageSize, pageSize)
	if err != nil {
		return nil, s.dbErr("list memberships failed", err)
	}

	userIDs := make([]string, 0, len(memberships))
	for _, m := range memberships {
		userIDs = append(userIDs, m.UserID)
	}
	orderCounts, err := s.mems.CountOrdersByReporter(ctx, enterpriseID, userIDs)
	if err != nil {
		return nil, s.dbErr("count orders failed", err)
	}

	list := make([]MemberItem, 0, len(memberships))
	for _, m := range memberships {
		list = append(list, MemberItem{
			MembershipID: m.ID,
			UserID:       m.UserID,
			Nickname:     m.User.Nickname,
			AvatarURL:    m.User.AvatarUrl,
			Phone:        m.User.Phone,
			Role:         memberRoleName(m.Role),
			RoleLabel:    memberRoleLabel(m.Role),
			Status:       m.Status,
			StatusLabel:  memberStatusLabel(m.Status),
			OrderCount:   orderCounts[m.UserID],
			JoinedAt:     m.JoinedAt,
		})
	}

	totalPages := int(total) / pageSize
	if int(total)%pageSize > 0 {
		totalPages++
	}
	return &MemberList{
		List:       list,
		Total:      total,
		Page:       page,
		PageSize:   pageSize,
		TotalPages: totalPages,
	}, nil
}

// Approve 批量审核通过成员 (仅平台管理员)
func (s *EnterpriseService) Approve(ctx context.Context, enterpriseID string, userIDs []string) (*BatchResult, error) {
	return s.batchUpdateStatus(ctx, enterpriseID, userIDs, model.MemberApproved)
}

// Reject 批量拒绝成员申请 (仅平台管理员)
func (s *EnterpriseService) Reject(ctx context.Context, enterpriseID string, userIDs []string) (*BatchResult, error) {
	return s.batchUpdateStatus(ctx, enterpriseID, userIDs, model.MemberRejected)
}

// batchUpdateStatus 批量更新成员状态 (仅 pending 可变更)
func (s *EnterpriseService) batchUpdateStatus(ctx context.Context, enterpriseID string, userIDs []string, target model.MemberStatus) (*BatchResult, error) {
	ent, err := s.ents.FindByID(ctx, enterpriseID)
	if err != nil {
		return nil, s.dbErr("find enterprise by id failed", err)
	}
	if ent == nil {
		return nil, apperrors.ErrEnterpriseNotFound
	}
	if len(userIDs) == 0 {
		return nil, apperrors.ErrInvalidParam.WithMessage("user_ids 不能为空")
	}

	memberships, err := s.mems.FindByEnterpriseAndUsers(ctx, enterpriseID, userIDs)
	if err != nil {
		return nil, s.dbErr("find memberships failed", err)
	}
	byUser := make(map[string]*model.Membership, len(memberships))
	for i := range memberships {
		byUser[memberships[i].UserID] = &memberships[i]
	}

	success, failed := 0, 0
	now := time.Now()
	for _, uid := range userIDs {
		m, ok := byUser[uid]
		if !ok || m.Status != string(model.MemberPending) {
			failed++
			continue
		}
		m.Status = string(target)
		if target == model.MemberApproved {
			m.JoinedAt = &now
		}
		if err := s.mems.Update(ctx, m); err != nil {
			return nil, s.dbErr("update membership failed", err)
		}
		success++
	}

	if target == model.MemberApproved {
		return &BatchResult{ApprovedCount: success, FailedCount: failed}, nil
	}
	return &BatchResult{SuccessCount: success, FailedCount: failed}, nil
}

// Remove 移除成员 (仅平台管理员)
func (s *EnterpriseService) Remove(ctx context.Context, enterpriseID, userID string) error {
	m, err := s.mems.FindByEnterpriseAndUser(ctx, enterpriseID, userID)
	if err != nil {
		return s.dbErr("find membership failed", err)
	}
	if m == nil {
		return apperrors.ErrMemberNotFound
	}
	if m.Status != string(model.MemberApproved) {
		return apperrors.ErrMemberStatusInvalid.WithMessage("仅已通过成员可移除")
	}

	now := time.Now()
	m.Status = string(model.MemberRemoved)
	m.RemovedAt = &now
	if err := s.mems.Update(ctx, m); err != nil {
		return s.dbErr("update membership failed", err)
	}
	s.logger.Info("member removed",
		zap.String("enterprise_id", enterpriseID),
		zap.String("user_id", userID))
	return nil
}

// RefreshInviteCode 刷新邀请码 (仅平台管理员)
func (s *EnterpriseService) RefreshInviteCode(ctx context.Context, enterpriseID, validity string) (*InviteCodeResult, error) {
	duration, err := parseValidity(validity)
	if err != nil {
		return nil, apperrors.ErrInvalidParam.WithMessage("validity 取值: 5mins|2hours|1days|7days|permanent")
	}

	ent, err := s.ents.FindByID(ctx, enterpriseID)
	if err != nil {
		return nil, s.dbErr("find enterprise by id failed", err)
	}
	if ent == nil {
		return nil, apperrors.ErrEnterpriseNotFound
	}

	code, err := s.generateUniqueInviteCode(ctx, ent.ID)
	if err != nil {
		return nil, err
	}
	ent.InviteCode = code

	var expiresAt *time.Time
	if duration > 0 {
		t := time.Now().Add(duration)
		expiresAt = &t
	}
	ent.InviteCodeExpiresAt = expiresAt

	if err := s.ents.Update(ctx, ent); err != nil {
		return nil, s.dbErr("update enterprise failed", err)
	}
	return &InviteCodeResult{InviteCode: code, ExpiresAt: expiresAt}, nil
}

// generateUniqueInviteCode 生成全局唯一邀请码 (重试 3 次)
func (s *EnterpriseService) generateUniqueInviteCode(ctx context.Context, excludeID string) (string, error) {
	for i := 0; i < 3; i++ {
		code, err := generateInviteCode()
		if err != nil {
			return "", apperrors.ErrGenerateCode.WithError(err)
		}
		exists, err := s.ents.ExistsInviteCode(ctx, code, excludeID)
		if err != nil {
			return "", s.dbErr("check invite code failed", err)
		}
		if !exists {
			return code, nil
		}
	}
	return "", apperrors.ErrGenerateCode
}

// dbErr 数据库错误包装
func (s *EnterpriseService) dbErr(msg string, err error) error {
	s.logger.Error(msg, zap.Error(err))
	return apperrors.ErrDatabaseError.WithError(err)
}

// generateInviteCode 生成 6 位邀请码
func generateInviteCode() (string, error) {
	b := make([]byte, 6)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	for i := range b {
		b[i] = inviteCodeChars[int(b[i])%len(inviteCodeChars)]
	}
	return string(b), nil
}

// parseValidity 解析邀请码有效期
func parseValidity(v string) (time.Duration, error) {
	switch v {
	case "5mins":
		return 5 * time.Minute, nil
	case "2hours":
		return 2 * time.Hour, nil
	case "1days":
		return 24 * time.Hour, nil
	case "7days":
		return 7 * 24 * time.Hour, nil
	case "permanent":
		return 0, nil
	default:
		return 0, errors.New("invalid validity")
	}
}

// memberRoleName 企业成员角色展示名
func memberRoleName(role int) string {
	if role == model.EnterpriseRoleAdmin {
		return "admin"
	}
	return "member"
}

// memberRoleLabel 角色中文名
func memberRoleLabel(role int) string {
	if role == model.EnterpriseRoleAdmin {
		return "管理员"
	}
	return "成员"
}

// memberStatusLabel 成员状态中文名
func memberStatusLabel(status string) string {
	switch model.MemberStatus(status) {
	case model.MemberPending:
		return "待审核"
	case model.MemberApproved:
		return "已通过"
	case model.MemberRejected:
		return "已拒绝"
	case model.MemberRemoved:
		return "已移除"
	default:
		return status
	}
}
