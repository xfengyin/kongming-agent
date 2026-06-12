# Kongming 重构设计文档

> **项目**：Kongming 孔明军师系统（多 Agent 编排框架）
> **日期**：2026-06-12
> **作者**：MiniMax-M3（brainstorming 输出）
> **状态**：草案，待用户评审
> **范围**：pkg/ 与 cmd/ 重构，允许 breaking change

---

## 0. 设计目标与背景

### 0.1 现有问题（已识别 15 项）

| # | 问题 | 位置 | 影响 |
|---|---|---|---|
| 1 | 配置未加载 | `cmd/kongming/main.go` | 启动无法定制 |
| 2 | 依赖未注入 | `pkg/cmd_center/commander.go` | 难替换/难测试 |
| 3 | observatory 全局 tracer | `pkg/observatory/observatory.go` | 多实例竞态 |
| 4 | 八卦阵缺 3 阵 | `pkg/bagua/engine.go` | Huyi/Niaoxiang/Shepan 未实现 |
| 5 | cmd_center / dispatch 职责重叠 | `pkg/cmd_center` + `pkg/dispatch` | 概念混乱 |
| 6 | generals 状态竞态 | `pkg/generals/types.go` | Execute 无锁 |
| 7 | 熔断器 HalfOpen 缺陷 | `pkg/repeater/repeater.go` | 多 goroutine 试探 |
| 8 | strategy_vault.LoadFromDir 是 TODO | `pkg/strategy_vault/types.go` | 占位 |
| 9 | traceId 未透传 | 全局 | 链路断 |
| 10 | 重试/熔断未业务化 | `pkg/repeater` | 实现孤立 |
| 11 | 无插件化 SPI 加载 | `pkg/bagua` 等 | 配置驱动受限 |
| 12 | memory.Search limit 无效 | `internal/memory/memory.go` | Bug |
| 13 | repeater.NewReperier 拼写错误 | `pkg/repeater/repeater.go` | 命名 Bug |
| 14 | CI 单测覆盖未设门槛 | `.github/workflows/ci.yml` | 质量漂移 |
| 15 | 缺 mock 工具与集成测试 | 全局 | 难维护 |

### 0.2 重构目标

1. **架构合规**：满足 12 条企业级规则（开闭/依赖倒置/单一职责/接口隔离/高可用/可观测/配置驱动/插件化/幂等/安全/性能/可测试）
2. **交付形态**：Go SDK + HTTP/gRPC 服务 + CLI（cobra）
3. **质量门槛**：单元测试 ≥ 80%、race detector 必跑、golangci-lint 零告警
4. **插件化**：bagua NodeExecutor、GeneralHandler、JinnangHandler 全部 SPI，支持进程内注册 + .so 动态加载 + fsnotify 热更新
5. **可观测**：traceId 端到端透传；结构化日志 + Prometheus + OpenTelemetry
6. **弹性**：retry/circuitbreaker/ratelimit/timeout 装饰器链式组合
7. **可扩展**：新增传输协议/新持久化后端/新 LLM provider 不侵入业务

### 0.3 非目标（本次不做）

- 真实 LLM provider 接入（保留 SPI 占位）
- 多实例分布式调度（Commander 仍单进程）
- 真实持久化后端（保留 memory，预留 redis 接口）
- 用户管理/认证/鉴权（占位中间件位置）

---

## 1. 顶层目录与依赖方向

```
kongming/
├── cmd/
│   ├── kongming-server/      # 服务入口：HTTP + gRPC
│   │   └── main.go
│   └── kongming/             # CLI 入口：cobra 子命令
│       └── main.go
│
├── pkg/
│   ├── kongming/             # 顶层统一入口：kongming.New(cfg) → *Kongming
│   │                          # 负责装配各层
│   │
│   ├── domain/               # 领域层：纯模型 + 端口接口（零外部依赖）
│   │   ├── model/            # 实体：Order, Strategy, General, Jinnang, ...
│   │   ├── port/             # 端口：Commander, Engine, GeneralPool, Vault, ...
│   │   └── errors/           # 领域错误
│   │
│   ├── application/          # 应用层：编排与用例
│   │   ├── commander/        # 军师用例
│   │   ├── dispatcher/       # 调度用例
│   │   ├── workflow/         # 工作流用例（八卦阵 8 阵）
│   │   ├── general/          # 将领调度用例
│   │   ├── vault/            # 锦囊库用例
│   │   └── courier/          # 传令兵用例
│   │
│   ├── transport/            # 输入适配器
│   │   ├── http/             # gin 路由
│   │   ├── grpc/             # gRPC service
│   │   └── cli/              # cobra 子命令
│   │
│   └── infra/                # 基础设施（输出适配器）
│       ├── config/           # viper 加载
│       ├── observability/    # prom + otel + traceId
│       ├── resilience/       # retry + circuitbreaker + ratelimit + timeout
│       ├── persistence/      # memory（后续 redis/db）
│       └── plugin/           # go plugin + fsnotify 热更新
│
├── api/proto/                # gRPC proto 定义
│
├── configs/
│   └── kongming.yaml
│
├── examples/                 # 4 个示例
│   ├── quickstart/
│   ├── longzhong_strategy/
│   ├── wuhu_campaign/
│   └── zhuge_bagua/
│
├── deployments/              # docker + prometheus + grafana
│
├── docs/superpowers/
│   ├── specs/                # 设计文档
│   └── plans/                # 实施计划
│
├── go.mod
├── Makefile
└── README.md
```

