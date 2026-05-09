package senders

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/zhongjinmuai-lang/mu-framework/internal/model"
)

// SMSConfig 短信通道配置
type SMSConfig struct {
	Endpoint string `json:"endpoint"`
	APIKey   string `json:"api_key"`

	AccessKeyID     string `json:"access_key_id"`
	AccessKeySecret string `json:"access_key_secret"`
	SignName        string `json:"sign_name"`
	TemplateCode    string `json:"template_code"`
	Region          string `json:"region"`
}

// SMSSender 短信发送器
type SMSSender struct {
	httpClient *http.Client
}

// NewSMSSender 构造
func NewSMSSender() *SMSSender {
	return &SMSSender{httpClient: &http.Client{Timeout: 15 * time.Second}}
}

// Type 通道类型
func (s *SMSSender) Type() model.NotifyChannelType { return model.NotifySMS }

// Send 发送短信
func (s *SMSSender) Send(ctx context.Context, receiver, content string, raw json.RawMessage) error {
	var cfg SMSConfig
	if err := json.Unmarshal(raw, &cfg); err != nil {
		return fmt.Errorf("解析短信配置失败: %w", err)
	}
	if cfg.Endpoint != "" {
		return s.sendViaGateway(ctx, &cfg, receiver, content)
	}
	if cfg.AccessKeyID == "" {
		return errors.New("短信配置缺失：endpoint 或 AccessKeyID 必填其一")
	}
	return errors.New("阿里云/腾讯云 SMS SDK 适配器待接入")
}

func (s *SMSSender) sendViaGateway(ctx context.Context, cfg *SMSConfig, receiver, content string) error {
	payload, _ := json.Marshal(map[string]any{"phone": receiver, "content": content})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, cfg.Endpoint, bytes.NewReader(payload))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	if cfg.APIKey != "" {
		req.Header.Set("Authorization", "Bearer "+cfg.APIKey)
	}
	resp, err := s.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("短信网关调用失败: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("短信网关返回 HTTP %d: %s", resp.StatusCode, string(body))
	}
	return nil
}
