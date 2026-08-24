// 军师府 - 核心决策与任务调度
// 运筹帷幄之中，决胜千里之外
//
// 军令/战报等共享域类型定义于 pkg/core，此处以别名导出以保持既有 API 兼容。

package cmd_center

import (
	"github.com/zhuge/kongming/pkg/core"
)

// 常量定义
const (
	DefaultTimeout = core.DefaultTimeout // 默认超时时间
)

// TaskState 任务状态
type TaskState = core.TaskState

const (
	StatePending   = core.StatePending   // 待处理
	StatePlanning  = core.StatePlanning  // 谋划中
	StateExecuting = core.StateExecuting // 执行中
	StateReviewing = core.StateReviewing // 审核中
	StateCompleted = core.StateCompleted // 已完成
	StateFailed    = core.StateFailed    // 失败
)

// TaskPriority 任务优先级
type TaskPriority = core.TaskPriority

const (
	PriorityLow    = core.PriorityLow    // 低优先级
	PriorityNormal = core.PriorityNormal // 普通优先级
	PriorityHigh   = core.PriorityHigh   // 高优先级
	PriorityUrgent = core.PriorityUrgent // 紧急军情
)

// MilitaryOrder 军令（任务定义）
type MilitaryOrder = core.MilitaryOrder

// NewMilitaryOrder 创建新军令
func NewMilitaryOrder(name, description string, priority TaskPriority) *MilitaryOrder {
	return core.NewMilitaryOrder(name, description, priority)
}

// Strategy 战略方案
type Strategy = core.Strategy

// Tactic 战术步骤
type Tactic = core.Tactic

// BattleReport 战报
type BattleReport = core.BattleReport

// GeneralReport 将领战报
type GeneralReport = core.GeneralReport

// Event 事件
type Event = core.Event

// EventHandler 事件处理器
type EventHandler = core.EventHandler

// Commander 军师：完整实现见 commander.go（结构体类型）
