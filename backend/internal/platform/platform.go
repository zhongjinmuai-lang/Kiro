// Package platform 三大统一中台聚合器：支付 / 存储 / 通知
package platform

import (
	"gorm.io/gorm"

	"github.com/zhongjinmuai-lang/mu-framework/internal/platform/notify"
	"github.com/zhongjinmuai-lang/mu-framework/internal/platform/payment"
	"github.com/zhongjinmuai-lang/mu-framework/internal/platform/storage"
	"github.com/zhongjinmuai-lang/mu-framework/internal/saas/hierarchy"
)

// Manager 中台总管理器
type Manager struct {
	Payment *payment.Service
	Storage *storage.Service
	Notify  *notify.Service
}

// NewManager 创建中台管理器
func NewManager(db *gorm.DB, h *hierarchy.Service) *Manager {
	return &Manager{
		Payment: payment.NewService(db, h),
		Storage: storage.NewService(db, h),
		Notify:  notify.NewService(db, h),
	}
}