### 1.1 依赖方向规则（goimports 校验）

- `domain` → 不允许 import 任何其他 kongming 子包（只依赖标准库）
- `application` → 可 import `domain`；禁止 import `transport` / `infra`（仅依赖其接口在 domain/port）
- `infra` → 可 import `domain` + 第三方库；禁止 import `application` / `transport`
- `transport` → 可 import `application` + `domain` + `infra`（用于 wire）
- `kongming` 顶层 → 可 import 全部，负责装配

**校验方式**：在 CI 中加入 `go vet` 自定义规则 + `arch-go` 或手写 import 边界检查脚本。

---

## 2. domain 层（领域模型与端口）

domain 层是**纯 Go**，不依赖任何第三方库，不依赖 kongming 内部其他包。

### 2.1 目录

```
pkg/domain/
├── model/                  # 实体
│   ├── order.go            # Order, OrderID, Priority, State
│   ├── strategy.go         # Strategy, Tactic, BaguaMode
│   ├── general.go          # General, GeneralType, GeneralStats
│   ├── jinnang.go          # Jinnang, JinnangSpec, JinnangInput/Output
│   ├── workflow.go         # Workflow, Node, Edge
│   ├── message.go          # Message, MessageType
│   └── report.go           # BattleReport, GeneralReport
│
├── port/                   # 端口（接口）
│   ├── commander.go        # Commander
│   ├── dispatcher.go       # Dispatcher
│   ├── engine.go           # Engine（工作流）
│   ├── general_pool.go     # GeneralPool
│   ├── vault.go            # Vault
│   ├── courier.go          # Courier
│   ├── resilience.go       # ResilientRunner
│   ├── observer.go         # Observer
│   ├── plugin.go           # PluginRegistry（接口）
│   └── persistence.go      # OrderRepository, GeneralRepository
│
└── errors/
    ├── code.go             # 错误码枚举
    └── errors.go           # Error struct + helpers
```

### 2.2 关键模型示例

