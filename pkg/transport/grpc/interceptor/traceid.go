package interceptor

import (
	"context"

	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"

	"github.com/zhuge/kongming/pkg/infra/observability"
)

// TraceIDKey metadata 中 traceId 的键名。
//
// 约定：与 HTTP transport 复用同一键 "x-trace-id"，便于跨协议链路追踪。
const TraceIDKey = "x-trace-id"

// TraceID 返回一个 UnaryServerInterceptor：
//   - 优先从 incoming metadata 中读取 x-trace-id；
//   - 若不存在，生成 UUID v4 作为新 traceId；
//   - 把 traceId 写入 ctx 与 outgoing metadata（追加响应头，便于客户端抓取）。
//
// 注入 ctx 后，下游业务可通过 observability.FromTraceIDContext(ctx) 读取。
func TraceID() grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		md, _ := metadata.FromIncomingContext(ctx)
		var traceID string
		if vals := md.Get(TraceIDKey); len(vals) > 0 {
			traceID = vals[0]
		}
		if traceID == "" {
			traceID = observability.NewTraceID()
		}
		// 注入到 ctx，供业务层读取。
		ctx = observability.NewTraceIDContext(ctx, traceID)
		// 透传到 outgoing metadata，便于客户端拿到本次请求的 traceId。
		_ = metadata.AppendToOutgoingContext(ctx, TraceIDKey, traceID)
		return handler(ctx, req)
	}
}
