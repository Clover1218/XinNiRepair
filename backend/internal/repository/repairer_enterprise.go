package repository

import (
	"context"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"xin-ni-repair/internal/model"
)

// ────────────────────────────────────────────
// 维修业务员经营范围数据访问 (V1.5)
// ────────────────────────────────────────────

// RepairerEnterpriseRepository 经营范围数据访问
type RepairerEnterpriseRepository struct {
	db *gorm.DB
}

// NewRepairerEnterpriseRepository 创建 RepairerEnterpriseRepository
func NewRepairerEnterpriseRepository(db *gorm.DB) *RepairerEnterpriseRepository {
	return &RepairerEnterpriseRepository{db: db}
}

// ListActiveByRepairer 查询某业务员的生效经营范围（含企业信息，用于后台页面展示）
func (r *RepairerEnterpriseRepository) ListActiveByRepairer(ctx context.Context, repairerID string) ([]model.RepairerEnterprise, error) {
	var list []model.RepairerEnterprise
	err := r.db.WithContext(ctx).
		Where("repairer_id = ? AND status = ?", repairerID, model.RepairerScopeActive).
		Preload("Enterprise").
		Order("created_at ASC").
		Find(&list).Error
	if err != nil {
		return nil, err
	}
	return list, nil
}

// ActiveEnterpriseIDsByRepairer 查询某业务员生效范围的企业 ID 列表（数据权限过滤主入口）
func (r *RepairerEnterpriseRepository) ActiveEnterpriseIDsByRepairer(ctx context.Context, repairerID string) ([]string, error) {
	var ids []string
	err := r.db.WithContext(ctx).Model(&model.RepairerEnterprise{}).
		Where("repairer_id = ? AND status = ?", repairerID, model.RepairerScopeActive).
		Pluck("enterprise_id", &ids).Error
	if err != nil {
		return nil, err
	}
	return ids, nil
}

// ListActiveGroupedByRepairer 取全部生效指派（含企业信息），按业务员分组。
// 列表页要直接展示"分配了哪些企业"，批量取避免 N+1 查询。
func (r *RepairerEnterpriseRepository) ListActiveGroupedByRepairer(ctx context.Context) (map[string][]model.RepairerEnterprise, error) {
	var list []model.RepairerEnterprise
	err := r.db.WithContext(ctx).
		Where("status = ?", model.RepairerScopeActive).
		Preload("Enterprise").
		Order("created_at ASC").
		Find(&list).Error
	if err != nil {
		return nil, err
	}
	out := make(map[string][]model.RepairerEnterprise, len(list))
	for _, it := range list {
		out[it.RepairerID] = append(out[it.RepairerID], it)
	}
	return out, nil
}

// ListByEnterprise 反向查询：服务该客户企业的业务员
func (r *RepairerEnterpriseRepository) ListByEnterprise(ctx context.Context, enterpriseID string) ([]model.RepairerEnterprise, error) {
	var list []model.RepairerEnterprise
	err := r.db.WithContext(ctx).
		Where("enterprise_id = ? AND status = ?", enterpriseID, model.RepairerScopeActive).
		Preload("Repairer").
		Order("created_at ASC").
		Find(&list).Error
	if err != nil {
		return nil, err
	}
	return list, nil
}

// FindOne 按业务员+企业查询指派记录，不存在时返回 nil
func (r *RepairerEnterpriseRepository) FindOne(ctx context.Context, repairerID, enterpriseID string) (*model.RepairerEnterprise, error) {
	var item model.RepairerEnterprise
	err := r.db.WithContext(ctx).
		Where("repairer_id = ? AND enterprise_id = ?", repairerID, enterpriseID).
		First(&item).Error
	if err == gorm.ErrRecordNotFound {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &item, nil
}

// ExistsActive 判断某业务员对该企业是否有生效指派（接单前置校验用）
func (r *RepairerEnterpriseRepository) ExistsActive(ctx context.Context, repairerID, enterpriseID string) (bool, error) {
	var count int64
	err := r.db.WithContext(ctx).Model(&model.RepairerEnterprise{}).
		Where("repairer_id = ? AND enterprise_id = ? AND status = ?",
			repairerID, enterpriseID, model.RepairerScopeActive).
		Count(&count).Error
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

// ExistsEnterprise 判断企业是否存在且有效，非法企业的指派请求应在 service 层拦截
func (r *RepairerEnterpriseRepository) ExistsEnterprise(ctx context.Context, enterpriseID string) (bool, error) {
	var count int64
	err := r.db.WithContext(ctx).Model(&model.Enterprise{}).
		Where("id = ? AND status = ?", enterpriseID, model.EnterpriseActive).
		Count(&count).Error
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

// ReplaceAll 全量覆盖某业务员的经营范围（幂等）。
//
// 策略：采用软撤销而非物理删除，保留指派证据用于对账。
//   - 新出现的企业   → INSERT (active) 或 UPDATE (revoked → active, 清 revoked_at)
//   - 未出现在列表中的 → UPDATE (active → revoked, 写 revoked_at)
func (r *RepairerEnterpriseRepository) ReplaceAll(ctx context.Context, repairerID string, enterpriseIDs []string, createdBy string) error {
	now := time.Now()
	wanted := make(map[string]struct{}, len(enterpriseIDs))
	for _, id := range enterpriseIDs {
		wanted[id] = struct{}{}
	}

	var by *string
	if createdBy != "" {
		v := createdBy
		by = &v
	}

	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var existing []model.RepairerEnterprise
		if err := tx.Where("repairer_id = ?", repairerID).Find(&existing).Error; err != nil {
			return err
		}

		// 撤销：原已存在且本次未提交的生效记录
		for _, item := range existing {
			if _, keep := wanted[item.EnterpriseID]; keep {
				continue
			}
			if item.Status != model.RepairerScopeActive {
				continue
			}
			if err := tx.Model(&model.RepairerEnterprise{}).
				Where("id = ?", item.ID).
				Updates(map[string]interface{}{
					"status":     model.RepairerScopeRevoked,
					"revoked_at": now,
				}).Error; err != nil {
				return err
			}
		}

		// 新增 / 恢复
		for _, entID := range enterpriseIDs {
			var item model.RepairerEnterprise
			err := tx.Where("repairer_id = ? AND enterprise_id = ?", repairerID, entID).First(&item).Error
			switch {
			case err == gorm.ErrRecordNotFound:
				item = model.RepairerEnterprise{
					ID:           uuid.New().String(),
					RepairerID:   repairerID,
					EnterpriseID: entID,
					Status:       model.RepairerScopeActive,
					CreatedBy:    by,
					CreatedAt:    now,
				}
				if err := tx.Create(&item).Error; err != nil {
					return err
				}
			case err != nil:
				return err
			default:
				if item.Status == model.RepairerScopeActive {
					continue // 幂等：已是生效状态
				}
				if err := tx.Model(&model.RepairerEnterprise{}).
					Where("id = ?", item.ID).
					Updates(map[string]interface{}{
						"status":     model.RepairerScopeActive,
						"revoked_at": nil,
						"created_by": by,
					}).Error; err != nil {
					return err
				}
			}
		}
		return nil
	})
}
