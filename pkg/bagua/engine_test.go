// 八卦阵混合执行引擎测试
// 验证 KDA/MLA 3:1 混合分配 + AttnRes 残差聚合

package bagua

import (
	"context"
	"testing"
)

// mockExecutor 模拟节点执行器
// 记录执行时是否收到 AttnRes 残差，以及执行模式
type mockExecutor struct {
	executedNodes []string
	attnresSeen   map[string]bool
	attnresAggs   map[string]*AttnResAggregation
}

func newMockExecutor() *mockExecutor {
	return &mockExecutor{
		attnresSeen: make(map[string]bool),
		attnresAggs: make(map[string]*AttnResAggregation),
	}
}

func (m *mockExecutor) Execute(ctx context.Context, node Node, ec *ExecutionContext) (*NodeState, error) {
	m.executedNodes = append(m.executedNodes, node.ID)
	// 检查是否注入了 AttnRes 聚合结果（MLA 模式才会有）
	if agg, ok := node.Config["_attnres"]; ok {
		m.attnresSeen[node.ID] = true
		// 记录聚合结果供测试断言
		if a, ok := agg.(*AttnResAggregation); ok {
			m.attnresAggs[node.ID] = a
		}
	}
	return &NodeState{
		Status: "completed",
		Output: map[string]interface{}{
			"node_id": node.ID,
			"count":   10,
			"score":   0.9,
		},
		Mode: node.Mode,
	}, nil
}

// buildLinearWorkflow 构建线性工作流：start -> n1 -> n2 -> n3 -> n4 -> end
func buildLinearWorkflow(mode BaguaMode) *Workflow {
	return &Workflow{
		ID:   "wf-test",
		Name: "test-workflow",
		Mode: mode,
		Nodes: []Node{
			{ID: "start", Type: NodeStart, Name: "开始"},
			{ID: "n1", Type: NodeTool, Name: "节点1"},
			{ID: "n2", Type: NodeTool, Name: "节点2"},
			{ID: "n3", Type: NodeTool, Name: "节点3"},
			{ID: "n4", Type: NodeTool, Name: "节点4"},
			{ID: "end", Type: NodeEnd, Name: "结束"},
		},
		Edges: []Edge{
			{ID: "e1", From: "start", To: "n1"},
			{ID: "e2", From: "n1", To: "n2"},
			{ID: "e3", From: "n2", To: "n3"},
			{ID: "e4", From: "n3", To: "n4"},
			{ID: "e5", From: "n4", To: "end"},
		},
	}
}

// TestAssignExecutionModes 验证 3:1 KDA/MLA 自动分配
func TestAssignExecutionModes(t *testing.T) {
	engine := NewEngine()
	wf := buildLinearWorkflow(Dizai)
	if err := engine.RegisterWorkflow(wf); err != nil {
		t.Fatalf("注册工作流失败: %v", err)
	}

	// 4 个执行节点（n1-n4），按 3:1 应为：n1=KDA, n2=KDA, n3=KDA, n4=MLA
	modes := map[string]ExecutionMode{}
	for _, node := range wf.Nodes {
		if node.Type == NodeStart || node.Type == NodeEnd {
			continue
		}
		modes[node.ID] = node.Mode
	}

	if modes["n1"] != ModeKDA || modes["n2"] != ModeKDA || modes["n3"] != ModeKDA {
		t.Errorf("前3个执行节点应为 KDA 模式，实际: n1=%s, n2=%s, n3=%s",
			modes["n1"], modes["n2"], modes["n3"])
	}
	if modes["n4"] != ModeMLA {
		t.Errorf("第4个执行节点应为 MLA 模式，实际: %s", modes["n4"])
	}
}

// TestExplicitModePreserved 验证显式指定的 Mode 不被覆盖
func TestExplicitModePreserved(t *testing.T) {
	engine := NewEngine()
	wf := buildLinearWorkflow(Dizai)
	// 显式指定 n1 为 MLA
	wf.Nodes[1].Mode = ModeMLA
	if err := engine.RegisterWorkflow(wf); err != nil {
		t.Fatalf("注册工作流失败: %v", err)
	}
	if wf.Nodes[1].Mode != ModeMLA {
		t.Errorf("显式指定的 MLA 模式应保留，实际: %s", wf.Nodes[1].Mode)
	}
}

// TestDizaiExecution 验证地载阵顺序执行 + 混合模式
func TestDizaiExecution(t *testing.T) {
	engine := NewEngine()
	executor := newMockExecutor()
	engine.RegisterNodeExecutor(NodeTool, executor)
	engine.RegisterNodeExecutor(NodeStart, executor)
	engine.RegisterNodeExecutor(NodeEnd, executor)

	wf := buildLinearWorkflow(Dizai)
	if err := engine.RegisterWorkflow(wf); err != nil {
		t.Fatalf("注册工作流失败: %v", err)
	}

	ec, err := engine.Execute(context.Background(), wf.ID, map[string]interface{}{})
	if err != nil {
		t.Fatalf("执行失败: %v", err)
	}
	// 所有节点应被执行
	for _, id := range []string{"start", "n1", "n2", "n3", "n4", "end"} {
		state, ok := ec.NodeStates[id]
		if !ok {
			t.Errorf("节点 %s 未被执行", id)
			continue
		}
		if state.Status != "completed" {
			t.Errorf("节点 %s 状态应为 completed，实际 %s", id, state.Status)
		}
	}
}

