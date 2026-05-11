// Package adapters 支付渠道适配器
package adapters

import (
	"context"
	"errors"

	"github.com/zhongjinmuai-lang/mu-framework/internal/model"
)

// PrepayRequest 下单入参
type PrepayRequest struct {
	OrderNo   string
	Amount    int64
	Currency  string
	Subject   string
	NotifyURL string
	ReturnURL string
	OpenID    string
	TradeType string
	UserIP    string
}

// PrepayResponse 下单结果
type PrepayResponse struct {
	PrepayID    string         `json:"prepay_id"`
	CodeURL     string         `json:"code_url,omitempty"`
	H5URL       string         `json:"h5_url,omitempty"`
	PayParams   map[string]any `json:"pay_params,omitempty"`
	RawResponse map[string]any `json:"raw,omitempty"`
}

// RefundRequest 退款入参
type RefundRequest struct {
	OrderNo  string
	RefundNo string
	Amount   int64
	Total    int64
	Reason   string
}

// CallbackResult 回调结果
type CallbackResult struct {
	OrderNo string
	TradeNo string
	Amount  int64
	Status  model.PaymentOrderStatus
	Raw     map[string]any
}

// Adapter 统一支付适配器接口
type Adapter interface {
	Type() model.PaymentChannelType
	Prepay(ctx context.Context, ch *model.PaymentChannel, req *PrepayRequest) (*PrepayResponse, error)
	Refund(ctx context.Context, ch *model.PaymentChannel, req *RefundRequest) error
	VerifyCallback(ctx context.Context, ch *model.PaymentChannel, headers map[string]string, rawBody []byte) (*CallbackResult, error)
}

// WechatPayAdapter 微信支付 V3（骨架，需对接 wechatpay-go）
type WechatPayAdapter struct{}

func NewWechatPayAdapter() *WechatPayAdapter               { return &WechatPayAdapter{} }
func (a *WechatPayAdapter) Type() model.PaymentChannelType { return model.PayChannelWechat }
func (a *WechatPayAdapter) Prepay(ctx context.Context, ch *model.PaymentChannel, req *PrepayRequest) (*PrepayResponse, error) {
	return nil, errors.New("WechatPayAdapter 待接入 wechatpay-apiv3/wechatpay-go")
}
func (a *WechatPayAdapter) Refund(ctx context.Context, ch *model.PaymentChannel, req *RefundRequest) error {
	return errors.New("WechatPayAdapter.Refund 待实现")
}
func (a *WechatPayAdapter) VerifyCallback(ctx context.Context, ch *model.PaymentChannel, headers map[string]string, raw []byte) (*CallbackResult, error) {
	return nil, errors.New("WechatPayAdapter.VerifyCallback 待实现")
}

// AlipayAdapter 支付宝适配器（骨架，需对接 smartwalle/alipay）
type AlipayAdapter struct{}

func NewAlipayAdapter() *AlipayAdapter                  { return &AlipayAdapter{} }
func (a *AlipayAdapter) Type() model.PaymentChannelType { return model.PayChannelAlipay }
func (a *AlipayAdapter) Prepay(ctx context.Context, ch *model.PaymentChannel, req *PrepayRequest) (*PrepayResponse, error) {
	return nil, errors.New("AlipayAdapter 待接入 smartwalle/alipay")
}
func (a *AlipayAdapter) Refund(ctx context.Context, ch *model.PaymentChannel, req *RefundRequest) error {
	return errors.New("AlipayAdapter.Refund 待实现")
}
func (a *AlipayAdapter) VerifyCallback(ctx context.Context, ch *model.PaymentChannel, headers map[string]string, raw []byte) (*CallbackResult, error) {
	return nil, errors.New("AlipayAdapter.VerifyCallback 待实现")
}

// SuixingPayAdapter 在 suixingpay.go 中完整实现
// 使用 NewSuixingPayAdapter() 创建
