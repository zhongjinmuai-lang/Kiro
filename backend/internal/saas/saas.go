// Package saas 三级SaaS管控聚合器
package saas

import (
	"context"
	"errors"
	"fmt"

	"go.uber.org/zap"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"

	"github.com/zhongjinmuai-lang/mu-framework/internal/model"
	"github.com/zhongjinmuai-lang/mu-framework/internal/saas/hierarchy"
	"github.com/zhongjinmuai-lang/mu-framework/internal/saas/permission"
	"github.com/zhongjinmuai-lang/mu-framework/internal/saas/tenant"
	"github.com/zhongjinmuai-lang/mu-framework/pkg/cache"
	"github.com/zhongjinmuai-lang/mu-framework/pkg/logger"
)

// Manager SaaS 管控总管理器
type Manager struct {
	db         *gorm.DB
	Tenant     *tenant.Service
	Hierarchy  *hierarchy.Service
	Permission *permission.Service
}

// NewManager 创建管理器
func NewManager(db *gorm.DB, rdb *cache.Client) *Manager {
	h := hierarchy.NewService(db)
	return &Manager{
		db:         db,
		Tenant:     tenant.NewService(db),
		Hierarchy:  h,
		Permission: permission.NewService(db, rdb, h),
	}
}

// ========== 创建下级租户并同步开账号 ==========

// CreateProviderInput 开发商创建服务商（含初始管理员账号）
type CreateProviderInput struct {
	Name   string `json:"name"           binding:"required,max=100"`
	Code   string `json:"code"           binding:"required,max=50"`
	Config string `json:"config"`
	// 初始管理员账号（必填，创建后登录服务商后台用）
	AdminUsername string `json:"admin_username" binding:"required,min=3,max=50"`
	AdminPassword string `json:"admin_password" binding:"required,min=6,max=64"`
	AdminNickname string `json:"admin_nickname" binding:"max=100"`
	AdminEmail    string `json:"admin_email"    binding:"omitempty,email,max=100"`
	AdminPhone    string `json:"admin_phone"    binding:"max=20"`
}

// CreatedTenant 创建结果（租户+初始管理员）
type CreatedTenant struct {
	Tenant *model.Tenant `json:"tenant"`
	Admin  *model.User   `json:"admin"`
}

// CreateProvider 开发商创建服务商，并同步创建初始管理员账号
//
// 事务保证：租户、角色、用户、权限授予同时成功，否则全部回滚。
// 默认授权：服务商获得其层级+下级的所有权限。
func (m *Manager) CreateProvider(ctx context.Context, developerID string, in *CreateProviderInput) (*CreatedTenant, error) {
	return m.createTenantWithAdmin(ctx, &createOpts{
		Name:          in.Name,
		Code:          in.Code,
		Level:         model.LevelProvider,
		ParentID:      developerID,
		Config:        in.Config,
		RoleName:      "服务商管理员",
		RoleCode:      "provider_admin",
		AdminUsername: in.AdminUsername,
		AdminPassword: in.AdminPassword,
		AdminNickname: firstNonEmpty(in.AdminNickname, in.Name+"管理员"),
		AdminEmail:    in.AdminEmail,
		AdminPhone:    in.AdminPhone,
		GrantedByID:   developerID,
	})
}

// CreateCustomerInput 服务商创建终端客户（含初始管理员账号）
type CreateCustomerInput struct {
	Name          string `json:"name"           binding:"required,max=100"`
	Code          string `json:"code"           binding:"required,max=50"`
	Config        string `json:"config"`
	AdminUsername string `json:"admin_username" binding:"required,min=3,max=50"`
	AdminPassword string `json:"admin_password" binding:"required,min=6,max=64"`
	AdminNickname string `json:"admin_nickname" binding:"max=100"`
	AdminEmail    string `json:"admin_email"    binding:"omitempty,email,max=100"`
	AdminPhone    string `json:"admin_phone"    binding:"max=20"`
}

// CreateCustomer 服务商创建终端客户（开账号）
//
// 核心能力：服务商直接给终端客户一个可登录的账号，用户拿到后
// 可在终端客户业务后台直接登录。
func (m *Manager) CreateCustomer(ctx context.Context, providerID string, in *CreateCustomerInput) (*CreatedTenant, error) {
	// 校验父级必须是服务商
	parent, err := m.Tenant.GetByID(ctx, providerID)
	if err != nil {
		return nil, err
	}
	if parent.Level != model.LevelProvider {
		return nil, errors.New("仅服务商可创建终端客户")
	}
	return m.createTenantWithAdmin(ctx, &createOpts{
		Name:          in.Name,
		Code:          in.Code,
		Level:         model.LevelCustomer,
		ParentID:      providerID,
		Config:        in.Config,
		RoleName:      "家族族长",
		RoleCode:      "family_head",
		AdminUsername: in.AdminUsername,
		AdminPassword: in.AdminPassword,
		AdminNickname: firstNonEmpty(in.AdminNickname, in.Name+"族长"),
		AdminEmail:    in.AdminEmail,
		AdminPhone:    in.AdminPhone,
		GrantedByID:   providerID,
	})
}

// ResetTenantPasswordInput 服务商/开发商重置下级租户管理员密码
type ResetTenantPasswordInput struct {
	TenantID    string `json:"tenant_id"    binding:"required"`
	Username    string `json:"username"     binding:"required"`
	NewPassword string `json:"new_password" binding:"required,min=6,max=64"`
}

