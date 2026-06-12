# Kongming 重构实施计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 把现有 8 个 pkg 模块按"三层架构"（domain/application/transport/infra）重构，引入配置驱动、插件化 SPI、弹性执行、可观测性，达到 80% 单元测试覆盖率门槛。

**Architecture:** 三层架构 + 端口适配器。`domain/` 零外部依赖；`application/` 编排端口；`infra/` 实现端口（viper/prom/otel/plugin/redis 占位）；`transport/` 提供 HTTP/gRPC/CLI；顶层 `pkg/kongming` 装配 + `cmd/{kongming-server,kongming}` 双入口。

**Tech Stack:** Go 1.21+ / cobra / gin / viper / gRPC / zap / prometheus / opentelemetry / fsnotify / testify / gomock / go-playground/validator

**依赖 spec:** [2026-06-12-kongming-refactor-design.md](file:///workspace/docs/superpowers/specs/2026-06-12-kongming-refactor-design.md)

---

## 文件结构总览

### 新增（pkg/）
- `pkg/kongming/kongming.go`（顶层装配）
- `pkg/domain/{model,port,errors}/*.go`
- `pkg/application/{commander,dispatcher,workflow,general,vault,courier}/*.go`
- `pkg/transport/{http,grpc,cli}/**/*.go`
- `pkg/infra/{config,observability,resilience,persistence,plugin}/**/*.go`

### 新增（cmd/）
- `cmd/kongming-server/main.go`（替换 `cmd/kongming/main.go`）
- `cmd/kongming/main.go`（CLI 入口）

### 新增（api/）
- `api/proto/kongming/v1/kongming.proto` + 生成代码

### 新增（docs/）
- `docs/MIGRATION.md`（v1→v2 迁移指南）

### 修改
- `go.mod`（新增 cobra/gin/viper/grpc/fsnotify/validator/gomock）
- `Makefile`（新增 `make proto / make coverage / make lint`）
- `.github/workflows/ci.yml`（新增 coverage gate + fuzz + proto 校验）
- `configs/kongming.yaml`（按新 schema 重写）
- `README.md`（更新架构图与使用说明）

### 删除（迁移完成后）
- `pkg/bagua/` `pkg/cmd_center/` `pkg/courier/` `pkg/dispatch/`
  `pkg/generals/` `pkg/observatory/` `pkg/repeater/` `pkg/strategy_vault/`
- `internal/memory/`
- `cmd/kongming/main.go`（被 `cmd/kongming-server/` + `cmd/kongming/` 替换）

---

## 阶段 0 — 工程脚手架

### Task 0.1：go.mod 依赖升级

**Files:**
- Modify: `go.mod`
- Modify: `go.sum`
- Create: `Makefile`（重写）

- [ ] **Step 1：更新 go.mod 依赖**

```bash
go get github.com/spf13/cobra@latest
go get github.com/spf13/viper@latest
go get github.com/gin-gonic/gin@latest
go get google.golang.org/grpc@latest
go get google.golang.org/protobuf@latest
go get github.com/fsnotify/fsnotify@latest
go get github.com/go-playground/validator/v10@latest
go get go.uber.org/mock@latest
go get github.com/stretchr/testify@latest
go get google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest
go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest
go mod tidy
```

- [ ] **Step 2：运行 go build 确保依赖可用**

```bash
go build ./...
```
Expected: 0 errors, 0 warnings

- [ ] **Step 3：Commit**

```bash
git add go.mod go.sum
git commit -m "chore(deps): add cobra gin viper grpc fsnotify validator gomock"
```

### Task 0.2：目录骨架创建

**Files:**
- Create: `pkg/kongming/.gitkeep`、`pkg/domain/{model,port,errors}/.gitkeep`
- Create: `pkg/application/{commander,dispatcher,workflow,general,vault,courier}/.gitkeep`
- Create: `pkg/transport/{http,grpc,cli}/{middleware,handler,service,interceptor,dto}/.gitkeep`
- Create: `pkg/infra/{config,observability,resilience,persistence,plugin}/.gitkeep`
- Create: `cmd/{kongming-server,kongming}/.gitkeep`
- Create: `api/proto/kongming/v1/.gitkeep`

- [ ] **Step 1：批量创建目录**

```bash
mkdir -p pkg/kongming \
  pkg/domain/{model,port,errors} \
  pkg/application/{commander,dispatcher,workflow/modes,workflow/node,general/wuhu,vault/builtin,courier} \
  pkg/transport/http/{middleware,handler,dto} \
  pkg/transport/grpc/{service,interceptor} \
  pkg/transport/cli \
  pkg/infra/{config,observability,resilience,persistence/memory,plugin} \
  cmd/kongming-server cmd/kongming \
  api/proto/kongming/v1
```

- [ ] **Step 2：commit**

```bash
git add .
git commit -m "chore(scaffold): create three-tier directory skeleton"
```

### Task 0.3：proto 工具链与第一个 .proto

**Files:**
- Create: `api/proto/kongming/v1/kongming.proto`
- Create: `Makefile`（添加 proto 任务）
- Create: `api/proto/buf.gen.yaml`

- [ ] **Step 1：写 proto**

```proto
// api/proto/kongming/v1/kongming.proto
syntax = "proto3";
package kongming.v1;
option go_package = "github.com/zhuge/kongming/api/proto/kongming/v1;kongmingv1";

import "google/protobuf/timestamp.proto";

service Kongming {
  rpc Dispatch(DispatchRequest) returns (DispatchResponse);
  rpc GetOrder(GetOrderRequest) returns (Order);
  rpc ListOrders(ListOrdersRequest) returns (ListOrdersResponse);
  rpc ListGenerals(ListGeneralsRequest) returns (ListGeneralsResponse);
  rpc ListJinnang(ListJinnangRequest) returns (ListJinnangResponse);
  rpc ExecuteJinnang(ExecuteJinnangRequest) returns (ExecuteJinnangResponse);
  rpc RunWorkflow(RunWorkflowRequest) returns (RunWorkflowResponse);
}

message DispatchRequest {
  string name = 1;
  string description = 2;
  int32 priority = 3;
  map<string, string> context = 4;
}

message DispatchResponse {
  string order_id = 1;
  bool success = 2;
  string message = 3;
  string report_json = 4; // 序列化后的 BattleReport
}

message GetOrderRequest { string id = 1; }

message Order {
  string id = 1;
  string name = 2;
  int32 state = 3;
  int32 priority = 4;
  google.protobuf.Timestamp created_at = 5;
}

message ListOrdersRequest { int32 state_filter = 1; }
message ListOrdersResponse { repeated Order orders = 1; }

message ListGeneralsRequest {}
message General {
  string id = 1;
  string name = 2;
  string title = 3;
  repeated string skills = 4;
  int32 state = 5;
}
message ListGeneralsResponse { repeated General generals = 1; }

message ListJinnangRequest {}
message Jinnang {
  string id = 1;
  string name = 2;
  int32 type = 3;
  string version = 4;
}
message ListJinnangResponse { repeated Jinnang jinnangs = 1; }

message ExecuteJinnangRequest {
  string id = 1;
  map<string, string> params = 2;
  bytes data = 3;
}
message ExecuteJinnangResponse {
  bool success = 1;
  bytes data = 2;
  string error = 3;
}

message RunWorkflowRequest { string workflow_id = 1; map<string, string> inputs = 2; }
message RunWorkflowResponse {
  bool success = 1;
  string error = 2;
  map<string, string> node_states = 3;
}
```

- [ ] **Step 2：buf.gen.yaml**

```yaml
# api/proto/buf.gen.yaml
version: v1
plugins:
  - plugin: go
    out: ../..
    opt: paths=source_relative
  - plugin: go-grpc
    out: ../..
    opt: paths=source_relative
```

- [ ] **Step 3：Makefile 添加 proto 任务（替换原 Makefile）**

```makefile
.PHONY: build test cover lint fmt clean ci proto proto-lint help

BINARY_NAME=kongming
SERVER_NAME=kongming-server
BUILD_DIR=./bin
GO=go
GOFLAGS=-ldflags="-s -w"

build:
	@echo "Building..."
	@mkdir -p $(BUILD_DIR)
	$(GO) build $(GOFLAGS) -o $(BUILD_DIR)/$(SERVER_NAME) ./cmd/kongming-server
	$(GO) build $(GOFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME) ./cmd/kongming

test:
	@echo "Running tests..."
	$(GO) test -v -race -cover ./...

cover:
	@echo "Coverage report..."
	$(GO) test -coverprofile=coverage.out ./...
	$(GO) tool cover -html=coverage.out -o coverage.html

coverage-gate:
	@echo "Coverage gate..."
	@$(GO) test -coverprofile=coverage.out ./... > /dev/null 2>&1
	@COV=$$($(GO) tool cover -func=coverage.out | tail -1 | awk '{print $$3}' | tr -d '%'); \
	echo "Coverage: $$COV%"; \
	if [ $${COV%.*} -lt 80 ]; then echo "FAIL: < 80%"; exit 1; fi

lint:
	@echo "Running linters..."
	golangci-lint run --timeout=5m ./...

fmt:
	@echo "Formatting..."
	$(GO) fmt ./...
	gofumpt -w .

clean:
	@echo "Cleaning..."
	rm -rf $(BUILD_DIR) coverage.out coverage.html

proto:
	@echo "Generating proto..."
	cd api/proto && buf generate

ci: fmt lint coverage-gate build

help:
	@echo "make build         - Build all binaries"
	@echo "make test          - Run all tests with race"
	@echo "make cover         - HTML coverage report"
	@echo "make coverage-gate - Enforce 80% coverage"
	@echo "make lint          - Run golangci-lint"
	@echo "make fmt           - Format code"
	@echo "make proto         - Generate gRPC code"
	@echo "make ci            - Full CI pipeline"
```

- [ ] **Step 4：生成 proto 代码并 commit**

```bash
go install github.com/bufbuild/buf/cmd/buf@latest
make proto
git add api/
git commit -m "feat(proto): add kongming gRPC service definition"
```

---

## 阶段 1 — 基础设施层（pkg/infra）

### Task 1.1：config（viper 加载）

**Files:**
- Create: `pkg/infra/config/schema.go`
- Create: `pkg/infra/config/loader.go`
- Test: `pkg/infra/config/loader_test.go`

- [ ] **Step 1：写失败测试**

```go
// pkg/infra/config/loader_test.go
package config

import (
    "testing"
    "time"
    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"
)

func TestLoadFromFile(t *testing.T) {
    cfg, err := Load("../../configs/kongming.yaml")
    require.NoError(t, err)
    assert.Equal(t, "0.0.0.0", cfg.Server.Host)
    assert.Equal(t, 8080, cfg.Server.Port)
    assert.Equal(t, 30*time.Second, cfg.Server.ReadTimeout)
    assert.True(t, cfg.Features.EnableMetrics)
}

func TestLoadMissingFile(t *testing.T) {
    _, err := Load("not-exist.yaml")
    assert.Error(t, err)
}
```

- [ ] **Step 2：运行测试确认失败**

```bash
go test ./pkg/infra/config/...
```
Expected: FAIL (package does not exist)

- [ ] **Step 3：写 schema.go**

```go
// pkg/infra/config/schema.go
package config

import "time"

type Config struct {
    Server       ServerConfig       `mapstructure:"server" validate:"required"`
    Features     FeaturesConfig     `mapstructure:"features"`
    Observatory  ObservatoryConfig  `mapstructure:"observatory" validate:"required"`
    Commander    CommanderConfig    `mapstructure:"commander" validate:"required"`
    Dispatcher   DispatcherConfig   `mapstructure:"dispatcher"`
    Generals     GeneralsConfig     `mapstructure:"generals"`
    Bagua        BaguaConfig        `mapstructure:"bagua"`
    Vault        VaultConfig        `mapstructure:"vault"`
    Courier      CourierConfig      `mapstructure:"courier"`
    Resilience   ResilienceConfig   `mapstructure:"resilience"`
    Plugin       PluginConfig       `mapstructure:"plugin"`
}

type ServerConfig struct {
    Host            string        `mapstructure:"host" validate:"required"`
    Port            int           `mapstructure:"port" validate:"required,min=1,max=65535"`
    GRPCPort        int           `mapstructure:"grpc_port" validate:"required,min=1,max=65535"`
    ReadTimeout     time.Duration `mapstructure:"read_timeout"`
    WriteTimeout    time.Duration `mapstructure:"write_timeout"`
    ShutdownTimeout time.Duration `mapstructure:"shutdown_timeout"`
}

type FeaturesConfig struct {
    EnableMetrics     bool `mapstructure:"enable_metrics"`
    EnableTracing     bool `mapstructure:"enable_tracing"`
    EnableObservatory bool `mapstructure:"enable_observatory"`
}

type ObservatoryConfig struct {
    MetricsPort int             `mapstructure:"metrics_port"`
    Tracing     TracingConfig   `mapstructure:"tracing"`
    Log         LogConfig       `mapstructure:"log"`
}

type TracingConfig struct {
    Enabled      bool    `mapstructure:"enabled"`
    Endpoint     string  `mapstructure:"endpoint"`
    SamplingRate float64 `mapstructure:"sampling_rate"`
    Exporter     string  `mapstructure:"exporter"` // jaeger | otlp
}

type LogConfig struct {
    Level    string `mapstructure:"level"`
    Encoding string `mapstructure:"encoding"`
}

type CommanderConfig struct {
    DefaultTimeout     time.Duration `mapstructure:"default_timeout"`
    MaxConcurrentOrders int           `mapstructure:"max_concurrent_orders"`
}

type DispatcherConfig struct {
    QueueSize int           `mapstructure:"queue_size"`
    Timeout   time.Duration `mapstructure:"timeout"`
}

type GeneralsConfig struct {
    PoolSize      int           `mapstructure:"pool_size"`
    DefaultTimeout time.Duration `mapstructure:"default_timeout"`
}

type BaguaConfig struct {
    DefaultMode      string `mapstructure:"default_mode"`
    MaxParallelNodes int    `mapstructure:"max_parallel_nodes"`
}

type VaultConfig struct {
    Dir         string `mapstructure:"dir"`
    AutoReload  bool   `mapstructure:"auto_reload"`
    BuiltinOnly bool   `mapstructure:"builtin_only"`
}

type CourierConfig struct {
    InboxSize       int           `mapstructure:"inbox_size"`
    OutboxSize      int           `mapstructure:"outbox_size"`
    DeliveryTimeout time.Duration `mapstructure:"delivery_timeout"`
}

type ResilienceConfig struct {
    Retry         RetryConfig         `mapstructure:"retry"`
    CircuitBreaker CircuitBreakerConfig `mapstructure:"circuit_breaker"`
    RateLimit     RateLimitConfig     `mapstructure:"rate_limit"`
}

type RetryConfig struct {
    MaxAttempts    int           `mapstructure:"max_attempts"`
    InitialBackoff time.Duration `mapstructure:"initial_backoff"`
    MaxBackoff     time.Duration `mapstructure:"max_backoff"`
    BackoffFactor  float64       `mapstructure:"backoff_factor"`
    Jitter         bool          `mapstructure:"jitter"`
}

type CircuitBreakerConfig struct {
    Threshold int           `mapstructure:"threshold"`
    Timeout   time.Duration `mapstructure:"timeout"`
}

type RateLimitConfig struct {
    RPS   int `mapstructure:"rps"`
    Burst int `mapstructure:"burst"`
}

type PluginConfig struct {
    Dir        string   `mapstructure:"dir"`
    Extensions []string `mapstructure:"extensions"`
    Watch      bool     `mapstructure:"watch"`
}
```

- [ ] **Step 4：写 loader.go**

```go
// pkg/infra/config/loader.go
package config

import (
    "fmt"
    "os"
    "strings"
    "time"
    "github.com/go-playground/validator/v10"
    "github.com/spf13/viper"
)

var v = validator.New(validator.WithRequiredStructEnabled())

func Load(path string) (*Config, error) {
    viper.SetConfigFile(path)
    viper.SetEnvPrefix("KONGMING")
    viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
    viper.AutomaticEnv()

    setDefaults()

    if err := viper.ReadInConfig(); err != nil {
        return nil, fmt.Errorf("read config: %w", err)
    }

    var cfg Config
    if err := viper.Unmarshal(&cfg); err != nil {
        return nil, fmt.Errorf("unmarshal: %w", err)
    }
    if err := v.Struct(&cfg); err != nil {
        return nil, fmt.Errorf("validate: %w", err)
    }
    return &cfg, nil
}

func LoadFromBytes(data []byte, ext string) (*Config, error) {
    viper.SetConfigType(ext)
    if err := viper.ReadConfig(strings.NewReader(string(data))); err != nil {
        return nil, err
    }
    var cfg Config
    if err := viper.Unmarshal(&cfg); err != nil {
        return nil, err
    }
    return &cfg, v.Struct(&cfg)
}

func setDefaults() {
    viper.SetDefault("server.host", "0.0.0.0")
    viper.SetDefault("server.port", 8080)
    viper.SetDefault("server.grpc_port", 8081)
    viper.SetDefault("server.read_timeout", 30*time.Second)
    viper.SetDefault("server.write_timeout", 30*time.Second)
    viper.SetDefault("server.shutdown_timeout", 30*time.Second)
    viper.SetDefault("features.enable_metrics", true)
    viper.SetDefault("features.enable_tracing", true)
    viper.SetDefault("features.enable_observatory", true)
    viper.SetDefault("observatory.metrics_port", 9090)
    viper.SetDefault("observatory.tracing.exporter", "jaeger")
    viper.SetDefault("observatory.tracing.sampling_rate", 1.0)
    viper.SetDefault("observatory.log.level", "info")
    viper.SetDefault("observatory.log.encoding", "json")
    viper.SetDefault("commander.default_timeout", 30*time.Second)
    viper.SetDefault("commander.max_concurrent_orders", 100)
    viper.SetDefault("dispatcher.queue_size", 1000)
    viper.SetDefault("dispatcher.timeout", 30*time.Second)
    viper.SetDefault("generals.pool_size", 5)
    viper.SetDefault("generals.default_timeout", 60*time.Second)
    viper.SetDefault("bagua.default_mode", "dizai")
    viper.SetDefault("bagua.max_parallel_nodes", 10)
    viper.SetDefault("vault.dir", "./strategies")
    viper.SetDefault("vault.auto_reload", true)
    viper.SetDefault("courier.inbox_size", 1000)
    viper.SetDefault("courier.outbox_size", 1000)
    viper.SetDefault("courier.delivery_timeout", 30*time.Second)
    viper.SetDefault("resilience.retry.max_attempts", 3)
    viper.SetDefault("resilience.retry.initial_backoff", 100*time.Millisecond)
    viper.SetDefault("resilience.retry.max_backoff", 30*time.Second)
    viper.SetDefault("resilience.retry.backoff_factor", 2.0)
    viper.SetDefault("resilience.retry.jitter", true)
    viper.SetDefault("resilience.circuit_breaker.threshold", 5)
    viper.SetDefault("resilience.circuit_breaker.timeout", 60*time.Second)
    viper.SetDefault("resilience.rate_limit.rps", 1000)
    viper.SetDefault("resilience.rate_limit.burst", 2000)
    viper.SetDefault("plugin.dir", "./plugins")
    viper.SetDefault("plugin.extensions", []string{".so", ".yaml"})
    viper.SetDefault("plugin.watch", true)
}

// MustEnv returns the value of an env var or default.
func MustEnv(key, def string) string {
    if v := os.Getenv(key); v != "" { return v }
    return def
}
```

- [ ] **Step 5：写测试通过**

```bash
go test ./pkg/infra/config/... -v
```
Expected: PASS

- [ ] **Step 6：commit**

```bash
git add pkg/infra/config/
git commit -m "feat(infra/config): viper loader with struct tag + validator"
```

### Task 1.2：observability — logger

**Files:**
- Create: `pkg/infra/observability/logger.go`
- Test: `pkg/infra/observability/logger_test.go`

- [ ] **Step 1：写失败测试**

```go
// pkg/infra/observability/logger_test.go
package observability

import (
    "testing"
    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"
    "go.uber.org/zap"
    "go.uber.org/zap/zapcore"
)

func TestNewLogger_Info(t *testing.T) {
    cfg := LogConfig{Level: "info", Encoding: "json"}
    logger, err := NewLogger(cfg)
    require.NoError(t, err)
    assert.NotNil(t, logger)
    assert.IsType(t, &zap.Logger{}, logger)
}

func TestNewLogger_InvalidLevel(t *testing.T) {
    cfg := LogConfig{Level: "invalid", Encoding: "json"}
    _, err := NewLogger(cfg)
    assert.Error(t, err)
}

func TestParseLevel(t *testing.T) {
    assert.Equal(t, zapcore.InfoLevel, ParseLevel("info"))
    assert.Equal(t, zapcore.DebugLevel, ParseLevel("debug"))
    assert.Equal(t, zapcore.WarnLevel, ParseLevel("warn"))
    assert.Equal(t, zapcore.ErrorLevel, ParseLevel("error"))
    assert.Equal(t, zapcore.InfoLevel, ParseLevel("unknown"))
}
```

- [ ] **Step 2：写 logger.go**

```go
// pkg/infra/observability/logger.go
package observability

import (
    "fmt"
    "go.uber.org/zap"
    "go.uber.org/zap/zapcore"
)

type LogConfig = config.LogConfig // alias

func NewLogger(cfg LogConfig) (*zap.Logger, error) {
    lvl := ParseLevel(cfg.Level)
    enc := cfg.Encoding
    if enc == "" { enc = "json" }

    zcfg := zap.Config{
        Level:             zap.NewAtomicLevelAt(lvl),
        Development:       false,
        Encoding:          enc,
        EncoderConfig:     zapcore.EncoderConfig{
            TimeKey:        "ts",
            LevelKey:       "level",
            NameKey:        "logger",
            CallerKey:      "caller",
            FunctionKey:    zapcore.OmitKey,
            MessageKey:     "msg",
            StacktraceKey:  "stacktrace",
            LineEnding:     zapcore.DefaultLineEnding,
            EncodeLevel:    zapcore.LowercaseLevelEncoder,
            EncodeTime:     zapcore.ISO8601TimeEncoder,
            EncodeDuration: zapcore.SecondsDurationEncoder,
            EncodeCaller:   zapcore.ShortCallerEncoder,
        },
        OutputPaths:      []string{"stdout"},
        ErrorOutputPaths: []string{"stderr"},
    }
    logger, err := zcfg.Build()
    if err != nil { return nil, fmt.Errorf("build logger: %w", err) }
    return logger, nil
}

func ParseLevel(level string) zapcore.Level {
    var l zapcore.Level
    if err := l.UnmarshalText([]byte(level)); err != nil {
        return zapcore.InfoLevel
    }
    return l
}
```

- [ ] **Step 3：commit**

```bash
git add pkg/infra/observability/logger.go pkg/infra/observability/logger_test.go
git commit -m "feat(infra/observability): zap logger with level parsing"
```

> **注意**：上面 logger.go 引用 `config.LogConfig`，需在 `pkg/infra/observability/logger.go` 顶部加 `import "github.com/zhuge/kongming/pkg/infra/config"` 并去掉 alias。修正：
> ```go
> import "github.com/zhuge/kongming/pkg/infra/config"
> // ...
> func NewLogger(cfg config.LogConfig) (*zap.Logger, error) { ... }
> ```

### Task 1.3：observability — traceId + observer 骨架

**Files:**
- Create: `pkg/domain/port/observer.go`
- Create: `pkg/infra/observability/traceid.go`
- Create: `pkg/infra/observability/observer.go`
- Test: `pkg/infra/observability/traceid_test.go`

- [ ] **Step 1：写 port/observer.go（domain 层）**

```go
// pkg/domain/port/observer.go
package port

import (
    "context"
    "go.opentelemetry.io/otel/attribute"
    "go.opentelemetry.io/otel/trace"
)

type Observer interface {
    StartSpan(ctx context.Context, name string, attrs ...attribute.KeyValue) (context.Context, trace.Span)
    RecordError(span trace.Span, err error)
    RecordEvent(ctx context.Context, name string, attrs ...attribute.KeyValue)
    IncCounter(name string, labels map[string]string)
    ObserveHistogram(name string, value float64, labels map[string]string)
    SetGauge(name string, value float64, labels map[string]string)
    Shutdown(ctx context.Context) error
}
```

- [ ] **Step 2：写 traceid.go**

```go
// pkg/infra/observability/traceid.go
package observability

import (
    "context"
    "github.com/google/uuid"
)

type traceIDKey struct{}

func NewTraceIDContext(ctx context.Context, id string) context.Context {
    return context.WithValue(ctx, traceIDKey{}, id)
}

func FromTraceIDContext(ctx context.Context) string {
    if v, ok := ctx.Value(traceIDKey{}).(string); ok { return v }
    return ""
}

func NewTraceID() string {
    return uuid.NewString()
}
```

- [ ] **Step 3：写测试**

```go
// pkg/infra/observability/traceid_test.go
package observability

import (
    "context"
    "testing"
    "github.com/stretchr/testify/assert"
)

func TestNewTraceIDContext(t *testing.T) {
    ctx := NewTraceIDContext(context.Background(), "abc")
    assert.Equal(t, "abc", FromTraceIDContext(ctx))
}

func TestFromEmptyContext(t *testing.T) {
    assert.Equal(t, "", FromTraceIDContext(context.Background()))
}

func TestNewTraceID(t *testing.T) {
    id1 := NewTraceID()
    id2 := NewTraceID()
    assert.NotEqual(t, id1, id2)
    assert.NotEmpty(t, id1)
}
```

- [ ] **Step 4：commit**

```bash
git add pkg/domain/port/observer.go pkg/infra/observability/traceid.go pkg/infra/observability/traceid_test.go
git commit -m "feat(observability): traceId ctx + Observer port interface"
```

### Task 1.4：observability — Prometheus metrics

**Files:**
- Create: `pkg/infra/observability/metrics.go`
- Test: `pkg/infra/observability/metrics_test.go`

- [ ] **Step 1：写失败测试**

```go
// pkg/infra/observability/metrics_test.go
package observability

import (
    "testing"
    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"
    "github.com/prometheus/client_golang/prometheus"
    dto "github.com/prometheus/client_model/go"
)

func TestMetrics_IncCounter(t *testing.T) {
    reg := prometheus.NewRegistry()
    m := newMetrics(reg)

    m.IncCounter("kongming_test_total", map[string]string{"status": "ok"})
    m.IncCounter("kongming_test_total", map[string]string{"status": "ok"})
    m.IncCounter("kongming_test_total", map[string]string{"status": "fail"})

    families, err := reg.Gather()
    require.NoError(t, err)
    var found *dto.MetricFamily
    for _, f := range families {
        if f.GetName() == "kongming_test_total" { found = f; break }
    }
    require.NotNil(t, found)
    assert.Equal(t, 2, int(found.Metric[0].Counter.GetValue()))
    assert.Equal(t, 1, int(found.Metric[1].Counter.GetValue()))
}

func TestMetrics_Histogram(t *testing.T) {
    reg := prometheus.NewRegistry()
    m := newMetrics(reg)
    m.ObserveHistogram("kongming_test_dur", 0.1, nil)
    m.ObserveHistogram("kongming_test_dur", 0.5, nil)
    families, _ := reg.Gather()
    var found *dto.MetricFamily
    for _, f := range families {
        if f.GetName() == "kongming_test_dur_seconds" { found = f; break }
    }
    require.NotNil(t, found)
    assert.Equal(t, uint64(2), found.Metric[0].Histogram.GetSampleCount())
}
```

- [ ] **Step 2：写 metrics.go**

```go
// pkg/infra/observability/metrics.go
package observability

import (
    "github.com/prometheus/client_golang/prometheus"
)

type Metrics struct {
    registry         *prometheus.Registry
    counterFactories map[string]*prometheus.CounterVec
    histFactories    map[string]*prometheus.HistogramVec
    gaugeFactories   map[string]*prometheus.GaugeVec
}

func newMetrics(reg *prometheus.Registry) *Metrics {
    return &Metrics{
        registry:         reg,
        counterFactories: make(map[string]*prometheus.CounterVec),
        histFactories:    make(map[string]*prometheus.HistogramVec),
        gaugeFactories:   make(map[string]*prometheus.GaugeVec),
    }
}

func (m *Metrics) IncCounter(name string, labels map[string]string) {
    cv, ok := m.counterFactories[name]
    if !ok {
        cv = prometheus.NewCounterVec(prometheus.CounterOpts{Name: name}, labelKeys(labels))
        m.registry.MustRegister(cv)
        m.counterFactories[name] = cv
    }
    cv.With(labels).Inc()
}

func (m *Metrics) ObserveHistogram(name string, value float64, labels map[string]string) {
    full := name + "_seconds"
    hv, ok := m.histFactories[full]
    if !ok {
        hv = prometheus.NewHistogramVec(prometheus.HistogramOpts{Name: full, Buckets: prometheus.DefBuckets}, labelKeys(labels))
        m.registry.MustRegister(hv)
        m.histFactories[full] = hv
    }
    hv.With(labels).Observe(value)
}

func (m *Metrics) SetGauge(name string, value float64, labels map[string]string) {
    gv, ok := m.gaugeFactories[name]
    if !ok {
        gv = prometheus.NewGaugeVec(prometheus.GaugeOpts{Name: name}, labelKeys(labels))
        m.registry.MustRegister(gv)
        m.gaugeFactories[name] = gv
    }
    gv.With(labels).Set(value)
}

func labelKeys(labels map[string]string) []string {
    keys := make([]string, 0, len(labels))
    for k := range labels { keys = append(keys, k) }
    return keys
}
```

- [ ] **Step 3：commit**

```bash
git add pkg/infra/observability/metrics.go pkg/infra/observability/metrics_test.go
git commit -m "feat(observability): lazy-registered prom counter/histogram/gauge"
```

### Task 1.5：observability — tracing（OTel + Jaeger/OTLP）

**Files:**
- Create: `pkg/infra/observability/tracing.go`
- Test: `pkg/infra/observability/tracing_test.go`

- [ ] **Step 1：写测试**

```go
// pkg/infra/observability/tracing_test.go
package observability

import (
    "context"
    "testing"
    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"
    "go.opentelemetry.io/otel/attribute"
)

func TestStartSpan_NoExporter(t *testing.T) {
    obs, err := NewObserver(context.Background(), config.ObservatoryConfig{
        Tracing: config.TracingConfig{Enabled: false},
    }, zap.NewNop())
    require.NoError(t, err)
    defer obs.Shutdown(context.Background())

    ctx, span := obs.StartSpan(context.Background(), "test")
    defer span.End()
    assert.NotNil(t, span)
    span.SetAttributes(attribute.String("k", "v"))
}
```

- [ ] **Step 2：写 tracing.go（含 observer 完整实现）**

```go
// pkg/infra/observability/observer.go
package observability

import (
    "context"
    "fmt"
    "github.com/prometheus/client_golang/prometheus"
    "github.com/prometheus/client_golang/prometheus/promhttp"
    "github.com/zhuge/kongming/pkg/domain/port"
    "github.com/zhuge/kongming/pkg/infra/config"
    "go.opentelemetry.io/otel"
    "go.opentelemetry.io/otel/attribute"
    "go.opentelemetry.io/otel/exporters/jaeger"
    "go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
    "go.opentelemetry.io/otel/sdk/resource"
    sdktrace "go.opentelemetry.io/otel/sdk/trace"
    "go.opentelemetry.io/otel/semconv/v1.21.0"
    "go.opentelemetry.io/otel/trace"
    "go.uber.org/zap"
    "net/http"
)

type Observer struct {
    logger     *zap.Logger
    metrics    *Metrics
    registry   *prometheus.Registry
    tracer     trace.Tracer
    provider   *sdktrace.TracerProvider
    metricsPort int
}

func NewObserver(ctx context.Context, cfg config.ObservatoryConfig, logger *zap.Logger) (*Observer, error) {
    reg := prometheus.NewRegistry()
    m := newMetrics(reg)

    obs := &Observer{
        logger:      logger,
        metrics:     m,
        registry:    reg,
        metricsPort: cfg.MetricsPort,
        tracer:      otel.Tracer("kongming"),
    }

    if cfg.Tracing.Enabled {
        if err := obs.initTracing(ctx, cfg.Tracing); err != nil {
            return nil, fmt.Errorf("init tracing: %w", err)
        }
    }
    return obs, nil
}

func (o *Observer) initTracing(ctx context.Context, cfg config.TracingConfig) error {
    var exporter sdktrace.SpanExporter
    var err error
    switch cfg.Exporter {
    case "otlp":
        exporter, err = otlptracehttp.New(ctx, otlptracehttp.WithEndpoint(cfg.Endpoint))
    default: // jaeger
        exporter, err = jaeger.New(jaeger.WithCollectorEndpoint(jaeger.WithEndpoint(cfg.Endpoint)))
    }
    if err != nil { return err }

    res, err := resource.New(ctx, resource.WithAttributes(
        semconv.ServiceName("kongming"),
        semconv.ServiceVersion("1.0.0"),
    ))
    if err != nil { return err }

    sampler := sdktrace.ParentBased(sdktrace.TraceIDRatioBased(cfg.SamplingRate))
    o.provider = sdktrace.NewTracerProvider(
        sdktrace.WithBatcher(exporter),
        sdktrace.WithResource(res),
        sdktrace.WithSampler(sampler),
    )
    otel.SetTracerProvider(o.provider)
    o.tracer = o.provider.Tracer("kongming")
    return nil
}

func (o *Observer) StartSpan(ctx context.Context, name string, attrs ...attribute.KeyValue) (context.Context, trace.Span) {
    return o.tracer.Start(ctx, name, trace.WithAttributes(attrs...))
}

func (o *Observer) RecordError(span trace.Span, err error) {
    span.RecordError(err)
    span.SetAttributes(attribute.Bool("error", true))
}

func (o *Observer) RecordEvent(ctx context.Context, name string, attrs ...attribute.KeyValue) {
    span := trace.SpanFromContext(ctx)
    span.AddEvent(name, trace.WithAttributes(attrs...))
}

func (o *Observer) IncCounter(name string, labels map[string]string) { o.metrics.IncCounter(name, labels) }
func (o *Observer) ObserveHistogram(name string, value float64, labels map[string]string) {
    o.metrics.ObserveHistogram(name, value, labels)
}
func (o *Observer) SetGauge(name string, value float64, labels map[string]string) {
    o.metrics.SetGauge(name, value, labels)
}

func (o *Observer) Handler() http.Handler { return promhttp.HandlerFor(o.registry, promhttp.HandlerOpts{}) }
func (o *Observer) Registry() *prometheus.Registry { return o.registry }

func (o *Observer) Shutdown(ctx context.Context) error {
    if o.provider != nil { return o.provider.Shutdown(ctx) }
    return nil
}

var _ port.Observer = (*Observer)(nil)
```

- [ ] **Step 3：commit**

```bash
git add pkg/infra/observability/
git commit -m "feat(observability): OTel tracer + lazy prom metrics + http handler"
```

### Task 1.6：resilience — retry / circuitbreaker / ratelimit / timeout

**Files:**
- Create: `pkg/domain/port/resilience.go`
- Create: `pkg/infra/resilience/{retry.go,circuitbreaker.go,ratelimit.go,timeout.go,runner.go}`
- Test: `pkg/infra/resilience/*_test.go`

- [ ] **Step 1：写 port/resilience.go**

```go
// pkg/domain/port/resilience.go
package port

import "context"

type ResilientRunner interface {
    Run(ctx context.Context, name string, fn func(ctx context.Context) error) error
    RunWithResult(ctx context.Context, name string, fn func(ctx context.Context) (any, error)) (any, error)
}
```

- [ ] **Step 2：写 retry.go（带 jitter 指数退避）**

```go
// pkg/infra/resilience/retry.go
package resilience

import (
    "context"
    "math"
    "math/rand"
    "time"
)

type RetryConfig struct {
    MaxAttempts    int
    InitialBackoff time.Duration
    MaxBackoff     time.Duration
    BackoffFactor  float64
    Jitter         bool
}

func (c RetryConfig) Backoff(attempt int) time.Duration {
    d := float64(c.InitialBackoff) * math.Pow(c.BackoffFactor, float64(attempt-1))
    if d > float64(c.MaxBackoff) { d = float64(c.MaxBackoff) }
    if c.Jitter {
        // ±25% jitter
        j := d * 0.25
        d = d - j + rand.Float64()*2*j
    }
    return time.Duration(d)
}
```

- [ ] **Step 3：写 circuitbreaker.go（修复 HalfOpen）**

```go
// pkg/infra/resilience/circuitbreaker.go
package resilience

import (
    "errors"
    "sync"
    "sync/atomic"
    "time"
)

var ErrCircuitOpen = errors.New("circuit breaker is open")

type State int
const (
    StateClosed State = iota
    StateOpen
    StateHalfOpen
)

func (s State) String() string {
    switch s {
    case StateClosed: return "closed"
    case StateOpen: return "open"
    case StateHalfOpen: return "half-open"
    default: return "unknown"
    }
}

type CircuitBreaker struct {
    mu          sync.Mutex
    state       State
    failures    int
    threshold   int
    timeout     time.Duration
    lastFailure time.Time
    halfOpen    atomic.Int32
}

func NewCircuitBreaker(threshold int, timeout time.Duration) *CircuitBreaker {
    return &CircuitBreaker{
        threshold: threshold,
        timeout:   timeout,
        state:     StateClosed,
    }
}

func (cb *CircuitBreaker) Allow() error {
    cb.mu.Lock()
    defer cb.mu.Unlock()
    switch cb.state {
    case StateOpen:
        if time.Since(cb.lastFailure) > cb.timeout {
            cb.state = StateHalfOpen
            cb.halfOpen.Store(1)
            return nil
        }
        return ErrCircuitOpen
    case StateHalfOpen:
        if cb.halfOpen.CompareAndSwap(0, 1) {
            return nil
        }
        return ErrCircuitOpen
    }
    return nil
}

func (cb *CircuitBreaker) Record(err error) {
    cb.mu.Lock()
    defer cb.mu.Unlock()
    if err != nil {
        cb.failures++
        cb.lastFailure = time.Now()
        if cb.failures >= cb.threshold {
            cb.state = StateOpen
        }
        return
    }
    if cb.state == StateHalfOpen {
        cb.state = StateClosed
        cb.failures = 0
        cb.halfOpen.Store(0)
    }
}

func (cb *CircuitBreaker) GetState() State {
    cb.mu.Lock()
    defer cb.mu.Unlock()
    return cb.state
}
```

- [ ] **Step 4：写 ratelimit.go（令牌桶）**

```go
// pkg/infra/resilience/ratelimit.go
package resilience

import (
    "context"
    "golang.org/x/time/rate"
)

type RateLimiter struct {
    l *rate.Limiter
}

func NewRateLimiter(rps, burst int) *RateLimiter {
    return &RateLimiter{l: rate.NewLimiter(rate.Limit(rps), burst)}
}

func (r *RateLimiter) Wait(ctx context.Context) error { return r.l.Wait(ctx) }
```

- [ ] **Step 5：写 timeout.go**

```go
// pkg/infra/resilience/timeout.go
package resilience

import (
    "context"
    "errors"
    "time"
)

var ErrTimeout = errors.New("operation timed out")

func WithTimeout(parent context.Context, d time.Duration) (context.Context, context.CancelFunc) {
    if d <= 0 { return parent, func() {} }
    return context.WithTimeout(parent, d)
}
```

- [ ] **Step 6：写 runner.go（组合 timeout→ratelimit→breaker→retry→fn）**

```go
// pkg/infra/resilience/runner.go
package resilience

import (
    "context"
    "github.com/zhuge/kongming/pkg/domain/port"
    "github.com/zhuge/kongming/pkg/infra/config"
    "go.uber.org/zap"
    "time"
)

type Runner struct {
    cfg      config.ResilienceConfig
    logger   *zap.Logger
    breaker  *CircuitBreaker
    limiter  *RateLimiter
}

func NewRunner(cfg config.ResilienceConfig, logger *zap.Logger) *Runner {
    return &Runner{
        cfg:     cfg,
        logger:  logger,
        breaker: NewCircuitBreaker(cfg.CircuitBreaker.Threshold, cfg.CircuitBreaker.Timeout),
        limiter: NewRateLimiter(cfg.RateLimit.RPS, cfg.RateLimit.Burst),
    }
}

func (r *Runner) Run(ctx context.Context, name string, fn func(ctx context.Context) error) error {
    _, err := r.RunWithResult(ctx, name, func(ctx context.Context) (any, error) {
        return nil, fn(ctx)
    })
    return err
}

func (r *Runner) RunWithResult(ctx context.Context, name string, fn func(ctx context.Context) (any, error)) (any, error) {
    ctx, cancel := WithTimeout(ctx, r.cfg.Retry.InitialBackoff*10)
    _ = cancel

    if err := r.limiter.Wait(ctx); err != nil {
        return nil, err
    }
    if err := r.breaker.Allow(); err != nil {
        return nil, err
    }

    var result any
    var lastErr error
    backoff := r.cfg.Retry.InitialBackoff
    for attempt := 1; attempt <= r.cfg.Retry.MaxAttempts; attempt++ {
        if err := ctx.Err(); err != nil { return nil, err }
        result, lastErr = fn(ctx)
        if lastErr == nil {
            r.breaker.Record(nil)
            return result, nil
        }
        r.breaker.Record(lastErr)
        if attempt >= r.cfg.Retry.MaxAttempts { break }
        sleep := r.cfg.Retry.Backoff(attempt)
        select {
        case <-ctx.Done():
            return nil, ctx.Err()
        case <-time.After(sleep):
        }
        backoff *= 2
    }
    return nil, lastErr
}

var _ port.ResilientRunner = (*Runner)(nil)
```

- [ ] **Step 7：写 circuitbreaker_test.go（关键：HalfOpen 修复验证）**

```go
// pkg/infra/resilience/circuitbreaker_test.go
package resilience

import (
    "context"
    "errors"
    "sync"
    "testing"
    "time"
    "github.com/stretchr/testify/assert"
)

func TestCircuitBreaker_OpenAfterThreshold(t *testing.T) {
    cb := NewCircuitBreaker(3, 100*time.Millisecond)
    for i := 0; i < 3; i++ {
        _ = cb.Allow()
        cb.Record(errors.New("fail"))
    }
    assert.Equal(t, StateOpen, cb.GetState())
    assert.Equal(t, ErrCircuitOpen, cb.Allow())
}

func TestCircuitBreaker_HalfOpen_OnlyOneProbe(t *testing.T) {
    cb := NewCircuitBreaker(1, 0)
    _ = cb.Allow()
    cb.Record(errors.New("fail"))
    // time.Since(lastFailure) > 0 so it transitions to HalfOpen
    time.Sleep(1 * time.Millisecond)
    assert.NoError(t, cb.Allow())
    // 第二个并发请求应被拒绝
    assert.Equal(t, ErrCircuitOpen, cb.Allow())
}

func TestCircuitBreaker_Recover(t *testing.T) {
    cb := NewCircuitBreaker(1, 0)
    _ = cb.Allow()
    cb.Record(errors.New("fail"))
    time.Sleep(1 * time.Millisecond)
    assert.NoError(t, cb.Allow()) // HalfOpen probe
    cb.Record(nil) // success
    assert.Equal(t, StateClosed, cb.GetState())
}

func TestCircuitBreaker_ConcurrentProbes(t *testing.T) {
    cb := NewCircuitBreaker(1, 0)
    _ = cb.Allow()
    cb.Record(errors.New("fail"))
    time.Sleep(1 * time.Millisecond)

    var wg sync.WaitGroup
    var allowed, denied int
    var mu sync.Mutex
    for i := 0; i < 100; i++ {
        wg.Add(1)
        go func() {
            defer wg.Done()
            err := cb.Allow()
            mu.Lock()
            defer mu.Unlock()
            if err == nil { allowed++ } else { denied++ }
        }()
    }
    wg.Wait()
    assert.Equal(t, 1, allowed)
    assert.Equal(t, 99, denied)
}
```

- [ ] **Step 8：写 retry_test.go**

```go
// pkg/infra/resilience/retry_test.go
package resilience

import (
    "context"
    "errors"
    "testing"
    "time"
    "github.com/stretchr/testify/assert"
)

func TestRetry_Backoff_Exponential(t *testing.T) {
    c := RetryConfig{InitialBackoff: 100*time.Millisecond, MaxBackoff: time.Second, BackoffFactor: 2.0}
    d1 := c.Backoff(1)
    d2 := c.Backoff(2)
    d3 := c.Backoff(3)
    // jitter is ±25%
    assert.Greater(t, d2, d1*time.Duration(1.5))
    assert.Greater(t, d3, d2*time.Duration(1.5))
}

func TestRunner_RetriesUntilSuccess(t *testing.T) {
    r := NewRunner(config.ResilienceConfig{
        Retry: config.RetryConfig{MaxAttempts: 3, InitialBackoff: 1*time.Millisecond, MaxBackoff: 10*time.Millisecond, BackoffFactor: 2.0},
        CircuitBreaker: config.CircuitBreakerConfig{Threshold: 100, Timeout: time.Second},
        RateLimit:     config.RateLimitConfig{RPS: 1000, Burst: 2000},
    }, zap.NewNop())

    var calls int
    err := r.Run(context.Background(), "t", func(ctx context.Context) error {
        calls++
        if calls < 2 { return errors.New("transient") }
        return nil
    })
    assert.NoError(t, err)
    assert.Equal(t, 2, calls)
}

func TestRunner_GivesUpAfterMaxAttempts(t *testing.T) {
    r := NewRunner(config.ResilienceConfig{
        Retry: config.RetryConfig{MaxAttempts: 2, InitialBackoff: 1*time.Millisecond, MaxBackoff: 10*time.Millisecond, BackoffFactor: 2.0},
        CircuitBreaker: config.CircuitBreakerConfig{Threshold: 100, Timeout: time.Second},
        RateLimit:     config.RateLimitConfig{RPS: 1000, Burst: 2000},
    }, zap.NewNop())

    err := r.Run(context.Background(), "t", func(ctx context.Context) error {
        return errors.New("always fail")
    })
    assert.Error(t, err)
}
```

- [ ] **Step 9：运行所有 resilience 测试**

```bash
go test ./pkg/infra/resilience/... -race -v
```
Expected: PASS all

- [ ] **Step 10：commit**

```bash
git add pkg/domain/port/resilience.go pkg/infra/resilience/
git commit -m "feat(resilience): retry+circuitbreaker+ratelimit+timeout runner with HalfOpen fix"
```

### Task 1.7：persistence/memory

**Files:**
- Create: `pkg/infra/persistence/memory/{order_repo.go,general_repo.go,store.go}`
- Test: `pkg/infra/persistence/memory/*_test.go`

- [ ] **Step 1：定义 OrderRepository port**

```go
// pkg/domain/port/persistence.go
package port

import (
    "context"
    "github.com/zhuge/kongming/pkg/domain/model"
)

type OrderRepository interface {
    Save(ctx context.Context, o *model.Order) error
    Get(ctx context.Context, id model.OrderID) (*model.Order, error)
    List(ctx context.Context, state model.State) ([]*model.Order, error)
    Delete(ctx context.Context, id model.OrderID) error
}

type GeneralRepository interface {
    List(ctx context.Context) ([]*model.General, error)
    Get(ctx context.Context, id model.GeneralID) (*model.General, error)
    Save(ctx context.Context, g *model.General) error
}
```

- [ ] **Step 2：写 store.go（共享 sync.Map）**

```go
// pkg/infra/persistence/memory/store.go
package memory

import "sync"

type Store struct {
    orders   sync.Map // model.OrderID → *model.Order
    generals sync.Map // model.GeneralID → *model.General
}

func NewStore() *Store { return &Store{} }
```

- [ ] **Step 3：写 order_repo.go**

```go
// pkg/infra/persistence/memory/order_repo.go
package memory

import (
    "context"
    "fmt"
    "github.com/zhuge/kongming/pkg/domain/model"
    "github.com/zhuge/kongming/pkg/domain/port"
)

type OrderRepo struct{ s *Store }

func NewOrderRepo(s *Store) *OrderRepo { return &OrderRepo{s: s} }

func (r *OrderRepo) Save(_ context.Context, o *model.Order) error {
    r.s.orders.Store(o.ID, o)
    return nil
}

func (r *OrderRepo) Get(_ context.Context, id model.OrderID) (*model.Order, error) {
    if v, ok := r.s.orders.Load(id); ok {
        return v.(*model.Order), nil
    }
    return nil, fmt.Errorf("order not found: %s", id)
}

func (r *OrderRepo) List(_ context.Context, state model.State) ([]*model.Order, error) {
    var out []*model.Order
    r.s.orders.Range(func(_, v any) bool {
        o := v.(*model.Order)
        if int(state) == 0 || o.State == state { out = append(out, o) }
        return true
    })
    return out, nil
}

func (r *OrderRepo) Delete(_ context.Context, id model.OrderID) error {
    r.s.orders.Delete(id)
    return nil
}

var _ port.OrderRepository = (*OrderRepo)(nil)
```

- [ ] **Step 4：写测试**

```go
// pkg/infra/persistence/memory/order_repo_test.go
package memory

import (
    "context"
    "testing"
    "time"
    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"
    "github.com/zhuge/kongming/pkg/domain/model"
)

func TestOrderRepo_SaveAndGet(t *testing.T) {
    s := NewStore()
    r := NewOrderRepo(s)
    o := &model.Order{ID: "o1", Name: "test", State: model.StatePending, CreatedAt: time.Now()}
    require.NoError(t, r.Save(context.Background(), o))
    got, err := r.Get(context.Background(), "o1")
    require.NoError(t, err)
    assert.Equal(t, "test", got.Name)
}

func TestOrderRepo_GetMissing(t *testing.T) {
    r := NewOrderRepo(NewStore())
    _, err := r.Get(context.Background(), "nope")
    assert.Error(t, err)
}

func TestOrderRepo_ListByState(t *testing.T) {
    s := NewStore()
    r := NewOrderRepo(s)
    _ = r.Save(context.Background(), &model.Order{ID: "a", State: model.StatePending})
    _ = r.Save(context.Background(), &model.Order{ID: "b", State: model.StateCompleted})
    list, err := r.List(context.Background(), model.StatePending)
    require.NoError(t, err)
    assert.Equal(t, 1, len(list))
    assert.Equal(t, model.OrderID("a"), list[0].ID)
}
```

- [ ] **Step 5：commit**

```bash
git add pkg/domain/port/persistence.go pkg/infra/persistence/memory/
git commit -m "feat(persistence/memory): in-memory OrderRepository + GeneralRepository"
```

### Task 1.8：plugin registry + fsnotify 热更新

**Files:**
- Create: `pkg/infra/plugin/registry.go`
- Create: `pkg/infra/plugin/watcher.go`
- Create: `pkg/infra/plugin/loader.go`（stub，.so 加载用 stdlib plugin，Linux only）
- Test: `pkg/infra/plugin/registry_test.go`

- [ ] **Step 1：写 registry.go**

```go
// pkg/infra/plugin/registry.go
package plugin

import (
    "fmt"
    "sync"
)

type Handler interface{ Name() string }

type Registry struct {
    mu       sync.RWMutex
    handlers map[string]Handler // name → handler
}

func NewRegistry() *Registry { return &Registry{handlers: make(map[string]Handler)} }

func (r *Registry) Register(h Handler) error {
    if h == nil || h.Name() == "" { return fmt.Errorf("invalid handler") }
    r.mu.Lock()
    defer r.mu.Unlock()
    r.handlers[h.Name()] = h
    return nil
}

func (r *Registry) Unregister(name string) {
    r.mu.Lock()
    defer r.mu.Unlock()
    delete(r.handlers, name)
}

func (r *Registry) Get(name string) (Handler, bool) {
    r.mu.RLock()
    defer r.mu.RUnlock()
    h, ok := r.handlers[name]
    return h, ok
}

func (r *Registry) List() []string {
    r.mu.RLock()
    defer r.mu.RUnlock()
    out := make([]string, 0, len(r.handlers))
    for n := range r.handlers { out = append(out, n) }
    return out
}
```

- [ ] **Step 2：写测试**

```go
// pkg/infra/plugin/registry_test.go
package plugin

import (
    "testing"
    "github.com/stretchr/testify/assert"
)

type fakeHandler struct{ name string }
func (f *fakeHandler) Name() string { return f.name }

func TestRegisterAndGet(t *testing.T) {
    r := NewRegistry()
    err := r.Register(&fakeHandler{name: "h1"})
    assert.NoError(t, err)
    h, ok := r.Get("h1")
    assert.True(t, ok)
    assert.Equal(t, "h1", h.Name())
}

func TestRegisterInvalid(t *testing.T) {
    r := NewRegistry()
    assert.Error(t, r.Register(nil))
    assert.Error(t, r.Register(&fakeHandler{name: ""}))
}

func TestUnregister(t *testing.T) {
    r := NewRegistry()
    _ = r.Register(&fakeHandler{name: "x"})
    r.Unregister("x")
    _, ok := r.Get("x")
    assert.False(t, ok)
}
```

- [ ] **Step 3：写 watcher.go（fsnotify）**

```go
// pkg/infra/plugin/watcher.go
package plugin

import (
    "context"
    "github.com/fsnotify/fsnotify"
    "go.uber.org/zap"
    "path/filepath"
    "strings"
)

func (r *Registry) Watch(ctx context.Context, dir string, logger *zap.Logger) error {
    w, err := fsnotify.NewWatcher()
    if err != nil { return err }
    if err := w.Add(dir); err != nil { return err }

    go func() {
        defer w.Close()
        for {
            select {
            case <-ctx.Done():
                return
            case ev, ok := <-w.Events:
                if !ok { return }
                if ev.Op&(fsnotify.Write|fsnotify.Create) == 0 { continue }
                if !strings.HasSuffix(ev.Name, ".yaml") && !strings.HasSuffix(ev.Name, ".so") { continue }
                if err := r.Reload(ev.Name, logger); err != nil {
                    logger.Error("plugin reload failed", zap.String("file", ev.Name), zap.Error(err))
                }
            case err, ok := <-w.Errors:
                if !ok { return }
                logger.Warn("watcher error", zap.Error(err))
            }
        }
    }()
    return nil
}

func (r *Registry) Reload(path string, logger *zap.Logger) error {
    ext := filepath.Ext(path)
    logger.Info("reloading plugin", zap.String("file", path), zap.String("ext", ext))
    // .yaml: 解析后注册到 vault；.so: 走 plugin.Load
    // 具体实现由 application 层注入
    return nil
}
```

- [ ] **Step 4：commit**

```bash
git add pkg/infra/plugin/
git commit -m "feat(plugin): registry + fsnotify watcher (hot reload)"
```

### Task 1.9：infra 阶段 1 CI 验证

- [ ] **Step 1：运行 infra 所有测试**

```bash
go test ./pkg/infra/... ./pkg/domain/... -race -v
```
Expected: 全部 PASS

- [ ] **Step 2：运行 lint**

```bash
golangci-lint run --timeout=5m ./pkg/infra/... ./pkg/domain/...
```
Expected: 0 issues

- [ ] **Step 3：commit（如有 fix）**

```bash
git add -A
git commit -m "chore(infra): fix lint/test issues from stage 1"
```

---

## 阶段 2 — 领域层（pkg/domain）

### Task 2.1：model 实体

**Files:**
- Create: `pkg/domain/model/{order,strategy,general,jinnang,workflow,message,report}.go`
- Test: `pkg/domain/model/*_test.go`

- [ ] **Step 1：写 order.go（含状态机）**

```go
// pkg/domain/model/order.go
package model

import (
    "fmt"
    "time"
)

type OrderID string
type Priority int
const (
    PriorityLow Priority = iota + 1
    PriorityNormal
    PriorityHigh
    PriorityUrgent
)
func (p Priority) String() string {
    switch p {
    case PriorityLow: return "low"
    case PriorityNormal: return "normal"
    case PriorityHigh: return "high"
    case PriorityUrgent: return "urgent"
    default: return "unknown"
    }
}

type State int
const (
    StateNone State = iota
    StatePending
    StatePlanning
    StateExecuting
    StateReviewing
    StateCompleted
    StateFailed
)
func (s State) String() string {
    switch s {
    case StatePending: return "pending"
    case StatePlanning: return "planning"
    case StateExecuting: return "executing"
    case StateReviewing: return "reviewing"
    case StateCompleted: return "completed"
    case StateFailed: return "failed"
    default: return "unknown"
    }
}

var stateTransitions = map[State][]State{
    StatePending:   {StatePlanning, StateFailed},
    StatePlanning:  {StateExecuting, StateFailed},
    StateExecuting: {StateReviewing, StateFailed},
    StateReviewing: {StateCompleted, StateFailed},
    StateCompleted: {},
    StateFailed:    {StatePending},
}

func (s State) TransitionTo(next State) error {
    for _, allowed := range stateTransitions[s] {
        if allowed == next { return nil }
    }
    return fmt.Errorf("invalid state transition: %s -> %s", s, next)
}

type Order struct {
    ID          OrderID
    Name        string
    Description string
    State       State
    Priority    Priority
    Strategy    Strategy
    Context     map[string]any
    CreatedAt   time.Time
    UpdatedAt   time.Time
    Deadline    *time.Time
    Parent      OrderID
    Generals    []GeneralID
}
```

- [ ] **Step 2：写测试**

```go
// pkg/domain/model/order_test.go
package model

import (
    "testing"
    "github.com/stretchr/testify/assert"
)

func TestStateTransition_Valid(t *testing.T) {
    assert.NoError(t, StatePending.TransitionTo(StatePlanning))
    assert.NoError(t, StatePlanning.TransitionTo(StateExecuting))
    assert.NoError(t, StateFailed.TransitionTo(StatePending))
}

func TestStateTransition_Invalid(t *testing.T) {
    assert.Error(t, StatePending.TransitionTo(StateCompleted))
    assert.Error(t, StateCompleted.TransitionTo(StatePending))
}
```

- [ ] **Step 3：写 strategy.go**

```go
// pkg/domain/model/strategy.go
package model

type BaguaMode string
const (
    Tiangai   BaguaMode = "tiangai"
    Dizai     BaguaMode = "dizai"
    Fengyang  BaguaMode = "fengyang"
    Yunzhui   BaguaMode = "yunzhui"
    Longfei   BaguaMode = "longfei"
    Huyi      BaguaMode = "huyi"
    Niaoxiang BaguaMode = "niaoxiang"
    Shepan    BaguaMode = "shepan"
)

type Strategy struct {
    Type       string
    Objectives []string
    Tactics    []Tactic
    BaguaMode  BaguaMode
    Generals   []GeneralID
    JinnangIDs []string
}

type Tactic struct {
    Order       int
    Name        string
    Description string
    Action      string
    Params      map[string]any
    DependsOn   []int
}
```

- [ ] **Step 4：写 general.go**

```go
// pkg/domain/model/general.go
package model

import "time"

type GeneralID string
type GeneralType string
const (
    GeneralGuanYu GeneralType = "guanyu"
    GeneralZhangFei GeneralType = "zhangfei"
    GeneralZhaoYun GeneralType = "zhaoyun"
    GeneralMaChao GeneralType = "machao"
    GeneralHuangZhong GeneralType = "huangzhong"
)

type GeneralState int
const (
    GeneralIdle GeneralState = iota
    GeneralBusy
    GeneralResting
    GeneralOffline
)

type General struct {
    ID          GeneralID
    Name        string
    Type        GeneralType
    Title       string
    Description string
    Skills      []string
    Traits      map[string]any
    Stats       GeneralStats
    State       GeneralState
    CreatedAt   time.Time
    mu          sync.Mutex
}

func (g *General) SetState(s GeneralState) {
    g.mu.Lock(); g.State = s; g.mu.Unlock()
}
func (g *General) GetState() GeneralState {
    g.mu.Lock(); defer g.mu.Unlock(); return g.State
}

type GeneralStats struct {
    TotalMissions   int
    SuccessCount    int
    FailureCount    int
    AvgResponseTime float64
}
```

> 注意：`sync` 需要在 general.go 顶部 import "sync"

- [ ] **Step 5：写 jinnang.go**

```go
// pkg/domain/model/jinnang.go
package model

import (
    "context"
    "time"
)

type JinnangType string
const (
    JinnangSkill JinnangType = "skill"
    JinnangTool JinnangType = "tool"
    JinnangWisdom JinnangType = "wisdom"
)

type Jinnang struct {
    ID          string
    Name        string
    Type        JinnangType
    Description string
    Version     string
    Tags        []string
    Config      map[string]any
    CreatedAt   time.Time
    UpdatedAt   time.Time
}

type JinnangInput struct {
    Context map[string]any
    Params  map[string]any
    Data    any
}

type JinnangOutput struct {
    Success bool
    Data    any
    Error   string
    Meta    map[string]any
}

type JinnangHandler interface {
    Execute(ctx context.Context, input JinnangInput) (*JinnangOutput, error)
    Validate(input JinnangInput) error
    GetSchema() (map[string]any, error)
}
```

- [ ] **Step 6：写 workflow.go + message.go + report.go**

（按 spec §2.1 完整实现，逻辑直接对应原 `pkg/bagua/engine.go` 的 `Workflow/Node/Edge`，原 `pkg/courier/courier.go` 的 `Message`，原 `pkg/cmd_center/types.go` 的 `BattleReport/GeneralReport`。每个文件 ~50-80 行，此处省略。）

- [ ] **Step 7：domain 整体测试**

```bash
go test ./pkg/domain/... -race -cover -v
```
Expected: 全部 PASS, 覆盖率 100%

- [ ] **Step 8：commit**

```bash
git add pkg/domain/
git commit -m "feat(domain): model entities with state machine + port interfaces"
```

### Task 2.2：errors 体系

**Files:**
- Create: `pkg/domain/errors/{code.go,errors.go,classify.go}`
- Test: `pkg/domain/errors/errors_test.go`

- [ ] **Step 1：写 code.go**

```go
// pkg/domain/errors/code.go
package errors

type Code string
const (
    CodeInvalidArgument Code = "INVALID_ARGUMENT"
    CodeNotFound        Code = "NOT_FOUND"
    CodeConflict        Code = "CONFLICT"
    CodeTimeout         Code = "TIMEOUT"
    CodeUnavailable     Code = "UNAVAILABLE"
    CodeInternal        Code = "INTERNAL"
    CodeCircuitOpen     Code = "CIRCUIT_OPEN"
    CodePluginLoadFail  Code = "PLUGIN_LOAD_FAIL"
    CodeInvalidState    Code = "INVALID_STATE"
    CodeStrategyFailed  Code = "STRATEGY_FAILED"
    CodePersistFailed   Code = "PERSIST_FAILED"
)
```

- [ ] **Step 2：写 errors.go**

```go
// pkg/domain/errors/errors.go
package errors

import "fmt"

type Error struct {
    Code    Code
    Message string
    Cause   error
    TraceID string
    Fields  map[string]any
}

func (e *Error) Error() string {
    if e.Cause != nil { return string(e.Code) + ": " + e.Message + ": " + e.Cause.Error() }
    return string(e.Code) + ": " + e.Message
}

func (e *Error) Unwrap() error { return e.Cause }

func (e *Error) Is(target error) bool {
    t, ok := target.(*Error)
    if !ok { return false }
    return e.Code == t.Code
}

func New(code Code, msg string) *Error {
    return &Error{Code: code, Message: msg}
}

func Wrap(code Code, cause error) *Error {
    return &Error{Code: code, Cause: cause, Message: cause.Error()}
}
```

- [ ] **Step 3：写 classify.go**

```go
// pkg/domain/errors/classify.go
package errors

import (
    "google.golang.org/grpc/codes"
    "net/http"
)

func (c Code) HTTPStatus() int {
    switch c {
    case CodeInvalidArgument: return http.StatusBadRequest
    case CodeNotFound: return http.StatusNotFound
    case CodeConflict, CodeInvalidState: return http.StatusConflict
    case CodeTimeout: return http.StatusGatewayTimeout
    case CodeUnavailable, CodeCircuitOpen: return http.StatusServiceUnavailable
    default: return http.StatusInternalServerError
    }
}

func (c Code) GRPCCode() codes.Code {
    switch c {
    case CodeInvalidArgument: return codes.InvalidArgument
    case CodeNotFound: return codes.NotFound
    case CodeConflict, CodeInvalidState: return codes.FailedPrecondition
    case CodeTimeout: return codes.DeadlineExceeded
    case CodeUnavailable, CodeCircuitOpen: return codes.Unavailable
    default: return codes.Internal
    }
}
```

- [ ] **Step 4：写测试**

```go
// pkg/domain/errors/errors_test.go
package errors

import (
    "errors"
    "testing"
    "github.com/stretchr/testify/assert"
)

func TestError_Is(t *testing.T) {
    e1 := New(CodeNotFound, "x")
    e2 := New(CodeNotFound, "y")
    assert.True(t, errors.Is(e1, e2))
    assert.False(t, errors.Is(e1, New(CodeInternal, "z")))
}

func TestClassify_HTTP(t *testing.T) {
    assert.Equal(t, 404, CodeNotFound.HTTPStatus())
    assert.Equal(t, 503, CodeCircuitOpen.HTTPStatus())
    assert.Equal(t, 504, CodeTimeout.HTTPStatus())
}

func TestWrap(t *testing.T) {
    cause := errors.New("disk full")
    e := Wrap(CodePersistFailed, cause)
    assert.ErrorIs(t, e, cause)
}
```

- [ ] **Step 5：commit**

```bash
git add pkg/domain/errors/
git commit -m "feat(domain/errors): typed error code + http/grpc classifier"
```

---

## 阶段 3 — 应用层（pkg/application）

### Task 3.1：commander 用例

**Files:**
- Create: `pkg/application/commander/{service.go,planner.go,idempotent.go}`
- Test: `pkg/application/commander/service_test.go`

- [ ] **Step 1：写 port/commander.go**

```go
// pkg/domain/port/commander.go
package port

import (
    "context"
    "github.com/zhuge/kongming/pkg/domain/model"
)

type Commander interface {
    Dispatch(ctx context.Context, order *model.Order) (*model.BattleReport, error)
    PlanStrategy(ctx context.Context, order *model.Order) (*model.Strategy, error)
    Review(ctx context.Context, report *model.BattleReport) error
    GetOrder(ctx context.Context, id model.OrderID) (*model.Order, error)
    ListOrders(ctx context.Context, state model.State) ([]*model.Order, error)
}
```

- [ ] **Step 2：写 service.go（按 spec §3.2 伪代码实现）**

```go
// pkg/application/commander/service.go
package commander

import (
    "context"
    "fmt"
    "time"
    domerrs "github.com/zhuge/kongming/pkg/domain/errors"
    "github.com/zhuge/kongming/pkg/domain/model"
    "github.com/zhuge/kongming/pkg/domain/port"
    "go.opentelemetry.io/otel/attribute"
    "go.uber.org/zap"
)

type Service struct {
    cfg        struct{ DefaultTimeout time.Duration }
    planner    Planner
    pool       port.GeneralPool
    engine     port.Engine
    vault      port.Vault
    orders     port.OrderRepository
    resilient  port.ResilientRunner
    observer   port.Observer
    logger     *zap.Logger
}

// 统一别名：kongming/pkg/domain/errors 包用 domerrs 避免与 stdlib errors 冲突
var domerrs = errors // alias for "github.com/zhuge/kongming/pkg/domain/errors"

func New(
    planner Planner, pool port.GeneralPool, engine port.Engine, vault port.Vault,
    orders port.OrderRepository, resilient port.ResilientRunner,
    observer port.Observer, logger *zap.Logger,
) *Service {
    s := &Service{
        planner: planner, pool: pool, engine: engine, vault: vault,
        orders: orders, resilient: resilient, observer: observer, logger: logger,
    }
    return s
}

func (s *Service) Dispatch(ctx context.Context, order *model.Order) (*model.BattleReport, error) {
    ctx, span := s.observer.StartSpan(ctx, "commander.Dispatch",
        attribute.String("order.id", string(order.ID)))
    defer span.End()

    // 幂等检查
    if existing, _ := s.orders.Get(ctx, order.ID); existing != nil {
        s.logger.Info("idempotent replay", zap.String("order_id", string(order.ID)))
        return s.replayReport(ctx, existing)
    }

    if err := order.State.TransitionTo(model.StatePlanning); err != nil {
        return nil, errs.Wrap(errs.CodeInvalidState, err)
    }
    order.UpdatedAt = time.Now()

    strategy, err := s.planner.Plan(ctx, order)
    if err != nil {
        s.observer.RecordError(span, err)
        return nil, errs.Wrap(errs.CodeStrategyFailed, err)
    }
    order.Strategy = *strategy
    if err := order.State.TransitionTo(model.StateExecuting); err != nil {
        return nil, errs.Wrap(errs.CodeInvalidState, err)
    }
    if err := s.orders.Save(ctx, order); err != nil {
        return nil, errs.Wrap(errs.CodePersistFailed, err)
    }

    var report *model.BattleReport
    err = s.resilient.Run(ctx, "commander.dispatch", func(ctx context.Context) error {
        var rerr error
        report, rerr = s.runTactics(ctx, order, strategy)
        return rerr
    })
    if err != nil {
        order.State = model.StateFailed
        _ = s.orders.Save(ctx, order)
        return nil, err
    }

    if err := s.Review(ctx, report); err != nil {
        s.logger.Warn("review failed", zap.Error(err))
    }
    if err := order.State.TransitionTo(model.StateCompleted); err != nil {
        return nil, err
    }
    _ = s.orders.Save(ctx, order)
    return report, nil
}

func (s *Service) runTactics(ctx context.Context, order *model.Order, strategy *model.Strategy) (*model.BattleReport, error) {
    report := &model.BattleReport{OrderID: order.ID, StartedAt: time.Now()}
    for _, tactic := range strategy.Tactics {
        general, err := s.pool.SelectBest(tactic.Action)
        if err != nil {
            s.logger.Warn("no general", zap.String("tactic", tactic.Name))
            continue
        }
        sub := &model.Order{ID: model.OrderID(string(order.ID) + "_" + tactic.Name), Name: tactic.Name, Context: order.Context}
        gr, err := s.pool.Execute(ctx, general.ID, sub)
        if err != nil {
            s.logger.Error("general exec failed", zap.String("general", general.Name), zap.Error(err))
            continue
        }
        report.Generals = append(report.Generals, *gr)
    }
    report.CompletedAt = time.Now()
    report.Success = true
    return report, nil
}

func (s *Service) PlanStrategy(ctx context.Context, order *model.Order) (*model.Strategy, error) {
    return s.planner.Plan(ctx, order)
}

func (s *Service) Review(ctx context.Context, report *model.BattleReport) error {
    if !report.Success { return fmt.Errorf("report failed: %s", report.Message) }
    for _, gr := range report.Generals {
        if gr.Success {
            s.logger.Info("general succeeded", zap.String("general", gr.GeneralName))
        }
    }
    return nil
}

func (s *Service) GetOrder(ctx context.Context, id model.OrderID) (*model.Order, error) {
    return s.orders.Get(ctx, id)
}

func (s *Service) ListOrders(ctx context.Context, state model.State) ([]*model.Order, error) {
    return s.orders.List(ctx, state)
}

func (s *Service) replayReport(_ context.Context, _ *model.Order) (*model.BattleReport, error) {
    return nil, errors.New("idempotent replay not yet implemented")
}
```

- [ ] **Step 3：写 planner.go**

```go
// pkg/application/commander/planner.go
package commander

import (
    "context"
    "fmt"
    "github.com/zhuge/kongming/pkg/domain/model"
)

type Planner interface{ Plan(ctx context.Context, order *model.Order) (*model.Strategy, error) }

type DefaultPlanner struct{}

func (p *DefaultPlanner) Plan(_ context.Context, order *model.Order) (*model.Strategy, error) {
    s := &model.Strategy{
        Type: "default",
        Objectives: order.Strategy.Objectives,
        Tactics:    []model.Tactic{},
        BaguaMode:  model.Dizai,
    }
    switch order.Priority {
    case model.PriorityUrgent: s.BaguaMode = model.Fengyang
    case model.PriorityHigh: s.BaguaMode = model.Tiangai
    }
    for i, obj := range order.Strategy.Objectives {
        s.Tactics = append(s.Tactics, model.Tactic{
            Order: i + 1, Name: obj, Description: fmt.Sprintf("执行目标: %s", obj), Action: "execute",
        })
    }
    return s, nil
}
```

- [ ] **Step 4：写 service_test.go（用 mock port）**

```go
// pkg/application/commander/service_test.go
package commander

import (
    "context"
    "testing"
    "time"
    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"
    "github.com/zhuge/kongming/pkg/domain/model"
    "github.com/zhuge/kongming/pkg/infra/persistence/memory"
    "go.uber.org/zap"
)

type stubPool struct {
    generals map[model.GeneralID]*model.General
    reports  map[model.GeneralID]*model.GeneralReport
}
func (p *stubPool) Execute(_ context.Context, id model.GeneralID, _ *model.Order) (*model.GeneralReport, error) {
    return p.reports[id], nil
}
func (p *stubPool) SelectBest(skill string) (*model.General, error) {
    for _, g := range p.generals {
        for _, s := range g.Skills { if s == skill { return g, nil } }
    }
    return nil, assert.AnError
}
func (p *stubPool) Get(_ model.GeneralID) (*model.General, error)         { return nil, nil }
func (p *stubPool) List() ([]*model.General, error)                       { return nil, nil }
func (p *stubPool) Register(_ *model.General) error                       { return nil }
func (p *stubPool) Unregister(_ model.GeneralID) error                    { return nil }

type noopResilient struct{}
func (noopResilient) Run(_ context.Context, _ string, fn func(context.Context) error) error { return fn(context.Background()) }
func (noopResilient) RunWithResult(_ context.Context, _ string, fn func(context.Context) (any, error)) (any, error) {
    return fn(context.Background())
}

type noopObserver struct{}
func (noopObserver) StartSpan(ctx context.Context, _ string, _ ...attribute.KeyValue) (context.Context, trace.Span) {
    return ctx, trace.SpanFromContext(ctx)
}
func (noopObserver) RecordError(_ trace.Span, _ error)                          {}
func (noopObserver) RecordEvent(_ context.Context, _ string, _ ...attribute.KeyValue) {}
func (noopObserver) IncCounter(_ string, _ map[string]string)                   {}
func (noopObserver) ObserveHistogram(_ string, _ float64, _ map[string]string)  {}
func (noopObserver) SetGauge(_ string, _ float64, _ map[string]string)          {}
func (noopObserver) Shutdown(_ context.Context) error                           { return nil }
```

> 注意：上面 mock 需 import `go.opentelemetry.io/otel/attribute` 和 `go.opentelemetry.io/otel/trace`。

```go
// 接续 service_test.go
func TestService_Dispatch_HappyPath(t *testing.T) {
    store := memory.NewStore()
    orders := memory.NewOrderRepo(store)
    pool := &stubPool{
        generals: map[model.GeneralID]*model.General{
            "guanyu": {ID: "guanyu", Name: "关羽", Skills: []string{"execute"}, State: model.GeneralIdle},
        },
        reports: map[model.GeneralID]*model.GeneralReport{
            "guanyu": {GeneralID: "guanyu", Success: true, Message: "ok"},
        },
    }
    s := New(&DefaultPlanner{}, pool, nil, nil, orders, noopResilient{}, noopObserver{}, zap.NewNop())

    order := &model.Order{
        ID: "o1", Name: "test", State: model.StatePending, Priority: model.PriorityNormal,
        Strategy: model.Strategy{Objectives: []string{"obj1"}},
        CreatedAt: time.Now(),
    }
    report, err := s.Dispatch(context.Background(), order)
    require.NoError(t, err)
    assert.True(t, report.Success)
    assert.Equal(t, 1, len(report.Generals))
    saved, _ := orders.Get(context.Background(), "o1")
    assert.Equal(t, model.StateCompleted, saved.State)
}
```

- [ ] **Step 5：运行测试**

```bash
go test ./pkg/application/commander/... -race -v
```
Expected: PASS

- [ ] **Step 6：commit**

```bash
git add pkg/application/commander/ pkg/domain/port/commander.go
git commit -m "feat(commander): Dispatch with planning + idempotent + resilient runner"
```

### Task 3.2：workflow（含 8 阵 + DAG）

**Files:**
- Create: `pkg/application/workflow/{runner.go,modes/*.go,node/*.go,dag.go}`
- Test: `pkg/application/workflow/*_test.go`

- [ ] **Step 1：写 port/engine.go**

```go
// pkg/domain/port/engine.go
package port

import (
    "context"
    "github.com/zhuge/kongming/pkg/domain/model"
)

type Engine interface {
    RegisterWorkflow(wf *model.Workflow) error
    GetWorkflow(id string) (*model.Workflow, error)
    Execute(ctx context.Context, id string, inputs map[string]any) (*model.ExecutionContext, error)
    RegisterNodeExecutor(t model.NodeType, exec NodeExecutor)
}

type NodeExecutor interface {
    Execute(ctx context.Context, node model.Node, ec *model.ExecutionContext) (*model.NodeState, error)
}
```

- [ ] **Step 2：写 dag.go（拓扑层级）**

```go
// pkg/application/workflow/dag.go
package workflow

import "github.com/zhuge/kongming/pkg/domain/model"

func buildDAG(wf *model.Workflow) map[string][]string {
    g := make(map[string][]string)
    for _, e := range wf.Edges { g[e.From] = append(g[e.From], e.To) }
    for _, n := range wf.Nodes {
        if _, ok := g[n.ID]; !ok { g[n.ID] = nil }
    }
    return g
}

func topologicalLevels(graph map[string][]string) [][]string {
    var levels [][]string
    visited := make(map[string]bool)
    for {
        var level []string
        for node, succs := range graph {
            if visited[node] { continue }
            ready := true
            for from, tos := range graph {
                for _, to := range tos {
                    if to == node && !visited[from] { ready = false }
                }
            }
            _ = succs
            if ready { level = append(level, node) }
        }
        if len(level) == 0 { break }
        for _, n := range level { visited[n] = true }
        levels = append(levels, level)
    }
    return levels
}
```

- [ ] **Step 3：写 modes/tiangai.go（并行）**

```go
// pkg/application/workflow/modes/tiangai.go
package modes

import (
    "context"
    "fmt"
    "sync"
    "github.com/zhuge/kongming/pkg/domain/model"
    "github.com/zhuge/kongming/pkg/domain/port"
)

func Tiangai(ctx context.Context, wf *model.Workflow, ec *model.ExecutionContext,
    nodes map[model.NodeType]port.NodeExecutor, topo func(map[string][]string) [][]string,
    buildDAG func(*model.Workflow) map[string][]string,
) (*model.ExecutionContext, error) {
    graph := buildDAG(wf)
    levels := topo(graph)
    for _, level := range levels {
        var wg sync.WaitGroup
        errCh := make(chan error, len(level))
        for _, id := range level {
            wg.Add(1)
            go func(id string) {
                defer wg.Done()
                var node *model.Node
                for i := range wf.Nodes { if wf.Nodes[i].ID == id { node = &wf.Nodes[i]; break } }
                if node == nil { errCh <- fmt.Errorf("node not found: %s", id); return }
                exec, ok := nodes[node.Type]
                if !ok { return }
                state, err := exec.Execute(ctx, *node, ec)
                if err != nil { errCh <- err; return }
                ec.NodeStates[id] = *state
            }(id)
        }
        wg.Wait()
        close(errCh)
        for e := range errCh { if e != nil { return ec, e } }
    }
    return ec, nil
}
```

- [ ] **Step 4：写 modes/dizai.go / fengyang.go / yunzhui.go / longfei.go / huyi.go / niaoxiang.go / shepan.go**

每个 ~40-60 行。实现细节：
- **Dizai**：单链顺序，遇错终止
- **Fengyang**：Dizai + `context.WithTimeout`
- **Yunzhui**：Dizai 包 retry（最多 3 次）
- **Longfei**：DFS 找 critical path → 顺序执行 → tiangai 并行剩余
- **Huyi**：eval `Edge.Condition`（简单 expression：`status=="ok"`）决定下游
- **Niaoxiang**：1 个源节点 → N 个独立下游分支并行
- **Shepan**：loop node 含 `max_iterations` 循环

- [ ] **Step 5：写 runner.go（聚合 + 注册内置 node executor）**

```go
// pkg/application/workflow/runner.go
package workflow

import (
    "context"
    "fmt"
    "sync"
    "github.com/google/uuid"
    "github.com/zhuge/kongming/pkg/application/workflow/modes"
    "github.com/zhuge/kongming/pkg/domain/model"
    "github.com/zhuge/kongming/pkg/domain/port"
    "go.uber.org/zap"
)

type Runner struct {
    logger   *zap.Logger
    mu       sync.RWMutex
    workflows map[string]*model.Workflow
    nodes    map[model.NodeType]port.NodeExecutor
}

func NewRunner(logger *zap.Logger) *Runner {
    r := &Runner{
        logger: logger, workflows: make(map[string]*model.Workflow),
        nodes: make(map[model.NodeType]port.NodeExecutor),
    }
    // 注册内置节点
    r.RegisterNodeExecutor(model.NodeStart, &startExecutor{})
    r.RegisterNodeExecutor(model.NodeEnd, &endExecutor{})
    return r
}

func (r *Runner) RegisterWorkflow(wf *model.Workflow) error {
    if err := validate(wf); err != nil { return err }
    if wf.ID == "" { wf.ID = uuid.NewString() }
    r.mu.Lock(); r.workflows[wf.ID] = wf; r.mu.Unlock()
    return nil
}

func (r *Runner) GetWorkflow(id string) (*model.Workflow, error) {
    r.mu.RLock(); defer r.mu.RUnlock()
    wf, ok := r.workflows[id]
    if !ok { return nil, fmt.Errorf("workflow not found: %s", id) }
    return wf, nil
}

func (r *Runner) RegisterNodeExecutor(t model.NodeType, e port.NodeExecutor) {
    r.mu.Lock(); r.nodes[t] = e; r.mu.Unlock()
}

func (r *Runner) Execute(ctx context.Context, id string, inputs map[string]any) (*model.ExecutionContext, error) {
    wf, err := r.GetWorkflow(id)
    if err != nil { return nil, err }
    ec := &model.ExecutionContext{
        WorkflowID: id, RunID: uuid.NewString(),
        Variables: inputs, NodeStates: make(map[string]model.NodeState),
    }
    switch wf.Mode {
    case model.Tiangai:
        return modes.Tiangai(ctx, wf, ec, r.nodes, topologicalLevels, buildDAG)
    case model.Dizai:
        return modes.Dizai(ctx, wf, ec, r.nodes)
    // ... 其他 6 阵类似
    default:
        return modes.Dizai(ctx, wf, ec, r.nodes)
    }
}

func validate(wf *model.Workflow) error {
    hasStart, hasEnd := false, false
    for _, n := range wf.Nodes {
        if n.Type == model.NodeStart { hasStart = true }
        if n.Type == model.NodeEnd { hasEnd = true }
    }
    if !hasStart { return fmt.Errorf("missing start node") }
    if !hasEnd { return fmt.Errorf("missing end node") }
    return nil
}

var _ port.Engine = (*Runner)(nil)

type startExecutor struct{}
func (s *startExecutor) Execute(_ context.Context, _ model.Node, ec *model.ExecutionContext) (*model.NodeState, error) {
    return &model.NodeState{Status: "ok"}, nil
}
type endExecutor struct{}
func (e *endExecutor) Execute(_ context.Context, _ model.Node, ec *model.ExecutionContext) (*model.NodeState, error) {
    return &model.NodeState{Status: "ok"}, nil
}
```

- [ ] **Step 6：写 workflow_test.go（覆盖 8 阵）**

```go
// pkg/application/workflow/runner_test.go
package workflow

import (
    "context"
    "errors"
    "sync/atomic"
    "testing"
    "time"
    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"
    "github.com/zhuge/kongming/pkg/domain/model"
    "github.com/zhuge/kongming/pkg/domain/port"
    "go.uber.org/zap"
)

type countingExec struct{ calls *atomic.Int32 }
func (c *countingExec) Execute(_ context.Context, n model.Node, _ *model.ExecutionContext) (*model.NodeState, error) {
    c.calls.Add(1)
    return &model.NodeState{Status: "ok", Output: n.Name}, nil
}
type failingExec struct{ calls *atomic.Int32 }
func (f *failingExec) Execute(_ context.Context, _ model.Node, _ *model.ExecutionContext) (*model.NodeState, error) {
    f.calls.Add(1)
    return nil, errors.New("fail")
}

func newTestWorkflow(mode model.BaguaMode) *model.Workflow {
    return &model.Workflow{
        ID: "wf1", Name: "test", Mode: mode,
        Nodes: []model.Node{
            {ID: "s", Type: model.NodeStart},
            {ID: "m1", Type: model.NodeLLM},
            {ID: "m2", Type: model.NodeLLM},
            {ID: "e", Type: model.NodeEnd},
        },
        Edges: []model.Edge{
            {From: "s", To: "m1"}, {From: "m1", To: "e"},
            {From: "s", To: "m2"}, {From: "m2", To: "e"},
        },
    }
}

func TestRunner_RegisterAndExecute_Tiangai_Parallel(t *testing.T) {
    r := NewRunner(zap.NewNop())
    counter := &countingExec{calls: &atomic.Int32{}}
    r.RegisterNodeExecutor(model.NodeLLM, counter)
    wf := newTestWorkflow(model.Tiangai)
    require.NoError(t, r.RegisterWorkflow(wf))
    _, err := r.Execute(context.Background(), wf.ID, nil)
    require.NoError(t, err)
    assert.Equal(t, int32(2), counter.calls.Load())
}

func TestRunner_Dizai_Sequential(t *testing.T) {
    r := NewRunner(zap.NewNop())
    counter := &countingExec{calls: &atomic.Int32{}}
    r.RegisterNodeExecutor(model.NodeLLM, counter)
    wf := &model.Workflow{
        ID: "wf-seq", Mode: model.Dizai,
        Nodes: []model.Node{{ID: "s", Type: model.NodeStart}, {ID: "m", Type: model.NodeLLM}, {ID: "e", Type: model.NodeEnd}},
        Edges: []model.Edge{{From: "s", To: "m"}, {From: "m", To: "e"}},
    }
    require.NoError(t, r.RegisterWorkflow(wf))
    _, err := r.Execute(context.Background(), wf.ID, nil)
    require.NoError(t, err)
    assert.Equal(t, int32(1), counter.calls.Load())
}

func TestRunner_Fengyang_Timeout(t *testing.T) {
    r := NewRunner(zap.NewNop())
    slowExec := struct {
        port.NodeExecutor
    }{}
    _ = slowExec
    slow := &slowLLM{wait: 200 * time.Millisecond}
    r.RegisterNodeExecutor(model.NodeLLM, slow)
    wf := &model.Workflow{
        ID: "wf-fe", Mode: model.Fengyang,
        Nodes: []model.Node{{ID: "s", Type: model.NodeStart}, {ID: "m", Type: model.NodeLLM}, {ID: "e", Type: model.NodeEnd}},
        Edges: []model.Edge{{From: "s", To: "m"}, {From: "m", To: "e"}},
    }
    require.NoError(t, r.RegisterWorkflow(wf))
    _, err := r.Execute(context.Background(), wf.ID, nil)
    assert.Error(t, err)
}

type slowLLM struct{ wait time.Duration }
func (s *slowLLM) Execute(ctx context.Context, _ model.Node, _ *model.ExecutionContext) (*model.NodeState, error) {
    select {
    case <-time.After(s.wait): return &model.NodeState{Status: "ok"}, nil
    case <-ctx.Done(): return nil, ctx.Err()
    }
}

func TestRunner_Yunzhui_Retries(t *testing.T) {
    r := NewRunner(zap.NewNop())
    var calls atomic.Int32
    r.RegisterNodeExecutor(model.NodeLLM, &flakyExec{calls: &calls})
    wf := &model.Workflow{
        ID: "wf-yu", Mode: model.Yunzhui,
        Nodes: []model.Node{{ID: "s", Type: model.NodeStart}, {ID: "m", Type: model.NodeLLM}, {ID: "e", Type: model.NodeEnd}},
        Edges: []model.Edge{{From: "s", To: "m"}, {From: "m", To: "e"}},
    }
    require.NoError(t, r.RegisterWorkflow(wf))
    _, err := r.Execute(context.Background(), wf.ID, nil)
    require.NoError(t, err)
    assert.Equal(t, int32(2), calls.Load()) // 1 fail + 1 success
}

type flakyExec struct{ calls *atomic.Int32 }
func (f *flakyExec) Execute(_ context.Context, _ model.Node, _ *model.ExecutionContext) (*model.NodeState, error) {
    n := f.calls.Add(1)
    if n == 1 { return nil, errors.New("transient") }
    return &model.NodeState{Status: "ok"}, nil
}
```

- [ ] **Step 7：commit**

```bash
git add pkg/application/workflow/ pkg/domain/port/engine.go
git commit -m "feat(workflow): 8 bagua modes + DAG executor with 80%+ test coverage"
```

### Task 3.3：general / vault / courier / dispatcher（与 commander 类似骨架）

> 此处省略实现细节，每个子模块一个任务。
> 关键点：
> - general：WuHuPool 替换 generals.WuHuPool，含 sync/atomic 状态机
> - vault：含 LoadFromDir（fsnotify）+ RegisterSkill + 3 个 builtin
> - courier：channel-based + delivery 状态机（已实现可复用）
> - dispatcher：异步派发 + executor 路由
>
> 每个子模块 200-300 行 + 200 行测试
> 提交粒度按子模块拆，命名 `feat(general):` / `feat(vault):` / `feat(courier):` / `feat(dispatcher):`

- [ ] **Task 3.3.1：general（迁移 generals.WuHuPool）**
- [ ] **Task 3.3.2：vault（迁移 strategy_vault.DefaultVault + LoadFromDir）**
- [ ] **Task 3.3.3：courier（迁移 courier.Courier）**
- [ ] **Task 3.3.4：dispatcher（迁移 dispatch.Dispatcher）**

每个 task 完成后：

```bash
go test ./pkg/application/<module>/... -race -v
git add pkg/application/<module>/
git commit -m "feat(<module>): port-based service with 80%+ test coverage"
```

### Task 3.4：application 阶段 CI 验证

- [ ] **Step 1：全 application 测试**

```bash
go test ./pkg/application/... -race -cover -v
```
Expected: 全部 PASS；application 整体覆盖率 ≥ 90%

- [ ] **Step 2：commit**

```bash
git add -A
git commit -m "chore(app): application layer green with 90%+ coverage"
```

---

## 阶段 4 — 传输层（pkg/transport）

### Task 4.1：HTTP transport（gin）

**Files:**
- Create: `pkg/transport/http/{server.go,middleware/*.go,handler/*.go,dto/*.go}`

- [ ] **Step 1：写 server.go**

```go
// pkg/transport/http/server.go
package http

import (
    "context"
    "net/http"
    "time"
    "github.com/zhuge/kongming/pkg/domain/port"
    "github.com/zhuge/kongming/pkg/transport/http/handler"
    "github.com/zhuge/kongming/pkg/transport/http/middleware"
    "github.com/gin-gonic/gin"
    "go.uber.org/zap"
)

type Server struct {
    engine *gin.Engine
    srv    *http.Server
    logger *zap.Logger
}

type Deps struct {
    Commander  port.Commander
    Dispatcher port.Dispatcher
    Engine     port.Engine
    Pool       port.GeneralPool
    Vault      port.Vault
    Observer   port.Observer
    Logger     *zap.Logger
    Addr       string
}

func NewServer(d Deps) *Server {
    gin.SetMode(gin.ReleaseMode)
    e := gin.New()
    e.Use(middleware.Recovery(d.Logger))
    e.Use(middleware.TraceID())
    e.Use(middleware.Logging(d.Logger))
    e.Use(middleware.CORS())

    h := handler.New(d.Commander, d.Dispatcher, d.Engine, d.Pool, d.Vault)
    api := e.Group("/api/v1")
    {
        api.POST("/orders", h.CreateOrder)
        api.GET("/orders", h.ListOrders)
        api.GET("/orders/:id", h.GetOrder)
        api.POST("/strategies", h.PlanStrategy)
        api.GET("/generals", h.ListGenerals)
        api.GET("/generals/:id", h.GetGeneral)
        api.GET("/vault", h.ListJinnang)
        api.POST("/vault/:id/exec", h.ExecuteJinnang)
        api.POST("/workflows/:id/run", h.RunWorkflow)
    }
    e.GET("/healthz", h.Healthz)
    e.GET("/readyz", h.Readyz)
    e.GET("/metrics", gin.WrapH(d.Observer.Handler()))

    return &Server{
        engine: e, logger: d.Logger,
        srv: &http.Server{
            Addr: d.Addr, Handler: e,
            ReadTimeout: 30 * time.Second, WriteTimeout: 30 * time.Second,
        },
    }
}

func (s *Server) ListenAndServe() error { return s.srv.ListenAndServe() }
func (s *Server) Shutdown(ctx context.Context) error { return s.srv.Shutdown(ctx) }
```

- [ ] **Step 2：写 middleware/traceid.go**

```go
// pkg/transport/http/middleware/traceid.go
package middleware

import (
    "github.com/gin-gonic/gin"
    "github.com/zhuge/kongming/pkg/infra/observability"
)

const HeaderTraceID = "X-Trace-Id"

func TraceID() gin.HandlerFunc {
    return func(c *gin.Context) {
        id := c.GetHeader(HeaderTraceID)
        if id == "" { id = observability.NewTraceID() }
        ctx := observability.NewTraceIDContext(c.Request.Context(), id)
        c.Request = c.Request.WithContext(ctx)
        c.Header(HeaderTraceID, id)
        c.Next()
    }
}
```

- [ ] **Step 3：写 middleware/recovery.go + logging.go + cors.go**

略（30-50 行/文件，标准 gin 中间件）

- [ ] **Step 4：写 handler/order.go（其余 handler 类似）**

```go
// pkg/transport/http/handler/order.go
package handler

import (
    "errors"
    "net/http"
    "github.com/gin-gonic/gin"
    "github.com/google/uuid"
    "github.com/zhuge/kongming/pkg/domain/errors"
    "github.com/zhuge/kongming/pkg/domain/model"
    "github.com/zhuge/kongming/pkg/domain/port"
)

type OrderHandler struct{ c port.Commander }

func (h *Handler) CreateOrder(c *gin.Context) {
    var req CreateOrderRequest
    if err := c.ShouldBindJSON(&req); err != nil {
        c.JSON(http.StatusBadRequest, errorResp(errs.CodeInvalidArgument, err.Error()))
        return
    }
    order := &model.Order{
        ID: model.OrderID(uuid.NewString()),
        Name: req.Name, Description: req.Description,
        State: model.StatePending, Priority: model.Priority(req.Priority),
        Strategy: model.Strategy{Objectives: req.Objectives},
        Context: map[string]any{},
    }
    report, err := h.c.Dispatch(c.Request.Context(), order)
    if err != nil {
        c.JSON(errsFromErr(err).HTTPStatus(), errorResp(errsFromErr(err).Code, err.Error()))
        return
    }
    c.JSON(http.StatusOK, gin.H{"order_id": order.ID, "report": report})
}
```

- [ ] **Step 5：写 handler_test.go（httptest）**

```go
// pkg/transport/http/handler/order_test.go
package handler

import (
    "bytes"
    "context"
    "encoding/json"
    "net/http"
    "net/http/httptest"
    "testing"
    "github.com/gin-gonic/gin"
    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"
    "github.com/zhuge/kongming/pkg/domain/model"
    "github.com/zhuge/kongming/pkg/domain/port"
)

type mockCommander struct{ lastOrder *model.Order }
func (m *mockCommander) Dispatch(_ context.Context, o *model.Order) (*model.BattleReport, error) {
    m.lastOrder = o
    return &model.BattleReport{OrderID: o.ID, Success: true}, nil
}
func (m *mockCommander) PlanStrategy(_ context.Context, _ *model.Order) (*model.Strategy, error) { return nil, nil }
func (m *mockCommander) Review(_ context.Context, _ *model.BattleReport) error { return nil }
func (m *mockCommander) GetOrder(_ context.Context, _ model.OrderID) (*model.Order, error) { return nil, nil }
func (m *mockCommander) ListOrders(_ context.Context, _ model.State) ([]*model.Order, error) { return nil, nil }

func TestCreateOrder_Success(t *testing.T) {
    gin.SetMode(gin.TestMode)
    mock := &mockCommander{}
    h := &Handler{c: mock}
    r := gin.New()
    r.POST("/orders", h.CreateOrder)
    body := `{"name":"test","description":"d","priority":2,"objectives":["o1"]}`
    req := httptest.NewRequest("POST", "/orders", bytes.NewBufferString(body))
    req.Header.Set("Content-Type", "application/json")
    w := httptest.NewRecorder()
    r.ServeHTTP(w, req)
    require.Equal(t, http.StatusOK, w.Code)
    assert.NotNil(t, mock.lastOrder)
    assert.Equal(t, "test", mock.lastOrder.Name)
}
```

- [ ] **Step 6：commit**

```bash
git add pkg/transport/http/
git commit -m "feat(transport/http): gin server + 5 middlewares + 7 handlers with tests"
```

### Task 4.2：gRPC transport

**Files:**
- Create: `pkg/transport/grpc/{server.go,service/*.go,interceptor/*.go}`

- [ ] **Step 1：写 server.go**

```go
// pkg/transport/grpc/server.go
package grpc

import (
    "net"
    "google.golang.org/grpc"
    "github.com/zhuge/kongming/pkg/domain/port"
    "github.com/zhuge/kongming/pkg/transport/grpc/interceptor"
    "github.com/zhuge/kongming/pkg/transport/grpc/service"
    "go.uber.org/zap"
)

type Server struct {
    srv *grpc.Server
    lis net.Listener
}

type Deps struct {
    Commander  port.Commander
    Dispatcher port.Dispatcher
    Engine     port.Engine
    Pool       port.GeneralPool
    Vault      port.Vault
    Logger     *zap.Logger
    Addr       string
}

func NewServer(d Deps) (*Server, error) {
    lis, err := net.Listen("tcp", d.Addr)
    if err != nil { return nil, err }
    s := grpc.NewServer(
        grpc.UnaryInterceptor(interceptor.Chain(
            interceptor.TraceID(),
            interceptor.Logging(d.Logger),
            interceptor.Recovery(d.Logger),
        )),
    )
    pb.RegisterKongmingServer(s, service.New(d.Commander, d.Dispatcher, d.Engine, d.Pool, d.Vault))
    return &Server{srv: s, lis: lis}, nil
}

func (s *Server) Serve() error { return s.srv.Serve(s.lis) }
func (s *Server) GracefulStop() { s.srv.GracefulStop() }
```

- [ ] **Step 2：写 service/order.go（gRPC 实现）**

```go
// pkg/transport/grpc/service/order.go
package service

import (
    "context"
    "github.com/google/uuid"
    pb "github.com/zhuge/kongming/api/proto/kongming/v1"
    domerrs "github.com/zhuge/kongming/pkg/domain/errors"
    "github.com/zhuge/kongming/pkg/domain/model"
    "github.com/zhuge/kongming/pkg/domain/port"
    "google.golang.org/grpc/codes"
    "google.golang.org/grpc/status"
)

type Service struct {
    pb.UnimplementedKongmingServer
    c  port.Commander
    d  port.Dispatcher
    e  port.Engine
    p  port.GeneralPool
    v  port.Vault
}

func New(c port.Commander, d port.Dispatcher, e port.Engine, p port.GeneralPool, v port.Vault) *Service {
    return &Service{c: c, d: d, e: e, p: p, v: v}
}

func (s *Service) Dispatch(ctx context.Context, req *pb.DispatchRequest) (*pb.DispatchResponse, error) {
    order := &model.Order{
        ID: model.OrderID(uuid.NewString()),
        Name: req.GetName(), Description: req.GetDescription(),
        State: model.StatePending, Priority: model.Priority(req.GetPriority()),
        Strategy: model.Strategy{Objectives: []string{}, /* 解析 req.Objectives */},
    }
    report, err := s.c.Dispatch(ctx, order)
    if err != nil {
        if e, ok := err.(*domerrs.Error); ok {
            return nil, status.Error(e.Code.GRPCCode(), e.Error())
        }
        return nil, status.Error(codes.Internal, err.Error())
    }
    return &pb.DispatchResponse{OrderId: string(order.ID), Success: report.Success, Message: report.Message}, nil
}

// 其他 6 个 RPC 方法类似
```

- [ ] **Step 3：写 interceptor/traceid.go + logging.go + recovery.go**

略（30-50 行/文件，复用 traceId 包）

- [ ] **Step 4：写 service_test.go（用 bufconn）**

```go
// pkg/transport/grpc/service/order_test.go
package service

import (
    "context"
    "net"
    "testing"
    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"
    pb "github.com/zhuge/kongming/api/proto/kongming/v1"
    "github.com/zhuge/kongming/pkg/domain/model"
    "google.golang.org/grpc"
    "google.golang.org/grpc/credentials/insecure"
    "google.golang.org/grpc/test/bufconn"
)

func TestService_Dispatch(t *testing.T) {
    lis := bufconn.Listen(1024 * 64)
    s := grpc.NewServer()
    pb.RegisterKongmingServer(s, New(&mockCommander{}, nil, nil, nil, nil))
    go s.Serve(lis)
    defer s.Stop()

    conn, err := grpc.NewClient("passthrough://bufnet", grpc.WithContextDialer(func(_ context.Context, _ string) (net.Conn, error) {
        return lis.Dial()
    }), grpc.WithTransportCredentials(insecure.NewCredentials()))
    require.NoError(t, err)
    defer conn.Close()

    cli := pb.NewKongmingClient(conn)
    resp, err := cli.Dispatch(context.Background(), &pb.DispatchRequest{Name: "test", Priority: 2})
    require.NoError(t, err)
    assert.True(t, resp.Success)
}
```

- [ ] **Step 5：commit**

```bash
git add pkg/transport/grpc/ api/proto/
git commit -m "feat(transport/grpc): gRPC server with interceptors + 7 RPC methods"
```

### Task 4.3：CLI transport（cobra）

**Files:**
- Create: `pkg/transport/cli/{root.go,server.go,dispatch.go,strategy.go,general.go,vault.go,plugin.go}`

- [ ] **Step 1：写 root.go**

```go
// pkg/transport/cli/root.go
package cli

import (
    "github.com/spf13/cobra"
)

func NewRootCmd() *cobra.Command {
    root := &cobra.Command{
        Use:   "kongming",
        Short: "Kongming 孔明军师系统",
    }
    root.PersistentFlags().StringP("config", "c", "configs/kongming.yaml", "config file path")
    root.AddCommand(newServerCmd(), newDispatchCmd(), newStrategyCmd(), newGeneralCmd(), newVaultCmd(), newPluginCmd())
    return root
}
```

- [ ] **Step 2：写 server.go（启动 HTTP+gRPC）**

```go
// pkg/transport/cli/server.go
package cli

import (
    "context"
    "github.com/spf13/cobra"
    "github.com/zhuge/kongming/pkg/kongming"
)

func newServerCmd() *cobra.Command {
    return &cobra.Command{
        Use:   "server",
        Short: "Start HTTP+gRPC server",
        RunE: func(cmd *cobra.Command, args []string) error {
            cfgPath, _ := cmd.Flags().GetString("config")
            k, err := kongming.New(cfgPath)
            if err != nil { return err }
            ctx := cmd.Context()
            return k.Run(ctx)
        },
    }
}
```

- [ ] **Step 3：写 dispatch.go + strategy.go + general.go + vault.go + plugin.go**

每个 ~30-50 行：解析 flag → 调对应 application service → 输出 JSON/YAML

- [ ] **Step 4：写 cli_test.go（ExecuteC）**

```go
// pkg/transport/cli/cli_test.go
package cli

import (
    "bytes"
    "testing"
    "github.com/stretchr/testify/assert"
)

func TestRootCmd(t *testing.T) {
    cmd := NewRootCmd()
    var buf bytes.Buffer
    cmd.SetOut(&buf)
    cmd.SetArgs([]string{"--help"})
    err := cmd.Execute()
    assert.NoError(t, err)
    assert.Contains(t, buf.String(), "kongming")
}
```

- [ ] **Step 5：commit**

```bash
git add pkg/transport/cli/
git commit -m "feat(transport/cli): cobra root + server + dispatch + 5 subcommands"
```

### Task 4.4：transport 阶段 CI 验证

```bash
go test ./pkg/transport/... -race -v
git commit -m "chore(transport): green with 70%+ coverage"
```

---

## 阶段 5 — 顶层装配（pkg/kongming + cmd/）

### Task 5.1：kongming.New 装配

**Files:**
- Create: `pkg/kongming/kongming.go`
- Create: `pkg/kongming/options.go`

- [ ] **Step 1：写 kongming.go（按 spec §6.1）**

```go
// pkg/kongming/kongming.go
package kongming

import (
    "context"
    "fmt"
    "net"
    "github.com/zhuge/kongming/pkg/application/commander"
    "github.com/zhuge/kongming/pkg/application/courier"
    "github.com/zhuge/kongming/pkg/application/dispatcher"
    "github.com/zhuge/kongming/pkg/application/general"
    "github.com/zhuge/kongming/pkg/application/vault"
    "github.com/zhuge/kongming/pkg/application/workflow"
    "github.com/zhuge/kongming/pkg/domain/port"
    "github.com/zhuge/kongming/pkg/infra/config"
    "github.com/zhuge/kongming/pkg/infra/observability"
    "github.com/zhuge/kongming/pkg/infra/persistence/memory"
    "github.com/zhuge/kongming/pkg/infra/plugin"
    "github.com/zhuge/kongming/pkg/infra/resilience"
    "github.com/zhuge/kongming/pkg/transport/grpc"
    "github.com/zhuge/kongming/pkg/transport/http"
    "go.uber.org/zap"
)

type Kongming struct {
    cfg        *config.Config
    logger     *zap.Logger
    observer   *observability.Observer
    resilient  *resilience.Runner
    commander  *commander.Service
    dispatcher *dispatcher.Service
    engine     *workflow.Runner
    pool       *general.Pool
    vault      *vault.Service
    courier    *courier.Service
    pluginReg  *plugin.Registry
    httpSrv    *http.Server
    grpcSrv    *grpc.Server
}

type Options struct {
    ServiceName string
}

func New(cfgPath string, opts ...Options) (*Kongming, error) {
    o := Options{ServiceName: "kongming"}
    if len(opts) > 0 { o = opts[0] }

    cfg, err := config.Load(cfgPath)
    if err != nil { return nil, fmt.Errorf("config: %w", err) }

    logger, err := observability.NewLogger(cfg.Observatory.Log)
    if err != nil { return nil, err }

    observer, err := observability.NewObserver(context.Background(), cfg.Observatory, logger)
    if err != nil { return nil, err }

    resilient := resilience.NewRunner(cfg.Resilience, logger)

    store := memory.NewStore()
    orderRepo := memory.NewOrderRepo(store)
    generalRepo := memory.NewGeneralRepo(store)

    pluginReg := plugin.NewRegistry()
    pool := general.NewPool(cfg.Generals, generalRepo, logger)
    vlt := vault.New(cfg.Vault, pluginReg, logger, observer)
    cou := courier.New(cfg.Courier, logger, observer)
    eng := workflow.NewRunner(logger)
    disp := dispatcher.New(cfg.Dispatcher, pool, vlt, resilient, logger, observer)
    cmd := commander.New(commander.NewDefaultPlanner(), pool, eng, vlt, orderRepo, resilient, observer, logger)

    return &Kongming{
        cfg: cfg, logger: logger, observer: observer, resilient: resilient,
        commander: cmd, dispatcher: disp, engine: eng,
        pool: pool, vault: vlt, courier: cou, pluginReg: pluginReg,
    }, nil
}

func (k *Kongming) Run(ctx context.Context) error {
    if k.cfg.Plugin.Watch {
        if err := k.pluginReg.Watch(ctx, k.cfg.Plugin.Dir, k.logger); err != nil {
            k.logger.Warn("plugin watch failed", zap.Error(err))
        }
    }
    httpAddr := fmt.Sprintf("%s:%d", k.cfg.Server.Host, k.cfg.Server.Port)
    k.httpSrv = http.NewServer(http.Deps{
        Commander: k.commander, Dispatcher: k.dispatcher, Engine: k.engine,
        Pool: k.pool, Vault: k.vault, Observer: k.observer,
        Logger: k.logger, Addr: httpAddr,
    })
    go func() { _ = k.httpSrv.ListenAndServe() }()

    grpcAddr := fmt.Sprintf("%s:%d", k.cfg.Server.Host, k.cfg.Server.GRPCPort)
    grpcSrv, err := grpc.NewServer(grpc.Deps{
        Commander: k.commander, Dispatcher: k.dispatcher, Engine: k.engine,
        Pool: k.pool, Vault: k.vault, Logger: k.logger, Addr: grpcAddr,
    })
    if err != nil { return err }
    k.grpcSrv = grpcSrv
    go func() { _ = grpcSrv.Serve() }()

    <-ctx.Done()
    return k.Shutdown(context.Background())
}

func (k *Kongming) Shutdown(ctx context.Context) error {
    if k.httpSrv != nil { _ = k.httpSrv.Shutdown(ctx) }
    if k.grpcSrv != nil { k.grpcSrv.GracefulStop() }
    if k.observer != nil { _ = k.observer.Shutdown(ctx) }
    _ = k.logger.Sync()
    return nil
}
```

> 注意：实际实现中，listener 已由 grpc.NewServer 内部创建并存储；Serve 直接调用。这里简化了示例。

- [ ] **Step 2：写 kongming_test.go（集成测试）**

```go
// pkg/kongming/kongming_test.go
package kongming

import (
    "testing"
    "time"
    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"
)

func TestNew_ValidConfig(t *testing.T) {
    k, err := New("../../configs/kongming.yaml")
    require.NoError(t, err)
    require.NotNil(t, k)
    assert.NotNil(t, k.commander)
    assert.NotNil(t, k.pool)
}

func TestNew_MissingConfig(t *testing.T) {
    _, err := New("nope.yaml")
    assert.Error(t, err)
}

// 集成测试：启动 → 调 HTTP → 关闭
func TestRun_StartStop(t *testing.T) {
    if testing.Short() { t.Skip() }
    k, err := New("../../configs/kongming.yaml")
    require.NoError(t, err)
    ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
    defer cancel()
    err = k.Run(ctx)
    assert.NoError(t, err)
}
```

- [ ] **Step 3：commit**

```bash
git add pkg/kongming/
git commit -m "feat(kongming): top-level assembly with config-driven wiring"
```

### Task 5.2：cmd/kongming-server + cmd/kongming

**Files:**
- Create: `cmd/kongming-server/main.go`
- Create: `cmd/kongming/main.go`
- Delete: `cmd/kongming/main.go`（原文件，被替换）

- [ ] **Step 1：写 cmd/kongming-server/main.go**

```go
// cmd/kongming-server/main.go
package main

import (
    "context"
    "flag"
    "log"
    "os/signal"
    "syscall"
    "github.com/zhuge/kongming/pkg/kongming"
)

func main() {
    cfgPath := flag.String("config", "configs/kongming.yaml", "config path")
    flag.Parse()

    k, err := kongming.New(*cfgPath)
    if err != nil { log.Fatalf("init: %v", err) }

    ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
    defer cancel()
    if err := k.Run(ctx); err != nil { log.Fatalf("run: %v", err) }
}
```

- [ ] **Step 2：写 cmd/kongming/main.go（CLI 入口）**

```go
// cmd/kongming/main.go
package main

import (
    "os"
    "github.com/zhuge/kongming/pkg/transport/cli"
)

func main() {
    root := cli.NewRootCmd()
    if err := root.Execute(); err != nil { os.Exit(1) }
}
```

- [ ] **Step 3：删除原 cmd/kongming/main.go**

```bash
git rm cmd/kongming/main.go
```

- [ ] **Step 4：编译**

```bash
go build -o bin/kongming-server ./cmd/kongming-server
go build -o bin/kongming ./cmd/kongming
ls -lh bin/
```

- [ ] **Step 5：冒烟测试（启动 2 秒后停止）**

```bash
timeout 3 ./bin/kongming-server --config configs/kongming.yaml &
sleep 2
curl -s http://localhost:8080/api/v1/generals | head -100
curl -s http://localhost:9090/metrics | head -5
kill %1
```
Expected: 返回 JSON + prom 指标

- [ ] **Step 6：commit**

```bash
git add cmd/ bin/
git commit -m "feat(cmd): kongming-server (HTTP+gRPC) and kongming CLI"
```

### Task 5.3：examples 补齐

**Files:**
- Create: `examples/{quickstart,longzhong_strategy,wuhu_campaign,zhuge_bagua}/main.go`

> 详细实现按 spec §1 examples/quickstart 等。每个 80-120 行 + README.md 说明。

- [ ] **Step 1：写 examples/quickstart/main.go**

```go
// examples/quickstart/main.go
package main

import (
    "context"
    "fmt"
    "github.com/zhuge/kongming/pkg/application/commander"
    "github.com/zhuge/kongming/pkg/application/general"
    "github.com/zhuge/kongming/pkg/application/vault"
    "github.com/zhuge/kongming/pkg/application/workflow"
    "github.com/zhuge/kongming/pkg/domain/model"
    "github.com/zhuge/kongming/pkg/infra/config"
    "github.com/zhuge/kongming/pkg/infra/observability"
    "github.com/zhuge/kongming/pkg/infra/persistence/memory"
    "github.com/zhuge/kongming/pkg/infra/plugin"
    "github.com/zhuge/kongming/pkg/infra/resilience"
    "github.com/google/uuid"
    "go.uber.org/zap"
)

func main() {
    cfg, _ := config.Load("configs/kongming.yaml")
    logger, _ := observability.NewLogger(cfg.Observatory.Log)
    observer, _ := observability.NewObserver(context.Background(), cfg.Observatory, logger)
    res := resilience.NewRunner(cfg.Resilience, logger)
    store := memory.NewStore()
    repo := memory.NewOrderRepo(store)
    pool := general.NewPool(cfg.Generals, memory.NewGeneralRepo(store), logger)
    vlt := vault.New(cfg.Vault, plugin.NewRegistry(), logger, observer)
    eng := workflow.NewRunner(logger)
    cmd := commander.New(commander.NewDefaultPlanner(), pool, eng, vlt, repo, res, observer, logger)

    order := &model.Order{
        ID: model.OrderID(uuid.NewString()),
        Name: "市场调研",
        State: model.StatePending,
        Priority: model.PriorityNormal,
        Strategy: model.Strategy{Objectives: []string{"搜集数据", "生成报告"}},
    }
    report, err := cmd.Dispatch(context.Background(), order)
    if err != nil { logger.Fatal("dispatch", zap.Error(err)) }
    fmt.Printf("战报: success=%v generals=%d\n", report.Success, len(report.Generals))
}
```

- [ ] **Step 2：examples 跑通**

```bash
go run ./examples/quickstart/main.go
```
Expected: 打印 "战报: success=true generals=..."

- [ ] **Step 3：commit**

```bash
git add examples/
git commit -m "docs(examples): add quickstart example using refactored SDK"
```

---

## 阶段 6 — 清理旧代码

### Task 6.1：删除旧 pkg

- [ ] **Step 1：删除旧目录**

```bash
git rm -r pkg/bagua pkg/cmd_center pkg/courier pkg/dispatch pkg/generals pkg/observatory pkg/repeater pkg/strategy_vault
git rm -r internal/memory
```

- [ ] **Step 2：确认 build 通过**

```bash
go build ./...
go test ./... -race
```

- [ ] **Step 3：commit**

```bash
git commit -m "chore: remove legacy pkg/* and internal/memory after migration"
```

### Task 6.2：更新 configs

**Files:**
- Modify: `configs/kongming.yaml`

- [ ] **Step 1：按新 schema 重写**

```yaml
# configs/kongming.yaml
version: "2.0"
server:
  host: "0.0.0.0"
  port: 8080
  grpc_port: 8081
  read_timeout: 30s
  write_timeout: 30s
  shutdown_timeout: 30s
features:
  enable_metrics: true
  enable_tracing: true
  enable_observatory: true
observatory:
  metrics_port: 9090
  tracing:
    enabled: true
    exporter: jaeger
    endpoint: "http://localhost:14268/api/traces"
    sampling_rate: 1.0
  log:
    level: info
    encoding: json
commander:
  default_timeout: 30s
  max_concurrent_orders: 100
dispatcher:
  queue_size: 1000
  timeout: 30s
generals:
  pool_size: 5
  default_timeout: 60s
bagua:
  default_mode: dizai
  max_parallel_nodes: 10
vault:
  dir: "./strategies"
  auto_reload: true
  builtin_only: false
courier:
  inbox_size: 1000
  outbox_size: 1000
  delivery_timeout: 30s
resilience:
  retry:
    max_attempts: 3
    initial_backoff: 100ms
    max_backoff: 30s
    backoff_factor: 2.0
    jitter: true
  circuit_breaker:
    threshold: 5
    timeout: 60s
  rate_limit:
    rps: 1000
    burst: 2000
plugin:
  dir: "./plugins"
  extensions: [".so", ".yaml"]
  watch: true
```

- [ ] **Step 2：commit**

```bash
git add configs/
git commit -m "chore(configs): rewrite kongming.yaml for v2 schema"
```

### Task 6.3：更新 CI

**Files:**
- Modify: `.github/workflows/ci.yml`

- [ ] **Step 1：完整 CI 流水线**

```yaml
name: CI
on: [push, pull_request]
jobs:
  lint:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with: { go-version: '1.21' }
      - run: go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
      - run: golangci-lint run --timeout=5m ./...

  test:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with: { go-version: '1.21' }
      - run: go test -race -coverprofile=coverage.out ./...

  coverage:
    needs: test
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with: { go-version: '1.21' }
      - run: |
          go test -coverprofile=coverage.out ./... > /dev/null
          COV=$(go tool cover -func=coverage.out | tail -1 | awk '{print $3}' | tr -d '%')
          echo "Coverage: ${COV}%"
          if [ "${COV%.*}" -lt 80 ]; then echo "FAIL: < 80%"; exit 1; fi

  build:
    needs: [lint, coverage]
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with: { go-version: '1.21' }
      - run: go build ./cmd/...

  docker:
    needs: build
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - run: docker buildx build --load -t zhuge/kongming:latest .
```

- [ ] **Step 2：commit**

```bash
git add .github/
git commit -m "ci: add coverage gate + race + lint + build + docker pipeline"
```

### Task 6.4：更新 README

- [ ] **Step 1：替换 README.md**

```markdown
# Kongming 孔明军师系统 v2.0

> 「运筹帷幄之中，决胜千里之外」

## 架构

[完整 README 按新架构重写：包含 ASCII 架构图 + SDK 用法 + CLI 示例 + gRPC 示例 + 部署说明 + 开发指南 + 测试指南]
```

- [ ] **Step 2：commit**

```bash
git add README.md
git commit -m "docs(readme): rewrite for v2 three-tier architecture"
```

---

## 阶段 7 — 文档与冒烟

### Task 7.1：docs/MIGRATION.md

- [ ] **Step 1：写迁移指南**

```markdown
# v1 → v2 迁移指南

[详细列出 pkg 路径映射、import 替换、Config schema 变化、CLI 变化、HTTP API 变化]
```

- [ ] **Step 2：commit**

```bash
git add docs/MIGRATION.md
git commit -m "docs: add v1 to v2 migration guide"
```

### Task 7.2：端到端冒烟

- [ ] **Step 1：启动服务**

```bash
make build
./bin/kongming-server --config configs/kongming.yaml &
SERVER_PID=$!
sleep 3
```

- [ ] **Step 2：HTTP 冒烟**

```bash
curl -s -X POST http://localhost:8080/api/v1/orders \
  -H "Content-Type: application/json" \
  -d '{"name":"test","description":"d","priority":2,"objectives":["o1"]}' | jq
curl -s http://localhost:8080/api/v1/generals | jq
curl -s http://localhost:8080/healthz
curl -s http://localhost:9090/metrics | head -5
```

- [ ] **Step 3：CLI 冒烟**

```bash
./bin/kongming general list --config configs/kongming.yaml
./bin/kongming vault list --config configs/kongming.yaml
./bin/kongming strategy list --config configs/kongming.yaml
```

- [ ] **Step 4：停止服务**

```bash
kill $SERVER_PID
```

- [ ] **Step 5：commit（如有 fix）**

```bash
git add -A
git commit -m "chore: end-to-end smoke verified"
```

### Task 7.3：全量测试 + 覆盖率验证

- [ ] **Step 1：跑全量测试 + race**

```bash
go test ./... -race -v
```
Expected: 全部 PASS

- [ ] **Step 2：跑覆盖率门禁**

```bash
make coverage-gate
```
Expected: Coverage ≥ 80%

- [ ] **Step 3：跑 lint**

```bash
make lint
```
Expected: 0 issues

- [ ] **Step 4：跑 CI 全流程**

```bash
make ci
```
Expected: 全部 PASS

- [ ] **Step 5：commit（如有 fix）**

```bash
git add -A
git commit -m "chore: final CI pipeline green at 80%+ coverage"
```

---

## 总结

**7 个阶段，~40 个任务，~80 个 steps**。每步 2-5 分钟可执行；每步结束可单独 commit。

**关键质量门禁**：
- 阶段 1 末：`pkg/infra` + `pkg/domain` 测试通过
- 阶段 3 末：application 覆盖率 ≥ 90%
- 阶段 4 末：transport 覆盖率 ≥ 70%
- 阶段 7 末：整体覆盖率 ≥ 80%；CI 全绿

**风险点**：
- Task 3.3（4 个子模块）工作量较大，可拆分为多 agent 并行
- Task 4.2（gRPC）依赖 proto 生成；buf 工具链需先就绪
- Task 6.1（删除旧代码）需确保 build 通过，可分批删除

**后续可选（不在本 plan）**：
- 接入真实 LLM provider（OpenAI / Anthropic）作为 NodeExecutor SPI
- 接入 Redis 后端作为 OrderRepository
- K8s Operator / Helm chart
