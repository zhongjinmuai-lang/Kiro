package middleware

import (
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"github.com/zhongjinmuai-lang/mu-framework/pkg/logger"
)

// AccessLog 访问日志中间件：结构化记录每次 HTTP 请求
// 基于 Zap，自动携带 TraceID / 用户 / 租户等上下文
func AccessLog() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		path := c.Request.URL.Path
		raw := c.Request.URL.RawQuery

		c.Next()

		cost := time.Since(start)
		status := c.Writer.Status()

		fields := []zap.Field{
			zap.Int("status", status),
			zap.String("method", c.Request.Method),
			zap.String("path", path),
			zap.String("query", raw),
			zap.String("ip", c.ClientIP()),
			zap.String("ua", c.Request.UserAgent()),
			zap.Duration("cost", cost),
			zap.Int("resp_size", c.Writer.Size()),
		}

		if v, ok := c.Get(CtxKeyTraceID); ok {
			if s, ok2 := v.(string); ok2 {
				fields = append(fields, zap.String("trace_id", s))
			}
		}
		if v, ok := c.Get(CtxKeyTenantID); ok {
			if s, ok2 := v.(string); ok2 {
				fields = append(fields, zap.String("tenant_id", s))
			}
		}
		if v, ok := c.Get(CtxKeyUserID); ok {
			if s, ok2 := v.(string); ok2 {
				fields = append(fields, zap.String("user_id", s))
			}
		}
		if errs := c.Errors.ByType(gin.ErrorTypePrivate); len(errs) > 0 {
			fields = append(fields, zap.String("errors", errs.String()))
		}

		log := logger.L()
		switch {
		case status >= 500:
			log.Error("HTTP请求异常", fields...)
		case status >= 400:
			log.Warn("HTTP请求异常", fields...)
		default:
			log.Info("HTTP请求", fields...)
		}
	}
}
