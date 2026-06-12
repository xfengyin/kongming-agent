// Package config 提供 Kongming 系统的统一配置加载能力。
//
// 设计原则（对应企业级 12 条规则）：
//   - 配置驱动（规则 7）：所有运行时参数通过 struct tag 绑定，零硬编码。
//   - 开闭原则（规则 1）：新增配置子段只需在 Config struct 中追加字段。
//   - 依赖倒置（规则 2）：上层仅依赖 Config 数据结构，不依赖 viper 细节。
//   - 接口隔离（规则 4）：Config 子段按业务域拆分（server/features/observatory/...）。
//   - 可观测（规则 6）：校验失败返回结构化错误信息。
//
// 加载流程：默认值（setDefaults）→ 配置文件（viper.ReadInConfig）→ 环境变量覆盖
// （KONGMING_*，dot 替换为下划线）→ struct tag 绑定（v.Unmarshal）→ 业务校验
// （go-playground/validator/v10）。
package config

import "time"

// Config 是 Kongming 系统的根配置对象，承载所有子域配置。
// mapstructure 负责 YAML/JSON 字段映射，validate 负责必填与边界校验。
type Config struct {
	Server      ServerConfig      `mapstructure:"server" validate:"required"`
	Features    FeaturesConfig    `mapstructure:"features"`
	Observatory ObservatoryConfig `mapstructure:"observatory" validate:"required"`
	Commander   CommanderConfig   `mapstructure:"commander" validate:"required"`
	Dispatcher  DispatcherConfig  `mapstructure:"dispatcher"`
	Generals    GeneralsConfig    `mapstructure:"generals"`
	Bagua       BaguaConfig       `mapstructure:"bagua"`
	Vault       VaultConfig       `mapstructure:"vault"`
	Courier     CourierConfig     `mapstructure:"courier"`
	Resilience  ResilienceConfig  `mapstructure:"resilience"`
	Plugin      PluginConfig      `mapstructure:"plugin"`
}

// ServerConfig 定义 HTTP/gRPC 服务监听与超时参数。
type ServerConfig struct {
	Host            string        `mapstructure:"host" validate:"required"`
	Port            int           `mapstructure:"port" validate:"required,min=1,max=65535"`
	GRPCPort        int           `mapstructure:"grpc_port" validate:"required,min=1,max=65535"`
	ReadTimeout     time.Duration `mapstructure:"read_timeout"`
	WriteTimeout    time.Duration `mapstructure:"write_timeout"`
	ShutdownTimeout time.Duration `mapstructure:"shutdown_timeout"`
}

// FeaturesConfig 控制可观测特性开关。
type FeaturesConfig struct {
	EnableMetrics     bool `mapstructure:"enable_metrics"`
	EnableTracing     bool `mapstructure:"enable_tracing"`
	EnableObservatory bool `mapstructure:"enable_observatory"`
}

// ObservatoryConfig 聚合 metrics/tracing/log 三个可观测子域。
type ObservatoryConfig struct {
	MetricsPort int           `mapstructure:"metrics_port"`
	Tracing     TracingConfig `mapstructure:"tracing"`
	Log         LogConfig     `mapstructure:"log"`
}

// TracingConfig OpenTelemetry 链路追踪配置。
type TracingConfig struct {
	Enabled      bool    `mapstructure:"enabled"`
	Endpoint     string  `mapstructure:"endpoint"`
	SamplingRate float64 `mapstructure:"sampling_rate"`
	Exporter     string  `mapstructure:"exporter"` // jaeger | otlp
}

// LogConfig 日志配置（encoding 限定 json/console）。
type LogConfig struct {
	Level    string `mapstructure:"level"`
	Encoding string `mapstructure:"encoding"`
}

// CommanderConfig 军师（Commander）执行参数。
type CommanderConfig struct {
	DefaultTimeout      time.Duration `mapstructure:"default_timeout"`
	MaxConcurrentOrders int           `mapstructure:"max_concurrent_orders"`
}

// DispatcherConfig 调度器参数。
type DispatcherConfig struct {
	QueueSize int           `mapstructure:"queue_size"`
	Timeout   time.Duration `mapstructure:"timeout"`
}

// GeneralsConfig 将领池配置。
type GeneralsConfig struct {
	PoolSize       int           `mapstructure:"pool_size"`
	DefaultTimeout time.Duration `mapstructure:"default_timeout"`
}

// BaguaConfig 八卦阵（workflow）默认参数。
type BaguaConfig struct {
	DefaultMode      string `mapstructure:"default_mode"`
	MaxParallelNodes int    `mapstructure:"max_parallel_nodes"`
}

// VaultConfig 锦囊库（strategy vault）参数。
type VaultConfig struct {
	Dir         string `mapstructure:"dir"`
	AutoReload  bool   `mapstructure:"auto_reload"`
	BuiltinOnly bool   `mapstructure:"builtin_only"`
}

// CourierConfig 传令兵（消息通道）参数。
type CourierConfig struct {
	InboxSize       int           `mapstructure:"inbox_size"`
	OutboxSize      int           `mapstructure:"outbox_size"`
	DeliveryTimeout time.Duration `mapstructure:"delivery_timeout"`
}

// ResilienceConfig 弹性执行链参数（retry + circuitbreaker + ratelimit）。
type ResilienceConfig struct {
	Retry          RetryConfig          `mapstructure:"retry"`
	CircuitBreaker CircuitBreakerConfig `mapstructure:"circuit_breaker"`
	RateLimit      RateLimitConfig      `mapstructure:"rate_limit"`
}

// RetryConfig 重试策略配置。
type RetryConfig struct {
	MaxAttempts    int           `mapstructure:"max_attempts"`
	InitialBackoff time.Duration `mapstructure:"initial_backoff"`
	MaxBackoff     time.Duration `mapstructure:"max_backoff"`
	BackoffFactor  float64       `mapstructure:"backoff_factor"`
	Jitter         bool          `mapstructure:"jitter"`
}

// CircuitBreakerConfig 熔断器配置。
type CircuitBreakerConfig struct {
	Threshold int           `mapstructure:"threshold"`
	Timeout   time.Duration `mapstructure:"timeout"`
}

// RateLimitConfig 限流配置（令牌桶）。
type RateLimitConfig struct {
	RPS   int `mapstructure:"rps"`
	Burst int `mapstructure:"burst"`
}

// PluginConfig 插件系统配置。
type PluginConfig struct {
	Dir        string   `mapstructure:"dir"`
	Extensions []string `mapstructure:"extensions"`
	Watch      bool     `mapstructure:"watch"`
}
