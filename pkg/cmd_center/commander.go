// 参谋部 - 核心调度与命令系统
// 运筹帷幄之中，决胜千里之外

package cmd_center

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/zhuge/kongming/pkg/courier"
	"github.com/zhuge/kongming/pkg/generals"
	"github.com/zhuge/kongming/pkg/strategy_vault"
	"go.uber.org/zap"
)

// Commander 军师 - 核心调度器
type Commander struct {
	logger        *zap.Logger
	generalPool   generals.GeneralPool
	strategyVault strategy_vault.Vault
	courier       *courier.Courier
	orders        map[string]*MilitaryOrder
	reports       map[string]*BattleReport
	mu            sync.RWMutex
}

// NewCommander 创建军师
func NewCommander(logger *zap.Logger) *Commander {
	return NewCommanderWithPool(logger, generals.NewWuHuPool())
}

// NewCommanderWithPool 使用自定义将领池创建军师（如接入 LLM 的军师诸葛亮）
func NewCommanderWithPool(logger *zap.Logger, pool generals.GeneralPool) *Commander {
	return &Commander{
		logger:        logger,
		generalPool:   pool,
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
		OrderID:   order.ID,
		StartedAt: time.Now(),
		Generals:  make([]GeneralReport, 0),
	}

	// 根据战略执行
	// 优先执行战略中点名的将领（strategy.Generals），否则按战术步骤选择将领
	targetGenerals := strategy.Generals
	if len(targetGenerals) == 0 {
		for _, tactic := range strategy.Tactics {
			g, err := c.selectGeneral(tactic)
			if err != nil {
				c.logger.Warn("无合适将领", zap.String("tactic", tactic.Name))
				continue
			}
			targetGenerals = append(targetGenerals, g.ID)
		}
	}

	for _, gid := range targetGenerals {
		general, err := c.generalPool.Get(gid)
		if err != nil {
			c.logger.Warn("将领不存在", zap.String("general_id", gid))
			continue
		}

		// 派遣执行
		tacticalOrder := &MilitaryOrder{
			ID:          fmt.Sprintf("%s_%s", order.ID, general.Name),
			Name:        general.Name,
			Description: order.Description,
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
		Type:       order.Strategy.Type,
		Objectives: order.Strategy.Objectives,
		Generals:   order.Strategy.Generals, // 保留点名将领
		JinnangIDs: order.Strategy.JinnangIDs,
		Tactics:    make([]Tactic, 0),
		BaguaMode:  "dizai",
	}

	// 根据优先级调整战略
	switch order.Priority {
	case PriorityUrgent:
		strategy.BaguaMode = "fengyang" // 风扬阵 - 快速响应
	case PriorityHigh:
		strategy.BaguaMode = "tiangai" // 天覆阵 - 并行执行
	default:
		strategy.BaguaMode = "dizai" // 地载阵 - 顺序执行
	}

	// 添加战术步骤：按目标关键词匹配合适将领技能
	for i, obj := range order.Strategy.Objectives {
		strategy.Tactics = append(strategy.Tactics, Tactic{
			Order:       i + 1,
			Name:        obj,
			Description: fmt.Sprintf("执行目标: %s", obj),
			Action:      skillForObjective(obj),
		})
	}

	return strategy, nil
}

// skillForObjective 按目标文本关键词映射将领技能
func skillForObjective(obj string) string {
	switch {
	case containsAny(obj, "情报", "搜集", "收集", "调研", "竞品", "资料", "信息"):
		return "data_collection"
	case containsAny(obj, "清洗", "结构化", "数据处理", "工程", "ETL"):
		return "data_processing"
	case containsAny(obj, "分析", "可视化", "图表", "洞察"):
		return "data_analysis"
	case containsAny(obj, "报告", "撰写", "文案", "文档", "总结"):
		return "writing"
	case containsAny(obj, "审核", "质检", "校验", "验收", "把关"):
		return "quality_check"
	default:
		return "llm" // 无法匹配时交由军师诸葛亮（LLM）
	}
}

func containsAny(s string, subs ...string) bool {
	for _, sub := range subs {
		if strings.Contains(s, sub) {
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
	g, err := c.generalPool.SelectBest(tactic.Action)
	if err == nil {
		return g, nil
	}
	// 兜底：选择任意待命将领
	idle := c.generalPool.List(generals.GeneralFilter{State: generals.GeneralIdle})
	if len(idle) == 0 {
		return nil, err
	}
	return idle[0], nil
}
