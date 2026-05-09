package senders

import (
	"context"
	"encoding/json"
	"sync"
	"time"

	"github.com/zhongjinmuai-lang/mu-framework/internal/model"
)

// WebSocketSender 站内信实时推送
type WebSocketSender struct {
	mu   sync.RWMutex
	subs map[string][]chan<- []byte
}

// NewWebSocketSender 构造
func NewWebSocketSender() *WebSocketSender {
	return &WebSocketSender{subs: make(map[string][]chan<- []byte)}
}

// Type 通道类型
func (s *WebSocketSender) Type() model.NotifyChannelType { return model.NotifyWebSocket }

// Register 注册订阅者（WS 连接建立时调用）
func (s *WebSocketSender) Register(receiver string, ch chan<- []byte) func() {
	s.mu.Lock()
	s.subs[receiver] = append(s.subs[receiver], ch)
	s.mu.Unlock()
	return func() {
		s.mu.Lock()
		defer s.mu.Unlock()
		arr := s.subs[receiver]
		for i, c := range arr {
			if c == ch {
				s.subs[receiver] = append(arr[:i], arr[i+1:]...)
				break
			}
		}
		if len(s.subs[receiver]) == 0 {
			delete(s.subs, receiver)
		}
	}
}

// Send 推送
func (s *WebSocketSender) Send(ctx context.Context, receiver, content string, raw json.RawMessage) error {
	s.mu.RLock()
	subs := append([]chan<- []byte{}, s.subs[receiver]...)
	s.mu.RUnlock()

	payload, _ := json.Marshal(map[string]any{
		"ts":      time.Now().Unix(),
		"content": content,
	})
	for _, ch := range subs {
		select {
		case ch <- payload:
		case <-time.After(2 * time.Second):
		}
	}
	return nil
}
