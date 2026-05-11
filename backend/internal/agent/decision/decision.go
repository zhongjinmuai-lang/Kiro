// Package decision MU 智能体自主决策引擎（v2.0）
//
// 实现 Goal → Plan → Action → Reflect 决策循环
// 智能体可基于目标自主规划、执行、反思并优化
package decision

import (
	"context"
	"fmt"
	"time"
)

// GoalStatus 目标状态
type GoalStatus string

const (
	GoalPending    GoalStatus = "pending"     // 待规划
	GoalPlanning   GoalStatus = "planning"    // 规划中
	GoalExecuting  GoalStatus = "executing"   // 执行中
	GoalCompleted  GoalStatus = "completed"   // 已完成
	GoalFailed     GoalStatus = "failed"      // 失败
	GoalCancelled  GoalStatus = "cancelled"   // 取消
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
}

// Plan 执行计划
type Plan struct {
	GoalID string  `json:"goal_id"`
	Steps  []*Step `json:"steps"`
}

// Step 计划步骤
type Step struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Action      string `json:"action"`      // 动作类型
	Params      map[string]interface{} `json:"params"`
	DependsOn   []string `json:"depends_on"` // 依赖的步骤 ID
	Status      string `json:"status"`       // pending/running/done/failed
	Result      string `json:"result"`
}

// ActionExecutor 动作执行器接口
type ActionExecutor interface {
	Execute(ctx context.Context, action string, params map[string]interface{}) (string, error)
}

// Engine 决策引擎
type Engine struct {
	goals     []*Goal
	plans     map[string]*Plan // goal_id -> plan
	executor  ActionExecutor
}

// NewEngine 创建决策引擎
func NewEngine(executor ActionExecutor) *Engine {
	return &Engine{
		plans:    make(map[string]*Plan),
		executor: executor,
	}
}

// SetGoal 设定目标
func (e *Engine) SetGoal(goal *Goal) {
	goal.Status = GoalPending
	goal.CreatedAt = time.Now()
	e.goals = append(e.goals, goal)
}

// PlanGoal 为目标生成执行计划
// planner: 外部规划器（可接入 AI 大模型做计划生成）
func (e *Engine) PlanGoal(ctx context.Context, goalID string, steps []*Step) error {
	for _, g := range e.goals {
		if g.ID == goalID {
			g.Status = GoalPlanning
			e.plans[goalID] = &Plan{GoalID: goalID, Steps: steps}
			return nil
		}
	}
	return fmt.Errorf("目标不存在: %s", goalID)
}

// ExecutePlan 按计划执行（按依赖顺序）
func (e *Engine) ExecutePlan(ctx context.Context, goalID string) error {
	plan, ok := e.plans[goalID]
	if !ok {
		return fmt.Errorf("无计划: %s", goalID)
	}

	// 更新目标状态
	for _, g := range e.goals {
		if g.ID == goalID {
			g.Status = GoalExecuting
			break
		}
	}

	// 简化：按顺序执行（生产应按 DependsOn 拓扑排序）
	for _, step := range plan.Steps {
		step.Status = "running"
		result, err := e.executor.Execute(ctx, step.Action, step.Params)
		if err != nil {
			step.Status = "failed"
			step.Result = err.Error()
			// 标记目标失败
			for _, g := range e.goals {
				if g.ID == goalID {
					g.Status = GoalFailed
					break
				}
			}
			return fmt.Errorf("步骤 %s 执行失败: %w", step.Name, err)
		}
		step.Status = "done"
		step.Result = result
	}

	// 标记目标完成
	now := time.Now()
	for _, g := range e.goals {
		if g.ID == goalID {
			g.Status = GoalCompleted
			g.CompletedAt = &now
			break
		}
	}
	return nil
}

// Reflect 反思：分析执行结果，生成经验教训
// 返回建议（可存入记忆系统）
func (e *Engine) Reflect(goalID string) string {
	plan, ok := e.plans[goalID]
	if !ok {
		return "无计划可反思"
	}

	total := len(plan.Steps)
	done := 0
	failed := 0
	for _, s := range plan.Steps {
		switch s.Status {
		case "done":
			done++
		case "failed":
			failed++
		}
	}

	if failed == 0 {
		return fmt.Sprintf("目标 %s 执行成功（%d/%d 步骤完成），无需调整", goalID, done, total)
	}
	return fmt.Sprintf("目标 %s 部分失败（%d 成功 / %d 失败），建议重试或调整策略", goalID, done, failed)
}

// ListGoals 列出所有目标
func (e *Engine) ListGoals() []*Goal {
	return e.goals
}

// GetPlan 获取计划
func (e *Engine) GetPlan(goalID string) *Plan {
	return e.plans[goalID]
}

// Stats 决策引擎统计
func (e *Engine) Stats() map[string]interface{} {
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
	}
}
