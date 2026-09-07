package handler

import (
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	apperrors "xin-ni-repair/internal/errors"
	"xin-ni-repair/internal/service"
	"xin-ni-repair/pkg/response"
)

// ProjectHandler 项目字典管理接口 (6.5, 仅超级管理员; 中间件 RequireSuperAdmin 在路由层统一校验)
type ProjectHandler struct {
	svc    *service.ProjectService
	logger *zap.Logger
}

// NewProjectHandler 创建 ProjectHandler
func NewProjectHandler(svc *service.ProjectService, logger *zap.Logger) *ProjectHandler {
	return &ProjectHandler{svc: svc, logger: logger}
}

// ── 项目大类 (project_categories) ──

// ListCategories GET /admin/categories
func (h *ProjectHandler) ListCategories(c *gin.Context) {
	list, err := h.svc.ListCategories(c.Request.Context())
	if err != nil {
		h.logger.Error("ListCategories: service error", zap.Error(err), zap.String("operator_id", c.GetString("user_id")))
		response.FailError(c, err)
		return
	}
	response.OK(c, gin.H{"list": list})
}

// CreateCategory POST /admin/categories
func (h *ProjectHandler) CreateCategory(c *gin.Context) {
	var in service.CategoryInput
	if err := c.ShouldBindJSON(&in); err != nil {
		response.Fail(c, apperrors.ErrInvalidParam.WithMessage("请求体格式错误"))
		return
	}
	view, err := h.svc.CreateCategory(c.Request.Context(), in)
	if err != nil {
		h.logger.Error("CreateCategory: service error", zap.Error(err))
		response.FailError(c, err)
		return
	}
	response.Created(c, view)
}

// UpdateCategory PUT /admin/categories/:category_id
func (h *ProjectHandler) UpdateCategory(c *gin.Context) {
	var in service.CategoryInput
	if err := c.ShouldBindJSON(&in); err != nil {
		response.Fail(c, apperrors.ErrInvalidParam.WithMessage("请求体格式错误"))
		return
	}
	view, err := h.svc.UpdateCategory(c.Request.Context(), c.Param("category_id"), in)
	if err != nil {
		h.logger.Error("UpdateCategory: service error", zap.Error(err), zap.String("category_id", c.Param("category_id")))
		response.FailError(c, err)
		return
	}
	response.OK(c, view)
}

// DeleteCategory DELETE /admin/categories/:category_id
func (h *ProjectHandler) DeleteCategory(c *gin.Context) {
	if err := h.svc.DeleteCategory(c.Request.Context(), c.Param("category_id")); err != nil {
		h.logger.Error("DeleteCategory: service error", zap.Error(err), zap.String("category_id", c.Param("category_id")))
		response.FailError(c, err)
		return
	}
	response.OK(c, gin.H{"deleted": true})
}

// ── 项目属性 (project_properties) ──

// ListProperties GET /admin/properties?category_id=xxx
func (h *ProjectHandler) ListProperties(c *gin.Context) {
	list, err := h.svc.ListProperties(c.Request.Context(), c.Query("category_id"))
	if err != nil {
		h.logger.Error("ListProperties: service error", zap.Error(err))
		response.FailError(c, err)
		return
	}
	response.OK(c, gin.H{"list": list})
}

// CreateProperty POST /admin/properties
func (h *ProjectHandler) CreateProperty(c *gin.Context) {
	var in service.PropertyInput
	if err := c.ShouldBindJSON(&in); err != nil {
		response.Fail(c, apperrors.ErrInvalidParam.WithMessage("请求体格式错误"))
		return
	}
	view, err := h.svc.CreateProperty(c.Request.Context(), in)
	if err != nil {
		h.logger.Error("CreateProperty: service error", zap.Error(err))
		response.FailError(c, err)
		return
	}
	response.Created(c, view)
}

// UpdateProperty PUT /admin/properties/:property_id
func (h *ProjectHandler) UpdateProperty(c *gin.Context) {
	var in service.PropertyInput
	if err := c.ShouldBindJSON(&in); err != nil {
		response.Fail(c, apperrors.ErrInvalidParam.WithMessage("请求体格式错误"))
		return
	}
	view, err := h.svc.UpdateProperty(c.Request.Context(), c.Param("property_id"), in)
	if err != nil {
		h.logger.Error("UpdateProperty: service error", zap.Error(err), zap.String("property_id", c.Param("property_id")))
		response.FailError(c, err)
		return
	}
	response.OK(c, view)
}

// DeleteProperty DELETE /admin/properties/:property_id
func (h *ProjectHandler) DeleteProperty(c *gin.Context) {
	if err := h.svc.DeleteProperty(c.Request.Context(), c.Param("property_id")); err != nil {
		h.logger.Error("DeleteProperty: service error", zap.Error(err), zap.String("property_id", c.Param("property_id")))
		response.FailError(c, err)
		return
	}
	response.OK(c, gin.H{"deleted": true})
}

// ── 常见问题 (project_problems) ──

// ListProblems GET /admin/problems?property_id=xxx (V1.4: 问题隶属属性)
func (h *ProjectHandler) ListProblems(c *gin.Context) {
	list, err := h.svc.ListProblems(c.Request.Context(), c.Query("property_id"))
	if err != nil {
		h.logger.Error("ListProblems: service error", zap.Error(err))
		response.FailError(c, err)
		return
	}
	response.OK(c, gin.H{"list": list})
}

// CreateProblem POST /admin/problems
func (h *ProjectHandler) CreateProblem(c *gin.Context) {
	var in service.ProblemInput
	if err := c.ShouldBindJSON(&in); err != nil {
		response.Fail(c, apperrors.ErrInvalidParam.WithMessage("请求体格式错误"))
		return
	}
	view, err := h.svc.CreateProblem(c.Request.Context(), in)
	if err != nil {
		h.logger.Error("CreateProblem: service error", zap.Error(err))
		response.FailError(c, err)
		return
	}
	response.Created(c, view)
}

// UpdateProblem PUT /admin/problems/:problem_id
func (h *ProjectHandler) UpdateProblem(c *gin.Context) {
	var in service.ProblemInput
	if err := c.ShouldBindJSON(&in); err != nil {
		response.Fail(c, apperrors.ErrInvalidParam.WithMessage("请求体格式错误"))
		return
	}
	view, err := h.svc.UpdateProblem(c.Request.Context(), c.Param("problem_id"), in)
	if err != nil {
		h.logger.Error("UpdateProblem: service error", zap.Error(err), zap.String("problem_id", c.Param("problem_id")))
		response.FailError(c, err)
		return
	}
	response.OK(c, view)
}

// DeleteProblem DELETE /admin/problems/:problem_id
func (h *ProjectHandler) DeleteProblem(c *gin.Context) {
	if err := h.svc.DeleteProblem(c.Request.Context(), c.Param("problem_id")); err != nil {
		h.logger.Error("DeleteProblem: service error", zap.Error(err), zap.String("problem_id", c.Param("problem_id")))
		response.FailError(c, err)
		return
	}
	response.OK(c, gin.H{"deleted": true})
}
