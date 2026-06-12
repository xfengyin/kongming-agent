// Package modes 八卦阵调度算法 - 单元测试。
//
// 覆盖三种典型阵型行为：
//   - Dizai_StopsOnError    顺序阵遇错即停
//   - Tiangai_AggregatesErrors  并行阵聚合多 goroutine 错误
//   - Huyi_Condition        条件分支按 Edge.Condition 路由
package modes

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"

	"github.com/zhuge/kongming/pkg/domain/model"
	"github.com/zhuge/kongming/pkg/domain/port"
)

// okExec 总是成功的执行器。
type okExec struct{ calls *atomic.Int32 }

func (o *okExec) Execute(_ context.Context, n model.Node, _ *model.ExecutionContext) (*model.NodeState, error) {
	o.calls.Add(1)
	return &model.NodeState{ID: n.ID, Status: model.NodeStatusOK}, nil
}

// failExec 总是失败的执行器。
type failExec struct{ calls *atomic.Int32 }

func (f *failExec) Execute(_ context.Context, n model.Node, _ *model.ExecutionContext) (*model.NodeState, error) {
	f.calls.Add(1)
	return nil, errors.New("forced fail")
}

// makeLinearDizaiWF 构造 3 节点线性工作流（Dizai 测试用）。
func makeLinearDizaiWF() *model.Workflow {
	return &model.Workflow{
		ID:   "dizai-linear",
		Name: "dizai-linear",
		Mode: model.Dizai,
		Nodes: []model.Node{
			{ID: "s", Type: model.NodeStart},
			{ID: "m1", Type: model.NodeAction},
			{ID: "m2", Type: model.NodeAction},
			{ID: "e", Type: model.NodeEnd},
		},
		Edges: []model.Edge{
			{From: "s", To: "m1"},
			{From: "m1", To: "m2"},
			{From: "m2", To: "e"},
		},
	}
}

// makeDiamondTiangaiWF 构造菱形 DAG（Tiangai 测试用）。
func makeDiamondTiangaiWF() *model.Workflow {
	return &model.Workflow{
		ID:   "tiangai-diamond",
		Name: "tiangai-diamond",
		Mode: model.Tiangai,
		Nodes: []model.Node{
			{ID: "s", Type: model.NodeStart},
			{ID: "a", Type: model.NodeAction},
			{ID: "b", Type: model.NodeAction},
			{ID: "c", Type: model.NodeAction},
			{ID: "e", Type: model.NodeEnd},
		},
		Edges: []model.Edge{
			{From: "s", To: "a"},
			{From: "s", To: "b"},
			{From: "s", To: "c"},
			{From: "a", To: "e"},
			{From: "b", To: "e"},
			{From: "c", To: "e"},
		},
	}
}

// makeHuyiWF 构造带条件的 Huyi 工作流。
//
// 结构：s → m1 → {c1, c2, c3} → e
//   - c1 条件：`var.x=="yes"` （m1 写 x="yes" → 命中 c1）
//   - c2 条件：`var.x=="no"`  （不命中）
//   - c3 条件：`""`           （无条件命中）
func makeHuyiWF() *model.Workflow {
	return &model.Workflow{
		ID:   "huyi-branch",
		Name: "huyi-branch",
		Mode: model.Huyi,
		Nodes: []model.Node{
			{ID: "s", Type: model.NodeStart},
			{ID: "m1", Type: model.NodeAction},
			{ID: "c1", Type: model.NodeAction},
			{ID: "c2", Type: model.NodeAction},
			{ID: "c3", Type: model.NodeAction},
			{ID: "e", Type: model.NodeEnd},
		},
		Edges: []model.Edge{
			{From: "s", To: "m1"},
			{From: "m1", To: "c1", Condition: `var.x=="yes"`},
			{From: "m1", To: "c2", Condition: `var.x=="no"`},
			{From: "m1", To: "c3", Condition: ""},
		},
	}
}

// writerExec 把 __yes__ 写入 ec.Variables["x"] 用于 Huyi 条件测试。
type writerExec struct{ id string }

func (w *writerExec) Execute(_ context.Context, n model.Node, ec *model.ExecutionContext) (*model.NodeState, error) {
	if n.ID == "m1" {
		ec.SetVar("x", "yes")
	}
	return &model.NodeState{ID: n.ID, Status: model.NodeStatusOK}, nil
}

