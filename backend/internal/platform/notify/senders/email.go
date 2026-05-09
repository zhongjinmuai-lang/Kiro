// Package senders 通知中台发送器实现
package senders

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/smtp"
	"strings"

	"github.com/zhongjinmuai-lang/mu-framework/internal/model"
)

// EmailConfig SMTP 邮件配置
type EmailConfig struct {
	Host     string `json:"host"`
	Port     int    `json:"port"`
	Username string `json:"username"`
	Password string `json:"password"`
	From     string `json:"from"`
	UseSSL   bool   `json:"use_ssl"`
	Subject  string `json:"subject"`
}

// EmailSender SMTP 邮件发送器
type EmailSender struct{}

// NewEmailSender 构造
func NewEmailSender() *EmailSender { return &EmailSender{} }

// Type 通道类型
func (s *EmailSender) Type() model.NotifyChannelType { return model.NotifyEmail }

// Send 发送邮件
func (s *EmailSender) Send(ctx context.Context, receiver, content string, raw json.RawMessage) error {
	var cfg EmailConfig
	if err := json.Unmarshal(raw, &cfg); err != nil {
		return fmt.Errorf("解析邮件配置失败: %w", err)
	}
	if cfg.Host == "" || cfg.Port == 0 || cfg.Username == "" {
		return errors.New("邮件配置缺失: host/port/username")
	}
	if cfg.From == "" {
		cfg.From = cfg.Username
	}
	subject := cfg.Subject
	if subject == "" {
		subject = "来自 MU 平台的通知"
	}
	mime := "text/plain; charset=UTF-8"
	if strings.HasPrefix(strings.TrimSpace(content), "<html") {
		mime = "text/html; charset=UTF-8"
	}
	msg := []byte(fmt.Sprintf(
		"To: %s\r\nFrom: %s\r\nSubject: %s\r\nMIME-Version: 1.0\r\nContent-Type: %s\r\n\r\n%s",
		receiver, cfg.From, subject, mime, content,
	))
	addr := fmt.Sprintf("%s:%d", cfg.Host, cfg.Port)
	auth := smtp.PlainAuth("", cfg.Username, cfg.Password, cfg.Host)
	return smtp.SendMail(addr, auth, cfg.Username, []string{receiver}, msg)
}
