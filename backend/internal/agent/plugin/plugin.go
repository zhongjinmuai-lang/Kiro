// Package plugin MU 插件热插拔引擎（v2.3）
//
// 修复：
//   - startWithDeps 中 RLock→Lock 死锁风险 → 分离读取与操作阶段
//   - StartAll 改用 TopologicalSort 保证正确启动顺序
package plugin

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"
)

// Status 插件状态
type Status string

const (
	StatusLoaded    Status = "loaded"    // 已加载
	StatusRunning   Status = "running"   // 运行中
	StatusStopped   Status = "stopped"   // 已停止
	StatusError     Status = "error"     // 异常
	StatusUpgrading Status = "upgrading" // 升级中
)

// Meta 插件元信息
type Meta struct {
	ID           string   `json:"id"`
	Name         string   `json:"name"`
	Version      string   `json:"version"`
	Author       string   `json:"author"`
	Description  string   `json:"description"`
	Category     string   `json:"category"`
	Dependencies []string `json:"dependencies"`  // 依赖的其他插件ID
	MinFramework string   `json:"min_framework"` // 最低框架版本要求
}

// HealthStatus 健康状态
type HealthStatus struct {
	Healthy   bool      `json:"healthy"`
	Message   string    `json:"message"`
	CheckedAt time.Time `json:"checked_at"`
}

// Plugin 插件接口（所有插件必须实现）
type Plugin interface {
	Meta() Meta
	Init(ctx context.Context) error
	Start() error
	Stop() error
	Health() HealthStatus
}

// Instance 插件运行实例
type Instance struct {
	Plugin    Plugin     `json:"-"`
	Meta      Meta       `json:"meta"`
	Status    Status     `json:"status"`
	LoadedAt  time.Time  `json:"loaded_at"`
	StartedAt *time.Time `json:"started_at,omitempty"`
	Error     string     `json:"error,omitempty"`
}

// Manager 插件管理器（热插拔核心）
type Manager struct {
	mu      sync.RWMutex
	plugins map[string]*Instance // key: plugin ID
	logger  *slog.Logger
}

// NewManager 创建插件管理器
func NewManager() *Manager {
	return &Manager{
		plugins: make(map[string]*Instance),
		logger:  slog.Default().With("module", "plugin-manager"),
	}
}

// Install 安装插件（热加载）
func (m *Manager) Install(ctx context.Context, p Plugin) error {
	meta := p.Meta()

	// 框架版本兼容性校验
	if err := ValidateVersion(meta.MinFramework); err != nil {
		return fmt.Errorf("版本校验失败: %w", err)
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	// 检查是否已安装
	if _, exists := m.plugins[meta.ID]; exists {
		return fmt.Errorf("插件 %s 已安装", meta.ID)
	}

	// 检查依赖
	if err := m.checkDependencies(meta.Dependencies); err != nil {
		return fmt.Errorf("依赖检查失败: %w", err)
	}

	// 初始化插件
	if err := p.Init(ctx); err != nil {
		return fmt.Errorf("插件初始化失败: %w", err)
	}

	instance := &Instance{
		Plugin:   p,
		Meta:     meta,
		Status:   StatusLoaded,
		LoadedAt: time.Now(),
	}

	m.plugins[meta.ID] = instance
	m.logger.Info("插件安装成功", "id", meta.ID, "name", meta.Name, "version", meta.Version)
	return nil
}

// Uninstall 卸载插件（热拔出）
func (m *Manager) Uninstall(pluginID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	instance, exists := m.plugins[pluginID]
	if !exists {
		return fmt.Errorf("插件 %s 未安装", pluginID)
	}

	// 检查是否有其他插件依赖此插件
	for _, other := range m.plugins {
		for _, dep := range other.Meta.Dependencies {
			if dep == pluginID && other.Status == StatusRunning {
				return fmt.Errorf("插件 %s 被运行中的 %s 依赖，无法卸载", pluginID, other.Meta.ID)
			}
		}
	}

	// 停止运行中的插件
	if instance.Status == StatusRunning {
		if err := instance.Plugin.Stop(); err != nil {
			m.logger.Error("停止插件失败", "id", pluginID, "error", err)
		}
	}

	delete(m.plugins, pluginID)
	m.logger.Info("插件卸载成功", "id", pluginID)
	return nil
}

// Start 启动插件
func (m *Manager) Start(pluginID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.startLocked(pluginID)
}

// startLocked 内部启动（调用方须持有写锁）
func (m *Manager) startLocked(pluginID string) error {
	instance, exists := m.plugins[pluginID]
	if !exists {
		return fmt.Errorf("插件 %s 未安装", pluginID)
	}

	if instance.Status == StatusRunning {
		return nil // 已在运行
	}

	if err := instance.Plugin.Start(); err != nil {
		instance.Status = StatusError
		instance.Error = err.Error()
		return fmt.Errorf("启动插件失败: %w", err)
	}

	now := time.Now()
	instance.Status = StatusRunning
	instance.StartedAt = &now
	instance.Error = ""

	m.logger.Info("插件启动成功", "id", pluginID)
	return nil
}

// Stop 停止插件
func (m *Manager) Stop(pluginID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	instance, exists := m.plugins[pluginID]
	if !exists {
		return fmt.Errorf("插件 %s 未安装", pluginID)
	}

	if instance.Status != StatusRunning {
		return nil
	}

	if err := instance.Plugin.Stop(); err != nil {
		return fmt.Errorf("停止插件失败: %w", err)
	}

	instance.Status = StatusStopped
	m.logger.Info("插件停止成功", "id", pluginID)
	return nil
}

// StartAll 启动所有已加载的插件（使用拓扑排序确保依赖顺序）
func (m *Manager) StartAll() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// 收集所有已加载（非运行中）的插件
	var toStart []*Instance
	for _, inst := range m.plugins {
		if inst.Status != StatusRunning {
			toStart = append(toStart, inst)
		}
	}

	if len(toStart) == 0 {
		return nil
	}

	// 拓扑排序
	sorted, err := TopologicalSort(toStart)
	if err != nil {
		m.logger.Error("插件拓扑排序失败", "error", err)
		// 降级：按默认顺序启动
		sorted = toStart
	}

	// 按序启动
	var startErrors []string
	for _, inst := range sorted {
		if err := m.startLocked(inst.Meta.ID); err != nil {
			startErrors = append(startErrors, fmt.Sprintf("%s: %v", inst.Meta.ID, err))
			m.logger.Error("启动插件失败", "id", inst.Meta.ID, "error", err)
		}
	}

	if len(startErrors) > 0 {
		return fmt.Errorf("部分插件启动失败: %v", startErrors)
	}
	return nil
}

