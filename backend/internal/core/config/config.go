package config

import (
	"os"

	"gopkg.in/yaml.v3"
)

// Config 全局配置结构
type Config struct {
	Server   ServerConfig   `yaml:"server"`
	Database DatabaseConfig `yaml:"database"`
	Redis    RedisConfig    `yaml:"redis"`
	JWT      JWTConfig      `yaml:"jwt"`
	Agent    AgentConfig    `yaml:"agent"`
	Platform PlatformConfig `yaml:"platform"`
}

// ServerConfig 服务器配置
type ServerConfig struct {
	Port      int    `yaml:"port"`
	AdminPort int    `yaml:"admin_port"`
	AgentPort int    `yaml:"agent_port"`
	Mode      string `yaml:"mode"` // dev / staging / prod
}

// DatabaseConfig 数据库配置
type DatabaseConfig struct {
	Host     string `yaml:"host"`
	Port     int    `yaml:"port"`
	User     string `yaml:"user"`
	Password string `yaml:"password"`
	DBName   string `yaml:"dbname"`
	SSLMode  string `yaml:"sslmode"`
	MaxConns int    `yaml:"max_conns"`
	MinConns int    `yaml:"min_conns"`
}

// RedisConfig Redis配置
type RedisConfig struct {
	Addr     string `yaml:"addr"`
	Password string `yaml:"password"`
	DB       int    `yaml:"db"`
	PoolSize int    `yaml:"pool_size"`
}

// JWTConfig JWT配置
type JWTConfig struct {
	Secret     string `yaml:"secret"`
	ExpireHour int    `yaml:"expire_hour"`
	Issuer     string `yaml:"issuer"`
}

// AgentConfig 智能体引擎配置
type AgentConfig struct {
	PluginDir      string `yaml:"plugin_dir"`
	MaxWorkers     int    `yaml:"max_workers"`
	TaskQueueSize  int    `yaml:"task_queue_size"`
	EvolutionCycle int    `yaml:"evolution_cycle"` // 自进化周期（秒）
}

// PlatformConfig 中台配置
type PlatformConfig struct {
	Payment PaymentConfig `yaml:"payment"`
	Storage StorageConfig `yaml:"storage"`
	Notify  NotifyConfig  `yaml:"notify"`
}

// PaymentConfig 支付中台配置
type PaymentConfig struct {
	Enabled  bool   `yaml:"enabled"`
	Provider string `yaml:"provider"` // wechat / alipay / stripe
}

// StorageConfig 存储中台配置
type StorageConfig struct {
	Enabled  bool   `yaml:"enabled"`
	Provider string `yaml:"provider"` // local / oss / cos / s3 / minio
	BasePath string `yaml:"base_path"`
}

// NotifyConfig 通知中台配置
type NotifyConfig struct {
	Enabled bool `yaml:"enabled"`
}

// Load 从YAML文件加载配置
func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	cfg := &Config{}
	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, err
	}

	// 环境变量覆盖
	cfg.applyEnvOverrides()

	return cfg, nil
}

// applyEnvOverrides 应用环境变量覆盖
func (c *Config) applyEnvOverrides() {
	if v := os.Getenv("MU_DB_HOST"); v != "" {
		c.Database.Host = v
	}
	if v := os.Getenv("MU_DB_PASSWORD"); v != "" {
		c.Database.Password = v
	}
	if v := os.Getenv("MU_REDIS_ADDR"); v != "" {
		c.Redis.Addr = v
	}
	if v := os.Getenv("MU_JWT_SECRET"); v != "" {
		c.JWT.Secret = v
	}
}

// DSN 生成数据库连接字符串
func (c *DatabaseConfig) DSN() string {
	return "host=" + c.Host +
		" port=" + itoa(c.Port) +
		" user=" + c.User +
		" password=" + c.Password +
		" dbname=" + c.DBName +
		" sslmode=" + c.SSLMode
}

func itoa(i int) string {
	return fmt.Sprintf("%d", i)
}
