// Package hierarchy 三级SaaS层级管控：权限链路校验、祖先/后代查询
// 底层复用 pkg/hierarchy 提供的 PG 递归 CTE 工具
package hierarchy

import (
	"context"
	"errors"
	"fmt"

	"gorm.io/gorm"

	"github.com/zhongjinmuai-lang/mu-framework/internal/model"
	pkgh "github.com/zhongjinmuai-lang/mu-framework/pkg/hierarchy"
)

// Level 常量
const (
	LevelDeveloper = string(model.LevelDeveloper)
	LevelProvider  = string(model.LevelProvider)
	LevelCustomer  = string(model.LevelCustomer)
)

// LevelWeight 权重（数字越小权限越高）
var LevelWeight = map[string]int{
	LevelDeveloper: 1,
	LevelProvider:  2,
	LevelCustomer:  3,
}

// Service 层级管控服务
type Service struct {
	db *gorm.DB
}

// NewService 创建服务
func NewService(db *gorm.DB) *Service {
	return &Service{db: db}
}

// CanGrant 上级授予下级（严格要求操作者权重 < 目标）
func (s *Service) CanGrant(granterLevel, granteeLevel string) bool {
	a, ok1 := LevelWeight[granterLevel]
	b, ok2 := LevelWeight[granteeLevel]
	return ok1 && ok2 && a < b
}

// CanAccess 权重更小或相等视为有权访问
func (s *Service) CanAccess(currentLevel, requiredLevel string) bool {
	a, ok1 := LevelWeight[currentLevel]
	b, ok2 := LevelWeight[requiredLevel]
	return ok1 && ok2 && a <= b
}

// GetAncestorChain 获取从当前到顶级的完整链路（近→远）
func (s *Service) GetAncestorChain(ctx context.Context, tenantID string) ([]pkgh.Node, error) {
	return pkgh.Ancestors(ctx, s.db, "tenants", tenantID, true)
}

// GetDescendants 获取所有下级ID（不含自身）
func (s *Service) GetDescendants(ctx context.Context, tenantID string) ([]string, error) {
	nodes, err := pkgh.Descendants(ctx, s.db, "tenants", tenantID, false)
	if err != nil {
		return nil, err
	}
	ids := make([]string, 0, len(nodes))
	for _, n := range nodes {
		ids = append(ids, n.ID)
	}
	return ids, nil
}

// IsAncestor 是否为祖先（严格）
func (s *Service) IsAncestor(ctx context.Context, ancestorID, descendantID string) (bool, error) {
	return pkgh.IsAncestor(ctx, s.db, "tenants", ancestorID, descendantID)
}

// ValidateControlFlow 校验控制流向（上级→下级，单向）
//  1. operator 的层级权重必须小于 target
//  2. operator 必须在 target 的祖先链路上
func (s *Service) ValidateControlFlow(ctx context.Context, operatorTenantID, targetTenantID string) error {
	opLvl, err := s.getLevel(ctx, operatorTenantID)
	if err != nil {
		return fmt.Errorf("获取操作者层级失败: %w", err)
	}
	tgLvl, err := s.getLevel(ctx, targetTenantID)
	if err != nil {
		return fmt.Errorf("获取目标层级失败: %w", err)
	}

	if LevelWeight[opLvl] >= LevelWeight[tgLvl] {
		return fmt.Errorf("权限不足：%s 不能对 %s 执行管控操作", opLvl, tgLvl)
	}

	isAncestor, err := s.IsAncestor(ctx, operatorTenantID, targetTenantID)
	if err != nil {
		return fmt.Errorf("校验祖先关系失败: %w", err)
	}
	if !isAncestor {
		return errors.New("操作者不在目标租户的上级链路中，无权操作")
	}
	return nil
}

func (s *Service) getLevel(ctx context.Context, tenantID string) (string, error) {
	var lvl string
	err := s.db.WithContext(ctx).
		Model(&model.Tenant{}).
		Select("level").
		Where("id = ?", tenantID).
		Scan(&lvl).Error
	if err != nil {
		return "", fmt.Errorf("查询租户层级失败: %w", err)
	}
	if lvl == "" {
		return "", fmt.Errorf("租户 %s 不存在或无层级信息", tenantID)
	}
	return lvl, nil
}
