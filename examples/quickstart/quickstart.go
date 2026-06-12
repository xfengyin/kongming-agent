// Package quickstart 是 Kongming SDK 最小可运行示例。
//
// 本文件实现 Run(ctx) 函数：装配一个内存版军师系统、派发一个最小 Order、返回
// 战报。把核心逻辑抽成独立函数（不是直接在 main 里写）有两个好处：
//  1. 单元测试（quickstart_test.go）可以传入独立 ctx 验证无 panic；
//  2. 任意宿主（CLI / 集成测试 / CI smoke）都能复用同一段装配+派单逻辑。
//
// 运行方式：
//
//	go run ./examples/quickstart
//	KONGMING_CONFIG=configs/kongming.yaml go run ./examples/quickstart
//
// 设计要点：
//   - 配置路径通过 os.Getenv("KONGMING_CONFIG") 解析，默认 "configs/kongming.yaml"，
//     兼容不同 cwd（plan 阶段明确要求）；
//   - 不引入任何新依赖（仅使用已存在的 application/infra 包）；
//   - 所有错误直接透传，调用方决定是否 fatal。
package quickstart

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/google/uuid"

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
)

// ConfigPath 返回当前进程应当使用的配置文件路径。
//
// 解析顺序：环境变量 KONGMING_CONFIG > 内置默认 "configs/kongming.yaml"。
// 该函数与 Run 解耦，方便测试单独 mock。
func ConfigPath() string {
	if v := os.Getenv("KONGMING_CONFIG"); v != "" {
		return v
	}
	return "configs/kongming.yaml"
}

// Run 装配一个最小 Kongming 系统并派发一条 Order，返回 BattleReport。
//
// 装配顺序严格遵循 plan 阶段模板（config → logger → observer → resilience →
// store → plugin → pool → vault → workflow → commander），与 pkg/kongming 顶层
// 装配入口等价，但不启动 HTTP/gRPC server，纯内存版适合 demo。
//
// 失败语义：任何装配步骤返回 error 时直接外抛；调用方决定是否 fatal。
func Run(ctx context.Context) (*model.BattleReport, error) {
	// 1. 配置
	cfg, err := config.Load(ConfigPath())
	if err != nil {
		return nil, fmt.Errorf("load config: %w", err)
	}

	// 2. logger
	logger, err := observability.NewLogger(cfg.Observatory.Log)
	if err != nil {
		return nil, fmt.Errorf("new logger: %w", err)
	}
	defer func() { _ = logger.Sync() }()

	// 3. observer（OTLP tracing 默认关闭，避免 demo 时强行连外部 collector）
	obs, err := observability.NewObserver(ctx, cfg.Observatory, logger)
	if err != nil {
		return nil, fmt.Errorf("new observer: %w", err)
	}

	// 4. 弹性执行器
	res := resilience.NewRunner(toResilienceConfig(cfg.Resilience), logger)

	// 5. 进程内仓库（OrderRepo 与 GeneralRepo 共享同一 Store）
	store := memory.NewStore()
	orderRepo := memory.NewOrderRepo(store)

	// 6. 插件注册中心（demo 中不使用，仅占位满足 commander 装配接口）
	_ = plugin.NewRegistry()

	// 7. 将领池：executor 传 nil（demo 不注入自定义五虎将 handler，SelectBest
	//    会返回 "no general for skill"，commander 内部 continue 跳过，不视为失败）。
	pool := general.NewPool(logger, obs, res, nil)

	// 8. 锦囊库：最小可用版
	vlt := vault.NewVault(logger, obs)

	// 9. 工作流引擎（demo 不直接调用，仅满足 commander 装配）
	eng := workflow.NewRunner(logger)

	// 10. 军师
	cmd := commander.New(commander.NewDefaultPlanner(), pool, eng, vlt, orderRepo, res, obs, logger)

	// 11. 构造 Order（ID 用 UUID 避免重复；Priority=Normal → Planner 选 Dizai 阵型）
	order := &model.Order{
		ID:       model.OrderID(uuid.NewString()),
		Name:     "出师表任务",
		State:    model.StatePending,
		Priority: model.PriorityNormal,
		Strategy: model.Strategy{
			Objectives: []string{"收服东吴", "北定中原"},
		},
		CreatedAt: time.Now(),
	}

	// 12. 派单并等待结果
	report, err := cmd.Dispatch(ctx, order)
	if err != nil {
		return nil, fmt.Errorf("dispatch: %w", err)
	}
	return report, nil
}

// toResilienceConfig 把 config.ResilienceConfig 映射到 resilience.Config，
// 与 pkg/kongming.NewWithOptions 中的 toResilienceConfig 等价。
// 在此重复定义以避免 examples → pkg/kongming 的反向依赖。
func toResilienceConfig(in config.ResilienceConfig) resilience.Config {
	return resilience.Config{
		Retry: resilience.RetryConfig{
			MaxAttempts:    in.Retry.MaxAttempts,
			InitialBackoff: in.Retry.InitialBackoff,
			MaxBackoff:     in.Retry.MaxBackoff,
			BackoffFactor:  in.Retry.BackoffFactor,
			Jitter:         in.Retry.Jitter,
		},
		CircuitBreaker: resilience.CircuitBreakerConfig{
			Threshold: in.CircuitBreaker.Threshold,
			Timeout:   in.CircuitBreaker.Timeout,
		},
		RateLimit: resilience.RateLimitConfig{
			RPS:   in.RateLimit.RPS,
			Burst: in.RateLimit.Burst,
		},
	}
}
