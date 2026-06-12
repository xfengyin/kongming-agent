// Package main 是「五虎将战役」示例：演示如何向 Pool 注册多个 General 并派单。
//
// 完整复刻「五虎将」语义（关羽/张飞/赵云/马超/黄忠），给每个将领注入
// 「execute」skill，再构造一个含 5 条 Tactic 的 Strategy（不显式构造 Order，
// 而是通过 Planner.Plan 让 DefaultPlanner 一次性把 5 条 objective 转成 tactic），
// 调用 Commander.Dispatch 一次性派单，最后打印每条 GeneralReport 的状态。
//
// 用法：
//
//	go run ./examples/wuhu_campaign
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"

	"github.com/zhuge/kongming/pkg/application/commander"
	"github.com/zhuge/kongming/pkg/application/general"
	"github.com/zhuge/kongming/pkg/application/vault"
	"github.com/zhuge/kongming/pkg/application/workflow"
	"github.com/zhuge/kongming/pkg/domain/model"
	"github.com/zhuge/kongming/pkg/infra/config"
	"github.com/zhuge/kongming/pkg/infra/observability"
	"github.com/zhuge/kongming/pkg/infra/persistence/memory"
	"github.com/zhuge/kongming/pkg/infra/resilience"
)

// wuhuExecutors 是五虎将的内置 Executor：把 Order.ID 写进 Output 并标记成功。
// 真实业务可替换为「调用 LLM / 调外部 API / 调搜索引擎」等具体实现。
type wuhuExecutors struct{}

// Execute 简单把 Order ID 写进 report.Output（demo 用：可观察派单是否真的命中）。
func (wuhuExecutors) Execute(_ context.Context, g *model.General, o *model.Order) (*model.GeneralReport, error) {
	return &model.GeneralReport{
		GeneralID: g.ID,
		Name:      g.Name,
		Success:   true,
		Output: map[string]any{
			"echo_order_id": string(o.ID),
			"echo_general":  g.Name,
		},
	}, nil
}

// compile-time 断言 wuhuExecutors 满足 general.Executor。
var _ general.Executor = wuhuExecutors{}

// 五虎将元数据：复刻「关张赵马黄」五位将领。
var wuhu = []struct {
	ID    model.GeneralID
	Name  string
	Title string
	Type  model.GeneralType
}{
	{model.GeneralID("guanyu"), "关羽", "前将军 · 武圣", model.GeneralGuanYu},
	{model.GeneralID("zhangfei"), "张飞", "右将军 · 万人敌", model.GeneralZhangFei},
	{model.GeneralID("zhaoyun"), "赵云", "翊军将军 · 一身是胆", model.GeneralZhaoYun},
	{model.GeneralID("machao"), "马超", "骠骑将军 · 一骑当千", model.GeneralMaChao},
	{model.GeneralID("huangzhong"), "黄忠", "后将军 · 老当益壮", model.GeneralHuangZhong},
}

func main() {
	logger := zap.NewNop()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// 1. observer 必须非 nil（commander.Service.Dispatch 内部会调 StartSpan）。
	//    本 demo 关闭 OTLP tracing，纯内存运行避免外部依赖。
	obsCfg := config.ObservatoryConfig{
		MetricsPort: 0,
		Tracing:     config.TracingConfig{Enabled: false},
		Log:         config.LogConfig{Level: "info", Encoding: "json"},
	}
	observer, err := observability.NewObserver(ctx, obsCfg, logger)
	if err != nil {
		fmt.Fprintln(os.Stderr, "new observer:", err)
		os.Exit(1)
	}

	// 2. 共享 Store（OrderRepo 与 GeneralRepo 同源；本 demo 不接 GeneralRepo）。
	store := memory.NewStore()
	orderRepo := memory.NewOrderRepo(store)

	// 3. 弹性执行器（demo 用最小配置）
	res := resilience.NewRunner(resilience.Config{
		Retry:          resilience.RetryConfig{MaxAttempts: 1},
		CircuitBreaker: resilience.CircuitBreakerConfig{Threshold: 100, Timeout: 10 * time.Second},
		RateLimit:      resilience.RateLimitConfig{RPS: 1000, Burst: 1000},
	}, logger)

	// 4. 将领池：注入 wuhuExecutors（五虎将共用的业务执行器）。
	pool := general.NewPool(logger, observer, res, wuhuExecutors{})

	// 5. 注册五虎将（每个将领打上 "execute" 技能，对应 DefaultPlanner 生成的 Tactic.Action）。
	for _, g := range wuhu {
		gm := &model.General{
			ID:          g.ID,
			Name:        g.Name,
			Title:       g.Title,
			Type:        g.Type,
			Description: "五虎上将之一",
			Skills:      []string{"execute"},
			State:       int(model.GeneralIdle),
			CreatedAt:   time.Now(),
		}
		if err := pool.Register(ctx, gm); err != nil {
			fmt.Fprintf(os.Stderr, "register %s failed: %v\n", g.Name, err)
			os.Exit(1)
		}
	}

	// 6. 构造 Order：5 条 objective 会被 Planner 映射为 5 条 Tactic（每个将领被点 1 次）。
	order := &model.Order{
		ID:       model.OrderID(uuid.NewString()),
		Name:     "五虎将战役",
		State:    model.StatePending,
		Priority: model.PriorityNormal,
		Strategy: model.Strategy{
			Objectives: []string{
				"关羽：华容道阻曹",
				"张飞：当阳桥喝退",
				"赵云：长坂坡救主",
				"马超：潼关战曹操",
				"黄忠：定军山斩夏侯",
			},
		},
		CreatedAt: time.Now(),
	}

	// 7. 装配 Commander（无 Engine / Vault 影响，但 commander.New 需要全依赖非 nil）。
	eng := workflow.NewRunner(logger)
	vlt := vault.NewVault(logger, nil)
	cmd := commander.New(commander.NewDefaultPlanner(), pool, eng, vlt, orderRepo, res, observer, logger)

	// 8. 派单。
	report, err := cmd.Dispatch(ctx, order)
	if err != nil {
		fmt.Fprintln(os.Stderr, "dispatch failed:", err)
		os.Exit(1)
	}

	// 9. 打印每条 GeneralReport：成功/失败/用时。
	fmt.Println("=== 五虎将战役战报 ===")
	fmt.Printf("  OrderID:   %s\n", report.OrderID)
	fmt.Printf("  整体成功:   %v\n", report.Success)
	fmt.Printf("  出战将领:   %d\n", len(report.Generals))
	fmt.Printf("  派单耗时:   %.3fs\n", report.Duration)
	fmt.Println()
	for i, gr := range report.Generals {
		status := "✓"
		if !gr.Success {
			status = "✗"
		}
		fmt.Printf("  %s [%d] %s（%s）耗时=%.3fs\n",
			status, i+1, gr.Name, gr.GeneralID, gr.Duration)
		if gr.Error != "" {
			fmt.Printf("      错误: %s\n", gr.Error)
		}
	}

	// 10. JSON 化全量战报。
	fmt.Println()
	fmt.Println("=== JSON 化 BattleReport ===")
	out, _ := json.MarshalIndent(report, "", "  ")
	fmt.Println(string(out))
}
