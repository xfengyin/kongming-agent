// 军师 Agent 核心测试
// 覆盖：单轮/多轮消息组装、知识注入、工具拦截、错误映射、历史截断、并发安全、ctx 取消。

package agent

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/xfengyin/kongming-agent/pkg/knowledge"
	"github.com/xfengyin/kongming-agent/pkg/llm"
	"github.com/xfengyin/kongming-agent/pkg/tools"
)

// recordingProvider 记录每次 Chat 收到的完整消息（断言多轮历史透传用）
type recordingProvider struct {
	llm.Provider
	lastMessages []llm.Message
}

func (r *recordingProvider) Name() string { return "recording" }

func (r *recordingProvider) Chat(ctx context.Context, req llm.ChatRequest) (*llm.ChatResponse, error) {
	r.lastMessages = req.Messages
	return &llm.ChatResponse{Content: "亮已记下", Model: "recording-model"}, nil
}

// ctxAwareProvider 尊重 ctx 取消的假 Provider（断言取消传播）
type ctxAwareProvider struct{}

func (p *ctxAwareProvider) Name() string { return "ctx-aware" }

func (p *ctxAwareProvider) Chat(ctx context.Context, req llm.ChatRequest) (*llm.ChatResponse, error) {
	<-ctx.Done()
	return nil, ctx.Err()
}

func TestAgentSingleTurn(t *testing.T) {
	a := New(Options{Provider: &llm.MockProvider{}, SystemPrompt: DefaultSystemPrompt})
	r, err := a.Ask(context.Background(), "天下大势如何？")
	if err != nil {
		t.Fatalf("Ask 失败: %v", err)
	}
	if !strings.Contains(r.Answer, "天下大势如何") {
		t.Errorf("回复应包含问题，实际: %q", r.Answer)
	}
	if r.Turns != 2 {
		t.Errorf("单轮应 [system, user] 共 2 条，实际 %d", r.Turns)
	}
	if r.Provider != "mock" {
		t.Errorf("Provider 名应为 mock，实际 %s", r.Provider)
	}
	// 历史已记录
	if len(a.History()) != 2 {
		t.Errorf("Ask 后历史应有 2 条，实际 %d", len(a.History()))
	}
}

func TestAgentNoSystemPromptWhenEmpty(t *testing.T) {
	rec := &recordingProvider{}
	a := New(Options{Provider: rec}) // SystemPrompt 为空
	_, err := a.Ask(context.Background(), "你好")
	if err != nil {
		t.Fatalf("Ask 失败: %v", err)
	}
	if len(rec.lastMessages) != 1 {
		t.Fatalf("空人设应只有 [user] 1 条，实际 %d", len(rec.lastMessages))
	}
	if rec.lastMessages[0].Role != llm.RoleUser {
		t.Errorf("第 1 条应为主公问题，实际 %+v", rec.lastMessages[0])
	}
}

func TestAgentMultiTurnHistory(t *testing.T) {
	ctx := context.Background()
	rec := &recordingProvider{}
	a := New(Options{Provider: rec, SystemPrompt: DefaultSystemPrompt})

	// 第一轮：无历史
	r1, err := a.Ask(ctx, "第一问：天下大势？")
	if err != nil {
		t.Fatalf("第一轮执行失败: %v", err)
	}
	if len(rec.lastMessages) != 2 {
		t.Fatalf("第一轮应只有 [system, user]，实际 %d 条", len(rec.lastMessages))
	}
	if rec.lastMessages[0].Role != llm.RoleSystem || rec.lastMessages[1].Role != llm.RoleUser {
		t.Errorf("第一轮消息角色错误: %+v", rec.lastMessages)
	}

	// 第二轮：应透传第一轮完整历史
	r2, err := a.Ask(ctx, "第二问：那我军当如何？")
	if err != nil {
		t.Fatalf("第二轮执行失败: %v", err)
	}
	// system + 历史(user,assistant) + 当前user = 4 条
	if len(rec.lastMessages) != 4 {
		t.Fatalf("第二轮应透传历史共 4 条，实际 %d 条", len(rec.lastMessages))
	}
	if rec.lastMessages[1].Role != llm.RoleUser || rec.lastMessages[1].Content != "第一问：天下大势？" {
		t.Errorf("第二轮应包含第一轮 user 消息，实际 %+v", rec.lastMessages[1])
	}
	if rec.lastMessages[2].Role != llm.RoleAssistant || rec.lastMessages[2].Content != "亮已记下" {
		t.Errorf("第二轮应包含第一轮 assistant 消息，实际 %+v", rec.lastMessages[2])
	}
	if rec.lastMessages[3].Content != "第二问：那我军当如何？" {
		t.Errorf("第二轮最后一条应为当前问题，实际 %+v", rec.lastMessages[3])
	}
	if r2.Turns != 4 {
		t.Errorf("第二轮 turns 应为 4，实际 %d", r2.Turns)
	}
	if r1.Turns != 2 {
		t.Errorf("第一轮 turns 应为 2，实际 %d", r1.Turns)
	}
}