// branchExec 简单的「被调用就计数」节点。
type branchExec struct {
	id    string
	calls *atomic.Int32
}

func (b *branchExec) Execute(_ context.Context, n model.Node, _ *model.ExecutionContext) (*model.NodeState, error) {
	if b.calls != nil {
		b.calls.Add(1)
	}
	return &model.NodeState{ID: n.ID, Status: model.NodeStatusOK}, nil
}

// TestDizai_StopsOnError 验证 Dizai 阵遇错立即停止后续节点。
func TestDizai_StopsOnError(t *testing.T) {
	wf := makeLinearDizaiWF()
	// 注册 failExec 给 m1，okExec 给 m2/start/end
	fail := &failExec{calls: &atomic.Int32{}}
	ok := &okExec{calls: &atomic.Int32{}}
	nodes := map[model.NodeType]port.NodeExecutor{
		model.NodeStart:  &branchExec{id: "s"},
		model.NodeAction: fail, // 所有 Action 节点都失败
		model.NodeEnd:    &branchExec{id: "e"},
	}
	ec := &model.ExecutionContext{
		WorkflowID: wf.ID, RunID: "test-run",
		Variables: map[string]any{}, NodeStates: map[string]model.NodeState{},
	}
	_, err := Dizai(context.Background(), wf, ec, nodes)
	if err == nil {
		t.Fatal("期望 Dizai 报错，实际通过")
	}
	// m1 失败后 m2 不应被执行
	if fail.calls.Load() != 1 {
		t.Errorf("m1 应被调用 1 次，实际 %d", fail.calls.Load())
	}
	// 节点状态：s + m1（failed），m2/e 不应有 NodeState
	_ = ok // 不验证 ok 的调用次数
	if _, ok := ec.NodeStates["m2"]; ok {
		t.Error("m2 不应有 NodeState（m1 失败后 Dizai 应停止）")
	}
}

// TestTiangai_AggregatesErrors 验证 Tiangai 阵能聚合并行节点的错误。
func TestTiangai_AggregatesErrors(t *testing.T) {
	wf := makeDiamondTiangaiWF()
	// 所有 Action 节点都失败
	fail := &failExec{calls: &atomic.Int32{}}
	nodes := map[model.NodeType]port.NodeExecutor{
		model.NodeStart:  &branchExec{id: "s"},
		model.NodeAction: fail,
		model.NodeEnd:    &branchExec{id: "e"},
	}
	ec := &model.ExecutionContext{
		WorkflowID: wf.ID, RunID: "test-run",
		Variables: map[string]any{}, NodeStates: map[string]model.NodeState{},
	}
	_, err := Tiangai(context.Background(), wf, ec, nodes)
	if err == nil {
		t.Fatal("期望 Tiangai 报错，实际通过")
	}
	// 三个 Action 节点（a, b, c）应都被调用
	if fail.calls.Load() != 3 {
		t.Errorf("a/b/c 应各被调用 1 次共 3 次，实际 %d", fail.calls.Load())
	}
}

// TestHuyi_Condition 验证 Huyi 阵按 Edge.Condition 选择分支。
//
// 场景：m1 写 x="yes" → c1 条件 var.x=="yes" 命中、c2 不命中、c3 无条件命中。
func TestHuyi_Condition(t *testing.T) {
	wf := makeHuyiWF()
	calls := &atomic.Int32{}
	nodes := map[model.NodeType]port.NodeExecutor{
		model.NodeStart:  &branchExec{id: "s"},
		model.NodeAction: &writerExec{id: "m1"},
		model.NodeEnd:    &branchExec{id: "e"},
	}
	// 把 c1/c2/c3 注册为 branchExec 计数
	cBranch := &branchExec{id: "branches", calls: calls}
	// 简单做法：Action 的 writer 只对 m1 生效；其它分支用空 writer
	// 由于 writerExec 对所有 Action 都生效且无害，c1/c2/c3 也会被调用
	// 改用：给所有 Action 注册一个混合执行器
	hybrid := &huyiHybrid{writer: &writerExec{id: "m1"}}
	nodes[model.NodeAction] = hybrid
	_ = cBranch

	ec := &model.ExecutionContext{
		WorkflowID: wf.ID, RunID: "test-run",
		Variables: map[string]any{}, NodeStates: map[string]model.NodeState{},
	}
	_, err := Huyi(context.Background(), wf, ec, nodes)
	if err != nil {
		t.Fatalf("Huyi 失败: %v", err)
	}
	// 验证 c2 被跳过（Huyi 会在 ec.NodeStates 写 Skipped）
	c2State, ok := ec.NodeStates["c2"]
	if !ok {
		t.Error("c2 应有 NodeState（标记为 Skipped）")
	} else if c2State.Status != model.NodeStatusSkipped {
		t.Errorf("c2 状态应为 skipped，实际 %s", c2State.Status)
	}
	// 验证 c1 命中
	if _, ok := ec.NodeStates["c1"]; !ok {
		t.Error("c1 应有 NodeState（命中条件）")
	}
	// 验证 c3 命中（无条件）
	if _, ok := ec.NodeStates["c3"]; !ok {
		t.Error("c3 应有 NodeState（无条件命中）")
	}
}

