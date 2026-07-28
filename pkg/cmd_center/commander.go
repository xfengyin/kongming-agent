// 参谋部 - 核心调度与命令系统
// 运筹帷幄之中，决胜千里之外
// 依赖倒置：仅依赖 ExpertExecutor 端口，不依赖 generals 具体实现

package cmd_center

import (
	"context"
	"fmt"
	"sync"
	"time"

	"go.uber.org/zap"
)

// Commander 军师 - 核心调度器
type Commander struct {
	logger         *zap.Logger
	expertExecutor ExpertExecutor // 专家执行器端口（依赖注入）
	orders         map[string]*MilitaryOrder
	reports        map[string]*BattleReport
	mu             sync.RWMutex
}

// NewCommander 创建军师
// expertExecutor 由外部注入（generals.MoEExpertPool 实现该端口），打破循环依赖
func NewCommander(logger *zap.Logger, expertExecutor ExpertExecutor) *Commander {
	return &Commander{
		logger:         logger,
		expertExecutor: expertExecutor,
		orders:         make(map[string]*MilitaryOrder),
		reports:        make(map[string]*BattleReport),
	}
}

// Dispatch 颁布军令
// 对应 kimi-k3 的"路由激活专家 -> 专家执行 -> 结果聚合"主流程
func (c *Commander) Dispatch(ctx context.Context, order *MilitaryOrder) (*BattleReport, error) {
	c.mu.Lock()
	if order.ID == "" {
		order.ID = fmt.Sprintf("order_%d", time.Now().UnixNano())
	}
	order.State = StatePlanning
	c.orders[order.ID] = order
	c.mu.Unlock()

	c.logger.Info("军令已颁布",
		zap.String("order_id", order.ID),
		zap.String("name", order.Name),
	)

	// 制定战略
	strategy, err := c.PlanStrategy(ctx, order)
	if err != nil {
		return nil, fmt.Errorf("战略制定失败: %w", err)
	}
	order.Strategy = *strategy
	order.State = StateExecuting

	report := &BattleReport{
		OrderID:   order.ID,
		StartedAt: time.Now(),
		Generals:  make([]GeneralReport, 0),
	}

	// 按战略战术顺序执行：每个战术由 ExpertExecutor 路由到合适专家
	for _, tactic := range strategy.Tactics {
		// 战术的 Action 即路由技能键
		skill := tactic.Action
		if skill == "" {
			skill = tactic.Name
		}

		tacticalOrder := &MilitaryOrder{
			ID:          fmt.Sprintf("%s_%s", order.ID, tactic.Name),
			Name:        tactic.Name,
			Description: tactic.Description,
			Context:     order.Context,
		}

		// 通过端口路由并执行（Top-1 激活）
		generalReport, err := c.expertExecutor.ExecuteBySkill(ctx, skill, tacticalOrder)
		if err != nil {
			c.logger.Error("专家执行失败",
				zap.String("tactic", tactic.Name),
				zap.String("skill", skill),
				zap.Error(err),
			)
			report.Generals = append(report.Generals, GeneralReport{
				Success: false,
				Message: fmt.Sprintf("战术 %s 执行失败: %v", tactic.Name, err),
			})
			continue
		}
		report.Generals = append(report.Generals, *generalReport)
	}

	report.CompletedAt = time.Now()
	report.Success = len(report.Generals) > 0
	for _, gr := range report.Generals {
		if !gr.Success {
			report.Success = false
			break
		}
	}

	// 审核
	c.Review(ctx, report)

	c.mu.Lock()
	c.reports[order.ID] = report
	order.State = StateCompleted
	c.mu.Unlock()

	return report, nil
}

// PlanStrategy 制定战略
func (c *Commander) PlanStrategy(ctx context.Context, order *MilitaryOrder) (*Strategy, error) {
	strategy := &Strategy{
		Objectives: order.Strategy.Objectives,
		Tactics:    make([]Tactic, 0),
		BaguaMode:  "dizai",
	}

	// 根据优先级调整八卦阵模式
	switch order.Priority {
	case PriorityUrgent:
		strategy.BaguaMode = "fengyang" // 风扬阵 - 快速响应
	case PriorityHigh:
		strategy.BaguaMode = "tiangai" // 天覆阵 - 并行执行
	default:
		strategy.BaguaMode = "dizai" // 地载阵 - 顺序执行
	}

	// 添加战术步骤：Action 字段作为 MoE 路由的技能键
	for i, obj := range order.Strategy.Objectives {
		strategy.Tactics = append(strategy.Tactics, Tactic{
			Order:       i + 1,
			Name:        obj,
			Description: fmt.Sprintf("执行目标: %s", obj),
			Action:      inferSkillFromObjective(obj),
		})
	}

	return strategy, nil
}

// inferSkillFromObjective 从目标推断路由技能
// 简化的启发式映射，实际生产可由 LLM 或配置驱动
func inferSkillFromObjective(objective string) string {
	skillMap := map[string]string{
		"收集":  "data_collection",
		"搜集":  "data_collection",
		"处理":  "data_processing",
		"清洗":  "data_cleaning",
		"分析":  "data_analysis",
		"可视化": "visualization",
		"报告":  "report_generation",
		"撰写":  "writing",
		"审核":  "quality_check",
		"校验":  "validation",
	}
	for keyword, skill := range skillMap {
		if contains(objective, keyword) {
			return skill
		}
	}
	return "data_collection" // 默认技能
}

// contains 简单的子串包含判断
func contains(s, substr string) bool {
	if len(substr) == 0 {
		return true
	}
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// Review 审核战报
func (c *Commander) Review(ctx context.Context, report *BattleReport) error {
	if !report.Success {
		return fmt.Errorf("战报审核失败: %s", report.Message)
	}
	for _, gr := range report.Generals {
		if gr.Success {
			c.logger.Info("专家立功",
				zap.String("expert", gr.GeneralName),
				zap.String("message", gr.Message),
			)
		}
	}
	return nil
}

// GetOrder 查询军令
func (c *Commander) GetOrder(orderID string) (*MilitaryOrder, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	order, exists := c.orders[orderID]
	if !exists {
		return nil, fmt.Errorf("军令不存在: %s", orderID)
	}
	return order, nil
}

// ListOrders 列出军令
func (c *Commander) ListOrders(state TaskState) []*MilitaryOrder {
	c.mu.RLock()
	defer c.mu.RUnlock()
	orders := make([]*MilitaryOrder, 0)
	for _, order := range c.orders {
		if state == 0 || order.State == state {
			orders = append(orders, order)
		}
	}
	return orders
}

// 确保 Commander 实现 CommanderPort 端口
var _ CommanderPort = (*Commander)(nil)
