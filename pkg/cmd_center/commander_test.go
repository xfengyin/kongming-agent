// 军师府测试
// 验证 Commander 通过 ExpertExecutor 端口调度专家执行的完整链路
// 对齐 kimi-k3 依赖倒置：Commander 只依赖抽象端口，不依赖具体专家实现

package cmd_center

import (
	"context"
	"fmt"
	"testing"

	"go.uber.org/zap"
)

// mockExpertExecutor 模拟专家执行器
// 验证 Commander 通过 ExpertExecutor 端口调用的契约
type mockExpertExecutor struct {
	executedSkills []string
	executedOrders []*MilitaryOrder
	failOnSkill    string // 模拟指定技能执行失败
}

func (m *mockExpertExecutor) ExecuteBySkill(ctx context.Context, skill string, order *MilitaryOrder) (*GeneralReport, error) {
	m.executedSkills = append(m.executedSkills, skill)
	m.executedOrders = append(m.executedOrders, order)
	if skill == m.failOnSkill {
		return nil, fmt.Errorf("模拟执行失败: %s", skill)
	}
	return &GeneralReport{
		GeneralID:   fmt.Sprintf("expert_%s", skill),
		GeneralName: skill,
		Success:     true,
		Message:     fmt.Sprintf("技能 %s 执行成功", skill),
	}, nil
}

func (m *mockExpertExecutor) ExecuteBySkillTopK(ctx context.Context, skill string, topK int, order *MilitaryOrder) (*BattleReport, error) {
	gr, err := m.ExecuteBySkill(ctx, skill, order)
	if err != nil {
		return &BattleReport{OrderID: order.ID, Success: false, Message: err.Error()}, nil
	}
	return &BattleReport{
		OrderID:     order.ID,
		Success:     true,
		StartedAt:   order.CreatedAt,
		CompletedAt: order.CreatedAt,
		Generals:    []GeneralReport{*gr},
	}, nil
}

// TestCommanderDispatchViaExecutorPort 验证 Commander 通过 ExpertExecutor 端口调度
func TestCommanderDispatchViaExecutorPort(t *testing.T) {
	logger := zap.NewNop()
	executor := &mockExpertExecutor{}
	commander := NewCommander(logger, executor)

	order := NewMilitaryOrder("测试任务", "验证 MoE 路由调度", PriorityNormal)
	order.Strategy.Objectives = []string{
		"收集数据",
		"处理数据",
	}

	report, err := commander.Dispatch(context.Background(), order)
	if err != nil {
		t.Fatalf("Dispatch 失败: %v", err)
	}
	if !report.Success {
		t.Errorf("任务应成功，消息: %s", report.Message)
	}
	// 验证 ExpertExecutor 被调用了 2 次（每个目标一次）
	if len(executor.executedSkills) != 2 {
		t.Errorf("期望 2 次技能执行，实际 %d", len(executor.executedSkills))
	}
	// 验证技能路由正确（"收集"->data_collection, "处理"->data_processing）
	expectedSkills := map[string]bool{"data_collection": false, "data_processing": false}
	for _, s := range executor.executedSkills {
		if _, ok := expectedSkills[s]; ok {
			expectedSkills[s] = true
		}
	}
	for skill, found := range expectedSkills {
		if !found {
			t.Errorf("技能 %s 未被路由激活", skill)
		}
	}
}

// TestCommanderDispatchWithFailure 验证单战术失败不影响整体流程
func TestCommanderDispatchWithFailure(t *testing.T) {
	logger := zap.NewNop()
	executor := &mockExpertExecutor{failOnSkill: "data_processing"}
	commander := NewCommander(logger, executor)

	order := NewMilitaryOrder("容错测试", "验证部分失败处理", PriorityNormal)
	order.Strategy.Objectives = []string{
		"收集数据",
		"处理数据", // 此技能模拟失败
	}

	report, err := commander.Dispatch(context.Background(), order)
	if err != nil {
		t.Fatalf("Dispatch 不应返回错误: %v", err)
	}
	// 整体应失败（有战术失败）
	if report.Success {
		t.Errorf("存在失败战术时整体应失败")
	}
	// 应有 2 个战报（1 成功 + 1 失败）
	if len(report.Generals) != 2 {
		t.Errorf("期望 2 个战报，实际 %d", len(report.Generals))
	}
}

// TestCommanderPlanStrategy 验证战略制定与八卦阵模式选择
func TestCommanderPlanStrategy(t *testing.T) {
	logger := zap.NewNop()
	executor := &mockExpertExecutor{}
	commander := NewCommander(logger, executor)

	// 紧急军情应选风扬阵（快速响应）
	urgentOrder := NewMilitaryOrder("紧急", "紧急任务", PriorityUrgent)
	urgentOrder.Strategy.Objectives = []string{"收集情报"}
	strategy, err := commander.PlanStrategy(context.Background(), urgentOrder)
	if err != nil {
		t.Fatalf("PlanStrategy 失败: %v", err)
	}
	if strategy.BaguaMode != "fengyang" {
		t.Errorf("紧急军情应选风扬阵，实际 %s", strategy.BaguaMode)
	}

	// 高优先级应选天覆阵（并行执行）
	highOrder := NewMilitaryOrder("高优", "高优先级任务", PriorityHigh)
	highOrder.Strategy.Objectives = []string{"收集情报"}
	highStrategy, _ := commander.PlanStrategy(context.Background(), highOrder)
	if highStrategy.BaguaMode != "tiangai" {
		t.Errorf("高优先级应选天覆阵，实际 %s", highStrategy.BaguaMode)
	}

	// 普通优先级应选地载阵（顺序执行）
	normalOrder := NewMilitaryOrder("普通", "普通任务", PriorityNormal)
	normalOrder.Strategy.Objectives = []string{"收集情报"}
	normalStrategy, _ := commander.PlanStrategy(context.Background(), normalOrder)
	if normalStrategy.BaguaMode != "dizai" {
		t.Errorf("普通优先级应选地载阵，实际 %s", normalStrategy.BaguaMode)
	}
}

// TestCommanderOrderManagement 验证军令查询与列表
func TestCommanderOrderManagement(t *testing.T) {
	logger := zap.NewNop()
	executor := &mockExpertExecutor{}
	commander := NewCommander(logger, executor)

	order := NewMilitaryOrder("查询测试", "验证军令管理", PriorityNormal)
	order.Strategy.Objectives = []string{"收集数据"}

	_, err := commander.Dispatch(context.Background(), order)
	if err != nil {
		t.Fatalf("Dispatch 失败: %v", err)
	}

	// 查询军令
	got, err := commander.GetOrder(order.ID)
	if err != nil {
		t.Errorf("查询军令失败: %v", err)
	}
	if got.Name != "查询测试" {
		t.Errorf("军令名称不符，实际 %s", got.Name)
	}
	if got.State != StateCompleted {
		t.Errorf("军令状态应为已完成，实际 %s", got.State)
	}

	// 列出已完成军令
	completed := commander.ListOrders(StateCompleted)
	if len(completed) != 1 {
		t.Errorf("期望1个已完成军令，实际 %d", len(completed))
	}
}

// TestCommanderImplementsPort 验证 Commander 实现 CommanderPort 端口
func TestCommanderImplementsPort(t *testing.T) {
	logger := zap.NewNop()
	executor := &mockExpertExecutor{}
	var port CommanderPort = NewCommander(logger, executor)
	if port == nil {
		t.Error("Commander 应实现 CommanderPort 端口")
	}
}
