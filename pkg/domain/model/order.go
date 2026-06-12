// Package model 存放领域实体（domain layer）。
//
// 本文件定义 Order 聚合的完整定义：OrderID、Priority、State 状态机与 Order 结构体。
//
// 设计原则：
//  1. 零外部依赖：不 import 任何 kongming 子包或第三方库。
//  2. 值类型：OrderID/Priority/State 用类型别名以增强可读性，避免误用 string/int。
//  3. 后向兼容：Stage 1 的 ID/Name/State/CreatedAt 字段保留，新字段是「追加」而非「修改」。
//  4. 状态机显式：所有合法迁移路径通过 stateTransitions 表声明，非法迁移直接拒绝。
package model

import (
	"fmt"
	"time"
)

// OrderID 是订单的唯一标识。string 别名，便于日志/JSON 序列化。
type OrderID string

// Priority 描述订单的优先级。
//
// 取值范围 1..4，0 视为未指定（unknown）。数值越大越紧急，便于调度器排序。
type Priority int

const (
	// PriorityLow 低优先级，可被延迟处理。
	PriorityLow Priority = iota + 1
	// PriorityNormal 默认优先级。
	PriorityNormal
	// PriorityHigh 高优先级。
	PriorityHigh
	// PriorityUrgent 紧急优先级，应被优先调度。
	PriorityUrgent
)

// String 返回 Priority 的可读名称，便于日志/错误信息展示。
func (p Priority) String() string {
	switch p {
	case PriorityLow:
		return "low"
	case PriorityNormal:
		return "normal"
	case PriorityHigh:
		return "high"
	case PriorityUrgent:
		return "urgent"
	default:
		return "unknown"
	}
}

// State 描述 Order 在生命周期中所处的状态。
//
// 状态机迁移规则见 stateTransitions 表，调用方应使用 TransitionTo 方法
// 而不是直接赋值以保证迁移合法性。
type State int

const (
	// StateNone 为零值默认状态，表示「未初始化」。List(state=0) 视作「不过滤」。
	StateNone State = iota
	// StatePending 订单已创建但尚未开始处理。
	StatePending
	// StatePlanning 正在制定战略（选择八卦阵/分配将领/编排 Tactics）。
	StatePlanning
	// StateExecuting 战术执行中（将领调度/工作流执行）。
	StateExecuting
	// StateReviewing 等待审核（人工或自动 Reviewer 校验战报）。
	StateReviewing
	// StateCompleted 订单成功完成（终态）。
	StateCompleted
	// StateFailed 订单失败（可经由 TransitionTo(StatePending) 重试）。
	StateFailed
)

// String 返回 State 的可读名称，便于日志/错误信息展示。
func (s State) String() string {
	switch s {
	case StateNone:
		return "none"
	case StatePending:
		return "pending"
	case StatePlanning:
		return "planning"
	case StateExecuting:
		return "executing"
	case StateReviewing:
		return "reviewing"
	case StateCompleted:
		return "completed"
	case StateFailed:
		return "failed"
	default:
		return "unknown"
	}
}

// stateTransitions 声明每个状态允许的合法迁移目标。
//
// 设计要点：
//   - StateCompleted 为终态，不允许任何迁出（避免终态被覆盖）。
//   - StateFailed → StatePending 允许「失败重试」，由 Commander 决定是否触发。
//   - StateNone 不在迁移表中，确保「未初始化」不会被误用作起点。
//   - 自迁移（s == next）一律不在合法表中，因此自然被拒绝。
var stateTransitions = map[State][]State{
	StatePending:   {StatePlanning, StateFailed},
	StatePlanning:  {StateExecuting, StateFailed},
	StateExecuting: {StateReviewing, StateFailed},
	StateReviewing: {StateCompleted, StateFailed},
	StateCompleted: {},
	StateFailed:    {StatePending},
}

// TransitionTo 检查从当前状态到 next 的迁移是否合法。
//
// 合法返回 nil；非法返回带状态名的错误，供上层包装为 CodeInvalidState。
func (s State) TransitionTo(next State) error {
	for _, allowed := range stateTransitions[s] {
		if allowed == next {
			return nil
		}
	}
	return fmt.Errorf("invalid state transition: %s -> %s", s, next)
}

// Order 是军师系统中最高层级的聚合根。
//
// Stage 1 字段（ID/Name/State/CreatedAt）保留以保证向后兼容；Stage 2 扩展字段
// （Description/Priority/Strategy/Context/UpdatedAt/Deadline/Parent/Generals）以
// 「追加」方式加入，不修改现有字段语义。
type Order struct {
	// ID 唯一标识一个 Order，必填。
	ID OrderID
	// Name 人类可读的订单名，便于运维/CLI 展示。
	Name string
	// Description 详细描述，用于审计与下游展示。
	Description string
	// State 订单当前状态，由状态机驱动。
	State State
	// Priority 订单优先级，影响调度顺序。
	Priority Priority
	// Strategy 派单前制定的战略（八卦阵/战术/将领分配）。
	Strategy Strategy
	// Context 跨层透传的上下文（如 trace_id/user_id），可空。
	Context map[string]any
	// CreatedAt 订单创建时间，用于审计/排序。
	CreatedAt time.Time
	// UpdatedAt 订单最近一次状态/字段更新时间。
	UpdatedAt time.Time
	// Deadline 可选截止时间，调度器应尽量在 Deadline 前完成。
	Deadline *time.Time
	// Parent 父订单 ID，用于构建子任务/依赖关系；空表示无父。
	Parent OrderID
	// Generals 派单锁定的将领 ID 列表。
	Generals []GeneralID
}
