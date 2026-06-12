// Package commander commander 应用层用例的单元测试。
//
// 本文件覆盖 Planner 组件（独立可测部分）。
// service.go 的端到端测试在 service_test.go 中（与 stage 3.3 port 一起 commit）。
package commander

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/zhuge/kongming/pkg/domain/model"
)

// TestDefaultPlanner_Priority 验证 DefaultPlanner 按 Priority 正确选择八卦阵：
//   - PriorityUrgent → Fengyang（风扬：快速响应）
//   - PriorityHigh   → Tiangai （天覆：并行）
//   - PriorityNormal → Dizai   （地载：顺序，默认）
//   - PriorityLow    → Dizai   （地载：顺序，默认）
func TestDefaultPlanner_Priority(t *testing.T) {
	cases := []struct {
		name     string
		priority model.Priority
		want     model.BaguaMode
	}{
		{"urgent→fengyang", model.PriorityUrgent, model.Fengyang},
		{"high→tiangai", model.PriorityHigh, model.Tiangai},
		{"normal→dizai", model.PriorityNormal, model.Dizai},
		{"low→dizai", model.PriorityLow, model.Dizai},
	}
	p := &DefaultPlanner{}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			order := &model.Order{
				ID:       model.OrderID("o-" + c.name),
				Priority: c.priority,
				Strategy: model.Strategy{Objectives: []string{"obj"}},
			}
			s, err := p.Plan(context.Background(), order)
			require.NoError(t, err)
			require.NotNil(t, s)
			assert.Equal(t, c.want, s.BaguaMode, "BaguaMode for %s priority", c.priority)
			assert.Equal(t, "default", s.Type, "Strategy.Type 应恒为 default")
		})
	}
}

// TestDefaultPlanner_TacticsCount 验证 Tactics 数量与 Order 序号连续性。
//
// 输入 3 个 objectives → 输出 3 个 tactics → Order 1/2/3 连续递增；
// 名称直接复用 objective 字符串，Action 默认 "execute"。
func TestDefaultPlanner_TacticsCount(t *testing.T) {
	objectives := []string{"水淹七军", "火攻连营", "空城退敌"}
	order := &model.Order{
		ID:       "o-wuhu",
		Priority: model.PriorityNormal,
		Strategy: model.Strategy{Objectives: objectives},
	}
	s, err := (&DefaultPlanner{}).Plan(context.Background(), order)
	require.NoError(t, err)
	require.NotNil(t, s)

	require.Equal(t, 3, len(s.Tactics), "Tactics 数量应等于 Objectives 数量")
	for i, tactic := range s.Tactics {
		assert.Equal(t, i+1, tactic.Order, "Tactic.Order 必须 1-based 连续")
		assert.Equal(t, objectives[i], tactic.Name, "Tactic.Name 应复用 objective 字符串")
		assert.Equal(t, "execute", tactic.Action, "Tactic.Action 默认值")
		assert.Contains(t, tactic.Description, objectives[i], "Description 应含原 objective")
	}
}

// TestDefaultPlanner_EmptyObjectives 验证无 objective 时 Tactics 为空切片（不 panic）。
func TestDefaultPlanner_EmptyObjectives(t *testing.T) {
	order := &model.Order{
		ID:       "o-empty",
		Priority: model.PriorityNormal,
		// 故意不设置 Objectives
	}
	s, err := (&DefaultPlanner{}).Plan(context.Background(), order)
	require.NoError(t, err)
	require.NotNil(t, s)
	assert.Empty(t, s.Tactics)
	assert.Equal(t, model.Dizai, s.BaguaMode)
}

// TestDefaultPlanner_Compiles 编译期断言：DefaultPlanner 必须实现 Planner。
//
// 当 DefaultPlanner 缺失方法或签名错误时，编译失败比运行期 panic 更友好。
func TestDefaultPlanner_Compiles(t *testing.T) {
	var _ Planner = (*DefaultPlanner)(nil)
}
