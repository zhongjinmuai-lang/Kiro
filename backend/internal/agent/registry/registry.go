package registry

import (
	"fmt"
	"log/slog"
	"sync"
	"time"
)

// Capability 能力定义
type Capability struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Version     string    `json:"version"`
	Provider    string    `json:"provider"` // 提供方（插件ID）
	Category    string    `json:"category"` // 能力分类
	Description string    `json:"description"`
	Endpoint    string    `json:"endpoint"` // 调用端点
	Protocol    string    `json:"protocol"` // 协议：http / grpc / internal
	Status      string    `json:"status"`   // active / inactive / deprecated
	RegisterAt  time.Time `json:"register_at"`
}

// Registry 能力注册中心
// 管理所有插件和模块注册的能力，实现服务发现和调用路由
type Registry struct {
	mu           sync.RWMutex
	capabilities map[string]*Capability // key: capability ID
	byCategory   map[string][]string    // 按分类索引
	byProvider   map[string][]string    // 按提供方索引
	logger       *slog.Logger
}

// NewRegistry 创建能力注册中心
func NewRegistry() *Registry {
	return &Registry{
		capabilities: make(map[string]*Capability),
		byCategory:   make(map[string][]string),
		byProvider:   make(map[string][]string),
		logger:       slog.Default().With("module", "registry"),
	}
}

// Register 注册能力
func (r *Registry) Register(cap *Capability) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.capabilities[cap.ID]; exists {
		return fmt.Errorf("能力 %s 已注册", cap.ID)
	}

	cap.RegisterAt = time.Now()
	cap.Status = "active"
	r.capabilities[cap.ID] = cap

	// 更新索引
	r.byCategory[cap.Category] = append(r.byCategory[cap.Category], cap.ID)
	r.byProvider[cap.Provider] = append(r.byProvider[cap.Provider], cap.ID)

	r.logger.Info("能力注册成功", "id", cap.ID, "name", cap.Name, "provider", cap.Provider)
	return nil
}

// Deregister 注销能力
func (r *Registry) Deregister(capID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	cap, exists := r.capabilities[capID]
	if !exists {
		return fmt.Errorf("能力 %s 未注册", capID)
	}

	delete(r.capabilities, capID)

	// 清理索引
	r.removeFromSlice(r.byCategory, cap.Category, capID)
	r.removeFromSlice(r.byProvider, cap.Provider, capID)

	r.logger.Info("能力注销成功", "id", capID)
	return nil
}

// DeregisterByProvider 注销某提供方的所有能力
func (r *Registry) DeregisterByProvider(providerID string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	capIDs, exists := r.byProvider[providerID]
	if !exists {
		return
	}

	for _, id := range capIDs {
		if cap, ok := r.capabilities[id]; ok {
			r.removeFromSlice(r.byCategory, cap.Category, id)
			delete(r.capabilities, id)
		}
	}
	delete(r.byProvider, providerID)

	r.logger.Info("已注销提供方所有能力", "provider", providerID, "count", len(capIDs))
}

// Discover 发现能力（按分类查找）
func (r *Registry) Discover(category string) []*Capability {
	r.mu.RLock()
	defer r.mu.RUnlock()

	ids, exists := r.byCategory[category]
	if !exists {
		return nil
	}

	var caps []*Capability
	for _, id := range ids {
		if cap, ok := r.capabilities[id]; ok && cap.Status == "active" {
			caps = append(caps, cap)
		}
	}
	return caps
}

// Get 获取指定能力
func (r *Registry) Get(capID string) (*Capability, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	cap, exists := r.capabilities[capID]
	if !exists {
		return nil, fmt.Errorf("能力 %s 不存在", capID)
	}
	return cap, nil
}

// ListAll 列出所有能力
func (r *Registry) ListAll() []*Capability {
	r.mu.RLock()
	defer r.mu.RUnlock()

	caps := make([]*Capability, 0, len(r.capabilities))
	for _, cap := range r.capabilities {
		caps = append(caps, cap)
	}
	return caps
}

// Stats 注册中心统计
func (r *Registry) Stats() map[string]interface{} {
	r.mu.RLock()
	defer r.mu.RUnlock()

	categoryStats := make(map[string]int)
	for cat, ids := range r.byCategory {
		categoryStats[cat] = len(ids)
	}

	return map[string]interface{}{
		"total_capabilities": len(r.capabilities),
		"total_providers":    len(r.byProvider),
		"by_category":        categoryStats,
	}
}

func (r *Registry) removeFromSlice(m map[string][]string, key, value string) {
	slice, exists := m[key]
	if !exists {
		return
	}
	for i, v := range slice {
		if v == value {
			m[key] = append(slice[:i], slice[i+1:]...)
			break
		}
	}
}
