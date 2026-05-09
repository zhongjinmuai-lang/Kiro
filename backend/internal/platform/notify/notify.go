// Package notify 消息通知中台：多通道统一封装 + 三级管控
package notify

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"go.uber.org/zap"
	"gorm.io/gorm"

	"github.com/zhongjinmuai-lang/mu-framework/internal/model"
	"github.com/zhongjinmuai-lang/mu-framework/internal/saas/hierarchy"
	"github.com/zhongjinmuai-lang/mu-framework/pkg/logger"
)

// Sender 发送器接口（不同通道实现各自适配器）
type Sender interface {
	Type() model.NotifyChannelType
	Send(ctx context.Context, receiver, content string, config json.RawMessage) error
}

// Service 通知中台
type Service struct {
	db        *gorm.DB
	hierarchy *hierarchy.Service
	senders   map[model.NotifyChannelType]Sender
}

// NewService 创建通知中台
func NewService(db *gorm.DB, h *hierarchy.Service) *Service {
	return &Service{
		db:        db,
		hierarchy: h,
		senders:   make(map[model.NotifyChannelType]Sender),
	}
}

// RegisterSender 注册发送器
func (s *Service) RegisterSender(sd Sender) {
	s.senders[sd.Type()] = sd
	logger.L().Info("通知发送器已注册", zap.String("type", string(sd.Type())))
}

// ========== 通道/模板管理 ==========

// CreateChannelInput 创建通道入参（仅开发商）
type CreateChannelInput struct {
	TenantID string                  `json:"tenant_id" binding:"required"`
	Type     model.NotifyChannelType `json:"type" binding:"required"`
	Name     string                  `json:"name" binding:"required,max=100"`
	Config   string                  `json:"config"`
}

// CreateChannel 创建通知通道
func (s *Service) CreateChannel(ctx context.Context, operatorLevel string, in *CreateChannelInput) (*model.NotifyChannel, error) {
	if operatorLevel != hierarchy.LevelDeveloper {
		return nil, errors.New("仅开发商可创建通知通道")
	}
	cfg := in.Config
	if cfg == "" {
		cfg = "{}"
	}
	ch := &model.NotifyChannel{
		TenantID: in.TenantID,
		Level:    model.LevelDeveloper,
		Type:     in.Type,
		Name:     in.Name,
		Config:   cfg,
		Status:   model.StatusEnabled,
	}
	if err := s.db.WithContext(ctx).Create(ch).Error; err != nil {
		return nil, fmt.Errorf("创建通道失败: %w", err)
	}
	return ch, nil
}

// UpsertTemplate 创建或更新模板
func (s *Service) UpsertTemplate(ctx context.Context, tpl *model.NotifyTemplate) error {
	return s.db.WithContext(ctx).
		Where("tenant_id = ? AND code = ?", tpl.TenantID, tpl.Code).
		Assign(map[string]any{
			"channel":    tpl.Channel,
			"name":       tpl.Name,
			"content":    tpl.Content,
			"status":     model.StatusEnabled,
			"updated_at": time.Now(),
		}).
		FirstOrCreate(tpl).Error
}

// ========== 发送 ==========

// SendInput 发送消息入参
type SendInput struct {
	TenantID     string                  `json:"tenant_id" binding:"required"`
	ChannelType  model.NotifyChannelType `json:"channel_type" binding:"required"`
	TemplateCode string                  `json:"template_code" binding:"required"`
	Receiver     string                  `json:"receiver" binding:"required"`
	Variables    map[string]string       `json:"variables"`
}

// Send 发送消息（异步）
func (s *Service) Send(ctx context.Context, in *SendInput) (*model.NotifyMessage, error) {
	// 1. 查通道
	var ch model.NotifyChannel
	err := s.db.WithContext(ctx).
		First(&ch, "tenant_id = ? AND type = ? AND status = 1", in.TenantID, in.ChannelType).Error
	if err != nil {
		return nil, fmt.Errorf("未找到可用通道: %w", err)
	}

	// 2. 查模板 + 渲染
	var tpl model.NotifyTemplate
	if err := s.db.WithContext(ctx).
		First(&tpl, "tenant_id = ? AND code = ? AND status = 1", in.TenantID, in.TemplateCode).Error; err != nil {
		return nil, fmt.Errorf("模板 %s 不存在: %w", in.TemplateCode, err)
	}
	content := renderTemplate(tpl.Content, in.Variables)

	// 3. 落库
	tplID := tpl.ID
	msg := &model.NotifyMessage{
		TenantID:   in.TenantID,
		ChannelID:  ch.ID,
		TemplateID: &tplID,
		Receiver:   in.Receiver,
		Content:    content,
		Status:     model.MsgPending,
	}
	if err := s.db.WithContext(ctx).Create(msg).Error; err != nil {
		return nil, fmt.Errorf("消息入库失败: %w", err)
	}

	// 4. 异步发送（实际生产建议入 Redis Stream）
	go s.doSend(context.Background(), msg, &ch)

	return msg, nil
}

// doSend 实际投递
func (s *Service) doSend(ctx context.Context, msg *model.NotifyMessage, ch *model.NotifyChannel) {
	sender, ok := s.senders[ch.Type]
	if !ok {
		s.updateStatus(ctx, msg.ID, model.MsgFailed)
		logger.L().Error("发送器未注册", zap.String("type", string(ch.Type)))
		return
	}
	s.updateStatus(ctx, msg.ID, model.MsgSending)

	if err := sender.Send(ctx, msg.Receiver, msg.Content, json.RawMessage(ch.Config)); err != nil {
		logger.L().Error("消息发送失败",
			zap.String("msg_id", msg.ID), zap.Error(err),
		)
		s.handleFailure(ctx, msg)
		return
	}

	now := time.Now()
	s.db.WithContext(ctx).Model(&model.NotifyMessage{}).
		Where("id = ?", msg.ID).
		Updates(map[string]any{
			"status":  model.MsgSent,
			"sent_at": now,
		})
	logger.L().Info("消息发送成功",
		zap.String("msg_id", msg.ID), zap.String("receiver", msg.Receiver),
	)
}

// handleFailure 失败重试（最多 3 次）
func (s *Service) handleFailure(ctx context.Context, msg *model.NotifyMessage) {
	const maxRetry = 3
	if msg.RetryCount >= maxRetry {
		s.updateStatus(ctx, msg.ID, model.MsgFailed)
		return
	}
	s.db.WithContext(ctx).Model(&model.NotifyMessage{}).
		Where("id = ?", msg.ID).
		Updates(map[string]any{
			"status":      model.MsgRetrying,
			"retry_count": gorm.Expr("retry_count + 1"),
		})
	// TODO: 接入延迟队列进行重试
}

func (s *Service) updateStatus(ctx context.Context, id string, st model.NotifyMessageStatus) {
	s.db.WithContext(ctx).Model(&model.NotifyMessage{}).
		Where("id = ?", id).Update("status", st)
}

// renderTemplate 模板变量替换（支持 {{key}} 占位）
func renderTemplate(tpl string, vars map[string]string) string {
	out := tpl
	for k, v := range vars {
		out = strings.ReplaceAll(out, "{{"+k+"}}", v)
	}
	return out
}
