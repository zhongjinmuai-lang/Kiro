package engine

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/zhongjinmuai-lang/mu-framework/internal/agent/plugin"
	"github.com/zhongjinmuai-lang/mu-framework/internal/core/config"
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
	Type      string       `json:"type"`      // 任务类型
	Payload   string       `json:"payload"`   // 任务载荷（JSON）
	Priority  TaskPriority `json:"priority"`
	Status    TaskStatus   `json:"status"`
	Result    string       `json:"result"`
	Error     string       `json:"error"`
	CreatedAt time.Time    `json:"created_at"`
	StartedAt *time.Time   `json:"started_at"`
	DoneAt    *time.Time   `json:"done_at"`
}

// TaskHandler 任务处理函数
type TaskHandler func(ctx context.Context, task *Task) (string, error)

// Engine MU智能体调度引擎
type Engine struct {
	cfg           *config.Config
	pluginMgr     *plugin.Manager
	taskQueue     chan *Task
	handlers      map[string]TaskHandler
	workers       int
	logger        *slog.Logger
	ctx           context.Context
	cancel        context.CancelFunc
	wg            sync.WaitGroup
	mu            sync.RWMutex

	// 运行统计
	stats         *Stats
}

// Stats 引擎运行统计
type Stats struct {
	mu             sync.Mutex
	TotalTasks     int64     `json:"total_tasks"`
	CompletedTasks int64     `json:"completed_tasks"`
	FailedTasks    int64     `json:"failed_tasks"`
	ActiveWorkers  int       `json:"active_workers"`
	QueueSize      int       `json:"queue_size"`
	StartedAt      time.Time `json:"started_at"`
	Uptime         string    `json:"uptime"`
}

// New 创建智能体引擎
func New(cfg *config.Config) (*Engine, error) {
	ctx, cancel := context.WithCancel(context.Background())

	eng := &Engine{
		cfg:       cfg,
		pluginMgr: plugin.NewManager(),
		taskQueue: make(chan *Task, cfg.Agent.TaskQueueSize),
		handlers:  make(map[string]TaskHandler),
		workers:   cfg.Agent.MaxWorkers,
		logger:    slog.Default().With("module", "agent-engine"),
		ctx:       ctx,
		cancel:    cancel,
		stats: &Stats{
			StartedAt: time.Now(),
		},
	}

	return eng, nil
}

// Start 启动引擎
func (e *Engine) Start() error {
	e.logger.Info("智能体引擎启动中...", "workers", e.workers)

	// 启动工作协程池
	for i := 0; i < e.workers; i++ {
		e.wg.Add(1)
		go e.worker(i)
	}

	// 启动插件系统
	if err := e.pluginMgr.StartAll(); err != nil {
		e.logger.Error("插件系统启动异常", "error", err)
	}

	// 启动健康检查定时器
	e.wg.Add(1)
	go e.healthCheckLoop()

	e.logger.Info("智能体引擎已启动", "workers", e.workers, "queue_size", e.cfg.Agent.TaskQueueSize)
	return nil
}

// Stop 停止引擎
func (e *Engine) Stop() {
	e.logger.Info("正在停止智能体引擎...")
	e.cancel()
	e.wg.Wait()
	close(e.taskQueue)
	e.logger.Info("智能体引擎已停止")
}

// Submit 提交任务
func (e *Engine) Submit(task *Task) error {
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
		e.logger.Info("任务已提交", "task_id", task.ID, "type", task.Type, "priority", task.Priority)
		return nil
	default:
		return fmt.Errorf("任务队列已满（容量：%d）", e.cfg.Agent.TaskQueueSize)
	}
}

// RegisterHandler 注册任务处理器
func (e *Engine) RegisterHandler(taskType string, handler TaskHandler) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.handlers[taskType] = handler
	e.logger.Info("任务处理器已注册", "type", taskType)
}

// GetPluginManager 获取插件管理器
func (e *Engine) GetPluginManager() *plugin.Manager {
	return e.pluginMgr
}

// GetStats 获取引擎状态
func (e *Engine) GetStats() *Stats {
	e.stats.mu.Lock()
	defer e.stats.mu.Unlock()
	e.stats.Uptime = time.Since(e.stats.StartedAt).String()
	e.stats.QueueSize = len(e.taskQueue)
	return e.stats
}

// worker 工作协程
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

// executeTask 执行任务
func (e *Engine) executeTask(workerID int, task *Task) {
	e.mu.RLock()
	handler, exists := e.handlers[task.Type]
	e.mu.RUnlock()

	if !exists {
		task.Status = TaskFailed
		task.Error = fmt.Sprintf("未注册的任务类型: %s", task.Type)
		e.stats.mu.Lock()
		e.stats.FailedTasks++
		e.stats.mu.Unlock()
		return
	}

	// 执行任务
	now := time.Now()
	task.Status = TaskRunning
	task.StartedAt = &now

	e.stats.mu.Lock()
	e.stats.ActiveWorkers++
	e.stats.mu.Unlock()

	result, err := handler(e.ctx, task)

	e.stats.mu.Lock()
	e.stats.ActiveWorkers--
	e.stats.mu.Unlock()

	doneAt := time.Now()
	task.DoneAt = &doneAt

	if err != nil {
		task.Status = TaskFailed
		task.Error = err.Error()
		e.stats.mu.Lock()
		e.stats.FailedTasks++
		e.stats.mu.Unlock()
		e.logger.Error("任务执行失败",
			"task_id", task.ID,
			"worker", workerID,
			"error", err,
			"duration", doneAt.Sub(now).String(),
		)
	} else {
		task.Status = TaskCompleted
		task.Result = result
		e.stats.mu.Lock()
		e.stats.CompletedTasks++
		e.stats.mu.Unlock()
		e.logger.Info("任务执行完成",
			"task_id", task.ID,
			"worker", workerID,
			"duration", doneAt.Sub(now).String(),
		)
	}
}

// healthCheckLoop 健康检查循环
func (e *Engine) healthCheckLoop() {
	defer e.wg.Done()

	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-e.ctx.Done():
			return
		case <-ticker.C:
			results := e.pluginMgr.HealthCheck()
			for id, status := range results {
				if !status.Healthy {
					e.logger.Warn("插件健康检查异常", "plugin_id", id, "message", status.Message)
				}
			}
		}
	}
}

// fmt import needed
import "fmt"
