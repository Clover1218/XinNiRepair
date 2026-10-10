// 维修业务员经营范围管理接口 (V1.5)。
//
// 路由全部要求超级管理员 (RequireSuperAdmin)，挂在 /api/v1/admin 下。
//
// 对应需求: 问题 #10 设置可接单企业, 不能所有企业都能查看
//
//	问题 #28 按权限范围内企业查询统计, 防止企业名单泄露
package handler

import (
	"go.uber.org/zap"

	apperrors "xin-ni-repair/internal/errors"

	"xin-ni-repair/internal/service"
	"xin-ni-repair/pkg/response"

	"github.com/gin-gonic/gin"
)

// RepairerScopeHandler 经营范围管理
type RepairerScopeHandler struct {
	scopeSvc *service.RepairerScopeService
	logger   *zap.Logger
}

// NewRepairerScopeHandler 创建 RepairerScopeHandler
func NewRepairerScopeHandler(scopeSvc *service.RepairerScopeService, logger *zap.Logger) *RepairerScopeHandler {
	return &RepairerScopeHandler{scopeSvc: scopeSvc, logger: logger}
}

// GetScope 某业务员的经营范围
// GET /api/v1/admin/repairers/:repairer_id/enterprises
func (h *RepairerScopeHandler) GetScope(c *gin.Context) {
	id := c.Param("repairer_id")
	if id == "" {
		response.FailError(c, apperrors.ErrInvalidParam.WithMessage("缺少 repairer_id"))
		return
	}
	result, err := h.scopeSvc.GetScope(c.Request.Context(), id)
	if err != nil {
		h.logger.Error("Scope.GetScope: service error", zap.Error(err), zap.String("repairer_id", id))
		response.FailError(c, err)
		return
	}
	response.OK(c, result)
}

// UpdateScope 全量覆盖某业务员的经营范围
// PUT /api/v1/admin/repairers/:repairer_id/enterprises
func (h *RepairerScopeHandler) UpdateScope(c *gin.Context) {
	id := c.Param("repairer_id")
	if id == "" {
		response.FailError(c, apperrors.ErrInvalidParam.WithMessage("缺少 repairer_id"))
		return
	}
	var in service.UpdateRepairerScopeInput
	if err := c.ShouldBindJSON(&in); err != nil {
		h.logger.Warn("Scope.UpdateScope: invalid body", zap.Error(err))
		response.FailError(c, apperrors.ErrInvalidParam.WithMessage("请求体格式错误"))
		return
	}
	result, err := h.scopeSvc.UpdateScope(c.Request.Context(), opOf(c), id, in)
	if err != nil {
		h.logger.Error("Scope.UpdateScope: service error", zap.Error(err), zap.String("repairer_id", id))
		response.FailError(c, err)
		return
	}
	response.OK(c, result)
}

// MyScope 当前业务员自身的可见范围
// GET /api/v1/admin/my/enterprises
func (h *RepairerScopeHandler) MyScope(c *gin.Context) {
	result, err := h.scopeSvc.GetMyScope(c.Request.Context(), opOf(c))
	if err != nil {
		h.logger.Error("Scope.MyScope: service error", zap.Error(err), zap.String("user_id", c.GetString("user_id")))
		response.FailError(c, err)
		return
	}
	response.OK(c, result)
}

// ListByEnterprise 服务某企业的业务员（反向查询）
// GET /api/v1/admin/enterprises/:enterprise_id/repairers
func (h *RepairerScopeHandler) ListByEnterprise(c *gin.Context) {
	id := c.Param("enterprise_id")
	if id == "" {
		response.FailError(c, apperrors.ErrInvalidParam.WithMessage("缺少 enterprise_id"))
		return
	}
	list, err := h.scopeSvc.ListByEnterprise(c.Request.Context(), id)
	if err != nil {
		h.logger.Error("Scope.ListByEnterprise: service error", zap.Error(err), zap.String("enterprise_id", id))
		response.FailError(c, err)
		return
	}
	response.OK(c, list)
}
