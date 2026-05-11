// Package config MU 框架统一配置中心（基于 Viper）
// 支持 YAML 文件 + 环境变量覆盖，多环境（dev/staging/prod）切换
package config

import (
	"fmt"
	"strings"
	"time"

	"github.com/spf13/viper"

	"github.com/zhongjinmuai-lang/mu-framework/pkg/cache"
	"github.com/zhongjinmuai-lang/mu-framework/pkg/jwt"
	"github.com/zhongjinmuai-lang/mu-framework/pkg/logger"
)

// Config 全局配置
type Config struct {
	App      AppConfig      `mapstructure:"app"`
	Server   ServerConfig   `mapstructure:"server"`
	Database DatabaseConfig `mapstructure:"database"`
	Redis    cache.Config   `mapstructure:"redis"`
	Logger   logger.Config  `mapstructure:"logger"`
	JWT      jwt.Config     `mapstructure:"jwt"`
	Agent    AgentConfig    `mapstructure:"agent"`
	Platform PlatformConfig `mapstructure:"platform"`
	Swagger  SwaggerConfig  `mapstructure:"swagger"`
}

// AppConfig 应用基础信息
type AppConfig struct {
	Name    string `mapstructure:"name"`
	Version string `mapstructure:"version"`
	Env     string `mapstructure:"env"` // dev / staging / prod
}

// ServerConfig 服务器配置
type ServerConfig struct {
	Port         int           `mapstructure:"port"`
	AdminPort    int           `mapstructure:"admin_port"`
	AgentPort    int           `mapstructure:"agent_port"`
	Mode         string        `mapstructure:"mode"` // debug / release / test
	ReadTimeout  time.Duration `mapstructure:"read_timeout"`
	WriteTimeout time.Duration `mapstructure:"write_timeout"`
	IdleTimeout  time.Duration `mapstructure:"idle_timeout"`
}

// DatabaseConfig PostgreSQL 18.3 配置
type DatabaseConfig struct {
	Host            string        `mapstructure:"host"`
	Port            int           `mapstructure:"port"`
	User            string        `mapstructure:"user"`
	Password        string        `mapstructure:"password"`
	DBName          string        `mapstructure:"dbname"`
	SSLMode         string        `mapstructure:"sslmode"`
	TimeZone        string        `mapstructure:"timezone"`
	MaxOpenConns    int           `mapstructure:"max_open_conns"`
	MaxIdleConns    int           `mapstructure:"max_idle_conns"`
	ConnMaxLifetime time.Duration `mapstructure:"conn_max_lifetime"`
	ConnMaxIdleTime time.Duration `mapstructure:"conn_max_idle_time"`
	LogSQL          bool          `mapstructure:"log_sql"` // 是否打印SQL
	SlowThreshold   time.Duration `mapstructure:"slow_threshold"`
}

// DSN 生成 GORM 驱动使用的 DSN
func (d *DatabaseConfig) DSN() string {
	tz := d.TimeZone
	if tz == "" {
		tz = "Asia/Shanghai"
	}
	return fmt.Sprintf(
		"host=%s port=%d user=%s password=%s dbname=%s sslmode=%s TimeZone=%s",
		d.Host, d.Port, d.User, d.Password, d.DBName, d.SSLMode, tz,
	)
}

// AgentConfig 智能体引擎配置
type AgentConfig struct {
	PluginDir        string `mapstructure:"plugin_dir"`
	MaxWorkers       int    `mapstructure:"max_workers"`
	TaskQueueSize    int    `mapstructure:"task_queue_size"`
	EvolutionCycleSec int   `mapstructure:"evolution_cycle"` // 进化周期（秒）
	TaskTimeoutSec   int    `mapstructure:"task_timeout"`    // 默认任务超时（秒）
}

// PlatformConfig 三大中台配置
type PlatformConfig struct {
	Payment PaymentConfig `mapstructure:"payment"`
	Storage StorageConfig `mapstructure:"storage"`
	Notify  NotifyConfig  `mapstructure:"notify"`
}

type PaymentConfig struct {
	Enabled bool `mapstructure:"enabled"`
}

