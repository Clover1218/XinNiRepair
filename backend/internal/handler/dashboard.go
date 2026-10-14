package handler

import (
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"xin-ni-repair/internal/service"
	"xin-ni-repair/pkg/response"
)

// DashboardHandler 首页待办看板处理器
type DashboardHandler struct {
	svc    *service.DashboardService
	logger *zap.Logger
}

// NewDashboardHandler 创建 DashboardHandler
func NewDashboardHandler(svc *service.DashboardService, logger *zap.Logger) *DashboardHandler {
	return &DashboardHandler{svc: svc, logger: logger}
}

// Dashboard 首页待办看板 (GET /dashboard)
//
// 按角色返回 3-4 张状态卡（数量 + 未读数 + 最近 1-2 条工单）。
// enterprise_id 仅对单位审核员生效（首页多单位切换），其余角色忽略。
func (h *DashboardHandler) Dashboard(c *gin.Context) {
	result, err := h.svc.Dashboard(c.Request.Context(), opOf(c), c.Query("enterprise_id"))
	if err != nil {
		h.logger.Warn("Dashboard: service error", zap.Error(err), zap.String("user_id", c.GetString("user_id")))
		response.FailError(c, err)
		return
	}
	response.OK(c, result)
}

// MarkRead 标记工单已读 (POST /orders/:order_id/read)
//
// 进工单详情页时调用，写入「该用户 × 该工单」的已读时间，首页红点随即消失。
func (h *DashboardHandler) MarkRead(c *gin.Context) {
	if err := h.svc.MarkRead(c.Request.Context(), opOf(c), c.Param("order_id")); err != nil {
		h.logger.Warn("MarkRead: service error", zap.Error(err), zap.String("user_id", c.GetString("user_id")))
		response.FailError(c, err)
		return
	}
	response.OK(c, nil)
}
