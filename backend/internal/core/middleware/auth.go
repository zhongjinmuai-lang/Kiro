package middleware

import (
	"context"
	"errors"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/zhongjinmuai-lang/mu-framework/pkg/jwt"
	"github.com/zhongjinmuai-lang/mu-framework/pkg/logger"
	"github.com/zhongjinmuai-lang/mu-framework/pkg/response"
)

// Auth JWT 认证中间件
// - 校验 AccessToken 合法性
// - 注入用户信息到 Gin Context
// - 执行智能续签（临近过期时在响应头追加新令牌）
func Auth(mgr *jwt.Manager) gin.HandlerFunc {
	return func(c *gin.Context) {
		tokenStr := extractToken(c)
		if tokenStr == "" {
			response.Unauthorized(c, "未提供认证令牌")
			return
		}

		claims, err := mgr.ParseAccessToken(c.Request.Context(), tokenStr)
		if err != nil {
			switch {
			case errors.Is(err, jwt.ErrTokenExpired):
				response.Unauthorized(c, "令牌已过期，请刷新或重新登录")
			case errors.Is(err, jwt.ErrTokenRevoked):
				response.Unauthorized(c, "令牌已注销")
			case errors.Is(err, jwt.ErrTokenTypeMismatch):
				response.Unauthorized(c, "令牌类型错误，请使用 AccessToken")
			default:
				response.Unauthorized(c, "令牌无效")
			}
			return
		}

		// 注入用户上下文（供后续业务及日志使用）
		c.Set(CtxKeyClaims, claims)
		c.Set(CtxKeyUserID, claims.UserID)
		c.Set(CtxKeyTenantID, claims.TenantID)
		c.Set(CtxKeyLevel, claims.Level)
		c.Set(CtxKeyRole, claims.Role)
		c.Set(CtxKeyUsername, claims.Username)

		// 传递到 request.Context，便于下游日志/GORM携带用户信息
		ctx := c.Request.Context()
		ctx = context.WithValue(ctx, logger.CtxKeyUserID, claims.UserID)
		ctx = context.WithValue(ctx, logger.CtxKeyTenantID, claims.TenantID)
		c.Request = c.Request.WithContext(ctx)

		// 智能续签：AccessToken 即将过期时自动签发新令牌对
		if pair, err := mgr.AutoRenewIfNeeded(c.Request.Context(), claims); err == nil && pair != nil {
			c.Header("X-New-Access-Token", pair.AccessToken)
			c.Header("X-New-Refresh-Token", pair.RefreshToken)
		}

		c.Next()
	}
}

// extractToken 从 Header 中提取 Token
// 安全策略：仅从 Authorization Header 提取，禁止 URL Query 传递（防止日志/Referer 泄露）
func extractToken(c *gin.Context) string {
	h := c.GetHeader("Authorization")
	if h == "" {
		return ""
	}
	if strings.HasPrefix(h, "Bearer ") {
		return strings.TrimPrefix(h, "Bearer ")
	}
	return h
}

// RequireLevel 层级校验：仅允许指定层级的租户访问
// 用法：RequireLevel("developer")、RequireLevel("developer", "provider")
func RequireLevel(levels ...string) gin.HandlerFunc {
	allow := make(map[string]struct{}, len(levels))
	for _, l := range levels {
		allow[l] = struct{}{}
	}
	return func(c *gin.Context) {
		current, ok := c.Get(CtxKeyLevel)
		if !ok {
			response.Forbidden(c, "缺少层级信息")
			return
		}
		if _, ok := allow[current.(string)]; !ok {
			response.Forbidden(c, "权限不足，需要层级："+strings.Join(levels, "/"))
			return
		}
		c.Next()
	}
}

// RequirePermission 权限校验（基于权限编码 module:resource:action）
// checker：业务层权限检查函数，签名 (tenantID, permCode) (bool, error)
func RequirePermission(permCode string, checker func(ctx context.Context, tenantID, permCode string) (bool, error)) gin.HandlerFunc {
	return func(c *gin.Context) {
		tenantID, _ := c.Get(CtxKeyTenantID)
		tid, _ := tenantID.(string)
		if tid == "" {
			response.Forbidden(c, "缺少租户信息")
			return
		}
		ok, err := checker(c.Request.Context(), tid, permCode)
		if err != nil {
			response.InternalError(c, "权限校验失败: "+err.Error())
			return
		}
		if !ok {
			response.Forbidden(c, "缺少权限: "+permCode)
			return
		}
		c.Next()
	}
}

// TenantRequired 租户必填校验（保障下游业务有租户上下文）
func TenantRequired() gin.HandlerFunc {
	return func(c *gin.Context) {
		tid, ok := c.Get(CtxKeyTenantID)
		if !ok || tid == nil || tid == "" {
			response.Forbidden(c, "租户信息缺失")
			return
		}
		c.Next()
	}
}