// ResetTenantPassword 重置下级租户管理员密码
//
// 校验上级权限链路（developer→provider→customer）
func (m *Manager) ResetTenantPassword(ctx context.Context, operatorID string, in *ResetTenantPasswordInput) error {
	if err := m.Hierarchy.ValidateControlFlow(ctx, operatorID, in.TenantID); err != nil {
		return fmt.Errorf("无权重置密码: %w", err)
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(in.NewPassword), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	res := m.db.WithContext(ctx).Model(&model.User{}).
		Where("tenant_id = ? AND username = ?", in.TenantID, in.Username).
		Update("password", string(hash))
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return errors.New("用户不存在")
	}
	logger.WithContext(ctx).Info("租户管理员密码已重置",
		zap.String("operator", operatorID),
		zap.String("tenant_id", in.TenantID),
		zap.String("username", in.Username),
	)
	return nil
}

// ========== 内部事务实现 ==========

type createOpts struct {
	Name          string
	Code          string
	Level         model.TenantLevel
	ParentID      string
	Config        string
	RoleName      string
	RoleCode      string
	AdminUsername string
	AdminPassword string
	AdminNickname string
	AdminEmail    string
	AdminPhone    string
	GrantedByID   string
}

// createTenantWithAdmin 事务性创建：租户 + 角色 + 管理员用户 + 权限授予
func (m *Manager) createTenantWithAdmin(ctx context.Context, opts *createOpts) (*CreatedTenant, error) {
	if opts.Config == "" {
		opts.Config = "{}"
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(opts.AdminPassword), bcrypt.DefaultCost)
	if err != nil {
		return nil, fmt.Errorf("密码加密失败: %w", err)
	}

	var (
		createdTenant *model.Tenant
		createdUser   *model.User
	)

	err = m.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// 1. 租户唯一性
		var cnt int64
		if err := tx.Model(&model.Tenant{}).Where("code = ?", opts.Code).Count(&cnt).Error; err != nil {
			return err
		}
		if cnt > 0 {
			return fmt.Errorf("租户编码 %q 已存在", opts.Code)
		}

		// 2. 创建租户
		parentID := opts.ParentID
		t := &model.Tenant{
			Name:     opts.Name,
			Code:     opts.Code,
			Level:    opts.Level,
			ParentID: &parentID,
			Status:   model.StatusEnabled,
			Config:   opts.Config,
		}
		if err := tx.Create(t).Error; err != nil {
			return fmt.Errorf("创建租户失败: %w", err)
		}
		createdTenant = t

		// 3. 创建管理员角色
		role := &model.Role{
			TenantID:    t.ID,
			Name:        opts.RoleName,
			Code:        opts.RoleCode,
			Level:       opts.Level,
			Permissions: `["*"]`,
			Status:      model.StatusEnabled,
		}
		if err := tx.Create(role).Error; err != nil {
			return fmt.Errorf("创建角色失败: %w", err)
		}

		// 4. 创建管理员账号（用户名在租户内唯一）
		var uCnt int64
		if err := tx.Model(&model.User{}).
			Where("tenant_id = ? AND username = ?", t.ID, opts.AdminUsername).
			Count(&uCnt).Error; err != nil {
			return err
		}
		if uCnt > 0 {
			return fmt.Errorf("用户名 %q 已存在", opts.AdminUsername)
		}

		u := &model.User{
			TenantID: t.ID,
			Username: opts.AdminUsername,
			Password: string(hash),
			Nickname: opts.AdminNickname,
			Email:    opts.AdminEmail,
			Phone:    opts.AdminPhone,
			RoleID:   role.ID,
			Status:   model.StatusEnabled,
		}
		if err := tx.Create(u).Error; err != nil {
			return fmt.Errorf("创建管理员失败: %w", err)
		}
		createdUser = u

		// 5. 授予该层级及以下的默认权限
		// 逻辑：service provider 拿到所有 provider + customer 权限
		//       customer 拿到所有 customer 权限
		allowedLevels := allowedPermLevels(opts.Level)
		if len(allowedLevels) > 0 {
			sql := `
INSERT INTO tenant_permissions (id, tenant_id, permission_code, granted_by, status, created_at, updated_at)
SELECT uuid_generate_v4(), ?, code, ?, 1, NOW(), NOW()
  FROM permissions
 WHERE level IN ? AND deleted_at IS NULL
ON CONFLICT (tenant_id, permission_code) DO NOTHING`
			if err := tx.Exec(sql, t.ID, opts.GrantedByID, allowedLevels).Error; err != nil {
				return fmt.Errorf("授予默认权限失败: %w", err)
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	createdUser.Password = "" // 响应不返回密码
	logger.WithContext(ctx).Info("租户与管理员创建成功",
		zap.String("tenant_id", createdTenant.ID),
		zap.String("tenant_code", createdTenant.Code),
		zap.String("level", string(createdTenant.Level)),
		zap.String("admin_username", createdUser.Username),
	)
	return &CreatedTenant{Tenant: createdTenant, Admin: createdUser}, nil
}

// allowedPermLevels 返回该租户层级默认应被授予的权限层级列表
func allowedPermLevels(level model.TenantLevel) []string {
	switch level {
	case model.LevelDeveloper:
		return []string{"developer", "provider", "customer"}
	case model.LevelProvider:
		return []string{"provider", "customer"}
	case model.LevelCustomer:
		return []string{"customer"}
	}
	return nil
}

func firstNonEmpty(s ...string) string {
	for _, v := range s {
		if v != "" {
			return v
		}
	}
	return ""
}