```go
// pkg/domain/model/order.go
package model

import "time"

type OrderID string
type Priority int
const ( PriorityLow Priority = iota; PriorityNormal; PriorityHigh; PriorityUrgent )

type State int
const (
    StatePending State = iota
    StatePlanning
    StateExecuting
    StateReviewing
    StateCompleted
    StateFailed
)

// 状态机：合法转换表
var stateTransitions = map[State][]State{
    StatePending:   {StatePlanning, StateFailed},
    StatePlanning:  {StateExecuting, StateFailed},
    StateExecuting: {StateReviewing, StateFailed},
    StateReviewing: {StateCompleted, StateFailed},
    StateCompleted: {},
    StateFailed:    {StatePending}, // 允许失败重试
}

func (s State) TransitionTo(next State) error {
    for _, allowed := range stateTransitions[s] {
        if allowed == next { return nil }
    }
    return errors.New("invalid state transition: %s -> %s", s, next)
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

### 2.3 关键端口示例

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

### 2.4 端口设计原则

- **接口隔离（ISP）**：每个端口只暴露一个用例域的最小能力集合
- **依赖倒置（DIP）**：application 依赖接口，infra 实现接口，transport 调用 application
- **不暴露实现类型**：端口方法签名只使用 `model.*` 和 `port.*` 类型

---

## 3. application 层（用例编排）

application 层负责**编排领域端口**实现具体用例，**不直接依赖**任何 infra 实现；通过接口注入。

### 3.1 目录

```
pkg/application/
├── commander/
│   ├── service.go          # 实现 domain/port.Commander
│   ├── planner.go          # 战略规划
│   └── idempotent.go       # 幂等性：相同 order_id 第二次返回缓存
│
├── dispatcher/
│   ├── service.go          # 实现 domain/port.Dispatcher
│   ├── executor.go         # Executor SPI 注入
│   └── selector.go         # Executor 选择策略
│
├── workflow/
│   ├── runner.go           # 实现 domain/port.Engine
│   ├── modes/
│   │   ├── tiangai.go      # 并行（DAG 拓扑层级）
│   │   ├── dizai.go        # 顺序
│   │   ├── fengyang.go     # 快速响应（带超时）
│   │   ├── yunzhui.go      # 容错重试
│   │   ├── longfei.go      # 动态调度（critical path）
│   │   ├── huyi.go         # 条件分支【补齐】
│   │   ├── niaoxiang.go    # 扇形扩散【补齐】
│   │   └── shepan.go       # 循环迭代【补齐】
│   ├── node/
│   │   ├── llm_node.go     # SPI 占位
│   │   ├── tool_node.go    # SPI 占位
│   │   └── condition_node.go
│   └── dag.go              # 共享：DAG 构建 + 拓扑排序
│
├── general/
│   ├── pool.go             # 实现 domain/port.GeneralPool
│   ├── wuhu/
│   │   ├── guanyu.go
│   │   ├── zhangfei.go
│   │   ├── zhaoyun.go
│   │   ├── machao.go
│   │   └── huangzhong.go
│   └── selector.go         # 评分/选择
│
├── vault/
│   ├── service.go          # 实现 domain/port.Vault
│   ├── loader.go           # 从目录/插件加载
│   └── builtin/
│       ├── huogong.go      # 火攻
│       ├── shuibo.go       # 水淹
│       └── kongcheng.go    # 空城
│
└── courier/
    ├── service.go          # 实现 domain/port.Courier
    └── delivery.go         # 投递状态机
```

### 3.2 Commander 编排（伪代码）

```go
type commanderService struct {
    cfg        config.CommanderConfig
    planner    Planner
    dispatcher port.Dispatcher
    engine     port.Engine
    pool       port.GeneralPool
    vault      port.Vault
    orders     port.OrderRepository
    resilient  port.ResilientRunner
    observer   port.Observer
    logger     *zap.Logger
}

func (s *commanderService) Dispatch(ctx context.Context, order *model.Order) (*model.BattleReport, error) {
    // 1. traceId 透传
    ctx, span := s.observer.StartSpan(ctx, "commander.Dispatch",
        trace.WithAttributes(attribute.String("order.id", string(order.ID))))
    defer span.End()

    // 2. 幂等检查
    if existing, err := s.orders.Get(ctx, order.ID); err == nil && existing != nil {
        s.observer.RecordEvent(ctx, "commander.idempotent_replay", order.ID)
        return s.replayReport(ctx, existing)
    }

    // 3. 状态机转换
    if err := order.State.TransitionTo(model.StatePlanning); err != nil {
        return nil, errs.Wrap(errs.CodeInvalidState, err)
    }
    order.UpdatedAt = time.Now()

    // 4. 制定战略
    strategy, err := s.planner.Plan(ctx, order)
    if err != nil {
        return nil, errs.Wrap(errs.CodeStrategyFailed, err)
    }
    order.Strategy = *strategy
    if err := order.State.TransitionTo(model.StateExecuting); err != nil {
        return nil, errs.Wrap(errs.CodeInvalidState, err)
    }

    // 5. 入库（写时排序）
    if err := s.orders.Save(ctx, order); err != nil {
        return nil, errs.Wrap(errs.CodePersistence, err)
    }

    // 6. 弹性执行（注入重试/熔断/限流/超时）
    report, err := s.resilient.Run(ctx, "commander.dispatch", func(ctx context.Context) (*model.BattleReport, error) {
        return s.runTactics(ctx, order, strategy)
    })
    if err != nil {
        order.State = model.StateFailed
        _ = s.orders.Save(ctx, order)
        return nil, err
    }

    // 7. 审核
    if err := s.Review(ctx, report); err != nil {
        s.logger.Warn("review failed", zap.Error(err))
    }

    // 8. 完成
    if err := order.State.TransitionTo(model.StateCompleted); err != nil {
        return nil, err
    }
    _ = s.orders.Save(ctx, order)
    return report, nil
}
```

### 3.3 八卦阵补齐设计

- **Huyi（虎翼-条件分支）**：condition node 评估 `Edge.Condition` 表达式（CEL/简单 DSL），决定下游分支
- **Niaoxiang（鸟翔-扇形扩散）**：单一输入广播到多个并行分支，每个分支独立处理
- **Shepan（蛇蟠-循环迭代）**：loop node 含 `max_iterations` + `loop_var`，每轮迭代可访问上一轮输出

### 3.4 弹性执行链

```
[timeout] → [ratelimit] → [circuitbreaker] → [retry] → fn(ctx)
```

每个装饰器独立可配置；调用方只感知 `ResilientRunner.Run(ctx, name, fn)`。

---

## 4. infra 层（基础设施）

### 4.1 目录

```
pkg/infra/
├── config/
│   ├── loader.go           # viper 加载 + struct tag 绑定
│   ├── schema.go           # Config struct
│   └── env.go              # 环境变量覆盖
│
├── observability/
│   ├── logger.go            # zap structured logger
│   ├── metrics.go           # prom 指标注册
│   ├── tracing.go           # otel tracer + Jaeger/OTLP exporter
│   ├── traceid.go           # traceId 透传
│   ├── observer.go          # 实现 domain/port.Observer
│   └── http_middleware.go   # gin 中间件
│
├── resilience/
│   ├── runner.go            # 实现 domain/port.ResilientRunner
│   ├── retry.go             # 指数退避 + 抖动
│   ├── circuitbreaker.go    # 熔断器（修复 HalfOpen）
│   ├── ratelimit.go         # 令牌桶
│   └── timeout.go           # 统一超时
│
├── persistence/
│   ├── memory/              # 进程内 memory 仓库
│   │   ├── order_repo.go
│   │   ├── general_repo.go
│   │   └── memory_store.go
│   └── redis/               # 预留
│       └── README.md
│
└── plugin/
    ├── registry.go          # SPI 注册中心
    ├── loader.go            # go plugin 加载
    ├── watcher.go           # fsnotify 监听
    └── sandbox.go           # 沙箱（超时/重入限制）