func TestAgentWithKnowledge(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	md := filepath.Join(dir, "sanguo.md")
	content := "# 空城计\n\n司马懿大军压境，诸葛亮大开城门，焚香抚琴。\n\n# 草船借箭\n\n大雾漫天，借箭十万。"
	if err := os.WriteFile(md, []byte(content), 0o644); err != nil {
		t.Fatalf("写知识文件失败: %v", err)
	}
	kb, err := knowledge.Load(dir)
	if err != nil {
		t.Fatalf("加载知识库失败: %v", err)
	}

	rec := &recordingProvider{}
	a := New(Options{Provider: rec, SystemPrompt: DefaultSystemPrompt, Knowledge: kb})
	r, err := a.Ask(ctx, "如何用空城计退敌？")
	if err != nil {
		t.Fatalf("Ask 失败: %v", err)
	}
	// system(人设) + system(知识) + user = 3 条
	if len(rec.lastMessages) != 3 {
		t.Fatalf("期望 3 条消息（人设+知识+问题），实际 %d", len(rec.lastMessages))
	}
	if rec.lastMessages[1].Role != llm.RoleSystem || !strings.Contains(rec.lastMessages[1].Content, "空城计") {
		t.Errorf("第 2 条应为知识库上下文，实际 %+v", rec.lastMessages[1])
	}
	if rec.lastMessages[2].Role != llm.RoleUser {
		t.Errorf("第 3 条应为主公问题，实际 %+v", rec.lastMessages[2])
	}
	if r.Turns != 3 {
		t.Errorf("turns 应为 3，实际 %d", r.Turns)
	}
	if len(r.Knowledge) == 0 {
		t.Errorf("Reply 应携带命中的知识标题")
	}
	if a.KnowledgeDir() != dir {
		t.Errorf("KnowledgeDir 应为 %s，实际 %s", dir, a.KnowledgeDir())
	}
}

func TestAgentWithoutKnowledgeDir(t *testing.T) {
	a := New(Options{Provider: &llm.MockProvider{}})
	if a.KnowledgeDir() != "" {
		t.Errorf("未启用知识库时 KnowledgeDir 应为空，实际 %q", a.KnowledgeDir())
	}
}

func TestAgentToolIntercept(t *testing.T) {
	rec := &recordingProvider{}
	a := New(Options{Provider: rec, Tools: tools.NewRegistry(tools.NewCalculator())})

	r, err := a.Ask(context.Background(), "计算 123*456")
	if err != nil {
		t.Fatalf("Ask 失败: %v", err)
	}
	if r.ToolUsed != "calc" {
		t.Errorf("应命中 calc 工具，实际 %q", r.ToolUsed)
	}
	if !strings.Contains(r.Answer, "56088") {
		t.Errorf("工具结果应包含 56088，实际 %q", r.Answer)
	}
	// 短路：不应调用 LLM
	if rec.lastMessages != nil {
		t.Errorf("工具命中时不应调用 LLM，实际收到消息 %+v", rec.lastMessages)
	}
	// 历史已记录本轮问答
	hist := a.History()
	if len(hist) != 2 {
		t.Errorf("工具命中后历史应有 2 条，实际 %d", len(hist))
	}
	if hist[0].Content != "计算 123*456" || hist[1].Content != r.Answer {
		t.Errorf("历史应记录本轮问答，实际 %+v", hist)
	}
}

func TestAgentToolInterceptErrorFallback(t *testing.T) {
	rec := &recordingProvider{}
	a := New(Options{Provider: rec, Tools: tools.NewRegistry(tools.NewCalculator())})

	// 命中但求值失败（除零）→ 回落 LLM，user 消息含错误
	r, err := a.Ask(context.Background(), "计算 1/0")
	if err != nil {
		t.Fatalf("Ask 失败: %v", err)
	}
	if rec.lastMessages == nil {
		t.Fatalf("求值失败应回落 LLM，但 LLM 未被调用")
	}
	last := rec.lastMessages[len(rec.lastMessages)-1]
	if !strings.Contains(last.Content, "求值失败") || !strings.Contains(last.Content, "除数为零") {
		t.Errorf("回落消息应包含工具错误，实际: %q", last.Content)
	}
	if r.ToolUsed != "" {
		t.Errorf("回落 LLM 时 ToolUsed 应为空，实际 %q", r.ToolUsed)
	}
}

func TestAgentNoToolMatch(t *testing.T) {
	rec := &recordingProvider{}
	a := New(Options{Provider: rec, Tools: tools.NewRegistry(tools.NewCalculator())})

	r, err := a.Ask(context.Background(), "天下大势如何？")
	if err != nil {
		t.Fatalf("Ask 失败: %v", err)
	}
	if r.ToolUsed != "" {
		t.Errorf("不命中工具时 ToolUsed 应为空，实际 %q", r.ToolUsed)
	}
	if rec.lastMessages == nil {
		t.Errorf("不命中工具应走 LLM")
	}
}

