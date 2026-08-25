package handler

import (
	"mime"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	apperrors "xin-ni-repair/internal/errors"
	"xin-ni-repair/internal/model"
	"xin-ni-repair/internal/repository"
	"xin-ni-repair/internal/service"
	"xin-ni-repair/pkg/response"
)

// AdminHandler 管理后台接口处理器 (第五章+第六章)
type AdminHandler struct {
	orders    *service.AdminOrderService
	entSvc    *service.EnterpriseService
	export    *service.OrderExportService
	userAdmin *service.UserAdminService
	logger    *zap.Logger
}

// NewAdminHandler 创建 AdminHandler
func NewAdminHandler(orders *service.AdminOrderService, entSvc *service.EnterpriseService, export *service.OrderExportService, userAdmin *service.UserAdminService, logger *zap.Logger) *AdminHandler {
	return &AdminHandler{orders: orders, entSvc: entSvc, export: export, userAdmin: userAdmin, logger: logger}
}

// ListOrders 工单列表 (GET /admin/orders, 5.1)
func (h *AdminHandler) ListOrders(c *gin.Context) {
	page, pageSize, err := parsePageParams(c)
	if err != nil {
		h.logger.Warn("ListOrders Admin: invalid page params", zap.Error(err))
		response.FailError(c, err)
		return
	}

	statuses, err := parseStatusList(c.Query("status"))
	if err != nil {
		h.logger.Warn("ListOrders Admin: invalid status list", zap.Error(err))
		response.FailError(c, err)
		return
	}
	dateFrom, err := parseTimeParam(c.Query("date_from"))
	if err != nil {
		h.logger.Warn("ListOrders Admin: invalid date_from", zap.Error(err))
		response.FailError(c, err)
		return
	}
	dateTo, err := parseTimeParam(c.Query("date_to"))
	if err != nil {
		h.logger.Warn("ListOrders Admin: invalid date_to", zap.Error(err))
		response.FailError(c, err)
		return
	}

	filter := repository.OrderAdminFilter{
		Status:     statuses,
		Urgency:    c.Query("urgency"),
		Keyword:    c.Query("keyword"),
		DateFrom:   dateFrom,
		DateTo:     dateTo,
		ReporterID: c.Query("reporter_id"),
		SortBy:     c.Query("sort_by"),
		SortOrder:  c.Query("sort_order"),
	}
	result, err := h.orders.ListOrders(c.Request.Context(), filter, page, pageSize)
	if err != nil {
		h.logger.Error("ListOrders Admin: service error", zap.Error(err), zap.String("user_id", c.GetString("user_id")))
		response.FailError(c, err)
		return
	}
	response.OK(c, result)
}

// OrderDetail 工单详情 (GET /admin/orders/:order_id, 5.2)
func (h *AdminHandler) OrderDetail(c *gin.Context) {
	detail, err := h.orders.Detail(c.Request.Context(), c.Param("order_id"))
	if err != nil {
		h.logger.Error("OrderDetail Admin: service error", zap.Error(err), zap.String("order_id", c.Param("order_id")))
		response.FailError(c, err)
		return
	}
	response.OK(c, detail)
}

