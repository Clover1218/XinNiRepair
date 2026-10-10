package repository

import (
	"context"

	"gorm.io/gorm"

	"xin-ni-repair/internal/model"
)

// SettingsRepository 系统设置数据访问
type SettingsRepository struct {
	db *gorm.DB
}

// NewSettingsRepository 创建 SettingsRepository
func NewSettingsRepository(db *gorm.DB) *SettingsRepository {
	return &SettingsRepository{db: db}
}

// List 列出全部设置项
func (r *SettingsRepository) List(ctx context.Context) ([]model.SystemSetting, error) {
	var rows []model.SystemSetting
	if err := r.db.WithContext(ctx).Order("key ASC").Find(&rows).Error; err != nil {
		return nil, err
	}
	return rows, nil
}

// Get 按 key 查询设置项, 不存在时返回 nil
func (r *SettingsRepository) Get(ctx context.Context, key string) (*model.SystemSetting, error) {
	var row model.SystemSetting
	err := r.db.WithContext(ctx).Where("key = ?", key).First(&row).Error
	if err == gorm.ErrRecordNotFound {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &row, nil
}

// Upsert 写入（存在则更新 value/description/updated_at/updated_by）
func (r *SettingsRepository) Upsert(ctx context.Context, row *model.SystemSetting) error {
	var existing model.SystemSetting
	err := r.db.WithContext(ctx).Where("key = ?", row.Key).First(&existing).Error
	if err == gorm.ErrRecordNotFound {
		return r.db.WithContext(ctx).Create(row).Error
	}
	if err != nil {
		return err
	}
	existing.Value = row.Value
	existing.Description = row.Description
	existing.UpdatedAt = row.UpdatedAt
	existing.UpdatedBy = row.UpdatedBy
	return r.db.WithContext(ctx).Save(&existing).Error
}
