// ProjectService 项目字典管理 (6.5, 超级管理员专属)。
//
// 对应《数据库字段设计文档 V1.4》2.4~2.6 三张表:
//   - project_categories  (项目大类)
//   - project_properties  (项目属性, 隶属大类, 一对多)
//   - project_problems    (常见问题, 隶属大类, 一对多; 仅快捷填充描述, 不落工单)
//
// 删除一律软删除 (deleted_at), ID 保留; 同属主下未删除记录 name 唯一。
package service

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"
	"gorm.io/datatypes"

	apperrors "xin-ni-repair/internal/errors"
	"xin-ni-repair/internal/model"
	"xin-ni-repair/internal/repository"
)

// ────────────────────────────────────────────
// 视图结构 (树形)
// ────────────────────────────────────────────

// PropertyView 项目属性视图
type PropertyView struct {
	ID          string `json:"id"`
	CategoryID  string `json:"category_id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	SortOrder   int    `json:"sort_order"`
}

// ProblemView 常见问题视图
type ProblemView struct {
	ID              string   `json:"id"`
	CategoryID      string   `json:"category_id"`
	Name            string   `json:"name"`
	Description     string   `json:"description"`
	CommonSolutions []string `json:"common_solutions"`
	SortOrder       int      `json:"sort_order"`
}

// CategoryView 项目大类视图 (含属性/常见问题子项)
type CategoryView struct {
	ID          string         `json:"id"`
	Name        string         `json:"name"`
	Description string         `json:"description"`
	SortOrder   int            `json:"sort_order"`
	Properties  []PropertyView `json:"properties"`
	Problems    []ProblemView  `json:"problems"`
}

// ────────────────────────────────────────────
// 入参
// ────────────────────────────────────────────

// CategoryInput 大类增改入参 (增: name 必填; 改: 局部更新)
type CategoryInput struct {
	Name        *string `json:"name"`
	Description *string `json:"description"`
	SortOrder   *int    `json:"sort_order"`
}

// PropertyInput 属性增改入参
type PropertyInput struct {
	CategoryID  *string `json:"category_id"`
	Name        *string `json:"name"`
	Description *string `json:"description"`
	SortOrder   *int    `json:"sort_order"`
}

// ProblemInput 常见问题增改入参
type ProblemInput struct {
	CategoryID      *string  `json:"category_id"`
	Name            *string  `json:"name"`
	Description     *string  `json:"description"`
	CommonSolutions []string `json:"common_solutions"`
	SortOrder       *int     `json:"sort_order"`
}

// ProjectService 项目字典业务逻辑
type ProjectService struct {
	projects *repository.ProjectRepository
	logger   *zap.Logger
}

// NewProjectService 创建 ProjectService
func NewProjectService(projects *repository.ProjectRepository, logger *zap.Logger) *ProjectService {
	return &ProjectService{projects: projects, logger: logger}
}

// ────────────────────────────────────────────
// 大类
// ────────────────────────────────────────────

// ListCategories 树形返回全部有效大类 (6.5.1 / 供后台配置页)
func (s *ProjectService) ListCategories(ctx context.Context) ([]CategoryView, error) {
	cats, err := s.projects.ListCategoriesWithChildren(ctx)
	if err != nil {
		return nil, s.dbErr("list categories failed", err)
	}
	return toCategoryViews(cats), nil
}

// CreateCategory 新增大类
func (s *ProjectService) CreateCategory(ctx context.Context, in CategoryInput) (*CategoryView, error) {
	name := strings.TrimSpace(ptrStr(in.Name))
	if err := validateProjectName("项目大类", name, 1, 50); err != nil {
		return nil, err
	}
	desc := strings.TrimSpace(ptrStr(in.Description))
	if err := validateLength("description", desc, 0, 200); err != nil {
		return nil, err
	}
	n, err := s.projects.CountCategoryName(ctx, name, "")
	if err != nil {
		return nil, s.dbErr("count category name failed", err)
	}
	if n > 0 {
		return nil, apperrors.ErrInvalidParam.WithMessage("项目大类名称已存在: " + name)
	}

	cat := &model.ProjectCategory{
		ID:          uuid.New().String(),
		Name:        name,
		Description: desc,
		SortOrder:   ptrInt(in.SortOrder),
	}
	if err := s.projects.CreateCategory(ctx, cat); err != nil {
		return nil, s.dbErr("create category failed", err)
	}
	return &CategoryView{ID: cat.ID, Name: cat.Name, Description: cat.Description, SortOrder: cat.SortOrder, Properties: []PropertyView{}, Problems: []ProblemView{}}, nil
}

// UpdateCategory 修改大类 (局部更新)
func (s *ProjectService) UpdateCategory(ctx context.Context, id string, in CategoryInput) (*CategoryView, error) {
	cat, err := s.projects.FindCategoryByID(ctx, id)
	if err != nil {
		return nil, s.dbErr("find category failed", err)
	}
	if cat == nil {
		return nil, apperrors.ErrNotFound.WithMessage("项目大类不存在")
	}
	if in.Name != nil {
		name := strings.TrimSpace(*in.Name)
		if err := validateProjectName("项目大类", name, 1, 50); err != nil {
			return nil, err
		}
		if name != cat.Name {
			n, err := s.projects.CountCategoryName(ctx, name, id)
			if err != nil {
				return nil, s.dbErr("count category name failed", err)
			}
			if n > 0 {
				return nil, apperrors.ErrInvalidParam.WithMessage("项目大类名称已存在: " + name)
			}
			cat.Name = name
		}
	}
	if in.Description != nil {
		desc := strings.TrimSpace(*in.Description)
		if err := validateLength("description", desc, 0, 200); err != nil {
			return nil, err
		}
		cat.Description = desc
	}
	if in.SortOrder != nil {
		cat.SortOrder = *in.SortOrder
	}
	if err := s.projects.UpdateCategory(ctx, cat); err != nil {
		return nil, s.dbErr("update category failed", err)
	}
	return s.singleCategory(ctx, id)
}

// DeleteCategory 软删除大类 (连带软删其属性/常见问题)
func (s *ProjectService) DeleteCategory(ctx context.Context, id string) error {
	cat, err := s.projects.FindCategoryByID(ctx, id)
	if err != nil {
		return s.dbErr("find category failed", err)
	}
	if cat == nil {
		return apperrors.ErrNotFound.WithMessage("项目大类不存在")
	}
	if err := s.projects.SoftDeleteCategory(ctx, id, time.Now()); err != nil {
		return s.dbErr("soft delete category failed", err)
	}
	return nil
}

// ────────────────────────────────────────────
// 属性
// ────────────────────────────────────────────

// ListProperties 按大类列出有效属性
func (s *ProjectService) ListProperties(ctx context.Context, categoryID string) ([]PropertyView, error) {
	if categoryID == "" {
		return nil, apperrors.ErrInvalidParam.WithMessage("category_id 必填")
	}
	if err := s.requireActiveCategory(ctx, categoryID); err != nil {
		return nil, err
	}
	list, err := s.projects.ListProperties(ctx, categoryID)
	if err != nil {
		return nil, s.dbErr("list properties failed", err)
	}
	return toPropertyViews(list), nil
}

// CreateProperty 新增属性
func (s *ProjectService) CreateProperty(ctx context.Context, in PropertyInput) (*PropertyView, error) {
	categoryID := strings.TrimSpace(ptrStr(in.CategoryID))
	if categoryID == "" {
		return nil, apperrors.ErrInvalidParam.WithMessage("category_id 必填")
	}
	if err := s.requireActiveCategory(ctx, categoryID); err != nil {
		return nil, err
	}
	name := strings.TrimSpace(ptrStr(in.Name))
	if err := validateProjectName("项目属性", name, 1, 50); err != nil {
		return nil, err
	}
	desc := strings.TrimSpace(ptrStr(in.Description))
	if err := validateLength("description", desc, 0, 200); err != nil {
		return nil, err
	}
	n, err := s.projects.CountPropertyName(ctx, categoryID, name, "")
	if err != nil {
		return nil, s.dbErr("count property name failed", err)
	}
	if n > 0 {
		return nil, apperrors.ErrInvalidParam.WithMessage("该项目大类下属性已存在: " + name)
	}

	p := &model.ProjectProperty{
		ID:          uuid.New().String(),
		CategoryID:  categoryID,
		Name:        name,
		Description: desc,
		SortOrder:   ptrInt(in.SortOrder),
	}
	if err := s.projects.CreateProperty(ctx, p); err != nil {
		return nil, s.dbErr("create property failed", err)
	}
	v := toPropertyView(*p)
	return &v, nil
}

// UpdateProperty 修改属性
func (s *ProjectService) UpdateProperty(ctx context.Context, id string, in PropertyInput) (*PropertyView, error) {
	p, err := s.projects.FindActivePropertyByID(ctx, id)
	if err != nil {
		return nil, s.dbErr("find property failed", err)
	}
	if p == nil {
		return nil, apperrors.ErrNotFound.WithMessage("项目属性不存在")
	}
	if in.Name != nil {
		name := strings.TrimSpace(*in.Name)
		if err := validateProjectName("项目属性", name, 1, 50); err != nil {
			return nil, err
		}
		if name != p.Name {
			n, err := s.projects.CountPropertyName(ctx, p.CategoryID, name, id)
			if err != nil {
				return nil, s.dbErr("count property name failed", err)
			}
			if n > 0 {
				return nil, apperrors.ErrInvalidParam.WithMessage("该项目大类下属性已存在: " + name)
			}
			p.Name = name
		}
	}
	if in.Description != nil {
		desc := strings.TrimSpace(*in.Description)
		if err := validateLength("description", desc, 0, 200); err != nil {
			return nil, err
		}
		p.Description = desc
	}
	if in.SortOrder != nil {
		p.SortOrder = *in.SortOrder
	}
	if err := s.projects.UpdateProperty(ctx, p); err != nil {
		return nil, s.dbErr("update property failed", err)
	}
	v := toPropertyView(*p)
	return &v, nil
}

// DeleteProperty 软删除属性
func (s *ProjectService) DeleteProperty(ctx context.Context, id string) error {
	p, err := s.projects.FindActivePropertyByID(ctx, id)
	if err != nil {
		return s.dbErr("find property failed", err)
	}
	if p == nil {
		return apperrors.ErrNotFound.WithMessage("项目属性不存在")
	}
	if err := s.projects.SoftDeleteProperty(ctx, id, time.Now()); err != nil {
		return s.dbErr("soft delete property failed", err)
	}
	return nil
}

// ────────────────────────────────────────────
// 常见问题
// ────────────────────────────────────────────

// ListProblems 按大类列出有效常见问题
func (s *ProjectService) ListProblems(ctx context.Context, categoryID string) ([]ProblemView, error) {
	if categoryID == "" {
		return nil, apperrors.ErrInvalidParam.WithMessage("category_id 必填")
	}
	if err := s.requireActiveCategory(ctx, categoryID); err != nil {
		return nil, err
	}
	list, err := s.projects.ListProblems(ctx, categoryID)
	if err != nil {
		return nil, s.dbErr("list problems failed", err)
	}
	return toProblemViews(list), nil
}

// CreateProblem 新增常见问题
func (s *ProjectService) CreateProblem(ctx context.Context, in ProblemInput) (*ProblemView, error) {
	categoryID := strings.TrimSpace(ptrStr(in.CategoryID))
	if categoryID == "" {
		return nil, apperrors.ErrInvalidParam.WithMessage("category_id 必填")
	}
	if err := s.requireActiveCategory(ctx, categoryID); err != nil {
		return nil, err
	}
	name := strings.TrimSpace(ptrStr(in.Name))
	if err := validateProjectName("常见问题", name, 1, 100); err != nil {
		return nil, err
	}
	desc := strings.TrimSpace(ptrStr(in.Description))
	if err := validateLength("description", desc, 0, 200); err != nil {
		return nil, err
	}
	n, err := s.projects.CountProblemName(ctx, categoryID, name, "")
	if err != nil {
		return nil, s.dbErr("count problem name failed", err)
	}
	if n > 0 {
		return nil, apperrors.ErrInvalidParam.WithMessage("该项目大类下常见问题已存在: " + name)
	}

	solutions, err := marshalSolutions(in.CommonSolutions)
	if err != nil {
		return nil, err
	}
	p := &model.ProjectProblem{
		ID:              uuid.New().String(),
		CategoryID:      categoryID,
		Name:            name,
		Description:     desc,
		CommonSolutions: solutions,
		SortOrder:       ptrInt(in.SortOrder),
	}
	if err := s.projects.CreateProblem(ctx, p); err != nil {
		return nil, s.dbErr("create problem failed", err)
	}
	v := toProblemView(*p)
	return &v, nil
}

// UpdateProblem 修改常见问题
func (s *ProjectService) UpdateProblem(ctx context.Context, id string, in ProblemInput) (*ProblemView, error) {
	p, err := s.projects.FindActiveProblemByID(ctx, id)
	if err != nil {
		return nil, s.dbErr("find problem failed", err)
	}
	if p == nil {
		return nil, apperrors.ErrNotFound.WithMessage("常见问题不存在")
	}
	if in.Name != nil {
		name := strings.TrimSpace(*in.Name)
		if err := validateProjectName("常见问题", name, 1, 100); err != nil {
			return nil, err
		}
		if name != p.Name {
			n, err := s.projects.CountProblemName(ctx, p.CategoryID, name, id)
			if err != nil {
				return nil, s.dbErr("count problem name failed", err)
			}
			if n > 0 {
				return nil, apperrors.ErrInvalidParam.WithMessage("该项目大类下常见问题已存在: " + name)
			}
			p.Name = name
		}
	}
	if in.Description != nil {
		desc := strings.TrimSpace(*in.Description)
		if err := validateLength("description", desc, 0, 200); err != nil {
			return nil, err
		}
		p.Description = desc
	}
	if in.CommonSolutions != nil {
		solutions, err := marshalSolutions(in.CommonSolutions)
		if err != nil {
			return nil, err
		}
		p.CommonSolutions = solutions
	}
	if in.SortOrder != nil {
		p.SortOrder = *in.SortOrder
	}
	if err := s.projects.UpdateProblem(ctx, p); err != nil {
		return nil, s.dbErr("update problem failed", err)
	}
	v := toProblemView(*p)
	return &v, nil
}

// DeleteProblem 软删除常见问题
func (s *ProjectService) DeleteProblem(ctx context.Context, id string) error {
	p, err := s.projects.FindActiveProblemByID(ctx, id)
	if err != nil {
		return s.dbErr("find problem failed", err)
	}
	if p == nil {
		return apperrors.ErrNotFound.WithMessage("常见问题不存在")
	}
	if err := s.projects.SoftDeleteProblem(ctx, id, time.Now()); err != nil {
		return s.dbErr("soft delete problem failed", err)
	}
	return nil
}

// ────────────────────────────────────────────
// 辅助
// ────────────────────────────────────────────

// requireActiveCategory 校验大类存在且有效
func (s *ProjectService) requireActiveCategory(ctx context.Context, categoryID string) error {
	cat, err := s.projects.FindActiveCategoryByID(ctx, categoryID)
	if err != nil {
		return s.dbErr("find category failed", err)
	}
	if cat == nil {
		return apperrors.ErrNotFound.WithMessage("项目大类不存在或已删除")
	}
	return nil
}

// singleCategory 返回单条大类树视图
func (s *ProjectService) singleCategory(ctx context.Context, id string) (*CategoryView, error) {
	list, err := s.ListCategories(ctx)
	if err != nil {
		return nil, err
	}
	for i := range list {
		if list[i].ID == id {
			return &list[i], nil
		}
	}
	return nil, apperrors.ErrNotFound.WithMessage("项目大类不存在")
}

func toCategoryViews(cats []model.ProjectCategory) []CategoryView {
	out := make([]CategoryView, 0, len(cats))
	for _, c := range cats {
		out = append(out, CategoryView{
			ID:          c.ID,
			Name:        c.Name,
			Description: c.Description,
			SortOrder:   c.SortOrder,
			Properties:  toPropertyViews(c.Properties),
			Problems:    toProblemViews(c.Problems),
		})
	}
	return out
}

func toPropertyViews(list []model.ProjectProperty) []PropertyView {
	out := make([]PropertyView, 0, len(list))
	for _, p := range list {
		out = append(out, toPropertyView(p))
	}
	return out
}

func toPropertyView(p model.ProjectProperty) PropertyView {
	return PropertyView{
		ID:          p.ID,
		CategoryID:  p.CategoryID,
		Name:        p.Name,
		Description: p.Description,
		SortOrder:   p.SortOrder,
	}
}

func toProblemViews(list []model.ProjectProblem) []ProblemView {
	out := make([]ProblemView, 0, len(list))
	for _, p := range list {
		out = append(out, toProblemView(p))
	}
	return out
}

func toProblemView(p model.ProjectProblem) ProblemView {
	return ProblemView{
		ID:              p.ID,
		CategoryID:      p.CategoryID,
		Name:            p.Name,
		Description:     p.Description,
		CommonSolutions: unmarshalSolutions(p.CommonSolutions),
		SortOrder:       p.SortOrder,
	}
}

// marshalSolutions 序列化常见解决建议 (去空白, 去空项)
func marshalSolutions(list []string) (datatypes.JSON, error) {
	clean := make([]string, 0, len(list))
	for _, v := range list {
		v = strings.TrimSpace(v)
		if v != "" {
			clean = append(clean, v)
		}
	}
	raw, err := json.Marshal(clean)
	if err != nil {
		return nil, apperrors.ErrInvalidParam.WithMessage("common_solutions 格式错误")
	}
	return datatypes.JSON(raw), nil
}

// unmarshalSolutions 解析 JSONB 常见解决建议
func unmarshalSolutions(raw datatypes.JSON) []string {
	var list []string
	if len(raw) > 0 {
		_ = json.Unmarshal(raw, &list)
	}
	if list == nil {
		list = []string{}
	}
	return list
}

// validateProjectName 项目字典名称校验 (必填 + 长度)
func validateProjectName(field, v string, min, max int) error {
	n := len([]rune(v))
	if n < min || n > max {
		return apperrors.ErrInvalidParam.WithMessage(fmtLen(field, min, max, n))
	}
	return nil
}

func fmtLen(field string, min, max, n int) string {
	if n == 0 {
		return field + " 为必填项"
	}
	return fmt.Sprintf("%s 长度需为 %d-%d 字符", field, min, max)
}

// ptrStr 解引用字符串指针, nil 返回空串
func ptrStr(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}

// ptrInt 解引用 int 指针, nil 返回 0
func ptrInt(p *int) int {
	if p == nil {
		return 0
	}
	return *p
}

// dbErr 数据库错误包装
func (s *ProjectService) dbErr(msg string, err error) error {
	s.logger.Error(msg, zap.Error(err))
	return apperrors.ErrDatabaseError.WithError(err)
}
