// LLM Provider 适配层测试

package llm

import (
	"context"
	"strings"
	"testing"
)

func TestMockProviderChat(t *testing.T) {
	m := &MockProvider{}
	ctx := context.Background()

	resp, err := m.Chat(ctx, ChatRequest{
		Model: "mock",
		Messages: []Message{
			{Role: RoleSystem, Content: "你是诸葛亮"},
			{Role: RoleUser, Content: "天下大势如何？"},
		},
	})
	if err != nil {
		t.Fatalf("Chat 失败: %v", err)
	}
	if !strings.Contains(resp.Content, "天下大势如何") {
		t.Errorf("回复应包含用户问题，实际: %s", resp.Content)
	}
	if m.Calls() != 1 {
		t.Errorf("期望调用1次，实际 %d", m.Calls())
	}
}

func TestMockProviderEmptyReply(t *testing.T) {
	m := &MockProvider{Reply: "固定回复"}
	resp, err := m.Chat(context.Background(), ChatRequest{})
	if err != nil {
		t.Fatalf("Chat 失败: %v", err)
	}
	if resp.Content != "固定回复" {
		t.Errorf("期望固定回复，实际: %s", resp.Content)
	}
}

func TestOpenAIProviderConfig(t *testing.T) {
	p := NewOpenAIProvider("sk-test", "https://api.deepseek.com/v1", "deepseek-chat")
	if p.Name() != "openai-compatible" {
		t.Errorf("默认名称错误: %s", p.Name())
	}
	if p.Model() != "deepseek-chat" {
		t.Errorf("模型错误: %s", p.Model())
	}

	// 默认值
	p2 := NewOpenAIProvider("sk-test", "", "")
	if p2.Model() != "gpt-4o-mini" {
		t.Errorf("默认模型错误: %s", p2.Model())
	}
	if !strings.HasSuffix(p2.baseURL, "/v1") {
		t.Errorf("默认 BaseURL 错误: %s", p2.baseURL)
	}
}
