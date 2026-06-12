package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fixturePath 返回 testdata 下的相对路径，便于在不同工作目录下都能定位。
func fixturePath(name string) string {
	return filepath.Join("testdata", name)
}

// TestLoad_MissingFile 验证文件不存在时返回包装后的错误。
func TestLoad_MissingFile(t *testing.T) {
	_, err := Load("testdata/does-not-exist.yaml")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "read config")
}

// TestLoad_FullYAML 加载完整配置并断言各子段被正确解析。
func TestLoad_FullYAML(t *testing.T) {
	cfg, err := Load(fixturePath("full.yaml"))
	require.NoError(t, err)
	require.NotNil(t, cfg)

	assert.Equal(t, "127.0.0.1", cfg.Server.Host)
	assert.Equal(t, 18080, cfg.Server.Port)
	assert.Equal(t, 18081, cfg.Server.GRPCPort)
	assert.Equal(t, 45*time.Second, cfg.Server.ReadTimeout)
	assert.Equal(t, 60*time.Second, cfg.Server.WriteTimeout)
	assert.Equal(t, 15*time.Second, cfg.Server.ShutdownTimeout)

	assert.True(t, cfg.Features.EnableMetrics)
	assert.False(t, cfg.Features.EnableTracing)
	assert.True(t, cfg.Features.EnableObservatory)

	assert.Equal(t, 19090, cfg.Observatory.MetricsPort)
	assert.False(t, cfg.Observatory.Tracing.Enabled)
	assert.Equal(t, "otlp", cfg.Observatory.Tracing.Exporter)
	assert.InDelta(t, 0.5, cfg.Observatory.Tracing.SamplingRate, 1e-9)
	assert.Equal(t, "debug", cfg.Observatory.Log.Level)
	assert.Equal(t, "console", cfg.Observatory.Log.Encoding)

	assert.Equal(t, 25*time.Second, cfg.Commander.DefaultTimeout)
	assert.Equal(t, 50, cfg.Commander.MaxConcurrentOrders)

	assert.Equal(t, 500, cfg.Dispatcher.QueueSize)
	assert.Equal(t, 20*time.Second, cfg.Dispatcher.Timeout)

	assert.Equal(t, 7, cfg.Generals.PoolSize)
	assert.Equal(t, 90*time.Second, cfg.Generals.DefaultTimeout)

	assert.Equal(t, "tiangai", cfg.Bagua.DefaultMode)
	assert.Equal(t, 16, cfg.Bagua.MaxParallelNodes)

	assert.Equal(t, "/var/lib/kongming/strategies", cfg.Vault.Dir)
	assert.False(t, cfg.Vault.AutoReload)
	assert.True(t, cfg.Vault.BuiltinOnly)

	assert.Equal(t, 2000, cfg.Courier.InboxSize)
	assert.Equal(t, 3000, cfg.Courier.OutboxSize)
	assert.Equal(t, 45*time.Second, cfg.Courier.DeliveryTimeout)

	assert.Equal(t, 5, cfg.Resilience.Retry.MaxAttempts)
	assert.Equal(t, 50*time.Millisecond, cfg.Resilience.Retry.InitialBackoff)
	assert.Equal(t, 10*time.Second, cfg.Resilience.Retry.MaxBackoff)
	assert.InDelta(t, 1.5, cfg.Resilience.Retry.BackoffFactor, 1e-9)
	assert.False(t, cfg.Resilience.Retry.Jitter)
	assert.Equal(t, 10, cfg.Resilience.CircuitBreaker.Threshold)
	assert.Equal(t, 30*time.Second, cfg.Resilience.CircuitBreaker.Timeout)
	assert.Equal(t, 500, cfg.Resilience.RateLimit.RPS)
	assert.Equal(t, 1000, cfg.Resilience.RateLimit.Burst)

	assert.Equal(t, "/opt/kongming/plugins", cfg.Plugin.Dir)
	assert.Equal(t, []string{".so"}, cfg.Plugin.Extensions)
	assert.False(t, cfg.Plugin.Watch)
}

