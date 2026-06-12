// Package model 领域模型单元测试。
//
// 覆盖范围：状态机迁移、优先级、Order 扩展字段。
//
// 测试策略：纯函数 + 边界条件，不依赖任何外部 IO。
package model

import (
	"testing"
	"time"
)

// TestStateTransition_Valid 验证合法状态迁移路径：
//   - happy path：pending → planning → executing → reviewing → completed
//   - 重试：failed → pending
func TestStateTransition_Valid(t *testing.T) {
	cases := []struct {
		from State
		to   State
	}{
		{StatePending, StatePlanning},
		{StatePlanning, StateExecuting},
		{StateExecuting, StateReviewing},
		{StateReviewing, StateCompleted},
		{StatePending, StateFailed},
		{StatePlanning, StateFailed},
		{StateExecuting, StateFailed},
		{StateReviewing, StateFailed},
		{StateFailed, StatePending},
	}
	for _, c := range cases {
		if err := c.from.TransitionTo(c.to); err != nil {
			t.Errorf("expected %s -> %s to be valid, got error: %v", c.from, c.to, err)
		}
	}
}

// TestStateTransition_Invalid 验证非法迁移被拒绝：
//   - 跳跃式迁移：pending → completed
//   - 终态回退：completed → pending
//   - 自迁移：任何状态到自己
//   - 未声明状态：StateNone 不可作为迁移目标
func TestStateTransition_Invalid(t *testing.T) {
	cases := []struct {
		from State
		to   State
	}{
		{StatePending, StateCompleted},
		{StatePending, StateReviewing},
		{StatePlanning, StateCompleted},
		{StateCompleted, StatePending},
		{StateCompleted, StatePlanning},
		{StateFailed, StateCompleted},
		{StatePending, StatePending},
		{StateCompleted, StateCompleted},
		{StateNone, StatePending},
	}
	for _, c := range cases {
		if err := c.from.TransitionTo(c.to); err == nil {
			t.Errorf("expected %s -> %s to be invalid, got nil error", c.from, c.to)
		}
	}
}

// TestPriority_String 验证 Priority 的可读字符串与零值（unknown）兜底。
func TestPriority_String(t *testing.T) {
	cases := map[Priority]string{
		PriorityLow:    "low",
		PriorityNormal: "normal",
		PriorityHigh:   "high",
		PriorityUrgent: "urgent",
		Priority(0):    "unknown",
		Priority(99):   "unknown",
	}
	for p, want := range cases {
		if got := p.String(); got != want {
			t.Errorf("Priority(%d).String() = %q, want %q", p, got, want)
		}
	}
}

// TestState_String 验证 State 的可读字符串。
func TestState_String(t *testing.T) {
	cases := map[State]string{
		StateNone:      "none",
		StatePending:   "pending",
		StatePlanning:  "planning",
		StateExecuting: "executing",
		StateReviewing: "reviewing",
		StateCompleted: "completed",
		StateFailed:    "failed",
		State(99):      "unknown",
	}
	for s, want := range cases {
		if got := s.String(); got != want {
			t.Errorf("State(%d).String() = %q, want %q", s, got, want)
		}
	}
}

// TestOrder_AppendFields 验证 Order 的扩展字段可正确赋值。
// 这是一个「烟雾测试」，确保新增字段不会因为类型不匹配而失败。
func TestOrder_AppendFields(t *testing.T) {
	now := time.Now()
	deadline := now.Add(2 * time.Hour)
	parent := OrderID("ord-parent")
	ctx := map[string]any{"trace_id": "t-1"}

	o := Order{
		ID:          "ord-1",
		Name:        "test",
		Description: "demo",
		State:       StatePending,
		Priority:    PriorityHigh,
		Strategy: Strategy{
			Type:      "offensive",
			BaguaMode: Tiangai,
		},
		Context:   ctx,
		CreatedAt: now,
		UpdatedAt: now,
		Deadline:  &deadline,
		Parent:    parent,
		Generals:  []GeneralID{"guanyu", "zhangfei"},
	}

	if o.ID != "ord-1" {
		t.Errorf("ID mismatch: %s", o.ID)
	}
	if o.Priority != PriorityHigh {
		t.Errorf("Priority mismatch: %v", o.Priority)
	}
	if o.Strategy.BaguaMode != Tiangai {
		t.Errorf("Strategy.BaguaMode mismatch: %v", o.Strategy.BaguaMode)
	}
	if o.Deadline == nil || !o.Deadline.Equal(deadline) {
		t.Errorf("Deadline mismatch: %v", o.Deadline)
	}
	if o.Parent != parent {
		t.Errorf("Parent mismatch: %s", o.Parent)
	}
	if len(o.Generals) != 2 {
		t.Errorf("Generals len mismatch: %d", len(o.Generals))
	}
	if o.Context["trace_id"] != "t-1" {
		t.Errorf("Context mismatch: %v", o.Context)
	}
}
