package middleware

import (
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
)

// CORS 跨域中间件（默认宽松策略，生产请按需收紧）
func CORS() gin.HandlerFunc {
	return cors.New(cors.Config{
		AllowAllOrigins:  true,
		AllowMethods:     []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS", "HEAD"},
		AllowHeaders:     []string{"Origin", "Content-Type", "Accept", "Authorization", "X-Tenant-ID", "X-Trace-ID"},
		ExposeHeaders:    []string{"Content-Length", "X-Trace-ID", "X-Span-ID", "X-New-Access-Token", "X-New-Refresh-Token"},
		AllowCredentials: false,
		MaxAge:           12 * time.Hour,
	})
}
