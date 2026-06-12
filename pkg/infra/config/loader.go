package config

import (
	"bytes"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/go-playground/validator/v10"
	"github.com/spf13/viper"
)

// EnvPrefix 配置覆盖所使用的环境变量前缀。
const EnvPrefix = "KONGMING"

// newValidator 创建一个开启必填结构体验证的 validator 实例。
// 使用 WithRequiredStructEnabled 后，validate:"required" 在嵌套结构体上才会生效。
func newValidator() *validator.Validate {
	return validator.New(validator.WithRequiredStructEnabled())
}

// newViper 返回一个独立的 viper 实例，避免测试间全局状态污染。
// 每个 Load / LoadFromBytes 调用都应使用全新实例。
func newViper() *viper.Viper {
	v := viper.New()
	v.SetEnvPrefix(EnvPrefix)
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()
	return v
}

// Load 从指定路径加载配置文件，返回校验通过的 Config 指针。
// 加载优先级：默认值 < 配置文件 < 环境变量。
// 失败原因（读取/解析/校验）会被包装并以 fmt.Errorf 形式返回。
func Load(path string) (*Config, error) {
	v := newViper()
	v.SetConfigFile(path)
	applyDefaults(v)

	if err := v.ReadInConfig(); err != nil {
		return nil, fmt.Errorf("read config %q: %w", path, err)
	}
	return unmarshalAndValidate(v)
}

// LoadFromBytes 从内存字节流加载配置，ext 指定格式（yaml/json/toml）。
// 主要用于测试和动态下发场景。
func LoadFromBytes(data []byte, ext string) (*Config, error) {
	v := newViper()
	v.SetConfigType(ext)
	applyDefaults(v)
	if err := v.ReadConfig(bytes.NewReader(data)); err != nil {
		return nil, fmt.Errorf("read config bytes: %w", err)
	}
	return unmarshalAndValidate(v)
}

// unmarshalAndValidate 将 viper 当前值绑定到 Config 并执行结构体校验。
func unmarshalAndValidate(v *viper.Viper) (*Config, error) {
	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("unmarshal config: %w", err)
	}
	if err := newValidator().Struct(&cfg); err != nil {
		return nil, fmt.Errorf("validate config: %w", err)
	}
	return &cfg, nil
}

// applyDefaults 把硬编码默认值注入 viper；这是 Load / LoadFromBytes 公共步骤。
// 注意：默认值是配置的"底线"，应保持安全、可工作的兜底值。
func applyDefaults(v *viper.Viper) {
	// server
	v.SetDefault("server.host", "0.0.0.0")
	v.SetDefault("server.port", 8080)
	v.SetDefault("server.grpc_port", 8081)
	v.SetDefault("server.read_timeout", 30*time.Second)
	v.SetDefault("server.write_timeout", 30*time.Second)
	v.SetDefault("server.shutdown_timeout", 30*time.Second)

	// features
	v.SetDefault("features.enable_metrics", true)
	v.SetDefault("features.enable_tracing", true)
	v.SetDefault("features.enable_observatory", true)

	// observatory
	v.SetDefault("observatory.metrics_port", 9090)
	v.SetDefault("observatory.tracing.exporter", "jaeger")
	v.SetDefault("observatory.tracing.sampling_rate", 1.0)
	v.SetDefault("observatory.log.level", "info")
	v.SetDefault("observatory.log.encoding", "json")

	// commander
	v.SetDefault("commander.default_timeout", 30*time.Second)
	v.SetDefault("commander.max_concurrent_orders", 100)

	// dispatcher
	v.SetDefault("dispatcher.queue_size", 1000)
	v.SetDefault("dispatcher.timeout", 30*time.Second)

	// generals
	v.SetDefault("generals.pool_size", 5)
	v.SetDefault("generals.default_timeout", 60*time.Second)

	// bagua
	v.SetDefault("bagua.default_mode", "dizai")
	v.SetDefault("bagua.max_parallel_nodes", 10)

	// vault
	v.SetDefault("vault.dir", "./strategies")
	v.SetDefault("vault.auto_reload", true)

	// courier
	v.SetDefault("courier.inbox_size", 1000)
	v.SetDefault("courier.outbox_size", 1000)
	v.SetDefault("courier.delivery_timeout", 30*time.Second)

	// resilience
	v.SetDefault("resilience.retry.max_attempts", 3)
	v.SetDefault("resilience.retry.initial_backoff", 100*time.Millisecond)
	v.SetDefault("resilience.retry.max_backoff", 30*time.Second)
	v.SetDefault("resilience.retry.backoff_factor", 2.0)
	v.SetDefault("resilience.retry.jitter", true)
	v.SetDefault("resilience.circuit_breaker.threshold", 5)
	v.SetDefault("resilience.circuit_breaker.timeout", 60*time.Second)
	v.SetDefault("resilience.rate_limit.rps", 1000)
	v.SetDefault("resilience.rate_limit.burst", 2000)

	// plugin
	v.SetDefault("plugin.dir", "./plugins")
	v.SetDefault("plugin.extensions", []string{".so", ".yaml"})
	v.SetDefault("plugin.watch", true)
}

// MustEnv 返回环境变量值；若未设置或为空，返回默认值 def。
// 主要用于在 viper 体系之外读取零散开关的便捷函数。
func MustEnv(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
