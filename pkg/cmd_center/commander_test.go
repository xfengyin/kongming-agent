// 军师府调度闭环测试（含 LLM 诸葛亮点将）

package cmd_center

import (
	"context"
	"strings"
	"testing"

	"github.com/zhuge/kongming/pkg/generals"
	"github.com/zhuge/kongming/pkg/llm"
	"go.uber.org/zap"
)

func TestDispatchWithNamedGenerals(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	pool := generals.NewWuHuPoolWithLLM(&llm.MockProvider{})
	commander := NewCommanderWithPool(logger, pool)
	ctx := context.Background()

	order := NewMilitaryOrder("隆中对", "请分析当前市场局势并给出建议", PriorityNormal)
	order.Strategy.Generals = []string{"kongming"}

	report, err := commander.Dispatch(ctx, order)
	if err != nil {
		t.Fatalf("Dispatch 失败: %v", err)
	}
	if !report.Success {
		t.Errorf("期望成功，实际失败")
	}
	if len(report.Generals) != 1 {
		t.Fatalf("期望1份将领战报，实际 %d", len(report.Generals))
	}
	gr := report.Generals[0]
	if gr.GeneralID != "kongming" {
		t.Errorf("期望点将诸葛亮，实际 %s", gr.GeneralID)
	}
	if !gr.Success {
		t.Errorf("诸葛亮执行应成功: %s", gr.Message)
	}
	answer, ok := gr.Data["answer"].(string)
	if !ok || !strings.Contains(answer, "此乃天机") {
		t.Errorf("战报应含 LLM 回复，实际: %v", gr.Data["answer"])
	}
}

func TestDispatchAutoSelectGeneral(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	commander := NewCommander(logger)
	ctx := context.Background()

	order := NewMilitaryOrder("市场调研", "调研智能硬件市场", PriorityNormal)
	order.Strategy.Objectives = []string{"收集竞品信息", "输出调研报告"}

	report, err := commander.Dispatch(ctx, order)
	if err != nil {
		t.Fatalf("Dispatch 失败: %v", err)
	}
	if len(report.Generals) == 0 {
		t.Errorf("自动选将不应为空")
	}
	// 五虎将 handler 均为 mock，应全部成功
	for _, gr := range report.Generals {
		if !gr.Success {
			t.Errorf("将领 %s 执行失败: %s", gr.GeneralName, gr.Message)
		}
	}
}

func TestPlanStrategyBaguaMode(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	commander := NewCommander(logger)
	ctx := context.Background()

	order := NewMilitaryOrder("紧急军情", "速报", PriorityUrgent)
	strategy, err := commander.PlanStrategy(ctx, order)
	if err != nil {
		t.Fatalf("PlanStrategy 失败: %v", err)
	}
	if strategy.BaguaMode != "fengyang" {
		t.Errorf("紧急任务应选风扬阵，实际 %s", strategy.BaguaMode)
	}
}
