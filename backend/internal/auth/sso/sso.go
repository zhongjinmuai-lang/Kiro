// Package sso MU 单点登录模块（v2.7）
//
// 支持多种登录方式：
//   - 手机验证码登录（对接阿里云/腾讯云 SMS）
//   - 微信小程序登录（code2session → unionid 绑定）
//   - 微信公众号/开放平台 OAuth2.0
//   - 密码登录（已有，此处集成统一入口）
//
// 设计原则：
//   - 统一 UserIdentity 表关联多种登录方式到同一用户
//   - 首次登录自动注册（按租户隔离）
//   - 与现有 JWT 双令牌体系完全兼容
package sso

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"time"

	"go.uber.org/zap"
	"gorm.io/gorm"

	"github.com/zhongjinmuai-lang/mu-framework/internal/model"
	"github.com/zhongjinmuai-lang/mu-framework/pkg/cache"
	"github.com/zhongjinmuai-lang/mu-framework/pkg/jwt"
	"github.com/zhongjinmuai-lang/mu-framework/pkg/logger"
)

// IdentityProvider 身份提供方
type IdentityProvider string

const (
	ProviderPhone    IdentityProvider = "phone"     // 手机验证码
	ProviderWechatMP IdentityProvider = "wechat_mp" // 微信小程序
	ProviderWechatOA IdentityProvider = "wechat_oa" // 微信公众号/开放平台
	ProviderPassword IdentityProvider = "password"  // 密码（兼容已有）
)

// UserIdentity 用户身份绑定表（一个用户可绑定多种登录方式）
type UserIdentity struct {
	model.BaseModel
	TenantID   string           `gorm:"column:tenant_id;type:uuid;not null;index:idx_identity_lookup,priority:1" json:"tenant_id"`
	UserID     string           `gorm:"column:user_id;type:uuid;not null;index" json:"user_id"`
	Provider   IdentityProvider `gorm:"column:provider;type:varchar(20);not null;index:idx_identity_lookup,priority:2" json:"provider"`
	ExternalID string           `gorm:"column:external_id;type:varchar(200);not null;index:idx_identity_lookup,priority:3" json:"external_id"` // 手机号/openid/unionid
	Nickname   string           `gorm:"column:nickname;type:varchar(100)" json:"nickname"`
	Avatar     string           `gorm:"column:avatar;type:varchar(500)" json:"avatar"`
	Extra      string           `gorm:"column:extra;type:jsonb;default:'{}'" json:"extra"` // 扩展信息
}

func (UserIdentity) TableName() string { return "user_identities" }

// Config SSO 配置
type Config struct {
	// 微信小程序
	WechatMPAppID  string `mapstructure:"wechat_mp_app_id" yaml:"wechat_mp_app_id"`
	WechatMPSecret string `mapstructure:"wechat_mp_secret" yaml:"wechat_mp_secret"`
	// 微信公众号/开放平台
	WechatOAAppID  string `mapstructure:"wechat_oa_app_id" yaml:"wechat_oa_app_id"`
	WechatOASecret string `mapstructure:"wechat_oa_secret" yaml:"wechat_oa_secret"`
	// 验证码
	CodeLength int           `mapstructure:"code_length" yaml:"code_length"` // 验证码位数，默认6
	CodeTTL    time.Duration `mapstructure:"code_ttl" yaml:"code_ttl"`       // 验证码有效期，默认5分钟
}

// DefaultConfig 默认配置
func DefaultConfig() *Config {
	return &Config{
		CodeLength: 6,
		CodeTTL:    5 * time.Minute,
	}
}

// Service SSO 服务
type Service struct {
	db     *gorm.DB
	rdb    *cache.Client
	jwt    *jwt.Manager
	cfg    *Config
	http   *http.Client
}

