// Package ai MU 智能体 AI 调度网关（v2.4）
//
// 修复：
//   - 移除非法重复 import 块（编译错误）
//   - 熔断器从包级全局变量改为 Gateway 实例字段
//   - 半开状态限制试探请求数量（防雪崩重试）
//
// 统一对接多家大模型供应商：豆包、通义千问、文心一言、DeepSeek、企业私有部署大模型
// 核心能力：多模型路由 / 降级兜底 / 熔断器 / 限流配额 / 调用审计
package ai

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"time"
)

// Provider AI 供应商标识
type Provider string

const (
	ProviderDoubao   Provider = "doubao"   // 豆包（火山引擎）
	ProviderTongyi   Provider = "tongyi"   // 通义千问（阿里）
	ProviderWenxin   Provider = "wenxin"   // 文心一言（百度）
	ProviderDeepSeek Provider = "deepseek" // DeepSeek
	ProviderPrivate  Provider = "private"  // 企业私有部署大模型
)

// Role 消息角色
type Role string

const (
	RoleSystem    Role = "system"
	RoleUser      Role = "user"
	RoleAssistant Role = "assistant"
)

// Message 对话消息
type Message struct {
	Role    Role   `json:"role"`
	Content string `json:"content"`
}

// ChatRequest 对话请求
type ChatRequest struct {
	Model       string    `json:"model"`
	Messages    []Message `json:"messages"`
	Temperature float32   `json:"temperature,omitempty"`
	MaxTokens   int       `json:"max_tokens,omitempty"`
	Stream      bool      `json:"stream,omitempty"`
	TenantID    string    `json:"-"` // 租户配额计量
}

// ChatResponse 对话响应
type ChatResponse struct {
	ID       string   `json:"id"`
	Provider Provider `json:"provider"`
	Model    string   `json:"model"`
	Content  string   `json:"content"`
	Usage    Usage    `json:"usage"`
	Latency  int64    `json:"latency_ms"`
}

// Usage Token 使用量
type Usage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

// Client 单个供应商客户端接口
type Client interface {
	Provider() Provider
	Chat(ctx context.Context, req *ChatRequest) (*ChatResponse, error)
	Health(ctx context.Context) error
}

// UsageRecorder 用量记录器（实现方可写DB/监控）
type UsageRecorder interface {
	Record(ctx context.Context, tenantID string, resp *ChatResponse, err error)
}

// Gateway AI 调度网关（v2.4）
// 支持：按租户路由 / 按优先级降级 / 故障熔断 / 调用审计
type Gateway struct {
	mu       sync.RWMutex
	clients  map[Provider]Client
	breakers map[Provider]*CircuitBreaker // 每个供应商独立熔断器
	priority []Provider                   // 默认优先级（降级顺序）
	recorder UsageRecorder
}

// NewGateway 创建调度网关
func NewGateway(recorder UsageRecorder) *Gateway {
	return &Gateway{
		clients:  make(map[Provider]Client),
		breakers: make(map[Provider]*CircuitBreaker),
		priority: []Provider{},
		recorder: recorder,
	}
}

// Register 注册供应商客户端（自动创建熔断器）
func (g *Gateway) Register(c Client) {
	g.mu.Lock()
	defer g.mu.Unlock()
	p := c.Provider()
	g.clients[p] = c
	g.priority = append(g.priority, p)
	// 默认熔断器：连续失败5次，熔断30秒
	if _, ok := g.breakers[p]; !ok {
		g.breakers[p] = NewCircuitBreaker(5, 30*time.Second)
	}
}

// RegisterWithBreaker 注册供应商并自定义熔断参数
func (g *Gateway) RegisterWithBreaker(c Client, threshold int64, timeout time.Duration) {
	g.mu.Lock()
	defer g.mu.Unlock()
	p := c.Provider()
	g.clients[p] = c
	g.priority = append(g.priority, p)
	g.breakers[p] = NewCircuitBreaker(threshold, timeout)
}

// SetPriority 自定义降级优先级
func (g *Gateway) SetPriority(order []Provider) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.priority = order
}

// Chat 智能路由调用（带熔断降级）
// provider 为空则按优先级自动选择并降级
func (g *Gateway) Chat(ctx context.Context, provider Provider, req *ChatRequest) (*ChatResponse, error) {
	g.mu.RLock()
	order := make([]Provider, len(g.priority))
	copy(order, g.priority)
	clients := g.clients
	breakers := g.breakers
	g.mu.RUnlock()

	// 指定供应商：只尝试该供应商（仍检查熔断）
	if provider != "" {
		c, ok := clients[provider]
		if !ok {
			return nil, fmt.Errorf("AI供应商未注册: %s", provider)
		}
		if cb, ok := breakers[provider]; ok && !cb.Allow() {
			return nil, fmt.Errorf("AI供应商 %s 已熔断，请稍后重试", provider)
		}
		resp, err := g.callAndRecord(ctx, c, req)
		if cb, ok := breakers[provider]; ok {
			if err != nil {
				cb.RecordFailure()
			} else {
				cb.RecordSuccess()
			}
		}
		return resp, err
	}

	// 未指定：按优先级降级（跳过已熔断的供应商）
	var lastErr error
	for _, p := range order {
		c, ok := clients[p]
		if !ok {
			continue
		}
		// 检查熔断器
		if cb, ok := breakers[p]; ok && !cb.Allow() {
			continue
		}

		resp, err := g.callAndRecord(ctx, c, req)
		if err == nil {
			if cb, ok := breakers[p]; ok {
				cb.RecordSuccess()
			}
			return resp, nil
		}
		lastErr = err
		if cb, ok := breakers[p]; ok {
			cb.RecordFailure()
		}
	}
	if lastErr == nil {
		lastErr = errors.New("无可用AI供应商（全部熔断或未注册）")
	}
	return nil, lastErr
}

