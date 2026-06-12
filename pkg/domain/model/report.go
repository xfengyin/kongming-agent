// Package model 领域模型 - 战报（BattleReport）聚合。
//
// 一份战报聚合一次完整「军师派单 → 将领执行」的结果，包含总览（OrderID/Success/
// Duration）以及每位将领的子报告（GeneralReport）。由 application/commander 在
// 收齐所有 GeneralReport 后归并生成。
package model

import "time"

// BattleReport 是单次派单的总战报。
//
// Commander.Dispatch 完成后回传，UI/CLI/Reviewer 据此判断是否成功、是否需要重试。
// 字段尽量扁平便于 JSON 序列化；Result 字段保留为 map[string]any 让业务方扩展
// 自定义指标（如「清理文件数」「调用 LLM token 数」），不污染核心结构。
type BattleReport struct {
	// OrderID 关联的 Order 唯一标识。
	OrderID OrderID
	// Generals 各将领的子报告列表，顺序与 Order.Generals 无关。
	Generals []GeneralReport
	// Success 所有将领是否全部成功（一个失败则整体 false）。
	Success bool
	// StartedAt 派单开始时间。
	StartedAt time.Time
	// CompletedAt 派单完成时间（含成功/失败/超时）。
	CompletedAt time.Time
	// Duration 派单总耗时（秒），由 StartedAt/CompletedAt 计算得出。
	// 冗余存储避免调用方反复相减。
	Duration float64
	// Result 业务方自定义结果（性能指标、计费、清理量等），可空。
	Result map[string]any
}

// GeneralReport 是单个将领的子战报。
type GeneralReport struct {
	// GeneralID 关联的 General 唯一标识。
	GeneralID GeneralID
	// Name 将领名（冗余存储，避免展示时再查仓库）。
	Name string
	// Success 该将领是否成功。
	Success bool
	// Output 业务输出（与 JinnangOutput.Data 形态一致）。
	Output any
	// Error 错误描述（仅在 Success=false 时有值）。
	Error string
	// Duration 该将领的执行耗时（秒）。
	Duration float64
}
