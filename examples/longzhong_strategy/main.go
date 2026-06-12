// Package main 是「隆中对」战略示例：演示 PlanStrategy 的用法。
//
// 隆中对是诸葛亮的著名战略：「先取荆州为家，再取益州成鼎足之势，外结好孙权，
// 内修政理」。本示例把 3 条 objective 装入 Order，再调用 PlanStrategy 单独生成
// Strategy（不真正执行 Dispatch），打印包含的 Tactics 步骤与八卦阵选择。
//
// 与 quickstart 的区别：
//   - quickstart 演示完整「派单 + 执行 + 战报」流程；
//   - 本示例演示「派单前预览战略」流程（PlanStrategy 单独使用）。
//
// 用法：
//
//	go run ./examples/longzhong_strategy
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/google/uuid"

	"github.com/zhuge/kongming/pkg/application/commander"
	"github.com/zhuge/kongming/pkg/domain/model"
)

// 隆中对的 3 条战略目标（自然语言）。
var longzhongObjectives = []string{
	"先取荆州为家",
	"再取益州成鼎足之势",
	"外结孙权、内修政理",
}

func main() {
	// PlanStrategy 是只读操作（不触发实际执行），无需装配完整的 Pool / Vault /
	// Workflow，DefaultPlanner 即可独立工作。
	planner := commander.NewDefaultPlanner()

	// 隆中对 Order：Priority=High → Planner 会选 Tiangai（天覆：并行 DAG）阵型。
	order := &model.Order{
		ID:       model.OrderID(uuid.NewString()),
		Name:     "隆中对 · 天下三分",
		State:    model.StatePending,
		Priority: model.PriorityHigh,
		Strategy: model.Strategy{
			Objectives: longzhongObjectives,
		},
		CreatedAt: time.Now(),
	}

	// PlanStrategy 单独使用：只生成战略、不派单、不执行。
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	strategy, err := planner.Plan(ctx, order)
	if err != nil {
		fmt.Fprintln(os.Stderr, "plan failed:", err)
		os.Exit(1)
	}

	// 打印战略概览。
	fmt.Println("=== 隆中对战略 ===")
	fmt.Printf("  战略类型: %s\n", strategy.Type)
	fmt.Printf("  八卦阵型: %s（高优先级→天覆并行 DAG）\n", strategy.BaguaMode)
	fmt.Printf("  战略目标: %d 条\n", len(strategy.Objectives))
	for i, obj := range strategy.Objectives {
		fmt.Printf("    %d. %s\n", i+1, obj)
	}

	// 打印战术清单。
	fmt.Println()
	fmt.Println("=== 战术清单 ===")
	for _, t := range strategy.Tactics {
		fmt.Printf("  Tactic #%d  %s\n", t.Order, t.Name)
		fmt.Printf("    描述: %s\n", t.Description)
		fmt.Printf("    动作: %s（Action='%s' 由 Commander 选将）\n", t.Action, t.Action)
	}

	// JSON 化输出便于下游消费。
	fmt.Println()
	fmt.Println("=== JSON 化 Strategy ===")
	out, err := json.MarshalIndent(strategy, "", "  ")
	if err != nil {
		fmt.Fprintln(os.Stderr, "marshal:", err)
		os.Exit(1)
	}
	fmt.Println(string(out))
}
