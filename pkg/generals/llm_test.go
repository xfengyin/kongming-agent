// 诸葛亮 LLM 将领测试

package generals

import (
	"context"
	"strings"
	"testing"

	"github.com/zhuge/kongming/pkg/core"
	"github.com/zhuge/kongming/pkg/llm"
)

func TestWuHuPoolWithLLMCount(t *testing.T) {
	pool := NewWuHuPoolWithLLM(&llm.MockProvider{})
	if pool.Count() != 6 {
		t.Errorf("期望6位将领（五虎将+诸葛亮），实际有 %d 位", pool.Count())
	}

	g, err := pool.Get("kongming")
	if err != nil {
		t.Fatalf("获取诸葛亮失败: %v", err)
	}
	if g.Type != GeneralKongMing {
		t.Errorf("期望类型 kongming，实际 %s", g.Type)
	}
}

func TestKongMingHandlerWithLLM(t *testing.T) {
	pool := NewWuHuPoolWithLLM(&llm.MockProvider{})
	ctx := context.Background()

	order := core.NewMilitaryOrder("问计", "天下大势如何？", core.PriorityNormal)
	report, err := pool.Execute(ctx, "kongming", order)
	if err != nil {
		t.Fatalf("执行失败: %v", err)
	}
	if !report.Success {
		t.Errorf("期望成功，实际失败: %s", report.Message)
	}
	answer, ok := report.Data["answer"].(string)
	if !ok || !strings.Contains(answer, "天下大势如何") {
		t.Errorf("战报应包含 LLM 回复，实际: %v", report.Data["answer"])
	}
}

func TestKongMingHandlerWithoutProvider(t *testing.T) {
	pool := NewWuHuPoolWithLLM(nil)
	ctx := context.Background()

	order := core.NewMilitaryOrder("问计", "天下大势如何？", core.PriorityNormal)
	report, err := pool.Execute(ctx, "kongming", order)
	if err != nil {
		t.Fatalf("执行失败: %v", err)
	}
	if report.Success {
		t.Errorf("无 Provider 时应返回引导配置的失败战报")
	}
	if !strings.Contains(report.Message, "KONGMING_API_KEY") {
		t.Errorf("失败信息应引导配置 Key，实际: %s", report.Message)
	}
}

func TestKongMingSystemPrompt(t *testing.T) {
	if !strings.Contains(KongMingSystemPrompt, "诸葛亮") {
		t.Errorf("人设应包含诸葛亮")
	}
}

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

func TestKongMingMultiTurnHistory(t *testing.T) {
	ctx := context.Background()
	rec := &recordingProvider{}
	pool := NewWuHuPoolWithLLM(rec)
	history := llm.NewHistory()

	// 第一轮：无历史
	order1 := core.NewMilitaryOrder("问计一", "第一问：天下大势？", core.PriorityNormal)
	order1.Context["history"] = history
	report1, err := pool.Execute(ctx, "kongming", order1)
	if err != nil {
		t.Fatalf("第一轮执行失败: %v", err)
	}
	if len(rec.lastMessages) != 2 {
		t.Fatalf("第一轮应只有 [system, user]，实际 %d 条", len(rec.lastMessages))
	}
	if rec.lastMessages[0].Role != llm.RoleSystem || rec.lastMessages[1].Role != llm.RoleUser {
		t.Errorf("第一轮消息角色错误: %+v", rec.lastMessages)
	}
	history.AddUser("第一问：天下大势？")
	history.AddAssistant(report1.Data["answer"].(string))

	// 第二轮：应透传第一轮完整历史
	order2 := core.NewMilitaryOrder("问计二", "第二问：那我军当如何？", core.PriorityNormal)
	order2.Context["history"] = history
	report2, err := pool.Execute(ctx, "kongming", order2)
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
	if !report2.Success {
		t.Errorf("第二轮应成功: %s", report2.Message)
	}
	if turns, ok := report2.Data["turns"].(int); !ok || turns != 4 {
		t.Errorf("战报 turns 应为 4，实际 %v", report2.Data["turns"])
	}
}

func TestKongMingWithKnowledgeContext(t *testing.T) {
	ctx := context.Background()
	rec := &recordingProvider{}
	pool := NewWuHuPoolWithLLM(rec)

	order := core.NewMilitaryOrder("问计", "如何用空城计退敌？", core.PriorityNormal)
	// 模拟 examples/longzhong --knowledge 注入的知识上下文
	order.Context["knowledge"] = "【空城计】司马懿大军压境，诸葛亮大开城门，焚香抚琴。司马懿疑有伏兵，退兵而去。"
	report, err := pool.Execute(ctx, "kongming", order)
	if err != nil {
		t.Fatalf("执行失败: %v", err)
	}
	if !report.Success {
		t.Fatalf("执行应成功: %s", report.Message)
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
	if turns, ok := report.Data["turns"].(int); !ok || turns != 3 {
		t.Errorf("turns 应为 3，实际 %v", report.Data["turns"])
	}
}