// huyiHybrid 混合执行器：m1 写 var、其它节点直接成功。
type huyiHybrid struct {
	writer *writerExec
}

func (h *huyiHybrid) Execute(ctx context.Context, n model.Node, ec *model.ExecutionContext) (*model.NodeState, error) {
	if n.ID == "m1" {
		return h.writer.Execute(ctx, n, ec)
	}
	return &model.NodeState{ID: n.ID, Status: model.NodeStatusOK}, nil
}

// === 覆盖率补充测试 ===

// TestFengyang_HappyPath 验证 Fengyang 阵在无超时场景下能正常顺序执行。
func TestFengyang_HappyPath(t *testing.T) {
	wf := &model.Workflow{
		ID: "fengyang-hp", Mode: model.Fengyang,
		Nodes: []model.Node{
			{ID: "s", Type: model.NodeStart},
			{ID: "m", Type: model.NodeAction},
			{ID: "e", Type: model.NodeEnd},
		},
		Edges: []model.Edge{{From: "s", To: "m"}, {From: "m", To: "e"}},
	}
	ok := &okExec{calls: &atomic.Int32{}}
	nodes := map[model.NodeType]port.NodeExecutor{
		model.NodeStart:  &branchExec{id: "s"},
		model.NodeAction: ok,
		model.NodeEnd:    &branchExec{id: "e"},
	}
	ec := &model.ExecutionContext{
		WorkflowID: wf.ID, RunID: "test",
		Variables: map[string]any{}, NodeStates: map[string]model.NodeState{},
	}
	_, err := Fengyang(context.Background(), wf, ec, nodes)
	if err != nil {
		t.Fatalf("Fengyang 失败: %v", err)
	}
	if ok.calls.Load() != 1 {
		t.Errorf("m 应被调用 1 次，实际 %d", ok.calls.Load())
	}
}

