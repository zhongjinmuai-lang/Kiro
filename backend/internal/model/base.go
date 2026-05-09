package model

import "time"

// BaseModel 基础模型（所有实体继承）
type BaseModel struct {
	ID        string    `json:"id" db:"id"`
	CreatedAt time.Time `json:"created_at" db:"created_at"`
	UpdatedAt time.Time `json:"updated_at" db:"updated_at"`
	DeletedAt *time.Time `json:"deleted_at,omitempty" db:"deleted_at"`
}

// TenantLevel 租户层级
type TenantLevel string

const (
	LevelDeveloper TenantLevel = "developer" // 开发商
	LevelProvider  TenantLevel = "provider"  // 服务商
	LevelCustomer  TenantLevel = "customer"  // 终端客户
)

// Tenant 租户模型
type Tenant struct {
	BaseModel
	Name     string      `json:"name" db:"name"`
	Code     string      `json:"code" db:"code"`           // 唯一编码
	Level    TenantLevel `json:"level" db:"level"`         // 层级
	ParentID *string     `json:"parent_id" db:"parent_id"` // 上级租户ID
	Status   int         `json:"status" db:"status"`       // 1:启用 0:禁用
	Config   string      `json:"config" db:"config"`       // JSON配置
}

// User 用户模型
type User struct {
	BaseModel
	TenantID string `json:"tenant_id" db:"tenant_id"`
	Username string `json:"username" db:"username"`
	Password string `json:"-" db:"password"`
	Nickname string `json:"nickname" db:"nickname"`
	Email    string `json:"email" db:"email"`
	Phone    string `json:"phone" db:"phone"`
	Avatar   string `json:"avatar" db:"avatar"`
	RoleID   string `json:"role_id" db:"role_id"`
	Status   int    `json:"status" db:"status"` // 1:启用 0:禁用
}

// Role 角色模型
type Role struct {
	BaseModel
	TenantID    string      `json:"tenant_id" db:"tenant_id"`
	Name        string      `json:"name" db:"name"`
	Code        string      `json:"code" db:"code"`
	Level       TenantLevel `json:"level" db:"level"`       // 角色所属层级
	Permissions string      `json:"permissions" db:"permissions"` // JSON权限列表
	Status      int         `json:"status" db:"status"`
}

// Permission 权限模型
type Permission struct {
	BaseModel
	Module   string      `json:"module" db:"module"`     // 所属模块
	Name     string      `json:"name" db:"name"`
	Code     string      `json:"code" db:"code"`         // 权限编码 如 payment:channel:create
	Level    TenantLevel `json:"level" db:"level"`       // 最低可用层级
	ParentID *string     `json:"parent_id" db:"parent_id"`
}
