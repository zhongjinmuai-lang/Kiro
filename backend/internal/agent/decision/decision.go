// Package decision MU 智能体自主决策引擎（v2.3）
//
// 修复：
//   - 添加 sync.RWMutex 并发保护（goals/plans 读写安全）
//   - 步骤执行支持超时控制
//
// 实现 Goal → Plan → Action → Reflect 决策循环
// 智能体可基于目标自主规划、执行、反思并优化
package decision

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// GoalStatus 目标状态
type GoalStatus string

const (
	GoalPending   GoalStatus = "pending"   // 待规划
	GoalPlanning  GoalStatus = "planning"  // 规划中
	GoalExecuting GoalStatus = "executing" // 执行中
	GoalCompleted GoalStatus = "completed" // 已完成
	GoalFailed    GoalStatus = "failed"    // 失败
	GoalCancelled GoalStatus = "cancelled" // 取消
)

// Goal 目标
type Goal struct {
	ID          string     `json:"id"`
	Name        string     `json:"name"`
	Description string     `json:"description"`
	Priority    int        `json:"priority"` // 1-10，10 最高
	Status      GoalStatus `json:"status"`
	CreatedAt   time.Time  `json:"created_at"`
	CompletedAt *time.Time `json:"completed_at,omitempty"`
	Error       string     `json:"error,omitempty"`
}

// Plan 执行计划
type Plan struct {
	GoalID    string  `json:"goal_id"`
	Steps     []*Step `json:"steps"`
	CreatedAt time.Time `json:"created_at"`
}

// Step 计划步骤
type Step struct {
	ID        string                 `json:"id"`
	Name      string                 `json:"name"`
	Action    string                 `json:"action"` // 动作类型
	Params    map[string]interface{} `json:"params"`
	DependsOn []string               `json:"depends_on"` // 依赖的步骤 ID
	Timeout   time.Duration          `json:"timeout"`    // 步骤超时（0=不限）
	Status    string                 `json:"status"`     // pending/running/done/failed
	Result    string                 `json:"result"`
	StartedAt *time.Time             `json:"started_at,omitempty"`
	DoneAt    *time.Time             `json:"done_at,omitempty"`
}

// ActionExecutor 动作执行器接口
type ActionExecutor interface {
	Execute(ctx context.Context, action string, params map[string]interface{}) (string, error)
}

// Engine 决策引擎（并发安全）
type Engine struct {
	mu       sync.RWMutex
	goals    []*Goal
	plans    map[string]*Plan // goal_id -> plan
	executor ActionExecutor
}

// NewEngine 创建决策引擎
func NewEngine(executor ActionExecutor) *Engine {
	return &Engine{
		goals:    make([]*Goal, 0),
		plans:    make(map[string]*Plan),
		executor: executor,
	}
}

// SetGoal 设定目标（并发安全）
func (e *Engine) SetGoal(goal *Goal) {
	e.mu.Lock()
	defer e.mu.Unlock()

	goal.Status = GoalPending
	goal.CreatedAt = time.Now()
	e.goals = append(e.goals, goal)
}

// CancelGoal 取消目标
func (e *Engine) CancelGoal(goalID string) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	for _, g := range e.goals {
		if g.ID == goalID {
			if g.Status == GoalCompleted || g.Status == GoalCancelled {
				return fmt.Errorf("目标 %s 已处于终态 %s，无法取消", goalID, g.Status)
			}
			g.Status = GoalCancelled
			return nil
		}
	}
	return fmt.Errorf("目标不存在: %s", goalID)
}

// PlanGoal 为目标生成执行计划
func (e *Engine) PlanGoal(ctx context.Context, goalID string, steps []*Step) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	for _, g := range e.goals {
		if g.ID == goalID {
			if g.Status != GoalPending {
				return fmt.Errorf("目标 %s 状态为 %s，仅 pending 可规划", goalID, g.Status)
			}
			g.Status = GoalPlanning
			// 初始化步骤状态
			for _, s := range steps {
				s.Status = "pending"
			}
			e.plans[goalID] = &Plan{
				GoalID:    goalID,
				Steps:     steps,
				CreatedAt: time.Now(),
			}
			return nil
		}
	}
	return fmt.Errorf("目标不存在: %s", goalID)
}

