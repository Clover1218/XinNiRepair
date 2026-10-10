package service

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"go.uber.org/zap"

	apperrors "xin-ni-repair/internal/errors"
	"xin-ni-repair/internal/model"
	"xin-ni-repair/internal/repository"
)

// ────────────────────────────────────────────
// 系统设置服务（超管可调参数；当前承载「完工后二次修改时限」闸门阈值）
// ────────────────────────────────────────────

// 已知设置项定义：key -> 默认值与校验
const (
	// SettingCompletedEditWindowHours 完工后可二次修改/重开的时限（小时）
	SettingCompletedEditWindowHours = "completed_edit_window_hours"
)

// 未知 key 的兜底默认值（数据库缺失或解析失败时采用）
const defaultCompletedEditWindowHours = 24

// settingDef 单个设置项的元数据
type settingDef struct {
	Key         string
	Default     string
	Description string
	// validate 校验用户输入值，返回错误（nil 表示合法）
	validate func(v string) error
}

// knownSettings 受支持的设置项白名单（含校验）
var knownSettings = map[string]settingDef{
	SettingCompletedEditWindowHours: {
		Key:         SettingCompletedEditWindowHours,
		Default:     "24",
		Description: "完工后允许二次修改/重开的时限（小时），超时后仅超级管理员可操作",
		validate: func(v string) error {
			n, err := strconv.Atoi(v)
			if err != nil || n <= 0 {
				return fmt.Errorf("必须为正整数（小时）")
			}
			return nil
		},
	},
}

// SettingsService 系统设置服务
type SettingsService struct {
	repo   *repository.SettingsRepository
	logger *zap.Logger
}

// NewSettingsService 创建 SettingsService
func NewSettingsService(repo *repository.SettingsRepository, logger *zap.Logger) *SettingsService {
	return &SettingsService{repo: repo, logger: logger}
}

// List 列出全部设置项（含当前值与定义描述，未知/缺失项用默认值回退）
func (s *SettingsService) List(ctx context.Context) (map[string]SettingItem, error) {
	rows, err := s.repo.List(ctx)
	if err != nil {
		return nil, apperrors.ErrDatabaseError.WithError(err)
	}
	byKey := make(map[string]model.SystemSetting, len(rows))
	for _, r := range rows {
		byKey[r.Key] = r
	}
	out := make(map[string]SettingItem, len(knownSettings))
	for k, def := range knownSettings {
		item := SettingItem{Key: k, Description: def.Description, Default: def.Default, Editable: true}
		if row, ok := byKey[k]; ok {
			item.Value = row.Value
			item.UpdatedAt = row.UpdatedAt
			if row.UpdatedBy != nil {
				item.UpdatedBy = *row.UpdatedBy
			}
		} else {
			item.Value = def.Default
		}
		out[k] = item
	}
	return out, nil
}

// GetString 取字符串值（缺失或解析失败回退默认值）
func (s *SettingsService) GetString(ctx context.Context, key string) (string, error) {
	def, ok := knownSettings[key]
	if !ok {
		return "", apperrors.ErrInvalidParam.WithMessage("未知设置项: " + key)
	}
	row, err := s.repo.Get(ctx, key)
	if err != nil {
		return "", apperrors.ErrDatabaseError.WithError(err)
	}
	if row == nil {
		return def.Default, nil
	}
	return row.Value, nil
}

// GetInt 取整数值（缺失/非法回退默认值）
func (s *SettingsService) GetInt(ctx context.Context, key string) (int, error) {
	v, err := s.GetString(ctx, key)
	if err != nil {
		return defaultCompletedEditWindowHours, err
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return defaultCompletedEditWindowHours, nil
	}
	return n, nil
}

// Update 校验并更新设置项（仅接受白名单内 key 与合法值）
func (s *SettingsService) Update(ctx context.Context, op Operator, key, value string) error {
	def, ok := knownSettings[key]
	if !ok {
		return apperrors.ErrInvalidParam.WithMessage("未知设置项: " + key)
	}
	if err := def.validate(value); err != nil {
		return apperrors.ErrInvalidParam.WithMessage(fmt.Sprintf("设置项 %s %v", key, err))
	}
	now := time.Now()
	uid := op.UserID
	row := &model.SystemSetting{
		Key:         key,
		Value:       value,
		Description: def.Description,
		UpdatedAt:   now,
		UpdatedBy:   &uid,
	}
	if err := s.repo.Upsert(ctx, row); err != nil {
		return apperrors.ErrDatabaseError.WithError(err)
	}
	s.logger.Info("system setting updated",
		zap.String("key", key), zap.String("value", value), zap.String("operator_id", op.UserID))
	return nil
}

// GetCompletedEditWindowHours 读取完工后二次修改时限（小时），非法/缺失回退默认 24
func (s *SettingsService) GetCompletedEditWindowHours(ctx context.Context) int {
	n, err := s.GetInt(ctx, SettingCompletedEditWindowHours)
	if err != nil || n <= 0 {
		return defaultCompletedEditWindowHours
	}
	return n
}

// SettingItem 设置项输出结构
type SettingItem struct {
	Key         string    `json:"key"`
	Value       string    `json:"value"`
	Default     string    `json:"default"`
	Description string    `json:"description"`
	Editable    bool      `json:"editable"`
	UpdatedAt   time.Time `json:"updated_at"`
	UpdatedBy   string    `json:"updated_by,omitempty"`
}
