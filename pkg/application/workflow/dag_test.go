// Package workflow 工作流应用层 - DAG 单元测试。
//
// 本测试覆盖 buildDAG 与 topologicalLevels 两个核心工具函数，遵循 TDD 原则：
// 先写测试 → 看 RED → 实现 → GREEN。本文件是 application/workflow 内部使用的
// 算法原语，外部不应直接依赖。
package workflow

import (
	"testing"

	"github.com/zhuge/kongming/pkg/domain/model"
)

// TestBuildDAG_Linear 验证线性 DAG 的邻接表正确构建。
func TestBuildDAG_Linear(t *testing.T) {
	wf := makeLinearWorkflow()
	g := buildDAG(wf)

	// 线性 A→B→C 的邻接表应为：A→[B], B→[C], C→[]
	if got := g["A"]; len(got) != 1 || got[0] != "B" {
		t.Errorf("A 的后继应为 [B]，实际 %v", got)
	}
	if got := g["B"]; len(got) != 1 || got[0] != "C" {
		t.Errorf("B 的后继应为 [C]，实际 %v", got)
	}
	if got := g["C"]; len(got) != 0 {
		t.Errorf("C 不应有后继，实际 %v", got)
	}
}

// TestBuildDAG_Empty 验证空工作流的邻接表为空。
func TestBuildDAG_Empty(t *testing.T) {
	g := buildDAG(&model.Workflow{})
	if len(g) != 0 {
		t.Errorf("空工作流应返回空邻接表，实际 %v", g)
	}
}

// TestBuildDAG_Diamond 验证菱形 DAG（A→B, A→C, B→D, C→D）邻接表正确。
func TestBuildDAG_Diamond(t *testing.T) {
	wf := makeDiamondWorkflow()
	g := buildDAG(wf)
	if got := g["A"]; len(got) != 2 {
		t.Errorf("A 的后继数应为 2，实际 %v", got)
	}
	if got := g["D"]; len(got) != 0 {
		t.Errorf("D 不应有后继，实际 %v", got)
	}
}

// TestTopologicalLevels_Linear 验证线性 DAG 的拓扑分层为 [[A],[B],[C]]。
func TestTopologicalLevels_Linear(t *testing.T) {
	g := buildDAG(makeLinearWorkflow())
	levels := topologicalLevels(g)
	if len(levels) != 3 {
		t.Fatalf("线性 DAG 应有 3 层，实际 %d 层：%v", len(levels), levels)
	}
	if len(levels[0]) != 1 || levels[0][0] != "A" {
		t.Errorf("第 1 层应为 [A]，实际 %v", levels[0])
	}
	if len(levels[1]) != 1 || levels[1][0] != "B" {
		t.Errorf("第 2 层应为 [B]，实际 %v", levels[1])
	}
	if len(levels[2]) != 1 || levels[2][0] != "C" {
		t.Errorf("第 3 层应为 [C]，实际 %v", levels[2])
	}
}

// TestTopologicalLevels_Diamond 验证菱形 DAG（A→B,A→C,B→D,C→D）的拓扑分层。
//
// 期望结构（具体每层内顺序不固定，但长度与覆盖必须一致）：
//
//	[[A], [B,C], [D]]
func TestTopologicalLevels_Diamond(t *testing.T) {
	g := buildDAG(makeDiamondWorkflow())
	levels := topologicalLevels(g)
	if len(levels) != 3 {
		t.Fatalf("菱形 DAG 应有 3 层，实际 %d 层：%v", len(levels), levels)
	}
	if len(levels[0]) != 1 || levels[0][0] != "A" {
		t.Errorf("第 1 层应为 [A]，实际 %v", levels[0])
	}
	if len(levels[1]) != 2 {
		t.Errorf("第 2 层应有 2 节点，实际 %v", levels[1])
	}
	if !contains(levels[1], "B") || !contains(levels[1], "C") {
		t.Errorf("第 2 层应包含 B 与 C，实际 %v", levels[1])
	}
	if len(levels[2]) != 1 || levels[2][0] != "D" {
		t.Errorf("第 3 层应为 [D]，实际 %v", levels[2])
	}
}

// TestTopologicalLevels_Single 验证单节点 DAG 只有 1 层。
func TestTopologicalLevels_Single(t *testing.T) {
	g := map[string][]string{"X": nil}
	levels := topologicalLevels(g)
	if len(levels) != 1 || len(levels[0]) != 1 || levels[0][0] != "X" {
		t.Errorf("单节点应返回 [[X]]，实际 %v", levels)
	}
}

// makeLinearWorkflow 构造线性 A→B→C 工作流（用于 DAG 测试）。
func makeLinearWorkflow() *model.Workflow {
	return &model.Workflow{
		ID: "linear",
		Nodes: []model.Node{
			{ID: "A", Action: "act-a"},
			{ID: "B", Action: "act-b"},
			{ID: "C", Action: "act-c"},
		},
		Edges: []model.Edge{
			{From: "A", To: "B"},
			{From: "B", To: "C"},
		},
	}
}

// makeDiamondWorkflow 构造菱形 A→{B,C}→D 工作流（用于 DAG 测试）。
func makeDiamondWorkflow() *model.Workflow {
	return &model.Workflow{
		ID: "diamond",
		Nodes: []model.Node{
			{ID: "A", Action: "act-a"},
			{ID: "B", Action: "act-b"},
			{ID: "C", Action: "act-c"},
			{ID: "D", Action: "act-d"},
		},
		Edges: []model.Edge{
			{From: "A", To: "B"},
			{From: "A", To: "C"},
			{From: "B", To: "D"},
			{From: "C", To: "D"},
		},
	}
}

// contains 检查字符串切片是否包含某元素。
func contains(s []string, e string) bool {
	for _, v := range s {
		if v == e {
			return true
		}
	}
	return false
}
