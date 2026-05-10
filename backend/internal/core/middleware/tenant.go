// Package middleware - 租户 RLS 行级安全中间件
package middleware

import (
	"context"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/zhongjinmuai-lang/mu-framework/pkg/response"
)

// dbKey 请求上下文中的 *gorm.DB 键
const dbKey = "db"

// TenantRLS 激活 PostgreSQL 行级安全（RLS）
//
// 【v1.5 关键修复】
// 旧版使用 set_config(_, true)（事务级）在 GORM 连接池下会立即失效：
// 每个 Exec 从池中取新连接，set_config 作用仅限该次调用的那个事务，
// 业务 SQL 再取新连接时，RLS 上下文已丢失，多租户隔离完全失败。
//
// 修复方案：
//  1. 开启一个 Transaction，所有业务 SQL 走同一事务
//  2. 在事务内先 set_config，确保 RLS 会话变量可见
//  3. 请求结束统一提交或回滚
//
// 使用方式（业务 handler 中）：
//
//	db := middleware.GetTenantDB(c)
//	db.Where(...).Find(&users)  // SQL 自动被 RLS 过滤
func TenantRLS(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		tenantIDVal, ok := c.Get(CtxKeyTenantID)
		if !ok {
			c.Set(dbKey, db.WithContext(c.Request.Context()))
			c.Next()
			return
		}
		tid, _ := tenantIDVal.(string)
		lvl, _ := c.Get(CtxKeyLevel)
		lvlStr, _ := lvl.(string)

		// 开启事务确保 RLS 与业务 SQL 在同一连接
		tx := db.WithContext(c.Request.Context()).Begin()
		if tx.Error != nil {
			response.InternalError(c, "数据库事务启动失败: "+tx.Error.Error())
			return
		}

		// 注入 RLS 会话变量
		if tid != "" {
			if err := tx.Exec("SELECT set_config('app.current_tenant_id', ?, true)", tid).Error; err != nil {
				tx.Rollback()
				response.InternalError(c, "租户上下文注入失败: "+err.Error())
				return
			}
		}
		if lvlStr != "" {
			if err := tx.Exec("SELECT set_config('app.current_tenant_level', ?, true)", lvlStr).Error; err != nil {
				tx.Rollback()
				response.InternalError(c, "层级上下文注入失败: "+err.Error())
				return
			}
		}

		c.Set(dbKey, tx)
		ctx := context.WithValue(c.Request.Context(), ctxDBKey{}, tx)
		c.Request = c.Request.WithContext(ctx)

		defer func() {
			if r := recover(); r != nil {
				tx.Rollback()
				panic(r)
			}
		}()

		c.Next()

		if c.Writer.Status() >= 500 || len(c.Errors) > 0 {
			tx.Rollback()
		} else {
			if err := tx.Commit().Error; err != nil {
				c.Error(err)
			}
		}
	}
}

type ctxDBKey struct{}

// GetTenantDB 从 Gin Context 获取带 RLS 的 *gorm.DB
func GetTenantDB(c *gin.Context) *gorm.DB {
	if v, ok := c.Get(dbKey); ok {
		if tx, ok2 := v.(*gorm.DB); ok2 {
			return tx
		}
	}
	return nil
}

// DBFromContext 从 context.Context 提取 *gorm.DB（供 service 层使用）
func DBFromContext(ctx context.Context, fallback *gorm.DB) *gorm.DB {
	if v := ctx.Value(ctxDBKey{}); v != nil {
		if tx, ok := v.(*gorm.DB); ok {
			return tx
		}
	}
	return fallback.WithContext(ctx)
}
