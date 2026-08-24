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
