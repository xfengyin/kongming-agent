package observability

import (
	"context"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel/attribute"
	"go.uber.org/zap"

	"github.com/zhuge/kongming/pkg/domain/port"
	"github.com/zhuge/kongming/pkg/infra/config"
)

func TestStartSpan_NoExporter(t *testing.T) {
	// Tracing 关闭时，StartSpan 仍应能正常工作（用 noop span 兜底）
	obs, err := NewObserver(context.Background(), config.ObservatoryConfig{
		Tracing: config.TracingConfig{Enabled: false},
	}, zap.NewNop())
	require.NoError(t, err)
	t.Cleanup(func() { _ = obs.Shutdown(context.Background()) })

	_, span := obs.StartSpan(context.Background(), "test")
	defer span.End()
	assert.NotNil(t, span)
	span.SetAttributes(attribute.String("k", "v"))
	span.End()
}

func TestNewObserver_MetricsHandler(t *testing.T) {
	obs, err := NewObserver(context.Background(), config.ObservatoryConfig{}, zap.NewNop())
	require.NoError(t, err)
	t.Cleanup(func() { _ = obs.Shutdown(context.Background()) })

	// 写一些指标，然后通过 handler 抓取
	obs.IncCounter("kongming_smoke_total", map[string]string{"result": "ok"})
	obs.ObserveHistogram("kongming_smoke_dur", 0.42, nil)
	obs.SetGauge("kongming_smoke_gauge", 9, nil)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/metrics", nil)
	obs.Handler().ServeHTTP(rec, req)
	body := rec.Body.String()

	assert.Equal(t, 200, rec.Code)
	assert.Contains(t, body, "kongming_smoke_total")
	assert.Contains(t, body, "kongming_smoke_dur_seconds")
	assert.Contains(t, body, "kongming_smoke_gauge")
}

func TestObserver_RecordErrorAndEvent(t *testing.T) {
	obs, err := NewObserver(context.Background(), config.ObservatoryConfig{}, zap.NewNop())
	require.NoError(t, err)
	t.Cleanup(func() { _ = obs.Shutdown(context.Background()) })

	ctx, span := obs.StartSpan(context.Background(), "op")
	defer span.End()

	// RecordError 在 nil span / nil err 时应安全
	obs.RecordError(nil, nil)
	obs.RecordError(span, assertErr("boom"))
	obs.RecordEvent(ctx, "evt", attribute.String("k", "v"))

	// 验证 provider 为 nil 时 Shutdown 是 noop
	require.Nil(t, obs.provider)
	assert.NoError(t, obs.Shutdown(context.Background()))
}

func TestObserver_Registry(t *testing.T) {
	obs, err := NewObserver(context.Background(), config.ObservatoryConfig{}, zap.NewNop())
	require.NoError(t, err)
	t.Cleanup(func() { _ = obs.Shutdown(context.Background()) })

	assert.NotNil(t, obs.Registry())
}

func TestNewObserver_TracingEnabled(t *testing.T) {
	// Enabled=true 时 initTracing 路径会被执行；
	// otlptracehttp.New 是 lazy 连接到 endpoint，不会立即失败，
	// 用 localhost 任意端口即可（请求会异步 batch 重试）。
	obs, err := NewObserver(context.Background(), config.ObservatoryConfig{
		Tracing: config.TracingConfig{
			Enabled:      true,
			Endpoint:     "localhost:14317",
			SamplingRate: 1.0,
		},
	}, zap.NewNop())
	require.NoError(t, err)
	require.NotNil(t, obs.provider)

	// Shutdown 现在走真实 provider 分支
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 2*1e9)
	defer cancel()
	// 不强制要求 Shutdown 返回 nil（endpoint 不通时 batcher 会丢弃）；只验证不 panic
	_ = obs.Shutdown(shutdownCtx)
}

func TestNewObserver_ImplementsPort(t *testing.T) {
	obs, err := NewObserver(context.Background(), config.ObservatoryConfig{}, zap.NewNop())
	require.NoError(t, err)
	t.Cleanup(func() { _ = obs.Shutdown(context.Background()) })

	// 运行期断言：Observer 必须可赋值给 port.Observer
	var _ port.Observer = obs
}

// assertErr 返回一个非 nil error 辅助函数，避免引入 errors 包增加 import。
func assertErr(s string) error { return stringErr(s) }

type stringErr string

func (e stringErr) Error() string { return strings.TrimSpace(string(e)) }
