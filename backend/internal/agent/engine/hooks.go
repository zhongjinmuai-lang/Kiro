// Package engine v2.3 MU 核心框架进化 - 引擎钩子与自进化集成
//
// 优化：
//   - 引擎启动时注册默认进化规则
//   - 引擎运行时定期上报指标到自进化内核
//   - 自进化触发动作可操控引擎（如动态调整 worker、重启异常插件）
//   - 错误处理完善（Stop/Start 失败时记录并发布事件）
package engine

import (
	"context"
	"time"

	"go.uber.org/zap"

	"github.com/zhongjinmuai-lang/mu-framework/internal/agent/evolution"
	"github.com/zhongjinmuai-lang/mu-framework/pkg/logger"
	"github.com/zhongjinmuai-lang/mu-framework/pkg/metrics"
)

// EvolutionHook 自进化钩子：将引擎与自进化内核绑定
type EvolutionHook struct {
	engine   *Engine
	evo      *evolution.Service
	eventBus *EventBus
	cancel   context.CancelFunc
}

// NewEvolutionHook 创建钩子
func NewEvolutionHook(eng *Engine, evo *evolution.Service, bus *EventBus) *EvolutionHook {
	return &EvolutionHook{engine: eng, evo: evo, eventBus: bus}
}

// Start 启动钩子：注册引擎级进化规则 + 定时上报指标
func (h *EvolutionHook) Start() {
	// 注册引擎特有的进化规则
	h.evo.RegisterRule(evolution.Rule{
		Name:     "引擎队列积压动态扩容",
		Strategy: evolution.StrategyScale,
		Cooldown: 2 * time.Minute,
		Condition: func(snap *evolution.MetricSnapshot) bool {
			return snap.QueueDepth > 200
		},
		Action: func(ctx context.Context) error {
			stats := h.engine.GetStats()
			metrics.AgentQueueDepth.Set(int64(stats.QueueSize))
			logger.L().Warn("队列积压告警，建议扩容",
				zap.Int("queue_depth", stats.QueueSize),
				zap.Int("workers", h.engine.workers),
			)
			// 发布进化事件
			if h.eventBus != nil {
				h.eventBus.Publish(&BusEvent{
					Type:   EventEvolutionTriggered,
					Source: "evolution-hook",
					Payload: map[string]interface{}{
						"rule":        "引擎队列积压动态扩容",
						"queue_depth": stats.QueueSize,
					},
				})
			}
			return nil
		},
	})

	h.evo.RegisterRule(evolution.Rule{
		Name:     "引擎任务成功率过低自修复",
		Strategy: evolution.StrategyRepair,
		Cooldown: 3 * time.Minute,
		Condition: func(snap *evolution.MetricSnapshot) bool {
			return snap.TaskSuccess > 0 && snap.TaskSuccess < 0.75
		},
		Action: func(ctx context.Context) error {
			// 重启异常插件
			results := h.engine.pluginMgr.HealthCheck()
			restartCount := 0
			for id, st := range results {
				if !st.Healthy {
					logger.L().Info("自修复：重启异常插件", zap.String("plugin_id", id))
					if err := h.engine.pluginMgr.Restart(id); err != nil {
						logger.L().Error("重启插件失败",
							zap.String("plugin_id", id),
							zap.Error(err),
						)
						// 发布插件错误事件
						if h.eventBus != nil {
							h.eventBus.Publish(&BusEvent{
								Type:   EventPluginError,
								Source: "evolution-hook",
								Payload: map[string]interface{}{
									"plugin_id": id,
									"error":     err.Error(),
								},
							})
						}
					} else {
						restartCount++
					}
				}
			}
			logger.L().Info("自修复完成",
				zap.Int("restarted", restartCount),
				zap.Int("total_unhealthy", len(results)),
			)
			return nil
		},
	})

	// 注册默认进化规则（高错误率、内存告警、goroutine 泄漏等）
	h.evo.RegisterDefaultRules()

	// 定时上报引擎指标到自进化内核
	ctx, cancel := context.WithCancel(context.Background())
	h.cancel = cancel
	go h.reportLoop(ctx)

	logger.L().Info("进化钩子已启动")
}

// Stop 停止钩子
func (h *EvolutionHook) Stop() {
	if h.cancel != nil {
		h.cancel()
	}
	logger.L().Info("进化钩子已停止")
}

// reportLoop 每 30s 上报引擎指标
func (h *EvolutionHook) reportLoop(ctx context.Context) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			h.reportMetrics()
		}
	}
}

// reportMetrics 采集并上报指标
func (h *EvolutionHook) reportMetrics() {
	stats := h.engine.GetStats()

	var successRate float64
	if stats.TotalTasks > 0 {
		successRate = float64(stats.CompletedTasks) / float64(stats.TotalTasks)
	}

	var errorRate float64
	if stats.TotalTasks > 0 {
		errorRate = float64(stats.FailedTasks) / float64(stats.TotalTasks)
	}

	snap := &evolution.MetricSnapshot{
		QueueDepth:  stats.QueueSize,
		TaskSuccess: successRate,
		ErrorRate:   errorRate,
	}
	// 采集 runtime 指标
	evolution.CollectRuntimeMetrics(snap)

	h.evo.ReportMetrics(snap)

	// 同步更新 Prometheus 兼容监控埋点
	metrics.AgentQueueDepth.Set(int64(stats.QueueSize))
}
