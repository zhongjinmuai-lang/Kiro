// Package payment 聚合支付中台（v2.6）
//
// 修复：
//   - CreateOrder 集成 Adapter.Prepay 获取预支付参数
//   - HandleCallback 增加签名验证（VerifyCallback）
//   - Refund 调用适配器实际退款 API
//   - 添加订单超时关单机制
//   - 适配器注册表模式
//
// 核心能力：开发商准入 → 服务商授权/绑定 → 终端受限使用
package payment

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"go.uber.org/zap"
	"gorm.io/gorm"

	"github.com/zhongjinmuai-lang/mu-framework/internal/model"
	"github.com/zhongjinmuai-lang/mu-framework/internal/platform/payment/adapters"
	"github.com/zhongjinmuai-lang/mu-framework/internal/saas/hierarchy"
	"github.com/zhongjinmuai-lang/mu-framework/pkg/logger"
)

// Service 支付中台服务
type Service struct {
	db        *gorm.DB
	hierarchy *hierarchy.Service
	mu        sync.RWMutex
	adapters  map[model.PaymentChannelType]adapters.Adapter
}

// NewService 创建服务
func NewService(db *gorm.DB, h *hierarchy.Service) *Service {
	s := &Service{
		db:        db,
		hierarchy: h,
		adapters:  make(map[model.PaymentChannelType]adapters.Adapter),
	}
	// 注册默认适配器骨架
	s.RegisterAdapter(adapters.NewWechatPayAdapter())
	s.RegisterAdapter(adapters.NewAlipayAdapter())
	return s
}

// RegisterAdapter 注册支付适配器
func (s *Service) RegisterAdapter(a adapters.Adapter) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.adapters[a.Type()] = a
	logger.L().Info("支付适配器已注册", zap.String("type", string(a.Type())))
}

// GetAdapter 获取适配器
func (s *Service) GetAdapter(channelType model.PaymentChannelType) (adapters.Adapter, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	a, ok := s.adapters[channelType]
	if !ok {
		return nil, fmt.Errorf("支付适配器未注册: %s", channelType)
	}
	return a, nil
}

// ========== 渠道管理（开发商顶层集权） ==========

// CreateChannelInput 创建渠道入参（仅开发商）
type CreateChannelInput struct {
	TenantID   string                   `json:"tenant_id" binding:"required"`
	Type       model.PaymentChannelType `json:"type" binding:"required"`
	Name       string                   `json:"name" binding:"required,max=100"`
	AppID      string                   `json:"app_id"`
	MerchantID string                   `json:"merchant_id"`
	SecretKey  string                   `json:"secret_key"`
	NotifyURL  string                   `json:"notify_url"`
}

// CreateChannel 创建支付渠道（顶层准入）
func (s *Service) CreateChannel(ctx context.Context, operatorLevel string, in *CreateChannelInput) (*model.PaymentChannel, error) {
	if operatorLevel != hierarchy.LevelDeveloper {
		return nil, errors.New("仅开发商层级可创建支付渠道")
	}

	ch := &model.PaymentChannel{
		TenantID:   in.TenantID,
		Level:      model.LevelDeveloper,
		Type:       in.Type,
		Name:       in.Name,
		AppID:      in.AppID,
		MerchantID: in.MerchantID,
		SecretKey:  in.SecretKey, // TODO: 生产环境应加密存储
		NotifyURL:  in.NotifyURL,
		Status:     model.StatusEnabled,
	}
	if err := s.db.WithContext(ctx).Create(ch).Error; err != nil {
		return nil, fmt.Errorf("创建支付渠道失败: %w", err)
	}
	logger.WithContext(ctx).Info("支付渠道已创建",
		zap.String("id", ch.ID), zap.String("type", string(ch.Type)),
	)
	return ch, nil
}

// ToggleChannel 启用/禁用渠道（顶层关闭 → 级联所有下级失效）
func (s *Service) ToggleChannel(ctx context.Context, operatorLevel, channelID string, enabled bool) error {
	if operatorLevel != hierarchy.LevelDeveloper {
		return errors.New("仅开发商可启用/禁用支付渠道")
	}
	status := model.StatusEnabled
	if !enabled {
		status = model.StatusDisabled
	}
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&model.PaymentChannel{}).
			Where("id = ?", channelID).
			Update("status", status).Error; err != nil {
			return err
		}
		// 渠道关闭 → 对应授权记录同步失效
		if !enabled {
			if err := tx.Model(&model.TenantPaymentAuth{}).
				Where("channel_id = ?", channelID).
				Update("status", model.StatusDisabled).Error; err != nil {
				return err
			}
			logger.WithContext(ctx).Info("渠道关闭，级联授权失效", zap.String("channel_id", channelID))
		}
		return nil
	})
}

