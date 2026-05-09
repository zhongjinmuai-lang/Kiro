// Package saas 三级SaaS管控聚合器
package saas

import (
	"context"
	"fmt"

	"gorm.io/gorm"

	"github.com/zhongjinmuai-lang/mu-framework/internal/model"
	"github.com/zhongjinmuai-lang/mu-framework/internal/saas/hierarchy"
	"github.com/zhongjinmuai-lang/mu-framework/internal/saas/permission"
	"github.com/zhongjinmuai-lang/mu-framework/internal/saas/tenant"
	"github.com/zhongjinmuai-lang/mu-framework/pkg/cache"
)

// Manager SaaS 管控总管理器
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

// ========== 管理后台便捷方法 ==========

// CreateProviderInput 开发商创建服务商
type CreateProviderInput struct {
	Name   string `json:"name" binding:"required,max=100"`
	Code   string `json:"code" binding:"required,max=50"`
	Config string `json:"config"`
}

// CreateProvider 开发商下属创建服务商
func (m *Manager) CreateProvider(ctx context.Context, developerID string, in *CreateProviderInput) (*model.Tenant, error) {
	return m.Tenant.Create(ctx, &tenant.CreateInput{
		Name:     in.Name,
		Code:     in.Code,
		Level:    model.LevelProvider,
		ParentID: &developerID,
		Config:   in.Config,
	})
}

// CreateCustomerInput 服务商创建终端客户
type CreateCustomerInput struct {
	Name   string `json:"name" binding:"required,max=100"`
	Code   string `json:"code" binding:"required,max=50"`
	Config string `json:"config"`
}

// CreateCustomer 服务商下属创建客户
func (m *Manager) CreateCustomer(ctx context.Context, providerID string, in *CreateCustomerInput) (*model.Tenant, error) {
	// 校验 providerID 确实是 provider
	parent, err := m.Tenant.GetByID(ctx, providerID)
	if err != nil {
		return nil, err
	}
	if parent.Level != model.LevelProvider {
		return nil, fmt.Errorf("仅服务商可创建终端客户")
	}
	return m.Tenant.Create(ctx, &tenant.CreateInput{
		Name:     in.Name,
		Code:     in.Code,
		Level:    model.LevelCustomer,
		ParentID: &providerID,
		Config:   in.Config,
	})
}
