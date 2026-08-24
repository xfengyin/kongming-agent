// 观测台 - 可观测性系统（Prometheus 指标）
// 知己知彼，百战不殆

package observatory

import (
	"context"
	"fmt"
	"sync"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
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

	llmCallsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "kongming_llm_calls_total",
			Help: "Total number of LLM calls",
		},
		[]string{"provider", "status"},
	)

	llmLatency = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "kongming_llm_latency_seconds",
			Help:    "LLM call latency in seconds",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"provider"},
	)
)

// Observatory 观测台（指标优先；链路追踪列为 roadmap）
type Observatory struct {
	metricsEnabled bool
	mu             sync.Mutex
}

// NewObservatory 创建观测台
func NewObservatory() *Observatory {
	return &Observatory{
		metricsEnabled: true,
	}
}

// Start 启动观测台
func (o *Observatory) Start(ctx context.Context) error {
	return nil
}

// Shutdown 关闭观测台
func (o *Observatory) Shutdown(ctx context.Context) error {
	return nil
}

// RecordHTTPRequest 记录HTTP请求
func RecordHTTPRequest(method, endpoint string, status int, duration float64) {
	httpRequestsTotal.WithLabelValues(method, endpoint, fmt.Sprintf("%d", status)).Inc()
	httpRequestDuration.WithLabelValues(method, endpoint).Observe(duration)
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

// RecordLLMCall 记录 LLM 调用（provider 适配层埋点）
func RecordLLMCall(provider, status string, latencySeconds float64) {
	llmCallsTotal.WithLabelValues(provider, status).Inc()
	llmLatency.WithLabelValues(provider).Observe(latencySeconds)
}
