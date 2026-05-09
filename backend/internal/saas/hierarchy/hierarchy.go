package hierarchy

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/zhongjinmuai-lang/mu-framework/internal/model"
)

// Level 层级常量
const (
	LevelDeveloper = "developer" // 开发商 - 最高权限
	LevelProvider  = "provider"  // 服务商 - 中间层级
	LevelCustomer  = "customer"  // 终端客户 - 最低权限
)

// LevelWeight 层级权重（数字越小权限越高）
var LevelWeight = map[string]int{
	LevelDeveloper: 1,
	LevelProvider:  2,
	LevelCustomer:  3,
}

// Service 层级管控服务
type Service struct {
	db     *pgxpool.Pool
	logger *slog.Logger
}

// NewService 创建层级管控服务
func NewService(db *pgxpool.Pool) *Service {
	return &Service{
		db:     db,
		logger: slog.Default().With("module", "hierarchy"),
	}
}

// CanGrant 判断是否有权限授予（上级只能授予不超过自身的权限给下级）
func (s *Service) CanGrant(granterLevel, granteeLevel string, permCode string) bool {
	granterWeight, ok1 := LevelWeight[granterLevel]
	granteeWeight, ok2 := LevelWeight[granteeLevel]

	if !ok1 || !ok2 {
		return false
	}

	// 只有上级才能授予下级权限
	if granterWeight >= granteeWeight {
		return false
	}

	return true
}

// CanAccess 判断某层级是否有权访问某资源
func (s *Service) CanAccess(currentLevel string, requiredLevel string) bool {
	currentWeight, ok1 := LevelWeight[currentLevel]
	requiredWeight, ok2 := LevelWeight[requiredLevel]

	if !ok1 || !ok2 {
		return false
	}

	// 权重越小，权限越高，可以访问同级或低级资源
	return currentWeight <= requiredWeight
}

// GetAncestorChain 获取租户的完整上级链路
// 返回从当前租户到顶级的完整层级链
func (s *Service) GetAncestorChain(ctx context.Context, tenantID string) ([]*model.Tenant, error) {
	var chain []*model.Tenant
	currentID := tenantID

	// 最大遍历深度为3（开发商→服务商→终端客户）
	for i := 0; i < 3; i++ {
		query := `SELECT id, name, code, level, parent_id, status, config, created_at, updated_at
			FROM tenants WHERE id = $1 AND deleted_at IS NULL`

		tenant := &model.Tenant{}
		row := s.db.QueryRow(ctx, query, currentID)
		err := row.Scan(
			&tenant.ID, &tenant.Name, &tenant.Code, &tenant.Level,
			&tenant.ParentID, &tenant.Status, &tenant.Config,
			&tenant.CreatedAt, &tenant.UpdatedAt,
		)
		if err != nil {
			break
		}

		chain = append(chain, tenant)

		if tenant.ParentID == nil {
			break // 已到顶级
		}
		currentID = *tenant.ParentID
	}

	return chain, nil
}

// GetDescendants 获取租户的所有下级租户ID
func (s *Service) GetDescendants(ctx context.Context, tenantID string) ([]string, error) {
	// 递归查询所有下级
	query := `WITH RECURSIVE descendants AS (
		SELECT id FROM tenants WHERE parent_id = $1 AND deleted_at IS NULL
		UNION ALL
		SELECT t.id FROM tenants t
		INNER JOIN descendants d ON t.parent_id = d.id
		WHERE t.deleted_at IS NULL
	)
	SELECT id FROM descendants`

	rows, err := s.db.Query(ctx, query, tenantID)
	if err != nil {
		return nil, fmt.Errorf("查询下级租户失败: %w", err)
	}
	defer rows.Close()

	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}

	return ids, nil
}

// ValidateControlFlow 验证控制流向（严格单向：上级→下级）
// 确保操作只能从上级发起，作用于下级
func (s *Service) ValidateControlFlow(ctx context.Context, operatorTenantID, targetTenantID string) error {
	operator, err := s.getTenantLevel(ctx, operatorTenantID)
	if err != nil {
		return fmt.Errorf("获取操作者层级失败: %w", err)
	}

	target, err := s.getTenantLevel(ctx, targetTenantID)
	if err != nil {
		return fmt.Errorf("获取目标层级失败: %w", err)
	}

	operatorWeight := LevelWeight[operator]
	targetWeight := LevelWeight[target]

	if operatorWeight >= targetWeight {
		return fmt.Errorf("权限不足：%s 不能对 %s 执行管控操作", operator, target)
	}

	// 校验是否在同一链路上（操作者必须是目标的上级）
	chain, err := s.GetAncestorChain(ctx, targetTenantID)
	if err != nil {
		return fmt.Errorf("获取层级链路失败: %w", err)
	}

	inChain := false
	for _, t := range chain {
		if t.ID == operatorTenantID {
			inChain = true
			break
		}
	}

	if !inChain {
		return fmt.Errorf("操作者不在目标租户的上级链路中，无权操作")
	}

	s.logger.Info("控制流验证通过",
		"operator", operatorTenantID,
		"target", targetTenantID,
	)
	return nil
}

// getTenantLevel 获取租户层级
func (s *Service) getTenantLevel(ctx context.Context, tenantID string) (string, error) {
	query := `SELECT level FROM tenants WHERE id = $1 AND deleted_at IS NULL`
	var level string
	err := s.db.QueryRow(ctx, query, tenantID).Scan(&level)
	if err != nil {
		return "", err
	}
	return level, nil
}
