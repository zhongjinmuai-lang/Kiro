// Package bootstrap 应用引导层：统一初始化数据库、缓存、JWT、日志等核心依赖
package bootstrap

import (
	"context"
	"errors"
	"fmt"
	"time"

	"go.uber.org/zap"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
	"gorm.io/gorm/schema"

	"github.com/zhongjinmuai-lang/mu-framework/internal/core/config"
	"github.com/zhongjinmuai-lang/mu-framework/pkg/cache"
	"github.com/zhongjinmuai-lang/mu-framework/pkg/jwt"
	"github.com/zhongjinmuai-lang/mu-framework/pkg/logger"
)

// App 应用实例：承载所有核心依赖
type App struct {
	Config *config.Config
	DB     *gorm.DB
	Redis  *cache.Client
	JWT    *jwt.Manager
}

// NewApp 创建并初始化应用实例
func NewApp(cfg *config.Config) (*App, error) {
	if cfg == nil {
		return nil, errors.New("配置不能为空")
	}

	// 1. 初始化日志（最先，后续组件都需要）
	if err := logger.Init(&cfg.Logger); err != nil {
		return nil, fmt.Errorf("日志初始化失败: %w", err)
	}
	logger.Info("MU Framework 引导启动",
		zap.String("app", cfg.App.Name),
		zap.String("version", cfg.App.Version),
		zap.String("env", cfg.App.Env),
	)

	app := &App{Config: cfg}

	// 2. 初始化 PostgreSQL（GORM v2）
	if err := app.initDatabase(); err != nil {
		return nil, fmt.Errorf("数据库初始化失败: %w", err)
	}

	// 3. 初始化 Redis（单机 / 集群 / 哨兵自适应）
	if err := app.initRedis(); err != nil {
		return nil, fmt.Errorf("Redis 初始化失败: %w", err)
	}

	// 4. 初始化 JWT 管理器（使用 Redis 实现令牌黑名单 + 刷新令牌旋转）
	app.JWT = jwt.NewManager(&cfg.JWT, app.Redis.UniversalClient)

	logger.Info("应用初始化完成",
		zap.Int("db_max_open_conns", cfg.Database.MaxOpenConns),
		zap.String("redis_mode", string(app.Redis.Mode())),
	)
	return app, nil
}

// initDatabase 初始化 PostgreSQL 连接池
func (a *App) initDatabase() error {
	cfg := &a.Config.Database

	// GORM Logger：桥接到 Zap
	gormLog := gormlogger.New(
		&gormZapWriter{log: logger.L()},
		gormlogger.Config{
			SlowThreshold:             cfg.SlowThreshold,
			LogLevel:                  gormLogLevel(cfg.LogSQL),
			IgnoreRecordNotFoundError: true,
			Colorful:                  false,
		},
	)

	db, err := gorm.Open(postgres.Open(cfg.DSN()), &gorm.Config{
		Logger:                                   gormLog,
		NamingStrategy:                           schema.NamingStrategy{SingularTable: false}, // 使用复数表名
		PrepareStmt:                              true, // 开启预编译缓存（性能优化）
		DisableForeignKeyConstraintWhenMigrating: false,
		NowFunc:                                  func() time.Time { return time.Now().Local() },
	})
	if err != nil {
		return err
	}

	sqlDB, err := db.DB()
	if err != nil {
		return err
	}

	// 连接池参数
	sqlDB.SetMaxOpenConns(cfg.MaxOpenConns)
	sqlDB.SetMaxIdleConns(cfg.MaxIdleConns)
	sqlDB.SetConnMaxLifetime(cfg.ConnMaxLifetime)
	sqlDB.SetConnMaxIdleTime(cfg.ConnMaxIdleTime)

	// 连通性测试
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := sqlDB.PingContext(ctx); err != nil {
		return fmt.Errorf("数据库连通性测试失败: %w", err)
	}

	a.DB = db
	logger.Info("PostgreSQL 连接池已建立",
		zap.String("host", cfg.Host),
		zap.Int("port", cfg.Port),
		zap.String("dbname", cfg.DBName),
	)
	return nil
}

// initRedis 初始化 Redis 统一客户端
func (a *App) initRedis() error {
	c, err := cache.New(&a.Config.Redis)
	if err != nil {
		return err
	}
	a.Redis = c
	logger.Info("Redis 已连接", zap.String("mode", string(c.Mode())))
	return nil
}

// Shutdown 优雅关闭所有连接
func (a *App) Shutdown() {
	logger.Info("MU Framework 正在关闭...")
	if a.DB != nil {
		if sqlDB, err := a.DB.DB(); err == nil {
			_ = sqlDB.Close()
			logger.Info("数据库连接已关闭")
		}
	}
	if a.Redis != nil {
		_ = a.Redis.Close()
		logger.Info("Redis 连接已关闭")
	}
	logger.Sync()
}

// WithTenantSession 返回携带 RLS 租户上下文的 *gorm.DB
// 通过会话变量 app.current_tenant_id / app.current_tenant_level 启用 PG 行级安全
func (a *App) WithTenantSession(ctx context.Context, tenantID, level string) *gorm.DB {
	db := a.DB.WithContext(ctx)
	if tenantID != "" {
		db = db.Set("app.current_tenant_id", tenantID)
		db.Exec("SELECT set_config('app.current_tenant_id', ?, true)", tenantID)
	}
	if level != "" {
		db.Exec("SELECT set_config('app.current_tenant_level', ?, true)", level)
	}
	return db
}

// ========== GORM ↔ Zap 日志桥接 ==========

type gormZapWriter struct{ log *zap.Logger }

func (w *gormZapWriter) Printf(format string, args ...any) {
	w.log.Sugar().Debugf(format, args...)
}

func gormLogLevel(verbose bool) gormlogger.LogLevel {
	if verbose {
		return gormlogger.Info
	}
	return gormlogger.Warn
}
