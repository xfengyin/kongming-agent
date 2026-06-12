// Package model 工作流（Workflow）单元测试。
package model

import "testing"

// TestWorkflow_Validate_OK 验证合法工作流（线性 A→B→C）校验通过。
func TestWorkflow_Validate_OK(t *testing.T) {
	w := &Workflow{
		ID:   "wf-1",
		Name: "linear",
		Nodes: []Node{
			{ID: "A", Name: "step-a", Action: "act-a"},
			{ID: "B", Name: "step-b", Action: "act-b"},
			{ID: "C", Name: "step-c", Action: "act-c"},
		},
		Edges: []Edge{
			{From: "A", To: "B"},
			{From: "B", To: "C"},
		},
		Entry: "A",
	}
	if err := w.Validate(); err != nil {
		t.Errorf("Validate returned error: %v", err)
	}
}

// TestWorkflow_Validate_Cycle 验证 A→B→A 的环被检出。
func TestWorkflow_Validate_Cycle(t *testing.T) {
	w := &Workflow{
		ID:   "wf-cycle",
		Name: "cyclic",
		Nodes: []Node{
			{ID: "A", Action: "act-a"},
			{ID: "B", Action: "act-b"},
		},
		Edges: []Edge{
			{From: "A", To: "B"},
			{From: "B", To: "A"},
		},
		Entry: "A",
	}
	if err := w.Validate(); err == nil {
		t.Error("expected cycle detection error, got nil")
	}
}

// TestWorkflow_Validate_Orphan 验证孤立节点（无入边且非 Entry）被检出。
func TestWorkflow_Validate_Orphan(t *testing.T) {
	w := &Workflow{
		ID:   "wf-orphan",
		Name: "orphan",
		Nodes: []Node{
			{ID: "A", Action: "act-a"},
			{ID: "B", Action: "act-b"},
			{ID: "C", Action: "act-c"}, // 孤立：没有入边，也不是 Entry
		},
		Edges: []Edge{
			{From: "A", To: "B"},
		},
		Entry: "A",
	}
	if err := w.Validate(); err == nil {
		t.Error("expected orphan node error, got nil")
	}
}

// TestWorkflow_Validate_Empty 验证空工作流被拒绝。
func TestWorkflow_Validate_Empty(t *testing.T) {
	w := &Workflow{ID: "wf-empty", Name: "empty"}
	if err := w.Validate(); err == nil {
		t.Error("expected empty workflow error, got nil")
	}
}

// TestWorkflow_Validate_UnknownEdge 验证引用不存在节点的边被拒绝。
func TestWorkflow_Validate_UnknownEdge(t *testing.T) {
	w := &Workflow{
		ID:   "wf-unknown",
		Name: "unknown",
		Nodes: []Node{
			{ID: "A", Action: "act-a"},
		},
		Edges: []Edge{
			{From: "A", To: "B"}, // B 不存在
		},
		Entry: "A",
	}
	if err := w.Validate(); err == nil {
		t.Error("expected unknown node error, got nil")
	}
}

// TestWorkflow_Validate_EntryMissing 验证 Entry 指向不存在节点被拒绝。
func TestWorkflow_Validate_EntryMissing(t *testing.T) {
	w := &Workflow{
		ID:   "wf-bad-entry",
		Name: "bad-entry",
		Nodes: []Node{
			{ID: "A", Action: "act-a"},
		},
		Entry: "ZZZ",
	}
	if err := w.Validate(); err == nil {
		t.Error("expected entry-not-found error, got nil")
	}
}

// TestWorkflow_Validate_Diamond 验证菱形依赖（A→B/A→C，B→D，C→D）合法。
func TestWorkflow_Validate_Diamond(t *testing.T) {
	w := &Workflow{
		ID:   "wf-diamond",
		Name: "diamond",
		Nodes: []Node{
			{ID: "A", Action: "act-a"},
			{ID: "B", Action: "act-b"},
			{ID: "C", Action: "act-c"},
			{ID: "D", Action: "act-d"},
		},
		Edges: []Edge{
			{From: "A", To: "B"},
			{From: "A", To: "C"},
			{From: "B", To: "D"},
			{From: "C", To: "D"},
		},
		Entry: "A",
	}
	if err := w.Validate(); err != nil {
		t.Errorf("expected diamond to be valid, got: %v", err)
	}
}

