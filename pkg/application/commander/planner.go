// Package commander 提供军师应用层用例。
//
// 本文件定义 Planner 组件 —— Commander.PlanStrategy 的「战略规划」能力。
// Planner 与 Commander 解耦：Commander 只依赖 Planner 接口，未来可替换为
// LLMPlanner（基于大模型生成战略）/ RulePlanner（基于业务规则）/ HybridPlanner
// 等不同实现，符合开闭原则。
//
// 选型规则（DefaultPlanner）：
//   - PriorityUrgent → Fengyang（风扬：带短超时的快速响应）
//   - PriorityHigh   → Tiangai （天覆：并行 DAG）
//   - PriorityNormal/Low → Dizai（地载：顺序执行）
//
// Tactics 编排：每个 Objective 转换为一个 Tactic，Order 序号 1-based 连续。
package commander

import (
	"context"
	"fmt"

	"github.com/zhuge/kongming/pkg/domain/model"
)

// Planner 是战略规划器的端口接口。
//
// 实现方：DefaultPlanner、未来的 LLMPlanner、RulePlanner。
// 调用方：commander.Service.PlanStrategy / Dispatch。
//
// 设计要点：
//   - 单一职责：只做"输入 Order → 输出 Strategy"，不涉及任何执行/持久化；
//   - 幂等：相同 Order 入参应产生相同 Strategy 输出（便于幂等 replay）。
type Planner interface {
	// Plan 根据 Order 的属性（Priority/Objectives/Context）生成一份 Strategy。
	// 失败时应返回包装了根本因的 error，由 Commander 转译为 STRATEGY_FAILED。
	Plan(ctx context.Context, order *model.Order) (*model.Strategy, error)
}

// DefaultPlanner 是 Planner 的默认实现 —— 基于规则的简单映射。
//
// 设计目标：
//  1. 可预测：相同输入永远产生相同输出（便于幂等 & 测试断言）；
//  2. 零依赖：不依赖 LLM/外部配置，纯函数式；
//  3. 易于扩展：未来可作为 LLMPlanner 的「兜底/fallback」实现。
type DefaultPlanner struct{}

// NewDefaultPlanner 构造一个 DefaultPlanner。
// 当前无内部状态，等价于 &DefaultPlanner{}；保留构造函数以备未来注入配置。
func NewDefaultPlanner() *DefaultPlanner {
	return &DefaultPlanner{}
}

// Plan 实现 Planner 接口。
//
// 规则：
//   - Type 恒为 "default"（便于下游按 Strategy.Type 路由）；
//   - BaguaMode 按 Priority 映射（见文件头注释）；
//   - Tactics 数量 = len(Objectives)，每条 Tactic 复用 Objective 字符串作为 Name；
//   - Action 默认为 "execute"，由 Service 层在执行时通过 pool.SelectBest(Action) 选将。
func (p *DefaultPlanner) Plan(_ context.Context, order *model.Order) (*model.Strategy, error) {
	// 防御：理论上 Service 在调用 Planner 之前已校验 order != nil，
	// 但 Planner 作为对外可独立调用的能力，仍做最基本校验避免 panic。
	if order == nil {
		return nil, fmt.Errorf("planner: order is nil")
	}

	s := &model.Strategy{
		Type:       "default",
		Objectives: append([]string(nil), order.Strategy.Objectives...),
		Tactics:    make([]model.Tactic, 0, len(order.Strategy.Objectives)),
		BaguaMode:  model.Dizai, // 默认地载
	}

	// 按 Priority 选择八卦阵
	switch order.Priority {
	case model.PriorityUrgent:
		s.BaguaMode = model.Fengyang
	case model.PriorityHigh:
		s.BaguaMode = model.Tiangai
	case model.PriorityNormal, model.PriorityLow:
		s.BaguaMode = model.Dizai
	}

	// 每个 Objective 转换为一个 Tactic
	for i, obj := range order.Strategy.Objectives {
		s.Tactics = append(s.Tactics, model.Tactic{
			Order:       i + 1, // 1-based 连续序号
			Name:        obj,
			Description: fmt.Sprintf("执行目标: %s", obj),
			Action:      "execute",
		})
	}
	return s, nil
}

// 编译期断言：DefaultPlanner 必须实现 Planner。
var _ Planner = (*DefaultPlanner)(nil)
