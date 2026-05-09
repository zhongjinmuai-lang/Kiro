// Package payment 聚合支付中台：开发商准入 → 服务商授权/绑定 → 终端受限使用
package payment

import (
	"context"
	"errors"
	"fmt"
	"time"

	"go.uber.org/zap"
	"gorm.io/gorm"

	"github.com/zhongjinmuai-lang/mu-framework/internal/model"
	"github.com/zhongjinmuai-lang/mu-framework/internal/saas/hierarchy"
	"github.com/zhongjinmuai-lang/mu-framework/pkg/logger"
)

// Service 支付中台服务
type Service struct {
	db        *gorm.DB
	hierarchy *hierarchy.Service
}

// NewService 创建服务
func NewService(db *gorm.DB, h *hierarchy.Service) *Service {
	return &Service{db: db, hierarchy: h}
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
		SecretKey:  in.SecretKey,
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

// ========== 订单能力 ==========

// CreateOrderInput 创建订单入参
type CreateOrderInput struct {
	TenantID  string `json:"tenant_id" binding:"required"`
	ChannelID string `json:"channel_id" binding:"required"`
	OrderNo   string `json:"order_no" binding:"required,max=64"`
	Amount    int64  `json:"amount" binding:"required,gt=0"` // 分
	Currency  string `json:"currency"`
	Subject   string `json:"subject" binding:"max=200"`
}

// CreateOrder 统一下单
func (s *Service) CreateOrder(ctx context.Context, in *CreateOrderInput) (*model.PaymentOrder, error) {
	// 校验：该租户是否有权使用该渠道
	var cnt int64
	if err := s.db.WithContext(ctx).Model(&model.TenantPaymentAuth{}).
		Where("tenant_id = ? AND channel_id = ? AND status = 1", in.TenantID, in.ChannelID).
		Count(&cnt).Error; err != nil {
		return nil, fmt.Errorf("校验渠道授权失败: %w", err)
	}
	if cnt == 0 {
		return nil, errors.New("当前租户未被授权使用该支付渠道")
	}

	// 查询渠道类型
	var ch model.PaymentChannel
	if err := s.db.WithContext(ctx).First(&ch, "id = ?", in.ChannelID).Error; err != nil {
		return nil, fmt.Errorf("渠道不存在: %w", err)
	}

	currency := in.Currency
	if currency == "" {
		currency = "CNY"
	}

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
	logger.WithContext(ctx).Info("支付订单已创建",
		zap.String("order_id", order.ID),
		zap.String("order_no", order.OrderNo),
		zap.Int64("amount", order.Amount),
	)
	return order, nil
}

// HandleCallback 处理支付回调（根据订单号幂等更新）
func (s *Service) HandleCallback(ctx context.Context, orderNo, tradeNo string) error {
	now := time.Now()
	res := s.db.WithContext(ctx).Model(&model.PaymentOrder{}).
		Where("order_no = ? AND status = ?", orderNo, model.OrderPending).
		Updates(map[string]any{
			"status":   model.OrderPaid,
			"trade_no": tradeNo,
			"paid_at":  now,
		})
	if res.Error != nil {
		return fmt.Errorf("更新订单状态失败: %w", res.Error)
	}
	if res.RowsAffected == 0 {
		return errors.New("订单状态非待支付，回调忽略")
	}
	return nil
}

// Refund 发起退款
func (s *Service) Refund(ctx context.Context, orderID string, amount int64) error {
	res := s.db.WithContext(ctx).Model(&model.PaymentOrder{}).
		Where("id = ? AND status = ?", orderID, model.OrderPaid).
		Update("status", model.OrderRefunding)
	if res.Error != nil {
		return fmt.Errorf("发起退款失败: %w", res.Error)
	}
	if res.RowsAffected == 0 {
		return errors.New("订单状态非已支付，无法退款")
	}
	// TODO: 调用第三方渠道实际退款接口
	logger.WithContext(ctx).Info("退款申请已提交",
		zap.String("order_id", orderID), zap.Int64("amount", amount),
	)
	return nil
}
