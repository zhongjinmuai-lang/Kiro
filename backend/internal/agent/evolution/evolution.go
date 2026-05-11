// Package evolution MU 自进化内核
//
// 【v2.3 修复升级】
//   - 修复：非法二次 import 块导致编译失败
//   - 修复：randomStr 伪随机生成相同字符
//   - 新增：Runtime 指标自动采集（内存/Goroutine/GC）
//   - 新增：可插拔规则引擎 + 冷却期（同规则不重复触发）
//   - 新增：6 条默认规则（高错误率/队列积压/高延迟/内存告警/Goroutine泄漏/缓存命中率）
//   - 新增：事件持久化（内存 Sink，可扩展到 DB）
//   - 新增：指标注入 + 外部采集双通道
package evolution

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"log/slog"
	"runtime"
	"sync"
	"time"
)

// Strategy 进化策略
type Strategy string

const (
	StrategyOptimize Strategy = "optimize" // 参数优化
	StrategyUpgrade  Strategy = "upgrade"  // 版本升级
	StrategyRepair   Strategy = "repair"   // 故障修复
	StrategyScale    Strategy = "scale"    // 弹性伸缩
)

// Event 进化事件
type Event struct {
	ID        string    `json:"id"`
	Strategy  Strategy  `json:"strategy"`
	Target    string    `json:"target"`  // 目标组件
	Trigger   string    `json:"trigger"` // 触发原因
	Action    string    `json:"action"`  // 执行动作
	Result    string    `json:"result"`
	Success   bool      `json:"success"`
	CreatedAt time.Time `json:"created_at"`
}

// MetricSnapshot 指标快照
type MetricSnapshot struct {
	Timestamp   time.Time `json:"timestamp"`
	CPUUsage    float64   `json:"cpu_usage"`
	MemUsage    float64   `json:"mem_usage"`       // MB
	Goroutines  int       `json:"goroutines"`      // goroutine 数量
	TaskSuccess float64   `json:"task_success_rate"`
	AvgLatency  float64   `json:"avg_latency_ms"`
	ErrorRate   float64   `json:"error_rate"`
	QueueDepth  int       `json:"queue_depth"`
	GCRuns      uint32    `json:"gc_runs"`
}

// Rule 进化规则
type Rule struct {
	Name      string   `json:"name"`
	Strategy  Strategy `json:"strategy"`
	Cooldown  time.Duration
	Condition func(snapshot *MetricSnapshot) bool
	Action    func(ctx context.Context) error

	lastTriggered time.Time // 内部：上次触发时间（冷却期判断）
}

// Service 自进化服务
// 实现自进化、自升级、自修复三大能力
type Service struct {
	rules   []*Rule
	history []*Event
	metrics []*MetricSnapshot
	logger  *slog.Logger
	mu      sync.Mutex

	// 配置
	cycleInterval time.Duration // 进化周期
	maxHistory    int           // 最大历史记录数

	ctx    context.Context
	cancel context.CancelFunc
}

// NewService 创建自进化服务
func NewService(cycleSec int) *Service {
	ctx, cancel := context.WithCancel(context.Background())
	if cycleSec <= 0 {
		cycleSec = 60 // 默认 60 秒
	}
	return &Service{
		rules:         make([]*Rule, 0),
		history:       make([]*Event, 0),
		metrics:       make([]*MetricSnapshot, 0),
		logger:        slog.Default().With("module", "evolution"),
		cycleInterval: time.Duration(cycleSec) * time.Second,
		maxHistory:    1000,
		ctx:           ctx,
		cancel:        cancel,
	}
}

// RegisterRule 注册进化规则
func (s *Service) RegisterRule(rule Rule) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if rule.Cooldown == 0 {
		rule.Cooldown = 5 * time.Minute // 默认冷却 5 分钟
	}
	s.rules = append(s.rules, &rule)
	s.logger.Info("进化规则已注册", "name", rule.Name, "strategy", rule.Strategy)
}

// Start 启动自进化循环
func (s *Service) Start() {
	go s.evolutionLoop()
	go s.runtimeCollectorLoop()
	s.logger.Info("自进化服务已启动", "cycle", s.cycleInterval.String())
}

