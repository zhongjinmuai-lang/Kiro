package notify

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/zhongjinmuai-lang/mu-framework/internal/saas/hierarchy"
)

// ChannelType 通知通道类型
type ChannelType string

const (
	ChannelSMS       ChannelType = "sms"       // 短信
	ChannelEmail     ChannelType = "email"     // 邮件
	ChannelPush      ChannelType = "push"      // App推送
	ChannelWechat    ChannelType = "wechat"    // 微信模板消息
	ChannelWebSocket ChannelType = "websocket" // WebSocket实时推送
)

// MessageStatus 消息状态
type MessageStatus int

const (
	StatusPending  MessageStatus = 0 // 待发送
	StatusSending  MessageStatus = 1 // 发送中
	StatusSent     MessageStatus = 2 // 已发送
	StatusFailed   MessageStatus = 3 // 发送失败
	StatusRetrying MessageStatus = 4 // 重试中
)

// Channel 通知通道配置
type Channel struct {
	ID        string      `json:"id" db:"id"`
	TenantID  string      `json:"tenant_id" db:"tenant_id"`
	Level     string      `json:"level" db:"level"`
	Type      ChannelType `json:"type" db:"type"`
	Name      string      `json:"name" db:"name"`
	Config    string      `json:"config" db:"config"` // JSON配置（不同通道不同）
	Status    int         `json:"status" db:"status"`
	CreatedAt time.Time   `json:"created_at" db:"created_at"`
	UpdatedAt time.Time   `json:"updated_at" db:"updated_at"`
}

// Template 消息模板
type Template struct {
	ID        string      `json:"id" db:"id"`
	TenantID  string      `json:"tenant_id" db:"tenant_id"`
	Channel   ChannelType `json:"channel" db:"channel"`
	Code      string      `json:"code" db:"code"`         // 模板编码
	Name      string      `json:"name" db:"name"`
	Content   string      `json:"content" db:"content"`   // 模板内容（支持变量占位）
	Status    int         `json:"status" db:"status"`
	CreatedAt time.Time   `json:"created_at" db:"created_at"`
	UpdatedAt time.Time   `json:"updated_at" db:"updated_at"`
}

// Message 消息记录
type Message struct {
	ID         string        `json:"id" db:"id"`
	TenantID   string        `json:"tenant_id" db:"tenant_id"`
	ChannelID  string        `json:"channel_id" db:"channel_id"`
	TemplateID string        `json:"template_id" db:"template_id"`
	Receiver   string        `json:"receiver" db:"receiver"` // 接收方标识
	Content    string        `json:"content" db:"content"`
	Status     MessageStatus `json:"status" db:"status"`
	RetryCount int           `json:"retry_count" db:"retry_count"`
	SentAt     *time.Time    `json:"sent_at" db:"sent_at"`
	CreatedAt  time.Time     `json:"created_at" db:"created_at"`
}

// Sender 发送器接口
type Sender interface {
	Send(ctx context.Context, receiver, content string, config json.RawMessage) error
	Type() ChannelType
}

// Service 通知中台服务
type Service struct {
	db        *pgxpool.Pool
	hierarchy *hierarchy.Service
	senders   map[ChannelType]Sender
	logger    *slog.Logger
}

// NewService 创建通知中台服务
func NewService(db *pgxpool.Pool, hierarchySvc *hierarchy.Service) *Service {
	return &Service{
		db:        db,
		hierarchy: hierarchySvc,
		senders:   make(map[ChannelType]Sender),
		logger:    slog.Default().With("module", "notify"),
	}
}

// RegisterSender 注册发送器
func (s *Service) RegisterSender(sender Sender) {
	s.senders[sender.Type()] = sender
	s.logger.Info("通知发送器已注册", "type", sender.Type())
}

// CreateChannel 创建通知通道（开发商配置）
func (s *Service) CreateChannel(ctx context.Context, operatorLevel string, input *Channel) (*Channel, error) {
	if !s.hierarchy.CanAccess(operatorLevel, hierarchy.LevelDeveloper) {
		return nil, fmt.Errorf("仅开发商层级可创建通知通道")
	}

	input.ID = uuid.New().String()
	input.Status = 1
	input.CreatedAt = time.Now()
	input.UpdatedAt = time.Now()

	query := `INSERT INTO notify_channels (id, tenant_id, level, type, name, config, status, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)`

	_, err := s.db.Exec(ctx, query,
		input.ID, input.TenantID, input.Level, input.Type,
		input.Name, input.Config, input.Status, input.CreatedAt, input.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("创建通知通道失败: %w", err)
	}

	s.logger.Info("通知通道创建成功", "id", input.ID, "type", input.Type)
	return input, nil
}

// SendInput 发送消息入参
type SendInput struct {
	TenantID     string            `json:"tenant_id"`
	ChannelType  ChannelType       `json:"channel_type"`
	TemplateCode string            `json:"template_code"`
	Receiver     string            `json:"receiver"`
	Variables    map[string]string `json:"variables"` // 模板变量
}

