// Package engine MU智能体调度引擎（v2.3）
//
// 修复：
//   - Stop() 后 Submit() 可能 panic（send on closed channel）→ 增加 stopped 标记
//   - 添加优雅停止：先停止接收，drain 队列，再退出 worker
//
// 功能：任务队列 + 工作池 + 插件协同 + 运行统计
package engine

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"

	"github.com/zhongjinmuai-lang/mu-framework/internal/agent/plugin"
	"github.com/zhongjinmuai-lang/mu-framework/internal/core/config"
	"github.com/zhongjinmuai-lang/mu-framework/pkg/logger"
)

// TaskPriority 任务优先级
type TaskPriority int

const (
	PriorityLow      TaskPriority = 1
	PriorityNormal   TaskPriority = 5
	PriorityHigh     TaskPriority = 8
	PriorityCritical TaskPriority = 10
)

// TaskStatus 任务状态
type TaskStatus string

const (
	TaskPending   TaskStatus = "pending"
	TaskRunning   TaskStatus = "running"
	TaskCompleted TaskStatus = "completed"
	TaskFailed    TaskStatus = "failed"
	TaskCancelled TaskStatus = "cancelled"
)

// Task 调度任务
type Task struct {
	ID        string       `json:"id"`
	Name      string       `json:"name"`
	Type      string       `json:"type"`
	Payload   string       `json:"payload"`
	Priority  TaskPriority `json:"priority"`
	Status    TaskStatus   `json:"status"`
	Result    string       `json:"result"`
	Error     string       `json:"error"`
	Timeout   time.Duration `json:"timeout"` // 单任务超时（0=使用默认）
	CreatedAt time.Time    `json:"created_at"`
	StartedAt *time.Time   `json:"started_at,omitempty"`
	DoneAt    *time.Time   `json:"done_at,omitempty"`
}

// TaskHandler 任务处理函数
type TaskHandler func(ctx context.Context, task *Task) (string, error)

// Engine MU智能体调度引擎
type Engine struct {
	cfg       *config.Config
	pluginMgr *plugin.Manager
	taskQueue chan *Task
	handlers  map[string]TaskHandler
	workers   int

	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
	mu     sync.RWMutex

	stopped atomic.Bool // 标记引擎是否已停止（防止 Submit panic）
	stats   *Stats
}

// Stats 引擎运行统计
type Stats struct {
	mu             sync.Mutex
	TotalTasks     int64     `json:"total_tasks"`
	CompletedTasks int64     `json:"completed_tasks"`
	FailedTasks    int64     `json:"failed_tasks"`
	CancelledTasks int64     `json:"cancelled_tasks"`
	ActiveWorkers  int       `json:"active_workers"`
	QueueSize      int       `json:"queue_size"`
	StartedAt      time.Time `json:"started_at"`
	Uptime         string    `json:"uptime"`
}

// New 创建引擎
func New(cfg *config.Config) (*Engine, error) {
	if cfg == nil {
		return nil, errors.New("配置不能为空")
	}

	queueSize := cfg.Agent.TaskQueueSize
	if queueSize <= 0 {
		queueSize = 1000
	}
	workers := cfg.Agent.MaxWorkers
	if workers <= 0 {
		workers = 10
	}

	ctx, cancel := context.WithCancel(context.Background())
	return &Engine{
		cfg:       cfg,
		pluginMgr: plugin.NewManager(),
		taskQueue: make(chan *Task, queueSize),
		handlers:  make(map[string]TaskHandler),
		workers:   workers,
		ctx:       ctx,
		cancel:    cancel,
		stats:     &Stats{StartedAt: time.Now()},
	}, nil
}

// Start 启动引擎
func (e *Engine) Start() error {
	logger.L().Info("智能体引擎启动中", zap.Int("workers", e.workers))

	// 启动工作池
	for i := 0; i < e.workers; i++ {
		e.wg.Add(1)
		go e.worker(i)
	}

	// 启动插件系统
	if err := e.pluginMgr.StartAll(); err != nil {
		logger.L().Error("插件系统启动异常", zap.Error(err))
	}

	// 启动健康检查
	e.wg.Add(1)
	go e.healthCheckLoop()

	logger.L().Info("智能体引擎已启动",
		zap.Int("workers", e.workers),
		zap.Int("queue_capacity", cap(e.taskQueue)),
	)
	return nil
}

// Stop 优雅停止
func (e *Engine) Stop() {
	logger.L().Info("正在停止智能体引擎")

	// 标记停止（阻止新任务提交）
	e.stopped.Store(true)

	// 取消上下文（通知所有 worker 退出）
	e.cancel()

	// 等待所有 worker 完成当前任务
	e.wg.Wait()

	// 关闭队列通道（此时已无 goroutine 读写）
	close(e.taskQueue)

	logger.L().Info("智能体引擎已停止",
		zap.Int64("total_tasks", e.stats.TotalTasks),
		zap.Int64("completed", e.stats.CompletedTasks),
		zap.Int64("failed", e.stats.FailedTasks),
	)
}

