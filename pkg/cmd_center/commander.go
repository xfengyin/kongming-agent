// 参谋部 - 核心调度与命令系统
// 运筹帷幄之中，决胜千里之外

package cmd_center

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/zhuge/kongming/pkg/generals"
	"github.com/zhuge/kongming/pkg/strategy_vault"
	"github.com/zhuge/kongming/pkg/courier"
	"go.uber.org/zap"
)

// Commander 军师 - 核心调度器
type Commander struct {
	logger       *zap.Logger
	generalPool  generals.GeneralPool
	strategyVault strategy_vault.Vault
	courier     *courier.Courier
	orders      map[string]*MilitaryOrder
	reports     map[string]*BattleReport
	mu          sync.RWMutex
}

// NewCommander 创建军师
func NewCommander(logger *zap.Logger) *Commander {
	return &Commander{
		logger:        logger,
		generalPool:   generals.NewWuHuPool(),
		strategyVault: strategy_vault.NewVault(),
		orders:        make(map[string]*MilitaryOrder),
		reports:       make(map[string]*BattleReport),
	}
}

// Dispatch 颁布军令
func (c *Commander) Dispatch(ctx context.Context, order *MilitaryOrder) (*BattleReport, error) {
	c.mu.Lock()
	if order.ID == "" {
		order.ID = fmt.Sprintf("order_%d", time.Now().UnixNano())
	}
	order.State = StatePlanning
	c.orders[order.ID] = order
	c.mu.Unlock()

	c.logger.Info("军令已颁布", zap.String("order_id", order.ID), zap.String("name", order.Name))

	// 制定战略
	strategy, err := c.PlanStrategy(ctx, order)
	if err != nil {
		return nil, fmt.Errorf("战略制定失败: %w", err)
	}
	order.Strategy = *strategy
	order.State = StateExecuting

	report := &BattleReport{
		OrderID:     order.ID,
		StartedAt:   time.Now(),
		Generals:    make([]GeneralReport, 0),
	}

	// 根据战略执行
	for _, tactic := range strategy.Tactics {
		// 选择将领
		general, err := c.selectGeneral(tactic)
		if err != nil {
			c.logger.Warn("无合适将领", zap.String("tactic", tactic.Name))
			continue
		}

		// 派遣执行
		tacticalOrder := &MilitaryOrder{
			ID:          fmt.Sprintf("%s_%s", order.ID, tactic.Name),
			Name:        tactic.Name,
			Description: tactic.Description,
			Context:     order.Context,
		}

		generalReport, err := c.generalPool.Execute(ctx, general.ID, tacticalOrder)
		if err != nil {
			c.logger.Error("将领执行失败", zap.String("general", general.Name), zap.Error(err))
			continue
		}

		report.Generals = append(report.Generals, *generalReport)
	}

	report.CompletedAt = time.Now()
	report.Success = true

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
	// 根据任务类型制定战略
	strategy := &Strategy{
		Objectives: order.Strategy.Objectives,
		Tactics:    make([]Tactic, 0),
		BaguaMode:  "dizai",
	}

	// 根据优先级调整战略
	switch order.Priority {
	case PriorityUrgent:
		strategy.BaguaMode = "fengyang" // 风扬阵 - 快速响应
	case PriorityHigh:
		strategy.BaguaMode = "tiangai"  // 天覆阵 - 并行执行
	default:
		strategy.BaguaMode = "dizai"    // 地载阵 - 顺序执行
	}

	// 添加战术步骤
	for i, obj := range order.Strategy.Objectives {
		strategy.Tactics = append(strategy.Tactics, Tactic{
			Order:       i + 1,
			Name:        obj,
			Description: fmt.Sprintf("执行目标: %s", obj),
			Action:      "execute",
		})
	}

	return strategy, nil
}

// Review 审核战报
func (c *Commander) Review(ctx context.Context, report *BattleReport) error {
	if !report.Success {
		return fmt.Errorf("战报审核失败: %s", report.Message)
	}

	// 统计将领表现
	for _, gr := range report.Generals {
		if gr.Success {
			c.logger.Info("将领立功",
				zap.String("general", gr.GeneralName),
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

func (c *Commander) selectGeneral(tactic Tactic) (*generals.General, error) {
	// 根据战术类型选择将领
	return c.generalPool.SelectBest(tactic.Action)
}

func (s TaskPriority) String() string {
	switch s {
	case PriorityLow:
		return "低"
	case PriorityNormal:
		return "普通"
	case PriorityHigh:
		return "高"
	case PriorityUrgent:
		return "紧急"
	default:
		return "未知"
	}
}
