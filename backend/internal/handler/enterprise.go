package handler

import (
	"strconv"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	apperrors "xin-ni-repair/internal/errors"
	"xin-ni-repair/internal/service"
	"xin-ni-repair/pkg/response"
)

// EnterpriseHandler 企业管理接口处理器
type EnterpriseHandler struct {
	svc    *service.EnterpriseService
	logger *zap.Logger
}

// NewEnterpriseHandler 创建 EnterpriseHandler
func NewEnterpriseHandler(svc *service.EnterpriseService, logger *zap.Logger) *EnterpriseHandler {
	return &EnterpriseHandler{svc: svc, logger: logger}
}

// Create 创建企业 (POST /enterprises, 仅平台管理员)
func (h *EnterpriseHandler) Create(c *gin.Context) {
	var req struct {
		Name string `json:"name" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		h.logger.Warn("Create Enterprise: invalid request body", zap.Error(err), zap.String("user_id", c.GetString("user_id")))
		response.Fail(c, apperrors.ErrInvalidParam.WithMessage("缺少企业名称"))
		return
	}

	detail, err := h.svc.Create(c.Request.Context(), req.Name)
	if err != nil {
		h.logger.Error("Create Enterprise: service error", zap.Error(err), zap.String("user_id", c.GetString("user_id")), zap.String("name", req.Name))
		response.FailError(c, err)
		return
	}
	response.Created(c, detail)
}

// Get 获取企业信息 (GET /enterprises/:enterprise_id)
func (h *EnterpriseHandler) Get(c *gin.Context) {
	detail, err := h.svc.Get(
		c.Request.Context(),
		c.GetString("user_id"),
		c.GetInt("role"),
		c.Param("enterprise_id"),
	)
	if err != nil {
		h.logger.Error("Get Enterprise: service error", zap.Error(err), zap.String("enterprise_id", c.Param("enterprise_id")), zap.String("user_id", c.GetString("user_id")))
		response.FailError(c, err)
		return
	}
	response.OK(c, detail)
}

// Update 更新企业设置 (PUT /enterprises/:enterprise_id, 仅平台管理员)
func (h *EnterpriseHandler) Update(c *gin.Context) {
	var req struct {
		Name        *string `json:"name"`
		AutoApprove *bool   `json:"auto_approve"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		h.logger.Warn("Update Enterprise: invalid request body", zap.Error(err), zap.String("enterprise_id", c.Param("enterprise_id")))
		response.Fail(c, apperrors.ErrInvalidParam.WithMessage("请求体格式错误"))
		return
	}

	detail, err := h.svc.Update(c.Request.Context(), c.Param("enterprise_id"), req.Name, req.AutoApprove)
	if err != nil {
		h.logger.Error("Update Enterprise: service error", zap.Error(err), zap.String("enterprise_id", c.Param("enterprise_id")))
		response.FailError(c, err)
		return
	}
	response.OK(c, detail)
}

