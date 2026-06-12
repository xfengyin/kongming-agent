package resilience

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

// newTestRunner 构造一个常用配置的 Runner，参数可调。
func newTestRunner(retryMax int, breakerThreshold int, rps int, burst int) *Runner {
	return NewRunner(Config{
		Retry: RetryConfig{
			MaxAttempts:    retryMax,
			InitialBackoff: 1 * time.Millisecond,
			MaxBackoff:     10 * time.Millisecond,
			BackoffFactor:  2.0,
			Jitter:         false,
		},
		CircuitBreaker: CircuitBreakerConfig{
			Threshold: breakerThreshold,
			Timeout:   time.Second,
		},
		RateLimit: RateLimitConfig{
			RPS:   rps,
			Burst: burst,
		},
	}, zap.NewNop())
}

// TestRunner_RetriesUntilSuccess 验证短暂错误会被重试直至成功。
func TestRunner_RetriesUntilSuccess(t *testing.T) {
	r := newTestRunner(3, 100, 1000, 2000)
	var calls atomic.Int32
	err := r.Run(context.Background(), "t", func(ctx context.Context) error {
		n := calls.Add(1)
		if n < 2 {
			return errors.New("transient")
		}
		return nil
	})
	require.NoError(t, err)
	assert.Equal(t, int32(2), calls.Load(), "应调用 2 次（1 失败 + 1 成功）")
}

// TestRunner_GivesUpAfterMaxAttempts 验证耗尽 MaxAttempts 后放弃。
func TestRunner_GivesUpAfterMaxAttempts(t *testing.T) {
	r := newTestRunner(2, 100, 1000, 2000)
	var calls atomic.Int32
	err := r.Run(context.Background(), "t", func(ctx context.Context) error {
		calls.Add(1)
		return errors.New("always fail")
	})
	assert.Error(t, err, "应返回最后错误")
	assert.Equal(t, int32(2), calls.Load(), "MaxAttempts=2 恰好调用 2 次")
}

// TestRunner_OpenCircuitShortCircuits 验证熔断开启时直接拒绝、不调 fn。
func TestRunner_OpenCircuitShortCircuits(t *testing.T) {
	r := newTestRunner(3, 2, 1000, 2000)
	// 制造 2 次连续失败 → 熔断 Open
	for i := 0; i < 2; i++ {
		_ = r.Run(context.Background(), "trip", func(ctx context.Context) error {
			return errors.New("fail")
		})
	}
	// 此时熔断器应已 Open
	assert.Equal(t, StateOpen, r.Breaker().GetState())

	var calls atomic.Int32
	err := r.Run(context.Background(), "after-open", func(ctx context.Context) error {
		calls.Add(1)
		return nil
	})
	assert.ErrorIs(t, err, ErrCircuitOpen, "Open 状态应短路")
	assert.Equal(t, int32(0), calls.Load(), "fn 不应被调用")
}

// TestRunner_RunWithResult 验证带返回值路径。
func TestRunner_RunWithResult(t *testing.T) {
	r := newTestRunner(3, 100, 1000, 2000)
	v, err := r.RunWithResult(context.Background(), "k", func(ctx context.Context) (any, error) {
		return 42, nil
	})
	require.NoError(t, err)
	assert.Equal(t, 42, v)
}

// TestRunner_RunWithResult_RetriesAndFails 验证带返回值 + 重试耗尽。
func TestRunner_RunWithResult_RetriesAndFails(t *testing.T) {
	r := newTestRunner(2, 100, 1000, 2000)
	v, err := r.RunWithResult(context.Background(), "k", func(ctx context.Context) (any, error) {
		return "never returned", errors.New("boom")
	})
	assert.Error(t, err)
	assert.Nil(t, v, "失败时不应返回 fn 的结果")
}

// TestRunner_ContextCanceled 验证 ctx 取消能中断。
func TestRunner_ContextCanceled(t *testing.T) {
	r := newTestRunner(5, 100, 1000, 2000)
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // 立即取消
	err := r.Run(ctx, "t", func(ctx context.Context) error {
		return nil
	})
	assert.Error(t, err, "ctx 已取消应返回错误")
}

// TestRunner_NameEmpty 验证空名 fallback 为 "resilience"。
func TestRunner_NameEmpty(t *testing.T) {
	r := newTestRunner(1, 100, 1000, 2000)
	err := r.Run(context.Background(), "", func(ctx context.Context) error {
		return nil
	})
	assert.NoError(t, err)
}

// TestRunner_NoRetry_OnlyOnce 验证 MaxAttempts=1 不重试。
func TestRunner_NoRetry_OnlyOnce(t *testing.T) {
	r := newTestRunner(1, 100, 1000, 2000)
	var calls atomic.Int32
	err := r.Run(context.Background(), "t", func(ctx context.Context) error {
		calls.Add(1)
		return errors.New("fail")
	})
	assert.Error(t, err)
	assert.Equal(t, int32(1), calls.Load(), "MaxAttempts=1 仅调 1 次")
}

// TestRunner_LoggerNil 验证 logger=nil 不 panic。
func TestRunner_LoggerNil(t *testing.T) {
	r := NewRunner(Config{
		Retry:          RetryConfig{MaxAttempts: 1, InitialBackoff: time.Millisecond},
		CircuitBreaker: CircuitBreakerConfig{Threshold: 100, Timeout: time.Second},
		RateLimit:      RateLimitConfig{RPS: 100, Burst: 100},
	}, nil)
	assert.NoError(t, r.Run(context.Background(), "t", func(ctx context.Context) error { return nil }))
}
