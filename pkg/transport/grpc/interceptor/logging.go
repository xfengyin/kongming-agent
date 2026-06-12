package interceptor

import (
	"context"
	"time"

	"go.uber.org/zap"
	"google.golang.org/grpc"

	"github.com/zhuge/kongming/pkg/infra/observability"
)

// Logging 返回一个 UnaryServerInterceptor：
//   - 记录 method / duration / traceId / code / error；
//   - 仅在出现错误时记录 error 字段（避免日志噪音）。
//
// 设计取舍：
//  1. 使用 zap 而非 stdlib log，便于结构化字段（zap.String/zap.Duration）；
//  2. 不记录 req/resp 完整 payload：避免敏感字段（APIKey/Command）泄漏；
//     需要时由业务方在 handler 内部显式 Debug 打印。
//  3. duration 精度到毫秒（time.Since 单位为 ns），便于监控。
func Logging(logger *zap.Logger) grpc.UnaryServerInterceptor {
	// 防御：logger 为 nil 时降级为 noop logger（zap.NewNop()），避免 nil deref。
	if logger == nil {
		logger = zap.NewNop()
	}
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		start := time.Now()
		resp, err := handler(ctx, req)
		// 只在出错时记 error 字段（结构化日志友好）。
		fields := []zap.Field{
			zap.String("method", info.FullMethod),
			zap.Duration("duration", time.Since(start)),
			zap.String("trace_id", observability.FromTraceIDContext(ctx)),
		}
		if err != nil {
			fields = append(fields, zap.Error(err))
			logger.Error("grpc call failed", fields...)
			return resp, err
		}
		logger.Info("grpc call ok", fields...)
		return resp, nil
	}
}