// Review 查阅工单 (POST /admin/orders/:order_id/review, 5.3)
func (h *AdminHandler) Review(c *gin.Context) {
	var req struct {
		Remark string `json:"remark"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		h.logger.Warn("Review Admin: invalid request body", zap.Error(err), zap.String("order_id", c.Param("order_id")))
		response.Fail(c, apperrors.ErrInvalidParam.WithMessage("请求体格式错误"))
		return
	}
	if err := h.orders.Review(c.Request.Context(), c.GetString("user_id"), c.Param("order_id"), req.Remark, c.ClientIP()); err != nil {
		h.logger.Error("Review Admin: service error", zap.Error(err), zap.String("order_id", c.Param("order_id")), zap.String("operator_id", c.GetString("user_id")))
		response.FailError(c, err)
		return
	}
	response.OK(c, gin.H{"status": "reviewed"})
}

// Accept 接单维修 (POST /admin/orders/:order_id/accept, 5.4)
func (h *AdminHandler) Accept(c *gin.Context) {
	var req struct {
		Remark string `json:"remark"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		h.logger.Warn("Accept Admin: invalid request body", zap.Error(err), zap.String("order_id", c.Param("order_id")))
		response.Fail(c, apperrors.ErrInvalidParam.WithMessage("请求体格式错误"))
		return
	}
	if err := h.orders.Accept(c.Request.Context(), c.GetString("user_id"), c.Param("order_id"), req.Remark, c.ClientIP()); err != nil {
		h.logger.Error("Accept Admin: service error", zap.Error(err), zap.String("order_id", c.Param("order_id")), zap.String("operator_id", c.GetString("user_id")))
		response.FailError(c, err)
		return
	}
	response.OK(c, gin.H{"status": "processing"})
}

// Reject 退回工单 (POST /admin/orders/:order_id/reject, 5.5)
func (h *AdminHandler) Reject(c *gin.Context) {
	var req struct {
		Reason string `json:"reason" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		h.logger.Warn("Reject Admin: invalid request body", zap.Error(err), zap.String("order_id", c.Param("order_id")))
		response.Fail(c, apperrors.ErrInvalidParam.WithMessage("缺少退回原因"))
		return
	}
	if err := h.orders.Reject(c.Request.Context(), c.GetString("user_id"), c.Param("order_id"), req.Reason, c.ClientIP()); err != nil {
		h.logger.Error("Reject Admin: service error", zap.Error(err), zap.String("order_id", c.Param("order_id")), zap.String("operator_id", c.GetString("user_id")))
		response.FailError(c, err)
		return
	}
	response.OK(c, gin.H{"status": "draft"})
}

// Complete 完工 (POST /admin/orders/:order_id/complete, 5.6)
func (h *AdminHandler) Complete(c *gin.Context) {
	var req struct {
		Remark        string               `json:"remark" binding:"required"`
		Receipts      []string             `json:"receipts"`
		Quantity      int                  `json:"quantity"`
		UnitPrice     float64              `json:"unit_price"`
		RepairContent string               `json:"repair_content"`
		Metadata      model.RepairMetadata `json:"metadata"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		h.logger.Warn("Complete Admin: invalid request body", zap.Error(err), zap.String("order_id", c.Param("order_id")))
		response.Fail(c, apperrors.ErrInvalidParam.WithMessage("请求体格式错误"))
		return
	}
	in := service.CompleteOrderInput{
		Remark:        req.Remark,
		Receipts:      req.Receipts,
		Quantity:      req.Quantity,
		UnitPrice:     req.UnitPrice,
		RepairContent: req.RepairContent,
		Metadata:      req.Metadata,
	}
	if err := h.orders.Complete(c.Request.Context(), c.GetString("user_id"), c.Param("order_id"), c.ClientIP(), in); err != nil {
		h.logger.Error("Complete Admin: service error", zap.Error(err), zap.String("order_id", c.Param("order_id")), zap.String("operator_id", c.GetString("user_id")))
		response.FailError(c, err)
		return
	}
	response.OK(c, gin.H{"status": "completed"})
}