// TestFengyang_OuterCtxCancel 验证外层 ctx 取消能终止 Fengyang。
func TestFengyang_OuterCtxCancel(t *testing.T) {
	wf := &model.Workflow{
		ID: "fengyang-cancel", Mode: model.Fengyang,
		Nodes: []model.Node{
			{ID: "s", Type: model.NodeStart},
			{ID: "m", Type: model.NodeAction},
			{ID: "e", Type: model.NodeEnd},
		},
		Edges: []model.Edge{{From: "s", To: "m"}, {From: "m", To: "e"}},
	}
	// 一个会响应 ctx 取消的 exec
	slow := &slowExec{}
	nodes := map[model.NodeType]port.NodeExecutor{
		model.NodeStart:  &branchExec{id: "s"},
		model.NodeAction: slow,
		model.NodeEnd:    &branchExec{id: "e"},
	}
	ec := &model.ExecutionContext{
		WorkflowID: wf.ID, RunID: "test",
		Variables: map[string]any{}, NodeStates: map[string]model.NodeState{},
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // 立即取消
	_, err := Fengyang(ctx, wf, ec, nodes)
	if err == nil {
		t.Error("期望 ctx 取消时 Fengyang 返回错误")
	}
}

// TestYunzhui_ExhaustedRetries 验证 Yunzhui 在重试用尽后返回错误。
func TestYunzhui_ExhaustedRetries(t *testing.T) {
	wf := &model.Workflow{
		ID: "yunzhui-exhaust", Mode: model.Yunzhui,
		Nodes: []model.Node{
			{ID: "s", Type: model.NodeStart},
			{ID: "m", Type: model.NodeAction, Retries: 2},
			{ID: "e", Type: model.NodeEnd},
		},
		Edges: []model.Edge{{From: "s", To: "m"}, {From: "m", To: "e"}},
	}
	// 一直失败的 exec
	fail := &failExec{calls: &atomic.Int32{}}
	nodes := map[model.NodeType]port.NodeExecutor{
		model.NodeStart:  &branchExec{id: "s"},
		model.NodeAction: fail,
		model.NodeEnd:    &branchExec{id: "e"},
	}
	ec := &model.ExecutionContext{
		WorkflowID: wf.ID, RunID: "test",
		Variables: map[string]any{}, NodeStates: map[string]model.NodeState{},
	}
	_, err := Yunzhui(context.Background(), wf, ec, nodes)
	if err == nil {
		t.Fatal("期望 Yunzhui 重试耗尽报错")
	}
	// 1 + 2 = 3 次
	if got := fail.calls.Load(); got != 3 {
		t.Errorf("m 应被调用 3 次，实际 %d", got)
	}
}

// TestYunzhui_NoRetries 验证 Yunzhui 在 Retries=0 时只调用 1 次。
func TestYunzhui_NoRetries(t *testing.T) {
	wf := &model.Workflow{
		ID: "yunzhui-no", Mode: model.Yunzhui,
		Nodes: []model.Node{
			{ID: "s", Type: model.NodeStart},
			{ID: "m", Type: model.NodeAction, Retries: 0},
			{ID: "e", Type: model.NodeEnd},
		},
		Edges: []model.Edge{{From: "s", To: "m"}, {From: "m", To: "e"}},
	}
	ok := &okExec{calls: &atomic.Int32{}}
	nodes := map[model.NodeType]port.NodeExecutor{
		model.NodeStart:  &branchExec{id: "s"},
		model.NodeAction: ok,
		model.NodeEnd:    &branchExec{id: "e"},
	}
	ec := &model.ExecutionContext{
		WorkflowID: wf.ID, RunID: "test",
		Variables: map[string]any{}, NodeStates: map[string]model.NodeState{},
	}
	_, err := Yunzhui(context.Background(), wf, ec, nodes)
	if err != nil {
		t.Fatalf("Yunzhui 失败: %v", err)
	}
	if ok.calls.Load() != 1 {
		t.Errorf("m 应被调用 1 次，实际 %d", ok.calls.Load())
	}
}

// TestLongfei_HappyPath 验证 Longfei 阵入口层并行 + 后续顺序。
func TestLongfei_HappyPath(t *testing.T) {
	wf := &model.Workflow{
		ID: "longfei-hp", Mode: model.Longfei,
		Nodes: []model.Node{
			{ID: "s", Type: model.NodeStart},
			{ID: "a", Type: model.NodeAction},
			{ID: "b", Type: model.NodeAction},
			{ID: "e", Type: model.NodeEnd},
		},
		Edges: []model.Edge{
			{From: "s", To: "a"},
			{From: "s", To: "b"},
			{From: "a", To: "e"},
			{From: "b", To: "e"},
		},
	}
	ok := &okExec{calls: &atomic.Int32{}}
	nodes := map[model.NodeType]port.NodeExecutor{
		model.NodeStart:  &branchExec{id: "s"},
		model.NodeAction: ok,
		model.NodeEnd:    &branchExec{id: "e"},
	}
	ec := &model.ExecutionContext{
		WorkflowID: wf.ID, RunID: "test",
		Variables: map[string]any{}, NodeStates: map[string]model.NodeState{},
	}
	_, err := Longfei(context.Background(), wf, ec, nodes)
	if err != nil {
		t.Fatalf("Longfei 失败: %v", err)
	}
	if ok.calls.Load() != 2 {
		t.Errorf("a/b 应各被调用 1 次共 2 次，实际 %d", ok.calls.Load())
	}
}

// TestLongfei_StopsOnError 验证 Longfei 在并行层遇错时终止。
func TestLongfei_StopsOnError(t *testing.T) {
	wf := &model.Workflow{
		ID: "longfei-err", Mode: model.Longfei,
		Nodes: []model.Node{
			{ID: "s", Type: model.NodeStart},
			{ID: "a", Type: model.NodeAction},
			{ID: "b", Type: model.NodeAction},
			{ID: "e", Type: model.NodeEnd},
		},
		Edges: []model.Edge{
			{From: "s", To: "a"},
			{From: "s", To: "b"},
		},
	}
	fail := &failExec{calls: &atomic.Int32{}}
	nodes := map[model.NodeType]port.NodeExecutor{
		model.NodeStart:  &branchExec{id: "s"},
		model.NodeAction: fail,
		model.NodeEnd:    &branchExec{id: "e"},
	}
	ec := &model.ExecutionContext{
		WorkflowID: wf.ID, RunID: "test",
		Variables: map[string]any{}, NodeStates: map[string]model.NodeState{},
	}
	_, err := Longfei(context.Background(), wf, ec, nodes)
	if err == nil {
		t.Fatal("期望 Longfei 报错")
	}
	// 并行层遇错：至少一个失败被捕获；其它可能尚未启动
	if got := fail.calls.Load(); got < 1 || got > 2 {
		t.Errorf("a/b 调用次数应在 1-2 之间，实际 %d", got)
	}
}

// TestNiaoxiang_HappyPath 验证 Niaoxiang 阵扇出后并行执行。
func TestNiaoxiang_HappyPath(t *testing.T) {
	wf := &model.Workflow{
		ID: "niaoxiang-hp", Mode: model.Niaoxiang,
		Nodes: []model.Node{
			{ID: "src", Type: model.NodeStart},
			{ID: "a", Type: model.NodeAction},
			{ID: "b", Type: model.NodeAction},
			{ID: "dst", Type: model.NodeEnd},
		},
		Edges: []model.Edge{
			{From: "src", To: "a"},
			{From: "src", To: "b"},
			{From: "a", To: "dst"},
			{From: "b", To: "dst"},
		},
	}
	ok := &okExec{calls: &atomic.Int32{}}
	nodes := map[model.NodeType]port.NodeExecutor{
		model.NodeStart:  &branchExec{id: "src"},
		model.NodeAction: ok,
		model.NodeEnd:    &branchExec{id: "dst"},
	}
	ec := &model.ExecutionContext{
		WorkflowID: wf.ID, RunID: "test",
		Variables: map[string]any{}, NodeStates: map[string]model.NodeState{},
	}
	_, err := Niaoxiang(context.Background(), wf, ec, nodes)
	if err != nil {
		t.Fatalf("Niaoxiang 失败: %v", err)
	}
	// 源节点（start）用 startExecutor，Action 节点 a/b 用 okExec
	if ok.calls.Load() != 2 {
		t.Errorf("a/b 应各被调用 1 次，实际 %d", ok.calls.Load())
	}
}

// TestNiaoxiang_StopsOnError 验证 Niaoxiang 源节点失败时终止。
func TestNiaoxiang_StopsOnError(t *testing.T) {
	wf := &model.Workflow{
		ID: "niaoxiang-err", Mode: model.Niaoxiang,
		Nodes: []model.Node{
			{ID: "src", Type: model.NodeStart},
			{ID: "a", Type: model.NodeAction},
			{ID: "b", Type: model.NodeAction},
		},
		Edges: []model.Edge{
			{From: "src", To: "a"},
			{From: "src", To: "b"},
		},
	}
	// 让 start 节点也走 failExec（不区分类型）
	fail := &failExec{calls: &atomic.Int32{}}
	nodes := map[model.NodeType]port.NodeExecutor{
		model.NodeStart:  fail,
		model.NodeAction: fail,
		model.NodeEnd:    fail,
	}
	ec := &model.ExecutionContext{
		WorkflowID: wf.ID, RunID: "test",
		Variables: map[string]any{}, NodeStates: map[string]model.NodeState{},
	}
	_, err := Niaoxiang(context.Background(), wf, ec, nodes)
	if err == nil {
		t.Fatal("期望 Niaoxiang 源节点失败时报错")
	}
	if fail.calls.Load() < 1 {
		t.Errorf("源节点应被调用至少 1 次，实际 %d", fail.calls.Load())
	}
}

// TestShepan_LoopNode 验证 Shepan 对 loop 节点按 max_iterations 迭代。
func TestShepan_LoopNode(t *testing.T) {
	wf := &model.Workflow{
		ID: "shepan-loop", Mode: model.Shepan,
		Nodes: []model.Node{
			{ID: "s", Type: model.NodeStart},
			{ID: "loop", Type: model.NodeLoop, Params: map[string]any{"max_iterations": 3}},
			{ID: "e", Type: model.NodeEnd},
		},
		Edges: []model.Edge{
			{From: "s", To: "loop"},
			{From: "loop", To: "e"},
		},
	}
	ok := &okExec{calls: &atomic.Int32{}}
	nodes := map[model.NodeType]port.NodeExecutor{
		model.NodeStart: &branchExec{id: "s"},
		model.NodeLoop:  ok,
		model.NodeEnd:   &branchExec{id: "e"},
	}
	ec := &model.ExecutionContext{
		WorkflowID: wf.ID, RunID: "test",
		Variables: map[string]any{}, NodeStates: map[string]model.NodeState{},
	}
	_, err := Shepan(context.Background(), wf, ec, nodes)
	if err != nil {
		t.Fatalf("Shepan 失败: %v", err)
	}
	if ok.calls.Load() != 3 {
		t.Errorf("loop 应被调用 3 次，实际 %d", ok.calls.Load())
	}
	// 验证 __iter__ 最后一次 = 2
	if v, ok := ec.GetVar("__iter__"); !ok || v != 2 {
		t.Errorf("__iter__ 最后一次应为 2，实际 %v", v)
	}
}

// TestShepan_DefaultIterations 验证 max_iterations 缺失时默认 1。
func TestShepan_DefaultIterations(t *testing.T) {
	wf := &model.Workflow{
		ID: "shepan-default", Mode: model.Shepan,
		Nodes: []model.Node{
			{ID: "s", Type: model.NodeStart},
			{ID: "loop", Type: model.NodeLoop}, // 不传 max_iterations
			{ID: "e", Type: model.NodeEnd},
		},
		Edges: []model.Edge{
			{From: "s", To: "loop"},
			{From: "loop", To: "e"},
		},
	}
	ok := &okExec{calls: &atomic.Int32{}}
	nodes := map[model.NodeType]port.NodeExecutor{
		model.NodeStart: &branchExec{id: "s"},
		model.NodeLoop:  ok,
		model.NodeEnd:   &branchExec{id: "e"},
	}
	ec := &model.ExecutionContext{
		WorkflowID: wf.ID, RunID: "test",
		Variables: map[string]any{}, NodeStates: map[string]model.NodeState{},
	}
	_, err := Shepan(context.Background(), wf, ec, nodes)
	if err != nil {
		t.Fatalf("Shepan 失败: %v", err)
	}
	if ok.calls.Load() != 1 {
		t.Errorf("loop 默认应被调用 1 次，实际 %d", ok.calls.Load())
	}
}

// TestShepan_NonLoopNode 验证 Shepan 对非 loop 节点走 Dizai 顺序。
func TestShepan_NonLoopNode(t *testing.T) {
	wf := &model.Workflow{
		ID: "shepan-nonloop", Mode: model.Shepan,
		Nodes: []model.Node{
			{ID: "s", Type: model.NodeStart},
			{ID: "m", Type: model.NodeAction},
			{ID: "e", Type: model.NodeEnd},
		},
		Edges: []model.Edge{
			{From: "s", To: "m"},
			{From: "m", To: "e"},
		},
	}
	ok := &okExec{calls: &atomic.Int32{}}
	nodes := map[model.NodeType]port.NodeExecutor{
		model.NodeStart:  &branchExec{id: "s"},
		model.NodeAction: ok,
		model.NodeEnd:    &branchExec{id: "e"},
	}
	ec := &model.ExecutionContext{
		WorkflowID: wf.ID, RunID: "test",
		Variables: map[string]any{}, NodeStates: map[string]model.NodeState{},
	}
	_, err := Shepan(context.Background(), wf, ec, nodes)
	if err != nil {
		t.Fatalf("Shepan 失败: %v", err)
	}
	if ok.calls.Load() != 1 {
		t.Errorf("m 应被调用 1 次，实际 %d", ok.calls.Load())
	}
}

// TestTiangai_HappyPath 验证 Tiangai 阵在无错误时的标准并行执行。
func TestTiangai_HappyPath(t *testing.T) {
	wf := makeDiamondTiangaiWF()
	ok := &okExec{calls: &atomic.Int32{}}
	nodes := map[model.NodeType]port.NodeExecutor{
		model.NodeStart:  &branchExec{id: "s"},
		model.NodeAction: ok,
		model.NodeEnd:    &branchExec{id: "e"},
	}
	ec := &model.ExecutionContext{
		WorkflowID: wf.ID, RunID: "test",
		Variables: map[string]any{}, NodeStates: map[string]model.NodeState{},
	}
	_, err := Tiangai(context.Background(), wf, ec, nodes)
	if err != nil {
		t.Fatalf("Tiangai 失败: %v", err)
	}
	if ok.calls.Load() != 3 {
		t.Errorf("a/b/c 应各被调用 1 次共 3 次，实际 %d", ok.calls.Load())
	}
}

// TestTiangai_StopsOnError 验证 Tiangai 在并行层遇错时返回首个错误。
func TestTiangai_StopsOnError(t *testing.T) {
	wf := makeDiamondTiangaiWF()
	// 一个会失败的 exec（用 hybrid：a 失败、b/c 成功）
	hybrid := &tErrOn{}
	ok := &okExec{calls: &atomic.Int32{}}
	nodes := map[model.NodeType]port.NodeExecutor{
		model.NodeStart:  &branchExec{id: "s"},
		model.NodeAction: hybrid,
		model.NodeEnd:    &branchExec{id: "e"},
	}
	_ = ok
	ec := &model.ExecutionContext{
		WorkflowID: wf.ID, RunID: "test",
		Variables: map[string]any{}, NodeStates: map[string]model.NodeState{},
	}
	_, err := Tiangai(context.Background(), wf, ec, nodes)
	if err == nil {
		t.Fatal("期望 Tiangai 报错")
	}
}

// tErrOn 测试辅助：a 失败，其它节点成功。
type tErrOn struct{}

func (t *tErrOn) Execute(_ context.Context, n model.Node, _ *model.ExecutionContext) (*model.NodeState, error) {
	if n.ID == "a" {
		return nil, errors.New("a fails")
	}
	return &model.NodeState{ID: n.ID, Status: model.NodeStatusOK}, nil
}

// slowExec 慢节点：用于验证超时。
type slowExec struct{}

func (s *slowExec) Execute(ctx context.Context, n model.Node, _ *model.ExecutionContext) (*model.NodeState, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-make(chan struct{}):
		return &model.NodeState{ID: n.ID, Status: model.NodeStatusOK}, nil
	}
}

