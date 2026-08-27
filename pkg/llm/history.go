// 多轮对话历史 - 简单内存实现
// 过目不忘，温故知新
//
// 注意：本类型将在「删除 generals 死模块」时一并移除（C3）。
// 新架构的对话历史由 pkg/agent 内部持有（[]llm.Message + sync.Mutex），
// 此文件仅为等待删除的 pkg/generals 提供过渡实现。

package llm

import "sync"

// History 简单的线程安全对话历史（内存实现）
type History struct {
	mu       sync.Mutex
	messages []Message
}

// NewHistory 创建空历史
func NewHistory() *History {
	return &History{messages: make([]Message, 0, 8)}
}

// Add 追加一条消息
func (h *History) Add(role Role, content string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.messages = append(h.messages, Message{Role: role, Content: content})
}

// AddUser 追加主公（用户）消息
func (h *History) AddUser(content string) { h.Add(RoleUser, content) }

// AddAssistant 追加军师（模型）回复
func (h *History) AddAssistant(content string) { h.Add(RoleAssistant, content) }

// Len 消息条数
func (h *History) Len() int {
	h.mu.Lock()
	defer h.mu.Unlock()
	return len(h.messages)
}

// Messages 返回历史消息的副本（调用方安全修改/透传）
func (h *History) Messages() []Message {
	h.mu.Lock()
	defer h.mu.Unlock()
	out := make([]Message, len(h.messages))
	copy(out, h.messages)
	return out
}

// Reset 清空历史
func (h *History) Reset() {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.messages = h.messages[:0]
}

// NewHistoryFromMessages 用已有消息序列构造历史（会话加载用）
func NewHistoryFromMessages(messages []Message) *History {
	h := NewHistory()
	h.mu.Lock()
	defer h.mu.Unlock()
	h.messages = append(h.messages, messages...)
	return h
}
