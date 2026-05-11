// Package agent MU 智能体协调器（v2.3）
//
// 将引擎、记忆、决策、进化、事件总线统一集成管理
// 提供智能体的完整生命周期管理
package agent

import (
	"context"
	"fmt"

	"go.uber.org/zap"

	"github.com/zhongjinmuai-lang/mu-framework/internal/agent/decision"
	"github.com/zhongjinmuai-lang/mu-framework/internal/agent/engine"
	"github.com/zhongjinmuai-lang/mu-framework/internal/agent/evolution"
	"github.com/zhongjinmuai-lang/mu-framework/internal/agent/memory"
	"github.com/zhongjinmuai-lang/mu-framework/internal/agent/registry"
	"github.com/zhongjinmuai-lang/mu-framework/internal/core/config"
	"github.com/zhongjinmuai-lang/mu-framework/pkg/logger"
)

// Coordinator 智能体协调器：统一管理所有智能体子系统
type Coordinator struct {
	Engine    *engine.Engine
	Memory    *memory.Manager
	Decision  *decision.Engine
	Evolution *evolution.Service
	EventBus  *engine.EventBus
	Registry  *registry.Registry

	hook *engine.EvolutionHook
	cfg  *config.Config
}

// NewCoordinator 创建智能体协调器
func NewCoordinator(cfg *config.Config) (*Coordinator, error) {
	if cfg == nil {
		return nil, fmt.Errorf("配置不能为空")
	}

	// 1. 创建事件总线（最先，其他组件需要它）
	eventBus := engine.NewEventBus()

	// 2. 创建调度引擎
	eng, err := engine.New(cfg)
	if err != nil {
		return nil, fmt.Errorf("创建调度引擎失败: %w", err)
	}

	// 3. 创建记忆系统
	memStore := memory.NewInMemoryStore()
	memMgr := memory.NewManager(memStore)

	// 4. 创建决策引擎（使用默认执行器）
	decisionEngine := decision.NewEngine(&defaultActionExecutor{})

	// 5. 创建自进化服务
	evolutionCycle := 60 // 默认 60 秒
	if cfg.Agent.EvolutionCycleSec > 0 {
		evolutionCycle = cfg.Agent.EvolutionCycleSec
	}
	evoService := evolution.NewService(evolutionCycle)

	// 6. 创建能力注册中心
	reg := registry.NewRegistry()

	// 7. 创建进化钩子（连接引擎与进化服务）
	hook := engine.NewEvolutionHook(eng, evoService, eventBus)

	return &Coordinator{
		Engine:    eng,
		Memory:    memMgr,
		Decision:  decisionEngine,
		Evolution: evoService,
		EventBus:  eventBus,
		Registry:  reg,
		hook:      hook,
		cfg:       cfg,
	}, nil
}

// Start 启动所有智能体子系统
func (c *Coordinator) Start() error {
	logger.L().Info("智能体协调器启动中...")

	// 1. 启动进化服务
	c.Evolution.Start()

	// 2. 启动调度引擎
	if err := c.Engine.Start(); err != nil {
		return fmt.Errorf("启动调度引擎失败: %w", err)
	}

	// 3. 启动进化钩子
	c.hook.Start()

	// 4. 注册内置任务处理器
	c.registerBuiltinHandlers()

	logger.L().Info("智能体协调器已启动",
		zap.String("version", "v2.3"),
		zap.Int("workers", c.cfg.Agent.MaxWorkers),
		zap.Int("evolution_cycle_sec", c.cfg.Agent.EvolutionCycleSec),
	)
	return nil
}

// Stop 优雅停止所有子系统
func (c *Coordinator) Stop() {
	logger.L().Info("智能体协调器停止中...")

	// 反向停止
	c.hook.Stop()
	c.Engine.Stop()
	c.Evolution.Stop()

	logger.L().Info("智能体协调器已停止")
}

// Stats 返回所有子系统统计
func (c *Coordinator) Stats() map[string]interface{} {
	return map[string]interface{}{
		"engine":    c.Engine.GetStats(),
		"evolution": c.Evolution.GetStats(),
		"decision":  c.Decision.Stats(),
		"registry":  c.Registry.Stats(),
		"plugins":   c.Engine.GetPluginManager().Stats(),
	}
}

// registerBuiltinHandlers 注册内置任务处理器
func (c *Coordinator) registerBuiltinHandlers() {
	// 记忆存储任务
	c.Engine.RegisterHandler("memory.save", func(ctx context.Context, task *engine.Task) (string, error) {
		return "记忆已存储", c.Memory.Remember(ctx, task.Name, task.Payload, nil)
	})

	// 记忆检索任务
	c.Engine.RegisterHandler("memory.recall", func(ctx context.Context, task *engine.Task) (string, error) {
		entries, err := c.Memory.Recall(ctx, task.Payload, 5)
		if err != nil {
			return "", err
		}
		return fmt.Sprintf("找到 %d 条相关记忆", len(entries)), nil
	})

	// 健康检查任务
	c.Engine.RegisterHandler("system.health", func(ctx context.Context, task *engine.Task) (string, error) {
		stats := c.Stats()
		return fmt.Sprintf("系统正常: %v", stats), nil
	})

	logger.L().Info("内置任务处理器已注册", zap.Int("count", 3))
}

// ========== 默认动作执行器 ==========

// defaultActionExecutor 默认动作执行器（决策引擎使用）
type defaultActionExecutor struct{}

func (e *defaultActionExecutor) Execute(ctx context.Context, action string, params map[string]interface{}) (string, error) {
	// 默认实现：记录动作并返回成功
	// 生产环境应对接具体的动作实现（AI调用、API调用等）
	logger.L().Info("执行决策动作",
		zap.String("action", action),
		zap.Any("params", params),
	)
	return fmt.Sprintf("动作 %s 已执行", action), nil
}