// TestDizai_HappyPath 验证 Dizai 阵无错误时正常顺序执行。
func TestDizai_HappyPath(t *testing.T) {
	wf := makeLinearDizaiWF()
	ok := &okExec{calls: &atomic.Int32{}}
	nodes := map[model.NodeType]port.NodeExecutor{
		model.NodeStart:  &branchExec{id: "s"},
		model.NodeAction: ok,
		model.NodeEnd:    &branchExec{id: "e"},
	}
	ec := &model.ExecutionContext{
		WorkflowID: wf.ID, RunID: "test",
		Variables: map[string]any{}, NodeStates: map[string]model.NodeState{},
	}
	_, err := Dizai(context.Background(), wf, ec, nodes)
	if err != nil {
		t.Fatalf("Dizai 失败: %v", err)
	}
	if ok.calls.Load() != 2 {
		t.Errorf("m1/m2 应各被调用 1 次共 2 次，实际 %d", ok.calls.Load())
	}
}

// TestHuyi_NoIncomingSkip 验证 Huyi 在没有入边 Condition 命中时跳过。
func TestHuyi_NoIncomingSkip(t *testing.T) {
	wf := &model.Workflow{
		ID: "huyi-skip", Mode: model.Huyi,
		Nodes: []model.Node{
			{ID: "s", Type: model.NodeStart},
			{ID: "m1", Type: model.NodeAction},
			{ID: "m2", Type: model.NodeAction},
			{ID: "e", Type: model.NodeEnd},
		},
		Edges: []model.Edge{
			{From: "s", To: "m1"},
			{From: "m1", To: "m2", Condition: `var.x=="yes"`}, // x 不存在 ⇒ false
		},
	}
	calls := &atomic.Int32{}
	nodes := map[model.NodeType]port.NodeExecutor{
		model.NodeStart:  &branchExec{id: "s"},
		model.NodeAction: &branchExec{id: "any", calls: calls},
		model.NodeEnd:    &branchExec{id: "e"},
	}
	ec := &model.ExecutionContext{
		WorkflowID: wf.ID, RunID: "test",
		Variables: map[string]any{}, NodeStates: map[string]model.NodeState{},
	}
	_, err := Huyi(context.Background(), wf, ec, nodes)
	if err != nil {
		t.Fatalf("Huyi 失败: %v", err)
	}
	// m2 应被标记为 Skipped
	if s, ok := ec.NodeStates["m2"]; !ok || s.Status != model.NodeStatusSkipped {
		t.Errorf("m2 状态应为 Skipped，实际 %+v", s)
	}
}