// UpdateFinance 修改对账信息 (POST /admin/orders/:order_id/finance, 5.6.1)
func (h *AdminHandler) UpdateFinance(c *gin.Context) {
	var req struct {
		Quantity      *int                  `json:"quantity"`
		UnitPrice     *float64              `json:"unit_price"`
		RepairContent *string               `json:"repair_content"`
		Metadata      *model.RepairMetadata `json:"metadata"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		h.logger.Warn("UpdateFinance Admin: invalid request body", zap.Error(err), zap.String("order_id", c.Param("order_id")))
		response.Fail(c, apperrors.ErrInvalidParam.WithMessage("请求体格式错误"))
		return
	}
	in := service.UpdateFinanceInput{
		Quantity:      req.Quantity,
		UnitPrice:     req.UnitPrice,
		RepairContent: req.RepairContent,
		Metadata:      req.Metadata,
	}
	if err := h.orders.UpdateFinance(c.Request.Context(), c.GetString("user_id"), c.Param("order_id"), c.ClientIP(), in); err != nil {
		h.logger.Error("UpdateFinance Admin: service error", zap.Error(err), zap.String("order_id", c.Param("order_id")), zap.String("operator_id", c.GetString("user_id")))
		response.FailError(c, err)
		return
	}
	response.OK(c, gin.H{"status": "updated"})
}

// UploadReceipt 上传收据图片 (POST /admin/orders/:order_id/receipts, 5.7)
// 智能识别两种 Content-Type:
//   - multipart/form-data: 字段名 file (二进制文件流)
//   - application/json: { "file": "<base64>", "filename": "xxx.jpg" }
func (h *AdminHandler) UploadReceipt(c *gin.Context) {
	uf, err := ParseUploadFile(c)
	if err != nil {
		h.logger.Warn("UploadReceipt Admin: parse file failed", zap.Error(err), zap.String("order_id", c.Param("order_id")))
		response.Fail(c, apperrors.ErrInvalidParam.WithMessage("缺少 file 文件或 base64 数据"))
		return
	}
	defer uf.Close()

	result, err := h.orders.UploadReceipt(
		c.Request.Context(),
		c.GetString("user_id"),
		c.Param("order_id"),
		uf.Filename,
		uf.Size,
		uf.Reader,
		c.ClientIP(),
	)
	if err != nil {
		h.logger.Error("UploadReceipt Admin: service error", zap.Error(err), zap.String("order_id", c.Param("order_id")), zap.String("operator_id", c.GetString("user_id")))
		response.FailError(c, err)
		return
	}
	response.OK(c, result)
}

// ListEnterprises 企业列表 (GET /admin/enterprises, 5.8)
func (h *AdminHandler) ListEnterprises(c *gin.Context) {
	page, pageSize, err := parsePageParams(c)
	if err != nil {
		h.logger.Warn("ListEnterprises Admin: invalid page params", zap.Error(err))
		response.FailError(c, err)
		return
	}
	result, err := h.entSvc.AdminListEnterprises(
		c.Request.Context(),
		page,
		pageSize,
		c.Query("keyword"),
		c.Query("status"),
	)
	if err != nil {
		h.logger.Error("ListEnterprises Admin: service error", zap.Error(err))
		response.FailError(c, err)
		return
	}
	response.OK(c, result)
}

// EnterpriseDetail 企业详情 (GET /admin/enterprises/:enterprise_id, 5.9)
func (h *AdminHandler) EnterpriseDetail(c *gin.Context) {
	detail, err := h.entSvc.AdminEnterpriseDetailByID(c.Request.Context(), c.Param("enterprise_id"))
	if err != nil {
		h.logger.Error("EnterpriseDetail Admin: service error", zap.Error(err), zap.String("enterprise_id", c.Param("enterprise_id")))
		response.FailError(c, err)
		return
	}
	response.OK(c, detail)
}

// ListMembers 成员列表 (GET /admin/enterprises/:enterprise_id/members, 5.10)
func (h *AdminHandler) ListMembers(c *gin.Context) {
	page, pageSize, err := parsePageParams(c)
	if err != nil {
		h.logger.Warn("ListMembers Admin: invalid page params", zap.Error(err))
		response.FailError(c, err)
		return
	}
	result, err := h.entSvc.ListMembers(
		c.Request.Context(),
		c.Param("enterprise_id"),
		page,
		pageSize,
		c.Query("status"),
		c.Query("role"),
		c.Query("keyword"),
	)
	if err != nil {
		h.logger.Error("ListMembers Admin: service error", zap.Error(err), zap.String("enterprise_id", c.Param("enterprise_id")))
		response.FailError(c, err)
		return
	}
	response.OK(c, result)
}

// Repairers 维修员(业务员)列表 (GET /admin/repairers, 5.15)
func (h *AdminHandler) Repairers(c *gin.Context) {
	users, err := h.orders.ListRepairers(c.Request.Context())
	if err != nil {
		h.logger.Error("Repairers Admin: service error", zap.Error(err))
		response.FailError(c, err)
		return
	}
	list := make([]service.AdminReporter, 0, len(users))
	for _, u := range users {
		list = append(list, service.AdminReporter{ID: u.ID, Nickname: u.Nickname, AvatarURL: u.AvatarUrl})
	}
	response.OK(c, gin.H{"list": list})
}

// ExportOrders 导出工单记录 (GET /admin/orders/export, 5.14)
func (h *AdminHandler) ExportOrders(c *gin.Context) {
	dateFrom, err := parseDateParam(c.Query("date_from"))
	if err != nil {
		h.logger.Warn("ExportOrders Admin: invalid date_from", zap.Error(err))
		response.FailError(c, err)
		return
	}
	dateTo, err := parseDateParam(c.Query("date_to"))
	if err != nil {
		h.logger.Warn("ExportOrders Admin: invalid date_to", zap.Error(err))
		response.FailError(c, err)
		return
	}
	dateTo = dateTo.Add(24*time.Hour - time.Nanosecond) // 结束日期含当天
	fields := h.parseExportFields(c)

	req := service.ExportRequest{
		Mode:         c.Query("mode"),
		EnterpriseID: c.Query("enterprise_id"),
		RepairerID:   c.Query("repairer_id"),
		DateFrom:     dateFrom,
		DateTo:       dateTo,
		Fields:       fields,
		Status:       c.Query("status"),
	}
	result, err := h.export.Export(c.Request.Context(), req)
	if err != nil {
		h.logger.Error("ExportOrders Admin: service error", zap.Error(err), zap.String("user_id", c.GetString("user_id")))
		response.FailError(c, err)
		return
	}

	// 中文文件名需按 RFC 5987 编码 (filename*=UTF-8''...), 否则浏览器按 Latin-1 解码会乱码
	disposition := mime.FormatMediaType("attachment", map[string]string{"filename": result.Filename})
	c.Header("Content-Disposition", disposition)
	c.Data(200, result.ContentType, result.Content.Bytes())
}

// ────────────────────────────────────────────
// 辅助
// ────────────────────────────────────────────

// parseExportFields 解析导出字段参数, 兼容两种形式:
//   - fields=order_no,time,amount     (逗号分隔)
//   - fields[]=order_no&fields[]=time (数组形式, 前端常用)
func (h *AdminHandler) parseExportFields(c *gin.Context) []string {
	var fields []string
	for _, v := range c.QueryArray("fields[]") {
		fields = append(fields, strings.Split(v, ",")...)
	}
	if v := c.Query("fields"); v != "" {
		fields = append(fields, strings.Split(v, ",")...)
	}
	// 去空白与空项
	res := make([]string, 0, len(fields))
	seen := make(map[string]bool, len(fields))
	for _, f := range fields {
		f = strings.TrimSpace(f)
		if f == "" || seen[f] {
			continue
		}
		seen[f] = true
		res = append(res, f)
	}
	if len(res) == 0 {
		return nil
	}
	return res
}

// parseStatusList 解析逗号分隔的状态列表
func parseStatusList(v string) ([]string, error) {
	if v == "" {
		return nil, nil
	}
	parts := strings.Split(v, ",")
	list := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		list = append(list, p)
	}
	return list, nil
}

// parseTimeParam 解析时间参数, 支持 RFC3339 / 2006-01-02 15:04:05 / 2006-01-02
func parseTimeParam(v string) (*time.Time, error) {
	if v == "" {
		return nil, nil
	}
	layouts := []string{time.RFC3339, "2006-01-02 15:04:05", "2006-01-02"}
	for _, layout := range layouts {
		if t, err := time.ParseInLocation(layout, v, time.Local); err == nil {
			return &t, nil
		}
	}
	return nil, apperrors.ErrInvalidParam.WithMessage("时间格式错误, 支持 RFC3339 / 2006-01-02 15:04:05 / 2006-01-02")
}

// parseDateParam 解析日期参数, 格式 YYYY-MM-DD (5.14 导出必填)
func parseDateParam(v string) (time.Time, error) {
	if v == "" {
		return time.Time{}, apperrors.ErrInvalidParam.WithMessage("date_from/date_to 必填, 格式 YYYY-MM-DD")
	}
	t, err := time.ParseInLocation("2006-01-02", v, time.Local)
	if err != nil {
		return time.Time{}, apperrors.ErrInvalidParam.WithMessage("日期格式错误, 需为 YYYY-MM-DD")
	}
	return t, nil
}

// ── 第六章: 用户管理 (仅超级管理员) ──

// ListUsers 用户列表 (GET /admin/users, 6.1)
func (h *AdminHandler) ListUsers(c *gin.Context) {
	page, pageSize, err := parsePageParams(c)
	if err != nil {
		response.FailError(c, err)
		return
	}
	keyword := c.Query("keyword")

	var role *int
	if roleStr := c.Query("role"); roleStr != "" {
		r, err := strconv.Atoi(roleStr)
		if err != nil || r < 0 || r > 2 {
			response.Fail(c, apperrors.ErrInvalidParam.WithMessage("role 取值: 0/1/2"))
			return
		}
		role = &r
	}

	result, err := h.userAdmin.ListUsers(c.Request.Context(), keyword, role, page, pageSize)
	if err != nil {
		h.logger.Error("ListUsers: service error", zap.Error(err))
		response.FailError(c, err)
		return
	}
	response.OK(c, result)
}

// UserDetail 用户详情 (GET /admin/users/:user_id, 6.2)
func (h *AdminHandler) UserDetail(c *gin.Context) {
	result, err := h.userAdmin.GetUser(c.Request.Context(), c.Param("user_id"))
	if err != nil {
		h.logger.Error("UserDetail: service error", zap.Error(err), zap.String("user_id", c.Param("user_id")))
		response.FailError(c, err)
		return
	}
	response.OK(c, result)
}

// UpdateUser 更新用户属性 (PUT /admin/users/:user_id, 6.3)
func (h *AdminHandler) UpdateUser(c *gin.Context) {
	var body struct {
		Nickname *string `json:"nickname"`
		Role     *int    `json:"role"`
		Phone    *string `json:"phone"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		response.Fail(c, apperrors.ErrInvalidParam.WithMessage("请求体格式错误"))
		return
	}

	input := service.UpdateUserInput{
		Nickname: body.Nickname,
		Role:     body.Role,
		Phone:    body.Phone,
	}
	result, err := h.userAdmin.UpdateUser(c.Request.Context(), c.Param("user_id"), c.GetString("user_id"), input)
	if err != nil {
		h.logger.Error("UpdateUser: service error", zap.Error(err), zap.String("user_id", c.Param("user_id")), zap.String("operator_id", c.GetString("user_id")))
		response.FailError(c, err)
		return
	}
	response.OK(c, result)
}

// ResetPassword 重置用户密码 (POST /admin/users/:user_id/reset-password, 6.4)
func (h *AdminHandler) ResetPassword(c *gin.Context) {
	var body struct {
		NewPassword string `json:"new_password"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		response.Fail(c, apperrors.ErrInvalidParam.WithMessage("请求体格式错误"))
		return
	}

	err := h.userAdmin.ResetPassword(c.Request.Context(), c.Param("user_id"), body.NewPassword)
	if err != nil {
		h.logger.Error("ResetPassword: service error", zap.Error(err), zap.String("user_id", c.Param("user_id")), zap.String("operator_id", c.GetString("user_id")))
		response.FailError(c, err)
		return
	}
	response.OK(c, gin.H{
		"id":         c.Param("user_id"),
		"updated_at": time.Now().Format(time.RFC3339),
	})
}
