package resilience

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestRateLimiter_Allow_InitialBurst 验证初始桶满允许 burst 突发。
func TestRateLimiter_Allow_InitialBurst(t *testing.T) {
	rl := NewRateLimiter(10, 5) // 10 RPS, burst 5
	// 桶初始为满 → 前 5 次 Allow 成功
	for i := 0; i < 5; i++ {
		assert.True(t, rl.Allow(), "前 %d 次 Allow 应成功", i+1)
	}
	// 第 6 次（无时间流逝）应失败
	assert.False(t, rl.Allow(), "第 6 次应失败（无令牌）")
}

// TestRateLimiter_Allow_RefillsOverTime 验证随时间补充令牌。
func TestRateLimiter_Allow_RefillsOverTime(t *testing.T) {
	rl := NewRateLimiter(1000, 1) // 1000 RPS, burst 1
	// 用掉初始 1 个
	assert.True(t, rl.Allow())
	assert.False(t, rl.Allow())
	// 等待 ~2ms 补充 2 个令牌（1000/s = 1/ms）
	time.Sleep(5 * time.Millisecond)
	assert.True(t, rl.Allow(), "5ms 后应至少有 1 个令牌")
}

// TestRateLimiter_Wait 验证 Wait 能阻塞到令牌可用。
func TestRateLimiter_Wait(t *testing.T) {
	rl := NewRateLimiter(100, 1) // 100 RPS, burst 1
	// 消耗初始 1 个
	require.True(t, rl.Allow())
	// 等待 ≤ 50ms 应能成功
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	start := time.Now()
	require.NoError(t, rl.Wait(ctx))
	elapsed := time.Since(start)
	assert.Less(t, elapsed, 50*time.Millisecond, "应在 50ms 内拿到令牌")
}

// TestRateLimiter_Wait_CtxCancel 验证 ctx 取消能立即返回。
func TestRateLimiter_Wait_CtxCancel(t *testing.T) {
	rl := NewRateLimiter(1, 1) // 1 RPS（很慢）
	require.True(t, rl.Allow())
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	err := rl.Wait(ctx)
	assert.ErrorIs(t, err, context.DeadlineExceeded, "ctx 取消应返回 DeadlineExceeded")
}

// TestRateLimiter_Wait_AlreadyHasToken 验证桶有令牌时立即返回。
func TestRateLimiter_Wait_AlreadyHasToken(t *testing.T) {
	rl := NewRateLimiter(10, 5)
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	start := time.Now()
	require.NoError(t, rl.Wait(ctx))
	assert.Less(t, time.Since(start), 10*time.Millisecond, "有令牌时应立即返回")
}

// TestRateLimiter_ZeroParams 验证零值参数退化为合理默认。
func TestRateLimiter_ZeroParams(t *testing.T) {
	rl := NewRateLimiter(0, 0) // 全部零值
	assert.NotNil(t, rl)
	// 不应 panic；至少 1 个令牌
	assert.True(t, rl.Allow())
}
