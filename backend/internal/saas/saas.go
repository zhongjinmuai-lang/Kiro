// Package saas 三级SaaS管控聚合器
package saas

import (
	"gorm.io/gorm"

	"github.com/zhongjinmuai-lang/mu-framework/internal/saas/hierarchy"
	"github.com/zhongjinmuai-lang/mu-framework/internal/saas/permission"
	"github.com/zhongjinmuai-lang/mu-framework/internal/saas/tenant"
	"github.com/zhongjinmuai-lang/mu-framework/pkg/cache"
)

// Manager SaaS 管控总管理器：整合租户/层级/权限三大子服务
type Manager struct {
	Tenant     *tenant.Service
	Hierarchy  *hierarchy.Service
	Permission *permission.Service
}

// NewManager 创建管理器
func NewManager(db *gorm.DB, rdb *cache.Client) *Manager {
	h := hierarchy.NewService(db)
	return &Manager{
		Tenant:     tenant.NewService(db),
		Hierarchy:  h,
		Permission: permission.NewService(db, rdb, h),
	}
}