```

### 4.2 关键设计

#### 4.2.1 配置走 struct tag + 校验

```go
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
```

启动时 viper 绑定到 Config struct；缺字段/格式错误立即报错。

#### 4.2.2 traceId 透传

```go
// pkg/infra/observability/traceid.go
type traceIDKey struct{}

func NewContext(ctx context.Context, id string) context.Context {
    return context.WithValue(ctx, traceIDKey{}, id)
}

func FromContext(ctx context.Context) string {
    if v, ok := ctx.Value(traceIDKey{}).(string); ok { return v }
    return ""
}

// HTTP 中间件
func TraceIDMiddleware() gin.HandlerFunc {
    return func(c *gin.Context) {
        id := c.GetHeader("X-Trace-Id")
        if id == "" { id = newTraceID() }
        ctx := traceid.NewContext(c.Request.Context(), id)
        c.Request = c.Request.WithContext(ctx)
        c.Header("X-Trace-Id", id)
        c.Next()
    }
}
```

#### 4.2.3 熔断器修复

```go
// pkg/infra/resilience/circuitbreaker.go
type CircuitBreaker struct {
    mu          sync.Mutex
    state       State
    failures    int
    threshold   int
    timeout     time.Duration
    lastFailure time.Time
    halfOpen    *atomic.Int32 // 修复：限制 HalfOpen 试探并发为 1
}

