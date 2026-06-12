package observability

import (
	"context"
	"fmt"
	"net/http"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"

	"github.com/zhuge/kongming/pkg/domain/port"
	"github.com/zhuge/kongming/pkg/infra/config"
)

// serviceName / serviceVersion 在 resource 中作为标准属性暴露给后端。
const (
	serviceName    = "kongming"
	serviceVersion = "1.0.0"
)

// Observer 是 domain/port.Observer 的具体实现，组合 Metrics + OTel Tracer。
// 通过 NewObserver 构造，支持 OTLP HTTP exporter 与关闭 tracing 两种模式。
type Observer struct {
	logger      *zap.Logger
	metrics     *Metrics
	registry    *prometheus.Registry
	tracer      trace.Tracer
	provider    *sdktrace.TracerProvider
	metricsPort int
}

// NewObserver 构造可观测性适配器。
//   - 总是创建独立 prom registry 与 metrics 工厂；
//   - 当 cfg.Tracing.Enabled=true 时初始化 OTLP HTTP exporter（注入到 otel 全局 provider）；
//   - 关闭 tracing 时 tracer 使用一个 noop span，调用方仍能正常 StartSpan/End。
func NewObserver(ctx context.Context, cfg config.ObservatoryConfig, logger *zap.Logger) (*Observer, error) {
	reg := prometheus.NewRegistry()
	m := newMetrics(reg)

	obs := &Observer{
		logger:      logger,
		metrics:     m,
		registry:    reg,
		metricsPort: cfg.MetricsPort,
		tracer:      otel.Tracer(serviceName),
	}

	if cfg.Tracing.Enabled {
		if err := obs.initTracing(ctx, cfg.Tracing); err != nil {
			return nil, fmt.Errorf("init tracing: %w", err)
		}
	}
	return obs, nil
}

// initTracing 初始化 OTel TracerProvider，使用 OTLP/HTTP exporter 推送 span。
// 资源属性直接用 attribute.KeyValue 构造，避免引入 semconv 包。
func (o *Observer) initTracing(ctx context.Context, cfg config.TracingConfig) error {
	exporter, err := otlptracehttp.New(ctx, otlptracehttp.WithEndpoint(cfg.Endpoint))
	if err != nil {
		return fmt.Errorf("create otlp exporter: %w", err)
	}

	res, err := resource.New(ctx, resource.WithAttributes(
		attribute.String("service.name", serviceName),
		attribute.String("service.version", serviceVersion),
	))
	if err != nil {
		return fmt.Errorf("create resource: %w", err)
	}

	sampler := sdktrace.ParentBased(sdktrace.TraceIDRatioBased(cfg.SamplingRate))
	o.provider = sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exporter),
		sdktrace.WithResource(res),
		sdktrace.WithSampler(sampler),
	)
	otel.SetTracerProvider(o.provider)
	o.tracer = o.provider.Tracer(serviceName)
	return nil
}

// StartSpan 启动一个 OTel span，attrs 会被附加。
func (o *Observer) StartSpan(ctx context.Context, name string, attrs ...attribute.KeyValue) (context.Context, trace.Span) {
	return o.tracer.Start(ctx, name, trace.WithAttributes(attrs...))
}

// RecordError 在 span 上记录错误并打 error=true 属性。
func (o *Observer) RecordError(span trace.Span, err error) {
	if span == nil || err == nil {
		return
	}
	span.RecordError(err)
	span.SetAttributes(attribute.Bool("error", true))
}

// RecordEvent 在 ctx 当前 span 上添加一个命名事件。
func (o *Observer) RecordEvent(ctx context.Context, name string, attrs ...attribute.KeyValue) {
	span := trace.SpanFromContext(ctx)
	span.AddEvent(name, trace.WithAttributes(attrs...))
}

// IncCounter 累加 counter。
func (o *Observer) IncCounter(name string, labels map[string]string) {
	o.metrics.IncCounter(name, labels)
}

// ObserveHistogram 记录一个直方图观测值。
func (o *Observer) ObserveHistogram(name string, value float64, labels map[string]string) {
	o.metrics.ObserveHistogram(name, value, labels)
}

// SetGauge 设置 gauge 当前值。
func (o *Observer) SetGauge(name string, value float64, labels map[string]string) {
	o.metrics.SetGauge(name, value, labels)
}

// Handler 返回 prom metrics 的 http handler（用于挂到 /metrics）。
func (o *Observer) Handler() http.Handler {
	return promhttp.HandlerFor(o.registry, promhttp.HandlerOpts{})
}

// Registry 返回 prom registry，供调用方做高级集成（自定义注册等）。
func (o *Observer) Registry() *prometheus.Registry {
	return o.registry
}

// Shutdown 优雅关闭底层 TracerProvider（确保 spans 刷出）。
// 未启用 tracing 时 noop。
func (o *Observer) Shutdown(ctx context.Context) error {
	if o.provider == nil {
		return nil
	}
	if err := o.provider.Shutdown(ctx); err != nil {
		o.logger.Warn("observer shutdown failed", zap.Error(err))
		return err
	}
	return nil
}

// 编译期断言：Observer 必须实现 domain/port.Observer。
var _ port.Observer = (*Observer)(nil)
