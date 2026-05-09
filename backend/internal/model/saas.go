package model

// Tenant 租户表（三级SaaS管控核心）
type Tenant struct {
	BaseModel
	Name     string      `gorm:"column:name;type:varchar(100);not null" json:"name"`
	Code     string      `gorm:"column:code;type:varchar(50);not null;uniqueIndex" json:"code"`
	Level    TenantLevel `gorm:"column:level;type:varchar(20);not null;index" json:"level"`
	ParentID *string     `gorm:"column:parent_id;type:uuid;index" json:"parent_id,omitempty"`
	Status   int         `gorm:"column:status;type:smallint;not null;default:1" json:"status"`
	Config   string      `gorm:"column:config;type:jsonb;default:'{}'" json:"config"`
}

// TableName 自定义表名
func (Tenant) TableName() string { return "tenants" }

// User 用户
type User struct {
	BaseModel
	TenantID string `gorm:"column:tenant_id;type:uuid;not null;index" json:"tenant_id"`
	Username string `gorm:"column:username;type:varchar(50);not null" json:"username"`
	Password string `gorm:"column:password;type:varchar(255);not null" json:"-"`
	Nickname string `gorm:"column:nickname;type:varchar(100)" json:"nickname"`
	Email    string `gorm:"column:email;type:varchar(100)" json:"email"`
	Phone    string `gorm:"column:phone;type:varchar(20)" json:"phone"`
	Avatar   string `gorm:"column:avatar;type:varchar(500)" json:"avatar"`
	RoleID   string `gorm:"column:role_id;type:uuid" json:"role_id"`
	Status   int    `gorm:"column:status;type:smallint;not null;default:1" json:"status"`
}

func (User) TableName() string { return "users" }

// Role 角色
type Role struct {
	BaseModel
	TenantID    string      `gorm:"column:tenant_id;type:uuid;not null;index" json:"tenant_id"`
	Name        string      `gorm:"column:name;type:varchar(50);not null" json:"name"`
	Code        string      `gorm:"column:code;type:varchar(50);not null" json:"code"`
	Level       TenantLevel `gorm:"column:level;type:varchar(20);not null" json:"level"`
	Permissions string      `gorm:"column:permissions;type:jsonb;default:'[]'" json:"permissions"`
	Status      int         `gorm:"column:status;type:smallint;not null;default:1" json:"status"`
}

func (Role) TableName() string { return "roles" }

// Permission 权限定义（全局）
type Permission struct {
	BaseModel
	Module   string      `gorm:"column:module;type:varchar(50);not null;index" json:"module"`
	Name     string      `gorm:"column:name;type:varchar(100);not null" json:"name"`
	Code     string      `gorm:"column:code;type:varchar(100);not null;uniqueIndex" json:"code"`
	Level    TenantLevel `gorm:"column:level;type:varchar(20);not null" json:"level"`
	ParentID *string     `gorm:"column:parent_id;type:uuid" json:"parent_id,omitempty"`
}

func (Permission) TableName() string { return "permissions" }

// TenantPermission 租户权限授予
type TenantPermission struct {
	BaseModel
	TenantID       string  `gorm:"column:tenant_id;type:uuid;not null;uniqueIndex:idx_tenant_perm,priority:1" json:"tenant_id"`
	PermissionCode string  `gorm:"column:permission_code;type:varchar(100);not null;uniqueIndex:idx_tenant_perm,priority:2" json:"permission_code"`
	GrantedBy      *string `gorm:"column:granted_by;type:uuid" json:"granted_by,omitempty"`
	Status         int     `gorm:"column:status;type:smallint;not null;default:1" json:"status"`
}

func (TenantPermission) TableName() string { return "tenant_permissions" }