func (cb *CircuitBreaker) Call(ctx context.Context, fn func() error) error {
    cb.mu.Lock()
    switch cb.state {
    case StateOpen:
        if time.Since(cb.lastFailure) > cb.timeout {
            cb.state = StateHalfOpen
            cb.halfOpen.Store(1) // 标记已有试探
        } else {
            cb.mu.Unlock()
            return ErrCircuitOpen
        }
    case StateHalfOpen:
        if !cb.halfOpen.CompareAndSwap(0, 1) {
            cb.mu.Unlock()
            return ErrCircuitOpen // 已有 goroutine 在试探
        }
    }
    cb.mu.Unlock()

    err := fn()

    cb.mu.Lock()
    defer cb.mu.Unlock()
    if err != nil {
        cb.failures++
        cb.lastFailure = time.Now()
        if cb.failures >= cb.threshold {
            cb.state = StateOpen
        }
        return err
    }
    if cb.state == StateHalfOpen {
        cb.state = StateClosed
        cb.failures = 0
        cb.halfOpen.Store(0)
    }
    return nil
}
```

#### 4.2.4 plugin 热更新

```go
// pkg/infra/plugin/watcher.go
func (r *Registry) Watch(ctx context.Context) error {
    watcher, err := fsnotify.NewWatcher()
    if err != nil { return err }
    defer watcher.Close()

    go func() {
        for {
            select {
            case <-ctx.Done(): return
            case ev, ok := <-watcher.Events:
                if !ok { return }
                if ev.Op&(fsnotify.Write|fsnotify.Create) != 0 {
                    if err := r.Reload(ev.Name); err != nil {
                        r.logger.Error("plugin reload failed", zap.Error(err))
                    }
                }
            }
        }
    }()
    return watcher.Add(r.cfg.WatchDir)
}
```

#### 4.2.5 第三方依赖

- `go.uber.org/zap`：结构化日志
- `github.com/spf13/viper`：配置加载
- `github.com/spf13/cobra`：CLI
- `github.com/gin-gonic/gin`：HTTP
- `google.golang.org/grpc`：gRPC
- `github.com/prometheus/client_golang`：指标
- `go.opentelemetry.io/otel`：追踪
- `github.com/fsnotify/fsnotify`：文件监听
- `github.com/stretchr/testify`：测试
- `github.com/golang/mock` 或 `go.uber.org/mock`：mock 生成
- `github.com/go-playground/validator/v10`：配置校验

---

## 5. transport 层（HTTP / gRPC / CLI）

### 5.1 目录

```
pkg/transport/
├── http/
│   ├── server.go
│   ├── middleware/
│   │   ├── traceid.go
│   │   ├── logging.go
│   │   ├── recovery.go
│   │   └── cors.go
│   ├── handler/
│   │   ├── order.go
│   │   ├── strategy.go
│   │   ├── general.go
│   │   ├── vault.go
│   │   ├── workflow.go
│   │   ├── health.go
│   │   └── metrics.go
│   └── dto/
│
├── grpc/
│   ├── server.go
│   ├── service/
│   │   ├── order.go
│   │   ├── strategy.go
│   │   ├── general.go
│   │   ├── vault.go
│   │   └── workflow.go
│   └── interceptor/
│       ├── traceid.go
│       ├── logging.go
│       └── recovery.go
│
└── cli/
    ├── root.go
    ├── server.go
    ├── dispatch.go
    ├── strategy.go
    ├── general.go
    ├── vault.go
    └── plugin.go
```

### 5.2 HTTP API

```
POST   /api/v1/orders              # 提交军令
GET    /api/v1/orders              # 列出军令
GET    /api/v1/orders/:id          # 查询军令
GET    /api/v1/orders/:id/result   # 查询战报
POST   /api/v1/strategies          # 制定战略
GET    /api/v1/generals            # 列出五虎将
GET    /api/v1/generals/:id        # 查询将领
GET    /api/v1/vault               # 列出锦囊
POST   /api/v1/vault/:id/exec      # 执行锦囊
POST   /api/v1/workflows/:id/run   # 执行工作流
GET    /healthz                    # 存活
GET    /readyz                     # 就绪
GET    /metrics                    # prom
```

### 5.3 CLI 子命令

```
kongming server --config configs/kongming.yaml
kongming dispatch --config configs/kongming.yaml --order order.yaml
kongming strategy list --config configs/kongming.yaml
kongming general list --config configs/kongming.yaml
kongming vault list --config configs/kongming.yaml
kongming vault exec <id> --input input.json
kongming plugin list
kongming plugin reload
```

### 5.4 gRPC service

```proto
// api/proto/kongming/v1/kongming.proto
syntax = "proto3";
package kongming.v1;

service Kongming {
  rpc Dispatch(DispatchRequest) returns (DispatchResponse);
  rpc GetOrder(GetOrderRequest) returns (Order);
  rpc ListOrders(ListOrdersRequest) returns (ListOrdersResponse);
  rpc ListGenerals(ListGeneralsRequest) returns (ListGeneralsResponse);
  rpc ListJinnang(ListJinnangRequest) returns (ListJinnangResponse);
  rpc ExecuteJinnang(ExecuteJinnangRequest) returns (ExecuteJinnangResponse);
  rpc RunWorkflow(RunWorkflowRequest) returns (RunWorkflowResponse);
}
```

### 5.5 中间件/拦截器统一

- 提取 `X-Trace-Id` header → 注入 ctx → span 标签 + 日志字段
- 错误统一格式 `{code, message, trace_id, request_id}`
- domain 错误 → HTTP 状态码 + gRPC code 映射（见 §7.2）

---

## 6. 启动流程与配置驱动

### 6.1 顶层入口

```go
// pkg/kongming/kongming.go
type Kongming struct {
    cfg         *config.Config
    logger      *zap.Logger
    observer    domain.Observer
    resilient   domain.ResilientRunner
    commander   domain.Commander
    dispatcher  domain.Dispatcher
    engine      domain.Engine
    pool        domain.GeneralPool
    vault       domain.Vault
    courier     domain.Courier
    pluginReg   *plugin.Registry
    httpSrv     *http.Server
    grpcSrv     *grpc.Server
}

