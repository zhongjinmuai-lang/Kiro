// Package engine v1.6 MU 核心框架进化 - 引擎钩子与自进化集成
//
// 将 Agent Engine 与 Evolution 自进化内核关联：
//   - 引擎启动时注册默认进化规则
//   - 引擎运行时定期上报指标到自进化内核
//   - 自进化触发动作可直接操控引擎（如动态调整 worker 数量）
package engine

import (
	"context"
	"time"

	"github.com/zhongjinmuai-lang/mu-framework/internal/agent/evolution"
	"github.com/zhongjinmuai-lang/mu-framework/pkg/metrics"
)

// EvolutionHook 自进化钩子：将引擎与自进化内核绑定
type EvolutionHook struct {
	engine *Engine
	evo    *evolution.Service
	cancel context.CancelFunc
}

// NewEvolutionHook 创建钩子
func NewEvolutionHook(eng *Engine, evo *evolution.Service) *EvolutionHook {
	return &EvolutionHook{engine: eng, evo: evo}
}

// Start 启动钩子：注册引擎级进化规则 + 定时上报指标
func (h *EvolutionHook) Start() {
	// 注册引擎特有的进化规则
	h.evo.RegisterRule(evolution.Rule{
		Name:     "引擎队列积压动态扩容",
		Strategy: evolution.StrategyScale,
		Condition: func(snap *evolution.MetricSnapshot) bool {
			return snap.QueueDepth > 200
		},
		Action: func(ctx context.Context) error {
			// 动态增加 worker（实际生产可调 K8s HPA 或增加 goroutine）
			metrics.AgentQueueDepth.Set(int64(len(h.engine.taskQueue)))
			return nil
		},
	})

	h.evo.RegisterRule(evolution.Rule{
		Name:     "引擎任务成功率过低",
		Strategy: evolution.StrategyRepair,
		Condition: func(snap *evolution.MetricSnapshot) bool {
			return snap.TaskSuccess > 0 && snap.TaskSuccess < 0.8
		},
		Action: func(ctx context.Context) error {
			// 重启异常插件
			results := h.engine.pluginMgr.HealthCheck()
			for id, st := range results {
				if !st.Healthy {
					_ = h.engine.pluginMgr.Stop(id)
					_ = h.engine.pluginMgr.Start(id)
				}
			}
			return nil
		},
	})

	// 定时上报引擎指标到自进化内核
	ctx, cancel := context.WithCancel(context.Background())
	h.cancel = cancel
	go h.reportLoop(ctx)
}

// Stop 停止钩子
func (h *EvolutionHook) Stop() {
	if h.cancel != nil {
		h.cancel()
	}
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
			stats := h.engine.GetStats()
			var successRate float64
			if stats.TotalTasks > 0 {
				successRate = float64(stats.CompletedTasks) / float64(stats.TotalTasks)
			}
			h.evo.ReportMetrics(&evolution.MetricSnapshot{
				QueueDepth:  stats.QueueSize,
				TaskSuccess: successRate,
			})

			// 同步更新监控埋点
			metrics.AgentQueueDepth.Set(int64(stats.QueueSize))
		}
	}
}
