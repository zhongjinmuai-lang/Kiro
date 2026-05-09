package middleware

import (
	"context"
	"log/slog"
	"net/http"
	"runtime/debug"
	"strings"
	"time"

	"github.com/zhongjinmuai-lang/mu-framework/internal/core/bootstrap"
)

// 上下文Key类型
type contextKey string

const (
	ContextKeyTenantID contextKey = "tenant_id"
	ContextKeyUserID   contextKey = "user_id"
	ContextKeyRole     contextKey = "role"
	ContextKeyLevel    contextKey = "level" // 层级：developer / provider / customer
)

// Chain 中间件链
func Chain(handler http.Handler, middlewares ...func(http.Handler) http.Handler) http.Handler {
	for i := len(middlewares) - 1; i >= 0; i-- {
		handler = middlewares[i](handler)
	}
	return handler
}

// Logger 请求日志中间件
func Logger(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		wrapped := &responseWriter{ResponseWriter: w, statusCode: 200}

		next.ServeHTTP(wrapped, r)

		slog.Info("HTTP请求",
			"method", r.Method,
			"path", r.URL.Path,
			"status", wrapped.statusCode,
			"duration", time.Since(start).String(),
			"ip", r.RemoteAddr,
		)
	})
}

// Recovery panic恢复中间件
func Recovery(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if err := recover(); err != nil {
				slog.Error("panic恢复",
					"error", err,
					"stack", string(debug.Stack()),
				)
				http.Error(w, `{"code":500,"message":"服务内部错误"}`, http.StatusInternalServerError)
			}
		}()
		next.ServeHTTP(wrapped, r)
	})
}

// CORS 跨域中间件
func CORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS, PATCH")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-Tenant-ID")
		w.Header().Set("Access-Control-Max-Age", "86400")

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}

		next.ServeHTTP(w, r)
	})
}

// Auth JWT认证中间件
func Auth(app *bootstrap.App) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			authHeader := r.Header.Get("Authorization")
			if authHeader == "" {
				http.Error(w, `{"code":401,"message":"未提供认证令牌"}`, http.StatusUnauthorized)
				return
			}

			token := strings.TrimPrefix(authHeader, "Bearer ")
			if token == authHeader {
				http.Error(w, `{"code":401,"message":"认证令牌格式错误"}`, http.StatusUnauthorized)
				return
			}

			// TODO: 验证JWT token并提取信息
			claims, err := validateToken(token, app.Config.JWT.Secret)
			if err != nil {
				http.Error(w, `{"code":401,"message":"认证令牌无效"}`, http.StatusUnauthorized)
				return
			}

			// 注入上下文
			ctx := r.Context()
			ctx = context.WithValue(ctx, ContextKeyUserID, claims.UserID)
			ctx = context.WithValue(ctx, ContextKeyTenantID, claims.TenantID)
			ctx = context.WithValue(ctx, ContextKeyRole, claims.Role)
			ctx = context.WithValue(ctx, ContextKeyLevel, claims.Level)

			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// RateLimit 限流中间件
func RateLimit(requestsPerSecond int) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// TODO: 基于Redis的分布式限流实现
			next.ServeHTTP(w, r)
		})
	}
}

// TenantIsolation 租户隔离中间件
func TenantIsolation(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tenantID := r.Context().Value(ContextKeyTenantID)
		if tenantID == nil || tenantID == "" {
			http.Error(w, `{"code":403,"message":"租户信息缺失"}`, http.StatusForbidden)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// responseWriter 包装ResponseWriter以获取状态码
type responseWriter struct {
	http.ResponseWriter
	statusCode int
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}

// TokenClaims JWT令牌声明
type TokenClaims struct {
	UserID   string
	TenantID string
	Role     string
	Level    string // developer / provider / customer
}

// validateToken 验证JWT令牌
func validateToken(tokenStr string, secret string) (*TokenClaims, error) {
	// TODO: 完整的JWT验证实现
	return &TokenClaims{}, nil
}
