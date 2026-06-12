package observability

import (
	"context"

	"github.com/google/uuid"
)

// traceIDKey 是 context 中 traceId 的私有 key 类型，避免外部包污染。
type traceIDKey struct{}

// NewTraceIDContext 把 traceId 注入到 ctx 中，后续可通过 FromTraceIDContext 读取。
func NewTraceIDContext(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, traceIDKey{}, id)
}

// FromTraceIDContext 读取 ctx 中的 traceId；不存在时返回空字符串（不报错）。
// 这样调用方可以在不确定 ctx 是否已注入 traceId 时安全调用。
func FromTraceIDContext(ctx context.Context) string {
	if v, ok := ctx.Value(traceIDKey{}).(string); ok {
		return v
	}
	return ""
}

// NewTraceID 生成新的 traceId，底层使用 UUID v4 字符串。
func NewTraceID() string {
	return uuid.NewString()
}