func TestAgentNoProvider(t *testing.T) {
	a := New(Options{})
	_, err := a.Ask(context.Background(), "你好")
	if !errors.Is(err, ErrNoProvider) {
		t.Fatalf("应返回 ErrNoProvider，实际 %v", err)
	}
}

func TestAgentEmptyQuestion(t *testing.T) {
	rec := &recordingProvider{}
	a := New(Options{Provider: rec})
	_, err := a.Ask(context.Background(), "   ")
	if err == nil {
		t.Fatal("空问题应报错")
	}
}

func TestAgentMaxHistoryTruncation(t *testing.T) {
	rec := &recordingProvider{}
	a := New(Options{Provider: rec, SystemPrompt: DefaultSystemPrompt, MaxHistory: 1})

	// 连问三轮（每轮 2 条 → 6 条历史），MaxHistory=1 只保留最近 2 条
	for i := 0; i < 3; i++ {
		if _, err := a.Ask(context.Background(), "问数"); err != nil {
			t.Fatalf("Ask 失败: %v", err)
		}
	}
	hist := a.History()
	if len(hist) != 2 {
		t.Fatalf("MaxHistory=1 应保留最近 2 条，实际 %d", len(hist))
	}
	if hist[0].Role != llm.RoleUser {
		t.Errorf("截断后应以 user 开头，实际 %+v", hist[0])
	}
	// 第四轮组装消息应为 system + 截断历史(2) + user = 4 条
	if _, err := a.Ask(context.Background(), "问数"); err != nil {
		t.Fatalf("Ask 失败: %v", err)
	}
	if len(rec.lastMessages) != 4 {
		t.Errorf("第四轮应 system+历史2+user = 4 条，实际 %d", len(rec.lastMessages))
	}
}

func TestAgentLoadHistoryTruncation(t *testing.T) {
	rec := &recordingProvider{}
	a := New(Options{Provider: rec, SystemPrompt: DefaultSystemPrompt, MaxHistory: 1})

	// 加载超限历史（4 轮 8 条）→ 截断为最近 2 条
	long := make([]llm.Message, 0, 8)
	for i := 0; i < 8; i++ {
		role := llm.RoleUser
		if i%2 == 1 {
			role = llm.RoleAssistant
		}
		long = append(long, llm.Message{Role: role, Content: "m"})
	}
	a.LoadHistory(long)
	hist := a.History()
	if len(hist) != 2 {
		t.Fatalf("LoadHistory 超限应截断为 2 条，实际 %d", len(hist))
	}
	if hist[0].Role != llm.RoleUser {
		t.Errorf("截断后应以 user 开头，实际 %+v", hist[0])
	}
}

func TestAgentHistoryCopySemantics(t *testing.T) {
	rec := &recordingProvider{}
	a := New(Options{Provider: rec})
	if _, err := a.Ask(context.Background(), "原始内容"); err != nil {
		t.Fatalf("Ask 失败: %v", err)
	}

	msgs := a.History()
	msgs[0].Content = "被调用方篡改"

	got := a.History()
	if got[0].Content != "原始内容" {
		t.Errorf("History 应返回副本，历史被污染: %s", got[0].Content)
	}
}

func TestAgentReset(t *testing.T) {
	rec := &recordingProvider{}
	a := New(Options{Provider: rec})
	if _, err := a.Ask(context.Background(), "a"); err != nil {
		t.Fatalf("Ask 失败: %v", err)
	}
	a.Reset()
	if len(a.History()) != 0 {
		t.Errorf("Reset 后应为 0 条，实际 %d", len(a.History()))
	}
	// Reset 后可继续
	if _, err := a.Ask(context.Background(), "c"); err != nil {
		t.Fatalf("Ask 失败: %v", err)
	}
	if len(a.History()) != 2 {
		t.Errorf("Reset 后追加失败，实际 %d", len(a.History()))
	}
}

func TestAgentConcurrentSafe(t *testing.T) {
	rec := &recordingProvider{}
	a := New(Options{Provider: rec})

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < 50; i++ {
			if _, err := a.Ask(context.Background(), "并发写"); err != nil {
				t.Errorf("并发 Ask 失败: %v", err)
			}
		}
	}()
	for i := 0; i < 50; i++ {
		_ = a.History()
	}
	wg.Wait()
	if len(a.History()) != 100 {
		t.Errorf("期望 100 条历史（50 轮），实际 %d", len(a.History()))
	}
}

func TestAgentContextCancel(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	a := New(Options{Provider: &ctxAwareProvider{}})
	cancel()
	_, err := a.Ask(ctx, "你好")
	if err == nil {
		t.Fatal("ctx 取消后 Ask 应报错")
	}
	if !errors.Is(err, context.Canceled) {
		t.Errorf("应透出 context.Canceled，实际 %v", err)
	}
}
