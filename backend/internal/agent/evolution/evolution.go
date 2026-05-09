package evolution

import (
	"context"
	"log/slog"
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
	Target    string    `json:"target"`    // 目标组件
	Trigger   string    `json:"trigger"`   // 触发原因
	Action    string    `json:"action"`    // 执行动作
	Result    string    `json:"result"`
	Success   bool      `json:"success"`
	CreatedAt time.Time `json:"created_at"`
}

// MetricSnapshot 指标快照
type MetricSnapshot struct {
	Timestamp    time.Time `json:"timestamp"`
	CPUUsage     float64   `json:"cpu_usage"`
	MemUsage     float64   `json:"mem_usage"`
	TaskSuccess  float64   `json:"task_success_rate"`
	AvgLatency   float64   `json:"avg_latency_ms"`
	ErrorRate    float64   `json:"error_rate"`
	QueueDepth   int       `json:"queue_depth"`
}

// Rule 进化规则
type Rule struct {
	Name      string   `json:"name"`
	Condition func(snapshot *MetricSnapshot) bool
	Strategy  Strategy `json:"strategy"`
	Action    func(ctx context.Context) error
}

// Service 自进化服务
// 实现自进化、自升级、自修复三大能力
type Service struct {
	rules    []Rule
	history  []*Event
	metrics  []*MetricSnapshot
	logger   *slog.Logger
	mu       sync.Mutex

	// 配置
	cycleInterval time.Duration // 进化周期
	maxHistory    int           // 最大历史记录数

	ctx    context.Context
	cancel context.CancelFunc
}

// NewService 创建自进化服务
func NewService(cycleSec int) *Service {
	ctx, cancel := context.WithCancel(context.Background())
	return &Service{
		rules:         make([]Rule, 0),
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
	s.rules = append(s.rules, rule)
	s.logger.Info("进化规则已注册", "name", rule.Name, "strategy", rule.Strategy)
}

// Start 启动自进化循环
func (s *Service) Start() {
	go s.evolutionLoop()
	s.logger.Info("自进化服务已启动", "cycle", s.cycleInterval.String())
}

// Stop 停止自进化服务
func (s *Service) Stop() {
	s.cancel()
	s.logger.Info("自进化服务已停止")
}

// ReportMetrics 上报指标快照
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

// evaluate 评估当前状态并触发进化
func (s *Service) evaluate() {
	s.mu.Lock()
	snapshot := s.latestMetric()
	rules := make([]Rule, len(s.rules))
	copy(rules, s.rules)
	s.mu.Unlock()

	if snapshot == nil {
		return
	}

	for _, rule := range rules {
		if rule.Condition(snapshot) {
			s.logger.Info("进化条件触发", "rule", rule.Name, "strategy", rule.Strategy)
			s.executeEvolution(rule, snapshot)
		}
	}
}

// executeEvolution 执行进化动作
func (s *Service) executeEvolution(rule Rule, snapshot *MetricSnapshot) {
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

// latestMetric 获取最新指标
func (s *Service) latestMetric() *MetricSnapshot {
	if len(s.metrics) == 0 {
		return nil
	}
	return s.metrics[len(s.metrics)-1]
}

// DefaultRules 默认进化规则
func DefaultRules() []Rule {
	return []Rule{
		{
			Name:     "高错误率自修复",
			Strategy: StrategyRepair,
			Condition: func(s *MetricSnapshot) bool {
				return s.ErrorRate > 0.1 // 错误率超过10%
			},
			Action: func(ctx context.Context) error {
				// TODO: 执行自修复逻辑（重启异常组件、切换备用链路等）
				slog.Info("执行自修复：重启异常组件")
				return nil
			},
		},
		{
			Name:     "队列积压自伸缩",
			Strategy: StrategyScale,
			Condition: func(s *MetricSnapshot) bool {
				return s.QueueDepth > 500 // 队列深度超过500
			},
			Action: func(ctx context.Context) error {
				// TODO: 执行自动扩容
				slog.Info("执行自伸缩：增加工作协程")
				return nil
			},
		},
		{
			Name:     "高延迟优化",
			Strategy: StrategyOptimize,
			Condition: func(s *MetricSnapshot) bool {
				return s.AvgLatency > 2000 // 平均延迟超过2秒
			},
			Action: func(ctx context.Context) error {
				// TODO: 执行优化（调整并发参数、启用缓存等）
				slog.Info("执行优化：调整性能参数")
				return nil
			},
		},
	}
}

func generateID() string {
	return time.Now().Format("20060102150405") + "-" + randomStr(8)
}

func randomStr(n int) string {
	const chars = "abcdefghijklmnopqrstuvwxyz0123456789"
	b := make([]byte, n)
	for i := range b {
		b[i] = chars[time.Now().UnixNano()%int64(len(chars))]
		time.Sleep(1) // 简单随机
	}
	return string(b)
}