// ExecutePlan 按计划执行（按依赖顺序）
func (e *Engine) ExecutePlan(ctx context.Context, goalID string) error {
	e.mu.RLock()
	plan, ok := e.plans[goalID]
	e.mu.RUnlock()

	if !ok {
		return fmt.Errorf("无计划: %s", goalID)
	}

	// 更新目标状态
	e.mu.Lock()
	for _, g := range e.goals {
		if g.ID == goalID {
			g.Status = GoalExecuting
			break
		}
	}
	e.mu.Unlock()

	// 按顺序执行步骤（TODO：生产应按 DependsOn 拓扑排序并行执行）
	for _, step := range plan.Steps {
		select {
		case <-ctx.Done():
			e.setGoalStatus(goalID, GoalCancelled, "上下文取消")
			return ctx.Err()
		default:
		}

		now := time.Now()
		step.Status = "running"
		step.StartedAt = &now

		// 步骤级超时控制
		var stepCtx context.Context
		var cancel context.CancelFunc
		if step.Timeout > 0 {
			stepCtx, cancel = context.WithTimeout(ctx, step.Timeout)
		} else {
			stepCtx, cancel = context.WithCancel(ctx)
		}

		result, err := e.executor.Execute(stepCtx, step.Action, step.Params)
		cancel()

		done := time.Now()
		step.DoneAt = &done

		if err != nil {
			step.Status = "failed"
			step.Result = err.Error()
			e.setGoalStatus(goalID, GoalFailed, fmt.Sprintf("步骤 %s 失败: %v", step.Name, err))
			return fmt.Errorf("步骤 %s 执行失败: %w", step.Name, err)
		}
		step.Status = "done"
		step.Result = result
	}

	// 标记目标完成
	e.setGoalStatus(goalID, GoalCompleted, "")
	return nil
}

// Reflect 反思：分析执行结果，生成经验教训
func (e *Engine) Reflect(goalID string) string {
	e.mu.RLock()
	plan, ok := e.plans[goalID]
	e.mu.RUnlock()

	if !ok {
		return "无计划可反思"
	}

	total := len(plan.Steps)
	done := 0
	failed := 0
	var failedSteps []string
	for _, s := range plan.Steps {
		switch s.Status {
		case "done":
			done++
		case "failed":
			failed++
			failedSteps = append(failedSteps, s.Name)
		}
	}

	if failed == 0 {
		return fmt.Sprintf("目标 %s 执行成功（%d/%d 步骤完成），无需调整", goalID, done, total)
	}
	return fmt.Sprintf("目标 %s 部分失败（%d 成功 / %d 失败），失败步骤: %v，建议重试或调整策略",
		goalID, done, failed, failedSteps)
}

// ListGoals 列出所有目标
func (e *Engine) ListGoals() []*Goal {
	e.mu.RLock()
	defer e.mu.RUnlock()
	result := make([]*Goal, len(e.goals))
	copy(result, e.goals)
	return result
}

// GetGoal 获取单个目标
func (e *Engine) GetGoal(goalID string) *Goal {
	e.mu.RLock()
	defer e.mu.RUnlock()
	for _, g := range e.goals {
		if g.ID == goalID {
			return g
		}
	}
	return nil
}

// GetPlan 获取计划
func (e *Engine) GetPlan(goalID string) *Plan {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.plans[goalID]
}

// Stats 决策引擎统计
func (e *Engine) Stats() map[string]interface{} {
	e.mu.RLock()
	defer e.mu.RUnlock()

	pending, executing, completed, failed := 0, 0, 0, 0
	for _, g := range e.goals {
		switch g.Status {
		case GoalPending, GoalPlanning:
			pending++
		case GoalExecuting:
			executing++
		case GoalCompleted:
			completed++
		case GoalFailed:
			failed++
		}
	}
	return map[string]interface{}{
		"total_goals": len(e.goals),
		"pending":     pending,
		"executing":   executing,
		"completed":   completed,
		"failed":      failed,
		"total_plans": len(e.plans),
	}
}

// ========== 内部方法 ==========

// setGoalStatus 设置目标状态（内部使用，加锁）
func (e *Engine) setGoalStatus(goalID string, status GoalStatus, errMsg string) {
	e.mu.Lock()
	defer e.mu.Unlock()
	for _, g := range e.goals {
		if g.ID == goalID {
			g.Status = status
			g.Error = errMsg
			if status == GoalCompleted {
				now := time.Now()
				g.CompletedAt = &now
			}
			break
		}
	}
}