// Join 加入企业 (POST /enterprises/join, 仅凭邀请码)
func (h *EnterpriseHandler) Join(c *gin.Context) {
	var req struct {
		InviteCode string `json:"invite_code" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		h.logger.Warn("Join Enterprise: invalid request body", zap.Error(err))
		response.Fail(c, apperrors.ErrInvalidParam.WithMessage("缺少邀请码"))
		return
	}

	result, err := h.svc.Join(c.Request.Context(), c.GetString("user_id"), req.InviteCode)
	if err != nil {
		h.logger.Error("Join Enterprise: service error", zap.Error(err), zap.String("user_id", c.GetString("user_id")), zap.String("invite_code", req.InviteCode))
		response.FailError(c, err)
		return
	}
	response.OK(c, result)
}

// JoinByGet 加入企业 (GET /enterprises/join?code=xxx, 与 POST 逻辑相同, 扫码场景)
func (h *EnterpriseHandler) JoinByGet(c *gin.Context) {
	code := c.Query("code")
	if code == "" {
		h.logger.Warn("JoinByGet Enterprise: missing code")
		response.Fail(c, apperrors.ErrInvalidParam.WithMessage("缺少邀请码"))
		return
	}

	result, err := h.svc.Join(c.Request.Context(), c.GetString("user_id"), code)
	if err != nil {
		h.logger.Error("JoinByGet Enterprise: service error", zap.Error(err), zap.String("user_id", c.GetString("user_id")), zap.String("invite_code", code))
		response.FailError(c, err)
		return
	}
	response.OK(c, result)
}

// ListMembers 成员列表 (GET /enterprises/:enterprise_id/members, 仅平台管理员)
func (h *EnterpriseHandler) ListMembers(c *gin.Context) {
	page, pageSize, err := parsePageParams(c)
	if err != nil {
		h.logger.Warn("ListMembers Enterprise: invalid page params", zap.Error(err))
		response.FailError(c, err)
		return
	}

	result, err := h.svc.ListMembers(
		c.Request.Context(),
		c.Param("enterprise_id"),
		page,
		pageSize,
		c.Query("status"),
		c.Query("role"),
		c.Query("keyword"),
	)
	if err != nil {
		h.logger.Error("ListMembers Enterprise: service error", zap.Error(err), zap.String("enterprise_id", c.Param("enterprise_id")))
		response.FailError(c, err)
		return
	}
	response.OK(c, result)
}

// Approve 批量审核通过 (PUT /enterprises/:enterprise_id/members/approve, 仅平台管理员)
func (h *EnterpriseHandler) Approve(c *gin.Context) {
	userIDs, ok := h.bindUserIDs(c)
	if !ok {
		return
	}
	result, err := h.svc.Approve(c.Request.Context(), c.Param("enterprise_id"), userIDs)
	if err != nil {
		h.logger.Error("Approve Member: service error", zap.Error(err), zap.String("enterprise_id", c.Param("enterprise_id")), zap.Strings("user_ids", userIDs))
		response.FailError(c, err)
		return
	}
	response.OK(c, result)
}

// Reject 批量拒绝申请 (PUT /enterprises/:enterprise_id/members/reject, 仅平台管理员)
func (h *EnterpriseHandler) Reject(c *gin.Context) {
	userIDs, ok := h.bindUserIDs(c)
	if !ok {
		return
	}
	result, err := h.svc.Reject(c.Request.Context(), c.Param("enterprise_id"), userIDs)
	if err != nil {
		h.logger.Error("Reject Member: service error", zap.Error(err), zap.String("enterprise_id", c.Param("enterprise_id")), zap.Strings("user_ids", userIDs))
		response.FailError(c, err)
		return
	}
	response.OK(c, result)
}

// Remove 移除成员 (DELETE /enterprises/:enterprise_id/members/:user_id, 仅平台管理员)
func (h *EnterpriseHandler) Remove(c *gin.Context) {
	if err := h.svc.Remove(c.Request.Context(), c.Param("enterprise_id"), c.Param("user_id")); err != nil {
		h.logger.Error("Remove Member: service error", zap.Error(err), zap.String("enterprise_id", c.Param("enterprise_id")), zap.String("user_id", c.Param("user_id")))
		response.FailError(c, err)
		return
	}
	response.OK(c, gin.H{"removed": true})
}

// RefreshCode 刷新邀请码 (POST /enterprises/:enterprise_id/refresh/code, 仅平台管理员)
func (h *EnterpriseHandler) RefreshCode(c *gin.Context) {
	var req struct {
		Validity string `json:"validity" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		h.logger.Warn("RefreshCode Enterprise: invalid request body", zap.Error(err))
		response.Fail(c, apperrors.ErrInvalidParam.WithMessage("缺少 validity"))
		return
	}

	result, err := h.svc.RefreshInviteCode(c.Request.Context(), c.Param("enterprise_id"), req.Validity)
	if err != nil {
		h.logger.Error("RefreshCode Enterprise: service error", zap.Error(err), zap.String("enterprise_id", c.Param("enterprise_id")))
		response.FailError(c, err)
		return
	}
	response.OK(c, result)
}

// bindUserIDs 绑定批量操作请求体
func (h *EnterpriseHandler) bindUserIDs(c *gin.Context) ([]string, bool) {
	var req struct {
		UserIDs []string `json:"user_ids" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, apperrors.ErrInvalidParam.WithMessage("缺少 user_ids"))
		return nil, false
	}
	return req.UserIDs, true
}

// parsePageParams 解析分页参数, 非法时返回 ErrInvalidPage
func parsePageParams(c *gin.Context) (int, int, error) {
	page, err := parseQueryInt(c, "page", 1)
	if err != nil {
		return 0, 0, err
	}
	pageSize, err := parseQueryInt(c, "page_size", 20)
	if err != nil {
		return 0, 0, err
	}
	if page < 1 {
		return 0, 0, apperrors.ErrInvalidPage
	}
	if pageSize > 100 {
		pageSize = 100
	}
	if pageSize < 1 {
		return 0, 0, apperrors.ErrInvalidPage
	}
	return page, pageSize, nil
}

func parseQueryInt(c *gin.Context, key string, def int) (int, error) {
	v := c.Query(key)
	if v == "" {
		return def, nil
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return 0, apperrors.ErrInvalidPage
	}
	return n, nil
}
