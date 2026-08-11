// Package response 提供统一的 API 响应格式。
package response

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"

	apperrors "xin-ni-repair/internal/errors"
)

// Response 统一响应结构
type Response struct {
	Code      int         `json:"code"`                 // 业务状态码
	Message   string      `json:"message"`              // 提示信息
	Data      interface{} `json:"data,omitempty"`       // 响应数据
	RequestID string      `json:"request_id,omitempty"` // 请求追踪ID
}

// PageData 分页数据包装
type PageData struct {
	List       interface{} `json:"list"`
	Total      int64       `json:"total"`
	Page       int         `json:"page"`
	PageSize   int         `json:"page_size"`
	TotalPages int         `json:"total_pages"`
}

// OK 成功响应
func OK(c *gin.Context, data interface{}) {
	c.JSON(http.StatusOK, Response{
		Code:    0,
		Message: "success",
		Data:    data,
	})
}

// OKWithMessage 成功响应 (自定义消息)
func OKWithMessage(c *gin.Context, message string, data interface{}) {
	c.JSON(http.StatusOK, Response{
		Code:    0,
		Message: message,
		Data:    data,
	})
}

// Created 创建成功响应
func Created(c *gin.Context, data interface{}) {
	c.JSON(http.StatusCreated, Response{
		Code:    0,
		Message: "created",
		Data:    data,
	})
}

// Fail 失败响应 (根据错误码自动映射 HTTP 状态码)
func Fail(c *gin.Context, err *apperrors.BizError) {
	status := httpStatus(err.Code)
	resp := Response{
		Code:    err.Code,
		Message: err.Message,
	}
	if c.Request != nil {
		if rid, ok := c.Get("request_id"); ok {
			resp.RequestID = rid.(string)
		}
	}
	c.AbortWithStatusJSON(status, resp)
}

// FailError 从任意 error 中提取 BizError 输出失败响应, 未知错误归为服务器内部错误
func FailError(c *gin.Context, err error) {
	var bizErr *apperrors.BizError
	if errors.As(err, &bizErr) {
		Fail(c, bizErr)
		return
	}
	Fail(c, apperrors.ErrInternal.WithError(err))
}

// Error 通用错误响应
func Error(c *gin.Context, httpStatus int, code int, message string) {
	c.AbortWithStatusJSON(httpStatus, Response{
		Code:    code,
		Message: message,
	})
}

// Page 分页成功响应
func Page(c *gin.Context, list interface{}, total int64, page, pageSize int) {
	totalPages := int(total) / pageSize
	if int(total)%pageSize > 0 {
		totalPages++
	}
	OK(c, PageData{
		List:       list,
		Total:      total,
		Page:       page,
		PageSize:   pageSize,
		TotalPages: totalPages,
	})
}

// httpStatus 业务码 → HTTP 状态码映射
func httpStatus(code int) int {
	switch {
	case code >= 1000 && code < 2000:
		return http.StatusUnauthorized // 认证相关
	case code >= 2000 && code < 3000:
		return http.StatusForbidden // 权限相关
	case code >= 3000 && code < 4000:
		return http.StatusBadRequest // 参数校验
	case code >= 4000 && code < 5000:
		return http.StatusNotFound // 资源不存在
	case code >= 5000 && code < 6000:
		return http.StatusInternalServerError // 服务端错误
	default:
		return http.StatusInternalServerError
	}
}