// GrantInput 授权入参（上级授予下级可用渠道）
type GrantInput struct {
	GranterTenantID string `json:"granter_tenant_id" binding:"required"`
	GranteeTenantID string `json:"grantee_tenant_id" binding:"required"`
	ChannelID       string `json:"channel_id" binding:"required"`
}

// GrantChannel 授予渠道使用权（开发商→服务商 / 服务商→终端客户）
func (s *Service) GrantChannel(ctx context.Context, in *GrantInput) error {
	if err := s.hierarchy.ValidateControlFlow(ctx, in.GranterTenantID, in.GranteeTenantID); err != nil {
		return fmt.Errorf("授权失败: %w", err)
	}

	return s.db.WithContext(ctx).Exec(`
INSERT INTO tenant_payment_auth (id, tenant_id, channel_id, granted_by, status, created_at, updated_at)
VALUES (uuid_generate_v4(), ?, ?, ?, 1, NOW(), NOW())
ON CONFLICT (tenant_id, channel_id)
DO UPDATE SET status = 1, granted_by = EXCLUDED.granted_by, updated_at = NOW()`,
		in.GranteeTenantID, in.ChannelID, in.GranterTenantID,
	).Error
}

// RevokeChannel 回收渠道使用权（级联所有下级）
func (s *Service) RevokeChannel(ctx context.Context, revokerID, targetID, channelID string) error {
	if err := s.hierarchy.ValidateControlFlow(ctx, revokerID, targetID); err != nil {
		return err
	}
	descendants, err := s.hierarchy.GetDescendants(ctx, targetID)
	if err != nil {
		return err
	}
	affected := append([]string{targetID}, descendants...)
	return s.db.WithContext(ctx).Model(&model.TenantPaymentAuth{}).
		Where("tenant_id IN ? AND channel_id = ?", affected, channelID).
		Update("status", model.StatusDisabled).Error
}

// GetAvailableChannels 获取租户可用的支付渠道（沿三级授权链路）
func (s *Service) GetAvailableChannels(ctx context.Context, tenantID string) ([]*model.PaymentChannel, error) {
	var list []*model.PaymentChannel
	err := s.db.WithContext(ctx).
		Model(&model.PaymentChannel{}).
		Joins("INNER JOIN tenant_payment_auth tpa ON payment_channels.id = tpa.channel_id").
		Where("tpa.tenant_id = ? AND tpa.status = 1 AND payment_channels.status = 1", tenantID).
		Find(&list).Error
	return list, err
}

// ========== 订单能力（v2.6 集成适配器） ==========

// CreateOrderInput 创建订单入参
type CreateOrderInput struct {
	TenantID  string `json:"tenant_id" binding:"required"`
	ChannelID string `json:"channel_id" binding:"required"`
	OrderNo   string `json:"order_no" binding:"required,max=64"`
	Amount    int64  `json:"amount" binding:"required,gt=0"` // 分
	Currency  string `json:"currency"`
	Subject   string `json:"subject" binding:"max=200"`
	// 支付场景参数
	OpenID    string `json:"open_id"`    // 微信JSAPI需要
	TradeType string `json:"trade_type"` // JSAPI/NATIVE/H5/APP
	ReturnURL string `json:"return_url"` // 支付完成跳转URL
	UserIP    string `json:"user_ip"`
}

// CreateOrderResult 下单结果
type CreateOrderResult struct {
	Order   *model.PaymentOrder      `json:"order"`
	Prepay  *adapters.PrepayResponse `json:"prepay"` // 预支付参数（前端调起支付用）
}

