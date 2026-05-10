// Package middleware v1.6 监控埋点中间件
//
// 自动采集每个 HTTP 请求的：
//   - 总数 / 错误数 / 响应时间
//   - 写入 pkg/metrics 预置指标
package middleware

import (
	"time"

	"github.com/gin-gonic/gin"

	"github.com/zhongjinmuai-lang/mu-framework/pkg/metrics"
)

// Metrics HTTP 请求自动埋点中间件
// 放在 Recovery 之后、业务 handler 之前
func Metrics() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()

		c.Next()

		duration := time.Since(start)
		status := c.Writer.Status()

		// 计数
		metrics.HTTPRequestsTotal.Inc()
		if status >= 400 {
			metrics.HTTPRequestErrors.Inc()
		}

		// 耗时分布
		metrics.HTTPRequestDuration.Observe(duration)
	}
}