// TestHuyi_LiteralTrue 验证 Huyi 条件字面量 true。
func TestHuyi_LiteralTrue(t *testing.T) {
	wf := &model.Workflow{
		ID: "huyi-true", Mode: model.Huyi,
		Nodes: []model.Node{
			{ID: "s", Type: model.NodeStart},
			{ID: "m1", Type: model.NodeAction},
			{ID: "e", Type: model.NodeEnd},
		},
		Edges: []model.Edge{
			{From: "s", To: "m1"},
			{From: "m1", To: "e", Condition: "true"},
		},
	}
	nodes := map[model.NodeType]port.NodeExecutor{
		model.NodeStart:  &branchExec{id: "s"},
		model.NodeAction: &branchExec{id: "m1"},
		model.NodeEnd:    &branchExec{id: "e"},
	}
	ec := &model.ExecutionContext{
		WorkflowID: wf.ID, RunID: "test",
		Variables: map[string]any{}, NodeStates: map[string]model.NodeState{},
	}
	_, err := Huyi(context.Background(), wf, ec, nodes)
	if err != nil {
		t.Fatalf("Huyi 失败: %v", err)
	}
	if _, ok := ec.NodeStates["e"]; !ok {
		t.Error("e 节点应有 NodeState（条件 true 命中）")
	}
}