// NewService 创建 SSO 服务
func NewService(db *gorm.DB, rdb *cache.Client, jwtMgr *jwt.Manager, cfg *Config) *Service {
	if cfg == nil {
		cfg = DefaultConfig()
	}
	if cfg.CodeLength <= 0 {
		cfg.CodeLength = 6
	}
	if cfg.CodeTTL <= 0 {
		cfg.CodeTTL = 5 * time.Minute
	}
	return &Service{
		db:   db,
		rdb:  rdb,
		jwt:  jwtMgr,
		cfg:  cfg,
		http: &http.Client{Timeout: 10 * time.Second},
	}
}

// ========== 手机验证码登录 ==========

// SendCodeInput 发送验证码入参
type SendCodeInput struct {
	TenantCode string `json:"tenant_code" binding:"required"`
	Phone      string `json:"phone" binding:"required,min=11,max=15"`
}

// SendCode 发送手机验证码（存入 Redis，TTL 5分钟）
func (s *Service) SendCode(ctx context.Context, in *SendCodeInput) error {
	// 频率限制：60 秒内不能重复发送
	lockKey := fmt.Sprintf("sso:code_lock:%s:%s", in.TenantCode, in.Phone)
	if s.rdb != nil {
		if exists, _ := s.rdb.Exists(ctx, lockKey).Result(); exists > 0 {
			return errors.New("验证码发送过于频繁，请60秒后重试")
		}
	}

	// 生成验证码
	code := generateCode(s.cfg.CodeLength)

	// 存入 Redis
	codeKey := fmt.Sprintf("sso:code:%s:%s", in.TenantCode, in.Phone)
	if s.rdb != nil {
		s.rdb.Set(ctx, codeKey, code, s.cfg.CodeTTL)
		s.rdb.Set(ctx, lockKey, "1", 60*time.Second) // 60秒锁
	}

	// TODO: 调用通知中台的 SMS 发送（当前仅日志输出，生产环境接入真实 SMS）
	logger.L().Info("验证码已生成（开发环境输出）",
		zap.String("phone", in.Phone),
		zap.String("code", code),
		zap.Duration("ttl", s.cfg.CodeTTL),
	)
	return nil
}

// PhoneLoginInput 手机验证码登录入参
type PhoneLoginInput struct {
	TenantCode string `json:"tenant_code" binding:"required"`
	Phone      string `json:"phone" binding:"required"`
	Code       string `json:"code" binding:"required"`
}

// PhoneLogin 手机验证码登录（自动注册）
func (s *Service) PhoneLogin(ctx context.Context, in *PhoneLoginInput) (*jwt.TokenPair, *model.User, error) {
	// 1. 验证码校验
	codeKey := fmt.Sprintf("sso:code:%s:%s", in.TenantCode, in.Phone)
	if s.rdb != nil {
		stored, err := s.rdb.Get(ctx, codeKey).Result()
		if err != nil || stored != in.Code {
			return nil, nil, errors.New("验证码错误或已过期")
		}
		// 使用后删除
		s.rdb.Del(ctx, codeKey)
	}

	// 2. 查租户
	var tenant model.Tenant
	if err := s.db.WithContext(ctx).First(&tenant, "code = ? AND status = 1", in.TenantCode).Error; err != nil {
		return nil, nil, errors.New("租户不存在或已禁用")
	}

	// 3. 查找或创建用户
	user, err := s.findOrCreateByIdentity(ctx, tenant.ID, ProviderPhone, in.Phone, in.Phone, "")
	if err != nil {
		return nil, nil, err
	}

	// 4. 签发令牌
	pair, err := s.jwt.GenerateTokenPair(ctx, &jwt.UserInfo{
		UserID:   user.ID,
		TenantID: tenant.ID,
		Level:    string(tenant.Level),
		Username: user.Username,
	})
	if err != nil {
		return nil, nil, err
	}

	logger.WithContext(ctx).Info("手机验证码登录成功",
		zap.String("phone", in.Phone),
		zap.String("user_id", user.ID),
	)
	return pair, user, nil
}

// ========== 微信小程序登录 ==========