func New(cfgPath string, opts ...Option) (*Kongming, error) {
    cfg, err := config.Load(cfgPath)              // 1. 配置
    if err != nil { return nil, fmt.Errorf("config: %w", err) }

    logger, err := observability.NewLogger(cfg.Observatory)    // 2. logger
    if err != nil { return nil, err }

    ctx := context.Background()
    observer, err := observability.NewObserver(ctx, cfg.Observatory)  // 3. observer
    if err != nil { return nil, err }

    resilient := resilience.NewRunner(cfg.Resilience, logger)  // 4. 弹性
    repos := persistence.NewMemoryRepos()                      // 5. 仓库
    pluginReg, err := plugin.NewRegistry(cfg.Plugin, logger, observer)  // 6. 插件
    if err != nil { return nil, err }

    pool := general.NewPool(cfg.Generals, repos, logger)
    vaultSvc := vaultsvc.New(cfg.Vault, pluginReg, logger, observer)
    courierSvc := couriersvc.New(cfg.Courier, logger, observer)
    engine := workflow.NewRunner(cfg.Bagua, pluginReg, resilient, logger, observer)
    dispatcher := dispatcher.New(cfg.Dispatcher, pool, vaultSvc, resilient, logger, observer)
    cmd := commander.New(cfg.Commander, dispatcher, engine, pool, vaultSvc, repos, resilient, logger, observer)

    return &Kongming{
        cfg: cfg, logger: logger, observer: observer, resilient: resilient,
        commander: cmd, dispatcher: dispatcher, engine: engine,
        pool: pool, vault: vaultSvc, courier: courierSvc,
        pluginReg: pluginReg,
    }, nil
}