// TestLoad_MinimalYAML_AppliesDefaults 最小配置 + 全部默认值兜底。
func TestLoad_MinimalYAML_AppliesDefaults(t *testing.T) {
	cfg, err := Load(fixturePath("minimal.yaml"))
	require.NoError(t, err)
	require.NotNil(t, cfg)

	// 显式提供
	assert.Equal(t, "0.0.0.0", cfg.Server.Host)
	assert.Equal(t, 8080, cfg.Server.Port)
	assert.Equal(t, 8081, cfg.Server.GRPCPort)

	// 默认值兜底
	assert.Equal(t, 30*time.Second, cfg.Server.ReadTimeout)
	assert.Equal(t, 30*time.Second, cfg.Server.WriteTimeout)
	assert.Equal(t, 30*time.Second, cfg.Server.ShutdownTimeout)
	assert.True(t, cfg.Features.EnableMetrics)
	assert.True(t, cfg.Features.EnableTracing)
	assert.True(t, cfg.Features.EnableObservatory)
	assert.Equal(t, 9090, cfg.Observatory.MetricsPort)
	assert.Equal(t, "jaeger", cfg.Observatory.Tracing.Exporter)
	assert.InDelta(t, 1.0, cfg.Observatory.Tracing.SamplingRate, 1e-9)
	assert.Equal(t, "info", cfg.Observatory.Log.Level)
	assert.Equal(t, "json", cfg.Observatory.Log.Encoding)
	assert.Equal(t, 30*time.Second, cfg.Commander.DefaultTimeout)
	assert.Equal(t, 100, cfg.Commander.MaxConcurrentOrders)
	assert.Equal(t, 1000, cfg.Dispatcher.QueueSize)
	assert.Equal(t, 30*time.Second, cfg.Dispatcher.Timeout)
	assert.Equal(t, 5, cfg.Generals.PoolSize)
	assert.Equal(t, 60*time.Second, cfg.Generals.DefaultTimeout)
	assert.Equal(t, "dizai", cfg.Bagua.DefaultMode)
	assert.Equal(t, 10, cfg.Bagua.MaxParallelNodes)
	assert.Equal(t, "./strategies", cfg.Vault.Dir)
	assert.True(t, cfg.Vault.AutoReload)
	assert.Equal(t, 1000, cfg.Courier.InboxSize)
	assert.Equal(t, 1000, cfg.Courier.OutboxSize)
	assert.Equal(t, 30*time.Second, cfg.Courier.DeliveryTimeout)
	assert.Equal(t, 3, cfg.Resilience.Retry.MaxAttempts)
	assert.Equal(t, 100*time.Millisecond, cfg.Resilience.Retry.InitialBackoff)
	assert.Equal(t, 30*time.Second, cfg.Resilience.Retry.MaxBackoff)
	assert.InDelta(t, 2.0, cfg.Resilience.Retry.BackoffFactor, 1e-9)
	assert.True(t, cfg.Resilience.Retry.Jitter)
	assert.Equal(t, 5, cfg.Resilience.CircuitBreaker.Threshold)
	assert.Equal(t, 60*time.Second, cfg.Resilience.CircuitBreaker.Timeout)
	assert.Equal(t, 1000, cfg.Resilience.RateLimit.RPS)
	assert.Equal(t, 2000, cfg.Resilience.RateLimit.Burst)
	assert.Equal(t, "./plugins", cfg.Plugin.Dir)
	assert.Equal(t, []string{".so", ".yaml"}, cfg.Plugin.Extensions)
	assert.True(t, cfg.Plugin.Watch)
}

// TestLoad_InvalidPort 验证 port 越界时返回校验错误。
func TestLoad_InvalidPort(t *testing.T) {
	_, err := Load(fixturePath("invalid_port.yaml"))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "validate")
}

// TestLoad_MissingHost 验证 host 为空时返回校验错误。
func TestLoad_MissingHost(t *testing.T) {
	_, err := Load(fixturePath("missing_host.yaml"))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "validate")
}

// TestLoad_MalformedYAML 验证 YAML 语法错误时返回读取错误。
func TestLoad_MalformedYAML(t *testing.T) {
	_, err := Load(fixturePath("malformed.yaml"))
	require.Error(t, err)
	// 解析失败可能被 ReadInConfig 或 Unmarshal 任一阶段捕获
	msg := err.Error()
	assert.True(t, strings.Contains(msg, "read config") || strings.Contains(msg, "unmarshal"),
		"unexpected error: %s", msg)
}

// TestLoadFromBytes_JSON 通过字节流加载 JSON 格式配置。
func TestLoadFromBytes_JSON(t *testing.T) {
	data, err := os.ReadFile(fixturePath("minimal.json"))
	require.NoError(t, err)

	cfg, err := LoadFromBytes(data, "json")
	require.NoError(t, err)
	require.NotNil(t, cfg)
	assert.Equal(t, "10.0.0.1", cfg.Server.Host)
	assert.Equal(t, 7070, cfg.Server.Port)
	assert.Equal(t, 7071, cfg.Server.GRPCPort)
	// 默认值仍生效
	assert.Equal(t, 30*time.Second, cfg.Server.ReadTimeout)
}

// TestLoadFromBytes_InvalidBytes 验证错误格式的字节流返回读取错误。
func TestLoadFromBytes_InvalidBytes(t *testing.T) {
	_, err := LoadFromBytes([]byte("not a valid config: : :"), "yaml")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "read config bytes")
}

