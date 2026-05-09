package middleware

import (
	"os"
	"strings"
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
)

// CORS 跨域中间件
//
// 三级独立部署下，服务商/终端客户前端跨服务器调用开发商 API。
//
// 配置方式（按优先级）：
//  1. 环境变量 MU_CORS_ALLOW_ORIGINS: 逗号分隔白名单，如
//     "https://provider.example.com,https://customer.example.com"
//  2. 环境变量 MU_CORS_ALLOW_ALL=true: 允许所有源（仅开发/测试环境）
//  3. 默认: 允许所有源（便于首次部署调试，生产务必收紧）
func CORS() gin.HandlerFunc {
	cfg := cors.Config{
		AllowMethods:     []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS", "HEAD"},
		AllowHeaders:     []string{"Origin", "Content-Type", "Accept", "Authorization", "X-Tenant-ID", "X-Trace-ID"},
		ExposeHeaders:    []string{"Content-Length", "X-Trace-ID", "X-Span-ID", "X-New-Access-Token", "X-New-Refresh-Token"},
		AllowCredentials: false,
		MaxAge:           12 * time.Hour,
	}

	if origins := os.Getenv("MU_CORS_ALLOW_ORIGINS"); origins != "" {
		// 白名单模式（生产推荐）
		list := make([]string, 0)
		for _, o := range strings.Split(origins, ",") {
			if s := strings.TrimSpace(o); s != "" {
				list = append(list, s)
			}
		}
		cfg.AllowOrigins = list
		// 白名单模式下允许携带凭证
		cfg.AllowCredentials = true
	} else {
		// 通配模式（初次部署便于调试）
		cfg.AllowAllOrigins = true
	}

	return cors.New(cfg)
}
