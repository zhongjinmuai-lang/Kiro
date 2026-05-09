// Package model 所有领域实体（GORM 模型）
// 统一约定：
//   - 使用 UUID 主键（PG uuid_generate_v4）
//   - 自动时间戳（created_at / updated_at，由 GORM 自动维护）
//   - 软删除（deleted_at），使用 soft_delete 插件兼容 PG 索引
//   - 所有表名使用复数蛇形（users, payment_orders 等）
package model

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/plugin/soft_delete"
)

// BaseModel 所有实体继承
type BaseModel struct {
	ID        string                `gorm:"column:id;type:uuid;primaryKey;default:uuid_generate_v4()" json:"id"`
	CreatedAt time.Time             `gorm:"column:created_at;not null;default:NOW()" json:"created_at"`
	UpdatedAt time.Time             `gorm:"column:updated_at;not null;default:NOW()" json:"updated_at"`
	DeletedAt soft_delete.DeletedAt `gorm:"column:deleted_at;softDelete:flag" json:"-"`
}

// BeforeCreate 创建前 Hook：若未提供 ID，则自动生成 UUID
func (m *BaseModel) BeforeCreate(tx *gorm.DB) error {
	if m.ID == "" {
		m.ID = uuid.New().String()
	}
	if m.CreatedAt.IsZero() {
		m.CreatedAt = time.Now()
	}
	m.UpdatedAt = time.Now()
	return nil
}

// BeforeUpdate 更新前 Hook：自动刷新 UpdatedAt
func (m *BaseModel) BeforeUpdate(tx *gorm.DB) error {
	m.UpdatedAt = time.Now()
	return nil
}

// ========== 枚举常量 ==========

// TenantLevel 租户层级
type TenantLevel string

const (
	LevelDeveloper TenantLevel = "developer" // 开发商（顶层集权）
	LevelProvider  TenantLevel = "provider"  // 服务商（二级管控）
	LevelCustomer  TenantLevel = "customer"  // 终端客户（三级受限）
)

// Status 通用启用状态
const (
	StatusDisabled = 0
	StatusEnabled  = 1
)
