// 军师 Agent - LLM 对话编排核心
// 持有 LLM 提供者与多轮历史，统一处理工具预检、RAG 检索、消息组装与错误映射。
//
// 职责边界：
//   - 领域编排（Ask/历史/工具/RAG），不持 logger、不做展示；
//   - 展示层（REPL/JSON 输出）由 cmd/kongming 负责。

package agent

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"

	"github.com/zhuge/kongming/pkg/knowledge"
	"github.com/zhuge/kongming/pkg/llm"
	"github.com/zhuge/kongming/pkg/tools"
)

// ErrNoProvider 表示未配置 LLM Provider（含引导文案，美化交给 CLI）
var ErrNoProvider = errors.New("未配置 LLM Provider（请设置 KONGMING_API_KEY 环境变量，或使用 --mock 离线演示）")

// DefaultSystemPrompt 默认人设（由 CLI 传入；Agent 不硬编码人设）
const DefaultSystemPrompt = `你是诸葛亮（字孔明），蜀汉丞相，智慧的化身。
你以《隆中对》闻名：审时度势、分析天下大势、提出可行战略。
请以简洁、务实、略带文言风骨的中文回答主公的咨询。
分析要点：1) 局势判断 2) 关键矛盾 3) 行动建议。`

// Reply 单轮对话的结构化结果
type Reply struct {
	Answer    string   // 军师回复正文
	Model     string   // 实际模型名
	Provider  string   // 提供者名
	ToolUsed  string   // 命中的工具名（如 "calc"），无则空
	Knowledge []string // RAG 检索命中的段落标题
	Turns     int      // 本轮发给 LLM 的总消息条数（含人设/知识/历史）
}

// Options Agent 构造参数
type Options struct {
	Provider     llm.Provider     // 必填；nil 时 Ask 返回 ErrNoProvider
	SystemPrompt string           // 人设；空则不发送 system 消息
	Knowledge    *knowledge.Store // 可选，启用轻量 RAG
	Tools        *tools.Registry  // 可选，启用工具拦截
	MaxHistory   int              // 0=不限；>0 按"轮"截断（保留最近 N 轮）
}

// Agent 军师：持有 LLM 提供者与多轮对话历史
type Agent struct {
	provider     llm.Provider
	systemPrompt string
	knowledge    *knowledge.Store
	tools        *tools.Registry
	maxHistory   int

	mu      sync.Mutex
	history []llm.Message
}

// New 创建 Agent
func New(opts Options) *Agent {
	return &Agent{
		provider:     opts.Provider,
		systemPrompt: opts.SystemPrompt,
		knowledge:    opts.Knowledge,
		tools:        opts.Tools,
		maxHistory:   opts.MaxHistory,
		history:      make([]llm.Message, 0, 8),
	}
}

// Ask 向军师问计：工具预检 → RAG 检索 → 组装消息 → 调 LLM → 记历史。
// 工具命中且成功时直接短路，不调 LLM。
func (a *Agent) Ask(ctx context.Context, question string) (*Reply, error) {
	if a.provider == nil {
		return nil, ErrNoProvider
	}
	question = strings.TrimSpace(question)
	if question == "" {
		return nil, fmt.Errorf("问题不能为空")
	}

	// 1. 工具预检（first-match-wins）
	if a.tools != nil {
		handled, toolName, output, err := a.tools.Try(ctx, question)
		if handled && err == nil {
			a.record(question, output)
			return &Reply{Answer: output, ToolUsed: toolName}, nil
		}
		if handled && err != nil {
			// 命中但求值失败：把错误拼进问题，回落 LLM 解释
			question = fmt.Sprintf("%s（工具 %s 求值失败：%v）", question, toolName, err)
		}
	}

	// 2. 轻量 RAG：检索相关段落拼入上下文
	knowledgeTitles := make([]string, 0, 3)
	knowledgeCtx := ""
	if a.knowledge != nil {
		paras := a.knowledge.Search(question, 3)
		if len(paras) > 0 {
			knowledgeCtx = formatKnowledge(paras)
			for _, p := range paras {
				if p.Title != "" {
					knowledgeTitles = append(knowledgeTitles, p.Title)
				}
			}
		}
	}

	// 3. 组装消息：人设 + 知识 + 历史 + 当前问题
	messages := make([]llm.Message, 0, 8)
	if a.systemPrompt != "" {
		messages = append(messages, llm.Message{Role: llm.RoleSystem, Content: a.systemPrompt})
	}
	if knowledgeCtx != "" {
		messages = append(messages, llm.Message{Role: llm.RoleSystem, Content: knowledgeCtx})
	}
	hist := a.snapshotHistory()
	if a.maxHistory > 0 {
		hist = truncateHistory(hist, a.maxHistory)
	}
	messages = append(messages, hist...)
	messages = append(messages, llm.Message{Role: llm.RoleUser, Content: question})

	// 4. 调 LLM
	resp, err := a.provider.Chat(ctx, llm.ChatRequest{
		Messages:    messages,
		Temperature: 0.7,
		MaxTokens:   1024,
	})
	if err != nil {
		return nil, fmt.Errorf("调用 LLM 失败: %w", err)
	}

	// 5. 记历史
	a.record(question, resp.Content)

	return &Reply{
		Answer:    resp.Content,
		Model:     resp.Model,
		Provider:  a.provider.Name(),
		Knowledge: knowledgeTitles,
		Turns:     len(messages),
	}, nil
}

