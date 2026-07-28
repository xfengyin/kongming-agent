// Kongming 快速开始
// 运筹帷幄之中，决胜千里之外
// 演示 kimi-k3 风格的 MoE 专家路由 + 混合执行引擎

package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/zhuge/kongming/pkg/bagua"
	"github.com/zhuge/kongming/pkg/cmd_center"
	"github.com/zhuge/kongming/pkg/generals"
	"github.com/zhuge/kongming/pkg/strategy_vault"
	"go.uber.org/zap"
)

func main() {
	logger, _ := zap.NewDevelopment()
	defer logger.Sync()
	_ = logger

	ctx := context.Background()

	fmt.Println("=== 诸葛孔明系统 - 快速开始（kimi-k3 风格 MoE + 混合执行引擎）===")
	fmt.Println()

	// 1. 初始化 MoE 专家池（对应 kimi-k3 Stable LatentMoE）
	fmt.Println("⚔️  五虎将专家池待命中...")
	pool := generals.NewMoEExpertPool()
	router := generals.NewMoERouter(pool, nil)
	router.RegisterBalancer(generals.NewQuantileBalancer(0.3))
	expertExecutor := generals.NewMoEExpertExecutor(pool, router, 1)
	for _, e := range pool.List(generals.ExpertFilter{}) {
		fmt.Printf("  • %s（%s）- %s [容量=%d 技能=%v]\n",
			e.Name, e.Title, e.Description, e.Capacity, e.Skills)
	}

	// 2. 初始化军师府（依赖注入 ExpertExecutor 端口）
	fmt.Println()
	fmt.Println("📜 初始化军师府（依赖注入 ExpertExecutor 端口）...")
	commander := cmd_center.NewCommander(logger, expertExecutor)
	fmt.Println("✓ 军师府已开张")

	// 3. 初始化八卦阵混合执行引擎（KDA/MLA 3:1 混合）
	fmt.Println()
	fmt.Println("🌑 八卦阵混合执行引擎（KDA:MLA = 3:1）...")
	engine := bagua.NewEngine()
	// 注册演示执行器，让工作流节点真正执行
	engine.RegisterNodeExecutor(bagua.NodeTool, &demoNodeExecutor{})
	engine.RegisterNodeExecutor(bagua.NodeStart, &demoNodeExecutor{})
	engine.RegisterNodeExecutor(bagua.NodeEnd, &demoNodeExecutor{})
	registerDemoWorkflow(engine)
	fmt.Println("✓ 工作流已注册，节点模式已按 3:1 自动分配")

	// 4. 颁布军令
	fmt.Println()
	fmt.Println("📋 颁布军令：市场调研任务")

	order := cmd_center.NewMilitaryOrder(
		"市场调研",
		"调研智能硬件市场现状",
		cmd_center.PriorityNormal,
	)
	order.Strategy.Objectives = []string{
		"收集竞品信息",
		"处理数据并清洗",
		"分析用户需求",
		"撰写调研报告",
	}

	// 5. 派遣执行（MoE 路由 + 专家执行）
	fmt.Println("⚔️  调兵遣将中（MoE 路由激活专家）...")
	report, err := commander.Dispatch(ctx, order)
	if err != nil {
		log.Fatalf("任务执行失败: %v", err)
	}

	// 6. 输出战报
	fmt.Println()
	fmt.Println("=== 战报 ===")
	fmt.Printf("任务: %s\n", order.Name)
	fmt.Printf("状态: %v\n", report.Success)
	fmt.Printf("执行时间: %v\n", report.CompletedAt.Sub(report.StartedAt))
	fmt.Println()
	fmt.Println("将领战功:")
	for _, gr := range report.Generals {
		status := "✓"
		if !gr.Success {
			status = "✗"
		}
		fmt.Printf("  %s %s: %s\n", status, gr.GeneralName, gr.Message)
	}

	// 7. 演示八卦阵引擎执行（KDA/MLA 混合调度）
	fmt.Println()
	fmt.Println("=== 八卦阵混合执行演示 ===")
	ec, err := engine.Execute(ctx, "wf-demo", map[string]interface{}{"topic": "智能硬件"})
	if err != nil {
		log.Printf("八卦阵执行失败: %v", err)
	} else {
		fmt.Printf("✓ 工作流执行完成，节点状态数: %d\n", len(ec.NodeStates))
		for id, st := range ec.NodeStates {
			fmt.Printf("  • %s [%s] status=%s\n", id, st.Mode, st.Status)
		}
	}

	// 8. 锦囊演示
	fmt.Println()
	fmt.Println("🎁 锦囊演示...")
	vault := strategy_vault.NewVault()
	vault.RegisterSkill("data_analysis", &DataAnalysisSkill{})

	result, err := vault.Execute(ctx, "data_analysis", strategy_vault.JinnangInput{
		Params: map[string]interface{}{
			"data": []int{1, 2, 3, 4, 5},
		},
	})
	if err != nil {
		log.Printf("锦囊执行失败: %v", err)
	} else {
		fmt.Printf("✓ 锦囊执行成功: %v\n", result.Data)
	}

	fmt.Println()
	fmt.Println("=== 演示完成 ===")
	time.Sleep(100 * time.Millisecond)
}

