package resilience

import (
	"math"
	"math/rand"
	"time"
)

// Backoff 计算第 n 次失败后的等待时间（n 从 1 开始）。
//
// 公式：InitialBackoff * BackoffFactor^(n-1)，封顶 MaxBackoff；
// 开启 Jitter 时叠加 ±25% 随机扰动以避免"惊群"。
//
// 边界：
//   - InitialBackoff <= 0 返回 0
//   - BackoffFactor <= 1 退化为 1（不再增长）
//   - MaxBackoff <= 0 不封顶
func (c RetryConfig) Backoff(attempt int) time.Duration {
	if attempt <= 0 {
		return 0
	}
	initial := float64(c.InitialBackoff)
	if initial <= 0 {
		return 0
	}
	factor := c.BackoffFactor
	if factor <= 1 {
		factor = 1
	}
	// 指数退避
	d := initial * math.Pow(factor, float64(attempt-1))
	// 封顶
	if c.MaxBackoff > 0 && d > float64(c.MaxBackoff) {
		d = float64(c.MaxBackoff)
	}
	// 抖动：±25%
	if c.Jitter {
		span := d * 0.25
		d = d - span + rand.Float64()*2*span
		if d < 0 {
			d = 0
		}
	}
	return time.Duration(d)
}

// IsExhausted 判断重试是否已用尽（最后一次尝试仍失败）。
// MaxAttempts <= 1 视为不重试，第一次即耗尽。
func (c RetryConfig) IsExhausted(attempt int) bool {
	if c.MaxAttempts <= 1 {
		return true
	}
	return attempt >= c.MaxAttempts
}
