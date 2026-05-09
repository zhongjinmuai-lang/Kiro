package middleware

import (
	"fmt"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/zhongjinmuai-lang/mu-framework/pkg/cache"
	"github.com/zhongjinmuai-lang/mu-framework/pkg/response"
)

// RateLimit 基于 Redis 的分布式限流中间件（固定窗口）
// limit：窗口内最大请求数；window：窗口长度；keyFunc：限流维度（如按IP、按用户）
func RateLimit(rdb *cache.Client, limit int64, window time.Duration, keyFunc func(c *gin.Context) string) gin.HandlerFunc {
	if keyFunc == nil {
		keyFunc = func(c *gin.Context) string { return c.ClientIP() }
	}
	return func(c *gin.Context) {
		dim := keyFunc(c)
		key := fmt.Sprintf("ratelimit:%s:%s", c.Request.URL.Path, dim)

		allowed, _, err := rdb.Allow(c.Request.Context(), key, limit, window)
		if err != nil {
			// 限流器异常时放行（避免误伤用户），但记录日志（已由后续日志中间件处理）
			c.Next()
			return
		}
		if !allowed {
			response.TooManyRequests(c, "")
			return
		}
		c.Next()
	}
}

// RateLimitByUser 按用户维度限流（要求已通过 Auth 中间件）
func RateLimitByUser(rdb *cache.Client, limit int64, window time.Duration) gin.HandlerFunc {
	return RateLimit(rdb, limit, window, func(c *gin.Context) string {
		if v, ok := c.Get(CtxKeyUserID); ok {
			if s, ok2 := v.(string); ok2 && s != "" {
				return "user:" + s
			}
		}
		return "ip:" + c.ClientIP()
	})
}

// RateLimitByTenant 按租户维度限流
func RateLimitByTenant(rdb *cache.Client, limit int64, window time.Duration) gin.HandlerFunc {
	return RateLimit(rdb, limit, window, func(c *gin.Context) string {
		if v, ok := c.Get(CtxKeyTenantID); ok {
			if s, ok2 := v.(string); ok2 && s != "" {
				return "tenant:" + s
			}
		}
		return "ip:" + c.ClientIP()
	})
}
