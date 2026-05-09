package model

import (
	"gorm.io/gorm"
)

// CoreModels SaaS + 中台核心模型
func CoreModels() []any {
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

// AllModels 兼容旧调用：仅核心模型
func AllModels() []any { return CoreModels() }

// AutoMigrate 自动迁移（开发使用；生产请执行 migrations/*.sql）
// 族谱域模型由 genealogy 包自行维护，在 bootstrap 组合时传入 extras
func AutoMigrate(db *gorm.DB, extras ...any) error {
	all := append(CoreModels(), extras...)
	return db.AutoMigrate(all...)
}
