package payment

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/zhongjinmuai-lang/mu-framework/internal/saas/hierarchy"
)

// ChannelType 支付渠道类型
type ChannelType string

const (
	ChannelWechat  ChannelType = "wechat"  // 微信支付
	ChannelAlipay  ChannelType = "alipay"  // 支付宝
	ChannelUnion   ChannelType = "union"   // 银联
	ChannelStripe  ChannelType = "stripe"  // Stripe
)

// OrderStatus 订单状态
type OrderStatus int

const (
	OrderPending   OrderStatus = 0 // 待支付
	OrderPaid      OrderStatus = 1 // 已支付
	OrderRefunding OrderStatus = 2 // 退款中
	OrderRefunded  OrderStatus = 3 // 已退款
	OrderClosed    OrderStatus = 4 // 已关闭
)

// Channel 支付渠道配置
type Channel struct {
	ID         string      `json:"id" db:"id"`
	TenantID   string      `json:"tenant_id" db:"tenant_id"`     // 配置方租户
	Level      string      `json:"level" db:"level"`             // 配置层级
	Type       ChannelType `json:"type" db:"type"`
	Name       string      `json:"name" db:"name"`
	AppID      string      `json:"app_id" db:"app_id"`
	MerchantID string      `json:"merchant_id" db:"merchant_id"`
	SecretKey  string      `json:"-" db:"secret_key"`            // 密钥不对外暴露
	NotifyURL  string      `json:"notify_url" db:"notify_url"`
	Status     int         `json:"status" db:"status"`
	CreatedAt  time.Time   `json:"created_at" db:"created_at"`
	UpdatedAt  time.Time   `json:"updated_at" db:"updated_at"`
}

// Order 支付订单
type Order struct {
	ID          string      `json:"id" db:"id"`
	TenantID    string      `json:"tenant_id" db:"tenant_id"`
	ChannelID   string      `json:"channel_id" db:"channel_id"`
	ChannelType ChannelType `json:"channel_type" db:"channel_type"`
	OrderNo     string      `json:"order_no" db:"order_no"`       // 业务订单号
	TradeNo     string      `json:"trade_no" db:"trade_no"`       // 第三方交易号
	Amount      int64       `json:"amount" db:"amount"`           // 金额（分）
	Currency    string      `json:"currency" db:"currency"`       // 币种
	Subject     string      `json:"subject" db:"subject"`         // 订单标题
	Status      OrderStatus `json:"status" db:"status"`
	PaidAt      *time.Time  `json:"paid_at" db:"paid_at"`
	CreatedAt   time.Time   `json:"created_at" db:"created_at"`
	UpdatedAt   time.Time   `json:"updated_at" db:"updated_at"`
}

// Service 支付中台服务
type Service struct {
	db        *pgxpool.Pool
	hierarchy *hierarchy.Service
	logger    *slog.Logger
}

// NewService 创建支付中台服务
func NewService(db *pgxpool.Pool, hierarchySvc *hierarchy.Service) *Service {
	return &Service{
		db:        db,
		hierarchy: hierarchySvc,
		logger:    slog.Default().With("module", "payment"),
	}
}

// CreateChannelInput 创建支付渠道入参
type CreateChannelInput struct {
	TenantID   string      `json:"tenant_id"`
	Type       ChannelType `json:"type"`
	Name       string      `json:"name"`
	AppID      string      `json:"app_id"`
	MerchantID string      `json:"merchant_id"`
	SecretKey  string      `json:"secret_key"`
	NotifyURL  string      `json:"notify_url"`
}

// CreateChannel 创建支付渠道（仅开发商可创建顶级渠道）
func (s *Service) CreateChannel(ctx context.Context, operatorLevel string, input *CreateChannelInput) (*Channel, error) {
	// 校验权限：只有开发商可以创建支付渠道
	if !s.hierarchy.CanAccess(operatorLevel, hierarchy.LevelDeveloper) {
		return nil, fmt.Errorf("仅开发商层级可创建支付渠道")
	}

	channel := &Channel{
		ID:         uuid.New().String(),
		TenantID:   input.TenantID,
		Level:      operatorLevel,
		Type:       input.Type,
		Name:       input.Name,
		AppID:      input.AppID,
		MerchantID: input.MerchantID,
		SecretKey:  input.SecretKey,
		NotifyURL:  input.NotifyURL,
		Status:     1,
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
	}

	query := `INSERT INTO payment_channels (id, tenant_id, level, type, name, app_id, merchant_id, secret_key, notify_url, status, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)`

	_, err := s.db.Exec(ctx, query,
		channel.ID, channel.TenantID, channel.Level, channel.Type,
		channel.Name, channel.AppID, channel.MerchantID, channel.SecretKey,
		channel.NotifyURL, channel.Status, channel.CreatedAt, channel.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("创建支付渠道失败: %w", err)
	}

	s.logger.Info("支付渠道创建成功", "id", channel.ID, "type", channel.Type)
	return channel, nil
}

