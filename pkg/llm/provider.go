// LLM Provider 适配层 - 驱动式接入各家大模型
// 运筹帷幄之中，决胜千里之外
//
// 设计原则（见 docs/architecture 合理限制文档）：
//   - 只定义最小接口，不绑定任何厂商；
//   - 模型能力完全委托外部 API，不做本地推理、不训练不微调；
//   - 首选 OpenAI 兼容协议（DeepSeek / 通义 / OpenAI 同一接口）。

package llm

import (
	"context"
	"errors"
)

// ErrNoAPIKey 表示未配置 API Key
var ErrNoAPIKey = errors.New("未配置 LLM API Key（请设置 KONGMING_API_KEY 环境变量）")

// Role 对话角色
type Role string

const (
	RoleSystem    Role = "system"    // 系统提示（锦囊/人设）
	RoleUser      Role = "user"      // 用户（主公）
	RoleAssistant Role = "assistant" // 模型回复
)

// Message 单轮对话消息
type Message struct {
	Role    Role   `json:"role"`
	Content string `json:"content"`
}

// ChatRequest 对话请求
type ChatRequest struct {
	Model       string    `json:"model"`
	Messages    []Message `json:"messages"`
	Temperature float64   `json:"temperature,omitempty"`
	MaxTokens   int       `json:"max_tokens,omitempty"`
}

// ChatResponse 对话响应
type ChatResponse struct {
	Content string `json:"content"`
	Model   string `json:"model"`
}

// Provider LLM 提供者接口
type Provider interface {
	// Name 提供者名称（用于日志/指标）
	Name() string

	// Chat 完成一轮对话，返回模型回复文本
	Chat(ctx context.Context, req ChatRequest) (*ChatResponse, error)
}
