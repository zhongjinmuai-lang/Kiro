package model

import "time"

// PaymentChannelType 支付渠道类型
type PaymentChannelType string

const (
	PayChannelWechat  PaymentChannelType = "wechat"  // 微信支付
	PayChannelAlipay  PaymentChannelType = "alipay"  // 支付宝
	PayChannelUnion   PaymentChannelType = "union"   // 银联
	PayChannelStripe  PaymentChannelType = "stripe"  // Stripe
	PayChannelSuixing PaymentChannelType = "suixing" // 随行付（天阙）
)

// PaymentOrderStatus 订单状态
type PaymentOrderStatus int

const (
	OrderPending   PaymentOrderStatus = 0 // 待支付
	OrderPaid      PaymentOrderStatus = 1 // 已支付
	OrderRefunding PaymentOrderStatus = 2 // 退款中
	OrderRefunded  PaymentOrderStatus = 3 // 已退款
	OrderClosed    PaymentOrderStatus = 4 // 已关闭
)

// PaymentChannel 支付渠道配置（开发商配置，逐级授权）
type PaymentChannel struct {
	BaseModel
	TenantID   string             `gorm:"column:tenant_id;type:uuid;not null;index" json:"tenant_id"`
	Level      TenantLevel        `gorm:"column:level;type:varchar(20);not null" json:"level"`
	Type       PaymentChannelType `gorm:"column:type;type:varchar(20);not null" json:"type"`
	Name       string             `gorm:"column:name;type:varchar(100);not null" json:"name"`
	AppID      string             `gorm:"column:app_id;type:varchar(100)" json:"app_id"`
	MerchantID string             `gorm:"column:merchant_id;type:varchar(100)" json:"merchant_id"`
	SecretKey  string             `gorm:"column:secret_key;type:text" json:"-"`
	NotifyURL  string             `gorm:"column:notify_url;type:varchar(500)" json:"notify_url"`
	Status     int                `gorm:"column:status;type:smallint;not null;default:1" json:"status"`
}

func (PaymentChannel) TableName() string { return "payment_channels" }

// TenantPaymentAuth 支付渠道授权（三级管控：上级授予下级可用渠道）
type TenantPaymentAuth struct {
	BaseModel
	TenantID  string  `gorm:"column:tenant_id;type:uuid;not null;uniqueIndex:idx_pay_auth,priority:1" json:"tenant_id"`
	ChannelID string  `gorm:"column:channel_id;type:uuid;not null;uniqueIndex:idx_pay_auth,priority:2" json:"channel_id"`
	GrantedBy *string `gorm:"column:granted_by;type:uuid" json:"granted_by,omitempty"`
	Status    int     `gorm:"column:status;type:smallint;not null;default:1" json:"status"`
}

func (TenantPaymentAuth) TableName() string { return "tenant_payment_auth" }

// PaymentOrder 支付订单
type PaymentOrder struct {
	BaseModel
	TenantID    string             `gorm:"column:tenant_id;type:uuid;not null;index:idx_orders_tenant,priority:1" json:"tenant_id"`
	ChannelID   string             `gorm:"column:channel_id;type:uuid;not null" json:"channel_id"`
	ChannelType PaymentChannelType `gorm:"column:channel_type;type:varchar(20)" json:"channel_type"`
	OrderNo     string             `gorm:"column:order_no;type:varchar(64);not null;uniqueIndex" json:"order_no"`
	TradeNo     string             `gorm:"column:trade_no;type:varchar(64)" json:"trade_no"`
	Amount      int64              `gorm:"column:amount;type:bigint;not null" json:"amount"` // 分
	Currency    string             `gorm:"column:currency;type:varchar(10);not null;default:'CNY'" json:"currency"`
	Subject     string             `gorm:"column:subject;type:varchar(200)" json:"subject"`
	Status      PaymentOrderStatus `gorm:"column:status;type:smallint;not null;default:0" json:"status"`
	PaidAt      *time.Time         `gorm:"column:paid_at" json:"paid_at,omitempty"`
}

func (PaymentOrder) TableName() string { return "payment_orders" }
