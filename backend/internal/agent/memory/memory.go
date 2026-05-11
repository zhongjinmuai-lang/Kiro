// Package memory MU 智能体记忆系统（v2.0）
//
// 支持短期记忆（请求级）和长期记忆（跨会话持久化）
// 为智能体自主决策、上下文推理、经验学习提供记忆底座
package memory

import (
	"context"
	"sync"
	"time"
)

// MemoryType 记忆类型
type MemoryType string

const (
	ShortTerm MemoryType = "short_term" // 短期（请求/会话级，自动过期）
	LongTerm  MemoryType = "long_term"  // 长期（跨会话持久化）
	Working   MemoryType = "working"    // 工作记忆（当前任务上下文）
)

// Entry 记忆条目
type Entry struct {
	ID        string            `json:"id"`
	Type      MemoryType        `json:"type"`
	Key       string            `json:"key"`
	Content   string            `json:"content"`
	Metadata  map[string]string `json:"metadata,omitempty"`
	Score     float64           `json:"score"`      // 相关性分数（检索用）
	CreatedAt time.Time         `json:"created_at"`
	ExpiresAt *time.Time        `json:"expires_at,omitempty"` // nil=永不过期
	AccessCnt int64             `json:"access_count"`
}

// Store 记忆存储接口
type Store interface {
	Save(ctx context.Context, entry *Entry) error
	Get(ctx context.Context, key string) (*Entry, error)
	Search(ctx context.Context, query string, limit int) ([]*Entry, error)
	Delete(ctx context.Context, key string) error
	List(ctx context.Context, memType MemoryType, limit int) ([]*Entry, error)
}

// InMemoryStore 内存实现（开发/测试用）
type InMemoryStore struct {
	mu      sync.RWMutex
	entries map[string]*Entry
}

// NewInMemoryStore 创建内存存储
func NewInMemoryStore() *InMemoryStore {
	return &InMemoryStore{entries: make(map[string]*Entry)}
}

// Save 保存
func (s *InMemoryStore) Save(ctx context.Context, e *Entry) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if e.CreatedAt.IsZero() {
		e.CreatedAt = time.Now()
	}
	s.entries[e.Key] = e
	return nil
}

// Get 获取
func (s *InMemoryStore) Get(ctx context.Context, key string) (*Entry, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if e, ok := s.entries[key]; ok {
		if e.ExpiresAt != nil && time.Now().After(*e.ExpiresAt) {
			return nil, nil // 已过期
		}
		e.AccessCnt++
		return e, nil
	}
	return nil, nil
}

// Search 简单关键词搜索（生产应接入向量数据库）
func (s *InMemoryStore) Search(ctx context.Context, query string, limit int) ([]*Entry, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var results []*Entry
	for _, e := range s.entries {
		if e.ExpiresAt != nil && time.Now().After(*e.ExpiresAt) {
			continue
		}
		// 简单包含匹配（生产用向量相似度）
		if contains(e.Content, query) || contains(e.Key, query) {
			results = append(results, e)
			if len(results) >= limit {
				break
			}
		}
	}
	return results, nil
}

// Delete 删除
func (s *InMemoryStore) Delete(ctx context.Context, key string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.entries, key)
	return nil
}

// List 按类型列出
func (s *InMemoryStore) List(ctx context.Context, memType MemoryType, limit int) ([]*Entry, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var results []*Entry
	for _, e := range s.entries {
		if e.Type == memType {
			results = append(results, e)
			if len(results) >= limit {
				break
			}
		}
	}
	return results, nil
}

// Manager 记忆管理器
type Manager struct {
	store Store
}

// NewManager 创建管理器
func NewManager(store Store) *Manager {
	if store == nil {
		store = NewInMemoryStore()
	}
	return &Manager{store: store}
}

// Remember 记住（存入长期记忆）
func (m *Manager) Remember(ctx context.Context, key, content string, meta map[string]string) error {
	return m.store.Save(ctx, &Entry{
		Type:    LongTerm,
		Key:     key,
		Content: content,
		Metadata: meta,
	})
}

// Recall 回忆（搜索记忆）
func (m *Manager) Recall(ctx context.Context, query string, limit int) ([]*Entry, error) {
	if limit <= 0 {
		limit = 10
	}
	return m.store.Search(ctx, query, limit)
}

// Forget 遗忘
func (m *Manager) Forget(ctx context.Context, key string) error {
	return m.store.Delete(ctx, key)
}

// SetWorking 设置工作记忆（当前任务上下文）
func (m *Manager) SetWorking(ctx context.Context, key, content string) error {
	exp := time.Now().Add(30 * time.Minute)
	return m.store.Save(ctx, &Entry{
		Type:      Working,
		Key:       "working:" + key,
		Content:   content,
		ExpiresAt: &exp,
	})
}

// GetWorking 获取工作记忆
func (m *Manager) GetWorking(ctx context.Context, key string) (*Entry, error) {
	return m.store.Get(ctx, "working:"+key)
}

func contains(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || len(sub) == 0 ||
		(len(s) > 0 && len(sub) > 0 && stringContains(s, sub)))
}

func stringContains(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
