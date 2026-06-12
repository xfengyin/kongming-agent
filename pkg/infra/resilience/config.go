package resilience

import "time"

// Config 弹性执行配置（与 config.ResilienceConfig 字段一一对应）。
//
// 定义本地类型而非直接 import pkg/infra/config 是为了：
//   - 保持本子包零外部依赖（除 domain/port 与 zap），便于单测
//   - 避免跨子包循环依赖
//   - application 层可通过 ConfigFrom() 桥接到 config.ResilienceConfig
type Config struct {
	Retry          RetryConfig
	CircuitBreaker CircuitBreakerConfig
	RateLimit      RateLimitConfig
}

// RetryConfig 重试配置。
type RetryConfig struct {
	MaxAttempts    int
	InitialBackoff time.Duration
	MaxBackoff     time.Duration
	BackoffFactor  float64
	Jitter         bool
}

// CircuitBreakerConfig 熔断器配置。
type CircuitBreakerConfig struct {
	Threshold int
	Timeout   time.Duration
}

// RateLimitConfig 令牌桶限流配置。
type RateLimitConfig struct {
	RPS   int
	Burst int
}
