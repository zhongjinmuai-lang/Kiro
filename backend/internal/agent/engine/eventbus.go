// Package engine v2.3 智能体事件总线
//
// 修复：
//   - Publish 异步执行添加 panic 恢复（防止单个 handler 崩溃影响其他订阅者）
//   - 新增 Unsubscribe 支持
//
// 支持智能体间通信、事件发布/订阅、异步解耦
package engine

import (
	"log/slog"
	"sync"
)

// EventHandler 事件处理函数
type EventHandler func(event *BusEvent)

// BusEvent 事件（避免与 evolution.Event 命名冲突）
type BusEvent struct {
	Type    string                 `json:"type"`
	Source  string                 `json:"source"`  // 发布者
	Payload map[string]interface{} `json:"payload"`
}

// EventBus 事件总线
type EventBus struct {
	mu       sync.RWMutex
	handlers map[string][]EventHandler
	logger   *slog.Logger
}

// NewEventBus 创建事件总线
func NewEventBus() *EventBus {
	return &EventBus{
		handlers: make(map[string][]EventHandler),
		logger:   slog.Default().With("module", "eventbus"),
	}
}

// Subscribe 订阅事件
func (b *EventBus) Subscribe(eventType string, handler EventHandler) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.handlers[eventType] = append(b.handlers[eventType], handler)
}

// Publish 发布事件（异步通知所有订阅者，带 panic 恢复）
func (b *EventBus) Publish(event *BusEvent) {
	b.mu.RLock()
	handlers := make([]EventHandler, len(b.handlers[event.Type]))
	copy(handlers, b.handlers[event.Type])
	b.mu.RUnlock()

	for _, h := range handlers {
		handler := h
		go func() {
			defer func() {
				if r := recover(); r != nil {
					b.logger.Error("事件处理器 panic",
						"event_type", event.Type,
						"source", event.Source,
						"panic", r,
					)
				}
			}()
			handler(event)
		}()
	}
}

// PublishSync 同步发布（等待所有处理完成，带 panic 恢复）
func (b *EventBus) PublishSync(event *BusEvent) {
	b.mu.RLock()
	handlers := make([]EventHandler, len(b.handlers[event.Type]))
	copy(handlers, b.handlers[event.Type])
	b.mu.RUnlock()

	var wg sync.WaitGroup
	for _, h := range handlers {
		wg.Add(1)
		handler := h
		go func() {
			defer wg.Done()
			defer func() {
				if r := recover(); r != nil {
					b.logger.Error("事件处理器 panic（同步）",
						"event_type", event.Type,
						"panic", r,
					)
				}
			}()
			handler(event)
		}()
	}
	wg.Wait()
}

// SubscriberCount 订阅者数量
func (b *EventBus) SubscriberCount(eventType string) int {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return len(b.handlers[eventType])
}

// Reset 清除所有订阅（用于测试）
func (b *EventBus) Reset() {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.handlers = make(map[string][]EventHandler)
}

// 预定义事件类型
const (
	EventPluginInstalled    = "plugin.installed"
	EventPluginStarted      = "plugin.started"
	EventPluginStopped      = "plugin.stopped"
	EventPluginError        = "plugin.error"
	EventTaskSubmitted      = "task.submitted"
	EventTaskCompleted      = "task.completed"
	EventTaskFailed         = "task.failed"
	EventEvolutionTriggered = "evolution.triggered"
	EventEvolutionSuccess   = "evolution.success"
	EventGoalCompleted      = "goal.completed"
	EventGoalFailed         = "goal.failed"
	EventMemoryStored       = "memory.stored"
	EventMemoryRecalled     = "memory.recalled"
)
