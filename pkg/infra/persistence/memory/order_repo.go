// OrderRepo 是 port.OrderRepository 的内存实现。
//
// 设计要点：
//   - 依赖倒置：仅依赖 *Store（聚合根容器），不直接 import application/transport。
//   - 单一职责：只做 Order 持久化，不持有任何业务状态。
//   - 并发安全：所有读写经过 sync.Map，无显式加锁。
//   - 幂等：Save 覆盖、Delete 对不存在的 key 无副作用。
//   - 可观测：errors 携带资源 ID，便于上游打日志/上报。
package memory

import (
	"context"
	"errors"
	"fmt"

	"github.com/zhuge/kongming/pkg/domain/model"
	"github.com/zhuge/kongming/pkg/domain/port"
)

// ErrOrderNotFound 在 Get 找不到订单时返回，调用方可用 errors.Is 判定。
// 使用独立 sentinel 而非 errors.New(fmt.Sprintf(...))，以便 errors.Is 工作。
var ErrOrderNotFound = errors.New("order not found")

// OrderRepo 实现 port.OrderRepository。
type OrderRepo struct {
	// s 是共享存储引用，可与 GeneralRepo 共享同一 Store 实例。
	s *Store
}

// NewOrderRepo 构造一个 OrderRepo，绑定到给定的 Store。
// 共享 Store 让多 Repo 间数据原子可见（读已写立即可见）。
func NewOrderRepo(s *Store) *OrderRepo {
	if s == nil {
		// 防御性 panic：传入 nil Store 是程序员错误。
		panic("memory: NewOrderRepo requires non-nil Store")
	}
	return &OrderRepo{s: s}
}

// Save 将订单写入共享 Store；已存在则覆盖（upsert 语义）。
// 当前实现忽略 ctx（纯内存无 IO），但保留参数以满足接口签名，便于后续
// 接入 redis 等需要 ctx 的实现时上层无感切换。
func (r *OrderRepo) Save(_ context.Context, o *model.Order) error {
	if o == nil {
		return errors.New("order is nil")
	}
	if o.ID == "" {
		return errors.New("order.ID is required")
	}
	r.s.orders.Store(o.ID, o)
	return nil
}

// Get 按 ID 查询订单。命中时返回 *model.Order；未命中返回 (nil, ErrOrderNotFound)。
// errors.Is(err, ErrOrderNotFound) 可用于调用方判定。
func (r *OrderRepo) Get(_ context.Context, id model.OrderID) (*model.Order, error) {
	if id == "" {
		return nil, fmt.Errorf("order id is empty")
	}
	if v, ok := r.s.orders.Load(id); ok {
		// 类型断言失败说明 store 中存在异常类型，属于严重 bug，向上抛 panic。
		return v.(*model.Order), nil
	}
	return nil, fmt.Errorf("%w: %s", ErrOrderNotFound, id)
}

// List 按 state 过滤订单。state==StateNone 时返回全量。
// 返回的切片为新分配，调用方可以安全修改；底层对象仍共享。
func (r *OrderRepo) List(_ context.Context, state model.State) ([]*model.Order, error) {
	var out []*model.Order
	r.s.orders.Range(func(_, v any) bool {
		o, ok := v.(*model.Order)
		if !ok {
			return true // 跳过非法条目，继续遍历
		}
		// state==StateNone 表示「不过滤」，等价于 0 值。
		if state == model.StateNone || o.State == state {
			out = append(out, o)
		}
		return true
	})
	return out, nil
}

// Delete 按 ID 删除订单。对不存在的 key 不返回错误（幂等）。
// 与 Get 不同，Delete 不需要返回 ErrNotFound —— 业务上「删除一个不存在的
// 资源」应当被视作成功（参考 S3 DeleteObject 语义）。
func (r *OrderRepo) Delete(_ context.Context, id model.OrderID) error {
	if id == "" {
		return errors.New("order id is empty")
	}
	r.s.orders.Delete(id)
	return nil
}

// 编译期断言：OrderRepo 必须实现 port.OrderRepository。
// 任何方法签名变动都会在编译期被捕获，避免运行时静默失败。
var _ port.OrderRepository = (*OrderRepo)(nil)