// TestNiaoxiang_NoDownstream 验证 Niaoxiang 在源节点无下游时直接成功。
func TestNiaoxiang_NoDownstream(t *testing.T) {
	wf := &model.Workflow{
		ID: "niaoxiang-no", Mode: model.Niaoxiang,
		Nodes: []model.Node{
			{ID: "src", Type: model.NodeStart},
			{ID: "e", Type: model.NodeEnd},
		},
		Edges: []model.Edge{}, // src 无下游
	}
	nodes := map[model.NodeType]port.NodeExecutor{
		model.NodeStart: &branchExec{id: "src"},
		model.NodeEnd:   &branchExec{id: "e"},
	}
	ec := &model.ExecutionContext{
		WorkflowID: wf.ID, RunID: "test",
		Variables: map[string]any{}, NodeStates: map[string]model.NodeState{},
	}
	_, err := Niaoxiang(context.Background(), wf, ec, nodes)
	if err != nil {
		t.Fatalf("Niaoxiang 失败: %v", err)
	}
	if _, ok := ec.NodeStates["src"]; !ok {
		t.Error("src 应有 NodeState")
	}
}

// TestShepan_LoopFails 验证 Shepan 循环体失败时立即返回错误。
func TestShepan_LoopFails(t *testing.T) {
	wf := &model.Workflow{
		ID: "shepan-fail", Mode: model.Shepan,
		Nodes: []model.Node{
			{ID: "s", Type: model.NodeStart},
			{ID: "loop", Type: model.NodeLoop, Params: map[string]any{"max_iterations": 5}},
			{ID: "e", Type: model.NodeEnd},
		},
		Edges: []model.Edge{
			{From: "s", To: "loop"},
			{From: "loop", To: "e"},
		},
	}
	fail := &failExec{calls: &atomic.Int32{}}
	nodes := map[model.NodeType]port.NodeExecutor{
		model.NodeStart: &branchExec{id: "s"},
		model.NodeLoop:  fail,
		model.NodeEnd:   &branchExec{id: "e"},
	}
	ec := &model.ExecutionContext{
		WorkflowID: wf.ID, RunID: "test",
		Variables: map[string]any{}, NodeStates: map[string]model.NodeState{},
	}
	_, err := Shepan(context.Background(), wf, ec, nodes)
	if err == nil {
		t.Fatal("期望 Shepan 循环失败时报错")
	}
	if fail.calls.Load() != 1 {
		t.Errorf("loop 应被调用 1 次后立即失败，实际 %d", fail.calls.Load())
	}
}