// Restart 重启插件
func (m *Manager) Restart(pluginID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	instance, exists := m.plugins[pluginID]
	if !exists {
		return fmt.Errorf("插件 %s 未安装", pluginID)
	}

	// 先停止
	if instance.Status == StatusRunning {
		if err := instance.Plugin.Stop(); err != nil {
			m.logger.Error("重启时停止插件失败", "id", pluginID, "error", err)
		}
		instance.Status = StatusStopped
	}

	// 再启动
	return m.startLocked(pluginID)
}

// GetAll 获取所有插件实例
func (m *Manager) GetAll() []*Instance {
	m.mu.RLock()
	defer m.mu.RUnlock()

	instances := make([]*Instance, 0, len(m.plugins))
	for _, inst := range m.plugins {
		instances = append(instances, inst)
	}
	return instances
}

// Get 获取指定插件
func (m *Manager) Get(pluginID string) (*Instance, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	inst, ok := m.plugins[pluginID]
	if !ok {
		return nil, fmt.Errorf("插件 %s 未安装", pluginID)
	}
	return inst, nil
}

// HealthCheck 健康检查
func (m *Manager) HealthCheck() map[string]HealthStatus {
	m.mu.RLock()
	defer m.mu.RUnlock()

	results := make(map[string]HealthStatus)
	for id, inst := range m.plugins {
		if inst.Status == StatusRunning {
			results[id] = inst.Plugin.Health()
		}
	}
	return results
}

// Stats 插件系统统计
func (m *Manager) Stats() map[string]interface{} {
	m.mu.RLock()
	defer m.mu.RUnlock()

	loaded, running, stopped, errored := 0, 0, 0, 0
	for _, inst := range m.plugins {
		switch inst.Status {
		case StatusLoaded:
			loaded++
		case StatusRunning:
			running++
		case StatusStopped:
			stopped++
		case StatusError:
			errored++
		}
	}
	return map[string]interface{}{
		"total":   len(m.plugins),
		"loaded":  loaded,
		"running": running,
		"stopped": stopped,
		"error":   errored,
	}
}

// checkDependencies 检查依赖是否满足（调用方须持有锁）
func (m *Manager) checkDependencies(deps []string) error {
	for _, dep := range deps {
		if _, exists := m.plugins[dep]; !exists {
			return fmt.Errorf("缺少依赖插件: %s", dep)
		}
	}
	return nil
}
