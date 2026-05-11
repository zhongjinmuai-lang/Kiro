// Package auth 认证与账户域：登录 / 注册 / 刷新 / 登出 / 改密
package auth

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"go.uber.org/zap"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"

	"github.com/zhongjinmuai-lang/mu-framework/internal/model"
	"github.com/zhongjinmuai-lang/mu-framework/pkg/cache"
	"github.com/zhongjinmuai-lang/mu-framework/pkg/jwt"
	"github.com/zhongjinmuai-lang/mu-framework/pkg/logger"
)

// 常见错误
var (
	ErrUserNotFound     = errors.New("用户不存在")
	ErrInvalidPassword  = errors.New("用户名或密码错误")
	ErrUserDisabled     = errors.New("用户已被禁用")
	ErrTenantNotFound   = errors.New("租户不存在")
	ErrTenantDisabled   = errors.New("租户已被禁用")
	ErrPasswordMismatch = errors.New("旧密码不正确")
	ErrUsernameTaken    = errors.New("用户名已存在")
)

// Service 认证服务
type Service struct {
	db  *gorm.DB
	rdb *cache.Client
	jwt *jwt.Manager
}

// NewService 构造
func NewService(db *gorm.DB, rdb *cache.Client, mgr *jwt.Manager) *Service {
	return &Service{db: db, rdb: rdb, jwt: mgr}
}

// LoginInput 登录入参
type LoginInput struct {
	TenantCode string `json:"tenant_code" binding:"required,max=50"`
	Username   string `json:"username"    binding:"required,max=50"`
	Password   string `json:"password"    binding:"required,min=6,max=64"`
}

// RegisterInput 注册入参（仅管理员调用）
type RegisterInput struct {
	TenantID string `json:"tenant_id" binding:"required"`
	Username string `json:"username"  binding:"required,min=3,max=50"`
	Password string `json:"password"  binding:"required,min=6,max=64"`
	Nickname string `json:"nickname"  binding:"max=100"`
	Email    string `json:"email"     binding:"omitempty,email,max=100"`
	Phone    string `json:"phone"     binding:"omitempty,max=20"`
	RoleID   string `json:"role_id"`
}

// ChangePasswordInput 改密入参
type ChangePasswordInput struct {
	OldPassword string `json:"old_password" binding:"required"`
	NewPassword string `json:"new_password" binding:"required,min=6,max=64"`
}

// Login 登录：校验租户+用户+密码 → 签发双令牌
func (s *Service) Login(ctx context.Context, in *LoginInput) (*jwt.TokenPair, *model.User, error) {
	var tenant model.Tenant
	if err := s.db.WithContext(ctx).First(&tenant, "code = ?", in.TenantCode).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil, ErrTenantNotFound
		}
		return nil, nil, fmt.Errorf("查询租户失败: %w", err)
	}
	if tenant.Status != model.StatusEnabled {
		return nil, nil, ErrTenantDisabled
	}

	var user model.User
	if err := s.db.WithContext(ctx).
		First(&user, "tenant_id = ? AND username = ?", tenant.ID, in.Username).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil, ErrUserNotFound
		}
		return nil, nil, fmt.Errorf("查询用户失败: %w", err)
	}
	if user.Status != model.StatusEnabled {
		return nil, nil, ErrUserDisabled
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(in.Password)); err != nil {
		return nil, nil, ErrInvalidPassword
	}

	roleCode := ""
	if user.RoleID != "" {
		var r model.Role
		if err := s.db.WithContext(ctx).Select("code").
			First(&r, "id = ?", user.RoleID).Error; err == nil {
			roleCode = r.Code
		}
	}

	pair, err := s.jwt.GenerateTokenPair(ctx, &jwt.UserInfo{
		UserID:   user.ID,
		TenantID: tenant.ID,
		Level:    string(tenant.Level),
		Role:     roleCode,
		Username: user.Username,
	})
	if err != nil {
		return nil, nil, err
	}

	logger.WithContext(ctx).Info("用户登录成功",
		zap.String("user_id", user.ID),
		zap.String("tenant_id", tenant.ID),
		zap.String("level", string(tenant.Level)),
	)
	return pair, &user, nil
}

// Refresh 刷新令牌（一次性）
func (s *Service) Refresh(ctx context.Context, refreshToken string) (*jwt.TokenPair, error) {
	return s.jwt.Refresh(ctx, refreshToken)
}

// Logout 登出：双令牌加入黑名单
func (s *Service) Logout(ctx context.Context, accessToken, refreshToken string) error {
	if accessToken != "" {
		if c, err := s.jwt.Parse(ctx, accessToken); err == nil {
			_ = s.jwt.Revoke(ctx, c)
		}
	}
	if refreshToken != "" {
		if c, err := s.jwt.Parse(ctx, refreshToken); err == nil {
			_ = s.jwt.Revoke(ctx, c)
		}
	}
	return nil
}

// Register 创建用户（管理员用）
// 使用数据库唯一约束保证用户名唯一性（避免 Count+Create 的竞态窗口）
func (s *Service) Register(ctx context.Context, in *RegisterInput) (*model.User, error) {
	var t model.Tenant
	if err := s.db.WithContext(ctx).First(&t, "id = ?", in.TenantID).Error; err != nil {
		return nil, ErrTenantNotFound
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(in.Password), bcrypt.DefaultCost)
	if err != nil {
		return nil, fmt.Errorf("密码加密失败: %w", err)
	}
	u := &model.User{
		TenantID: in.TenantID,
		Username: in.Username,
		Password: string(hash),
		Nickname: in.Nickname,
		Email:    in.Email,
		Phone:    in.Phone,
		RoleID:   in.RoleID,
		Status:   model.StatusEnabled,
	}
	if err := s.db.WithContext(ctx).Create(u).Error; err != nil {
		// 捕获唯一约束冲突（PostgreSQL error code 23505）
		if isDuplicateKeyError(err) {
			return nil, ErrUsernameTaken
		}
		return nil, fmt.Errorf("创建用户失败: %w", err)
	}
	u.Password = ""
	return u, nil
}

// isDuplicateKeyError 判断是否为唯一约束冲突（PostgreSQL 23505）
func isDuplicateKeyError(err error) bool {
	if err == nil {
		return false
	}
	errStr := err.Error()
	// PostgreSQL 唯一约束违反：ERROR: duplicate key value violates unique constraint
	return strings.Contains(errStr, "duplicate key") || strings.Contains(errStr, "23505")
}

// ChangePassword 修改密码
func (s *Service) ChangePassword(ctx context.Context, userID string, in *ChangePasswordInput) error {
	var u model.User
	if err := s.db.WithContext(ctx).First(&u, "id = ?", userID).Error; err != nil {
		return ErrUserNotFound
	}
	if err := bcrypt.CompareHashAndPassword([]byte(u.Password), []byte(in.OldPassword)); err != nil {
		return ErrPasswordMismatch
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(in.NewPassword), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	return s.db.WithContext(ctx).Model(&model.User{}).
		Where("id = ?", userID).
		Updates(map[string]any{"password": string(hash), "updated_at": time.Now()}).Error
}

// Profile 获取当前用户信息
func (s *Service) Profile(ctx context.Context, userID string) (*model.User, error) {
	var u model.User
	if err := s.db.WithContext(ctx).First(&u, "id = ?", userID).Error; err != nil {
		return nil, ErrUserNotFound
	}
	u.Password = ""
	return &u, nil
}
