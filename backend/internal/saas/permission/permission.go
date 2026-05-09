// Package permission 三级权限引擎
// 核心逻辑：
//   - 授权：上级向下级授予不超过自身范围的权限
//   - 回收：上级回收下级权限 → 级联回收所有下级
//   - 检查：基于 Redis 缓存加速，5分钟TTL
package permission

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"go.uber.org/zap"
	"gorm.io/gorm"

	"github.com/zhongjinmuai-lang/mu-framework/internal/model"
	"github.com/zhongjinmuai-lang/mu-framework/internal/saas/hierarchy"
	"github.com/zhongjinmuai-lang/mu-framework/pkg/cache"
	"github.com/zhongjinmuai-lang/mu-framework/pkg/logger"
)

// Service 权限服务
type Service struct {
	db        *gorm.DB
	rdb       *cache.Client
	hierarchy *hierarchy.Service
}

// NewService 创建权限服务
func NewService(db *gorm.DB, rdb *cache.Client, h *hierarchy.Service) *Service {
	return &Service{db: db, rdb: rdb, hierarchy: h}
}

// GrantInput 授权入参
type GrantInput struct {
	GranterTenantID string   `json:"granter_tenant_id" binding:"required"`
	GranteeTenantID string   `json:"grantee_tenant_id" binding:"required"`
	PermissionCodes []string `json:"permission_codes" binding:"required,min=1"`
}

// Grant 授予权限（严格遵循三级管控规则）
func (s *Service) Grant(ctx context.Context, in *GrantInput) error {
	// 1. 验证控制流向
	if err := s.hierarchy.ValidateControlFlow(ctx, in.GranterTenantID, in.GranteeTenantID); err != nil {
		return fmt.Errorf("授权失败: %w", err)
	}

	// 2. 校验授权方本身拥有所授权限
	for _, code := range in.PermissionCodes {
		has, err := s.HasPermission(ctx, in.GranterTenantID, code)
		if err != nil {
			return fmt.Errorf("检查授权方权限失败: %w", err)
		}
		if !has {
			return fmt.Errorf("授权方不拥有权限 %s，无法授予", code)
		}
	}

	// 3. 事务性批量授予（UPSERT：存在则激活，不存在则插入）
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		for _, code := range in.PermissionCodes {
			tp := model.TenantPermission{
				TenantID:       in.GranteeTenantID,
				PermissionCode: code,
				GrantedBy:      &in.GranterTenantID,
				Status:         model.StatusEnabled,
			}
			// ON CONFLICT DO UPDATE
			if err := tx.Exec(`
INSERT INTO tenant_permissions (id, tenant_id, permission_code, granted_by, status, created_at, updated_at)
VALUES (uuid_generate_v4(), ?, ?, ?, 1, NOW(), NOW())
ON CONFLICT (tenant_id, permission_code)
DO UPDATE SET status = 1, granted_by = EXCLUDED.granted_by, updated_at = NOW()`,
				tp.TenantID, tp.PermissionCode, tp.GrantedBy,
			).Error; err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return err
	}

	s.invalidateCache(ctx, in.GranteeTenantID)

	logger.WithContext(ctx).Info("权限授予成功",
		zap.String("granter", in.GranterTenantID),
		zap.String("grantee", in.GranteeTenantID),
		zap.Strings("permissions", in.PermissionCodes),
	)
	return nil
}

// Revoke 回收权限（级联回收所有下级的同名权限）
func (s *Service) Revoke(ctx context.Context, revokerTenantID, targetTenantID string, permCodes []string) error {
	if err := s.hierarchy.ValidateControlFlow(ctx, revokerTenantID, targetTenantID); err != nil {
		return fmt.Errorf("回收权限失败: %w", err)
	}
	if len(permCodes) == 0 {
		return errors.New("权限列表不能为空")
	}

	// 一次性拉取目标及其所有下级，统一失效
	descendants, err := s.hierarchy.GetDescendants(ctx, targetTenantID)
	if err != nil {
		return fmt.Errorf("获取下级租户失败: %w", err)
	}
	affectedTenants := append([]string{targetTenantID}, descendants...)

	err = s.db.WithContext(ctx).Model(&model.TenantPermission{}).
		Where("tenant_id IN ? AND permission_code IN ?", affectedTenants, permCodes).
		Update("status", model.StatusDisabled).Error
	if err != nil {
		return fmt.Errorf("批量回收权限失败: %w", err)
	}

	// 清缓存
	for _, tid := range affectedTenants {
		s.invalidateCache(ctx, tid)
	}

	logger.WithContext(ctx).Info("权限回收成功（含级联）",
		zap.String("revoker", revokerTenantID),
		zap.String("target", targetTenantID),
		zap.Int("cascade_count", len(descendants)),
		zap.Strings("permissions", permCodes),
	)
	return nil
}

