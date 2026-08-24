// 多轮对话历史 - 简单内存实现
// 过目不忘，温故知新
//
// 设计约束（见 docs/architecture 合理限制文档）：
//   - 仅内存、不落盘、不引入任何依赖（不做向量检索/持久化）；
//   - 多轮场景由调用方持有 *History 并在每轮传入 order.Context["history"]，
//     messages 数组原样透传给 Provider（不截断、不重排）。

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
