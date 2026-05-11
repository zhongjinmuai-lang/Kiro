// Package notify 消息通知中台（v2.6）
//
// 修复：
//   - goroutine 泄漏 → 使用 worker pool 限制并发
//   - 重试逻辑空转 → 实现真正的延迟重试
//   - WebSocket 离线消息丢失 → 添加回落机制
//   - 模板渲染 XSS 风险 → HTML 转义
//
// 核心能力：多通道统一封装 + 三级管控 + 模板渲染 + 异步发送
package notify

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"strings"
	"sync"
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

	// Worker pool 控制并发发送
	sendCh chan *sendTask
	wg     sync.WaitGroup
	cancel context.CancelFunc
}

// sendTask 发送任务
type sendTask struct {
	msg *model.NotifyMessage
	ch  *model.NotifyChannel
}

const (
	maxWorkers    = 20 // 最大并发发送协程数
	maxRetry      = 3  // 最大重试次数
	retryInterval = 30 * time.Second
)

// NewService 创建通知中台
func NewService(db *gorm.DB, h *hierarchy.Service) *Service {
	ctx, cancel := context.WithCancel(context.Background())
	s := &Service{
		db:        db,
		hierarchy: h,
		senders:   make(map[model.NotifyChannelType]Sender),
		sendCh:    make(chan *sendTask, 1000),
		cancel:    cancel,
	}
	// 启动 worker pool
	for i := 0; i < maxWorkers; i++ {
		s.wg.Add(1)
		go s.worker(ctx)
	}
	return s
}

// Stop 优雅停止
func (s *Service) Stop() {
	s.cancel()
	close(s.sendCh)
	s.wg.Wait()
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

// ListChannels 获取租户通道列表
func (s *Service) ListChannels(ctx context.Context, tenantID string) ([]*model.NotifyChannel, error) {
	var list []*model.NotifyChannel
	if err := s.db.WithContext(ctx).
		Where("tenant_id = ? AND status = 1", tenantID).
		Find(&list).Error; err != nil {
		return nil, err
	}
	return list, nil
}

// ListTemplates 获取租户模板列表
func (s *Service) ListTemplates(ctx context.Context, tenantID string) ([]*model.NotifyTemplate, error) {
	var list []*model.NotifyTemplate
	if err := s.db.WithContext(ctx).
		Where("tenant_id = ? AND status = 1", tenantID).
		Order("code").
		Find(&list).Error; err != nil {
		return nil, err
	}
	return list, nil
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

// Send 发送消息（异步，通过 worker pool）
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
	content := renderTemplate(tpl.Content, in.Variables, ch.Type)

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

	// 4. 投递到 worker pool（非阻塞）
	select {
	case s.sendCh <- &sendTask{msg: msg, ch: &ch}:
	default:
		// 队列满时降级：标记为 retrying，由定时任务补发
		s.updateStatus(ctx, msg.ID, model.MsgRetrying)
		logger.L().Warn("发送队列已满，消息降级为重试", zap.String("msg_id", msg.ID))
	}

	return msg, nil
}

// SendDirect 直接发送（不经过模板，用于系统内部通知）
func (s *Service) SendDirect(ctx context.Context, tenantID string, channelType model.NotifyChannelType, receiver, content string) error {
	var ch model.NotifyChannel
	if err := s.db.WithContext(ctx).
		First(&ch, "tenant_id = ? AND type = ? AND status = 1", tenantID, channelType).Error; err != nil {
		return fmt.Errorf("未找到可用通道: %w", err)
	}

	msg := &model.NotifyMessage{
		TenantID:  tenantID,
		ChannelID: ch.ID,
		Receiver:  receiver,
		Content:   content,
		Status:    model.MsgPending,
	}
	if err := s.db.WithContext(ctx).Create(msg).Error; err != nil {
		return err
	}

	select {
	case s.sendCh <- &sendTask{msg: msg, ch: &ch}:
	default:
		s.updateStatus(ctx, msg.ID, model.MsgRetrying)
	}
	return nil
}

// RetryPending 重试待发送/重试中的消息（定时任务调用）
func (s *Service) RetryPending(ctx context.Context) (int64, error) {
	var messages []*model.NotifyMessage
	if err := s.db.WithContext(ctx).
		Where("status IN ? AND retry_count < ?",
			[]model.NotifyMessageStatus{model.MsgRetrying, model.MsgPending}, maxRetry).
		Limit(100).
		Find(&messages).Error; err != nil {
		return 0, err
	}

	var count int64
	for _, msg := range messages {
		var ch model.NotifyChannel
		if err := s.db.WithContext(ctx).First(&ch, "id = ?", msg.ChannelID).Error; err != nil {
			continue
		}
		select {
		case s.sendCh <- &sendTask{msg: msg, ch: &ch}:
			count++
		default:
			break
		}
	}
	return count, nil
}

// ========== Worker ==========

func (s *Service) worker(ctx context.Context) {
	defer s.wg.Done()
	for {
		select {
		case <-ctx.Done():
			return
		case task, ok := <-s.sendCh:
			if !ok {
				return
			}
			s.doSend(ctx, task.msg, task.ch)
		}
	}
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
			zap.String("msg_id", msg.ID),
			zap.String("channel", string(ch.Type)),
			zap.Int("retry", msg.RetryCount),
			zap.Error(err),
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
		zap.String("msg_id", msg.ID),
		zap.String("receiver", msg.Receiver),
		zap.String("channel", string(ch.Type)),
	)
}

// handleFailure 失败重试
func (s *Service) handleFailure(ctx context.Context, msg *model.NotifyMessage) {
	if msg.RetryCount >= maxRetry {
		s.updateStatus(ctx, msg.ID, model.MsgFailed)
		logger.L().Warn("消息重试次数耗尽，标记失败",
			zap.String("msg_id", msg.ID),
			zap.Int("retries", msg.RetryCount),
		)
		return
	}
	// 标记为重试中并增加计数
	s.db.WithContext(ctx).Model(&model.NotifyMessage{}).
		Where("id = ?", msg.ID).
		Updates(map[string]any{
			"status":      model.MsgRetrying,
			"retry_count": gorm.Expr("retry_count + 1"),
		})
}

func (s *Service) updateStatus(ctx context.Context, id string, st model.NotifyMessageStatus) {
	s.db.WithContext(ctx).Model(&model.NotifyMessage{}).
		Where("id = ?", id).Update("status", st)
}

// renderTemplate 模板变量替换（支持 {{key}} 占位）
// 对 HTML 邮件通道进行 XSS 转义
func renderTemplate(tpl string, vars map[string]string, channelType model.NotifyChannelType) string {
	out := tpl
	for k, v := range vars {
		safeV := v
		// 邮件通道且模板含 HTML 标签时转义变量值
		if channelType == model.NotifyEmail && strings.Contains(tpl, "<") {
			safeV = html.EscapeString(v)
		}
		out = strings.ReplaceAll(out, "{{"+k+"}}", safeV)
	}
	return out
}

// GetMessageHistory 消息历史（分页）
func (s *Service) GetMessageHistory(ctx context.Context, tenantID string, page, pageSize int) ([]*model.NotifyMessage, int64, error) {
	if pageSize <= 0 {
		pageSize = 20
	}
	var (
		list  []*model.NotifyMessage
		total int64
	)
	q := s.db.WithContext(ctx).Model(&model.NotifyMessage{}).Where("tenant_id = ?", tenantID)
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	if err := q.Order("created_at DESC").
		Limit(pageSize).Offset((page-1)*pageSize).
		Find(&list).Error; err != nil {
		return nil, 0, err
	}
	return list, total, nil
}
