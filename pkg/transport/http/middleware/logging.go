package middleware

import (
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"github.com/zhuge/kongming/pkg/infra/observability"
)

// Logging 请求访问日志中间件。
//
// 输出格式：method / path / status / duration_ms / traceId / client_ip。
//
// 设计要点：
//  1. 必须在 TraceID 之后注册（依赖 ctx 中的 traceId）。
//  2. 用 c.Next() 之后取 status/duration，避免在 defer 中误读。
//  3. 异常路径不写 ERROR 级别（由 Recovery 单独记录），统一按 status code 分级。
func Logging(logger *zap.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		path := c.Request.URL.Path
		raw := c.Request.URL.RawQuery

		// 业务处理。
		c.Next()

		// 收尾日志。
		duration := time.Since(start)
		traceID := observability.FromTraceIDContext(c.Request.Context())
		status := c.Writer.Status()

		fields := []zap.Field{
			zap.String("method", c.Request.Method),
			zap.String("path", path),
			zap.String("query", raw),
			zap.Int("status", status),
			zap.Duration("duration", duration),
			zap.String("trace_id", traceID),
			zap.String("client_ip", c.ClientIP()),
		}
		if logger == nil {
			return
		}
		switch {
		case status >= 500:
			logger.Error("http request", fields...)
		case status >= 400:
			logger.Warn("http request", fields...)
		default:
			logger.Info("http request", fields...)
		}
	}
}