func (g *Gateway) callAndRecord(ctx context.Context, c Client, req *ChatRequest) (*ChatResponse, error) {
	start := time.Now()
	resp, err := c.Chat(ctx, req)
	if resp != nil {
		resp.Provider = c.Provider()
		resp.Latency = time.Since(start).Milliseconds()
	}
	if g.recorder != nil {
		g.recorder.Record(ctx, req.TenantID, resp, err)
	}
	return resp, err
}

// List 已注册供应商
func (g *Gateway) List() []Provider {
	g.mu.RLock()
	defer g.mu.RUnlock()
	out := make([]Provider, 0, len(g.clients))
	for p := range g.clients {
		out = append(out, p)
	}
	return out
}

// BreakerStats 所有供应商熔断器统计
func (g *Gateway) BreakerStats() map[string]interface{} {
	g.mu.RLock()
	defer g.mu.RUnlock()
	stats := make(map[string]interface{})
	for p, cb := range g.breakers {
		stats[string(p)] = cb.Stats()
	}
	return stats
}

// Stats 网关统计
func (g *Gateway) Stats() map[string]interface{} {
	g.mu.RLock()
	defer g.mu.RUnlock()
	return map[string]interface{}{
		"registered_providers": len(g.clients),
		"priority":            g.priority,
		"breakers":            g.BreakerStats(),
	}
}

// ========== 熔断器实现 ==========

// CircuitState 熔断器状态
type CircuitState int32

const (
	CircuitClosed   CircuitState = 0 // 正常（允许所有请求）
	CircuitOpen     CircuitState = 1 // 熔断（拒绝请求）
	CircuitHalfOpen CircuitState = 2 // 半开（试探性放行有限请求）
)

// CircuitBreaker 熔断器（每个供应商独立）
type CircuitBreaker struct {
	state        atomic.Int32
	failCount    atomic.Int64
	successCount atomic.Int64
	lastFailAt   atomic.Value  // time.Time
	probeCount   atomic.Int64  // 半开状态已放行的试探请求数
	threshold    int64         // 连续失败 N 次触发熔断
	timeout      time.Duration // 熔断恢复超时
	maxProbes    int64         // 半开状态最大试探数
}

// NewCircuitBreaker 创建熔断器
func NewCircuitBreaker(threshold int64, timeout time.Duration) *CircuitBreaker {
	cb := &CircuitBreaker{
		threshold: threshold,
		timeout:   timeout,
		maxProbes: 3, // 半开状态最多放行3个试探请求
	}
	cb.state.Store(int32(CircuitClosed))
	return cb
}

// Allow 是否允许请求通过
func (cb *CircuitBreaker) Allow() bool {
	state := CircuitState(cb.state.Load())
	switch state {
	case CircuitClosed:
		return true
	case CircuitOpen:
		// 检查是否超过超时进入半开
		if last, ok := cb.lastFailAt.Load().(time.Time); ok {
			if time.Since(last) > cb.timeout {
				// CAS 切换到半开（只有一个 goroutine 能成功切换）
				if cb.state.CompareAndSwap(int32(CircuitOpen), int32(CircuitHalfOpen)) {
					cb.probeCount.Store(0)
				}
				return cb.allowProbe()
			}
		}
		return false
	case CircuitHalfOpen:
		return cb.allowProbe()
	}
	return true
}

// allowProbe 半开状态限制试探请求数量
func (cb *CircuitBreaker) allowProbe() bool {
	current := cb.probeCount.Add(1)
	return current <= cb.maxProbes
}

// RecordSuccess 记录成功
func (cb *CircuitBreaker) RecordSuccess() {
	cb.successCount.Add(1)
	cb.failCount.Store(0)
	// 半开状态下成功 → 恢复关闭
	if CircuitState(cb.state.Load()) == CircuitHalfOpen {
		cb.state.Store(int32(CircuitClosed))
		cb.probeCount.Store(0)
	}
}

// RecordFailure 记录失败
func (cb *CircuitBreaker) RecordFailure() {
	count := cb.failCount.Add(1)
	cb.lastFailAt.Store(time.Now())
	state := CircuitState(cb.state.Load())

	switch state {
	case CircuitClosed:
		// 达到阈值 → 熔断
		if count >= cb.threshold {
			cb.state.Store(int32(CircuitOpen))
		}
	case CircuitHalfOpen:
		// 半开状态下失败 → 立即回到全开
		cb.state.Store(int32(CircuitOpen))
		cb.probeCount.Store(0)
	}
}

// State 当前状态
func (cb *CircuitBreaker) State() CircuitState { return CircuitState(cb.state.Load()) }

// Reset 手动重置熔断器
func (cb *CircuitBreaker) Reset() {
	cb.state.Store(int32(CircuitClosed))
	cb.failCount.Store(0)
	cb.successCount.Store(0)
	cb.probeCount.Store(0)
}

// Stats 统计
func (cb *CircuitBreaker) Stats() map[string]interface{} {
	stateStr := "closed"
	switch cb.State() {
	case CircuitOpen:
		stateStr = "open"
	case CircuitHalfOpen:
		stateStr = "half_open"
	}
	return map[string]interface{}{
		"state":         stateStr,
		"fail_count":    cb.failCount.Load(),
		"success_count": cb.successCount.Load(),
		"threshold":     cb.threshold,
		"timeout_sec":   cb.timeout.Seconds(),
	}
}