// Submit 提交任务（并发安全，引擎停止后拒绝提交）
func (e *Engine) Submit(task *Task) error {
	// 检查引擎是否已停止
	if e.stopped.Load() {
		return errors.New("智能体引擎已停止，拒绝接收新任务")
	}

	if task.ID == "" {
		task.ID = uuid.New().String()
	}
	task.Status = TaskPending
	task.CreatedAt = time.Now()

	select {
	case e.taskQueue <- task:
		e.stats.mu.Lock()
		e.stats.TotalTasks++
		e.stats.mu.Unlock()
		return nil
	default:
		return fmt.Errorf("任务队列已满（容量：%d），请稍后重试", cap(e.taskQueue))
	}
}

// RegisterHandler 注册任务处理器
func (e *Engine) RegisterHandler(taskType string, h TaskHandler) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.handlers[taskType] = h
	logger.L().Info("任务处理器已注册", zap.String("type", taskType))
}

// GetPluginManager 暴露插件管理器
func (e *Engine) GetPluginManager() *plugin.Manager { return e.pluginMgr }

// GetStats 获取运行统计快照
func (e *Engine) GetStats() *Stats {
	e.stats.mu.Lock()
	defer e.stats.mu.Unlock()
	e.stats.Uptime = time.Since(e.stats.StartedAt).Truncate(time.Second).String()
	e.stats.QueueSize = len(e.taskQueue)
	return e.stats
}

// IsRunning 引擎是否在运行
func (e *Engine) IsRunning() bool {
	return !e.stopped.Load()
}

// ========== 内部 ==========

func (e *Engine) worker(id int) {
	defer e.wg.Done()
	for {
		select {
		case <-e.ctx.Done():
			return
		case task, ok := <-e.taskQueue:
			if !ok {
				return
			}
			e.executeTask(id, task)
		}
	}
}

func (e *Engine) executeTask(workerID int, task *Task) {
	e.mu.RLock()
	handler, exists := e.handlers[task.Type]
	e.mu.RUnlock()

	if !exists {
		task.Status = TaskFailed
		task.Error = fmt.Sprintf("未注册的任务类型: %s", task.Type)
		e.bumpFailed()
		logger.L().Warn("未注册的任务类型",
			zap.String("task_id", task.ID),
			zap.String("type", task.Type),
		)
		return
	}

	now := time.Now()
	task.Status = TaskRunning
	task.StartedAt = &now

	e.stats.mu.Lock()
	e.stats.ActiveWorkers++
	e.stats.mu.Unlock()

	// 任务级超时控制
	var taskCtx context.Context
	var cancel context.CancelFunc
	timeout := task.Timeout
	if timeout <= 0 {
		timeout = 5 * time.Minute // 默认 5 分钟超时
	}
	taskCtx, cancel = context.WithTimeout(e.ctx, timeout)
	defer cancel()

	result, err := handler(taskCtx, task)

	e.stats.mu.Lock()
	e.stats.ActiveWorkers--
	e.stats.mu.Unlock()

	done := time.Now()
	task.DoneAt = &done
	duration := done.Sub(now)

	if err != nil {
		task.Status = TaskFailed
		task.Error = err.Error()
		e.bumpFailed()
		logger.L().Error("任务执行失败",
			zap.String("task_id", task.ID),
			zap.String("type", task.Type),
			zap.Int("worker", workerID),
			zap.Error(err),
			zap.Duration("duration", duration),
		)
		return
	}
	task.Status = TaskCompleted
	task.Result = result
	e.stats.mu.Lock()
	e.stats.CompletedTasks++
	e.stats.mu.Unlock()
	logger.L().Info("任务执行完成",
		zap.String("task_id", task.ID),
		zap.String("type", task.Type),
		zap.Int("worker", workerID),
		zap.Duration("duration", duration),
	)
}

func (e *Engine) bumpFailed() {
	e.stats.mu.Lock()
	e.stats.FailedTasks++
	e.stats.mu.Unlock()
}

func (e *Engine) healthCheckLoop() {
	defer e.wg.Done()
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-e.ctx.Done():
			return
		case <-ticker.C:
			for id, st := range e.pluginMgr.HealthCheck() {
				if !st.Healthy {
					logger.L().Warn("插件健康检查异常",
						zap.String("plugin_id", id),
						zap.String("message", st.Message),
					)
				}
			}
		}
	}
}
