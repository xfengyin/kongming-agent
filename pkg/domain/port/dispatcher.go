// Package port 定义领域层对外暴露的端口（接口）契约。
//
// 本文件定义「调度器」（Dispatcher）的端口契约：异步将 Order 路由到对应 Executor。
// 实现位于 pkg/application/dispatcher，遵循依赖倒置原则。
package port

import (
	"context"

	"github.com/zhuge/kongming/pkg/domain/model"
)

// Executor 是执行器子接口，用于 Dispatcher 把 Order 真正「落地」执行。
//
// 不同 Executor 可代表不同优先级/不同战术/不同 backend；
// 注册时用 name 索引，Dispatch 时按 name 查找。
type Executor interface {
	// Execute 执行一个 Order 并返回 BattleReport 或 error。
	// ctx 用于取消/超时，调用方应保证 ctx 派生自 Dispatch 的入参 ctx。
	Execute(ctx context.Context, order *model.Order) (*model.BattleReport, error)
}

// Dispatcher 是「调度器」应用层端口，抽象出 3 个能力。
//
//  设计要点：
//   - 异步派发：Dispatch 不阻塞，order 进入 worker pool 后立刻返回；
//   - 优先级路由：按 order.Priority 找名为 "priority-<value>" 的 executor；
//   - 优雅关闭：Wait 等待所有 in-flight 任务完成。
type Dispatcher interface {
	// Dispatch 把 order 投递到 worker pool，异步执行。
	// 不返回执行结果（执行结果由 Executor 自行处理/上报）；
	// 返回 nil 仅表示「投递成功」。
	Dispatch(ctx context.Context, order *model.Order) error

	// RegisterExecutor 注册一个 executor 到指定 name。
	// 同一 name 重复注册会覆盖（最新生效）。
	RegisterExecutor(name string, exec Executor)

	// Wait 阻塞直到所有 in-flight 任务完成，或 ctx 取消/超时。
	// 通常在 Stop 流程中调用，确保无任务被截断。
	Wait(ctx context.Context) error
}
