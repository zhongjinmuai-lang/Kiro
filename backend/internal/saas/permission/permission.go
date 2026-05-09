package permission

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"

	"github.com/zhongjinmuai-lang/mu-framework/internal/model"
	"github.com/zhongjinmuai-lang/mu-framework/internal/saas/hierarchy"
)

// Service 权限服务
type Service struct {
	db        *pgxpool.Pool
	redis     *redis.Client
	hierarchy *hierarchy.Service
	logger    *slog.Logger
}

// NewService 创建权限服务
func NewService(db *pgxpool.Pool, rdb *redis.Client, hierarchySvc *hierarchy.Service) *Service {
	return &Service{
		db:        db,
		redis:     rdb,
		hierarchy: hierarchySvc,
		logger:    slog.Default().With("module", "permission"),
	}
}

// GrantInput 授权入参
type GrantInput struct {
	GranterTenantID string   `json:"granter_tenant_id"` // 授权方租户ID
	GranteeTenantID string   `json:"grantee_tenant_id"` // 被授权方租户ID
	PermissionCodes []string `json:"permission_codes"`  // 权限编码列表
}

// Grant 授予权限（严格遵循三级管控规则）
func (s *Service) Grant(ctx context.Context, input *GrantInput) error {
	// 1. 验证控制流向（上级→下级）
	if err := s.hierarchy.ValidateControlFlow(ctx, input.GranterTenantID, input.GranteeTenantID); err != nil {
		return fmt.Errorf("授权失败: %w", err)
	}

	// 2. 验证授权方自身是否拥有这些权限
	for _, code := range input.PermissionCodes {
		has, err := s.HasPermission(ctx, input.GranterTenantID, code)
		if err != nil {
			return fmt.Errorf("检查权限失败: %w", err)
		}
		if !has {
			return fmt.Errorf("授权方不拥有权限 %s，无法授予", code)
		}
	}

	// 3. 执行授权
	for _, code := range input.PermissionCodes {
		if err := s.grantSingle(ctx, input.GranterTenantID, input.GranteeTenantID, code); err != nil {
			return fmt.Errorf("授权 %s 失败: %w", code, err)
		}
	}

	// 4. 清除缓存
	s.invalidateCache(ctx, input.GranteeTenantID)

	s.logger.Info("权限授予成功",
		"granter", input.GranterTenantID,
		"grantee", input.GranteeTenantID,
		"permissions", input.PermissionCodes,
	)
	return nil
}

// Revoke 回收权限（上级可随时回收下级权限，级联回收）
func (s *Service) Revoke(ctx context.Context, revokerTenantID, targetTenantID string, permCodes []string) error {
	// 验证控制流向
	if err := s.hierarchy.ValidateControlFlow(ctx, revokerTenantID, targetTenantID); err != nil {
		return fmt.Errorf("回收权限失败: %w", err)
	}

	for _, code := range permCodes {
		// 回收目标租户的权限
		if err := s.revokeSingle(ctx, targetTenantID, code); err != nil {
			return fmt.Errorf("回收权限 %s 失败: %w", code, err)
		}

		// 级联回收：目标租户的所有下级也失去该权限
		descendants, err := s.hierarchy.GetDescendants(ctx, targetTenantID)
		if err != nil {
			return fmt.Errorf("获取下级租户失败: %w", err)
		}
		for _, descID := range descendants {
			if err := s.revokeSingle(ctx, descID, code); err != nil {
				s.logger.Error("级联回收权限失败", "tenant_id", descID, "perm", code, "error", err)
			}
			s.invalidateCache(ctx, descID)
		}
	}

	// 清除缓存
	s.invalidateCache(ctx, targetTenantID)

	s.logger.Info("权限回收成功（含级联）",
		"revoker", revokerTenantID,
		"target", targetTenantID,
		"permissions", permCodes,
	)
	return nil
}

// HasPermission 检查租户是否拥有某权限
func (s *Service) HasPermission(ctx context.Context, tenantID string, permCode string) (bool, error) {
	// 优先从缓存读取
	cacheKey := fmt.Sprintf("perm:%s:%s", tenantID, permCode)
	cached, err := s.redis.Get(ctx, cacheKey).Result()
	if err == nil {
		return cached == "1", nil
	}

	// 从数据库查询
	query := `SELECT EXISTS(
		SELECT 1 FROM tenant_permissions
		WHERE tenant_id = $1 AND permission_code = $2 AND status = 1
	)`
	var exists bool
	if err := s.db.QueryRow(ctx, query, tenantID, permCode).Scan(&exists); err != nil {
		return false, fmt.Errorf("查询权限失败: %w", err)
	}

	// 写入缓存（5分钟过期）
	cacheVal := "0"
	if exists {
		cacheVal = "1"
	}
	s.redis.Set(ctx, cacheKey, cacheVal, 5*time.Minute)

	return exists, nil
}

