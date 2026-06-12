// Package plugin 提供 SPI（Service Provider Interface）注册中心与 .so/.yaml 插件热更新能力。
//
// 设计原则：
//   - 开闭原则：新增插件只新增 Handler 实现 + Register，不修改主流程。
//   - 依赖倒置：依赖 Handler 抽象接口而非具体实现。
//   - 单一职责：本包只负责插件元数据注册、文件监听、占位 loader；具体业务注册由 application 层注入。
//
// 本文件仅实现注册中心骨架，热更新触发后的具体业务逻辑由 application 层（vault、workflow 等）注入。
package plugin

import (
	"errors"
	"fmt"
	"sort"
	"sync"
)

// Handler 是所有插件必须实现的 SPI 契约。
// 当前仅要求暴露 Name()，业务侧可按需扩展。
type Handler interface {
	// Name 返回插件唯一标识；同名重复注册后者覆盖前者。
	Name() string
}

// ErrInvalidHandler 当 Handler 为 nil 或 Name() 返回空字符串时返回。
var ErrInvalidHandler = errors.New("plugin: invalid handler (nil or empty name)")

// ErrNotFound 当 Get 一个未注册名称时返回。
var ErrNotFound = errors.New("plugin: handler not found")

// Registry 是线程安全的 Handler 注册中心。
// 内部使用 sync.RWMutex 保护 map，Get/List 走读锁，Register/Unregister 走写锁。
type Registry struct {
	// mu 保护 handlers map。
	mu sync.RWMutex
	// handlers 以 handler 名称为 key。
	handlers map[string]Handler
}

// NewRegistry 构造一个空的注册中心。
func NewRegistry() *Registry {
	return &Registry{handlers: make(map[string]Handler)}
}

// Register 注册一个 Handler。nil handler 或空 name 返回 ErrInvalidHandler。
// 同名 handler 重复注册按"后者覆盖前者"语义处理，调用方可据此实现插件热替换。
func (r *Registry) Register(h Handler) error {
	if h == nil || h.Name() == "" {
		return fmt.Errorf("%w: name=%q", ErrInvalidHandler, nameOrEmpty(h))
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.handlers[h.Name()] = h
	return nil
}

// MustRegister 是 Register 的 panic 包装，常用于 init() 期注册。
// 注册失败直接 panic，避免漏检。
func (r *Registry) MustRegister(h Handler) {
	if err := r.Register(h); err != nil {
		panic(err)
	}
}

// Unregister 移除指定名称的 Handler；若不存在则静默忽略。
func (r *Registry) Unregister(name string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.handlers, name)
}

// Get 按名称查找 Handler；未找到时返回 (nil, false)。
func (r *Registry) Get(name string) (Handler, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	h, ok := r.handlers[name]
	return h, ok
}

// List 返回当前已注册 Handler 名称的有序切片（按字典序）。
// 返回拷贝以避免外部修改影响内部状态。
func (r *Registry) List() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]string, 0, len(r.handlers))
	for n := range r.handlers {
		out = append(out, n)
	}
	sort.Strings(out)
	return out
}

// Len 返回已注册 Handler 数量（便于测试与可观测性）。
func (r *Registry) Len() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.handlers)
}

// nameOrEmpty 安全地从 Handler 抽取 Name，避免 nil 解引用。
func nameOrEmpty(h Handler) string {
	if h == nil {
		return ""
	}
	return h.Name()
}
