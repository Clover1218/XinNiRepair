package handler

import (
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	apperrors "xin-ni-repair/internal/errors"
	"xin-ni-repair/internal/service"
	"xin-ni-repair/pkg/response"
)

// SettingsHandler 系统设置接口处理器（仅超级管理员）
type SettingsHandler struct {
	settings *service.SettingsService
	logger   *zap.Logger
}

// NewSettingsHandler 创建 SettingsHandler
func NewSettingsHandler(settings *service.SettingsService, logger *zap.Logger) *SettingsHandler {
	return &SettingsHandler{settings: settings, logger: logger}
}

// List 列出全部可配置项 (GET /admin/settings)
func (h *SettingsHandler) List(c *gin.Context) {
	items, err := h.settings.List(c.Request.Context())
	if err != nil {
		h.logger.Error("Settings List: service error", zap.Error(err))
		response.FailError(c, err)
		return
	}
	response.OK(c, gin.H{"settings": items})
}

// Update 更新单个设置项 (PUT /admin/settings)
func (h *SettingsHandler) Update(c *gin.Context) {
	var req struct {
		Key   string `json:"key" binding:"required"`
		Value string `json:"value" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		h.logger.Warn("Settings Update: invalid request body", zap.Error(err))
		response.Fail(c, apperrors.ErrInvalidParam.WithMessage("请求体格式错误"))
		return
	}
	if err := h.settings.Update(c.Request.Context(), opOf(c), req.Key, req.Value); err != nil {
		h.logger.Error("Settings Update: service error", zap.Error(err), zap.String("key", req.Key))
		response.FailError(c, err)
		return
	}
	response.OK(c, gin.H{"status": "updated"})
}