// Send 发送消息
func (s *Service) Send(ctx context.Context, input *SendInput) (*Message, error) {
	// 1. 获取通知通道
	channel, err := s.getChannelByType(ctx, input.TenantID, input.ChannelType)
	if err != nil {
		return nil, fmt.Errorf("获取通知通道失败: %w", err)
	}

	// 2. 获取模板并渲染
	content, templateID, err := s.renderTemplate(ctx, input.TenantID, input.TemplateCode, input.Variables)
	if err != nil {
		return nil, fmt.Errorf("渲染模板失败: %w", err)
	}

	// 3. 创建消息记录
	msg := &Message{
		ID:         uuid.New().String(),
		TenantID:   input.TenantID,
		ChannelID:  channel.ID,
		TemplateID: templateID,
		Receiver:   input.Receiver,
		Content:    content,
		Status:     StatusPending,
		CreatedAt:  time.Now(),
	}

	query := `INSERT INTO notify_messages (id, tenant_id, channel_id, template_id, receiver, content, status, retry_count, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, 0, $8)`

	_, err = s.db.Exec(ctx, query,
		msg.ID, msg.TenantID, msg.ChannelID, msg.TemplateID,
		msg.Receiver, msg.Content, msg.Status, msg.CreatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("创建消息记录失败: %w", err)
	}

	// 4. 异步发送（实际生产应入消息队列）
	go s.doSend(context.Background(), msg, channel)

	return msg, nil
}

// doSend 执行发送
func (s *Service) doSend(ctx context.Context, msg *Message, channel *Channel) {
	sender, ok := s.senders[ChannelType(channel.Type)]
	if !ok {
		s.updateStatus(ctx, msg.ID, StatusFailed)
		s.logger.Error("发送器未注册", "type", channel.Type)
		return
	}

	// 更新为发送中
	s.updateStatus(ctx, msg.ID, StatusSending)

	err := sender.Send(ctx, msg.Receiver, msg.Content, json.RawMessage(channel.Config))
	if err != nil {
		s.logger.Error("消息发送失败", "msg_id", msg.ID, "error", err)
		s.handleSendFailure(ctx, msg)
		return
	}

	// 发送成功
	now := time.Now()
	query := `UPDATE notify_messages SET status = $1, sent_at = $2 WHERE id = $3`
	s.db.Exec(ctx, query, StatusSent, now, msg.ID)
	s.logger.Info("消息发送成功", "msg_id", msg.ID, "receiver", msg.Receiver)
}

// handleSendFailure 处理发送失败（重试机制）
func (s *Service) handleSendFailure(ctx context.Context, msg *Message) {
	maxRetry := 3
	if msg.RetryCount >= maxRetry {
		s.updateStatus(ctx, msg.ID, StatusFailed)
		return
	}

	// 更新重试计数
	query := `UPDATE notify_messages SET status = $1, retry_count = retry_count + 1 WHERE id = $2`
	s.db.Exec(ctx, query, StatusRetrying, msg.ID)

	// TODO: 延迟重试（实际应使用延迟队列）
}

func (s *Service) updateStatus(ctx context.Context, msgID string, status MessageStatus) {
	query := `UPDATE notify_messages SET status = $1 WHERE id = $2`
	s.db.Exec(ctx, query, status, msgID)
}

func (s *Service) getChannelByType(ctx context.Context, tenantID string, channelType ChannelType) (*Channel, error) {
	query := `SELECT id, tenant_id, level, type, name, config, status FROM notify_channels
		WHERE tenant_id = $1 AND type = $2 AND status = 1 LIMIT 1`

	ch := &Channel{}
	err := s.db.QueryRow(ctx, query, tenantID, channelType).Scan(
		&ch.ID, &ch.TenantID, &ch.Level, &ch.Type, &ch.Name, &ch.Config, &ch.Status,
	)
	if err != nil {
		return nil, err
	}
	return ch, nil
}

func (s *Service) renderTemplate(ctx context.Context, tenantID, code string, vars map[string]string) (string, string, error) {
	query := `SELECT id, content FROM notify_templates WHERE tenant_id = $1 AND code = $2 AND status = 1`
	var templateID, content string
	if err := s.db.QueryRow(ctx, query, tenantID, code).Scan(&templateID, &content); err != nil {
		return "", "", fmt.Errorf("模板 %s 不存在", code)
	}

	// 简单的变量替换
	for k, v := range vars {
		content = replaceVar(content, k, v)
	}

	return content, templateID, nil
}

func replaceVar(content, key, value string) string {
	placeholder := "{{" + key + "}}"
	result := content
	for i := 0; i < len(result); i++ {
		idx := indexOf(result, placeholder, i)
		if idx == -1 {
			break
		}
		result = result[:idx] + value + result[idx+len(placeholder):]
		i = idx + len(value)
	}
	return result
}

func indexOf(s, sub string, start int) int {
	if start >= len(s) {
		return -1
	}
	for i := start; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}
