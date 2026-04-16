// 参谋部 - 任务调度系统
// 调兵遣将，运筹决策

package dispatch

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/zhuge/kongming/pkg/cmd_center"
	"github.com/zhuge/kongming/pkg/observatory"
	"go.uber.org/zap"
)

// Dispatcher 调度器
type Dispatcher struct {
	logger    *zap.Logger
	orders    map[string]*cmd_center.MilitaryOrder
	results   map[string]*cmd_center.BattleReport
	executors map[string]Executor
	mu        sync.RWMutex
	running   bool
}

// Executor 执行器接口
type Executor interface {
	Execute(ctx context.Context, order *cmd_center.MilitaryOrder) (*cmd_center.BattleReport, error)
}

// NewDispatcher 创建调度器
func NewDispatcher(logger *zap.Logger) *Dispatcher {
	return &Dispatcher{
		logger:    logger,
		orders:    make(map[string]*cmd_center.MilitaryOrder),
		results:   make(map[string]*cmd_center.BattleReport),
		executors: make(map[string]Executor),
	}
}

// RegisterExecutor 注册执行器
func (d *Dispatcher) RegisterExecutor(name string, executor Executor) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.executors[name] = executor
}

// Submit 提交任务
func (d *Dispatcher) Submit(ctx context.Context, order *cmd_center.MilitaryOrder) (string, error) {
	if order.ID == "" {
		return "", fmt.Errorf("任务ID不能为空")
	}

	d.mu.Lock()
	d.orders[order.ID] = order
	observatory.SetActiveOrders(len(d.orders))
	d.mu.Unlock()

	d.logger.Info("任务已提交",
		zap.String("order_id", order.ID),
		zap.String("name", order.Name),
		zap.String("priority", order.Priority.String()),
	)

	// 根据策略类型选择执行器
	executorName := order.Strategy.Type
	if executorName == "" {
		executorName = "default"
	}

	d.mu.RLock()
	executor, exists := d.executors[executorName]
	d.mu.RUnlock()

	if !exists {
		// 使用默认执行器
		executor = &DefaultExecutor{}
	}

	// 异步执行
	go func() {
		start := time.Now()
		report, err := executor.Execute(ctx, order)

		result := &cmd_center.BattleReport{
			OrderID:     order.ID,
			StartedAt:   start,
			CompletedAt: time.Now(),
		}

		if err != nil {
			result.Success = false
			result.Message = fmt.Sprintf("执行失败: %v", err)
			observatory.RecordTaskProcessed("failed")
		} else {
			result.Success = report.Success
			result.Message = report.Message
			result.Data = report.Data
			result.Generals = report.Generals
			observatory.RecordTaskProcessed("success")
		}

		d.mu.Lock()
		d.results[order.ID] = result
		delete(d.orders, order.ID)
		observatory.SetActiveOrders(len(d.orders))
		d.mu.Unlock()

		d.logger.Info("任务执行完成",
			zap.String("order_id", order.ID),
			zap.Bool("success", result.Success),
			zap.Duration("duration", result.CompletedAt.Sub(start)),
		)
	}()

	return order.ID, nil
}

// GetStatus 获取任务状态
func (d *Dispatcher) GetStatus(orderID string) (*cmd_center.MilitaryOrder, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()
	order, exists := d.orders[orderID]
	if !exists {
		return nil, fmt.Errorf("任务不存在: %s", orderID)
	}
	return order, nil
}

// GetResult 获取任务结果
func (d *Dispatcher) GetResult(orderID string) (*cmd_center.BattleReport, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()
	result, exists := d.results[orderID]
	if !exists {
		return nil, fmt.Errorf("结果不存在: %s", orderID)
	}
	return result, nil
}

// ListPending 列出待处理任务
func (d *Dispatcher) ListPending() []*cmd_center.MilitaryOrder {
	d.mu.RLock()
	defer d.mu.RUnlock()
	orders := make([]*cmd_center.MilitaryOrder, 0, len(d.orders))
	for _, order := range d.orders {
		orders = append(orders, order)
	}
	return orders
}

// DefaultExecutor 默认执行器
type DefaultExecutor struct{}

func (e *DefaultExecutor) Execute(ctx context.Context, order *cmd_center.MilitaryOrder) (*cmd_center.BattleReport, error) {
	// 默认执行逻辑
	report := &cmd_center.BattleReport{
		OrderID: order.ID,
		Success: true,
		Message: "任务执行成功",
		Data:    map[string]interface{}{},
	}
	return report, nil
}
