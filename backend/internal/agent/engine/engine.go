// Package engine MU智能体调度引擎：任务队列 + 工作池 + 插件协同 + 运行统计
package engine

import (
	"context"
	"errors"
	"fmt"
	"sync"
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

	stats *Stats
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

// New 创建引擎
func New(cfg *config.Config) (*Engine, error) {
	if cfg == nil {
		return nil, errors.New("配置不能为空")
	}
	ctx, cancel := context.WithCancel(context.Background())
	return &Engine{
		cfg:       cfg,
		pluginMgr: plugin.NewManager(),
		taskQueue: make(chan *Task, cfg.Agent.TaskQueueSize),
		handlers:  make(map[string]TaskHandler),
		workers:   cfg.Agent.MaxWorkers,
		ctx:       ctx,
		cancel:    cancel,
		stats:     &Stats{StartedAt: time.Now()},
	}, nil
}

// Start 启动引擎
func (e *Engine) Start() error {
	logger.L().Info("智能体引擎启动中", zap.Int("workers", e.workers))

	for i := 0; i < e.workers; i++ {
		e.wg.Add(1)
		go e.worker(i)
	}
	if err := e.pluginMgr.StartAll(); err != nil {
		logger.L().Error("插件系统启动异常", zap.Error(err))
	}

	e.wg.Add(1)
	go e.healthCheckLoop()

	logger.L().Info("智能体引擎已启动",
		zap.Int("workers", e.workers),
		zap.Int("queue_size", e.cfg.Agent.TaskQueueSize),
	)
	return nil
}

// Stop 优雅停止
func (e *Engine) Stop() {
	logger.L().Info("正在停止智能体引擎")
	e.cancel()
	e.wg.Wait()
	close(e.taskQueue)
	logger.L().Info("智能体引擎已停止")
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
		return nil
	default:
		return fmt.Errorf("任务队列已满（容量：%d）", e.cfg.Agent.TaskQueueSize)
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
	e.stats.Uptime = time.Since(e.stats.StartedAt).String()
	e.stats.QueueSize = len(e.taskQueue)
	return e.stats
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
		return
	}

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

	done := time.Now()
	task.DoneAt = &done
	if err != nil {
		task.Status = TaskFailed
		task.Error = err.Error()
		e.bumpFailed()
		logger.L().Error("任务执行失败",
			zap.String("task_id", task.ID),
			zap.Int("worker", workerID),
			zap.Error(err),
			zap.Duration("duration", done.Sub(now)),
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
		zap.Int("worker", workerID),
		zap.Duration("duration", done.Sub(now)),
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
