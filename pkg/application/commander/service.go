// Package commander 提供军师应用层用例。
//
// 本文件实现 port.Commander —— 军师用例的应用层编排服务。
//
// 职责（按 spec §3.2）：
//  1. 派单前幂等检查（OrderRepository.Get），命中则走 replayReport；
//  2. 驱动 Order 状态机：StatePending → StatePlanning → StateExecuting → StateCompleted
//     或降级为 StateFailed；
//  3. 委托 Planner 生成 Strategy；
//  4. 委托 GeneralPool 选将 + Execute（无将领时 skip 当条 tactic，runTactics 仍返回 Success=true）；
//  5. 走 ResilientRunner 包裹执行，失败时落 StateFailed；
//  6. 调用 Review（失败仅记录日志，不阻断派单结果）；
//  7. 持久化完成态。
//
// 设计要点：
//   - 依赖倒置：仅依赖 port.Commander / Planner 等接口，不感知具体实现；
//   - engine / vault 字段在当前派单主流程中未直接使用，但保留以备 Stage 4+
//     引入「工作流驱动派单」时按需启用（开闭原则）；
//   - 错误透传：业务错误用 domerrs.Wrap 附加 Code，状态机迁移错误保留根因便于上游定位。
package commander

import (
	"context"
	"fmt"
	"time"

	domerrs "github.com/zhuge/kongming/pkg/domain/errors"
	"github.com/zhuge/kongming/pkg/domain/model"
	"github.com/zhuge/kongming/pkg/domain/port"
	"go.opentelemetry.io/otel/attribute"
	"go.uber.org/zap"
)

// Service 是 port.Commander 的应用层实现。
//
// 字段分组：
//   - 业务依赖：planner / pool / engine / vault / orders
//   - 横切依赖：resilient（弹性执行）/ observer（可观测）/ logger（日志）
//   - 配置：cfg.DefaultTimeout 保留以备未来按 Order 维度设置超时
type Service struct {
	cfg       struct{ DefaultTimeout time.Duration }
	planner   Planner
	pool      port.GeneralPool
	engine    port.Engine
	vault     port.Vault
	orders    port.OrderRepository
	resilient port.ResilientRunner
	observer  port.Observer
	logger    *zap.Logger
}

// New 构造一个 Service，注入所有依赖。
//
// 依赖注入约定：调用方（kongming 顶层装配器）必须保证非 nil；本函数不进行
// nil-check，由 transport 层在装配期 fail-fast 校验。
func New(
	planner Planner,
	pool port.GeneralPool,
	engine port.Engine,
	vault port.Vault,
	orders port.OrderRepository,
	resilient port.ResilientRunner,
	observer port.Observer,
	logger *zap.Logger,
) *Service {
	return &Service{
		planner:   planner,
		pool:      pool,
		engine:    engine,
		vault:     vault,
		orders:    orders,
		resilient: resilient,
		observer:  observer,
		logger:    logger,
	}
}

// Dispatch 派单并执行一次完整 Order 生命周期。
//
// 流程：幂等检查 → 状态机迁移 → 战略规划 → 持久化 → 弹性执行 → 审核 → 完成。
// 错误码语义：见 port.Commander.Dispatch 文档。
func (s *Service) Dispatch(ctx context.Context, order *model.Order) (*model.BattleReport, error) {
	ctx, span := s.observer.StartSpan(ctx, "commander.Dispatch",
		attribute.String("order.id", string(order.ID)))
	defer span.End()

	// 幂等检查：已存在则直接走 replay 路径，不再触发副作用。
	if existing, _ := s.orders.Get(ctx, order.ID); existing != nil {
		s.logger.Info("idempotent replay", zap.String("order_id", string(order.ID)))
		return s.replayReport(ctx, existing)
	}

	// 状态机：StatePending → StatePlanning。
	// 注：TransitionTo 当前是值接收器（仅做合法性校验），因此需手动赋值新状态。
	if err := order.State.TransitionTo(model.StatePlanning); err != nil {
		return nil, domerrs.Wrap(domerrs.INVALID_STATE, err)
	}
	order.State = model.StatePlanning
	order.UpdatedAt = time.Now()

	// 战略规划
	strategy, err := s.planner.Plan(ctx, order)
	if err != nil {
		s.observer.RecordError(span, err)
		return nil, domerrs.Wrap(domerrs.STRATEGY_FAILED, err)
	}
	order.Strategy = *strategy

	// 状态机：StatePlanning → StateExecuting
	if err := order.State.TransitionTo(model.StateExecuting); err != nil {
		return nil, domerrs.Wrap(domerrs.INVALID_STATE, err)
	}
	order.State = model.StateExecuting
	if err := s.orders.Save(ctx, order); err != nil {
		return nil, domerrs.Wrap(domerrs.PERSIST_FAILED, err)
	}

	// 弹性执行：runTactics 内已对单 tactic 失败做 continue，整体成功即视为完成。
	var report *model.BattleReport
	err = s.resilient.Run(ctx, "commander.dispatch", func(ctx context.Context) error {
		var rerr error
		report, rerr = s.runTactics(ctx, order, strategy)
		return rerr
	})
	if err != nil {
		// 整体失败：直接落 StateFailed（不经过状态机迁移，因 StateFailed 是终态之外的
		// 任意态可降级目标，且 TransitionTo 链路已用尽）。
		order.State = model.StateFailed
		_ = s.orders.Save(ctx, order)
		return nil, err
	}

	// 审核：失败仅记录日志，不阻断派单结果（按 port.Commander.Review 契约）。
	if err := s.Review(ctx, report); err != nil {
		s.logger.Warn("review failed", zap.Error(err))
	}

	// 状态机：StateExecuting → StateReviewing → StateCompleted。
	// 状态机强制要求必须经过 Reviewing；Review 失败不阻断 StateFailed 之外的状态推进。
	if err := order.State.TransitionTo(model.StateReviewing); err != nil {
		return nil, domerrs.Wrap(domerrs.INVALID_STATE, err)
	}
	order.State = model.StateReviewing
	if err := order.State.TransitionTo(model.StateCompleted); err != nil {
		return nil, domerrs.Wrap(domerrs.INVALID_STATE, err)
	}
	order.State = model.StateCompleted
	_ = s.orders.Save(ctx, order)
	return report, nil
}

