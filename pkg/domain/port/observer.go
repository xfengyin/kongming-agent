// Package port 定义领域层对外暴露的端口（接口）契约。
// 端口实现位于 pkg/infra 层，遵循依赖倒置原则。
package port

import (
	"context"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

// Observer 是可观测性端口，抽象出 tracing/metrics/logging 三大能力，
// 业务侧只依赖此接口，infra 层提供具体实现（zap+prom+otlp）。
type Observer interface {
	// StartSpan 开启一个 span，attrs 会被附加到 span 上，返回新的 ctx。
	StartSpan(ctx context.Context, name string, attrs ...attribute.KeyValue) (context.Context, trace.Span)

	// RecordError 在 span 上记录一个错误并设置 error=true 属性。
	RecordError(span trace.Span, err error)

	// RecordEvent 在 ctx 当前 span 上添加一个命名事件。
	RecordEvent(ctx context.Context, name string, attrs ...attribute.KeyValue)

	// IncCounter 累加 counter（按 labels 分桶），首次调用会懒注册到 registry。
	IncCounter(name string, labels map[string]string)

	// ObserveHistogram 记录一个直方图观测值；metric 名会自动追加 _seconds 后缀。
	ObserveHistogram(name string, value float64, labels map[string]string)

	// SetGauge 设置 gauge 当前值（按 labels 分桶）。
	SetGauge(name string, value float64, labels map[string]string)

	// Shutdown 释放底层资源（tracer provider 等），应在进程退出前调用。
	Shutdown(ctx context.Context) error
}
