// 多轮对话历史测试

package llm

import (
	"testing"
)

func TestHistoryAppendAndRead(t *testing.T) {
	h := NewHistory()
	h.AddUser("主公问一")
	h.AddAssistant("亮答一")
	h.AddUser("主公问二")

	if h.Len() != 3 {
		t.Errorf("期望 3 条消息，实际 %d", h.Len())
	}

	msgs := h.Messages()
	if len(msgs) != 3 {
		t.Fatalf("期望 3 条消息副本，实际 %d", len(msgs))
	}
	if msgs[0].Role != RoleUser || msgs[0].Content != "主公问一" {
		t.Errorf("第 1 条应为 user 问一，实际 %+v", msgs[0])
	}
	if msgs[1].Role != RoleAssistant || msgs[1].Content != "亮答一" {
		t.Errorf("第 2 条应为 assistant 答一，实际 %+v", msgs[1])
	}
}

func TestHistoryMessagesIsCopy(t *testing.T) {
	h := NewHistory()
	h.AddUser("原始内容")

	msgs := h.Messages()
	msgs[0].Content = "被调用方篡改"

	got := h.Messages()
	if got[0].Content != "原始内容" {
		t.Errorf("Messages 应返回副本，历史被污染: %s", got[0].Content)
	}
}

func TestHistoryReset(t *testing.T) {
	h := NewHistory()
	h.AddUser("a")
	h.AddAssistant("b")
	h.Reset()
	if h.Len() != 0 {
		t.Errorf("Reset 后应为 0 条，实际 %d", h.Len())
	}
	// Reset 后可继续追加
	h.AddUser("c")
	if h.Len() != 1 {
		t.Errorf("Reset 后追加失败，实际 %d", h.Len())
	}
}

func TestHistoryConcurrentSafe(t *testing.T) {
	h := NewHistory()
	done := make(chan struct{})
	go func() {
		defer close(done)
		for i := 0; i < 100; i++ {
			h.AddUser("并发写")
		}
	}()
	for i := 0; i < 100; i++ {
		_ = h.Messages()
		h.Len()
	}
	<-done
	if h.Len() != 100 {
		t.Errorf("期望 100 条，实际 %d", h.Len())
	}
}

func TestHistoryFromMessages(t *testing.T) {
	src := []Message{
		{Role: RoleUser, Content: "问一"},
		{Role: RoleAssistant, Content: "答一"},
	}
	h := NewHistoryFromMessages(src)
	if h.Len() != 2 {
		t.Fatalf("期望 2 条消息，实际 %d", h.Len())
	}
	msgs := h.Messages()
	if msgs[0].Content != "问一" || msgs[1].Role != RoleAssistant {
		t.Errorf("消息还原错误: %+v", msgs)
	}
}
