// Package middleware 提供 HTTP 中间件。
//
// 设计原则：
//  1. 单一职责：每个中间件只做一件事（如 traceid 注入、panic 恢复、日志、CORS）。
//  2. 不依赖具体业务端口：中间件只使用 gin.Context + zap.Logger。
//  3. 配置外置：中间件无全局状态，便于测试与多实例部署。
package middleware

import (
	"github.com/gin-gonic/gin"

	"github.com/zhuge/kongming/pkg/infra/observability"
)

// HeaderTraceID 是 HTTP 头中 traceId 的字段名。
//
// 与 traceid.go 配套：客户端可主动传 X-Trace-Id 实现跨服务链路追踪；
// 不传则由本中间件生成新 UUID 兜底。
const HeaderTraceID = "X-Trace-Id"

// TraceID 注入/透传 traceId 中间件。
//
// 行为：
//  1. 读取 X-Trace-Id header；若为空则生成新的 UUID v4。
//  2. 注入到 c.Request.Context()（供下游 handler/服务使用）。
//  3. 回写 X-Trace-Id response header（便于客户端调试/排障）。
//
// 该中间件必须在 Recovery 之后、Logging 之前注册：Logging 需要从 ctx 读取 traceId。
func TraceID() gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.GetHeader(HeaderTraceID)
		if id == "" {
			id = observability.NewTraceID()
		}
		// 注入 ctx。
		c.Request = c.Request.WithContext(observability.NewTraceIDContext(c.Request.Context(), id))
		// 回写 response header。
		c.Header(HeaderTraceID, id)
		c.Next()
	}
}