// runTactics 按 strategy.Tactics 顺序选将并执行，生成 BattleReport。
//
// 单条 tactic 失败（无将/Execute error）的处理：continue 跳过、记日志，**不**把
// report.Success 置为 false。这样上层可以将「部分将不可用」与「整体执行失败」
// 区分开来，遵循「单点失败不污染整体」的高可用原则。
func (s *Service) runTactics(
	ctx context.Context,
	order *model.Order,
	strategy *model.Strategy,
) (*model.BattleReport, error) {
	report := &model.BattleReport{
		OrderID:   order.ID,
		StartedAt: time.Now(),
	}
	for _, tactic := range strategy.Tactics {
		general, err := s.pool.SelectBest(tactic.Action)
		if err != nil {
			s.logger.Warn("no general for tactic",
				zap.String("tactic", tactic.Name),
				zap.String("action", tactic.Action))
			continue
		}
		// 派单子单：复用 order.Context，便于 traceId 透传到子执行链路。
		sub := &model.Order{
			ID:      model.OrderID(string(order.ID) + "_" + tactic.Name),
			Name:    tactic.Name,
			Context: order.Context,
		}
		gr, err := s.pool.Execute(ctx, general.ID, sub)
		if err != nil {
			s.logger.Error("general exec failed",
				zap.String("general", string(general.ID)),
				zap.String("name", general.Name),
				zap.Error(err))
			continue
		}
		report.Generals = append(report.Generals, *gr)
	}
	report.CompletedAt = time.Now()
	report.Success = true
	return report, nil
}

// PlanStrategy 单独使用战略规划能力（不执行战术）。
func (s *Service) PlanStrategy(ctx context.Context, order *model.Order) (*model.Strategy, error) {
	return s.planner.Plan(ctx, order)
}

// Review 审核战报：失败时返回 error（含失败原因），成功时返回 nil。
//
// 失败不阻断派单结果（Dispatch 内部对 Review 失败仅记录日志）。
// 当前实现：成功时把每位成功将领记 INFO 日志；失败时返回 fmt.Errorf（含 OrderID）。
func (s *Service) Review(_ context.Context, report *model.BattleReport) error {
	if report == nil {
		return fmt.Errorf("review: report is nil")
	}
	if !report.Success {
		return fmt.Errorf("report failed: order=%s", report.OrderID)
	}
	for _, gr := range report.Generals {
		if gr.Success {
			s.logger.Info("general succeeded",
				zap.String("general_id", string(gr.GeneralID)),
				zap.String("name", gr.Name))
		}
	}
	return nil
}

// GetOrder 按 ID 查询订单，原样透传 OrderRepository.Get。
func (s *Service) GetOrder(ctx context.Context, id model.OrderID) (*model.Order, error) {
	return s.orders.Get(ctx, id)
}

// ListOrders 按状态过滤订单列表，原样透传 OrderRepository.List。
func (s *Service) ListOrders(ctx context.Context, state model.State) ([]*model.Order, error) {
	return s.orders.List(ctx, state)
}

// replayReport 幂等路径：基于已持久化 Order 重建 BattleReport。
//
// 语义：
//   - 已有 Order 且 State==StateCompleted → 构造一份「复用」战报（Success=true）；
//   - 已有 Order 但 State 非 Completed → 拒绝 replay，返回 INVALID_STATE 错误；
//   - 理论上调用方在 Get 命中 nil 时不会进入本方法；防御性 nil check 仍保留。
func (s *Service) replayReport(_ context.Context, existing *model.Order) (*model.BattleReport, error) {
	if existing == nil {
		return nil, domerrs.New(domerrs.NOT_FOUND, "no order to replay")
	}
	if existing.State != model.StateCompleted {
		return nil, domerrs.New(domerrs.INVALID_STATE,
			"idempotent replay requires StateCompleted order")
	}
	// 复用窗口：把幂等标记塞入 Result 便于调用方 / 测试判定。
	startedAt := existing.CreatedAt
	completedAt := existing.UpdatedAt
	if completedAt.Before(startedAt) {
		completedAt = startedAt
	}
	return &model.BattleReport{
		OrderID:     existing.ID,
		Success:     true,
		StartedAt:   startedAt,
		CompletedAt: completedAt,
		Result:      map[string]any{"message": "idempotent replay"},
	}, nil
}

// 编译期断言：Service 必须实现 port.Commander。
var _ port.Commander = (*Service)(nil)