// CreateOrder 统一下单（集成适配器预支付）
func (s *Service) CreateOrder(ctx context.Context, in *CreateOrderInput) (*CreateOrderResult, error) {
	// 1. 校验：该租户是否有权使用该渠道
	var cnt int64
	if err := s.db.WithContext(ctx).Model(&model.TenantPaymentAuth{}).
		Where("tenant_id = ? AND channel_id = ? AND status = 1", in.TenantID, in.ChannelID).
		Count(&cnt).Error; err != nil {
		return nil, fmt.Errorf("校验渠道授权失败: %w", err)
	}
	if cnt == 0 {
		return nil, errors.New("当前租户未被授权使用该支付渠道")
	}

	// 2. 查询渠道配置
	var ch model.PaymentChannel
	if err := s.db.WithContext(ctx).First(&ch, "id = ? AND status = 1", in.ChannelID).Error; err != nil {
		return nil, fmt.Errorf("渠道不存在或已禁用: %w", err)
	}

	// 3. 获取适配器
	adapter, err := s.GetAdapter(ch.Type)
	if err != nil {
		return nil, err
	}

	currency := in.Currency
	if currency == "" {
		currency = "CNY"
	}

	// 4. 创建订单记录（事务内）
	order := &model.PaymentOrder{
		TenantID:    in.TenantID,
		ChannelID:   in.ChannelID,
		ChannelType: ch.Type,
		OrderNo:     in.OrderNo,
		Amount:      in.Amount,
		Currency:    currency,
		Subject:     in.Subject,
		Status:      model.OrderPending,
	}
	if err := s.db.WithContext(ctx).Create(order).Error; err != nil {
		return nil, fmt.Errorf("创建订单失败: %w", err)
	}

	// 5. 调用适配器预支付
	notifyURL := ch.NotifyURL
	if notifyURL == "" {
		notifyURL = fmt.Sprintf("/api/v1/pay/callback/%s", string(ch.Type))
	}
	prepayReq := &adapters.PrepayRequest{
		OrderNo:   in.OrderNo,
		Amount:    in.Amount,
		Currency:  currency,
		Subject:   in.Subject,
		NotifyURL: notifyURL,
		ReturnURL: in.ReturnURL,
		OpenID:    in.OpenID,
		TradeType: in.TradeType,
		UserIP:    in.UserIP,
	}
	prepayResp, err := adapter.Prepay(ctx, &ch, prepayReq)
	if err != nil {
		// 预支付失败，关闭订单
		s.db.WithContext(ctx).Model(order).Update("status", model.OrderClosed)
		logger.WithContext(ctx).Error("预支付失败",
			zap.String("order_no", in.OrderNo),
			zap.String("channel", string(ch.Type)),
			zap.Error(err),
		)
		return nil, fmt.Errorf("预支付失败: %w", err)
	}

	logger.WithContext(ctx).Info("支付订单已创建",
		zap.String("order_id", order.ID),
		zap.String("order_no", order.OrderNo),
		zap.Int64("amount", order.Amount),
		zap.String("channel", string(ch.Type)),
	)
	return &CreateOrderResult{Order: order, Prepay: prepayResp}, nil
}

// HandleCallbackInput 回调处理入参
type HandleCallbackInput struct {
	ChannelType model.PaymentChannelType
	Headers     map[string]string
	RawBody     []byte
}

// HandleCallback 处理支付回调（带签名验证）
func (s *Service) HandleCallback(ctx context.Context, in *HandleCallbackInput) error {
	// 1. 获取适配器
	adapter, err := s.GetAdapter(in.ChannelType)
	if err != nil {
		return fmt.Errorf("回调处理失败: %w", err)
	}

	// 2. 获取该渠道类型的配置（用于验签）
	var ch model.PaymentChannel
	if err := s.db.WithContext(ctx).
		First(&ch, "type = ? AND status = 1", in.ChannelType).Error; err != nil {
		return fmt.Errorf("未找到渠道配置: %w", err)
	}

	// 3. 验证签名并解析回调
	result, err := adapter.VerifyCallback(ctx, &ch, in.Headers, in.RawBody)
	if err != nil {
		logger.WithContext(ctx).Warn("支付回调签名验证失败",
			zap.String("channel", string(in.ChannelType)),
			zap.Error(err),
		)
		return fmt.Errorf("回调签名验证失败: %w", err)
	}

	// 4. 幂等更新订单状态
	now := time.Now()
	res := s.db.WithContext(ctx).Model(&model.PaymentOrder{}).
		Where("order_no = ? AND status = ?", result.OrderNo, model.OrderPending).
		Updates(map[string]any{
			"status":   result.Status,
			"trade_no": result.TradeNo,
			"paid_at":  now,
		})
	if res.Error != nil {
		return fmt.Errorf("更新订单状态失败: %w", res.Error)
	}
	if res.RowsAffected == 0 {
		logger.WithContext(ctx).Info("订单状态非待支付，回调忽略（幂等）",
			zap.String("order_no", result.OrderNo),
		)
		return nil // 幂等，不返回错误
	}

	logger.WithContext(ctx).Info("支付回调处理成功",
		zap.String("order_no", result.OrderNo),
		zap.String("trade_no", result.TradeNo),
	)
	return nil
}

