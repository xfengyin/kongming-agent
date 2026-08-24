// Mock Provider - 无网络的本地假实现（测试与离线演示用）

package llm

import (
	"context"
	"fmt"
	"strings"
)

// MockProvider 返回固定的剧本式回复，便于测试与离线演示
type MockProvider struct {
	// Reply 可覆盖默认回复；为空时按请求内容拼装
	Reply string

	calls int
}

// Name 提供者名称
func (m *MockProvider) Name() string { return "mock" }

// Chat 返回模拟回复
func (m *MockProvider) Chat(ctx context.Context, req ChatRequest) (*ChatResponse, error) {
	m.calls++
	if m.Reply != "" {
		return &ChatResponse{Content: m.Reply, Model: req.Model}, nil
	}
	last := ""
	if len(req.Messages) > 0 {
		last = req.Messages[len(req.Messages)-1].Content
	}
	trimmed := strings.TrimSpace(last)
	if trimmed == "" {
		trimmed = "（空问题）"
	}
	// 多轮演示：汇报收到的消息条数，便于观察历史透传
	return &ChatResponse{
		Content: fmt.Sprintf("[mock] 主公问：%s\n亮答：此乃天机，容亮细想——离线演示模式，请配置 KONGMING_API_KEY 接入真实 LLM。（本轮收到 %d 条消息）", trimmed, len(req.Messages)),
		Model:   "mock-model",
	}, nil
}

// Calls 已调用次数（测试断言用）
func (m *MockProvider) Calls() int { return m.calls }
