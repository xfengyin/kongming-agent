// Package interceptor 提供 gRPC 服务端拦截器与组装工具。
//
// 设计原则：
//  1. 单一职责：每个拦截器只做一件事（traceId 注入 / 日志 / panic 兜底）；
//  2. 装饰器风格：拦截器对 handler 透明，调用方按需链式组合；
//  3. 零业务依赖：拦截器只读 ctx / metadata / info，不直接调用任何 port；
//  4. 可测试：拦截器内部不持有外部状态，便于单测。
//
// 典型用法（pkg/transport/grpc/server.go）：
//
//	grpc.NewServer(
//	    grpc.UnaryInterceptor(interceptor.Chain(
//	        interceptor.TraceID(),
//	        interceptor.Logging(logger),
//	        interceptor.Recovery(logger),
//	    )),
//	)
package interceptor

import (
	"context"

	"google.golang.org/grpc"
)

// Chain 把多个 UnaryServerInterceptor 串成一个。
//
// 执行顺序：第一个拦截器最外层，最后一个最内层（紧贴 handler）。
// 这与标准 gRPC chained unary interceptor 的语义一致，便于日志/traceId
// 在最外层先生成，Recovery 在最内层兜底。
//
// 入参非法：传入 nil 或空切片时返回 noop 拦截器（直接调用 handler），
// 避免上游必须做防御性检查。
func Chain(unary ...grpc.UnaryServerInterceptor) grpc.UnaryServerInterceptor {
	// 过滤掉 nil，避免运行时 panic。
	chain := make([]grpc.UnaryServerInterceptor, 0, len(unary))
	for _, u := range unary {
		if u != nil {
			chain = append(chain, u)
		}
	}
	switch len(chain) {
	case 0:
		return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
			return handler(ctx, req)
		}
	case 1:
		return chain[0]
	}
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		// 递归构造调用链：最右侧（chain[len-1]）紧贴 handler。
		return chain[0](ctx, req, info, buildChain(chain[1:], info, handler))
	}
}

// buildChain 把剩余 chain 包装成一个虚拟 handler，与 Chain 配套使用。
func buildChain(
	chain []grpc.UnaryServerInterceptor,
	info *grpc.UnaryServerInfo,
	final grpc.UnaryHandler,
) grpc.UnaryHandler {
	if len(chain) == 0 {
		return final
	}
	return func(ctx context.Context, req any) (any, error) {
		return chain[0](ctx, req, info, buildChain(chain[1:], info, final))
	}
}
