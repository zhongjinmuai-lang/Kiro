// Package adapters 随行付（天阙）支付适配器（v2.8）
//
// 随行付天阙开放平台 API 对接：
//   - 聚合支付（微信/支付宝/银联扫码）
//   - H5 收银台
//   - 退款
//   - 回调签名验证（MD5/SHA256）
//
// 文档参考：https://open.suixingpay.com（天阙开放平台）
// 接口规范：RESTful JSON + MD5/SHA256 签名
package adapters

import (
	"context"
	"crypto/md5"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/zhongjinmuai-lang/mu-framework/internal/model"
)

const (
	// 随行付天阙网关地址
	suixingPayGateway    = "https://openapi.suixingpay.com" // 生产
	suixingPayGatewayDev = "https://openapi.test.suixingpay.com" // 测试

	// API 路径
	suixingPayPathPrepay  = "/api/pay/unifiedorder"  // 统一下单
	suixingPayPathRefund  = "/api/pay/refund"        // 退款
	suixingPayPathQuery   = "/api/pay/query"         // 订单查询
)

// SuixingPayConfig 随行付渠道扩展配置（存储在 PaymentChannel.SecretKey JSON 中）
type SuixingPayConfig struct {
	OrgID      string `json:"org_id"`       // 机构号
	MerchantNo string `json:"merchant_no"`  // 商户号
	TerminalID string `json:"terminal_id"`  // 终端号
	SecretKey  string `json:"secret_key"`   // 签名密钥（MD5 Key）
	SignType   string `json:"sign_type"`    // 签名类型：MD5 / SHA256
	IsSandbox  bool   `json:"is_sandbox"`   // 是否沙箱环境
}

// SuixingPayAdapter 随行付（天阙）支付适配器
type SuixingPayAdapter struct {
	httpClient *http.Client
}

// NewSuixingPayAdapter 创建随行付适配器
func NewSuixingPayAdapter() *SuixingPayAdapter {
	return &SuixingPayAdapter{
		httpClient: &http.Client{Timeout: 30 * time.Second},
	}
}

func (a *SuixingPayAdapter) Type() model.PaymentChannelType {
	return model.PayChannelSuixing
}

// Prepay 随行付统一下单（聚合支付）
func (a *SuixingPayAdapter) Prepay(ctx context.Context, ch *model.PaymentChannel, req *PrepayRequest) (*PrepayResponse, error) {
	cfg, err := a.parseConfig(ch)
	if err != nil {
		return nil, err
	}

	// 构建请求参数
	params := map[string]string{
		"org_id":       cfg.OrgID,
		"merchant_no":  cfg.MerchantNo,
		"terminal_id":  cfg.TerminalID,
		"out_trade_no": req.OrderNo,
		"total_amount": fmt.Sprintf("%d", req.Amount), // 分
		"subject":      req.Subject,
		"notify_url":   req.NotifyURL,
		"timestamp":    time.Now().Format("20060102150405"),
		"nonce_str":    generateNonce()[:32],
	}

	// 支付方式映射
	switch strings.ToUpper(req.TradeType) {
	case "WECHAT", "JSAPI":
		params["pay_type"] = "WECHAT"
		if req.OpenID != "" {
			params["open_id"] = req.OpenID
		}
	case "ALIPAY":
		params["pay_type"] = "ALIPAY"
	case "UNION", "UNIONPAY":
		params["pay_type"] = "UNIONPAY"
	case "H5":
		params["pay_type"] = "H5"
		params["return_url"] = req.ReturnURL
	default:
		params["pay_type"] = "WECHAT" // 默认微信
	}

	if req.UserIP != "" {
		params["client_ip"] = req.UserIP
	}

	// 签名
	params["sign"] = a.sign(params, cfg.SecretKey, cfg.SignType)

	// 发送请求
	gateway := suixingPayGateway
	if cfg.IsSandbox {
		gateway = suixingPayGatewayDev
	}

	respBody, err := a.doPost(ctx, gateway+suixingPayPathPrepay, params)
	if err != nil {
		return nil, fmt.Errorf("随行付下单请求失败: %w", err)
	}

	// 解析响应
	var result map[string]interface{}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("解析随行付响应失败: %w", err)
	}

	// 检查返回码
	code, _ := result["resp_code"].(string)
	if code != "00" && code != "SUCCESS" {
		msg, _ := result["resp_msg"].(string)
		return nil, fmt.Errorf("随行付下单失败: [%s] %s", code, msg)
	}

	resp := &PrepayResponse{RawResponse: result}

	// 提取支付凭证
	if payInfo, ok := result["pay_info"].(string); ok {
		resp.CodeURL = payInfo // 二维码链接或支付跳转URL
	}
	if h5URL, ok := result["h5_url"].(string); ok {
		resp.H5URL = h5URL
	}
	if prepayID, ok := result["prepay_id"].(string); ok {
		resp.PrepayID = prepayID
	}
	// 小程序/JSAPI 调起支付参数
	if payParams, ok := result["pay_params"].(map[string]interface{}); ok {
		resp.PayParams = payParams
	}

	return resp, nil
}

