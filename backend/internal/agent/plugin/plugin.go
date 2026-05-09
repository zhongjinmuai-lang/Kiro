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
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Version     string   `json:"version"`
	Author      string   `json:"author"`
	Description string   `json:"description"`
	Category    string   `json:"category"`
	Dependencies []string `json:"dependencies"` // 依赖的其他插件ID
	MinFramework string  `json:"min_framework"` // 最低框架版本要求
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
	Plugin    Plugin       `json:"-"`
	Meta      Meta         `json:"meta"`
	Status    Status       `json:"status"`
	LoadedAt  time.Time    `json:"loaded_at"`
	StartedAt *time.Time   `json:"started_at,omitempty"`
	Error     string       `json:"error,omitempty"`
}

// Manager 插件管理器（热插拔核心）
type Manager struct {
	mu        sync.RWMutex
	plugins   map[string]*Instance // key: plugin ID
	logger    *slog.Logger
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
			if dep == pluginID {
				return fmt.Errorf("插件 %s 被 %s 依赖，无法卸载", pluginID, other.Meta.ID)
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

// StartAll 启动所有已加载的插件
func (m *Manager) StartAll() error {
	m.mu.RLock()
	ids := make([]string, 0, len(m.plugins))
	for id := range m.plugins {
		ids = append(ids, id)
	}
	m.mu.RUnlock()

	// 按依赖顺序启动
	started := make(map[string]bool)
	for _, id := range ids {
		if err := m.startWithDeps(id, started); err != nil {
			m.logger.Error("启动插件失败", "id", id, "error", err)
		}
	}
	return nil
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

// checkDependencies 检查依赖是否满足
func (m *Manager) checkDependencies(deps []string) error {
	for _, dep := range deps {
		if _, exists := m.plugins[dep]; !exists {
			return fmt.Errorf("缺少依赖插件: %s", dep)
		}
	}
	return nil
}

// startWithDeps 按依赖顺序启动
func (m *Manager) startWithDeps(id string, started map[string]bool) error {
	if started[id] {
		return nil
	}

	m.mu.RLock()
	inst, exists := m.plugins[id]
	m.mu.RUnlock()

	if !exists {
		return fmt.Errorf("插件 %s 不存在", id)
	}

	// 先启动依赖
	for _, dep := range inst.Meta.Dependencies {
		if err := m.startWithDeps(dep, started); err != nil {
			return err
		}
	}

	// 启动自己
	if err := m.Start(id); err != nil {
		return err
	}

	started[id] = true
	return nil
}
