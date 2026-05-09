// Package middleware Gin 中间件集合
package middleware

import (
	"context"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/zhongjinmuai-lang/mu-framework/pkg/logger"
)

// 上下文Key / Header Key
const (
	HeaderTraceID = "X-Trace-ID"
	HeaderSpanID  = "X-Span-ID"

	CtxKeyTraceID  = "trace_id"
	CtxKeySpanID   = "span_id"
	CtxKeyUserID   = "user_id"
	CtxKeyTenantID = "tenant_id"
	CtxKeyLevel    = "tenant_level"
	CtxKeyRole     = "role"
	CtxKeyUsername = "username"
	CtxKeyClaims   = "jwt_claims"
)

// Trace 全链路追踪中间件
// - 注入 TraceID 到 Gin Context 和标准 context.Context
// - 在响应头回写 TraceID 便于客户端/网关追踪
// - 注入到 request.Context 以便下游（GORM/Redis/日志）使用
func Trace() gin.HandlerFunc {
	return func(c *gin.Context) {
		traceID := c.GetHeader(HeaderTraceID)
		if traceID == "" {
			traceID = uuid.New().String()
		}
		spanID := uuid.New().String()[:8]

		c.Set(CtxKeyTraceID, traceID)
		c.Set(CtxKeySpanID, spanID)
		c.Header(HeaderTraceID, traceID)
		c.Header(HeaderSpanID, spanID)

		// 注入到 request.Context，供日志/GORM/Redis链路使用
		ctx := c.Request.Context()
		ctx = context.WithValue(ctx, logger.CtxKeyTraceID, traceID)
		ctx = context.WithValue(ctx, logger.CtxKeySpanID, spanID)
		c.Request = c.Request.WithContext(ctx)

		c.Next()
	}
}
