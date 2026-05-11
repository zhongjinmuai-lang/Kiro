// Package engine v2.0 智能体事件总线
// 支持智能体间通信、事件发布/订阅、异步解耦
package engine

import (
	"sync"
)

// EventHandler 事件处理函数
type EventHandler func(event *Event)

// Event 事件
type Event struct {
	Type    string                 `json:"type"`
	Source  string                 `json:"source"`  // 发布者
	Payload map[string]interface{} `json:"payload"`
}

// EventBus 事件总线
type EventBus struct {
	mu       sync.RWMutex
	handlers map[string][]EventHandler
}

// NewEventBus 创建事件总线
func NewEventBus() *EventBus {
	return &EventBus{handlers: make(map[string][]EventHandler)}
}

// Subscribe 订阅事件
func (b *EventBus) Subscribe(eventType string, handler EventHandler) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.handlers[eventType] = append(b.handlers[eventType], handler)
}

// Publish 发布事件（异步通知所有订阅者）
func (b *EventBus) Publish(event *Event) {
	b.mu.RLock()
	handlers := b.handlers[event.Type]
	b.mu.RUnlock()

	for _, h := range handlers {
		go h(event) // 异步执行
	}
}

// PublishSync 同步发布（等待所有处理完成）
func (b *EventBus) PublishSync(event *Event) {
	b.mu.RLock()
	handlers := b.handlers[event.Type]
	b.mu.RUnlock()

	var wg sync.WaitGroup
	for _, h := range handlers {
		wg.Add(1)
		handler := h
		go func() {
			defer wg.Done()
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

// 预定义事件类型
const (
	EventPluginInstalled   = "plugin.installed"
	EventPluginStarted     = "plugin.started"
	EventPluginStopped     = "plugin.stopped"
	EventTaskCompleted     = "task.completed"
	EventTaskFailed        = "task.failed"
	EventEvolutionTriggered = "evolution.triggered"
	EventGoalCompleted     = "goal.completed"
	EventMemoryStored      = "memory.stored"
)
