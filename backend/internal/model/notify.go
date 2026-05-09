package model

import "time"

// NotifyChannelType 通知通道类型
type NotifyChannelType string

const (
	NotifySMS       NotifyChannelType = "sms"
	NotifyEmail     NotifyChannelType = "email"
	NotifyPush      NotifyChannelType = "push"      // App推送
	NotifyWechat    NotifyChannelType = "wechat"    // 微信模板消息
	NotifyWebSocket NotifyChannelType = "websocket" // 站内信实时推送
)

// NotifyMessageStatus 消息状态
type NotifyMessageStatus int

const (
	MsgPending   NotifyMessageStatus = 0 // 待发送
	MsgSending   NotifyMessageStatus = 1 // 发送中
	MsgSent      NotifyMessageStatus = 2 // 已发送
	MsgFailed    NotifyMessageStatus = 3 // 失败
	MsgRetrying  NotifyMessageStatus = 4 // 重试中
)

// NotifyChannel 通知通道配置
type NotifyChannel struct {
	BaseModel
	TenantID string            `gorm:"column:tenant_id;type:uuid;not null;index:idx_notify_ch_tenant,priority:1" json:"tenant_id"`
	Level    TenantLevel       `gorm:"column:level;type:varchar(20);not null" json:"level"`
	Type     NotifyChannelType `gorm:"column:type;type:varchar(20);not null;index:idx_notify_ch_tenant,priority:2" json:"type"`
	Name     string            `gorm:"column:name;type:varchar(100);not null" json:"name"`
	Config   string            `gorm:"column:config;type:jsonb;not null;default:'{}'" json:"config"`
	Status   int               `gorm:"column:status;type:smallint;not null;default:1" json:"status"`
}

func (NotifyChannel) TableName() string { return "notify_channels" }

// NotifyTemplate 通知模板
type NotifyTemplate struct {
	BaseModel
	TenantID string            `gorm:"column:tenant_id;type:uuid;not null;uniqueIndex:idx_tpl_code,priority:1" json:"tenant_id"`
	Channel  NotifyChannelType `gorm:"column:channel;type:varchar(20);not null" json:"channel"`
	Code     string            `gorm:"column:code;type:varchar(50);not null;uniqueIndex:idx_tpl_code,priority:2" json:"code"`
	Name     string            `gorm:"column:name;type:varchar(100);not null" json:"name"`
	Content  string            `gorm:"column:content;type:text;not null" json:"content"` // 支持 {{变量}}
	Status   int               `gorm:"column:status;type:smallint;not null;default:1" json:"status"`
}

func (NotifyTemplate) TableName() string { return "notify_templates" }

// NotifyMessage 消息发送记录
type NotifyMessage struct {
	BaseModel
	TenantID   string              `gorm:"column:tenant_id;type:uuid;not null;index" json:"tenant_id"`
	ChannelID  string              `gorm:"column:channel_id;type:uuid;not null" json:"channel_id"`
	TemplateID *string             `gorm:"column:template_id;type:uuid" json:"template_id,omitempty"`
	Receiver   string              `gorm:"column:receiver;type:varchar(200);not null" json:"receiver"`
	Content    string              `gorm:"column:content;type:text;not null" json:"content"`
	Status     NotifyMessageStatus `gorm:"column:status;type:smallint;not null;default:0;index" json:"status"`
	RetryCount int                 `gorm:"column:retry_count;type:smallint;not null;default:0" json:"retry_count"`
	SentAt     *time.Time          `gorm:"column:sent_at" json:"sent_at,omitempty"`
}

func (NotifyMessage) TableName() string { return "notify_messages" }