// DataAnalysisSkill 数据分析技能
type DataAnalysisSkill struct{}

func (s *DataAnalysisSkill) Name() string { return "data_analysis" }

func (s *DataAnalysisSkill) Execute(ctx context.Context, input map[string]interface{}) (map[string]interface{}, error) {
	data, ok := input["data"].([]int)
	if !ok {
		return nil, fmt.Errorf("需要整数数组")
	}
	sum := 0
	for _, v := range data {
		sum += v
	}
	return map[string]interface{}{
		"count": len(data),
		"sum":   sum,
		"avg":   float64(sum) / float64(len(data)),
	}, nil
}

// registerDemoWorkflow 注册演示工作流（验证 KDA/MLA 3:1 自动分配）
// 工作流结构：start -> collect -> process -> analyze -> report -> end
// 4 个执行节点，按 3:1 应为：collect=KDA, process=KDA, analyze=KDA, report=MLA
func registerDemoWorkflow(engine *bagua.Engine) {
	wf := &bagua.Workflow{
		ID:   "wf-demo",
		Name: "市场调研工作流",
		Mode: bagua.Dizai, // 地载阵 - 顺序执行
		Nodes: []bagua.Node{
			{ID: "start", Type: bagua.NodeStart, Name: "开始"},
			{ID: "collect", Type: bagua.NodeTool, Name: "数据收集"},
			{ID: "process", Type: bagua.NodeTool, Name: "数据处理"},
			{ID: "analyze", Type: bagua.NodeTool, Name: "数据分析"},
			{ID: "report", Type: bagua.NodeTool, Name: "报告生成",
				// report 为第4个执行节点，自动分配为 MLA；配置 AttnRes 残差源
				ResidualSources: []bagua.ResidualSource{
					{NodeID: "collect", Alpha: 0.4},
					{NodeID: "analyze", Alpha: 0.6},
				},
			},
			{ID: "end", Type: bagua.NodeEnd, Name: "结束"},
		},
		Edges: []bagua.Edge{
			{ID: "e1", From: "start", To: "collect"},
			{ID: "e2", From: "collect", To: "process"},
			{ID: "e3", From: "process", To: "analyze"},
			{ID: "e4", From: "analyze", To: "report"},
			{ID: "e5", From: "report", To: "end"},
		},
	}
	if err := engine.RegisterWorkflow(wf); err != nil {
		log.Printf("注册演示工作流失败: %v", err)
	}
}

// demoNodeExecutor 演示节点执行器
// 模拟节点前向计算，输出可被 AttnRes 检索的表示
type demoNodeExecutor struct{}

func (e *demoNodeExecutor) Execute(ctx context.Context, node bagua.Node, ec *bagua.ExecutionContext) (*bagua.NodeState, error) {
	// 模拟产生数值型输出（供 AttnRes α 算子加权融合）
	output := map[string]interface{}{
		"node_id": node.ID,
		"records": 100,
		"score":   0.85,
	}
	// MLA 节点打印收到的 AttnRes 聚合信息（演示 α 算子效果）
	if node.Mode == bagua.ModeMLA {
		if agg, ok := node.Config["_attnres"]; ok {
			if a, ok := agg.(*bagua.AttnResAggregation); ok {
				fmt.Printf("  ⟐ MLA 节点 %s 收到 AttnRes 聚合: %d 个残差源, 总α=%.2f\n",
					node.ID, len(a.Sources), a.TotalAlpha)
				for _, src := range a.Sources {
					fmt.Printf("     • 源 %s: α=%.2f (归一化=%.2f)\n",
						src.NodeID, src.Alpha, src.NormalizedAlpha)
				}
			}
		}
	}
	return &bagua.NodeState{
		Status: "completed",
		Output: output,
		Mode:   node.Mode,
	}, nil
}