// GetAvailableChannels 获取租户可用的支付渠道
// 三级权限管控：终端客户只能使用服务商授权的渠道，服务商只能使用开发商配置的渠道
func (s *Service) GetAvailableChannels(ctx context.Context, tenantID string) ([]*Channel, error) {
	// 获取租户的上级链路，查找所有可用渠道
	query := `SELECT pc.id, pc.tenant_id, pc.level, pc.type, pc.name, pc.app_id, pc.merchant_id, pc.notify_url, pc.status, pc.created_at, pc.updated_at
		FROM payment_channels pc
		INNER JOIN tenant_payment_auth tpa ON pc.id = tpa.channel_id
		WHERE tpa.tenant_id = $1 AND tpa.status = 1 AND pc.status = 1`

	rows, err := s.db.Query(ctx, query, tenantID)
	if err != nil {
		return nil, fmt.Errorf("查询可用支付渠道失败: %w", err)
	}
	defer rows.Close()

	var channels []*Channel
	for rows.Next() {
		ch := &Channel{}
		if err := rows.Scan(
			&ch.ID, &ch.TenantID, &ch.Level, &ch.Type, &ch.Name,
			&ch.AppID, &ch.MerchantID, &ch.NotifyURL, &ch.Status,
			&ch.CreatedAt, &ch.UpdatedAt,
		); err != nil {
			return nil, err
		}
		channels = append(channels, ch)
	}

	return channels, nil
}

// CreateOrderInput 创建订单入参
type CreateOrderInput struct {
	TenantID  string `json:"tenant_id"`
	ChannelID string `json:"channel_id"`
	OrderNo   string `json:"order_no"`
	Amount    int64  `json:"amount"`
	Currency  string `json:"currency"`
	Subject   string `json:"subject"`
}

// CreateOrder 创建支付订单（统一下单）
func (s *Service) CreateOrder(ctx context.Context, input *CreateOrderInput) (*Order, error) {
	order := &Order{
		ID:        uuid.New().String(),
		TenantID:  input.TenantID,
		ChannelID: input.ChannelID,
		OrderNo:   input.OrderNo,
		Amount:    input.Amount,
		Currency:  input.Currency,
		Subject:   input.Subject,
		Status:    OrderPending,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	query := `INSERT INTO payment_orders (id, tenant_id, channel_id, order_no, amount, currency, subject, status, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)`

	_, err := s.db.Exec(ctx, query,
		order.ID, order.TenantID, order.ChannelID, order.OrderNo,
		order.Amount, order.Currency, order.Subject, order.Status,
		order.CreatedAt, order.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("创建支付订单失败: %w", err)
	}

	s.logger.Info("支付订单创建成功", "order_id", order.ID, "amount", order.Amount)
	return order, nil
}

// HandleCallback 处理支付回调
func (s *Service) HandleCallback(ctx context.Context, channelID, tradeNo string) error {
	query := `UPDATE payment_orders SET status = $1, trade_no = $2, paid_at = $3, updated_at = $3
		WHERE channel_id = $4 AND status = $5`

	now := time.Now()
	_, err := s.db.Exec(ctx, query, OrderPaid, tradeNo, now, channelID, OrderPending)
	if err != nil {
		return fmt.Errorf("更新订单状态失败: %w", err)
	}

	return nil
}

// Refund 退款
func (s *Service) Refund(ctx context.Context, orderID string, amount int64) error {
	query := `UPDATE payment_orders SET status = $1, updated_at = $2 WHERE id = $3 AND status = $4`
	_, err := s.db.Exec(ctx, query, OrderRefunding, time.Now(), orderID, OrderPaid)
	if err != nil {
		return fmt.Errorf("发起退款失败: %w", err)
	}

	// TODO: 调用第三方退款接口
	s.logger.Info("退款申请已提交", "order_id", orderID, "amount", amount)
	return nil
}