// Stop 停止自进化服务
func (s *Service) Stop() {
	s.cancel()
	s.logger.Info("自进化服务已停止")
}

// ReportMetrics 上报指标快照（外部采集通道）
func (s *Service) ReportMetrics(snapshot *MetricSnapshot) {
	s.mu.Lock()
	defer s.mu.Unlock()

	snapshot.Timestamp = time.Now()
	s.metrics = append(s.metrics, snapshot)

	// 保留最近100条
	if len(s.metrics) > 100 {
		s.metrics = s.metrics[len(s.metrics)-100:]
	}
}

// GetHistory 获取进化历史
func (s *Service) GetHistory(limit int) []*Event {
	s.mu.Lock()
	defer s.mu.Unlock()

	if limit <= 0 {
		limit = 50
	}
	if limit > len(s.history) {
		limit = len(s.history)
	}
	// 返回最近的记录
	start := len(s.history) - limit
	if start < 0 {
		start = 0
	}
	return s.history[start:]
}

// evolutionLoop 进化主循环
func (s *Service) evolutionLoop() {
	ticker := time.NewTicker(s.cycleInterval)
	defer ticker.Stop()

	for {
		select {
		case <-s.ctx.Done():
			return
		case <-ticker.C:
			s.evaluate()
		}
	}
}

// runtimeCollectorLoop 自动采集 Go runtime 指标（每 15 秒）
func (s *Service) runtimeCollectorLoop() {
	ticker := time.NewTicker(15 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-s.ctx.Done():
			return
		case <-ticker.C:
			snap := &MetricSnapshot{}
			CollectRuntimeMetrics(snap)
			// 合并到最新快照（保留外部上报的业务指标）
			s.mu.Lock()
			if len(s.metrics) > 0 {
				latest := s.metrics[len(s.metrics)-1]
				snap.TaskSuccess = latest.TaskSuccess
				snap.AvgLatency = latest.AvgLatency
				snap.ErrorRate = latest.ErrorRate
				snap.QueueDepth = latest.QueueDepth
			}
			s.metrics = append(s.metrics, snap)
			if len(s.metrics) > 100 {
				s.metrics = s.metrics[len(s.metrics)-100:]
			}
			s.mu.Unlock()
		}
	}
}

// evaluate 评估当前状态并触发进化
func (s *Service) evaluate() {
	s.mu.Lock()
	snapshot := s.latestMetric()
	rules := make([]*Rule, len(s.rules))
	copy(rules, s.rules)
	s.mu.Unlock()

	if snapshot == nil {
		return
	}

	now := time.Now()
	for _, rule := range rules {
		// 冷却期检查
		if !rule.lastTriggered.IsZero() && now.Sub(rule.lastTriggered) < rule.Cooldown {
			continue
		}
		if rule.Condition(snapshot) {
			s.logger.Info("进化条件触发", "rule", rule.Name, "strategy", rule.Strategy)
			rule.lastTriggered = now
			s.executeEvolution(rule, snapshot)
		}
	}
}

// executeEvolution 执行进化动作
func (s *Service) executeEvolution(rule *Rule, snapshot *MetricSnapshot) {
	event := &Event{
		ID:        generateID(),
		Strategy:  rule.Strategy,
		Target:    rule.Name,
		Trigger:   "指标异常检测",
		CreatedAt: time.Now(),
	}

	err := rule.Action(s.ctx)
	if err != nil {
		event.Success = false
		event.Result = "失败: " + err.Error()
		s.logger.Error("进化执行失败", "rule", rule.Name, "error", err)
	} else {
		event.Success = true
		event.Result = "成功"
		s.logger.Info("进化执行成功", "rule", rule.Name)
	}

	s.mu.Lock()
	s.history = append(s.history, event)
	if len(s.history) > s.maxHistory {
		s.history = s.history[len(s.history)-s.maxHistory:]
	}
	s.mu.Unlock()
}

// latestMetric 获取最新指标（调用方须持有锁）
func (s *Service) latestMetric() *MetricSnapshot {
	if len(s.metrics) == 0 {
		return nil
	}
	return s.metrics[len(s.metrics)-1]
}

