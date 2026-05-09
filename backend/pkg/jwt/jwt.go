// Package jwt JWT双令牌鉴权组件
// 核心能力：
//   - AccessToken（短时效，默认2小时）：用于接口鉴权
//   - RefreshToken（长时效，默认7天）：用于刷新AccessToken
//   - 智能续签：AccessToken临近过期时自动签发新令牌
//   - Token撤销：通过Redis黑名单实现即时失效
package jwt

import (
	"context"
	"errors"
	"fmt"
	"time"

	jwtv5 "github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

// TokenType 令牌类型
type TokenType string

const (
	TokenTypeAccess  TokenType = "access"  // 访问令牌
	TokenTypeRefresh TokenType = "refresh" // 刷新令牌
)

// 常见错误
var (
	ErrTokenInvalid      = errors.New("令牌无效")
	ErrTokenExpired      = errors.New("令牌已过期")
	ErrTokenRevoked      = errors.New("令牌已撤销")
	ErrTokenTypeMismatch = errors.New("令牌类型不匹配")
	ErrRefreshRequired   = errors.New("需要使用刷新令牌")
)

// Claims JWT 载荷
type Claims struct {
	UserID   string    `json:"uid"`   // 用户ID
	TenantID string    `json:"tid"`   // 租户ID
	Level    string    `json:"lvl"`   // 层级 developer/provider/customer
	Role     string    `json:"role"`  // 角色编码
	Username string    `json:"uname"` // 用户名
	Type     TokenType `json:"typ"`   // 令牌类型
	JTI      string    `json:"jti"`   // 令牌唯一ID（用于撤销）
	jwtv5.RegisteredClaims
}

// TokenPair 令牌对
type TokenPair struct {
	AccessToken      string    `json:"access_token"`
	RefreshToken     string    `json:"refresh_token"`
	TokenType        string    `json:"token_type"`         // 固定 Bearer
	AccessExpiresAt  time.Time `json:"access_expires_at"`  // Access过期时间
	RefreshExpiresAt time.Time `json:"refresh_expires_at"` // Refresh过期时间
	ExpiresIn        int64     `json:"expires_in"`         // Access剩余秒数
}

// Config JWT配置
type Config struct {
	Secret             string        `mapstructure:"secret" yaml:"secret"`
	Issuer             string        `mapstructure:"issuer" yaml:"issuer"`
	AccessTTL          time.Duration `mapstructure:"access_ttl" yaml:"access_ttl"`                     // AccessToken有效期
	RefreshTTL         time.Duration `mapstructure:"refresh_ttl" yaml:"refresh_ttl"`                   // RefreshToken有效期
	AutoRenewThreshold time.Duration `mapstructure:"auto_renew_threshold" yaml:"auto_renew_threshold"` // 智能续签阈值（剩余时间小于此值时触发）
}

// DefaultConfig 默认配置
func DefaultConfig() *Config {
	return &Config{
		Secret:             "mu-default-jwt-secret-please-change-in-production",
		Issuer:             "mu-framework",
		AccessTTL:          2 * time.Hour,
		RefreshTTL:         7 * 24 * time.Hour,
		AutoRenewThreshold: 15 * time.Minute,
	}
}

// Manager JWT 管理器
type Manager struct {
	cfg   *Config
	redis redis.UniversalClient // 用于黑名单和刷新令牌存储，可为nil；兼容单机/集群/哨兵
}

// NewManager 创建JWT管理器
// rdb 可为 nil（此时不启用令牌撤销黑名单）
func NewManager(cfg *Config, rdb redis.UniversalClient) *Manager {
	if cfg == nil {
		cfg = DefaultConfig()
	}
	return &Manager{cfg: cfg, redis: rdb}
}

// UserInfo 签发令牌所需的用户信息
type UserInfo struct {
	UserID   string
	TenantID string
	Level    string
	Role     string
	Username string
}

// GenerateTokenPair 签发令牌对（AccessToken + RefreshToken）
func (m *Manager) GenerateTokenPair(ctx context.Context, user *UserInfo) (*TokenPair, error) {
	now := time.Now()
	accessExpire := now.Add(m.cfg.AccessTTL)
	refreshExpire := now.Add(m.cfg.RefreshTTL)

	accessJTI := uuid.New().String()
	refreshJTI := uuid.New().String()

	accessToken, err := m.sign(user, TokenTypeAccess, accessJTI, now, accessExpire)
	if err != nil {
		return nil, fmt.Errorf("签发AccessToken失败: %w", err)
	}

	refreshToken, err := m.sign(user, TokenTypeRefresh, refreshJTI, now, refreshExpire)
	if err != nil {
		return nil, fmt.Errorf("签发RefreshToken失败: %w", err)
	}

	// 将 RefreshToken 绑定关系写入 Redis（可选）
	if m.redis != nil {
		key := fmt.Sprintf("jwt:refresh:%s", refreshJTI)
		_ = m.redis.Set(ctx, key, user.UserID, m.cfg.RefreshTTL).Err()
	}

	return &TokenPair{
		AccessToken:      accessToken,
		RefreshToken:     refreshToken,
		TokenType:        "Bearer",
		AccessExpiresAt:  accessExpire,
		RefreshExpiresAt: refreshExpire,
		ExpiresIn:        int64(m.cfg.AccessTTL.Seconds()),
	}, nil
}

// sign 签发单个令牌
func (m *Manager) sign(user *UserInfo, tokenType TokenType, jti string, issuedAt, expiresAt time.Time) (string, error) {
	claims := &Claims{
		UserID:   user.UserID,
		TenantID: user.TenantID,
		Level:    user.Level,
		Role:     user.Role,
		Username: user.Username,
		Type:     tokenType,
		JTI:      jti,
		RegisteredClaims: jwtv5.RegisteredClaims{
			Issuer:    m.cfg.Issuer,
			Subject:   user.UserID,
			IssuedAt:  jwtv5.NewNumericDate(issuedAt),
			NotBefore: jwtv5.NewNumericDate(issuedAt),
			ExpiresAt: jwtv5.NewNumericDate(expiresAt),
			ID:        jti,
		},
	}

	token := jwtv5.NewWithClaims(jwtv5.SigningMethodHS256, claims)
	return token.SignedString([]byte(m.cfg.Secret))
}

// Parse 解析并验证令牌
func (m *Manager) Parse(ctx context.Context, tokenStr string) (*Claims, error) {
	token, err := jwtv5.ParseWithClaims(tokenStr, &Claims{}, func(t *jwtv5.Token) (interface{}, error) {
		if _, ok := t.Method.(*jwtv5.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("非预期的签名方法: %v", t.Header["alg"])
		}
		return []byte(m.cfg.Secret), nil
	})

	if err != nil {
		switch {
		case errors.Is(err, jwtv5.ErrTokenExpired):
			return nil, ErrTokenExpired
		case errors.Is(err, jwtv5.ErrTokenNotValidYet), errors.Is(err, jwtv5.ErrTokenMalformed):
			return nil, ErrTokenInvalid
		default:
			return nil, fmt.Errorf("%w: %v", ErrTokenInvalid, err)
		}
	}

	claims, ok := token.Claims.(*Claims)
	if !ok || !token.Valid {
		return nil, ErrTokenInvalid
	}

	// 检查是否已撤销（黑名单）
	if m.isRevoked(ctx, claims.JTI) {
		return nil, ErrTokenRevoked
	}

	return claims, nil
}

// ParseAccessToken 解析 AccessToken（校验类型）
func (m *Manager) ParseAccessToken(ctx context.Context, tokenStr string) (*Claims, error) {
	claims, err := m.Parse(ctx, tokenStr)
	if err != nil {
		return nil, err
	}
	if claims.Type != TokenTypeAccess {
		return nil, ErrTokenTypeMismatch
	}
	return claims, nil
}

// Refresh 使用 RefreshToken 刷新，返回新的令牌对
func (m *Manager) Refresh(ctx context.Context, refreshToken string) (*TokenPair, error) {
	claims, err := m.Parse(ctx, refreshToken)
	if err != nil {
		return nil, err
	}

	if claims.Type != TokenTypeRefresh {
		return nil, ErrRefreshRequired
	}

	// 刷新成功后将旧 RefreshToken 加入黑名单（一次性使用，增强安全）
	if m.redis != nil {
		remain := time.Until(claims.ExpiresAt.Time)
		if remain > 0 {
			m.revoke(ctx, claims.JTI, remain)
		}
	}

	return m.GenerateTokenPair(ctx, &UserInfo{
		UserID:   claims.UserID,
		TenantID: claims.TenantID,
		Level:    claims.Level,
		Role:     claims.Role,
		Username: claims.Username,
	})
}

// ShouldRenew 判断 AccessToken 是否需要自动续签
// 返回 true 表示当前 AccessToken 剩余时间低于阈值，客户端应发起刷新
func (m *Manager) ShouldRenew(claims *Claims) bool {
	if claims == nil || claims.ExpiresAt == nil {
		return false
	}
	remain := time.Until(claims.ExpiresAt.Time)
	return remain > 0 && remain < m.cfg.AutoRenewThreshold
}

// AutoRenewIfNeeded 智能续签：若 Access 接近过期则签发新令牌对，否则返回 nil
// 业务中间件可据此在响应头中追加新令牌
func (m *Manager) AutoRenewIfNeeded(ctx context.Context, claims *Claims) (*TokenPair, error) {
	if !m.ShouldRenew(claims) {
		return nil, nil
	}
	return m.GenerateTokenPair(ctx, &UserInfo{
		UserID:   claims.UserID,
		TenantID: claims.TenantID,
		Level:    claims.Level,
		Role:     claims.Role,
		Username: claims.Username,
	})
}

// Revoke 撤销令牌（加入黑名单直至其原定过期时间）
func (m *Manager) Revoke(ctx context.Context, claims *Claims) error {
	if m.redis == nil || claims == nil {
		return nil
	}
	remain := time.Until(claims.ExpiresAt.Time)
	if remain <= 0 {
		return nil
	}
	return m.revoke(ctx, claims.JTI, remain)
}

func (m *Manager) revoke(ctx context.Context, jti string, ttl time.Duration) error {
	key := fmt.Sprintf("jwt:blacklist:%s", jti)
	return m.redis.Set(ctx, key, "1", ttl).Err()
}

func (m *Manager) isRevoked(ctx context.Context, jti string) bool {
	if m.redis == nil || jti == "" {
		return false
	}
	key := fmt.Sprintf("jwt:blacklist:%s", jti)
	n, err := m.redis.Exists(ctx, key).Result()
	if err != nil {
		return false // 缓存异常时放行，避免影响业务
	}
	return n > 0
}

// Config 返回管理器配置（只读）
func (m *Manager) Config() *Config {
	return m.cfg
}
