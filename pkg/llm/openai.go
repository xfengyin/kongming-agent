// OpenAI 兼容 Provider 适配（DeepSeek / 通义 / OpenAI 通用）
// 单一 OpenAI 兼容协议接入多家厂商，通过 BaseURL 区分。

package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/zhuge/kongming/pkg/observatory"
)

// 环境变量约定
const (
	EnvAPIKey   = "KONGMING_API_KEY"  // 必填：API Key
	EnvBaseURL  = "KONGMING_BASE_URL" // 可选：OpenAI 兼容 BaseURL，默认 https://api.openai.com/v1
	EnvModel    = "KONGMING_MODEL"    // 可选：模型名，默认 gpt-4o-mini
	EnvProvider = "KONGMING_PROVIDER" // 可选：指标标签，默认 "openai"
)

// DefaultBaseURL OpenAI 官方
const DefaultBaseURL = "https://api.openai.com/v1"

// OpenAIProvider 通过 OpenAI 兼容协议访问任意厂商
type OpenAIProvider struct {
	name    string
	apiKey  string
	baseURL string
	model   string
	client  *http.Client
}

// NewOpenAIProvider 显式构造
func NewOpenAIProvider(apiKey, baseURL, model string) *OpenAIProvider {
	if baseURL == "" {
		baseURL = DefaultBaseURL
	}
	if model == "" {
		model = "gpt-4o-mini"
	}
	return &OpenAIProvider{
		name:    "openai-compatible",
		apiKey:  apiKey,
		baseURL: strings.TrimRight(baseURL, "/"),
		model:   model,
		client: &http.Client{
			Timeout: 120 * time.Second,
		},
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
		model = "gpt-4o-mini"
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
	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("序列化请求失败: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, p.baseURL+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("构造请求失败: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+p.apiKey)

	start := time.Now()
	resp, err := p.client.Do(httpReq)
	latency := time.Since(start).Seconds()
	if err != nil {
		observatory.RecordLLMCall(p.name, "error", latency)
		return nil, fmt.Errorf("调用 %s 失败: %w", p.name, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		observatory.RecordLLMCall(p.name, "error", latency)
		return nil, fmt.Errorf("LLM API 返回 %d: %s", resp.StatusCode, strings.TrimSpace(string(b)))
	}

	var payload struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
		Model string `json:"model"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		observatory.RecordLLMCall(p.name, "error", latency)
		return nil, fmt.Errorf("解析响应失败: %w", err)
	}
	observatory.RecordLLMCall(p.name, "success", latency)

	if len(payload.Choices) == 0 {
		return nil, fmt.Errorf("LLM API 返回空 choices")
	}
	return &ChatResponse{
		Content: payload.Choices[0].Message.Content,
		Model:   payload.Model,
	}, nil
}
