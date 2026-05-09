package model

import (
	"gorm.io/gorm"
)

// AllModels 所有 GORM 模型清单（用于 AutoMigrate 和依赖注入）
func AllModels() []any {
	return []any{
		// SaaS 核心
		&Tenant{}, &User{}, &Role{}, &Permission{}, &TenantPermission{},
		// 支付中台
		&PaymentChannel{}, &TenantPaymentAuth{}, &PaymentOrder{},
		// 存储中台
		&StorageSource{}, &StorageFile{}, &StorageQuota{},
		// 通知中台
		&NotifyChannel{}, &NotifyTemplate{}, &NotifyMessage{},
	}
}

// AutoMigrate 自动迁移所有模型（开发环境使用，生产请使用 migrations SQL）
func AutoMigrate(db *gorm.DB) error {
	return db.AutoMigrate(AllModels()...)
}
