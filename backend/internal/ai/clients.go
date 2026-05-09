package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// openaiCompatRequest 兼容 OpenAI ChatCompletion 的请求体
type openaiCompatRequest struct {
	Model       string    `json:"model"`
	Messages    []Message `json:"messages"`
	Temperature float32   `json:"temperature,omitempty"`
	MaxTokens   int       `json:"max_tokens,omitempty"`
	Stream      bool      `json:"stream,omitempty"`
}

// openaiCompatResponse OpenAI 兼容响应
type openaiCompatResponse struct {
	ID      string `json:"id"`
	Model   string `json:"model"`
	Choices []struct {
		Index   int `json:"index"`
		Message struct {
			Role    string `json:"role"`
			Content string `json:"content"`
		} `json:"message"`
		FinishReason string `json:"finish_reason"`
	} `json:"choices"`
	Usage struct {
		PromptTokens     int `json:"prompt_tokens"`
		CompletionTokens int `json:"completion_tokens"`
		TotalTokens      int `json:"total_tokens"`
	} `json:"usage"`
}

// openaiCompatClient 通用 OpenAI 兼容客户端
type openaiCompatClient struct {
	provider     Provider
	endpoint     string
	apiKey       string
	defaultModel string
	httpClient   *http.Client
}

func (c *openaiCompatClient) Provider() Provider { return c.provider }

// Health 轻量探测
func (c *openaiCompatClient) Health(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet,
		strings.TrimSuffix(c.endpoint, "/chat/completions"), nil)
	if err != nil {
		return err
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 500 {
		return fmt.Errorf("AI 供应商 %s 不可用: HTTP %d", c.provider, resp.StatusCode)
	}
	return nil
}

// Chat 发起对话
func (c *openaiCompatClient) Chat(ctx context.Context, req *ChatRequest) (*ChatResponse, error) {
	if c.apiKey == "" {
		return nil, fmt.Errorf("AI 供应商 %s 未配置 apiKey", c.provider)
	}
	if len(req.Messages) == 0 {
		return nil, errors.New("messages 不能为空")
	}
	model := req.Model
	if model == "" {
		model = c.defaultModel
	}
	body := openaiCompatRequest{
		Model:       model,
		Messages:    req.Messages,
		Temperature: req.Temperature,
		MaxTokens:   req.MaxTokens,
	}
	raw, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.endpoint, bytes.NewReader(raw))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+c.apiKey)
	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("AI 调用失败 %s: %w", c.provider, err)
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("AI %s 返回错误 HTTP %d: %s", c.provider, resp.StatusCode, string(data))
	}
	var parsed openaiCompatResponse
	if err := json.Unmarshal(data, &parsed); err != nil {
		return nil, fmt.Errorf("解析 AI 响应失败: %w", err)
	}
	if len(parsed.Choices) == 0 {
		return nil, errors.New("AI 返回空结果")
	}
	return &ChatResponse{
		ID:      parsed.ID,
		Model:   parsed.Model,
		Content: parsed.Choices[0].Message.Content,
		Usage: Usage{
			PromptTokens:     parsed.Usage.PromptTokens,
			CompletionTokens: parsed.Usage.CompletionTokens,
			TotalTokens:      parsed.Usage.TotalTokens,
		},
	}, nil
}

// ========== 预置供应商 ==========

// NewDeepSeekClient DeepSeek
func NewDeepSeekClient(apiKey string) Client {
	return &openaiCompatClient{
		provider: ProviderDeepSeek, endpoint: "https://api.deepseek.com/v1/chat/completions",
		apiKey: apiKey, defaultModel: "deepseek-chat", httpClient: defaultHTTPClient(),
	}
}

// NewDoubaoClient 豆包（火山方舟）
func NewDoubaoClient(apiKey, defaultModel string) Client {
	if defaultModel == "" {
		defaultModel = "doubao-pro-32k"
	}
	return &openaiCompatClient{
		provider: ProviderDoubao, endpoint: "https://ark.cn-beijing.volces.com/api/v3/chat/completions",
		apiKey: apiKey, defaultModel: defaultModel, httpClient: defaultHTTPClient(),
	}
}

// NewTongyiClient 通义千问（DashScope 兼容）
func NewTongyiClient(apiKey, defaultModel string) Client {
	if defaultModel == "" {
		defaultModel = "qwen-plus"
	}
	return &openaiCompatClient{
		provider: ProviderTongyi, endpoint: "https://dashscope.aliyuncs.com/compatible-mode/v1/chat/completions",
		apiKey: apiKey, defaultModel: defaultModel, httpClient: defaultHTTPClient(),
	}
}

// NewWenxinClient 文心一言（千帆）
func NewWenxinClient(apiKey, defaultModel string) Client {
	if defaultModel == "" {
		defaultModel = "ernie-4.0-8k"
	}
	return &openaiCompatClient{
		provider: ProviderWenxin, endpoint: "https://qianfan.baidubce.com/v2/chat/completions",
		apiKey: apiKey, defaultModel: defaultModel, httpClient: defaultHTTPClient(),
	}
}

// NewPrivateClient 企业私有部署
func NewPrivateClient(endpoint, apiKey, defaultModel string) Client {
	return &openaiCompatClient{
		provider: ProviderPrivate, endpoint: endpoint,
		apiKey: apiKey, defaultModel: defaultModel, httpClient: defaultHTTPClient(),
	}
}

func defaultHTTPClient() *http.Client {
	return &http.Client{
		Timeout: 60 * time.Second,
		Transport: &http.Transport{
			MaxIdleConns:        32,
			MaxIdleConnsPerHost: 8,
			IdleConnTimeout:     90 * time.Second,
		},
	}
}
