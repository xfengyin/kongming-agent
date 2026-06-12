// Package port 定义领域层端口接口（端口适配器架构）。
//
// ResilientRunner 抽象"带弹性（重试/熔断/限流/超时）的执行器"，
// 由 infra/resilience 提供实现。application 层只依赖此接口，
// 不感知底层装饰器链顺序，便于替换为不同弹性策略。
package port

import "context"

// ResilientRunner 提供"带弹性的执行"能力。
//
// 设计要点：
//   - 装饰顺序由实现方决定；推荐：timeout → ratelimit → circuitbreaker → retry → fn
//   - 名称 (name) 用于日志/metrics 关联，禁止为空
//   - 内部使用 traceId/span 串联；错误透传由实现包装
type ResilientRunner interface {
	// Run 执行无返回值的 fn。
	// fn 返回 nil 即视为成功；返回 error 即触发重试与熔断统计。
	Run(ctx context.Context, name string, fn func(ctx context.Context) error) error

	// RunWithResult 执行带返回值的 fn。
	// 成功时返回 fn 的结果；失败时返回最后一次的 error。
	RunWithResult(ctx context.Context, name string, fn func(ctx context.Context) (any, error)) (any, error)
}
