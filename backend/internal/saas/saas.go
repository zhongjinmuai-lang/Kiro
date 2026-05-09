package saas

import (
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"

	"github.com/zhongjinmuai-lang/mu-framework/internal/saas/hierarchy"
	"github.com/zhongjinmuai-lang/mu-framework/internal/saas/permission"
	"github.com/zhongjinmuai-lang/mu-framework/internal/saas/tenant"
)

// Manager SaaS管控总管理器
// 整合租户管理、层级管控、权限引擎三大模块
type Manager struct {
	Tenant     *tenant.Service
	Hierarchy  *hierarchy.Service
	Permission *permission.Service
}

// NewManager 创建SaaS管控管理器
func NewManager(db *pgxpool.Pool, rdb *redis.Client) *Manager {
	hierarchySvc := hierarchy.NewService(db)
	tenantSvc := tenant.NewService(db)
	permSvc := permission.NewService(db, rdb, hierarchySvc)

	return &Manager{
		Tenant:     tenantSvc,
		Hierarchy:  hierarchySvc,
		Permission: permSvc,
	}
}
