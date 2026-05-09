package bootstrap

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"

	"github.com/zhongjinmuai-lang/mu-framework/internal/core/config"
)

// App 应用实例，承载所有核心依赖
type App struct {
	Config *config.Config
	DB     *pgxpool.Pool
	Redis  *redis.Client
}

// NewApp 创建并初始化应用实例
func NewApp(cfg *config.Config) (*App, error) {
	app := &App{
		Config: cfg,
	}

	// 初始化数据库连接池
	if err := app.initDatabase(); err != nil {
		return nil, fmt.Errorf("数据库初始化失败: %w", err)
	}

	// 初始化Redis
	if err := app.initRedis(); err != nil {
		return nil, fmt.Errorf("Redis初始化失败: %w", err)
	}

	slog.Info("应用初始化完成")
	return app, nil
}

// initDatabase 初始化PostgreSQL连接池
func (a *App) initDatabase() error {
	dsn := a.Config.Database.DSN()

	poolCfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		return fmt.Errorf("解析数据库配置失败: %w", err)
	}

	poolCfg.MaxConns = int32(a.Config.Database.MaxConns)
	poolCfg.MinConns = int32(a.Config.Database.MinConns)

	pool, err := pgxpool.NewWithConfig(context.Background(), poolCfg)
	if err != nil {
		return fmt.Errorf("创建连接池失败: %w", err)
	}

	// 测试连接
	if err := pool.Ping(context.Background()); err != nil {
		return fmt.Errorf("数据库连接测试失败: %w", err)
	}

	a.DB = pool
	slog.Info("数据库连接池已建立", "host", a.Config.Database.Host)
	return nil
}

// initRedis 初始化Redis客户端
func (a *App) initRedis() error {
	a.Redis = redis.NewClient(&redis.Options{
		Addr:     a.Config.Redis.Addr,
		Password: a.Config.Redis.Password,
		DB:       a.Config.Redis.DB,
		PoolSize: a.Config.Redis.PoolSize,
	})

	// 测试连接
	if err := a.Redis.Ping(context.Background()).Err(); err != nil {
		return fmt.Errorf("Redis连接测试失败: %w", err)
	}

	slog.Info("Redis连接已建立", "addr", a.Config.Redis.Addr)
	return nil
}

// Shutdown 优雅关闭所有连接
func (a *App) Shutdown() {
	if a.DB != nil {
		a.DB.Close()
		slog.Info("数据库连接已关闭")
	}
	if a.Redis != nil {
		_ = a.Redis.Close()
		slog.Info("Redis连接已关闭")
	}
}
