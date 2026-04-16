// 复读机 - 重复执行与重试系统
// 知己知彼，百战不殆

package repeater

import (
	"context"
	"fmt"
	"sync"
	"time"

	"go.uber.org/zap"
)

// RetryPolicy 重试策略
type RetryPolicy struct {
	MaxAttempts    int           // 最大重试次数
	InitialBackoff time.Duration // 初始退避时间
	MaxBackoff     time.Duration // 最大退避时间
	BackoffFactor  float64       // 退避因子
	Jitter         bool          // 是否添加抖动
}

// DefaultRetryPolicy 默认重试策略
var DefaultRetryPolicy = &RetryPolicy{
	MaxAttempts:    3,
	InitialBackoff:  100 * time.Millisecond,
	MaxBackoff:      30 * time.Second,
	BackoffFactor:   2.0,
	Jitter:          true,
}

// TaskFunc 任务函数
type TaskFunc func(ctx context.Context) error

// Repeater 复读机
type Repeater struct {
	logger *zap.Logger
}

// NewRepeater 创建复读机
func NewReperier(logger *zap.Logger) *Repeater {
	return &Repeater{logger: logger}
}

// Retry 执行重试
func (r *Repeater) Retry(ctx context.Context, policy *RetryPolicy, task string, fn TaskFunc) error {
	if policy == nil {
		policy = DefaultRetryPolicy
	}

	var lastErr error
	backoff := policy.InitialBackoff

	for attempt := 1; attempt <= policy.MaxAttempts; attempt++ {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		r.logger.Debug("执行任务",
			zap.String("task", task),
			zap.Int("attempt", attempt),
			zap.Int("max", policy.MaxAttempts),
		)

		if err := fn(ctx); err != nil {
			lastErr = err
			r.logger.Warn("任务失败，准备重试",
				zap.String("task", task),
				zap.Int("attempt", attempt),
				zap.Error(err),
			)

			if attempt < policy.MaxAttempts {
				// 计算退避时间
				sleep := backoff
				if policy.Jitter {
					sleep = addJitter(sleep)
				}
				backoff = time.Duration(float64(backoff) * policy.BackoffFactor)
				if backoff > policy.MaxBackoff {
					backoff = policy.MaxBackoff
				}

				r.logger.Debug("等待退避", zap.Duration("sleep", sleep))
				select {
				case <-ctx.Done():
					return ctx.Err()
				case <-time.After(sleep):
				}
			}
			continue
		}

		r.logger.Info("任务成功", zap.String("task", task), zap.Int("attempts", attempt))
		return nil
	}

	return fmt.Errorf("重试%d次后仍失败: %w", policy.MaxAttempts, lastErr)
}

// RetryWithResult 带结果的重试
func (r *Repeater) RetryWithResult(ctx context.Context, policy *RetryPolicy, task string, fn func(ctx context.Context) (interface{}, error)) (interface{}, error) {
	if policy == nil {
		policy = DefaultRetryPolicy
	}

	var lastErr error
	backoff := policy.InitialBackoff

	for attempt := 1; attempt <= policy.MaxAttempts; attempt++ {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		result, err := fn(ctx)
		if err != nil {
			lastErr = err
			if attempt < policy.MaxAttempts {
				sleep := backoff
				if policy.Jitter {
					sleep = addJitter(sleep)
				}
				backoff = time.Duration(float64(backoff) * policy.BackoffFactor)
				if backoff > policy.MaxBackoff {
					backoff = policy.MaxBackoff
				}

				select {
				case <-ctx.Done():
					return nil, ctx.Err()
				case <-time.After(sleep):
				}
			}
			continue
		}

		return result, nil
	}

	return nil, fmt.Errorf("重试%d次后仍失败: %w", policy.MaxAttempts, lastErr)
}

// CircuitBreaker 熔断器
type CircuitBreaker struct {
	mu          sync.RWMutex
	state       CircuitState
	failures    int
	threshold   int
	timeout     time.Duration
	lastFailure time.Time
}

// CircuitState 熔断状态
type CircuitState int

const (
	CircuitClosed CircuitState = iota
	CircuitOpen
	CircuitHalfOpen
)

func (s CircuitState) String() string {
	switch s {
	case CircuitClosed:
		return "closed"
	case CircuitOpen:
		return "open"
	case CircuitHalfOpen:
		return "half-open"
	default:
		return "unknown"
	}
}

// NewCircuitBreaker 创建熔断器
func NewCircuitBreaker(threshold int, timeout time.Duration) *CircuitBreaker {
	return &CircuitBreaker{
		threshold: threshold,
		timeout:   timeout,
		state:     CircuitClosed,
	}
}

// Call 执行调用
func (cb *CircuitBreaker) Call(ctx context.Context, fn TaskFunc) error {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	switch cb.state {
	case CircuitOpen:
		// 检查是否超时
		if time.Since(cb.lastFailure) > cb.timeout {
			cb.state = CircuitHalfOpen
			cb.failures = 0
		} else {
			return fmt.Errorf("熔断器已打开")
		}
	case CircuitHalfOpen:
		// 只允许一次调用
	}

	err := fn(ctx)
	if err != nil {
		cb.failures++
		cb.lastFailure = time.Now()
		if cb.failures >= cb.threshold {
			cb.state = CircuitOpen
		}
		return err
	}

	// 成功调用
	if cb.state == CircuitHalfOpen {
		cb.state = CircuitClosed
		cb.failures = 0
	}

	return nil
}

// GetState 获取熔断器状态
func (cb *CircuitBreaker) GetState() CircuitState {
	cb.mu.RLock()
	defer cb.mu.RUnlock()
	return cb.state
}

// addJitter 添加抖动
func addJitter(d time.Duration) time.Duration {
	// 50%的随机抖动
	jitter := time.Duration(float64(d) * 0.5 * (float64(time.Now().UnixNano()%100)/100.0))
	return d + jitter
}