// Refund 随行付退款
func (a *SuixingPayAdapter) Refund(ctx context.Context, ch *model.PaymentChannel, req *RefundRequest) error {
	cfg, err := a.parseConfig(ch)
	if err != nil {
		return err
	}

	params := map[string]string{
		"org_id":        cfg.OrgID,
		"merchant_no":   cfg.MerchantNo,
		"terminal_id":   cfg.TerminalID,
		"out_trade_no":  req.OrderNo,
		"out_refund_no": req.RefundNo,
		"refund_amount": fmt.Sprintf("%d", req.Amount),
		"total_amount":  fmt.Sprintf("%d", req.Total),
		"refund_reason": req.Reason,
		"timestamp":     time.Now().Format("20060102150405"),
		"nonce_str":     generateNonce()[:32],
	}
	params["sign"] = a.sign(params, cfg.SecretKey, cfg.SignType)

	gateway := suixingPayGateway
	if cfg.IsSandbox {
		gateway = suixingPayGatewayDev
	}

	respBody, err := a.doPost(ctx, gateway+suixingPayPathRefund, params)
	if err != nil {
		return fmt.Errorf("随行付退款请求失败: %w", err)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return fmt.Errorf("解析随行付退款响应失败: %w", err)
	}

	code, _ := result["resp_code"].(string)
	if code != "00" && code != "SUCCESS" {
		msg, _ := result["resp_msg"].(string)
		return fmt.Errorf("随行付退款失败: [%s] %s", code, msg)
	}

	return nil
}

// VerifyCallback 验证随行付回调签名
func (a *SuixingPayAdapter) VerifyCallback(ctx context.Context, ch *model.PaymentChannel, headers map[string]string, rawBody []byte) (*CallbackResult, error) {
	cfg, err := a.parseConfig(ch)
	if err != nil {
		return nil, err
	}

	// 解析回调参数
	var params map[string]string
	if err := json.Unmarshal(rawBody, &params); err != nil {
		return nil, fmt.Errorf("解析随行付回调数据失败: %w", err)
	}

	// 提取并验证签名
	receivedSign := params["sign"]
	delete(params, "sign")
	expectedSign := a.sign(params, cfg.SecretKey, cfg.SignType)

	if !strings.EqualFold(receivedSign, expectedSign) {
		return nil, fmt.Errorf("随行付回调签名验证失败: expected=%s, received=%s", expectedSign, receivedSign)
	}

	// 检查交易状态
	tradeStatus := params["trade_status"]
	if tradeStatus != "SUCCESS" && tradeStatus != "TRADE_SUCCESS" {
		return nil, fmt.Errorf("交易状态非成功: %s", tradeStatus)
	}

	// 解析金额（分）
	var amount int64
	if v, ok := params["total_amount"]; ok {
		fmt.Sscanf(v, "%d", &amount)
	}

	return &CallbackResult{
		OrderNo: params["out_trade_no"],
		TradeNo: params["trade_no"],
		Amount:  amount,
		Status:  model.OrderPaid,
		Raw:     map[string]any{"trade_status": tradeStatus, "pay_type": params["pay_type"]},
	}, nil
}

// ========== 内部方法 ==========

// parseConfig 从 PaymentChannel 解析随行付配置
// SecretKey 字段存储 JSON 格式的完整配置
func (a *SuixingPayAdapter) parseConfig(ch *model.PaymentChannel) (*SuixingPayConfig, error) {
	cfg := &SuixingPayConfig{}

	// 尝试从 SecretKey 字段解析 JSON 配置
	if ch.SecretKey != "" {
		if err := json.Unmarshal([]byte(ch.SecretKey), cfg); err != nil {
			// 非 JSON 格式，当作纯密钥处理
			cfg.SecretKey = ch.SecretKey
			cfg.MerchantNo = ch.MerchantID
			cfg.OrgID = ch.AppID
		}
	}

	if cfg.MerchantNo == "" {
		cfg.MerchantNo = ch.MerchantID
	}
	if cfg.OrgID == "" {
		cfg.OrgID = ch.AppID
	}
	if cfg.SignType == "" {
		cfg.SignType = "MD5"
	}

	if cfg.MerchantNo == "" || cfg.SecretKey == "" {
		return nil, fmt.Errorf("随行付配置不完整: 需要 merchant_no + secret_key")
	}

	return cfg, nil
}

// sign 随行付签名（参数按 ASCII 排序 + key 拼接 + MD5/SHA256）
func (a *SuixingPayAdapter) sign(params map[string]string, key, signType string) string {
	// 1. 参数按 key ASCII 排序
	keys := make([]string, 0, len(params))
	for k := range params {
		if k != "sign" && params[k] != "" {
			keys = append(keys, k)
		}
	}
	sort.Strings(keys)

	// 2. 拼接 key=value&
	var sb strings.Builder
	for i, k := range keys {
		if i > 0 {
			sb.WriteByte('&')
		}
		sb.WriteString(k)
		sb.WriteByte('=')
		sb.WriteString(params[k])
	}
	// 3. 拼接密钥
	sb.WriteString("&key=")
	sb.WriteString(key)

	signStr := sb.String()

	// 4. 计算签名
	switch strings.ToUpper(signType) {
	case "SHA256":
		hash := sha256.Sum256([]byte(signStr))
		return strings.ToUpper(hex.EncodeToString(hash[:]))
	default: // MD5
		hash := md5.Sum([]byte(signStr))
		return strings.ToUpper(hex.EncodeToString(hash[:]))
	}
}

// doPost 发送 POST JSON 请求
func (a *SuixingPayAdapter) doPost(ctx context.Context, url string, params map[string]string) ([]byte, error) {
	body, _ := json.Marshal(params)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, strings.NewReader(string(body)))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := a.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("随行付 HTTP %d: %s", resp.StatusCode, string(respBody))
	}

	return respBody, nil
}
