// Package plugin v1.7 插件生命周期管理增强
//
// 新增：Upgrade（无停机升级）/ ListWithHealth / ExportSnapshot
package plugin

import (
	"context"
	"fmt"
	"time"
)

// UpgradeResult 升级结果
type UpgradeResult struct {
	PluginID   string `json:"plugin_id"`
	OldVersion string `json:"old_version"`
	NewVersion string `json:"new_version"`
	Success    bool   `json:"success"`
	Message    string `json:"message"`
	Duration   int64  `json:"duration_ms"`
}

// Upgrade 无停机升级插件（安装新版→停旧版→启新版→健康检查→卸载旧版/回滚）
func (m *Manager) Upgrade(ctx context.Context, newPlugin Plugin) (*UpgradeResult, error) {
	meta := newPlugin.Meta()
	start := time.Now()
	result := &UpgradeResult{PluginID: meta.ID, NewVersion: meta.Version}

	m.mu.RLock()
	old, exists := m.plugins[meta.ID]
	m.mu.RUnlock()

	if exists {
		result.OldVersion = old.Meta.Version
		if old.Meta.Version == meta.Version {
			result.Message = "版本相同，无需升级"
			return result, nil
		}
	}

	if err := ValidateVersion(meta.MinFramework); err != nil {
		result.Message = err.Error()
		return result, err
	}

	if err := newPlugin.Init(ctx); err != nil {
		result.Message = fmt.Sprintf("新版本初始化失败: %v", err)
		return result, err
	}

	if exists && old.Status == StatusRunning {
		_ = old.Plugin.Stop()
	}

	if err := newPlugin.Start(); err != nil {
		if exists {
			_ = old.Plugin.Start()
		}
		result.Message = fmt.Sprintf("新版本启动失败，已回滚: %v", err)
		return result, err
	}

	health := newPlugin.Health()
	if !health.Healthy {
		_ = newPlugin.Stop()
		if exists {
			_ = old.Plugin.Start()
		}
		result.Message = "健康检查失败，已回滚: " + health.Message
		return result, fmt.Errorf("健康检查失败")
	}

	now := time.Now()
	m.mu.Lock()
	m.plugins[meta.ID] = &Instance{
		Plugin: newPlugin, Meta: meta, Status: StatusRunning,
		LoadedAt: now, StartedAt: &now,
	}
	m.mu.Unlock()

	result.Success = true
	result.Message = "升级成功"
	result.Duration = time.Since(start).Milliseconds()
	return result, nil
}

// PluginSnapshot 插件快照
type PluginSnapshot struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Version string `json:"version"`
	Status  Status `json:"status"`
	Healthy bool   `json:"healthy"`
	Message string `json:"message"`
}

// ListWithHealth 列出所有插件含实时健康
func (m *Manager) ListWithHealth() []PluginSnapshot {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := make([]PluginSnapshot, 0, len(m.plugins))
	for _, inst := range m.plugins {
		s := PluginSnapshot{
			ID: inst.Meta.ID, Name: inst.Meta.Name,
			Version: inst.Meta.Version, Status: inst.Status,
		}
		if inst.Status == StatusRunning {
			h := inst.Plugin.Health()
			s.Healthy = h.Healthy
			s.Message = h.Message
		}
		out = append(out, s)
	}
	return out
}

// ExportSnapshot 导出快照（灾备）
func (m *Manager) ExportSnapshot() map[string]interface{} {
	plugins := m.ListWithHealth()
	running := 0
	for _, p := range plugins {
		if p.Status == StatusRunning {
			running++
		}
	}
	return map[string]interface{}{
		"total":       len(plugins),
		"running":     running,
		"plugins":     plugins,
		"exported_at": time.Now(),
	}
}