// Refund 发起退款（调用适配器）
func (s *Service) Refund(ctx context.Context, orderID string, refundNo string, amount int64) error {
	// 查询订单
	var order model.PaymentOrder
	if err := s.db.WithContext(ctx).First(&order, "id = ? AND status = ?", orderID, model.OrderPaid).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return errors.New("订单状态非已支付，无法退款")
		}
		return err
	}

	// 获取渠道配置
	var ch model.PaymentChannel
	if err := s.db.WithContext(ctx).First(&ch, "id = ?", order.ChannelID).Error; err != nil {
		return fmt.Errorf("渠道不存在: %w", err)
	}

	// 获取适配器
	adapter, err := s.GetAdapter(ch.Type)
	if err != nil {
		return err
	}

	// 更新状态为退款中
	if err := s.db.WithContext(ctx).Model(&order).Update("status", model.OrderRefunding).Error; err != nil {
		return err
	}

	// 调用适配器退款
	refundReq := &adapters.RefundRequest{
		OrderNo:  order.OrderNo,
		RefundNo: refundNo,
		Amount:   amount,
		Total:    order.Amount,
		Reason:   "用户发起退款",
	}
	if err := adapter.Refund(ctx, &ch, refundReq); err != nil {
		// 退款失败，回滚状态
		s.db.WithContext(ctx).Model(&order).Update("status", model.OrderPaid)
		logger.WithContext(ctx).Error("退款调用失败",
			zap.String("order_id", orderID),
			zap.Error(err),
		)
		return fmt.Errorf("退款失败: %w", err)
	}

	// 退款成功
	s.db.WithContext(ctx).Model(&order).Update("status", model.OrderRefunded)
	logger.WithContext(ctx).Info("退款成功",
		zap.String("order_id", orderID),
		zap.String("refund_no", refundNo),
		zap.Int64("amount", amount),
	)
	return nil
}

// GetOrder 查询订单
func (s *Service) GetOrder(ctx context.Context, tenantID, orderID string) (*model.PaymentOrder, error) {
	var order model.PaymentOrder
	if err := s.db.WithContext(ctx).
		First(&order, "id = ? AND tenant_id = ?", orderID, tenantID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.New("订单不存在")
		}
		return nil, err
	}
	return &order, nil
}

// ListOrders 订单列表（分页）
func (s *Service) ListOrders(ctx context.Context, tenantID string, page, pageSize int) ([]*model.PaymentOrder, int64, error) {
	if pageSize <= 0 {
		pageSize = 20
	}
	if page <= 0 {
		page = 1
	}
	var (
		list  []*model.PaymentOrder
		total int64
	)
	q := s.db.WithContext(ctx).Model(&model.PaymentOrder{}).Where("tenant_id = ?", tenantID)
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

// CloseExpiredOrders 关闭超时未支付订单（定时任务调用）
func (s *Service) CloseExpiredOrders(ctx context.Context, expireMinutes int) (int64, error) {
	if expireMinutes <= 0 {
		expireMinutes = 30 // 默认 30 分钟
	}
	cutoff := time.Now().Add(-time.Duration(expireMinutes) * time.Minute)
	res := s.db.WithContext(ctx).Model(&model.PaymentOrder{}).
		Where("status = ? AND created_at < ?", model.OrderPending, cutoff).
		Update("status", model.OrderClosed)
	if res.Error != nil {
		return 0, res.Error
	}
	if res.RowsAffected > 0 {
		logger.L().Info("已关闭超时订单", zap.Int64("count", res.RowsAffected))
	}
	return res.RowsAffected, nil
}
