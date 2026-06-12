package resilience

import (
	"context"
	"sync"
	"time"
)

// RateLimiter 令牌桶限流器（自实现，避免引入 golang.org/x/time 外部依赖）。
//
// 算法说明：
//   - 桶容量 = burst
//   - 令牌补充速率 = rps 个/秒（连续补充，不以离散 tick 计算）
//   - 每次 Wait 消耗 1 个令牌；不足则阻塞至补足或 ctx 取消
//   - 首次调用允许 burst 个突发（桶初始为满）
type RateLimiter struct {
	mu       sync.Mutex
	rps      float64
	burst    float64
	tokens   float64
	lastTime time.Time
	// now 注入时钟，便于测试。
	now func() time.Time
}

// NewRateLimiter 构造限流器。
//
// rps <= 0 退化为 1（每秒 1 个）；burst <= 0 退化为 rps。
func NewRateLimiter(rps, burst int) *RateLimiter {
	if rps <= 0 {
		rps = 1
	}
	if burst <= 0 {
		burst = rps
	}
	lim := &RateLimiter{
		rps:      float64(rps),
		burst:    float64(burst),
		tokens:   float64(burst), // 初始桶满，允许 burst 突发
		lastTime: time.Now(),
		now:      time.Now,
	}
	return lim
}

// setNow 注入时间源（仅测试用）。
func (l *RateLimiter) setNow(now func() time.Time) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.now = now
	l.lastTime = now()
}

// refill 内部：按时间差补充令牌（锁内调用）。
func (l *RateLimiter) refill(now time.Time) {
	elapsed := now.Sub(l.lastTime).Seconds()
	if elapsed <= 0 {
		return
	}
	l.tokens += elapsed * l.rps
	if l.tokens > l.burst {
		l.tokens = l.burst
	}
	l.lastTime = now
}

// Allow 非阻塞检查是否可放行。
func (l *RateLimiter) Allow() bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	now := l.now()
	l.refill(now)
	if l.tokens >= 1 {
		l.tokens--
		return true
	}
	return false
}

// Wait 阻塞直到获得令牌或 ctx 取消。
//
// 返回 nil 表示成功获得令牌；
// 返回 ctx.Err() 表示被取消或超时。
func (l *RateLimiter) Wait(ctx context.Context) error {
	for {
		l.mu.Lock()
		now := l.now()
		l.refill(now)
		if l.tokens >= 1 {
			l.tokens--
			l.mu.Unlock()
			return nil
		}
		// 算出还需等待多久能补到 1 个令牌
		need := 1 - l.tokens
		wait := time.Duration(need / l.rps * float64(time.Second))
		l.mu.Unlock()

		// 防止 wait 过小导致自旋
		if wait < time.Millisecond {
			wait = time.Millisecond
		}

		t := time.NewTimer(wait)
		select {
		case <-ctx.Done():
			t.Stop()
			return ctx.Err()
		case <-t.C:
			// 继续循环尝试取令牌
		}
	}
}
