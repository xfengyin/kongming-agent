// OpenAI 兼容适配层 HTTP 契约测试（httptest，无外网）

package llm

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// TestOpenAIProviderChatRequest 校验请求结构与鉴权头
func TestOpenAIProviderChatRequest(t *testing.T) {
	var gotPath string
	var gotAuth string
	var gotBody map[string]interface{}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotAuth = r.Header.Get("Authorization")
		b, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(b, &gotBody)
		w.Header().Set("Content-Type", "application/json")
		io.WriteString(w, `{"choices":[{"message":{"role":"assistant","content":"此乃天机"}}],"model":"deepseek-chat"}`)
	}))
	defer srv.Close()

	p := NewOpenAIProvider("sk-abc", srv.URL, "deepseek-chat")
	resp, err := p.Chat(context.Background(), ChatRequest{
		Messages: []Message{{Role: RoleUser, Content: "天下大势如何？"}},
	})
	if err != nil {
		t.Fatalf("Chat 失败: %v", err)
	}

	if gotPath != "/chat/completions" {
		t.Errorf("期望路径 /chat/completions，实际 %s", gotPath)
	}
	if gotAuth != "Bearer sk-abc" {
		t.Errorf("期望鉴权头 Bearer sk-abc，实际 %q", gotAuth)
	}
	if gotBody["model"] != "deepseek-chat" {
		t.Errorf("期望模型 deepseek-chat，实际 %v", gotBody["model"])
	}
	if resp.Content != "此乃天机" {
		t.Errorf("期望内容 此乃天机，实际 %q", resp.Content)
	}
	if resp.Model != "deepseek-chat" {
		t.Errorf("期望返回模型 deepseek-chat，实际 %q", resp.Model)
	}
}

// TestOpenAIProviderError 校验非 200 响应透出
func TestOpenAIProviderError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		io.WriteString(w, `{"error":{"message":"Invalid API key"}}`)
	}))
	defer srv.Close()

	p := NewOpenAIProvider("sk-bad", srv.URL, "gpt-4o-mini")
	_, err := p.Chat(context.Background(), ChatRequest{Messages: []Message{{Role: RoleUser, Content: "hi"}}})
	if err == nil {
		t.Fatal("期望错误，实际成功")
	}
	if !strings.Contains(err.Error(), "401") {
		t.Errorf("错误信息应包含状态码，实际: %v", err)
	}
}
