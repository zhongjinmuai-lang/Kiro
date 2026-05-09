package platform

import (
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/zhongjinmuai-lang/mu-framework/internal/platform/notify"
	"github.com/zhongjinmuai-lang/mu-framework/internal/platform/payment"
	"github.com/zhongjinmuai-lang/mu-framework/internal/platform/storage"
	"github.com/zhongjinmuai-lang/mu-framework/internal/saas/hierarchy"
)

// Manager 中台总管理器
// 统一管理三大中台：支付、存储、通知
type Manager struct {
	Payment *payment.Service
	Storage *storage.Service
	Notify  *notify.Service
}

// NewManager 创建中台管理器
func NewManager(db *pgxpool.Pool, hierarchySvc *hierarchy.Service) *Manager {
	return &Manager{
		Payment: payment.NewService(db, hierarchySvc),
		Storage: storage.NewService(db, hierarchySvc),
		Notify:  notify.NewService(db, hierarchySvc),
	}
}
