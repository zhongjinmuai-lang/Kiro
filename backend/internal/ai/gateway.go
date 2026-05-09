// Package ai MU 智能体 AI 调度网关
// 统一对接多家大模型供应商：豆包、通义千问、文心一言、DeepSeek、企业私有部署大模型
// 核心能力：多模型路由 / 降级兜底 / 限流配额 / 调用审计
package ai

import (
	"context"
	"errors"
	"fmt"
	"sync"
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
	// Health 健康检查，用于熔断与降级判定
	Health(ctx context.Context) error
}

// Gateway AI 调度网关
// 支持：按租户路由 / 按优先级降级 / 故障熔断 / 调用审计
type Gateway struct {
	mu       sync.RWMutex
	clients  map[Provider]Client
	priority []Provider // 默认优先级（降级顺序）
	recorder UsageRecorder
}

// UsageRecorder 用量记录器（实现方可写DB/监控）
type UsageRecorder interface {
	Record(ctx context.Context, tenantID string, resp *ChatResponse, err error)
}

// NewGateway 创建调度网关
func NewGateway(recorder UsageRecorder) *Gateway {
	return &Gateway{
		clients:  make(map[Provider]Client),
		priority: []Provider{},
		recorder: recorder,
	}
}

// Register 注册供应商客户端
func (g *Gateway) Register(c Client) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.clients[c.Provider()] = c
	g.priority = append(g.priority, c.Provider())
}

// SetPriority 自定义降级优先级
func (g *Gateway) SetPriority(order []Provider) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.priority = order
}

// Chat 调用指定供应商（provider 为空则按优先级自动选择并降级）
func (g *Gateway) Chat(ctx context.Context, provider Provider, req *ChatRequest) (*ChatResponse, error) {
	g.mu.RLock()
	order := g.priority
	clients := g.clients
	g.mu.RUnlock()

	// 指定供应商：只尝试该供应商
	if provider != "" {
		c, ok := clients[provider]
		if !ok {
			return nil, fmt.Errorf("AI供应商未注册: %s", provider)
		}
		return g.callAndRecord(ctx, c, req)
	}

	// 未指定：按优先级降级
	var lastErr error
	for _, p := range order {
		c, ok := clients[p]
		if !ok {
			continue
		}
		resp, err := g.callAndRecord(ctx, c, req)
		if err == nil {
			return resp, nil
		}
		lastErr = err
	}
	if lastErr == nil {
		lastErr = errors.New("无可用AI供应商")
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
