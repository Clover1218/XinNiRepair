package handler

import (
	"github.com/gin-gonic/gin"

	apperrors "xin-ni-repair/internal/errors"
	"xin-ni-repair/internal/service"
	"xin-ni-repair/pkg/imagebed"
	"xin-ni-repair/pkg/response"
)

// AuthHandler 认证接口处理器
type AuthHandler struct {
	svc    *service.AuthService
	imgBed *imagebed.Client
}

// NewAuthHandler 创建 AuthHandler
func NewAuthHandler(svc *service.AuthService, imgBed *imagebed.Client) *AuthHandler {
	return &AuthHandler{svc: svc, imgBed: imgBed}
}

// Login 微信回调登录 (POST /auth/login)
func (h *AuthHandler) Login(c *gin.Context) {
	var req struct {
		Code string `json:"code" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, apperrors.ErrInvalidParam.WithMessage("缺少微信登录 code"))
		return
	}

	result, err := h.svc.Login(c.Request.Context(), req.Code)
	if err != nil {
		response.FailError(c, err)
		return
	}
	response.OK(c, result)
}

// AdminLogin 管理后台密码登录 (POST /auth/admin-login, 2.4)
func (h *AuthHandler) AdminLogin(c *gin.Context) {
	var req struct {
		Nickname string `json:"nickname" binding:"required"`
		Password string `json:"password" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, apperrors.ErrInvalidParam.WithMessage("缺少昵称或密码"))
		return
	}
	result, err := h.svc.AdminLogin(c.Request.Context(), req.Nickname, req.Password)
	if err != nil {
		response.FailError(c, err)
		return
	}
	response.OK(c, result)
}

// Register 新用户资料完善注册 (POST /auth/register, 2.5)
// phone_code(微信授权) 与 phone(手动输入) 至少传一个, 同时传以 phone_code 为准
func (h *AuthHandler) Register(c *gin.Context) {
	var req struct {
		Code      string `json:"code" binding:"required"`
		Nickname  string `json:"nickname" binding:"required"`
		AvatarURL string `json:"avatar_url"`
		PhoneCode string `json:"phone_code"`
		Phone     string `json:"phone"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, apperrors.ErrInvalidParam.WithMessage("缺少必填字段"))
		return
	}
	if req.PhoneCode == "" && req.Phone == "" {
		response.Fail(c, apperrors.ErrInvalidParam.WithMessage("请微信授权获取手机号或手动输入手机号"))
		return
	}
	result, err := h.svc.Register(c.Request.Context(), req.Code, req.Nickname, req.AvatarURL, req.PhoneCode, req.Phone)
	if err != nil {
		response.FailError(c, err)
		return
	}
	response.OK(c, result)
}

// Me 获取当前用户信息 (GET /auth/me)
func (h *AuthHandler) Me(c *gin.Context) {
	profile, err := h.svc.Me(c.Request.Context(), c.GetString("user_id"))
	if err != nil {
		response.FailError(c, err)
		return
	}
	response.OK(c, profile)
}

// BindPhone 绑定手机号 (PUT /auth/bind-phone)
func (h *AuthHandler) BindPhone(c *gin.Context) {
	var req struct {
		Phone string `json:"phone" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, apperrors.ErrInvalidParam.WithMessage("缺少手机号"))
		return
	}

	if err := h.svc.BindPhone(c.Request.Context(), c.GetString("user_id"), req.Phone); err != nil {
		response.FailError(c, err)
		return
	}
	response.OK(c, gin.H{"phone": req.Phone})
}
