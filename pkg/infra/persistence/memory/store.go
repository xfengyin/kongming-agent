// Package memory 提供基于进程内 sync.Map 的轻量级仓储实现。
//
// 本子包是 pkg/infra/persistence 唯一的 Stage 1 实现，专为本地开发、单进程
// 部署与单元测试设计。后续 Stage 会在同级目录新增 redis、postgres 实现，
// 共享相同的 port.OrderRepository / port.GeneralRepository 接口。
//
// Store 是共享存储：OrderRepo、GeneralRepo 各自仅持有一个 *Store 引用，
// 通过 Store 内的两个 sync.Map 协作。这样多 Repo 之间可以原子地观察同一份
// 内存数据（例如 commander 写入 Order 后 general 模块立刻能查到对应将领）。
package memory

import (
	"sync"
)

// Store 是 OrderRepo / GeneralRepo 共享的进程内存储。
//
// 并发安全由 sync.Map 自身保证；Store 本身无状态，零值即可用。
// 未来需要分片/容量上限/LRU 等特性时，扩展 Store 结构体（开闭原则）。
type Store struct {
	// orders 存储 Order 聚合：key=string(OrderID), value=*model.Order。
	orders sync.Map
	// generals 存储 General 聚合：key=string(GeneralID), value=*model.General。
	generals sync.Map
}

// NewStore 构造一个空的 Store。
// 不持有任何锁，开箱即用。
func NewStore() *Store {
	return &Store{}
}