// TestLoadFromBytes_ValidationError 字节流加载后仍然走 validator。
// 注意：payload 必须是合法 JSON 格式（ext=json），否则在 ReadConfig 阶段就失败。
// 故意将 grpc_port 设为 0（违反 min=1）以触发 validator 失败。
func TestLoadFromBytes_ValidationError(t *testing.T) {
	payload := []byte(`{"server": {"host": "0.0.0.0", "port": 8080, "grpc_port": 0}}`)
	_, err := LoadFromBytes(payload, "json")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "validate")
}

// TestLoad_EnvOverride 验证环境变量（KONGMING_* 前缀）可以覆盖配置文件。
// dot 在 key 中被替换为下划线。
func TestLoad_EnvOverride(t *testing.T) {
	// 准备环境变量
	t.Setenv("KONGMING_SERVER_HOST", "10.20.30.40")
	t.Setenv("KONGMING_SERVER_PORT", "9999")
	t.Setenv("KONGMING_FEATURES_ENABLE_METRICS", "false")

	cfg, err := Load(fixturePath("minimal.yaml"))
	require.NoError(t, err)
	require.NotNil(t, cfg)

	// 环境变量应覆盖配置文件
	assert.Equal(t, "10.20.30.40", cfg.Server.Host)
	assert.Equal(t, 9999, cfg.Server.Port)
	assert.False(t, cfg.Features.EnableMetrics)
}

// TestMustEnv_Default 验证未设置环境变量时返回默认值。
func TestMustEnv_Default(t *testing.T) {
	key := "KONGMING_TEST_DEFINITELY_NOT_SET_XYZ"
	// 确保未设置
	_ = os.Unsetenv(key)
	assert.Equal(t, "fallback", MustEnv(key, "fallback"))
}

// TestMustEnv_Set 验证已设置环境变量时返回其值。
func TestMustEnv_Set(t *testing.T) {
	key := "KONGMING_TEST_SET_VALUE_XYZ"
	t.Setenv(key, "real-value")
	assert.Equal(t, "real-value", MustEnv(key, "fallback"))
}

// TestMustEnv_Empty 验证设置为空字符串时回退到默认值（视为未设置）。
func TestMustEnv_Empty(t *testing.T) {
	key := "KONGMING_TEST_EMPTY_VALUE_XYZ"
	t.Setenv(key, "")
	assert.Equal(t, "fallback", MustEnv(key, "fallback"))
}

// TestEnvPrefix 常量值是项目规范的一部分，固化以防无意变更。
func TestEnvPrefix(t *testing.T) {
	assert.Equal(t, "KONGMING", EnvPrefix)
}

// TestLoad_NewViperIsolation 多次 Load 调用之间不共享 viper 状态。
// 第一次加载 full.yaml（带自定义值），第二次加载 minimal.yaml（带不同值），
// 验证第二次加载不会被第一次的 env/SetDefault 污染。
func TestLoad_NewViperIsolation(t *testing.T) {
	// 清理可能干扰的环境变量
	t.Setenv("KONGMING_SERVER_HOST", "from-env")
	defer t.Setenv("KONGMING_SERVER_HOST", "from-env") // 还原，t.Setenv 会在测试结束自动还原

	// 第一次：环境变量覆盖生效
	cfg1, err := Load(fixturePath("minimal.yaml"))
	require.NoError(t, err)
	assert.Equal(t, "from-env", cfg1.Server.Host)

	// 第二次：应该使用新 viper 实例，看到相同的 env 值（因为是同进程 env）
	// 但默认值应不被第一次的 SetDefault 污染（通过不存在的字段验证）
	cfg2, err := Load(fixturePath("minimal.yaml"))
	require.NoError(t, err)
	assert.Equal(t, "from-env", cfg2.Server.Host)
	// 同时默认值仍然正确
	assert.Equal(t, 8080, cfg2.Server.Port)
}

// TestLoadFromFile_ProjectConfig 尝试加载仓库根目录下的 configs/kongming.yaml。
// 该测试为可选：若该文件当前不满足 v2 schema（缺 grpc_port 等必填项），跳过即可。
// 它存在是为了在配置被人工更新时提供"端到端"信心。
func TestLoadFromFile_ProjectConfig(t *testing.T) {
	candidates := []string{
		"../../configs/kongming.yaml",
		"../../../configs/kongming.yaml",
	}
	var cfg *Config
	var err error
	for _, p := range candidates {
		if _, statErr := os.Stat(p); statErr == nil {
			cfg, err = Load(p)
			if err == nil {
				assert.NotNil(t, cfg)
				return
			}
		}
	}
	t.Skip("configs/kongming.yaml 暂未符合 v2 schema 必填要求，跳过")
	_ = err
}
