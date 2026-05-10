// Package ai v1.7 AI 网关成本统计
//
// 按租户/供应商维度统计 Token 消耗和估算成本
package ai

import (
	"context"
	"sync"
	"time"
)

// CostConfig 成本配置（每百万 Token 价格，单位：元）
type CostConfig struct {
	PromptCostPer1M     float64 `json:"prompt_cost_per_1m"`
	CompletionCostPer1M float64 `json:"completion_cost_per_1m"`
}

// DefaultCostConfig 各供应商默认价格（2026 年参考价）
var DefaultCostConfig = map[Provider]CostConfig{
	ProviderDoubao:   {PromptCostPer1M: 0.8, CompletionCostPer1M: 2.0},
	ProviderTongyi:   {PromptCostPer1M: 2.0, CompletionCostPer1M: 6.0},
	ProviderWenxin:   {PromptCostPer1M: 8.0, CompletionCostPer1M: 8.0},
	ProviderDeepSeek: {PromptCostPer1M: 1.0, CompletionCostPer1M: 2.0},
	ProviderPrivate:  {PromptCostPer1M: 0.0, CompletionCostPer1M: 0.0},
}

// CostEntry 单条成本记录
type CostEntry struct {
	TenantID         string    `json:"tenant_id"`
	Provider         Provider  `json:"provider"`
	PromptTokens     int       `json:"prompt_tokens"`
	CompletionTokens int       `json:"completion_tokens"`
	TotalTokens      int       `json:"total_tokens"`
	EstimatedCost    float64   `json:"estimated_cost"`
	Timestamp        time.Time `json:"timestamp"`
}

// CostTracker AI 成本跟踪器（实现 UsageRecorder 接口）
type CostTracker struct {
	mu      sync.Mutex
	entries []CostEntry
	configs map[Provider]CostConfig
	limit   int
}

// NewCostTracker 创建成本跟踪器
func NewCostTracker() *CostTracker {
	return &CostTracker{
		entries: make([]CostEntry, 0),
		configs: DefaultCostConfig,
		limit:   10000,
	}
}

// Record 记录一次调用（实现 UsageRecorder 接口）
func (ct *CostTracker) Record(ctx context.Context, tenantID string, resp *ChatResponse, err error) {
	if resp == nil || err != nil {
		return
	}
	ct.mu.Lock()
	defer ct.mu.Unlock()

	cfg := ct.configs[resp.Provider]
	cost := float64(resp.Usage.PromptTokens)/1_000_000*cfg.PromptCostPer1M +
		float64(resp.Usage.CompletionTokens)/1_000_000*cfg.CompletionCostPer1M

	ct.entries = append(ct.entries, CostEntry{
		TenantID:         tenantID,
		Provider:         resp.Provider,
		PromptTokens:     resp.Usage.PromptTokens,
		CompletionTokens: resp.Usage.CompletionTokens,
		TotalTokens:      resp.Usage.TotalTokens,
		EstimatedCost:    cost,
		Timestamp:        time.Now(),
	})
	if len(ct.entries) > ct.limit {
		ct.entries = ct.entries[len(ct.entries)-ct.limit:]
	}
}

// TotalCost 总成本
func (ct *CostTracker) TotalCost() float64 {
	ct.mu.Lock()
	defer ct.mu.Unlock()
	var total float64
	for _, e := range ct.entries {
		total += e.EstimatedCost
	}
	return total
}

// SummaryByProvider 按供应商汇总
func (ct *CostTracker) SummaryByProvider() map[Provider]float64 {
	ct.mu.Lock()
	defer ct.mu.Unlock()
	m := make(map[Provider]float64)
	for _, e := range ct.entries {
		m[e.Provider] += e.EstimatedCost
	}
	return m
}

// SummaryByTenant 按租户汇总
func (ct *CostTracker) SummaryByTenant() map[string]float64 {
	ct.mu.Lock()
	defer ct.mu.Unlock()
	m := make(map[string]float64)
	for _, e := range ct.entries {
		m[e.TenantID] += e.EstimatedCost
	}
	return m
}