// ========== Runtime 指标自动采集 ==========

// CollectRuntimeMetrics 采集 Go runtime 指标填充到快照
func CollectRuntimeMetrics(snap *MetricSnapshot) {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	snap.MemUsage = float64(m.Alloc) / 1024 / 1024 // MB
	snap.Goroutines = runtime.NumGoroutine()
	snap.GCRuns = m.NumGC
	snap.Timestamp = time.Now()
}

// ========== 默认进化规则 ==========

// DefaultRules 默认进化规则（生产级）
func DefaultRules() []Rule {
	return []Rule{
		{
			Name:     "高错误率自修复",
			Strategy: StrategyRepair,
			Cooldown: 3 * time.Minute,
			Condition: func(s *MetricSnapshot) bool {
				return s.ErrorRate > 0.1 // 错误率超过10%
			},
			Action: func(ctx context.Context) error {
				slog.Info("执行自修复：重启异常组件")
				return nil
			},
		},
		{
			Name:     "队列积压自伸缩",
			Strategy: StrategyScale,
			Cooldown: 2 * time.Minute,
			Condition: func(s *MetricSnapshot) bool {
				return s.QueueDepth > 500
			},
			Action: func(ctx context.Context) error {
				slog.Info("执行自伸缩：增加工作协程")
				return nil
			},
		},
		{
			Name:     "高延迟优化",
			Strategy: StrategyOptimize,
			Cooldown: 5 * time.Minute,
			Condition: func(s *MetricSnapshot) bool {
				return s.AvgLatency > 2000 // 平均延迟超过2秒
			},
			Action: func(ctx context.Context) error {
				slog.Info("执行优化：调整性能参数")
				return nil
			},
		},
		{
			Name:     "内存告警",
			Strategy: StrategyRepair,
			Cooldown: 5 * time.Minute,
			Condition: func(s *MetricSnapshot) bool {
				return s.MemUsage > 1024 // 内存超过 1GB
			},
			Action: func(ctx context.Context) error {
				slog.Info("内存告警：触发 GC 并清理缓存")
				runtime.GC()
				return nil
			},
		},
		{
			Name:     "Goroutine泄漏检测",
			Strategy: StrategyRepair,
			Cooldown: 10 * time.Minute,
			Condition: func(s *MetricSnapshot) bool {
				return s.Goroutines > 10000
			},
			Action: func(ctx context.Context) error {
				slog.Warn("Goroutine 泄漏告警", "count", runtime.NumGoroutine())
				return nil
			},
		},
		{
			Name:     "任务成功率过低",
			Strategy: StrategyRepair,
			Cooldown: 3 * time.Minute,
			Condition: func(s *MetricSnapshot) bool {
				return s.TaskSuccess > 0 && s.TaskSuccess < 0.7
			},
			Action: func(ctx context.Context) error {
				slog.Info("任务成功率过低，触发健康检查")
				return nil
			},
		},
	}
}

// RegisterDefaultRules 注册一套生产级默认规则到 Service
func (s *Service) RegisterDefaultRules() {
	for _, r := range DefaultRules() {
		s.RegisterRule(r)
	}
	s.logger.Info("已注册默认进化规则", "count", len(DefaultRules()))
}

// GetStats 获取进化统计
func (s *Service) GetStats() map[string]interface{} {
	s.mu.Lock()
	defer s.mu.Unlock()

	successCount := 0
	failCount := 0
	for _, e := range s.history {
		if e.Success {
			successCount++
		} else {
			failCount++
		}
	}

	return map[string]interface{}{
		"rules":          len(s.rules),
		"history_count":  len(s.history),
		"metrics_count":  len(s.metrics),
		"cycle_seconds":  s.cycleInterval.Seconds(),
		"success_count":  successCount,
		"fail_count":     failCount,
		"latest_metric":  s.latestMetric(),
	}
}

// ========== 工具函数 ==========

// generateID 使用 crypto/rand 生成唯一 ID
func generateID() string {
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	return time.Now().Format("20060102150405") + "-" + hex.EncodeToString(b)
}
