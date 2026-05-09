// Package tenant 租户管理（三级SaaS核心）
package tenant

import (
	"context"
	"errors"
	"fmt"

	"go.uber.org/zap"
	"gorm.io/gorm"

	"github.com/zhongjinmuai-lang/mu-framework/internal/model"
	"github.com/zhongjinmuai-lang/mu-framework/pkg/logger"
)

// Service 租户服务
type Service struct {
	db *gorm.DB
}

// NewService 创建服务
func NewService(db *gorm.DB) *Service {
	return &Service{db: db}
}

// CreateInput 创建租户入参
type CreateInput struct {
	Name     string            `json:"name" binding:"required,max=100"`
	Code     string            `json:"code" binding:"required,max=50"`
	Level    model.TenantLevel `json:"level" binding:"required,oneof=developer provider customer"`
	ParentID *string           `json:"parent_id"`
	Config   string            `json:"config"`
}

// Create 创建租户
// 校验逻辑：
//   - 层级关系合法（developer无上级 / provider上级=developer / customer上级=provider）
//   - code 唯一
func (s *Service) Create(ctx context.Context, in *CreateInput) (*model.Tenant, error) {
	if err := s.validateHierarchy(ctx, in.Level, in.ParentID); err != nil {
		return nil, err
	}

	// 唯一性
	var cnt int64
	if err := s.db.WithContext(ctx).Model(&model.Tenant{}).
		Where("code = ?", in.Code).Count(&cnt).Error; err != nil {
		return nil, fmt.Errorf("检查编码唯一性失败: %w", err)
	}
	if cnt > 0 {
		return nil, fmt.Errorf("租户编码 %q 已存在", in.Code)
	}

	t := &model.Tenant{
		Name:     in.Name,
		Code:     in.Code,
		Level:    in.Level,
		ParentID: in.ParentID,
		Status:   model.StatusEnabled,
		Config:   in.Config,
	}
	if t.Config == "" {
		t.Config = "{}"
	}

	if err := s.db.WithContext(ctx).Create(t).Error; err != nil {
		return nil, fmt.Errorf("创建租户失败: %w", err)
	}

	logger.WithContext(ctx).Info("租户创建成功",
		zap.String("id", t.ID),
		zap.String("code", t.Code),
		zap.String("level", string(t.Level)),
	)
	return t, nil
}

// GetByID 按ID查询
func (s *Service) GetByID(ctx context.Context, id string) (*model.Tenant, error) {
	var t model.Tenant
	if err := s.db.WithContext(ctx).First(&t, "id = ?", id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, fmt.Errorf("租户不存在: %s", id)
		}
		return nil, err
	}
	return &t, nil
}

// GetByCode 按编码查询
func (s *Service) GetByCode(ctx context.Context, code string) (*model.Tenant, error) {
	var t model.Tenant
	if err := s.db.WithContext(ctx).First(&t, "code = ?", code).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, fmt.Errorf("租户不存在: %s", code)
		}
		return nil, err
	}
	return &t, nil
}

// ListByParent 获取下级租户列表（分页）
func (s *Service) ListByParent(ctx context.Context, parentID string, page, pageSize int) ([]*model.Tenant, int64, error) {
	if pageSize <= 0 {
		pageSize = 20
	}
	if page <= 0 {
		page = 1
	}

	var (
		list  []*model.Tenant
		total int64
	)
	q := s.db.WithContext(ctx).Model(&model.Tenant{}).Where("parent_id = ?", parentID)
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	if err := q.Order("created_at DESC").
		Limit(pageSize).Offset((page - 1) * pageSize).
		Find(&list).Error; err != nil {
		return nil, 0, err
	}
	return list, total, nil
}

// UpdateStatus 更新状态（上级禁用 → 级联禁用所有下级）
func (s *Service) UpdateStatus(ctx context.Context, id string, status int) error {
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&model.Tenant{}).Where("id = ?", id).
			Update("status", status).Error; err != nil {
			return err
		}
		// 禁用时级联禁用所有下级（利用递归 CTE）
		if status == model.StatusDisabled {
			sql := `
WITH RECURSIVE descendants AS (
    SELECT id FROM tenants WHERE parent_id = ? AND deleted_at IS NULL
  UNION ALL
    SELECT t.id FROM tenants t
    INNER JOIN descendants d ON t.parent_id = d.id
    WHERE t.deleted_at IS NULL
)
UPDATE tenants SET status = 0, updated_at = NOW()
WHERE id IN (SELECT id FROM descendants)`
			if err := tx.Exec(sql, id).Error; err != nil {
				return err
			}
			logger.WithContext(ctx).Info("已级联禁用下级租户", zap.String("parent_id", id))
		}
		return nil
	})
}

// Delete 软删除（级联软删所有下级）
func (s *Service) Delete(ctx context.Context, id string) error {
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// 本身软删
		if err := tx.Delete(&model.Tenant{}, "id = ?", id).Error; err != nil {
			return err
		}
		// 级联软删（soft_delete flag 语义：deleted_at 非0代表已删）
		sql := `
WITH RECURSIVE descendants AS (
    SELECT id FROM tenants WHERE parent_id = ? AND deleted_at IS NULL
  UNION ALL
    SELECT t.id FROM tenants t
    INNER JOIN descendants d ON t.parent_id = d.id
    WHERE t.deleted_at IS NULL
)
UPDATE tenants SET deleted_at = EXTRACT(EPOCH FROM NOW())::bigint, updated_at = NOW()
WHERE id IN (SELECT id FROM descendants)`
		return tx.Exec(sql, id).Error
	})
}

// validateHierarchy 层级关系合法性校验
func (s *Service) validateHierarchy(ctx context.Context, level model.TenantLevel, parentID *string) error {
	switch level {
	case model.LevelDeveloper:
		if parentID != nil && *parentID != "" {
			return errors.New("开发商层级不能有上级租户")
		}
	case model.LevelProvider:
		if parentID == nil || *parentID == "" {
			return errors.New("服务商必须有上级租户（开发商）")
		}
		p, err := s.GetByID(ctx, *parentID)
		if err != nil {
			return err
		}
		if p.Level != model.LevelDeveloper {
			return errors.New("服务商的上级必须是开发商")
		}
	case model.LevelCustomer:
		if parentID == nil || *parentID == "" {
			return errors.New("终端客户必须有上级租户（服务商）")
		}
		p, err := s.GetByID(ctx, *parentID)
		if err != nil {
			return err
		}
		if p.Level != model.LevelProvider {
			return errors.New("终端客户的上级必须是服务商")
		}
	default:
		return fmt.Errorf("未知的租户层级: %s", level)
	}
	return nil
}