// GetTenantPermissions 获取租户的所有权限列表
func (s *Service) GetTenantPermissions(ctx context.Context, tenantID string) ([]string, error) {
	// 尝试从缓存获取
	cacheKey := fmt.Sprintf("perms:all:%s", tenantID)
	cached, err := s.redis.Get(ctx, cacheKey).Result()
	if err == nil {
		var perms []string
		if err := json.Unmarshal([]byte(cached), &perms); err == nil {
			return perms, nil
		}
	}

	// 从数据库查询
	query := `SELECT permission_code FROM tenant_permissions
		WHERE tenant_id = $1 AND status = 1 ORDER BY permission_code`

	rows, err := s.db.Query(ctx, query, tenantID)
	if err != nil {
		return nil, fmt.Errorf("查询权限列表失败: %w", err)
	}
	defer rows.Close()

	var perms []string
	for rows.Next() {
		var code string
		if err := rows.Scan(&code); err != nil {
			return nil, err
		}
		perms = append(perms, code)
	}

	// 写入缓存
	data, _ := json.Marshal(perms)
	s.redis.Set(ctx, cacheKey, string(data), 5*time.Minute)

	return perms, nil
}

// CheckModuleAccess 检查模块访问权限（基于权限编码前缀）
// 编码规则：module:resource:action，如 payment:channel:create
func (s *Service) CheckModuleAccess(ctx context.Context, tenantID string, module string) (bool, error) {
	query := `SELECT EXISTS(
		SELECT 1 FROM tenant_permissions
		WHERE tenant_id = $1 AND permission_code LIKE $2 AND status = 1
	)`
	pattern := module + ":%"
	var exists bool
	if err := s.db.QueryRow(ctx, query, tenantID, pattern).Scan(&exists); err != nil {
		return false, err
	}
	return exists, nil
}

// CreatePermission 创建权限定义
func (s *Service) CreatePermission(ctx context.Context, perm *model.Permission) error {
	perm.ID = uuid.New().String()
	perm.CreatedAt = time.Now()
	perm.UpdatedAt = time.Now()

	query := `INSERT INTO permissions (id, module, name, code, level, parent_id, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`

	_, err := s.db.Exec(ctx, query,
		perm.ID, perm.Module, perm.Name, perm.Code,
		perm.Level, perm.ParentID, perm.CreatedAt, perm.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("创建权限定义失败: %w", err)
	}
	return nil
}

// ListPermissionsByLevel 获取某层级可用的所有权限
func (s *Service) ListPermissionsByLevel(ctx context.Context, level model.TenantLevel) ([]*model.Permission, error) {
	// 获取该层级及以下的所有权限
	levelWeight := hierarchy.LevelWeight[string(level)]

	query := `SELECT id, module, name, code, level, parent_id, created_at, updated_at
		FROM permissions WHERE deleted_at IS NULL ORDER BY module, code`

	rows, err := s.db.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("查询权限列表失败: %w", err)
	}
	defer rows.Close()

	var perms []*model.Permission
	for rows.Next() {
		p := &model.Permission{}
		if err := rows.Scan(
			&p.ID, &p.Module, &p.Name, &p.Code,
			&p.Level, &p.ParentID, &p.CreatedAt, &p.UpdatedAt,
		); err != nil {
			return nil, err
		}
		// 过滤：只返回权重 >= 当前层级的权限
		permWeight := hierarchy.LevelWeight[string(p.Level)]
		if permWeight >= levelWeight {
			perms = append(perms, p)
		}
	}

	return perms, nil
}

// ========== 内部方法 ==========

func (s *Service) grantSingle(ctx context.Context, granterID, granteeID, permCode string) error {
	id := uuid.New().String()
	query := `INSERT INTO tenant_permissions (id, tenant_id, permission_code, granted_by, status, created_at, updated_at)
		VALUES ($1, $2, $3, $4, 1, $5, $5)
		ON CONFLICT (tenant_id, permission_code)
		DO UPDATE SET status = 1, granted_by = $4, updated_at = $5`

	now := time.Now()
	_, err := s.db.Exec(ctx, query, id, granteeID, permCode, granterID, now)
	return err
}

func (s *Service) revokeSingle(ctx context.Context, tenantID, permCode string) error {
	query := `UPDATE tenant_permissions SET status = 0, updated_at = $1
		WHERE tenant_id = $2 AND permission_code = $3`
	_, err := s.db.Exec(ctx, query, time.Now(), tenantID, permCode)
	return err
}

func (s *Service) invalidateCache(ctx context.Context, tenantID string) {
	// 删除该租户所有权限相关缓存
	pattern := fmt.Sprintf("perm:%s:*", tenantID)
	keys, _ := s.redis.Keys(ctx, pattern).Result()
	if len(keys) > 0 {
		s.redis.Del(ctx, keys...)
	}

	// 删除全量缓存
	allKey := fmt.Sprintf("perms:all:%s", tenantID)
	s.redis.Del(ctx, allKey)
}

// ParsePermCode 解析权限编码
// 格式：module:resource:action
func ParsePermCode(code string) (module, resource, action string) {
	parts := strings.SplitN(code, ":", 3)
	switch len(parts) {
	case 3:
		return parts[0], parts[1], parts[2]
	case 2:
		return parts[0], parts[1], ""
	case 1:
		return parts[0], "", ""
	default:
		return "", "", ""
	}
}
