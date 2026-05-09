package middleware

import (
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/zhongjinmuai-lang/mu-framework/pkg/response"
)

// TenantRLS 激活 PostgreSQL 行级安全（RLS）
// 通过 SET_CONFIG 将当前租户ID和层级注入会话变量，
// 后续该请求的所有 SQL 会被 RLS 策略自动按租户过滤。
//
// 注意：必须放在 Auth 之后、业务 Handler 之前。
func TenantRLS(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		tenantID, ok := c.Get(CtxKeyTenantID)
		if !ok {
			c.Next() // 匿名接口（如登录）无租户上下文，直接放行
			return
		}
		tid, _ := tenantID.(string)
		level, _ := c.Get(CtxKeyLevel)
		lvl, _ := level.(string)

		// 使用 set_config('key', 'value', true) —— 第三个参数 true 表示事务级，
		// 在同一连接内有效直到事务结束或连接回池时重置。
		ctx := c.Request.Context()
		session := db.WithContext(ctx)
		if tid != "" {
			if err := session.Exec("SELECT set_config('app.current_tenant_id', ?, true)", tid).Error; err != nil {
				response.InternalError(c, "租户上下文注入失败: "+err.Error())
				return
			}
		}
		if lvl != "" {
			if err := session.Exec("SELECT set_config('app.current_tenant_level', ?, true)", lvl).Error; err != nil {
				response.InternalError(c, "层级上下文注入失败: "+err.Error())
				return
			}
		}
		c.Set("db", session)
		c.Next()
	}
}
