// Package model 存放领域实体（domain layer）。
//
// 本文件定义 Order 聚合的最小子集：OrderID、State 枚举与 Order 结构体。
// Stage 2 会基于此文件扩展 Priority、Strategy、Context、UpdatedAt、Deadline、
// Parent、Generals 等字段，并补全 State 状态机迁移表（stateTransitions）。
//
// 设计原则：
//  1. 零外部依赖：不 import 任何 kongming 子包或第三方库。
//  2. 值类型：OrderID/State 用类型别名以增强可读性，避免误用 string/int。
//  3. 后向兼容：扩展字段使用「追加」而非「修改」语义，确保后续 stage 替换时
//     persistence 层不会发生破坏性变更。
package model

import "time"

// OrderID 是订单的唯一标识。string 别名，便于日志/JSON 序列化。
type OrderID string

// State 描述 Order 在生命周期中所处的状态。
//
// 当前为最小枚举（仅保留持久化层需要的 Pending / Completed），Stage 2 会
// 扩展 Planning/Executing/Reviewing/Failed 并补全 TransitionTo 状态机。
type State int

const (
	// StateNone 为零值默认状态，表示「未初始化」。List(state=0) 视作「不过滤」。
	StateNone State = iota
	// StatePending 订单已创建但尚未开始处理。
	StatePending
	// StateCompleted 订单成功完成。
	StateCompleted
)

// String 返回 State 的可读名称，便于日志/错误信息展示。
func (s State) String() string {
	switch s {
	case StateNone:
		return "none"
	case StatePending:
		return "pending"
	case StateCompleted:
		return "completed"
	default:
		return "unknown"
	}
}

// Order 是军师系统中最高层级的聚合根。
//
// 最小字段集（Stage 1）仅包含持久化层必需的主键、名称、状态与创建时间。
// Stage 2 将按 spec §2.1 扩展为完整聚合（Description/Priority/Strategy/Context/
// UpdatedAt/Deadline/Parent/Generals 等），新增字段为追加，不修改现有字段语义。
type Order struct {
	// ID 唯一标识一个 Order，必填。
	ID OrderID
	// Name 人类可读的订单名，便于运维/CLI 展示。
	Name string
	// State 订单当前状态，由状态机驱动。
	State State
	// CreatedAt 订单创建时间，用于审计/排序。
	CreatedAt time.Time
}
