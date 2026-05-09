// Package model 所有领域实体（GORM 模型）
package model

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// BaseModel 所有实体继承
//
// 软删除：使用 GORM 原生 gorm.DeletedAt（TIMESTAMPTZ NULL），
// 与迁移 SQL `deleted_at TIMESTAMPTZ` 完全对齐。
//   - 未删除：deleted_at IS NULL
//   - 已删除：deleted_at = 删除时间
type BaseModel struct {
	ID        string         `gorm:"column:id;type:uuid;primaryKey;default:uuid_generate_v4()" json:"id"`
	CreatedAt time.Time      `gorm:"column:created_at;not null;default:NOW()" json:"created_at"`
	UpdatedAt time.Time      `gorm:"column:updated_at;not null;default:NOW()" json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"column:deleted_at;index" json:"-"`
}

// BeforeCreate 创建前 Hook：自动分配 UUID、刷新时间戳
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

// BeforeUpdate 更新前 Hook
func (m *BaseModel) BeforeUpdate(tx *gorm.DB) error {
	m.UpdatedAt = time.Now()
	return nil
}

// TenantLevel 租户层级
type TenantLevel string

const (
	LevelDeveloper TenantLevel = "developer" // 开发商（顶层集权）
	LevelProvider  TenantLevel = "provider"  // 服务商（二级管控）
	LevelCustomer  TenantLevel = "customer"  // 终端客户（三级受限）
)

// 通用启用状态
const (
	StatusDisabled = 0
	StatusEnabled  = 1
)
