// Package hello 示例插件 —— 展示 MU 插件 SDK 与热插拔接入
package hello

import (
	"context"
	"sync/atomic"
	"time"

	"github.com/zhongjinmuai-lang/mu-framework/internal/agent/plugin"
)

// HelloPlugin 示例插件
type HelloPlugin struct {
	counter atomic.Int64
	stop    chan struct{}
}

// New 构造
func New() *HelloPlugin { return &HelloPlugin{stop: make(chan struct{})} }

// Meta 元信息
func (p *HelloPlugin) Meta() plugin.Meta {
	return plugin.Meta{
		ID:           "hello",
		Name:         "Hello 示例插件",
		Version:      "1.0.0",
		Author:       "MU Framework",
		Description:  "演示：每秒自增计数 + 问候 API",
		Category:     "sample",
		MinFramework: "1.0.0",
	}
}

// Init 初始化
func (p *HelloPlugin) Init(ctx context.Context) error { return nil }

// Start 启动
func (p *HelloPlugin) Start() error {
	go func() {
		t := time.NewTicker(time.Second)
		defer t.Stop()
		for {
			select {
			case <-p.stop:
				return
			case <-t.C:
				p.counter.Add(1)
			}
		}
	}()
	return nil
}

// Stop 停止
func (p *HelloPlugin) Stop() error {
	select {
	case <-p.stop:
	default:
		close(p.stop)
	}
	return nil
}

// Health 健康检查
func (p *HelloPlugin) Health() plugin.HealthStatus {
	return plugin.HealthStatus{Healthy: true, Message: "ok", CheckedAt: time.Now()}
}

// Count 对外业务方法：当前计数
func (p *HelloPlugin) Count() int64 { return p.counter.Load() }

// Sayhi 对外业务方法：问候
func (p *HelloPlugin) Sayhi(name string) string {
	if name == "" {
		name = "世界"
	}
	return "你好，" + name + "！当前计数：" + time.Now().Format(time.RFC3339)
}
