// Package main 是「八卦阵」工作流引擎示例：演示 RegisterWorkflow + Execute 的用法。
//
// 八卦阵源于诸葛亮的「八阵图」，在 Kongming 中被抽象为 8 种工作流编排模式。
// 本示例构造一份 3 节点的简单工作流（start → action → end），使用 Dizai（地载）
// 阵型顺序执行，再注册一个 echo 类型的 Action 节点执行器（把 ec.Variables
// 透传到 Output），最后打印 ExecutionContext.NodeStates 的最终状态。
//
// 与 quickstart 的区别：
//   - quickstart 演示 Commander 派单；
//   - 本示例演示 Engine 工作流：注册、执行、读 NodeStates。
//
// 用法：
//
//	go run ./examples/zhuge_bagua
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"go.uber.org/zap"

	"github.com/zhuge/kongming/pkg/application/workflow"
	"github.com/zhuge/kongming/pkg/domain/model"
	"github.com/zhuge/kongming/pkg/domain/port"
)

// echoExecutor 把 ec.Variables["k"] 写入 NodeState.Output。
//
// 真实业务可替换为：调用 LLM / 调外部 API / 触发子 Workflow 等。
type echoExecutor struct{}

// Execute 简单实现 port.NodeExecutor。
func (echoExecutor) Execute(_ context.Context, n model.Node, ec *model.ExecutionContext) (*model.NodeState, error) {
	v, _ := ec.GetVar("k")
	started := time.Now()
	return &model.NodeState{
		ID:          n.ID,
		Status:      model.NodeStatusOK,
		Output:      v,
		StartedAt:   started,
		CompletedAt: time.Now(),
	}, nil
}

// 编译期断言：echoExecutor 满足 port.NodeExecutor。
var _ port.NodeExecutor = echoExecutor{}

func main() {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// 1. 构造 Runner（自动注册内置 start/end 执行器）。
	eng := workflow.NewRunner(zap.NewNop())

	// 2. 注册自定义 Action 执行器：把 ec.Variables["k"] 透传。
	eng.RegisterNodeExecutor(model.NodeAction, echoExecutor{})

	// 3. 构造工作流：3 节点，start → action → end，Dizai 顺序阵型。
	wf := &model.Workflow{
		ID:   "demo-bagua",
		Name: "八卦阵演示 · 顺序三节点",
		Mode: model.Dizai,
		Nodes: []model.Node{
			{ID: "start", Name: "入口", Type: model.NodeStart},
			{ID: "echo", Name: "回声", Type: model.NodeAction, Action: "echo"},
			{ID: "end", Name: "出口", Type: model.NodeEnd},
		},
		Edges: []model.Edge{
			{From: "start", To: "echo"},
			{From: "echo", To: "end"},
		},
		Entry: "start",
	}

	if err := eng.RegisterWorkflow(wf); err != nil {
		fmt.Fprintln(os.Stderr, "register workflow failed:", err)
		os.Exit(1)
	}

	// 4. 执行：传入初始变量 k="隆中对" → echo 节点把 k 写入 Output。
	ec, err := eng.Execute(ctx, "demo-bagua", map[string]any{"k": "隆中对"})
	if err != nil {
		fmt.Fprintln(os.Stderr, "execute workflow failed:", err)
		os.Exit(1)
	}

	// 5. 打印 ExecutionContext 总览。
	fmt.Println("=== 八卦阵执行结果 ===")
	fmt.Printf("  WorkflowID:  %s\n", ec.WorkflowID)
	fmt.Printf("  RunID:       %s\n", ec.RunID)
	fmt.Printf("  耗时:        %.3fs\n", ec.CompletedAt.Sub(ec.StartedAt).Seconds())
	fmt.Printf("  节点状态:    %d 条\n", len(ec.NodeStates))
	fmt.Println()

	// 6. 按 Workflow 节点顺序打印每个 NodeState（顺序：start → echo → end）。
	for _, n := range wf.Nodes {
		ns, ok := ec.NodeStates[n.ID]
		if !ok {
			fmt.Printf("  节点 %-6s  (未执行)\n", n.ID)
			continue
		}
		dur := ns.CompletedAt.Sub(ns.StartedAt).Seconds()
		fmt.Printf("  节点 %-6s  status=%-7s duration=%.3fs output=%v\n",
			ns.ID, ns.Status, dur, ns.Output)
	}

	// 7. JSON 化 ExecutionContext 便于下游消费。
	fmt.Println()
	fmt.Println("=== JSON 化 ExecutionContext ===")
	out, _ := json.MarshalIndent(ec, "", "  ")
	fmt.Println(string(out))
}
