package handler

import (
	"strconv"

	"github.com/gin-gonic/gin"

	apperrors "xin-ni-repair/internal/errors"
	"xin-ni-repair/internal/service"
	"xin-ni-repair/pkg/response"
)

// OrderHandler 工单接口处理器 (用户端)
type OrderHandler struct {
	svc *service.OrderService
}

// NewOrderHandler 创建 OrderHandler
func NewOrderHandler(svc *service.OrderService) *OrderHandler {
	return &OrderHandler{svc: svc}
}

// Options 查询新建工单可选枚举 (GET /orders/options)
func (h *OrderHandler) Options(c *gin.Context) {
	options, err := h.svc.Options(c.Request.Context(), c.GetString("user_id"))
	if err != nil {
		response.FailError(c, err)
		return
	}
	response.OK(c, options)
}

// Create 创建空草稿 (POST /orders, 请求体为空)
func (h *OrderHandler) Create(c *gin.Context) {
	result, err := h.svc.Create(c.Request.Context(), c.GetString("user_id"))
	if err != nil {
		response.FailError(c, err)
		return
	}
	response.Created(c, result)
}

// List 我的工单列表 (GET /orders)
func (h *OrderHandler) List(c *gin.Context) {
	page, pageSize, err := parsePageParams(c)
	if err != nil {
		response.FailError(c, err)
		return
	}
	result, err := h.svc.List(
		c.Request.Context(),
		c.GetString("user_id"),
		c.Query("enterprise_id"),
		c.Query("status"),
		page,
		pageSize,
	)
	if err != nil {
		response.FailError(c, err)
		return
	}
	response.OK(c, result)
}

// Detail 工单详情 (GET /orders/:order_id)
func (h *OrderHandler) Detail(c *gin.Context) {
	detail, err := h.svc.Detail(c.Request.Context(), c.GetString("user_id"), c.Param("order_id"))
	if err != nil {
		response.FailError(c, err)
		return
	}
	response.OK(c, detail)
}

// Update 更新草稿 (PUT /orders/:order_id)
func (h *OrderHandler) Update(c *gin.Context) {
	var req service.UpdateOrderInput
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, apperrors.ErrInvalidParam.WithMessage("请求体格式错误"))
		return
	}
	result, err := h.svc.Update(c.Request.Context(), c.GetString("user_id"), c.Param("order_id"), req)
	if err != nil {
		response.FailError(c, err)
		return
	}
	response.OK(c, result)
}

// Submit 提交上报 (POST /orders/:order_id/submit)
func (h *OrderHandler) Submit(c *gin.Context) {
	result, err := h.svc.Submit(c.Request.Context(), c.GetString("user_id"), c.Param("order_id"))
	if err != nil {
		response.FailError(c, err)
		return
	}
	response.OK(c, result)
}

// Delete 删除草稿 (DELETE /orders/:order_id)
func (h *OrderHandler) Delete(c *gin.Context) {
	if err := h.svc.Delete(c.Request.Context(), c.GetString("user_id"), c.Param("order_id")); err != nil {
		response.FailError(c, err)
		return
	}
	response.OK(c, gin.H{"deleted": true})
}

// Cancel 取消工单 (POST /orders/:order_id/cancel)
func (h *OrderHandler) Cancel(c *gin.Context) {
	var req struct {
		Reason string `json:"reason"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, apperrors.ErrInvalidParam.WithMessage("请求体格式错误"))
		return
	}
	if err := h.svc.Cancel(c.Request.Context(), c.GetString("user_id"), c.Param("order_id"), req.Reason); err != nil {
		response.FailError(c, err)
		return
	}
	response.OK(c, gin.H{"cancelled": true})
}

// UploadImage 图片上传 (POST /orders/:order_id/images)
func (h *OrderHandler) UploadImage(c *gin.Context) {
	file, err := c.FormFile("file")
	if err != nil {
		response.Fail(c, apperrors.ErrInvalidParam.WithMessage("缺少 file 文件"))
		return
	}
	sortOrder, _ := strconv.Atoi(c.PostForm("sort_order"))

	f, err := file.Open()
	if err != nil {
		response.FailError(c, err)
		return
	}
	defer f.Close()

	result, err := h.svc.UploadImage(
		c.Request.Context(),
		c.GetString("user_id"),
		c.Param("order_id"),
		file.Filename,
		file.Size,
		f,
		sortOrder,
	)
	if err != nil {
		response.FailError(c, err)
		return
	}
	response.OK(c, result)
}