func (k *Kongming) Run(ctx context.Context) error {
    // 1. 启动热更新
    if err := k.pluginReg.Watch(ctx); err != nil { return err }

    // 2. 启动 HTTP
    httpSrv := httpsrv.New(k.cfg.Server, k.commander, k.dispatcher, k.engine, k.pool, k.vault, k.observer, k.logger)
    k.httpSrv = httpSrv
    go httpSrv.ListenAndServe()

    // 3. 启动 gRPC
    lis, err := net.Listen("tcp", k.cfg.Server.GRPCAddr)
    if err != nil { return err }
    grpcSrv := grpcsrv.New(k.cfg.Server, k.commander, k.dispatcher, k.engine, k.pool, k.vault, k.observer, k.logger)
    k.grpcSrv = grpcSrv
    go grpcSrv.Serve(lis)

    // 4. 等待信号
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

### 6.2 cmd 入口

```go
// cmd/kongming-server/main.go
func main() {
    cfgPath := flag.String("config", "configs/kongming.yaml", "config path")
    k, err := kongming.New(*cfgPath, kongming.WithServiceName("kongming-server"))
    if err != nil { log.Fatal(err) }
    ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
    defer cancel()
    if err := k.Run(ctx); err != nil { log.Fatal(err) }
}

// cmd/kongming/main.go
func main() {
    root := cli.NewRootCmd()
    if err := root.Execute(); err != nil { os.Exit(1) }
}
```

### 6.3 配置优先级

1. 命令行 flag
2. 环境变量 `KONGMING_*`
3. 配置文件
4. 默认值

### 6.4 配置热更新

viper 监听 SIGHUP 重新加载；reload 时对不变更的依赖复用，变更的依赖按需重启。

---

## 7. 错误处理与可观测性

### 7.1 错误体系

```go
// pkg/domain/errors/errors.go
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
)

type Error struct {
    Code    Code
    Message string
    Cause   error
    TraceID string
    Fields  map[string]any
}

func (e *Error) Error() string  { return string(e.Code) + ": " + e.Message }
func (e *Error) Unwrap() error   { return e.Cause }
func (e *Error) Is(t error) bool {
    target, ok := t.(*Error)
    if !ok { return false }
    return e.Code == target.Code
}
```

### 7.2 错误码 → 传输层映射

| domain.Code | HTTP | gRPC |
|---|---|---|
| CodeInvalidArgument | 400 | InvalidArgument |
| CodeNotFound | 404 | NotFound |
| CodeConflict | 409 | AlreadyExists |
| CodeTimeout | 504 | DeadlineExceeded |
| CodeUnavailable | 503 | Unavailable |
| CodeInternal | 500 | Internal |
| CodeCircuitOpen | 503 | Unavailable |
| CodePluginLoadFail | 500 | Internal |
| CodeInvalidState | 409 | FailedPrecondition |

### 7.3 统一响应格式

```json
{
  "code": "ORDER_NOT_FOUND",
  "message": "军令不存在: ord_xxx",
  "trace_id": "01HXXXXXX",
  "request_id": "req_xxx",
  "details": { "order_id": "ord_xxx" }
}
```

### 7.4 可观测性三件套

#### 7.4.1 结构化日志

- 字段：`ts, level, logger, msg, trace_id, span_id, ...`
- 注入到所有 service；HTTP 中间件 + gRPC 拦截器自动注入 trace_id
- 日志级别：debug/info/warn/error 五级，可配置

#### 7.4.2 Prometheus 指标

```
# Counter
kongming_orders_total{status="success|failed",priority="low|normal|high|urgent"}
kongming_tactics_total{mode,status}
kongming_general_executions_total{general_id,status}
kongming_jinnang_executions_total{jinnang_id,status}
kongming_plugin_reloads_total{result}

# Histogram
kongming_dispatch_duration_seconds{priority}
kongming_workflow_duration_seconds{mode}
kongming_jinnang_duration_seconds{jinnang_id}

# Gauge
kongming_active_orders
kongming_general_utilization{general_id,general_name}
kongming_circuitbreaker_state{name}
```

#### 7.4.3 OpenTelemetry 追踪

- Tracer：`kongming-server` / `kongming-cli`
- Span 命名：`<package>.<method>`（如 `commander.Dispatch`、`workflow.executeTiangai`）
- Span 属性：`order.id`, `general.id`, `bagua.mode`, `jinnang.id`
- Exporter：Jaeger（已有）/ OTLP（新增，可配置）
- traceId 透传：HTTP `X-Trace-Id` → gRPC metadata → ctx

### 7.5 全链路 trace 接入点

- gin 中间件：每个 HTTP 请求开 span
- gRPC 拦截器：每个 RPC 开 span
- Commander.Dispatch、WorkflowRunner.Run、GeneralPool.Execute、Jinnang.Execute：各自开子 span
- 重试/熔断/限流：包装成独立 span，错误记录到 `error` 属性

---

## 8. 测试策略

### 8.1 测试金字塔

```
        ┌────────────────┐
        │   E2E 测试     │  5%  完整服务 + HTTP/gRPC 客户端 + 真后端
        ├────────────────┤
        │  集成测试      │ 25%  多组件协作 + in-memory + 假 plugin
        ├────────────────┤
        │  单元测试      │ 70%  纯函数 + 接口 mock + 边界条件
        └────────────────┘
```

### 8.2 每层规范

| 层级 | 工具 | Mock 方式 | 覆盖目标 |
|---|---|---|---|
| 单元 | `testing` + `testify/assert` + `testify/mock` | 手写 mock 或 gomock | domain 100% / application ≥ 90% / infra ≥ 80% / transport ≥ 70% |
| 集成 | `testify/suite` + 真 in-memory 仓库 | 无 mock（用真组件） | 关键流程 100% |
| E2E | `dockertest` + 真实 prom/jaeger | 无 mock | 至少 1 个 happy path + 1 个 failure path |
| 模糊 | `go test -fuzz` | n/a | parser/validator 路径 |

### 8.3 关键测试场景

#### Commander
- Happy path：派遣 5 个将领全部成功
- 战略规划：根据 priority 选择不同 BaguaMode
- 状态机：非法状态转换返回错误
- 幂等：相同 order_id 第二次调用返回缓存结果
- 失败：将领执行失败 → 战报标记失败 → 触发重试/熔断

#### Engine（八卦阵）
- Tiangai（并行）：DAG 多层并行执行
- Dizai（顺序）：线性执行，遇错终止
- Fengyang（带超时）：超时后 ctx 取消传播
- Yunzhui（重试）：失败后重试 ≤ MaxAttempts
- Longfei（动态）：critical path + 后并行
- Huyi（条件分支）：condition node 路由正确
- Niaoxiang（扇形）：单源多目标
- Shepan（循环）：max_iterations 限制

#### GeneralPool
- SelectBest：基于 skills + 评分选择
- 并发：100 个请求下无竞态（race detector 必跑）
- 失败统计：成功率/响应时间更新正确

#### Vault
- Register/Unregister：并发安全
- LoadFromDir：从 yaml 目录加载
- 热更新：fsnotify 触发 reload
- 插件加载：.so 失败不阻塞主流程

#### Resilience
- Retry：指数退避 + jitter
- CircuitBreaker：失败阈值 → Open → 超时 → HalfOpen → 成功 → Closed
- 修复验证：HalfOpen 状态并发试探只放行 1 个

#### Observability
- 指标：counter/histogram/gauge 累加正确
- traceId：从 header 提取 → ctx 注入 → span 标签
- 错误：RecordError 后 span 状态为 Error

#### Transport
- HTTP：handler 单测（httptest）+ 中间件链
- gRPC：service 单测（bufconn/grpc-test）
- CLI：cobra 命令 execute 单测

### 8.4 覆盖率门槛（CI 阻断）

- 整体 ≥ 80%
- 新增/修改文件 ≥ 80%
- 关键包 `domain/`、`application/commander/`、`application/workflow/` 100%

### 8.5 CI 流水线

```yaml
jobs:
  - lint:        golangci-lint run --timeout=5m
  - test:        go test -race -coverprofile=coverage.out ./...
  - coverage:    go test -coverpkg=./... -coverprofile=coverage.out ./...
                 && coverage=$(go tool cover -func=coverage.out | tail -1 | awk '{print $3}' | tr -d '%')
                 && [ "$coverage" -ge 80 ]
  - fuzz:        go test -fuzz=. -fuzztime=30s ./pkg/domain/...
  - build:       go build ./cmd/...
  - docker:      docker buildx build --load .
```

### 8.6 TDD 流程

1. 先写失败测试（红灯）
2. 实现最小代码让它变绿
3. 重构（保持绿灯）
4. 提交（commit 粒度小，message 描述意图）

---

## 9. 迁移路径（实施步骤概要）

> 详细步骤将由 writing-plans 阶段产出。

按依赖方向从底向上：

1. **阶段 1 — 基础设施**：`pkg/infra/{config,observability,resilience,persistence,plugin}` + 单元测试
2. **阶段 2 — 领域层**：`pkg/domain/{model,port,errors}` + 100% 单元测试
3. **阶段 3 — 应用层**：`pkg/application/{commander,dispatcher,workflow,general,vault,courier}` + 单元 + 集成测试
4. **阶段 4 — 传输层**：`pkg/transport/{http,grpc,cli}` + 单元 + E2E 测试
5. **阶段 5 — 顶层装配**：`pkg/kongming` + `cmd/{kongming-server,kongming}` + 端到端冒烟
6. **阶段 6 — 清理**：`rm -rf pkg/{bagua,cmd_center,courier,dispatch,generals,observatory,repeater,strategy_vault} internal/memory` + 删除原 main.go → 替换为 kongming-server main + kongming cli main
7. **阶段 7 — 文档 & CI**：更新 README、CI、CHANGELOG；deployments/ 重新生成

---

## 10. 风险与缓解

| 风险 | 缓解措施 |
|---|---|
| 现有 examples 不可用 | 每个阶段保留 1 个 examples 跑通；阶段性切流 |
| go plugin 跨平台限制（Linux only） | plugin 加载提供接口抽象；非 Linux 用进程内注册 + 文件加载 |
| 重试/熔断装饰顺序错误 | 单元测试覆盖所有组合；明确文档化 |
| 覆盖率门槛初期达不到 | 先按"新文件 ≥ 80%"灰度，旧文件逐步补齐 |
| 重大 breaking change 影响使用方 | 在 README 标注 v2.0.0；提供 migration guide（由 writing-plans 阶段产出） |

---

## 11. 开放问题（待用户评审时确认）

- [ ] 是否要支持多实例分布式 Commander？（当前设计是单进程）
- [ ] examples/ 是否需要补齐 4 个场景（quickstart / longzhong_strategy / wuhu_campaign / zhuge_bagua）？
- [ ] 是否需要支持 gRPC streaming？（当前设计是 unary）
- [ ] 是否要支持 OpenTelemetry OTLP exporter（替代 Jaeger）？
- [ ] 配置文件是否要支持 JSON/TOML（当前只 yaml）？
- [ ] 是否要在 v1.0 引入 RBAC / Auth / 限流（按 IP/token）？
