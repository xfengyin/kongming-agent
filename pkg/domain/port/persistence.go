// Package port 定义领域层对外暴露的「端口」（接口）。
//
// 本文件聚焦于「持久化」端口：OrderRepository 与 GeneralRepository。
// 这些接口由 application/commander、application/general 等用例通过依赖倒置
// 注入到业务逻辑中，由 infra/persistence（memory / 未来的 redis / db）实现。
//
// 设计原则（六边形架构）：
//  1. 接口最小化：只暴露用例真正需要的 4 个方法（CRUD + 按状态查询）。
//  2. ctx 透传：所有方法接收 context.Context，便于超时/取消/链路追踪。
//  3. 错误语义：返回 error 而非 bool，让实现者自由选择错误类型
//     （建议统一使用 pkg/domain/errors.CodeNotFound）。
//  4. Stage 2 兼容：参数类型 model.OrderID/State 已稳定，扩展 model 字段
//     不需要修改本接口（开闭原则）。
package port

import (
	"context"

	"github.com/zhuge/kongming/pkg/domain/model"
)

// OrderRepository 是 Order 聚合的持久化端口。
//
// 实现示例：pkg/infra/persistence/memory.OrderRepo。
// 调用方：pkg/application/commander、pkg/application/dispatcher。
type OrderRepository interface {
	// Save 将订单写入仓储。已存在则覆盖（upsert 语义）。
	// ctx 用于传递超时/取消/traceId。
	Save(ctx context.Context, o *model.Order) error

	// Get 按 ID 查询订单。返回 (nil, ErrNotFound) 表示不存在。
	Get(ctx context.Context, id model.OrderID) (*model.Order, error)

	// List 按状态过滤订单列表。
	// 当 state == model.StateNone（零值）时返回全量。
	// 顺序不保证稳定（实现可按 ID/时间排序）。
	List(ctx context.Context, state model.State) ([]*model.Order, error)

	// Delete 按 ID 删除订单。不存在时不应返回错误（幂等）。
	Delete(ctx context.Context, id model.OrderID) error
}

// GeneralRepository 是 General 聚合的持久化端口。
//
// 实现示例：pkg/infra/persistence/memory.GeneralRepo。
// 调用方：pkg/application/general。
type GeneralRepository interface {
	// List 返回全量将领列表。
	List(ctx context.Context) ([]*model.General, error)

	// Get 按 ID 查询单个将领。
	Get(ctx context.Context, id model.GeneralID) (*model.General, error)

	// Save 写入或更新一个将领。
	Save(ctx context.Context, g *model.General) error
}
