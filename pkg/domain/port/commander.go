// Package port 定义领域层对外暴露的「端口」（接口）。
//
// 本文件定义 Commander 端口 —— 军师用例的对外契约。
// 包含 5 个用例方法：Dispatch/PlanStrategy/Review/GetOrder/ListOrders，
// 覆盖「军师派单 → 制定战略 → 审核战报 → 查询订单」完整链路。
//
// 设计原则（六边形架构 + 接口隔离）：
//  1. 依赖倒置：application/commander.Service 实现本接口，transport 通过本接口调用业务；
//  2. 单一职责：Commander 只负责「订单派单 + 战报审核」，不涉及具体执行（执行下沉到
//     Engine/GeneralPool/Vault 等其他端口）；
//  3. 接口最小化：5 个方法覆盖完整用例域，避免污染；
//  4. ctx 透传：所有方法接收 context.Context，便于超时/取消/链路追踪。
package port

import (
	"context"

	"github.com/zhuge/kongming/pkg/domain/model"
)

// Commander 是军师用例的端口接口。
//
// 实现方：pkg/application/commander.Service。
// 调用方：pkg/transport/{http,grpc,cli}、kongming.Kongming 顶层装配器。
//
// 幂等性约束：Dispatch(order) 当 order.ID 已被处理过（OrderRepository.Get 返回非 nil）
// 时，调用方应期待 Service 返回与首次执行一致的 BattleReport，不应重复执行。
// 具体幂等策略由 Service.replayReport 决定，本接口不强制语义。
type Commander interface {
	// Dispatch 派单并执行一次完整 Order 生命周期。
	//
	// 流程（伪代码）：
	//  1. 幂等检查：若 orders.Get(order.ID) 命中则直接返回缓存战报；
	//  2. 状态机：StatePending → StatePlanning → StateExecuting → StateCompleted；
	//  3. 制定战略：planner.Plan → *model.Strategy；
	//  4. 入库 + 弹性执行：resilient.Run 包裹 runTactics；
	//  5. 审核 + 持久化完成态。
	//
	// 错误语义：失败时返回的 *Error.Code 必为以下之一：
	//   - INVALID_STATE    状态机非法迁移
	//   - STRATEGY_FAILED  战略规划失败
	//   - PERSIST_FAILED   订单持久化失败
	//   - INTERNAL         兜底未预期错误
	Dispatch(ctx context.Context, order *model.Order) (*model.BattleReport, error)

	// PlanStrategy 单独使用战略规划能力（不执行战术）。
	//
	// 适用场景：用户在派单前预览战略、Reviewer 评估派单方案。
	PlanStrategy(ctx context.Context, order *model.Order) (*model.Strategy, error)

	// Review 审核战报：失败时返回 error（含失败原因），成功时返回 nil。
	//
	// 失败不阻断派单结果（Dispatch 内部对 Review 失败仅记录日志），
	// 但调用方可主动 Review 以实现人工/自动审核工作流。
	Review(ctx context.Context, report *model.BattleReport) error

	// GetOrder 按 ID 查询订单。订单不存在时返回 (nil, *errors.Error{Code: NOT_FOUND})。
	GetOrder(ctx context.Context, id model.OrderID) (*model.Order, error)

	// ListOrders 按状态过滤订单列表。
	// state == model.StateNone（零值）时返回全量；顺序不保证稳定。
	ListOrders(ctx context.Context, state model.State) ([]*model.Order, error)
}

// 编译期断言：任何实现 Commander 接口的类型都应满足本契约。
// 此行不在接口中暴露，避免污染 godoc。
var _ Commander = (Commander)(nil)
