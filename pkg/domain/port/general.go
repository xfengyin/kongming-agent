// Package port 端口定义 - 将领池（GeneralPool）。
//
// 设计原则（六边形架构）：
//  1. 接口最小化：仅暴露 6 个用例方法（CRUD + 选将 + 执行）。
//  2. ctx 透传：所有方法（SelectBest 除外，无 IO）接收 context.Context，
//     便于超时/取消/链路追踪。
//  3. 返回 error 而非 bool：让实现者自由包装为领域错误。
//  4. 不暴露实现类型：参数/返回值只使用 model.* 类型。
//
// 与 port.GeneralRepository 的区别：
//   - Repository（infra/persistence/memory.GeneralRepo）：纯 CRUD 持久化抽象。
//   - GeneralPool（本接口）：业务用例抽象，包含选将 / 派单 / 弹性执行等能力。
//     二者解耦：Pool 可独立使用 Registry 缓存，Repository 仅作为冷启动数据源。
package port

import (
	"context"

	"github.com/zhuge/kongming/pkg/domain/model"
)

// GeneralPool 是将领池（Five Tiger Generals Pool）的应用层端口。
//
// 实现示例：pkg/application/general.Pool。
// 调用方：pkg/application/commander、pkg/transport/http/handler。
//
// 业务语义：
//   - 池中每个 General 拥有独立 Stats（任务总数/成功/失败/平均响应时间）。
//   - SelectBest 按"成功数降序"选择最稳的将，简单且可解释。
//   - Execute 走 ResilientRunner 包装（重试/熔断/限流/超时），返回
//     GeneralReport 聚合（含 Success/Output/Error/Duration）。
type GeneralPool interface {
	// Get 按 ID 查询单个将领。返回 (nil, err) 表示不存在或参数非法。
	Get(ctx context.Context, id model.GeneralID) (*model.General, error)

	// List 返回全量将领快照。返回的切片是新分配的，调用方修改不影响池内数据。
	// 顺序不保证稳定。
	List(ctx context.Context) ([]*model.General, error)

	// Register 注册一个将领到池中。已存在则覆盖（upsert 语义）。
	// 每次成功注册会上报 general_registered 指标。
	Register(ctx context.Context, g *model.General) error

	// Unregister 从池中注销一个将领。不存在时不应返回错误（幂等）。
	Unregister(ctx context.Context, id model.GeneralID) error

	// SelectBest 在池中按 skill 标签挑选最佳将领。
	// 当前实现：按 Stats.SuccessCount 降序选第一。
	// 找不到具备该 skill 的将领时返回错误。
	SelectBest(skill string) (*model.General, error)

	// Execute 由指定 ID 的将领执行一个 Order。
	// 内部走 ResilientRunner 包装，提供重试/熔断/限流/超时能力；
	// 即便执行失败也会返回非 nil 的 *GeneralReport，error 字段会被填充。
	Execute(ctx context.Context, id model.GeneralID, o *model.Order) (*model.GeneralReport, error)
}
