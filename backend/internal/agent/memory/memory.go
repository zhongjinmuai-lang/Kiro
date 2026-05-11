// Package memory MU 智能体记忆系统（v2.3）
//
// 修复：
//   - RLock 下写入 AccessCnt 导致数据竞争 → 改用 atomic
//   - 过期条目不清理导致内存泄漏 → 添加后台 GC
//
// 支持短期记忆（请求级）和长期记忆（跨会话持久化）
// 为智能体自主决策、上下文推理、经验学习提供记忆底座
package memory

import (
	"context"
	"strings"
	"sync"
	"sync/atomic"
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
	AccessCnt int64             `json:"access_count"`         // 使用 atomic 操作
}

// Store 记忆存储接口
type Store interface {
	Save(ctx context.Context, entry *Entry) error
	Get(ctx context.Context, key string) (*Entry, error)
	Search(ctx context.Context, query string, limit int) ([]*Entry, error)
	Delete(ctx context.Context, key string) error
	List(ctx context.Context, memType MemoryType, limit int) ([]*Entry, error)
	GC(ctx context.Context) int // 清理过期条目，返回清理数量
}

// InMemoryStore 内存实现（开发/测试用，生产应接入 Redis/PG）
type InMemoryStore struct {
	mu      sync.RWMutex
	entries map[string]*Entry
	cancel  context.CancelFunc
}

// NewInMemoryStore 创建内存存储（自动启动后台 GC）
func NewInMemoryStore() *InMemoryStore {
	ctx, cancel := context.WithCancel(context.Background())
	s := &InMemoryStore{
		entries: make(map[string]*Entry),
		cancel:  cancel,
	}
	go s.gcLoop(ctx)
	return s
}

// Close 关闭存储（停止 GC）
func (s *InMemoryStore) Close() {
	if s.cancel != nil {
		s.cancel()
	}
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

// Get 获取（使用 atomic 更新访问计数，避免数据竞争）
func (s *InMemoryStore) Get(ctx context.Context, key string) (*Entry, error) {
	s.mu.RLock()
	e, ok := s.entries[key]
	s.mu.RUnlock()

	if !ok {
		return nil, nil
	}

	// 检查过期
	if e.ExpiresAt != nil && time.Now().After(*e.ExpiresAt) {
		// 惰性删除过期条目
		s.mu.Lock()
		delete(s.entries, key)
		s.mu.Unlock()
		return nil, nil
	}

	// 使用 atomic 自增访问计数（无需写锁）
	atomic.AddInt64(&e.AccessCnt, 1)
	return e, nil
}

// Search 简单关键词搜索（生产应接入向量数据库）
func (s *InMemoryStore) Search(ctx context.Context, query string, limit int) ([]*Entry, error) {
	if limit <= 0 {
		limit = 10
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	now := time.Now()
	var results []*Entry
	queryLower := strings.ToLower(query)

	for _, e := range s.entries {
		// 跳过过期条目
		if e.ExpiresAt != nil && now.After(*e.ExpiresAt) {
			continue
		}
		// 关键词匹配（不区分大小写）
		if strings.Contains(strings.ToLower(e.Content), queryLower) ||
			strings.Contains(strings.ToLower(e.Key), queryLower) {
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
	if limit <= 0 {
		limit = 50
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	now := time.Now()
	var results []*Entry
	for _, e := range s.entries {
		if e.Type == memType {
			if e.ExpiresAt != nil && now.After(*e.ExpiresAt) {
				continue
			}
			results = append(results, e)
			if len(results) >= limit {
				break
			}
		}
	}
	return results, nil
}

// GC 清理过期条目
func (s *InMemoryStore) GC(ctx context.Context) int {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now()
	count := 0
	for key, e := range s.entries {
		if e.ExpiresAt != nil && now.After(*e.ExpiresAt) {
			delete(s.entries, key)
			count++
		}
	}
	return count
}

// gcLoop 后台定期 GC（每 60 秒清理过期条目）
func (s *InMemoryStore) gcLoop(ctx context.Context) {
	ticker := time.NewTicker(60 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.GC(ctx)
		}
	}
}

// Stats 存储统计
func (s *InMemoryStore) Stats() map[string]int {
	s.mu.RLock()
	defer s.mu.RUnlock()

	stats := map[string]int{
		"total":      len(s.entries),
		"short_term": 0,
		"long_term":  0,
		"working":    0,
	}
	for _, e := range s.entries {
		switch e.Type {
		case ShortTerm:
			stats["short_term"]++
		case LongTerm:
			stats["long_term"]++
		case Working:
			stats["working"]++
		}
	}
	return stats
}

// ========== Manager 记忆管理器 ==========

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
		Type:     LongTerm,
		Key:      key,
		Content:  content,
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

// SetWorking 设置工作记忆（当前任务上下文，30分钟过期）
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

// SetShortTerm 设置短期记忆（自定义 TTL）
func (m *Manager) SetShortTerm(ctx context.Context, key, content string, ttl time.Duration) error {
	exp := time.Now().Add(ttl)
	return m.store.Save(ctx, &Entry{
		Type:      ShortTerm,
		Key:       "short:" + key,
		Content:   content,
		ExpiresAt: &exp,
	})
}

// GetStore 暴露底层 Store（供高级用途）
func (m *Manager) GetStore() Store {
	return m.store
}
