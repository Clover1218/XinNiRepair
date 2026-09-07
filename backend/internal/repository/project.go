// 项目字典数据访问 (project_categories / project_properties / project_problems)
//
// 替代原 JSON 文件存储。删除一律软删除 (置 deleted_at), ID 与行保留,
// 因此工单可通过 category_id/property_id 持续关联; 查询默认过滤已软删除行。
package repository

import (
	"context"
	"time"

	"gorm.io/gorm"

	"xin-ni-repair/internal/model"
)

// ProjectRepository 项目字典数据访问 (大类/属性/常见问题)
type ProjectRepository struct {
	db *gorm.DB
}

// NewProjectRepository 创建 ProjectRepository
func NewProjectRepository(db *gorm.DB) *ProjectRepository {
	return &ProjectRepository{db: db}
}

// ────────────────────────────────────────────
// 项目大类
// ────────────────────────────────────────────

// ListCategories 查询全部有效大类 (deleted_at IS NULL), 按 sort_order 升序
func (r *ProjectRepository) ListCategories(ctx context.Context) ([]model.ProjectCategory, error) {
	var cats []model.ProjectCategory
	err := r.db.WithContext(ctx).
		Where("deleted_at IS NULL").
		Order("sort_order ASC, created_at ASC").
		Find(&cats).Error
	if err != nil {
		return nil, err
	}
	return cats, nil
}

// ListCategoriesWithChildren 查询大类并预加载其有效属性及属性下问题 (options/6.5 树形展示用)
// V1.4 修订: 常见问题直接挂属性, 不再预加载大类级 problems
func (r *ProjectRepository) ListCategoriesWithChildren(ctx context.Context) ([]model.ProjectCategory, error) {
	var cats []model.ProjectCategory
	err := r.db.WithContext(ctx).
		Where("deleted_at IS NULL").
		Order("sort_order ASC, created_at ASC").
		Preload("Properties", "deleted_at IS NULL", func(db *gorm.DB) *gorm.DB {
			return db.Order("sort_order ASC, created_at ASC")
		}).
		Preload("Properties.Problems", "deleted_at IS NULL", func(db *gorm.DB) *gorm.DB {
			return db.Order("sort_order ASC, created_at ASC")
		}).
		Find(&cats).Error
	if err != nil {
		return nil, err
	}
	return cats, nil
}

// FindCategoryByID 查询大类 (含已软删除, 供回显/判断), 不存在返回 nil
func (r *ProjectRepository) FindCategoryByID(ctx context.Context, id string) (*model.ProjectCategory, error) {
	var c model.ProjectCategory
	err := r.db.WithContext(ctx).Where("id = ?", id).First(&c).Error
	if err == gorm.ErrRecordNotFound {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &c, nil
}

// FindActiveCategoryByID 查询有效大类 (deleted_at IS NULL)
func (r *ProjectRepository) FindActiveCategoryByID(ctx context.Context, id string) (*model.ProjectCategory, error) {
	var c model.ProjectCategory
	err := r.db.WithContext(ctx).Where("id = ? AND deleted_at IS NULL", id).First(&c).Error
	if err == gorm.ErrRecordNotFound {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &c, nil
}

// CountCategoryName 统计未删除且名称相同的大类数 (excludeID 排除自身)
func (r *ProjectRepository) CountCategoryName(ctx context.Context, name, excludeID string) (int64, error) {
	q := r.db.WithContext(ctx).Model(&model.ProjectCategory{}).
		Where("name = ? AND deleted_at IS NULL", name)
	if excludeID != "" {
		q = q.Where("id <> ?", excludeID)
	}
	var n int64
	err := q.Count(&n).Error
	return n, err
}

// CreateCategory 新增大类
func (r *ProjectRepository) CreateCategory(ctx context.Context, c *model.ProjectCategory) error {
	return r.db.WithContext(ctx).Create(c).Error
}

// UpdateCategory 更新大类
func (r *ProjectRepository) UpdateCategory(ctx context.Context, c *model.ProjectCategory) error {
	return r.db.WithContext(ctx).Save(c).Error
}

// SoftDeleteCategory 软删除大类 (置 deleted_at; 连带软删除其属性, 并级联软删属性下的常见问题)
func (r *ProjectRepository) SoftDeleteCategory(ctx context.Context, id string, at time.Time) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&model.ProjectCategory{}).Where("id = ?", id).Update("deleted_at", at).Error; err != nil {
			return err
		}
		if err := tx.Model(&model.ProjectProperty{}).Where("category_id = ? AND deleted_at IS NULL", id).
			Update("deleted_at", at).Error; err != nil {
			return err
		}
		// 问题不再直接关联大类: 级联其属性下问题
		return tx.Model(&model.ProjectProblem{}).
			Where("deleted_at IS NULL AND property_id IN (SELECT id FROM project_properties WHERE category_id = ?)", id).
			Update("deleted_at", at).Error
	})
}

