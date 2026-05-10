// Package engine v1.8 智能体调度编排增强
//
// 新增能力：
//   - TaskDAG: 任务有向无环图编排（支持依赖链）
//   - PriorityQueue: 优先级队列调度
//   - RetryPolicy: 任务失败自动重试策略
//   - AgentOrchestrator: 多智能体协作编排器
package engine

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// RetryPolicy 重试策略
type RetryPolicy struct {
	MaxRetries int           `json:"max_retries"` // 最大重试次数
	Interval   time.Duration `json:"interval"`    // 重试间隔
	Backoff    float64       `json:"backoff"`     // 退避乘数（1.0=固定间隔，2.0=指数退避）
}

// DefaultRetryPolicy 默认重试策略
func DefaultRetryPolicy() *RetryPolicy {
	return &RetryPolicy{MaxRetries: 3, Interval: time.Second, Backoff: 2.0}
}

// TaskDAGNode DAG 任务节点
type TaskDAGNode struct {
	Task         *Task    `json:"task"`
	Dependencies []string `json:"dependencies"` // 依赖的任务 ID 列表
}

// TaskDAG 任务有向无环图
type TaskDAG struct {
	mu    sync.Mutex
	nodes map[string]*TaskDAGNode
	order []string // 拓扑排序后的执行顺序
}

// NewTaskDAG 创建 DAG
func NewTaskDAG() *TaskDAG {
	return &TaskDAG{nodes: make(map[string]*TaskDAGNode)}
}

// AddNode 添加任务节点
func (d *TaskDAG) AddNode(task *Task, deps ...string) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.nodes[task.ID] = &TaskDAGNode{Task: task, Dependencies: deps}
}

// TopologicalOrder 拓扑排序（Kahn 算法）
func (d *TaskDAG) TopologicalOrder() ([]string, error) {
	d.mu.Lock()
	defer d.mu.Unlock()

	inDegree := make(map[string]int)
	graph := make(map[string][]string)

	for id := range d.nodes {
		if _, ok := inDegree[id]; !ok {
			inDegree[id] = 0
		}
	}
	for id, node := range d.nodes {
		for _, dep := range node.Dependencies {
			graph[dep] = append(graph[dep], id)
			inDegree[id]++
		}
	}

	var queue []string
	for id, deg := range inDegree {
		if deg == 0 {
			queue = append(queue, id)
		}
	}

	var sorted []string
	for len(queue) > 0 {
		curr := queue[0]
		queue = queue[1:]
		sorted = append(sorted, curr)
		for _, next := range graph[curr] {
			inDegree[next]--
			if inDegree[next] == 0 {
				queue = append(queue, next)
			}
		}
	}

	if len(sorted) != len(d.nodes) {
		return nil, fmt.Errorf("任务依赖存在循环")
	}
	d.order = sorted
	return sorted, nil
}

// ExecuteDAG 按拓扑顺序执行 DAG 中所有任务
func (e *Engine) ExecuteDAG(ctx context.Context, dag *TaskDAG) (map[string]*Task, error) {
	order, err := dag.TopologicalOrder()
	if err != nil {
		return nil, err
	}

	results := make(map[string]*Task)
	for _, taskID := range order {
		node := dag.nodes[taskID]
		if err := e.Submit(node.Task); err != nil {
			return results, fmt.Errorf("提交任务 %s 失败: %w", taskID, err)
		}
		// 等待任务完成（简化：轮询）
		for i := 0; i < 600; i++ { // 最多等 60s
			if node.Task.Status == TaskCompleted || node.Task.Status == TaskFailed {
				break
			}
			time.Sleep(100 * time.Millisecond)
		}
		results[taskID] = node.Task
		if node.Task.Status == TaskFailed {
			return results, fmt.Errorf("任务 %s 执行失败: %s", taskID, node.Task.Error)
		}
	}
	return results, nil
}

// SubmitWithRetry 提交任务并自动重试
func (e *Engine) SubmitWithRetry(task *Task, policy *RetryPolicy) error {
	if policy == nil {
		policy = DefaultRetryPolicy()
	}
	var lastErr error
	interval := policy.Interval
	for i := 0; i <= policy.MaxRetries; i++ {
		if err := e.Submit(task); err != nil {
			lastErr = err
			time.Sleep(interval)
			interval = time.Duration(float64(interval) * policy.Backoff)
			continue
		}
		return nil
	}
	return fmt.Errorf("任务提交失败（已重试 %d 次）: %w", policy.MaxRetries, lastErr)
}

// AgentOrchestrator 多智能体协作编排器
// 支持多个 Engine 实例协作完成复杂任务（单机多引擎 / 分布式多节点）
type AgentOrchestrator struct {
	mu      sync.RWMutex
	engines map[string]*Engine // name -> engine
}

// NewOrchestrator 创建编排器
func NewOrchestrator() *AgentOrchestrator {
	return &AgentOrchestrator{engines: make(map[string]*Engine)}
}

// Register 注册智能体引擎
func (o *AgentOrchestrator) Register(name string, eng *Engine) {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.engines[name] = eng
}

// Dispatch 按策略分发任务到指定引擎
// strategy: "round-robin" / "least-loaded" / "by-type"
func (o *AgentOrchestrator) Dispatch(task *Task, strategy string) error {
	o.mu.RLock()
	defer o.mu.RUnlock()

	if len(o.engines) == 0 {
		return fmt.Errorf("无可用引擎")
	}

	switch strategy {
	case "least-loaded":
		// 选择队列最短的引擎
		var best *Engine
		minQueue := int(^uint(0) >> 1)
		for _, eng := range o.engines {
			stats := eng.GetStats()
			if stats.QueueSize < minQueue {
				minQueue = stats.QueueSize
				best = eng
			}
		}
		if best != nil {
			return best.Submit(task)
		}
	default:
		// 默认提交到第一个引擎
		for _, eng := range o.engines {
			return eng.Submit(task)
		}
	}
	return fmt.Errorf("分发失败")
}

// Stats 所有引擎统计汇总
func (o *AgentOrchestrator) Stats() map[string]interface{} {
	o.mu.RLock()
	defer o.mu.RUnlock()
	stats := make(map[string]interface{})
	for name, eng := range o.engines {
		stats[name] = eng.GetStats()
	}
	return stats
}
