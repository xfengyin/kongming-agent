// OpenAI 兼容 Provider 适配（DeepSeek / 通义 / OpenAI 通用）
// 单一 OpenAI 兼容协议接入多家厂商，通过 BaseURL 区分。
//
// 实现基于 github.com/sashabaranov/go-openai，不再手写 HTTP 客户端：
// 鉴权、重试、超时、响应解析、错误分类等细节统一交给成熟库处理。

package llm

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"

	openai "github.com/sashabaranov/go-openai"
)

// 环境变量约定
const (
	EnvAPIKey   = "KONGMING_API_KEY"  // 必填：API Key
	EnvBaseURL  = "KONGMING_BASE_URL" // 可选：OpenAI 兼容 BaseURL，默认 https://api.openai.com/v1
	EnvModel    = "KONGMING_MODEL"    // 可选：模型名，默认 gpt-4o-mini
	EnvProvider = "KONGMING_PROVIDER" // 可选：Provider 显示名，默认 "openai-compatible"
)

// DefaultBaseURL OpenAI 官方
const DefaultBaseURL = "https://api.openai.com/v1"

// DefaultModel 默认模型
const DefaultModel = "gpt-4o-mini"

// OpenAIProvider 通过 OpenAI 兼容协议访问任意厂商。
// 内部使用 go-openai 的 Client 与默认 HTTP 配置（超时 120s）。
type OpenAIProvider struct {
	name    string
	apiKey  string
	baseURL string
	model   string
	client  *openai.Client
}

// NewOpenAIProvider 显式构造
func NewOpenAIProvider(apiKey, baseURL, model string) *OpenAIProvider {
	if baseURL == "" {
		baseURL = DefaultBaseURL
	}
	if model == "" {
		model = DefaultModel
	}
	cfg := openai.DefaultConfig(apiKey)
	cfg.BaseURL = strings.TrimRight(baseURL, "/")
	return &OpenAIProvider{
		name:    "openai-compatible",
		apiKey:  apiKey,
		baseURL: cfg.BaseURL,
		model:   model,
		client:  openai.NewClientWithConfig(cfg),
	}
}

// NewOpenAIProviderFromEnv 从环境变量构造；缺 Key 时返回 ErrNoAPIKey
func NewOpenAIProviderFromEnv() (*OpenAIProvider, error) {
	apiKey := os.Getenv(EnvAPIKey)
	if apiKey == "" {
		return nil, ErrNoAPIKey
	}
	baseURL := os.Getenv(EnvBaseURL)
	if baseURL == "" {
		baseURL = DefaultBaseURL
	}
	model := os.Getenv(EnvModel)
	if model == "" {
		model = DefaultModel
	}
	p := NewOpenAIProvider(apiKey, baseURL, model)
	if name := os.Getenv(EnvProvider); name != "" {
		p.name = name
	}
	return p, nil
}

// Name 提供者名称
func (p *OpenAIProvider) Name() string { return p.name }

// Model 当前模型
func (p *OpenAIProvider) Model() string { return p.model }

// Chat 调用 /chat/completions
func (p *OpenAIProvider) Chat(ctx context.Context, req ChatRequest) (*ChatResponse, error) {
	if req.Model == "" {
		req.Model = p.model
	}
	messages := make([]openai.ChatCompletionMessage, 0, len(req.Messages))
	for _, m := range req.Messages {
		messages = append(messages, openai.ChatCompletionMessage{
			Role:    string(m.Role),
			Content: m.Content,
		})
	}

	resp, err := p.client.CreateChatCompletion(ctx, openai.ChatCompletionRequest{
		Model:       req.Model,
		Messages:    messages,
		Temperature: float32(req.Temperature),
		MaxTokens:   req.MaxTokens,
	})
	if err != nil {
		return nil, wrapAPIError(p.name, err)
	}
	if len(resp.Choices) == 0 {
		return nil, fmt.Errorf("LLM API 返回空 choices")
	}
	return &ChatResponse{
		Content: resp.Choices[0].Message.Content,
		Model:   resp.Model,
	}, nil
}

// wrapAPIError 将 go-openai 的错误包装成 CLI 可读信息，保留 HTTP 状态码。
func wrapAPIError(providerName string, err error) error {
	var apiErr *openai.APIError
	if errors.As(err, &apiErr) {
		return fmt.Errorf("调用 %s 失败: LLM API 返回 %d: %s", providerName, apiErr.HTTPStatusCode, strings.TrimSpace(apiErr.Message))
	}
	return fmt.Errorf("调用 %s 失败: %w", providerName, err)
}