// ────────────────────────────────────────────
// 项目属性
// ────────────────────────────────────────────

// ListProperties 查询某大类下有效属性, 按 sort_order 升序
func (r *ProjectRepository) ListProperties(ctx context.Context, categoryID string) ([]model.ProjectProperty, error) {
	var list []model.ProjectProperty
	err := r.db.WithContext(ctx).
		Where("category_id = ? AND deleted_at IS NULL", categoryID).
		Order("sort_order ASC, created_at ASC").
		Find(&list).Error
	if err != nil {
		return nil, err
	}
	return list, nil
}

// FindActivePropertyByID 查询有效属性 (需校验属主 category_id 时用)
func (r *ProjectRepository) FindActivePropertyByID(ctx context.Context, id string) (*model.ProjectProperty, error) {
	var p model.ProjectProperty
	err := r.db.WithContext(ctx).Where("id = ? AND deleted_at IS NULL", id).First(&p).Error
	if err == gorm.ErrRecordNotFound {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &p, nil
}

// CountPropertyName 统计某大类下未删除同名属性数
func (r *ProjectRepository) CountPropertyName(ctx context.Context, categoryID, name, excludeID string) (int64, error) {
	q := r.db.WithContext(ctx).Model(&model.ProjectProperty{}).
		Where("category_id = ? AND name = ? AND deleted_at IS NULL", categoryID, name)
	if excludeID != "" {
		q = q.Where("id <> ?", excludeID)
	}
	var n int64
	err := q.Count(&n).Error
	return n, err
}

// CreateProperty 新增属性
func (r *ProjectRepository) CreateProperty(ctx context.Context, p *model.ProjectProperty) error {
	return r.db.WithContext(ctx).Create(p).Error
}

// UpdateProperty 更新属性
func (r *ProjectRepository) UpdateProperty(ctx context.Context, p *model.ProjectProperty) error {
	return r.db.WithContext(ctx).Save(p).Error
}

// SoftDeleteProperty 软删除属性 (级联软删其下常见问题)
func (r *ProjectRepository) SoftDeleteProperty(ctx context.Context, id string, at time.Time) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&model.ProjectProperty{}).
			Where("id = ?", id).Update("deleted_at", at).Error; err != nil {
			return err
		}
		return tx.Model(&model.ProjectProblem{}).
			Where("property_id = ? AND deleted_at IS NULL", id).
			Update("deleted_at", at).Error
	})
}

// ────────────────────────────────────────────
// 常见问题
// ────────────────────────────────────────────

// ListProblems 查询某属性下有效常见问题, 按 sort_order 升序 (V1.4: 问题隶属属性)
func (r *ProjectRepository) ListProblems(ctx context.Context, propertyID string) ([]model.ProjectProblem, error) {
	var list []model.ProjectProblem
	err := r.db.WithContext(ctx).
		Where("property_id = ? AND deleted_at IS NULL", propertyID).
		Order("sort_order ASC, created_at ASC").
		Find(&list).Error
	if err != nil {
		return nil, err
	}
	return list, nil
}

// FindActiveProblemByID 查询有效常见问题
func (r *ProjectRepository) FindActiveProblemByID(ctx context.Context, id string) (*model.ProjectProblem, error) {
	var p model.ProjectProblem
	err := r.db.WithContext(ctx).Where("id = ? AND deleted_at IS NULL", id).First(&p).Error
	if err == gorm.ErrRecordNotFound {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &p, nil
}

// CountProblemName 统计某属性下未删除同名问题数 (V1.4: 同一属性内唯一)
func (r *ProjectRepository) CountProblemName(ctx context.Context, propertyID, name, excludeID string) (int64, error) {
	q := r.db.WithContext(ctx).Model(&model.ProjectProblem{}).
		Where("property_id = ? AND name = ? AND deleted_at IS NULL", propertyID, name)
	if excludeID != "" {
		q = q.Where("id <> ?", excludeID)
	}
	var n int64
	err := q.Count(&n).Error
	return n, err
}

// CreateProblem 新增常见问题
func (r *ProjectRepository) CreateProblem(ctx context.Context, p *model.ProjectProblem) error {
	return r.db.WithContext(ctx).Create(p).Error
}

// UpdateProblem 更新常见问题
func (r *ProjectRepository) UpdateProblem(ctx context.Context, p *model.ProjectProblem) error {
	return r.db.WithContext(ctx).Save(p).Error
}

// SoftDeleteProblem 软删除常见问题
func (r *ProjectRepository) SoftDeleteProblem(ctx context.Context, id string, at time.Time) error {
	return r.db.WithContext(ctx).Model(&model.ProjectProblem{}).
		Where("id = ?", id).Update("deleted_at", at).Error
}