// HasPermission 检查权限（带缓存）
func (s *Service) HasPermission(ctx context.Context, tenantID, permCode string) (bool, error) {
	cacheKey := fmt.Sprintf("perm:%s:%s", tenantID, permCode)
	if s.rdb != nil {
		if v, err := s.rdb.Get(ctx, cacheKey).Result(); err == nil {
			return v == "1", nil
		}
	}

	var cnt int64
	if err := s.db.WithContext(ctx).Model(&model.TenantPermission{}).
		Where("tenant_id = ? AND permission_code = ? AND status = 1", tenantID, permCode).
		Count(&cnt).Error; err != nil {
		return false, err
	}
	has := cnt > 0
	if s.rdb != nil {
		val := "0"
		if has {
			val = "1"
		}
		_ = s.rdb.Set(ctx, cacheKey, val, 5*time.Minute).Err()
	}
	return has, nil
}

// GetTenantPermissions 获取租户所有有效权限（带缓存）
func (s *Service) GetTenantPermissions(ctx context.Context, tenantID string) ([]string, error) {
	cacheKey := fmt.Sprintf("perms:all:%s", tenantID)
	if s.rdb != nil {
		if v, err := s.rdb.Get(ctx, cacheKey).Result(); err == nil {
			var codes []string
			if err := json.Unmarshal([]byte(v), &codes); err == nil {
				return codes, nil
			}
		}
	}

	var codes []string
	if err := s.db.WithContext(ctx).Model(&model.TenantPermission{}).
		Where("tenant_id = ? AND status = 1", tenantID).
		Order("permission_code").
		Pluck("permission_code", &codes).Error; err != nil {
		return nil, err
	}

	if s.rdb != nil {
		if data, err := json.Marshal(codes); err == nil {
			_ = s.rdb.Set(ctx, cacheKey, data, 5*time.Minute).Err()
		}
	}
	return codes, nil
}

// CheckModuleAccess 模块级访问检查（基于编码前缀 module:*）
func (s *Service) CheckModuleAccess(ctx context.Context, tenantID, module string) (bool, error) {
	var cnt int64
	if err := s.db.WithContext(ctx).Model(&model.TenantPermission{}).
		Where("tenant_id = ? AND status = 1 AND permission_code LIKE ?", tenantID, module+":%").
		Count(&cnt).Error; err != nil {
		return false, err
	}
	return cnt > 0, nil
}

// CreatePermission 创建权限定义
func (s *Service) CreatePermission(ctx context.Context, p *model.Permission) error {
	return s.db.WithContext(ctx).Create(p).Error
}

// ListPermissionsByLevel 获取指定层级及以下可用的所有权限
func (s *Service) ListPermissionsByLevel(ctx context.Context, level model.TenantLevel) ([]*model.Permission, error) {
	w := hierarchy.LevelWeight[string(level)]
	if w == 0 {
		return nil, fmt.Errorf("未知层级: %s", level)
	}

	// 收集权重 >= 当前层级的所有层级
	var allowed []string
	for l, lw := range hierarchy.LevelWeight {
		if lw >= w {
			allowed = append(allowed, l)
		}
	}

	var list []*model.Permission
	if err := s.db.WithContext(ctx).
		Where("level IN ?", allowed).
		Order("module, code").
		Find(&list).Error; err != nil {
		return nil, err
	}
	return list, nil
}

// ParsePermCode 解析权限编码 module:resource:action
func ParsePermCode(code string) (module, resource, action string) {
	parts := strings.SplitN(code, ":", 3)
	switch len(parts) {
	case 3:
		return parts[0], parts[1], parts[2]
	case 2:
		return parts[0], parts[1], ""
	case 1:
		return parts[0], "", ""
	}
	return "", "", ""
}

// invalidateCache 清除租户权限缓存
func (s *Service) invalidateCache(ctx context.Context, tenantID string) {
	if s.rdb == nil {
		return
	}
	// 清单级
	_ = s.rdb.Del(ctx, fmt.Sprintf("perms:all:%s", tenantID)).Err()
	// 单项级：使用 SCAN 避免 KEYS 在集群下阻塞
	pattern := fmt.Sprintf("perm:%s:*", tenantID)
	var cursor uint64
	for {
		keys, next, err := s.rdb.Scan(ctx, cursor, pattern, 100).Result()
		if err != nil {
			return
		}
		if len(keys) > 0 {
			_ = s.rdb.Del(ctx, keys...).Err()
		}
		if next == 0 {
			break
		}
		cursor = next
	}
}
