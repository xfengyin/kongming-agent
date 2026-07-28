// 观测台 - 可观测性系统
// 知己知彼，百战不殆
// 注：原 jaeger exporter v1.23.0 已被 otel 弃用，改用 no-op tracer 保持指标可用

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
	"go.opentelemetry.io/otel/trace"
)

// 注：time 包用于 RecordHTTPRequest 的 time.Duration 参数

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

	// Tracer（懒初始化）
	tracer trace.Tracer
	once   sync.Once
)

// Observatory 观测台
type Observatory struct {
	metricsEnabled bool
	tracingEnabled bool
}

// NewObservatory 创建观测台
func NewObservatory() *Observatory {
	return &Observatory{
		metricsEnabled: true,
		tracingEnabled: true,
	}
}

// Start 启动观测台
func (o *Observatory) Start(ctx context.Context) error {
	// 使用 otel 默认全局 TracerProvider（no-op，可通过 otel.SetTracerProvider 注入实际实现）
	once.Do(func() {
		tracer = otel.Tracer("kongming")
	})
	return nil
}

// Shutdown 关闭观测台
func (o *Observatory) Shutdown(ctx context.Context) error {
	return nil
}

// RecordHTTPRequest 记录HTTP请求
// duration 使用 time.Duration，内部转换为秒观测到直方图
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