type StorageConfig struct {
	Enabled  bool   `mapstructure:"enabled"`
	BasePath string `mapstructure:"base_path"`
}

type NotifyConfig struct {
	Enabled bool `mapstructure:"enabled"`
}

// SwaggerConfig Swagger UI 配置
type SwaggerConfig struct {
	Enabled bool   `mapstructure:"enabled"`
	Path    string `mapstructure:"path"` // /swagger/*any
	Title   string `mapstructure:"title"`
	Version string `mapstructure:"version"`
}

// Load 从指定路径加载配置
func Load(path string) (*Config, error) {
	v := viper.New()
	v.SetConfigFile(path)
	v.SetConfigType("yaml")

	// 环境变量覆盖：MU_DATABASE_PASSWORD / MU_JWT_SECRET ...
	v.SetEnvPrefix("MU")
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()

	// 默认值
	setDefaults(v)

	if err := v.ReadInConfig(); err != nil {
		return nil, fmt.Errorf("读取配置文件失败: %w", err)
	}

	cfg := &Config{}
	if err := v.Unmarshal(cfg); err != nil {
		return nil, fmt.Errorf("解析配置失败: %w", err)
	}
	return cfg, nil
}

// setDefaults 统一默认值
func setDefaults(v *viper.Viper) {
	v.SetDefault("app.name", "mu-framework")
	v.SetDefault("app.version", "1.0.0")
	v.SetDefault("app.env", "dev")

	v.SetDefault("server.port", 8080)
	v.SetDefault("server.admin_port", 8081)
	v.SetDefault("server.agent_port", 8082)
	v.SetDefault("server.mode", "debug")
	v.SetDefault("server.read_timeout", "30s")
	v.SetDefault("server.write_timeout", "30s")
	v.SetDefault("server.idle_timeout", "60s")

	v.SetDefault("database.host", "localhost")
	v.SetDefault("database.port", 5432)
	v.SetDefault("database.sslmode", "disable")
	v.SetDefault("database.timezone", "Asia/Shanghai")
	v.SetDefault("database.max_open_conns", 50)
	v.SetDefault("database.max_idle_conns", 10)
	v.SetDefault("database.conn_max_lifetime", "1h")
	v.SetDefault("database.conn_max_idle_time", "10m")
	v.SetDefault("database.slow_threshold", "200ms")

	v.SetDefault("redis.mode", "standalone")
	v.SetDefault("redis.addr", "localhost:6379")
	v.SetDefault("redis.pool_size", 20)
	v.SetDefault("redis.min_idle", 5)

	v.SetDefault("logger.level", "info")
	v.SetDefault("logger.format", "json")
	v.SetDefault("logger.output", "both")
	v.SetDefault("logger.dir", "./logs")
	v.SetDefault("logger.file_name", "mu-framework.log")
	v.SetDefault("logger.max_size", 100)
	v.SetDefault("logger.max_age", 30)
	v.SetDefault("logger.max_backups", 10)
	v.SetDefault("logger.compress", true)
	v.SetDefault("logger.caller", true)
	v.SetDefault("logger.stacktrace", true)

	v.SetDefault("jwt.issuer", "mu-framework")
	v.SetDefault("jwt.access_ttl", "2h")
	v.SetDefault("jwt.refresh_ttl", "168h")
	v.SetDefault("jwt.auto_renew_threshold", "15m")

	v.SetDefault("agent.plugin_dir", "./plugins")
	v.SetDefault("agent.max_workers", 20)
	v.SetDefault("agent.task_queue_size", 2000)
	v.SetDefault("agent.evolution_cycle", 1800)

	v.SetDefault("swagger.enabled", true)
	v.SetDefault("swagger.path", "/swagger/*any")
	v.SetDefault("swagger.title", "MU Framework API")
	v.SetDefault("swagger.version", "v1")
}

// IsDev 是否为开发环境
func (c *Config) IsDev() bool { return c.App.Env == "dev" }

// IsProd 是否为生产环境
func (c *Config) IsProd() bool { return c.App.Env == "prod" }
