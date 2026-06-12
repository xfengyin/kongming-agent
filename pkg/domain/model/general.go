// Package model 领域模型 - 将领（General）聚合。
//
// 本文件定义 General 聚合的完整定义：GeneralID、GeneralType（五虎将常量）、
// GeneralState 状态机、General 结构体、GeneralStats 统计。
//
// Stage 1 兼容说明：保留原 `State int` 字段以保证 Stage 1 调用方不破坏；
// 新增 GeneralState 枚举 + SetState/GetState 线程安全方法。
// 两个字段共用同一底层 int 存储：Stage 1 写入 g.State 立即可被
// Stage 2 的 g.GetState() 读到，反之亦然。
package model

import (
	"sync"
	"time"
)

// GeneralID 是 General 的唯一标识。string 别名，但作为独立类型以便
// Stage 2 在不破坏调用方的前提下扩展方法集与状态机。
type GeneralID string

// GeneralType 是将领类型，对应「五虎将」枚举。
//
// 使用 string 别名便于配置/YAML/JSON 反序列化与多语言展示。
type GeneralType string

const (
	// GeneralGuanYu 关羽 - 武圣
	GeneralGuanYu GeneralType = "guanyu"
	// GeneralZhangFei 张飞 - 万人敌
	GeneralZhangFei GeneralType = "zhangfei"
	// GeneralZhaoYun 赵云 - 一身是胆
	GeneralZhaoYun GeneralType = "zhaoyun"
	// GeneralMaChao 马超 - 一骑当千
	GeneralMaChao GeneralType = "machao"
	// GeneralHuangZhong 黄忠 - 老当益壮
	GeneralHuangZhong GeneralType = "huangzhong"
)

// GeneralState 描述 General 当前的工作状态。
//
// 设计为 int 枚举，与 Stage 1 的 `State int` 字段共用底层存储。
// 状态机迁移规则（业务层调用 SetState 时校验）：
//   - GeneralIdle    空闲，可被 Commander 派单
//   - GeneralBusy    执行中，不可被抢占
//   - GeneralResting 冷却中（最近执行失败/超时），不可被派单
//   - GeneralOffline 离线（不可达），不可被派单
type GeneralState int

const (
	// GeneralIdle 将领空闲，可被派单。
	GeneralIdle GeneralState = iota
	// GeneralBusy 将领正在执行任务，不可被抢占。
	GeneralBusy
	// GeneralResting 将领冷却中（如执行失败后进入冷却），不可被派单。
	GeneralResting
	// GeneralOffline 将领离线（不可达/禁用），不可被派单。
	GeneralOffline
)

// String 返回 GeneralState 的可读名称，便于日志展示。
func (s GeneralState) String() string {
	switch s {
	case GeneralIdle:
		return "idle"
	case GeneralBusy:
		return "busy"
	case GeneralResting:
		return "resting"
	case GeneralOffline:
		return "offline"
	default:
		return "unknown"
	}
}

// General 是将领聚合根。
//
// Stage 1 字段（ID/Name/State/CreatedAt）保留以保证向后兼容；Stage 2 扩展字段
// （Type/Title/Description/Skills/Traits/Stats）以「追加」方式加入。
// 内嵌 sync.Mutex 为 State 字段变更提供互斥访问。
type General struct {
	// ID 唯一标识一个 General。
	ID GeneralID
	// Name 人类可读的将领名。
	Name string
	// Type 将领类型（五虎将枚举），便于按类型筛选/路由。
	Type GeneralType
	// Title 头衔/官职（如「前将军」），展示用。
	Title string
	// Description 将领详细描述，运维/审计场景使用。
	Description string
	// Skills 将领掌握的能力标签（如 ["llm", "tool", "search"]），用于匹配锦囊。
	Skills []string
	// Traits 性格/偏好配置（KPI 权重、风险偏好等），可空。
	Traits map[string]any
	// Stats 统计指标（累计任务数/成功率/平均响应时间）。
	Stats GeneralStats
	// State 将领状态。Stage 1 兼容字段（int 类型），与 GeneralState 枚举共用底层存储。
	// Stage 2 推荐使用 SetState/GetState 线程安全方法。
	State int
	// CreatedAt 创建时间。
	CreatedAt time.Time
	// mu 保护 State 字段的并发读写。
	// 嵌入而非命名（匿名），避免污染导出 API；
	// 嵌入后 *General 仍实现 sync.Locker 语义。
	mu sync.Mutex
}

// SetState 线程安全地设置将领状态。
//
// 加锁写入，避免与并发派单/统计上报产生竞态。
func (g *General) SetState(s GeneralState) {
	g.mu.Lock()
	g.State = int(s)
	g.mu.Unlock()
}

// GetState 线程安全地读取将领状态。
func (g *General) GetState() GeneralState {
	g.mu.Lock()
	defer g.mu.Unlock()
	return GeneralState(g.State)
}

// GeneralStats 将领的运行统计。
//
// 由 application/general 在每次执行完成后更新，可用于评分/淘汰/告警。
type GeneralStats struct {
	// TotalMissions 累计执行任务数。
	TotalMissions int
	// SuccessCount 累计成功任务数。
	SuccessCount int
	// FailureCount 累计失败任务数。
	FailureCount int
	// AvgResponseTime 平均响应时间（秒），用于超时策略。
	AvgResponseTime float64
}
