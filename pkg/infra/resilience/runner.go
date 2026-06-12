package resilience

import (
	"context"
	"fmt"
	"time"

	"github.com/zhuge/kongming/pkg/domain/port"
	"go.uber.org/zap"
)

// Runner 弹性执行器（ResilientRunner 的默认实现）。
//
// 装饰器链顺序（由外到内）：
//
//	[ctx 截止] → [ratelimit] → [circuitbreaker] → [retry] → fn(ctx)
//
// 选择理由：
//   - ctx 截止在最外层：硬上限，连等待令牌的时间也算上
//   - ratelimit 次之：避免大量请求排队冲击下游
//   - circuitbreaker 第三：先粗粒度拒绝，再让重试去尝试
//   - retry 最内层：在拿到令牌且熔断放行后，反复尝试
//
// 不可变 + 无全局状态：每个 Runner 独立持有 breaker/limiter，
// 同一 Runner 可被多 goroutine 并发调用（内部已用 mutex/atomic 保护）。
type Runner struct {
	cfg     Config
	logger  *zap.Logger
	breaker *CircuitBreaker
	limiter *RateLimiter
	retry   RetryConfig
}

// NewRunner 构造弹性执行器。
//
// cfg 任意字段为零值时退化为合理默认（详见各子模块）。
// logger 为 nil 时使用 zap.NewNop()，方便测试与零依赖启动。
func NewRunner(cfg Config, logger *zap.Logger) *Runner {
	if logger == nil {
		logger = zap.NewNop()
	}
	return &Runner{
		cfg:     cfg,
		logger:  logger,
		breaker: NewCircuitBreaker(cfg.CircuitBreaker.Threshold, cfg.CircuitBreaker.Timeout),
		limiter: NewRateLimiter(cfg.RateLimit.RPS, cfg.RateLimit.Burst),
		retry: RetryConfig{
			MaxAttempts:    cfg.Retry.MaxAttempts,
			InitialBackoff: cfg.Retry.InitialBackoff,
			MaxBackoff:     cfg.Retry.MaxBackoff,
			BackoffFactor:  cfg.Retry.BackoffFactor,
			Jitter:         cfg.Retry.Jitter,
		},
	}
}

// Breaker 暴露内部熔断器（仅用于指标/测试；不要在外部直接调用 Allow/Record）。
func (r *Runner) Breaker() *CircuitBreaker { return r.breaker }

// Limiter 暴露内部限流器。
func (r *Runner) Limiter() *RateLimiter { return r.limiter }

// Run 实现 port.ResilientRunner.Run。
func (r *Runner) Run(ctx context.Context, name string, fn func(ctx context.Context) error) error {
	_, err := r.RunWithResult(ctx, name, func(ctx context.Context) (any, error) {
		return nil, fn(ctx)
	})
	return err
}

// RunWithResult 实现 port.ResilientRunner.RunWithResult。
//
// 流程：
//  1. 检查父 ctx 是否已取消/超时
//  2. 等待限流令牌
//  3. 请求熔断器放行
//  4. 在重试循环中调用 fn；遇错误指数退避后重试
//  5. 成功 / 耗尽 / 熔断拒绝 / ctx 取消 → 返回对应结果
func (r *Runner) RunWithResult(
	ctx context.Context,
	name string,
	fn func(ctx context.Context) (any, error),
) (any, error) {
	if name == "" {
		name = "resilience"
	}

	// 1. ctx 状态
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	// 2. 限流
	if err := r.limiter.Wait(ctx); err != nil {
		return nil, err
	}

	// 3. 熔断（一次性放行；拒绝则不进入重试循环）
	if err := r.breaker.Allow(); err != nil {
		r.logger.Debug("circuit open", zap.String("name", name), zap.Error(err))
		return nil, err
	}

	// 4. 重试循环
	var (
		result   any
		lastErr  error
		attempts = r.retry.MaxAttempts
	)
	if attempts < 1 {
		attempts = 1
	}

	for attempt := 1; attempt <= attempts; attempt++ {
		if err := ctx.Err(); err != nil {
			r.breaker.Record(err)
			return nil, err
		}

		result, lastErr = fn(ctx)
		if lastErr == nil {
			r.breaker.Record(nil)
			return result, nil
		}

		r.breaker.Record(lastErr)
		r.logger.Debug("resilience attempt failed",
			zap.String("name", name),
			zap.Int("attempt", attempt),
			zap.Int("max_attempts", attempts),
			zap.Error(lastErr),
		)

		// 最后一次失败不再 sleep
		if r.retry.IsExhausted(attempt) {
			break
		}
		sleep := r.retry.Backoff(attempt)
		t := time.NewTimer(sleep)
		select {
		case <-ctx.Done():
			t.Stop()
			return nil, ctx.Err()
		case <-t.C:
		}
	}

	if lastErr == nil {
		// 理论不会到这里；防御性保护
		return nil, fmt.Errorf("resilience: exhausted without error")
	}
	return nil, lastErr
}

// 编译期断言：Runner 实现 port.ResilientRunner
var _ port.ResilientRunner = (*Runner)(nil)
