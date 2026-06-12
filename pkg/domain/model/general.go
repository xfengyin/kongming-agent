// Package model 领域模型层。
//
// 本文件定义 General 聚合的最小子集：GeneralID 与 General 结构体。
// Stage 2 会扩展 Name/Type/Title/Description/Skills/Traits/Stats/State 等字段，
// 并补全 GeneralState 状态机（Idle/Busy/Resting/Offline）。
package model

import "time"

// GeneralID 是 General 的唯一标识。string 别名，但作为独立类型以便
// Stage 2 在不破坏调用方的前提下扩展方法集与状态机。
type GeneralID string

// General 是将领聚合根的最小子集。
//
// Stage 1 仅包含 ID/Name/State/CreatedAt 四个字段，足够让 Store 与
// OrderRepo 共享内存并通过测试验证同步原语（sync.Map）。
//
// Stage 2 将按 spec §2.1 扩展为完整聚合。
type General struct {
	// ID 唯一标识一个 General。
	ID GeneralID
	// Name 人类可读的将领名。
	Name string
	// State 将领状态（最小占位枚举，Stage 2 引入 GeneralState 状态机）。
	State int
	// CreatedAt 创建时间。
	CreatedAt time.Time
}
