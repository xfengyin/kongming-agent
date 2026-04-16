// 观测台 - 可观测性系统
// 知己知彼，百战不殆

package observatory

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/jaeger"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.21.0"
	"go.opentelemetry.io/otel/trace"
)

var (
	// Prometheus指标
	httpRequestsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "kongming_http_requests_total",
			Help: "Total number of HTTP requests",
		},
		[]string{"method", "endpoint", "status"},
	)

	httpRequestDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "kongming_http_request_duration_seconds",
			Help:    "HTTP request duration in seconds",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"method", "endpoint"},
	)

	activeOrders = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "kongming_active_orders",
			Help: "Number of active orders",
		},
	)

	tasksProcessed = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "kongming_tasks_processed_total",
			Help: "Total number of processed tasks",
		},
		[]string{"status"},
	)

	generalUtilization = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "kongming_general_utilization",
			Help: "General utilization percentage",
		},
		[]string{"general_id", "general_name"},
	)

	// Tracer
	tracer trace.Tracer
	once   sync.Once
)

// Observatory 观测台
type Observatory struct {
	metricsEnabled bool
	tracingEnabled bool
	jaegerEndpoint string
	tracerProvider *sdktrace.TracerProvider
}

// NewObservatory 创建观测台
func NewObservatory() *Observatory {
	return &Observatory{
		metricsEnabled: true,
		tracingEnabled: true,
		jaegerEndpoint: "http://localhost:14268/api/traces",
	}
}

// Start 启动观测台
func (o *Observatory) Start(ctx context.Context) error {
	if o.tracingEnabled {
		if err := o.initTracing(ctx); err != nil {
			return fmt.Errorf("初始化追踪失败: %w", err)
		}
	}
	return nil
}

// initTracing 初始化追踪
func (o *Observatory) initTracing(ctx context.Context) error {
	exporter, err := jaeger.New(jaeger.WithCollectorEndpoint(jaeger.WithEndpoint(o.jaegerEndpoint)))
	if err != nil {
		return err
	}

	res, err := resource.New(ctx,
		resource.WithAttributes(
			semconv.ServiceName("kongming"),
			semconv.ServiceVersion("1.0.0"),
		),
	)
	if err != nil {
		return err
	}

	o.tracerProvider = sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exporter),
		sdktrace.WithResource(res),
		sdktrace.WithSampler(sdktrace.AlwaysSample()),
	)

	otel.SetTracerProvider(o.tracerProvider)
	tracer = o.tracerProvider.Tracer("kongming")

	return nil
}

// Shutdown 关闭观测台
func (o *Observatory) Shutdown(ctx context.Context) error {
	if o.tracerProvider != nil {
		return o.tracerProvider.Shutdown(ctx)
	}
	return nil
}

// RecordHTTPRequest 记录HTTP请求
func RecordHTTPRequest(method, endpoint string, status int, duration time.Duration) {
	httpRequestsTotal.WithLabelValues(method, endpoint, fmt.Sprintf("%d", status)).Inc()
	httpRequestDuration.WithLabelValues(method, endpoint).Observe(duration.Seconds())
}

// SetActiveOrders 设置活跃订单数
func SetActiveOrders(count int) {
	activeOrders.Set(float64(count))
}

// RecordTaskProcessed 记录处理完成的任务
func RecordTaskProcessed(status string) {
	tasksProcessed.WithLabelValues(status).Inc()
}

// SetGeneralUtilization 设置将领利用率
func SetGeneralUtilization(generalID, name string, utilization float64) {
	generalUtilization.WithLabelValues(generalID, name).Set(utilization)
}

// StartSpan 开始追踪跨度
func StartSpan(ctx context.Context, name string, attrs ...attribute.KeyValue) (context.Context, trace.Span) {
	if tracer == nil {
		once.Do(func() {
			tracer = otel.Tracer("kongming")
		})
	}
	return tracer.Start(ctx, name, trace.WithAttributes(attrs...))
}

// RecordError 记录错误
func RecordError(span trace.Span, err error) {
	span.RecordError(err)
	span.SetAttributes(attribute.Bool("error", true))
}