// History 返回历史消息的深拷贝（供 session 持久化，调用方安全修改）
func (a *Agent) History() []llm.Message {
	a.mu.Lock()
	defer a.mu.Unlock()
	return cloneMessages(a.history)
}

// KnowledgeDir 知识库加载目录（会话保存/恢复用）；未启用返回 ""
func (a *Agent) KnowledgeDir() string {
	if a.knowledge == nil {
		return ""
	}
	return a.knowledge.Dir()
}

// LoadHistory 载入历史（会话恢复用）；超限会话按 MaxHistory 截断
func (a *Agent) LoadHistory(messages []llm.Message) {
	a.mu.Lock()
	defer a.mu.Unlock()
	msgs := cloneMessages(messages)
	if a.maxHistory > 0 {
		msgs = truncateHistory(msgs, a.maxHistory)
	}
	a.history = msgs
}

// Reset 清空历史（非交互模式每轮调用，避免上下文串扰）
func (a *Agent) Reset() {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.history = a.history[:0]
}

// record 追加一轮问答到历史（内部持锁），超限时截断
func (a *Agent) record(userQ, assistantA string) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.history = append(a.history,
		llm.Message{Role: llm.RoleUser, Content: userQ},
		llm.Message{Role: llm.RoleAssistant, Content: assistantA},
	)
	if a.maxHistory > 0 {
		a.history = truncateHistory(a.history, a.maxHistory)
	}
}

// snapshotHistory 返回历史深拷贝（供消息组装用，避免组装期间被篡改）
func (a *Agent) snapshotHistory() []llm.Message {
	a.mu.Lock()
	defer a.mu.Unlock()
	return cloneMessages(a.history)
}

// cloneMessages 深拷贝消息切片
func cloneMessages(msgs []llm.Message) []llm.Message {
	out := make([]llm.Message, len(msgs))
	copy(out, msgs)
	return out
}

// truncateHistory 按"轮"截断：保留最近 maxRounds 轮（2*maxRounds 条消息），
// 截断后若开头是 assistant 再丢一条，保证以 user 开头（OpenAI 约定历史不以 assistant 起头）。
func truncateHistory(msgs []llm.Message, maxRounds int) []llm.Message {
	if maxRounds <= 0 || len(msgs) <= 2*maxRounds {
		return msgs
	}
	msgs = msgs[len(msgs)-2*maxRounds:]
	if len(msgs) > 0 && msgs[0].Role == llm.RoleAssistant {
		msgs = msgs[1:]
	}
	return msgs
}

// formatKnowledge 把检索到的段落拼成参考上下文（标题 + 正文）
func formatKnowledge(paras []knowledge.Paragraph) string {
	var sb strings.Builder
	sb.WriteString("以下是主公的军师知识库中与本问相关的记载，可参考但不必逐字引用：\n")
	for i, p := range paras {
		if i > 0 {
			sb.WriteString("\n")
		}
		if p.Title != "" {
			sb.WriteString("【" + p.Title + "】\n")
		}
		sb.WriteString(p.Content)
	}
	return sb.String()
}
