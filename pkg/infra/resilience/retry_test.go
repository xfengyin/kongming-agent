package resilience

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// TestRetry_Backoff_Exponential 验证指数退避按 factor 增长。
func TestRetry_Backoff_Exponential(t *testing.T) {
	c := RetryConfig{
		InitialBackoff: 100 * time.Millisecond,
		MaxBackoff:     time.Second,
		BackoffFactor:  2.0,
		Jitter:         false,
	}
	d1 := c.Backoff(1)
	d2 := c.Backoff(2)
	d3 := c.Backoff(3)
	assert.Equal(t, 100*time.Millisecond, d1, "第 1 次退避 = InitialBackoff")
	assert.Equal(t, 200*time.Millisecond, d2, "第 2 次退避 = InitialBackoff * 2")
	assert.Equal(t, 400*time.Millisecond, d3, "第 3 次退避 = InitialBackoff * 4")
}

// TestRetry_Backoff_MaxBackoff 验证封顶 MaxBackoff。
func TestRetry_Backoff_MaxBackoff(t *testing.T) {
	c := RetryConfig{
		InitialBackoff: 100 * time.Millisecond,
		MaxBackoff:     250 * time.Millisecond,
		BackoffFactor:  2.0,
	}
	// 100 * 2^4 = 1600ms → 封顶 250ms
	assert.Equal(t, 250*time.Millisecond, c.Backoff(5))
}

// TestRetry_Backoff_Jitter 验证抖动落在 ±25% 区间。
func TestRetry_Backoff_Jitter(t *testing.T) {
	c := RetryConfig{
		InitialBackoff: 100 * time.Millisecond,
		MaxBackoff:     time.Second,
		BackoffFactor:  1.0, // 不增长
		Jitter:         true,
	}
	// 100 ± 25% → [75, 125]
	lo := time.Duration(float64(100*time.Millisecond) * 0.75)
	hi := time.Duration(float64(100*time.Millisecond) * 1.25)
	for i := 0; i < 200; i++ {
		d := c.Backoff(1)
		assert.GreaterOrEqual(t, d, lo-1, "下界 75ms")
		assert.LessOrEqual(t, d, hi+1, "上界 125ms")
	}
}

// TestRetry_Backoff_NoJitter 验证关闭抖动时为确定值。
func TestRetry_Backoff_NoJitter(t *testing.T) {
	c := RetryConfig{
		InitialBackoff: 50 * time.Millisecond,
		MaxBackoff:     time.Second,
		BackoffFactor:  2.0,
		Jitter:         false,
	}
	assert.Equal(t, 50*time.Millisecond, c.Backoff(1))
	assert.Equal(t, 50*time.Millisecond, c.Backoff(1), "应返回相同值")
}

// TestRetry_Backoff_ZeroInitial 验证 InitialBackoff <= 0 退化为 0。
func TestRetry_Backoff_ZeroInitial(t *testing.T) {
	c := RetryConfig{InitialBackoff: 0, BackoffFactor: 2.0}
	assert.Equal(t, time.Duration(0), c.Backoff(1))
	assert.Equal(t, time.Duration(0), c.Backoff(5))
}

// TestRetry_Backoff_NegativeAttempt 验证 attempt <= 0 退化为 0。
func TestRetry_Backoff_NegativeAttempt(t *testing.T) {
	c := RetryConfig{InitialBackoff: 100 * time.Millisecond, BackoffFactor: 2.0}
	assert.Equal(t, time.Duration(0), c.Backoff(0))
	assert.Equal(t, time.Duration(0), c.Backoff(-1))
}

// TestRetry_IsExhausted 验证耗尽判定。
func TestRetry_IsExhausted(t *testing.T) {
	// MaxAttempts<=1 视为 0 重试，第一次即耗尽
	c0 := RetryConfig{MaxAttempts: 0}
	assert.True(t, c0.IsExhausted(1), "MaxAttempts=0 第一次即耗尽")
	c1 := RetryConfig{MaxAttempts: 1}
	assert.True(t, c1.IsExhausted(1), "MaxAttempts=1 第一次即耗尽")
	// 正常路径
	c3 := RetryConfig{MaxAttempts: 3}
	assert.False(t, c3.IsExhausted(1), "第 1 次未耗尽")
	assert.False(t, c3.IsExhausted(2), "第 2 次未耗尽")
	assert.True(t, c3.IsExhausted(3), "第 3 次耗尽")
	assert.True(t, c3.IsExhausted(4), "第 4 次已耗尽")
}