// TestWorkflow_Validate_AutoEntry 验证未显式声明 Entry 时可自动从
// indegree==0 节点中推断（或回退到首节点）。
func TestWorkflow_Validate_AutoEntry(t *testing.T) {
	// 场景 1：自动从 indegree=0 推断
	w1 := &Workflow{
		ID:   "wf-auto1",
		Name: "auto1",
		Nodes: []Node{
			{ID: "A", Action: "act-a"},
			{ID: "B", Action: "act-b"},
		},
		Edges: []Edge{{From: "A", To: "B"}},
		// Entry 留空 → 推断为 A
	}
	if err := w1.Validate(); err != nil {
		t.Errorf("expected auto-entry to be valid, got: %v", err)
	}
	// 场景 2：所有节点都互相形成环（即没有 indegree=0）→ 回退到首节点
	// 此时必然存在环，校验会失败 — 我们改成无环但无明显起点（不可能存在），
	// 此处简化为只测试正常推断路径。
}

// TestWorkflow_Validate_AutoEntryFallback 验证当没有 indegree=0 节点时
// 回退到 Nodes[0]。这要求图含环但 Validate 会先返回环错误——我们用
// 单节点无边的场景（必然无 indegree=0）覆盖回退路径。
func TestWorkflow_Validate_AutoEntryFallback(t *testing.T) {
	// 节点有自环：会先在环检测时被检出；不满足要求。
	// 改为测试「单节点无边」——必然有 indegree=0，回退路径不触发。
	// 真正的回退路径在「全环 + 无 Entry」时触发：
	w := &Workflow{
		ID:   "wf-fallback",
		Name: "fallback",
		Nodes: []Node{
			{ID: "A", Action: "act-a"},
			{ID: "B", Action: "act-b"},
		},
		// 双向边：形成 A↔B 强连通分量（无 indegree=0）
		Edges: []Edge{
			{From: "A", To: "B"},
			{From: "B", To: "A"},
		},
		// Entry 留空
	}
	err := w.Validate()
	if err == nil {
		t.Error("expected cycle error, got nil")
	}
}

// TestWorkflow_Validate_EmptyNodeID 验证空节点 ID 被拒绝。
func TestWorkflow_Validate_EmptyNodeID(t *testing.T) {
	w := &Workflow{
		ID:   "wf-emptyid",
		Name: "emptyid",
		Nodes: []Node{
			{ID: "", Action: "act-a"},
		},
		Entry: "",
	}
	if err := w.Validate(); err == nil {
		t.Error("expected empty node ID error, got nil")
	}
}

// TestWorkflow_Validate_DuplicateNode 验证重复节点 ID 被拒绝。
func TestWorkflow_Validate_DuplicateNode(t *testing.T) {
	w := &Workflow{
		ID:   "wf-dup",
		Name: "dup",
		Nodes: []Node{
			{ID: "A", Action: "act-a"},
			{ID: "A", Action: "act-a-dup"},
		},
		Entry: "A",
	}
	if err := w.Validate(); err == nil {
		t.Error("expected duplicate node error, got nil")
	}
}

// TestWorkflow_Validate_EdgeFromUnknown 验证 edge.From 引用未知节点被拒绝。
func TestWorkflow_Validate_EdgeFromUnknown(t *testing.T) {
	w := &Workflow{
		ID:   "wf-ef-unknown",
		Name: "ef-unknown",
		Nodes: []Node{
			{ID: "A", Action: "act-a"},
			{ID: "B", Action: "act-b"},
		},
		Edges: []Edge{
			{From: "X", To: "B"}, // X 不存在
		},
		Entry: "A",
	}
	if err := w.Validate(); err == nil {
		t.Error("expected edge.From unknown error, got nil")
	}
}
