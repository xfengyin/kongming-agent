package resilience

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// TestWithTimeout_AppliesDeadline 验证正常路径会加 deadline。
func TestWithTimeout_AppliesDeadline(t *testing.T) {
	ctx, cancel := WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	deadline, ok := ctx.Deadline()
	assert.True(t, ok, "应设置 deadline")
	assert.WithinDuration(t, time.Now().Add(50*time.Millisecond), deadline, 10*time.Millisecond)
}

// TestWithTimeout_NonPositive 验证 d <= 0 返回 noop cancel。
func TestWithTimeout_NonPositive(t *testing.T) {
	cases := []time.Duration{0, -1, -time.Hour}
	for _, d := range cases {
		ctx, cancel := WithTimeout(context.Background(), d)
		_, ok := ctx.Deadline()
		assert.False(t, ok, "d=%v 时不应有 deadline", d)
		cancel() // 必须可调用
	}
}

// TestWithTimeout_CancelTriggers 验证 cancel 能触发 ctx 取消。
func TestWithTimeout_CancelTriggers(t *testing.T) {
	ctx, cancel := WithTimeout(context.Background(), time.Hour)
	cancel()
	assert.ErrorIs(t, ctx.Err(), context.Canceled)
}

// TestErrTimeout 验证哨兵错误。
func TestErrTimeout(t *testing.T) {
	assert.NotNil(t, ErrTimeout)
	assert.True(t, errors.Is(ErrTimeout, ErrTimeout))
}

// TestIsTimeout 验证 IsTimeout 识别超时错误。
func TestIsTimeout(t *testing.T) {
	assert.False(t, IsTimeout(nil))
	assert.True(t, IsTimeout(ErrTimeout))
	assert.True(t, IsTimeout(context.DeadlineExceeded))
	// 用 %w 包装 DeadlineExceeded
	wrapped := fmt.Errorf("wrap: %w", context.DeadlineExceeded)
	assert.True(t, IsTimeout(wrapped), "包装的 DeadlineExceeded 应识别")
	// 用 %w 包装 ErrTimeout
	wrapped2 := fmt.Errorf("wrap: %w", ErrTimeout)
	assert.True(t, IsTimeout(wrapped2), "包装的 ErrTimeout 应识别")
	assert.False(t, IsTimeout(errors.New("other error")), "普通错误不应识别")
	// errors.Join
	joined := errors.Join(ErrTimeout, errors.New("inner"))
	assert.True(t, IsTimeout(joined), "errors.Join 链应识别")
}