// TestAttnResGathering 验证 AttnRes α 算子残差聚合
// MLA 节点 n4 配置多残差源（n1 α=0.3, n3 α=0.7），执行时应收到归一化聚合结果
func TestAttnResGathering(t *testing.T) {
	engine := NewEngine()
	executor := newMockExecutor()
	engine.RegisterNodeExecutor(NodeTool, executor)
	engine.RegisterNodeExecutor(NodeStart, executor)
	engine.RegisterNodeExecutor(NodeEnd, executor)

	wf := buildLinearWorkflow(Dizai)
	// 为 n4（MLA 节点）配置 AttnRes 残差源：n1 α=0.3, n3 α=0.7
	wf.Nodes[4].ResidualSources = []ResidualSource{
		{NodeID: "n1", Alpha: 0.3},
		{NodeID: "n3", Alpha: 0.7},
	}
	if err := engine.RegisterWorkflow(wf); err != nil {
		t.Fatalf("注册工作流失败: %v", err)
	}

	ec, err := engine.Execute(context.Background(), wf.ID, map[string]interface{}{})
	if err != nil {
		t.Fatalf("执行失败: %v", err)
	}

	// n4 是 MLA 节点且配置了残差源，应看到 AttnRes 聚合
	if !executor.attnresSeen["n4"] {
		t.Errorf("MLA 节点 n4 应收到 AttnRes 残差聚合，实际未收到")
	}
	// KDA 节点（n1-n3）不应收到残差（即使有配置也不聚合）
	for _, id := range []string{"n1", "n2", "n3"} {
		if executor.attnresSeen[id] {
			t.Errorf("KDA 节点 %s 不应收到 AttnRes 残差", id)
		}
	}
	// 验证 α 归一化：n1=0.3, n3=0.7，总和 1.0，归一化后 n1=0.3, n3=0.7
	agg := executor.attnresAggs["n4"]
	if agg == nil {
		t.Fatalf("n4 应有 AttnRes 聚合结果")
	}
	if len(agg.Sources) != 2 {
		t.Errorf("期望2个残差源，实际 %d", len(agg.Sources))
	}
	// 验证归一化权重总和为 1.0
	totalNorm := 0.0
	for _, src := range agg.Sources {
		totalNorm += src.NormalizedAlpha
	}
	if totalNorm < 0.99 || totalNorm > 1.01 {
		t.Errorf("归一化权重总和应接近 1.0，实际 %v", totalNorm)
	}
	// 验证数值型字段加权融合（mock 输出 count=10, score=0.9，两源相同则融合后应仍为该值）
	fused, ok := agg.FusedOutput.(map[string]interface{})
	if !ok {
		t.Fatalf("融合输出应为 map 类型，实际 %T", agg.FusedOutput)
	}
	if count, ok := fused["count"].(float64); !ok || count != 10 {
		t.Errorf("融合后 count 应为 10，实际 %v", fused["count"])
	}
	// 验证节点表示已存储（供 AttnRes 检索）
	rep, ok := ec.GetNodeRepresentation("n1")
	if !ok {
		t.Errorf("节点 n1 的表示应可检索")
	}
	if rep.Output == nil {
		t.Errorf("节点 n1 表示输出不应为空")
	}
}

// TestTiangaiParallelExecution 验证天覆阵并行执行
func TestTiangaiParallelExecution(t *testing.T) {
	engine := NewEngine()
	executor := newMockExecutor()
	engine.RegisterNodeExecutor(NodeTool, executor)
	engine.RegisterNodeExecutor(NodeStart, executor)
	engine.RegisterNodeExecutor(NodeEnd, executor)

	wf := buildLinearWorkflow(Tiangai)
	if err := engine.RegisterWorkflow(wf); err != nil {
		t.Fatalf("注册工作流失败: %v", err)
	}

	ec, err := engine.Execute(context.Background(), wf.ID, nil)
	if err != nil {
		t.Fatalf("执行失败: %v", err)
	}
	if len(ec.NodeStates) != 6 {
		t.Errorf("期望6个节点状态，实际 %d", len(ec.NodeStates))
	}
}

// TestNodeRepresentationRetrieval 验证节点表示检索
func TestNodeRepresentationRetrieval(t *testing.T) {
	engine := NewEngine()
	executor := newMockExecutor()
	engine.RegisterNodeExecutor(NodeTool, executor)
	engine.RegisterNodeExecutor(NodeStart, executor)
	engine.RegisterNodeExecutor(NodeEnd, executor)

	wf := buildLinearWorkflow(Dizai)
	if err := engine.RegisterWorkflow(wf); err != nil {
		t.Fatalf("注册工作流失败: %v", err)
	}

	ec, _ := engine.Execute(context.Background(), wf.ID, nil)

	// 验证每个执行节点的表示都可检索，且 Mode 正确
	for _, id := range []string{"n1", "n2", "n3", "n4"} {
		rep, ok := ec.GetNodeRepresentation(id)
		if !ok {
			t.Errorf("节点 %s 表示应可检索", id)
			continue
		}
		mode, _ := rep.Meta["mode"].(string)
		if mode == "" {
			t.Errorf("节点 %s 表示应包含执行模式", id)
		}
	}
}

// TestWorkflowValidation 验证工作流校验
func TestWorkflowValidation(t *testing.T) {
	engine := NewEngine()

	// 缺少开始节点
	wfNoStart := &Workflow{
		ID:   "wf-no-start",
		Mode: Dizai,
		Nodes: []Node{
			{ID: "end", Type: NodeEnd},
		},
	}
	if err := engine.RegisterWorkflow(wfNoStart); err == nil {
		t.Errorf("缺少开始节点应报错")
	}

	// 缺少结束节点
	wfNoEnd := &Workflow{
		ID:   "wf-no-end",
		Mode: Dizai,
		Nodes: []Node{
			{ID: "start", Type: NodeStart},
		},
	}
	if err := engine.RegisterWorkflow(wfNoEnd); err == nil {
		t.Errorf("缺少结束节点应报错")
	}
}
