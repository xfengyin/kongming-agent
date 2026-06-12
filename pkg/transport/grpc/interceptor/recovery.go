package interceptor

import (
	"context"
	"runtime/debug"

	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// Recovery 返回一个 UnaryServerInterceptor：
//   - 捕获 handler 中的 panic，防止单个 RPC 拖垮整个 gRPC server；
//   - 记录 stack trace 到 logger；
//   - 返回 codes.Internal 给调用方，避免泄漏内部细节。
//
// 设计取舍：
//  1. 必须放在 Chain 的最内层（紧贴 handler）才能保证 panic 必然被捕获；
//  2. 总是返回 Internal 而非原始 panic 值，避免泄漏栈/敏感字段。
//  3. debug.Stack() 仅在 panic 时调用，正常路径零开销。
func Recovery(logger *zap.Logger) grpc.UnaryServerInterceptor {
	if logger == nil {
		logger = zap.NewNop()
	}
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (resp any, err error) {
		// 延迟兜底：recover 必须在 defer 中。
		defer func() {
			if r := recover(); r != nil {
				logger.Error("grpc handler panic",
					zap.String("method", info.FullMethod),
					zap.Any("panic", r),
					zap.String("stack", string(debug.Stack())),
				)
				err = status.Error(codes.Internal, "internal server error")
				resp = nil
			}
		}()
		return handler(ctx, req)
	}
}
