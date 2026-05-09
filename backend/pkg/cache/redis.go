// Package cache Redis 7.4.x 集群版统一客户端封装
// 覆盖能力：缓存 / 会话 / 接口限流 / 分布式锁 / 分布式任务入队
package cache

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

// Mode Redis 部署模式
type Mode string

const (
	ModeStandalone Mode = "standalone" // 单机
	ModeCluster    Mode = "cluster"    // 集群（生产默认）
	ModeSentinel   Mode = "sentinel"   // 哨兵
)

// Config Redis 配置
type Config struct {
	Mode       Mode     `mapstructure:"mode" yaml:"mode"`
	Addrs      []string `mapstructure:"addrs" yaml:"addrs"`             // 集群/哨兵节点
	Addr       string   `mapstructure:"addr" yaml:"addr"`               // 单机地址
	MasterName string   `mapstructure:"master_name" yaml:"master_name"` // 哨兵主节点名
	Password   string   `mapstructure:"password" yaml:"password"`
	DB         int      `mapstructure:"db" yaml:"db"`
	PoolSize   int      `mapstructure:"pool_size" yaml:"pool_size"`
	MinIdle    int      `mapstructure:"min_idle" yaml:"min_idle"`
}

// Client 统一客户端（同时兼容单机与集群）
type Client struct {
	redis.UniversalClient
	mode Mode
}

// New 根据配置创建统一客户端
func New(cfg *Config) (*Client, error) {
	if cfg == nil {
		return nil, errors.New("redis配置不能为空")
	}

	opts := &redis.UniversalOptions{
		Password:     cfg.Password,
		DB:           cfg.DB,
		PoolSize:     cfg.PoolSize,
		MinIdleConns: cfg.MinIdle,
	}

	switch cfg.Mode {
	case ModeCluster:
		opts.Addrs = cfg.Addrs
	case ModeSentinel:
		opts.Addrs = cfg.Addrs
		opts.MasterName = cfg.MasterName
	default: // 单机
		if len(cfg.Addrs) > 0 {
			opts.Addrs = cfg.Addrs
		} else {
			opts.Addrs = []string{cfg.Addr}
		}
	}

	uc := redis.NewUniversalClient(opts)
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if err := uc.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("Redis连接失败: %w", err)
	}

	return &Client{UniversalClient: uc, mode: cfg.Mode}, nil
}

// Mode 当前模式
func (c *Client) Mode() Mode { return c.mode }

// ========== 分布式锁（SET NX PX + 释放脚本，支持集群） ==========

// Lock 分布式锁对象
type Lock struct {
	client *Client
	key    string
	token  string
}

// luaUnlock 原子释放锁脚本：仅当 value 匹配当前 token 时删除
const luaUnlock = `if redis.call("GET", KEYS[1]) == ARGV[1] then return redis.call("DEL", KEYS[1]) else return 0 end`

// Acquire 尝试获取分布式锁（非阻塞）
// ttl：锁的最长持有时间（防止死锁）
func (c *Client) Acquire(ctx context.Context, key string, ttl time.Duration) (*Lock, error) {
	token, err := randomToken()
	if err != nil {
		return nil, err
	}
	ok, err := c.SetNX(ctx, key, token, ttl).Result()
	if err != nil {
		return nil, fmt.Errorf("获取分布式锁失败: %w", err)
	}
	if !ok {
		return nil, ErrLockHeld
	}
	return &Lock{client: c, key: key, token: token}, nil
}

// AcquireWait 阻塞获取（带超时与重试间隔）
func (c *Client) AcquireWait(ctx context.Context, key string, ttl, waitTimeout, interval time.Duration) (*Lock, error) {
	deadline := time.Now().Add(waitTimeout)
	for {
		l, err := c.Acquire(ctx, key, ttl)
		if err == nil {
			return l, nil
		}
		if !errors.Is(err, ErrLockHeld) {
			return nil, err
		}
		if time.Now().After(deadline) {
			return nil, ErrLockWaitTimeout
		}
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(interval):
		}
	}
}

// Release 安全释放锁（仅当令牌匹配时才删除）
func (l *Lock) Release(ctx context.Context) error {
	if l == nil {
		return nil
	}
	_, err := l.client.Eval(ctx, luaUnlock, []string{l.key}, l.token).Result()
	return err
}

var (
	ErrLockHeld        = errors.New("锁已被其他客户端持有")
	ErrLockWaitTimeout = errors.New("获取锁超时")
)

func randomToken() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

// ========== 固定窗口限流（适合接口QPS限流） ==========

// luaRateLimit 限流脚本：INCR + EXPIRE（首次设置TTL），返回当前计数
const luaRateLimit = `local c = redis.call("INCR", KEYS[1]) if c == 1 then redis.call("EXPIRE", KEYS[1], ARGV[1]) end return c`

// Allow 在 window 窗口内是否允许继续（当前计数 <= limit 则放行）
func (c *Client) Allow(ctx context.Context, key string, limit int64, window time.Duration) (bool, int64, error) {
	seconds := int64(window.Seconds())
	if seconds <= 0 {
		seconds = 1
	}
	res, err := c.Eval(ctx, luaRateLimit, []string{key}, seconds).Int64()
	if err != nil {
		return false, 0, err
	}
	return res <= limit, res, nil
}

// ========== 分布式任务入队（基于 Redis Stream） ==========

// Enqueue 投递任务到指定 stream
func (c *Client) Enqueue(ctx context.Context, stream string, payload map[string]any) (string, error) {
	return c.XAdd(ctx, &redis.XAddArgs{
		Stream: stream,
		Values: payload,
	}).Result()
}
