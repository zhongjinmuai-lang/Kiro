// Package metrics MU 框架监控埋点
//
// 轻量级内置指标收集器，兼容 Prometheus exposition 格式。
// 无需引入重型 prometheus 客户端库，支持 /metrics 端点暴露。
package metrics

import (
	"fmt"
	"net/http"
	"runtime"
	"sync"
	"sync/atomic"
	"time"
)

// Counter 计数器
type Counter struct {
	name  string
	value atomic.Int64
}

// NewCounter 创建计数器
func NewCounter(name string) *Counter { return &Counter{name: name} }

// Inc 自增
func (c *Counter) Inc() { c.value.Add(1) }

// Add 增加指定值
func (c *Counter) Add(n int64) { c.value.Add(n) }

// Value 当前值
func (c *Counter) Value() int64 { return c.value.Load() }

// Gauge 仪表盘（可升可降）
type Gauge struct {
	name  string
	value atomic.Int64 // 存 float64 的 bits（简化）
}

// NewGauge 创建 Gauge
func NewGauge(name string) *Gauge { return &Gauge{name: name} }

// Set 设置值
func (g *Gauge) Set(v int64) { g.value.Store(v) }

// Inc 自增
func (g *Gauge) Inc() { g.value.Add(1) }

// Dec 自减
func (g *Gauge) Dec() { g.value.Add(-1) }

// Value 当前值
func (g *Gauge) Value() int64 { return g.value.Load() }

// Histogram 直方图（简化版：仅记录 count/sum/max）
type Histogram struct {
	name  string
	count atomic.Int64
	sum   atomic.Int64 // 微秒
	max   atomic.Int64 // 微秒
}

// NewHistogram 创建 Histogram
func NewHistogram(name string) *Histogram { return &Histogram{name: name} }

// Observe 记录一次观测值（duration）
func (h *Histogram) Observe(d time.Duration) {
	us := d.Microseconds()
	h.count.Add(1)
	h.sum.Add(us)
	for {
		old := h.max.Load()
		if us <= old {
			break
		}
		if h.max.CompareAndSwap(old, us) {
			break
		}
	}
}

// Avg 平均值（毫秒）
func (h *Histogram) Avg() float64 {
	c := h.count.Load()
	if c == 0 {
		return 0
	}
	return float64(h.sum.Load()) / float64(c) / 1000.0
}

// Max 最大值（毫秒）
func (h *Histogram) Max() float64 { return float64(h.max.Load()) / 1000.0 }

// Count 总数
func (h *Histogram) Count() int64 { return h.count.Load() }

// ========== 全局注册表 ==========

var (
	mu         sync.RWMutex
	counters   = make(map[string]*Counter)
	gauges     = make(map[string]*Gauge)
	histograms = make(map[string]*Histogram)
)

// RegisterCounter 注册全局计数器
func RegisterCounter(name string) *Counter {
	mu.Lock()
	defer mu.Unlock()
	c := NewCounter(name)
	counters[name] = c
	return c
}

// RegisterGauge 注册全局 Gauge
func RegisterGauge(name string) *Gauge {
	mu.Lock()
	defer mu.Unlock()
	g := NewGauge(name)
	gauges[name] = g
	return g
}

// RegisterHistogram 注册全局 Histogram
func RegisterHistogram(name string) *Histogram {
	mu.Lock()
	defer mu.Unlock()
	h := NewHistogram(name)
	histograms[name] = h
	return h
}

// ========== 预置应用指标 ==========

var (
	// HTTP 请求
	HTTPRequestsTotal   = RegisterCounter("mu_http_requests_total")
	HTTPRequestErrors   = RegisterCounter("mu_http_request_errors_total")
	HTTPRequestDuration = RegisterHistogram("mu_http_request_duration_ms")

	// 数据库
	DBQueryTotal     = RegisterCounter("mu_db_query_total")
	DBSlowQueryTotal = RegisterCounter("mu_db_slow_query_total")
	DBActiveConns    = RegisterGauge("mu_db_active_connections")

	// Redis
	CacheHits   = RegisterCounter("mu_cache_hits_total")
	CacheMisses = RegisterCounter("mu_cache_misses_total")

	// 智能体引擎
	AgentTasksSubmitted = RegisterCounter("mu_agent_tasks_submitted_total")
	AgentTasksCompleted = RegisterCounter("mu_agent_tasks_completed_total")
	AgentTasksFailed    = RegisterCounter("mu_agent_tasks_failed_total")
	AgentQueueDepth     = RegisterGauge("mu_agent_queue_depth")

	// 自进化
	EvolutionTriggered = RegisterCounter("mu_evolution_triggered_total")
	EvolutionSuccess   = RegisterCounter("mu_evolution_success_total")
	EvolutionFailed    = RegisterCounter("mu_evolution_failed_total")

	// AI 网关
	AICallsTotal  = RegisterCounter("mu_ai_calls_total")
	AICallsFailed = RegisterCounter("mu_ai_calls_failed_total")
	AICallLatency = RegisterHistogram("mu_ai_call_duration_ms")

	// 认证
	AuthLoginSuccess = RegisterCounter("mu_auth_login_success_total")
	AuthLoginFailed  = RegisterCounter("mu_auth_login_failed_total")
)

// ========== /metrics 端点 ==========

// Handler 返回 Prometheus 兼容的 /metrics HTTP handler
func Handler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")

		// Runtime 指标
		var mem runtime.MemStats
		runtime.ReadMemStats(&mem)
		fmt.Fprintf(w, "# HELP mu_runtime_goroutines Current goroutine count\n")
		fmt.Fprintf(w, "mu_runtime_goroutines %d\n", runtime.NumGoroutine())
		fmt.Fprintf(w, "mu_runtime_mem_alloc_bytes %d\n", mem.Alloc)
		fmt.Fprintf(w, "mu_runtime_mem_sys_bytes %d\n", mem.Sys)
		fmt.Fprintf(w, "mu_runtime_gc_runs_total %d\n", mem.NumGC)

		// 注册的 Counters
		mu.RLock()
		for name, c := range counters {
			fmt.Fprintf(w, "%s %d\n", name, c.Value())
		}
		for name, g := range gauges {
			fmt.Fprintf(w, "%s %d\n", name, g.Value())
		}
		for name, h := range histograms {
			fmt.Fprintf(w, "%s_count %d\n", name, h.Count())
			fmt.Fprintf(w, "%s_avg_ms %.2f\n", name, h.Avg())
			fmt.Fprintf(w, "%s_max_ms %.2f\n", name, h.Max())
		}
		mu.RUnlock()
	}
}

// Snapshot 返回所有指标的 JSON 快照
func Snapshot() map[string]interface{} {
	mu.RLock()
	defer mu.RUnlock()

	snap := make(map[string]interface{})
	snap["runtime_goroutines"] = runtime.NumGoroutine()

	var mem runtime.MemStats
	runtime.ReadMemStats(&mem)
	snap["runtime_mem_alloc_mb"] = float64(mem.Alloc) / 1024 / 1024
	snap["runtime_gc_runs"] = mem.NumGC

	for name, c := range counters {
		snap[name] = c.Value()
	}
	for name, g := range gauges {
		snap[name] = g.Value()
	}
	for name, h := range histograms {
		snap[name+"_count"] = h.Count()
		snap[name+"_avg_ms"] = h.Avg()
		snap[name+"_max_ms"] = h.Max()
	}
	return snap
}