// WechatMPLoginInput 微信小程序登录入参
type WechatMPLoginInput struct {
	TenantCode string `json:"tenant_code" binding:"required"`
	Code       string `json:"code" binding:"required"` // wx.login() 获取的 code
	Nickname   string `json:"nickname"`
	Avatar     string `json:"avatar"`
}

// wechatCode2SessionResp 微信 code2session 响应
type wechatCode2SessionResp struct {
	OpenID     string `json:"openid"`
	SessionKey string `json:"session_key"`
	UnionID    string `json:"unionid"`
	ErrCode    int    `json:"errcode"`
	ErrMsg     string `json:"errmsg"`
}

// WechatMPLogin 微信小程序登录
func (s *Service) WechatMPLogin(ctx context.Context, in *WechatMPLoginInput) (*jwt.TokenPair, *model.User, error) {
	if s.cfg.WechatMPAppID == "" || s.cfg.WechatMPSecret == "" {
		return nil, nil, errors.New("微信小程序未配置 AppID/Secret")
	}

	// 1. 调用微信 code2session
	url := fmt.Sprintf(
		"https://api.weixin.qq.com/sns/jscode2session?appid=%s&secret=%s&js_code=%s&grant_type=authorization_code",
		s.cfg.WechatMPAppID, s.cfg.WechatMPSecret, in.Code,
	)
	resp, err := s.http.Get(url)
	if err != nil {
		return nil, nil, fmt.Errorf("微信接口调用失败: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	var wxResp wechatCode2SessionResp
	if err := json.Unmarshal(body, &wxResp); err != nil {
		return nil, nil, fmt.Errorf("解析微信响应失败: %w", err)
	}
	if wxResp.ErrCode != 0 {
		return nil, nil, fmt.Errorf("微信登录失败: %s (code=%d)", wxResp.ErrMsg, wxResp.ErrCode)
	}
	if wxResp.OpenID == "" {
		return nil, nil, errors.New("微信返回 openid 为空")
	}

	// 2. 查租户
	var tenant model.Tenant
	if err := s.db.WithContext(ctx).First(&tenant, "code = ? AND status = 1", in.TenantCode).Error; err != nil {
		return nil, nil, errors.New("租户不存在或已禁用")
	}

	// 3. 使用 openid 查找或创建用户
	externalID := wxResp.OpenID
	if wxResp.UnionID != "" {
		externalID = wxResp.UnionID // 优先使用 UnionID
	}
	user, err := s.findOrCreateByIdentity(ctx, tenant.ID, ProviderWechatMP, externalID, in.Nickname, in.Avatar)
	if err != nil {
		return nil, nil, err
	}

	// 4. 签发令牌
	pair, err := s.jwt.GenerateTokenPair(ctx, &jwt.UserInfo{
		UserID:   user.ID,
		TenantID: tenant.ID,
		Level:    string(tenant.Level),
		Username: user.Username,
	})
	if err != nil {
		return nil, nil, err
	}

	logger.WithContext(ctx).Info("微信小程序登录成功",
		zap.String("openid", wxResp.OpenID),
		zap.String("user_id", user.ID),
	)
	return pair, user, nil
}

// ========== 微信公众号/开放平台 OAuth2.0 ==========

// WechatOAuthInput 微信OAuth登录入参
type WechatOAuthInput struct {
	TenantCode string `json:"tenant_code" binding:"required"`
	Code       string `json:"code" binding:"required"` // OAuth 授权码
}

// wechatOAuthTokenResp 微信 OAuth access_token 响应
type wechatOAuthTokenResp struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	OpenID       string `json:"openid"`
	UnionID      string `json:"unionid"`
	Scope        string `json:"scope"`
	ErrCode      int    `json:"errcode"`
	ErrMsg       string `json:"errmsg"`
}

// wechatUserInfoResp 微信用户信息响应
type wechatUserInfoResp struct {
	OpenID   string `json:"openid"`
	UnionID  string `json:"unionid"`
	Nickname string `json:"nickname"`
	Avatar   string `json:"headimgurl"`
	Sex      int    `json:"sex"`
	ErrCode  int    `json:"errcode"`
}

// WechatOAuthLogin 微信公众号/开放平台 OAuth 登录
func (s *Service) WechatOAuthLogin(ctx context.Context, in *WechatOAuthInput) (*jwt.TokenPair, *model.User, error) {
	if s.cfg.WechatOAAppID == "" || s.cfg.WechatOASecret == "" {
		return nil, nil, errors.New("微信 OAuth 未配置 AppID/Secret")
	}

	// 1. 用 code 换 access_token
	tokenURL := fmt.Sprintf(
		"https://api.weixin.qq.com/sns/oauth2/access_token?appid=%s&secret=%s&code=%s&grant_type=authorization_code",
		s.cfg.WechatOAAppID, s.cfg.WechatOASecret, in.Code,
	)
	tokenResp, err := s.httpGet(ctx, tokenURL)
	if err != nil {
		return nil, nil, err
	}
	var oauthToken wechatOAuthTokenResp
	if err := json.Unmarshal(tokenResp, &oauthToken); err != nil {
		return nil, nil, fmt.Errorf("解析微信 OAuth 响应失败: %w", err)
	}
	if oauthToken.ErrCode != 0 || oauthToken.OpenID == "" {
		return nil, nil, fmt.Errorf("微信 OAuth 失败: %s", oauthToken.ErrMsg)
	}

	// 2. 获取用户信息
	nickname, avatar := "", ""
	if oauthToken.Scope == "snsapi_userinfo" {
		infoURL := fmt.Sprintf(
			"https://api.weixin.qq.com/sns/userinfo?access_token=%s&openid=%s&lang=zh_CN",
			oauthToken.AccessToken, oauthToken.OpenID,
		)
		infoBody, err := s.httpGet(ctx, infoURL)
		if err == nil {
			var info wechatUserInfoResp
			if json.Unmarshal(infoBody, &info) == nil && info.ErrCode == 0 {
				nickname = info.Nickname
				avatar = info.Avatar
			}
		}
	}

	// 3. 查租户
	var tenant model.Tenant
	if err := s.db.WithContext(ctx).First(&tenant, "code = ? AND status = 1", in.TenantCode).Error; err != nil {
		return nil, nil, errors.New("租户不存在或已禁用")
	}

	// 4. 查找或创建用户
	externalID := oauthToken.OpenID
	if oauthToken.UnionID != "" {
		externalID = oauthToken.UnionID
	}
	user, err := s.findOrCreateByIdentity(ctx, tenant.ID, ProviderWechatOA, externalID, nickname, avatar)
	if err != nil {
		return nil, nil, err
	}

	// 5. 签发令牌
	pair, err := s.jwt.GenerateTokenPair(ctx, &jwt.UserInfo{
		UserID:   user.ID,
		TenantID: tenant.ID,
		Level:    string(tenant.Level),
		Username: user.Username,
	})
	if err != nil {
		return nil, nil, err
	}

	logger.WithContext(ctx).Info("微信OAuth登录成功",
		zap.String("openid", oauthToken.OpenID),
		zap.String("user_id", user.ID),
	)
	return pair, user, nil
}

// ========== 绑定/解绑 ==========

// BindIdentity 绑定新的登录方式到已有用户
func (s *Service) BindIdentity(ctx context.Context, userID, tenantID string, provider IdentityProvider, externalID string) error {
	// 检查是否已被其他用户绑定
	var existing UserIdentity
	err := s.db.WithContext(ctx).
		First(&existing, "tenant_id = ? AND provider = ? AND external_id = ?", tenantID, provider, externalID).Error
	if err == nil && existing.UserID != userID {
		return errors.New("该身份已绑定到其他账户")
	}
	if err == nil {
		return nil // 已绑定到当前用户
	}

	identity := &UserIdentity{
		TenantID:   tenantID,
		UserID:     userID,
		Provider:   provider,
		ExternalID: externalID,
	}
	return s.db.WithContext(ctx).Create(identity).Error
}

// UnbindIdentity 解绑登录方式
func (s *Service) UnbindIdentity(ctx context.Context, userID, tenantID string, provider IdentityProvider) error {
	// 至少保留一种登录方式
	var count int64
	s.db.WithContext(ctx).Model(&UserIdentity{}).
		Where("user_id = ? AND tenant_id = ?", userID, tenantID).
		Count(&count)
	if count <= 1 {
		return errors.New("至少保留一种登录方式")
	}
	return s.db.WithContext(ctx).
		Where("user_id = ? AND tenant_id = ? AND provider = ?", userID, tenantID, provider).
		Delete(&UserIdentity{}).Error
}

// GetUserIdentities 获取用户已绑定的登录方式
func (s *Service) GetUserIdentities(ctx context.Context, userID, tenantID string) ([]*UserIdentity, error) {
	var list []*UserIdentity
	if err := s.db.WithContext(ctx).
		Where("user_id = ? AND tenant_id = ?", userID, tenantID).
		Find(&list).Error; err != nil {
		return nil, err
	}
	return list, nil
}

// ========== 内部方法 ==========

// findOrCreateByIdentity 通过身份标识查找或自动创建用户
func (s *Service) findOrCreateByIdentity(ctx context.Context, tenantID string, provider IdentityProvider, externalID, nickname, avatar string) (*model.User, error) {
	// 查找已绑定的身份
	var identity UserIdentity
	err := s.db.WithContext(ctx).
		First(&identity, "tenant_id = ? AND provider = ? AND external_id = ?", tenantID, provider, externalID).Error

	if err == nil {
		// 已存在，获取用户
		var user model.User
		if err := s.db.WithContext(ctx).First(&user, "id = ?", identity.UserID).Error; err != nil {
			return nil, fmt.Errorf("用户不存在: %w", err)
		}
		user.Password = ""
		return &user, nil
	}

	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}

	// 自动注册新用户
	username := fmt.Sprintf("%s_%s", provider, externalID)
	if len(username) > 50 {
		username = username[:50]
	}
	if nickname == "" {
		nickname = string(provider) + "用户"
	}

	var user *model.User
	err = s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		u := &model.User{
			TenantID: tenantID,
			Username: username,
			Password: "", // SSO 用户无密码
			Nickname: nickname,
			Avatar:   avatar,
			Phone:    "",
			Status:   model.StatusEnabled,
		}
		if provider == ProviderPhone {
			u.Phone = externalID
			u.Username = externalID // 手机号作为用户名
		}
		if err := tx.Create(u).Error; err != nil {
			return fmt.Errorf("自动注册用户失败: %w", err)
		}

		// 创建身份绑定
		identity := &UserIdentity{
			TenantID:   tenantID,
			UserID:     u.ID,
			Provider:   provider,
			ExternalID: externalID,
			Nickname:   nickname,
			Avatar:     avatar,
		}
		if err := tx.Create(identity).Error; err != nil {
			return fmt.Errorf("创建身份绑定失败: %w", err)
		}

		user = u
		return nil
	})
	if err != nil {
		return nil, err
	}

	user.Password = ""
	logger.WithContext(ctx).Info("SSO 自动注册用户",
		zap.String("user_id", user.ID),
		zap.String("provider", string(provider)),
		zap.String("external_id", externalID),
	)
	return user, nil
}

func (s *Service) httpGet(ctx context.Context, url string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	resp, err := s.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	return io.ReadAll(resp.Body)
}

// generateCode 生成随机数字验证码
func generateCode(length int) string {
	code := make([]byte, length)
	for i := range code {
		n, _ := rand.Int(rand.Reader, big.NewInt(10))
		code[i] = byte('0' + n.Int64())
	}
	return string(code)
}
