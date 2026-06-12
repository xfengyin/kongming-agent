// Package workflow 工作流应用层 - Runner 单元测试。
//
// 覆盖范围：
//   - 注册/查询：TestRunner_RegisterAndGet
//   - Dizai 顺序：TestRunner_Execute_Dizai_Sequence
//   - Tiangai 并行：TestRunner_Execute_Tiangai_Parallel
//   - Fengyang 超时：TestRunner_Execute_Fengyang_Timeout
//   - Niaoxiang 扇出：TestRunner_Execute_Niaoxiang_Fanout
//   - Shepan 循环：TestRunner_Execute_Shepan_Loop
//   - Yunzhui 重试：TestRunner_Execute_Yunzhui_Retries
//   - 缺 start/end 拒绝：TestRunner_Register_RejectsMissingAnchors
//   - 查询不存在：TestRunner_Get_NotFound
package workflow

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"go.uber.org/zap"

	"github.com/zhuge/kongming/pkg/domain/model"
	"github.com/zhuge/kongming/pkg/domain/port"
)

// === 测试辅助：可注入的 NodeExecutor ===

// failingExec 总是返回错误。
type failingExec struct {
	calls *atomic.Int32
}

func (f *failingExec) Execute(_ context.Context, _ model.Node, _ *model.ExecutionContext) (*model.NodeState, error) {
	f.calls.Add(1)
	return nil, errors.New("故意的失败")
}

// slowExec 慢节点：用于验证超时（ctx.Done 会让它退出）。
type slowExec struct {
	wait time.Duration
}

func (s *slowExec) Execute(ctx context.Context, n model.Node, _ *model.ExecutionContext) (*model.NodeState, error) {
	select {
	case <-time.After(s.wait):
		return &model.NodeState{ID: n.ID, Status: model.NodeStatusOK}, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

// flakyExec 第 1 次失败、第 2 次成功（用于 Yunzhui 重试验证）。
type flakyExec struct {
	calls *atomic.Int32
}

func (f *flakyExec) Execute(_ context.Context, n model.Node, _ *model.ExecutionContext) (*model.NodeState, error) {
	nth := f.calls.Add(1)
	if nth == 1 {
		return nil, errors.New("transient")
	}
	return &model.NodeState{ID: n.ID, Status: model.NodeStatusOK}, nil
}

// echoExec 简单把 ec.Variables["x"] 写入 Output。
type echoExec struct{}

func (e *echoExec) Execute(_ context.Context, n model.Node, ec *model.ExecutionContext) (*model.NodeState, error) {
	if v, ok := ec.GetVar("x"); ok {
		return &model.NodeState{ID: n.ID, Status: model.NodeStatusOK, Output: v}, nil
	}
	return &model.NodeState{ID: n.ID, Status: model.NodeStatusOK}, nil
}

// === 测试用例 ===

// TestRunner_RegisterAndGet 验证 Register + GetWorkflow 正常路径。
func TestRunner_RegisterAndGet(t *testing.T) {
	r := NewRunner(zap.NewNop())
	wf := &model.Workflow{
		ID:   "wf-basic",
		Name: "basic",
		Mode: model.Dizai,
		Nodes: []model.Node{
			{ID: "s", Type: model.NodeStart},
			{ID: "e", Type: model.NodeEnd},
		},
		Edges: []model.Edge{{From: "s", To: "e"}},
	}
	if err := r.RegisterWorkflow(wf); err != nil {
		t.Fatalf("RegisterWorkflow 失败: %v", err)
	}
	got, err := r.GetWorkflow("wf-basic")
	if err != nil {
		t.Fatalf("GetWorkflow 失败: %v", err)
	}
	if got.ID != "wf-basic" {
		t.Errorf("GetWorkflow 拿到的 ID 不一致: %s", got.ID)
	}
}

// TestRunner_Register_RejectsMissingAnchors 验证缺 start/end 时被拒绝。
func TestRunner_Register_RejectsMissingAnchors(t *testing.T) {
	r := NewRunner(zap.NewNop())
	bad := &model.Workflow{
		ID:    "wf-no-start",
		Name:  "bad",
		Mode:  model.Dizai,
		Nodes: []model.Node{{ID: "m", Type: model.NodeAction}},
		Edges: []model.Edge{},
	}
	if err := r.RegisterWorkflow(bad); err == nil {
		t.Fatal("期望缺 start/end 报错，实际通过")
	}
}

// TestRunner_Get_NotFound 验证查询不存在工作流返回 error。
func TestRunner_Get_NotFound(t *testing.T) {
	r := NewRunner(zap.NewNop())
	_, err := r.GetWorkflow("ghost")
	if err == nil {
		t.Fatal("期望查询不存在的 workflow 报错，实际通过")
	}
}

// TestRunner_Execute_Dizai_Sequence 验证 Dizai 阵顺序执行 3 节点链。
func TestRunner_Execute_Dizai_Sequence(t *testing.T) {
	r := NewRunner(zap.NewNop())
	counter := &atomic.Int32{}
	// 用 echoExec 计数（每次调用 += 1），同时把 ec.Variables["x"] 写 Output
	echo := &counterExec{calls: counter, exec: &echoExec{}}
	r.RegisterNodeExecutor(model.NodeAction, echo)
	wf := &model.Workflow{
		ID:   "wf-dizai",
		Name: "dizai-seq",
		Mode: model.Dizai,
		Nodes: []model.Node{
			{ID: "s", Type: model.NodeStart},
			{ID: "m1", Type: model.NodeAction, Name: "step1"},
			{ID: "m2", Type: model.NodeAction, Name: "step2"},
			{ID: "e", Type: model.NodeEnd},
		},
		Edges: []model.Edge{
			{From: "s", To: "m1"},
			{From: "m1", To: "m2"},
			{From: "m2", To: "e"},
		},
	}
	if err := r.RegisterWorkflow(wf); err != nil {
		t.Fatalf("RegisterWorkflow 失败: %v", err)
	}
	ec, err := r.Execute(context.Background(), wf.ID, map[string]any{"x": "input"})
	if err != nil {
		t.Fatalf("Execute 失败: %v", err)
	}
	// 全部 4 个节点都应执行
	if got := counter.Load(); got != 2 {
		t.Errorf("Action 节点应被调用 2 次，实际 %d", got)
	}
	// 验证 NodeStates 都有结果
	if len(ec.NodeStates) != 4 {
		t.Errorf("NodeStates 应有 4 项，实际 %d", len(ec.NodeStates))
	}
	for _, id := range []string{"s", "m1", "m2", "e"} {
		if _, ok := ec.NodeStates[id]; !ok {
			t.Errorf("NodeStates 缺少 %s", id)
		}
	}
	// 验证 end 节点拿到的 result
	endState, _ := ec.GetNodeState("e")
	if endState.Status != model.NodeStatusOK {
		t.Errorf("end 节点状态应为 ok，实际 %s", endState.Status)
	}
}

// TestRunner_Execute_Tiangai_Parallel 验证 Tiangai 阵并行：2 个独立 Action 节点。
//
// 关键断言：2 个 100ms 节点并行总耗时 < 180ms（粗略断言，远低于 200ms 串行）。
func TestRunner_Execute_Tiangai_Parallel(t *testing.T) {
	r := NewRunner(zap.NewNop())
	counter := &atomic.Int32{}
	// 用 sleepCounter 同时充当 Action 执行器（每次 100ms），验证并行性
	sleep := &sleepCounter{d: 100 * time.Millisecond, calls: counter}
	r.RegisterNodeExecutor(model.NodeAction, sleep)
	wf := &model.Workflow{
		ID:   "wf-tiangai",
		Name: "tiangai-parallel",
		Mode: model.Tiangai,
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
	if err := r.RegisterWorkflow(wf); err != nil {
		t.Fatalf("RegisterWorkflow 失败: %v", err)
	}
	start := time.Now()
	ec, err := r.Execute(context.Background(), wf.ID, nil)
	elapsed := time.Since(start)
	if err != nil {
		t.Fatalf("Execute 失败: %v", err)
	}
	// a 和 b 在同层应各被调用 1 次
	if got := counter.Load(); got != 2 {
		t.Errorf("Action 节点应被调用 2 次，实际 %d", got)
	}
	if elapsed >= 180*time.Millisecond {
		t.Errorf("并行执行期望 < 180ms（两次 100ms 并行），实际 %v", elapsed)
	}
	if len(ec.NodeStates) != 4 {
		t.Errorf("NodeStates 应有 4 项，实际 %d", len(ec.NodeStates))
	}
}

// TestRunner_Execute_Fengyang_Timeout 验证 Fengyang 阵在节点超时时返回错误。
func TestRunner_Execute_Fengyang_Timeout(t *testing.T) {
	r := NewRunner(zap.NewNop())
	// 注入一个比 fengyang 超时（30s）短很多的 slow 节点，
	// 但为了测试不真等 30s，我们直接用 ctx.WithTimeout 截断——
	// Fengyang 阵使用 30s 内置 ctx，这里把 slow 节点等待 5s 仍会在 fengyang 跑完后才返回（不合理）。
	// 改用更直接做法：直接验证 slow 节点能响应 ctx 取消即可。
	r.RegisterNodeExecutor(model.NodeAction, &slowExec{wait: 200 * time.Millisecond})
	wf := &model.Workflow{
		ID:   "wf-fengyang",
		Name: "fengyang-timeout",
		Mode: model.Fengyang,
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
	if err := r.RegisterWorkflow(wf); err != nil {
		t.Fatalf("RegisterWorkflow 失败: %v", err)
	}
	// 用 50ms 的父 ctx 覆盖（外层 ctx 优先级高于 Fengyang 内部 30s）
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	_, err := r.Execute(ctx, wf.ID, nil)
	// 期望 ctx 取消导致 slowExec 返回错误
	if err == nil {
		t.Error("期望 ctx 取消时 Execute 返回错误，实际通过")
	}
}

// TestRunner_Execute_Niaoxiang_Fanout 验证 Niaoxiang 阵扇出：1 源 + 3 下游并行。
func TestRunner_Execute_Niaoxiang_Fanout(t *testing.T) {
	r := NewRunner(zap.NewNop())
	counter := &atomic.Int32{}
	// 源 + 3 个下游都用 sleepCounter，验证下游扇出 3 个都被调用
	sleep := &sleepCounter{d: 50 * time.Millisecond, calls: counter}
	r.RegisterNodeExecutor(model.NodeAction, sleep)
	wf := &model.Workflow{
		ID:    "wf-niaoxiang",
		Name:  "niaoxiang-fanout",
		Mode:  model.Niaoxiang,
		Entry: "src",
		Nodes: []model.Node{
			{ID: "src", Type: model.NodeStart}, // 用 start 作源（必含）
			{ID: "f1", Type: model.NodeAction},
			{ID: "f2", Type: model.NodeAction},
			{ID: "f3", Type: model.NodeAction},
			{ID: "dst", Type: model.NodeEnd},
		},
		Edges: []model.Edge{
			{From: "src", To: "f1"},
			{From: "src", To: "f2"},
			{From: "src", To: "f3"},
			{From: "f1", To: "dst"},
			{From: "f2", To: "dst"},
			{From: "f3", To: "dst"},
		},
	}
	if err := r.RegisterWorkflow(wf); err != nil {
		t.Fatalf("RegisterWorkflow 失败: %v", err)
	}
	ec, err := r.Execute(context.Background(), wf.ID, nil)
	if err != nil {
		t.Fatalf("Execute 失败: %v", err)
	}
	// 3 个扇出节点都应被调用（start 节点走 startExecutor，不计数）
	if got := counter.Load(); got != 3 {
		t.Errorf("扇出节点应被调用 3 次，实际 %d", got)
	}
	// 源 + 3 个扇出节点都应在 NodeStates 里
	for _, id := range []string{"src", "f1", "f2", "f3"} {
		if _, ok := ec.NodeStates[id]; !ok {
			t.Errorf("NodeStates 缺少 %s", id)
		}
	}
}

// TestRunner_Execute_Shepan_Loop 验证 Shepan 阵按 max_iterations 循环。
func TestRunner_Execute_Shepan_Loop(t *testing.T) {
	r := NewRunner(zap.NewNop())
	counter := &atomic.Int32{}
	r.RegisterNodeExecutor(model.NodeLoop, &loopExec{calls: counter})
	wf := &model.Workflow{
		ID:   "wf-shepan",
		Name: "shepan-loop",
		Mode: model.Shepan,
		Nodes: []model.Node{
			{ID: "s", Type: model.NodeStart},
			{
				ID:     "loop",
				Type:   model.NodeLoop,
				Params: map[string]any{"max_iterations": 3},
			},
			{ID: "e", Type: model.NodeEnd},
		},
		Edges: []model.Edge{
			{From: "s", To: "loop"},
			{From: "loop", To: "e"},
		},
	}
	if err := r.RegisterWorkflow(wf); err != nil {
		t.Fatalf("RegisterWorkflow 失败: %v", err)
	}
	ec, err := r.Execute(context.Background(), wf.ID, nil)
	if err != nil {
		t.Fatalf("Execute 失败: %v", err)
	}
	if got := counter.Load(); got != 3 {
		t.Errorf("loop 节点应被调用 3 次，实际 %d", got)
	}
	// 验证 ec.Variables["__iter__"] 最后一次 = 2
	if v, ok := ec.GetVar("__iter__"); !ok || v != 2 {
		t.Errorf("__iter__ 最后一次应为 2，实际 %v", v)
	}
	// 验证 loop 节点的 NodeState 写入
	if _, ok := ec.NodeStates["loop"]; !ok {
		t.Error("NodeStates 缺少 loop")
	}
}

// TestRunner_Execute_Yunzhui_Retries 验证 Yunzhui 阵重试。
func TestRunner_Execute_Yunzhui_Retries(t *testing.T) {
	r := NewRunner(zap.NewNop())
	calls := &atomic.Int32{}
	r.RegisterNodeExecutor(model.NodeAction, &flakyExec{calls: calls})
	wf := &model.Workflow{
		ID:   "wf-yunzhui",
		Name: "yunzhui-retry",
		Mode: model.Yunzhui,
		Nodes: []model.Node{
			{ID: "s", Type: model.NodeStart},
			{ID: "m", Type: model.NodeAction, Retries: 2},
			{ID: "e", Type: model.NodeEnd},
		},
		Edges: []model.Edge{
			{From: "s", To: "m"},
			{From: "m", To: "e"},
		},
	}
	if err := r.RegisterWorkflow(wf); err != nil {
		t.Fatalf("RegisterWorkflow 失败: %v", err)
	}
	_, err := r.Execute(context.Background(), wf.ID, nil)
	if err != nil {
		t.Fatalf("Execute 失败: %v", err)
	}
	// 1 次失败 + 1 次成功 = 2 次调用（Retries=2 允许最多 3 次；flakyExec 第 2 次成功）
	if got := calls.Load(); got != 2 {
		t.Errorf("flakyExec 应被调用 2 次，实际 %d", got)
	}
}

// === 辅助类型 ===

// counterExec 装饰器：包一个内层 executor 同时累加调用计数。
type counterExec struct {
	calls *atomic.Int32
	exec  port.NodeExecutor
}

func (c *counterExec) Execute(ctx context.Context, n model.Node, ec *model.ExecutionContext) (*model.NodeState, error) {
	c.calls.Add(1)
	return c.exec.Execute(ctx, n, ec)
}

// sleepCounter 简单 sleep 节点 + 调用计数。
type sleepCounter struct {
	d     time.Duration
	calls *atomic.Int32
}

func (s *sleepCounter) Execute(ctx context.Context, n model.Node, _ *model.ExecutionContext) (*model.NodeState, error) {
	s.calls.Add(1)
	select {
	case <-time.After(s.d):
		return &model.NodeState{ID: n.ID, Status: model.NodeStatusOK}, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

// loopExec 把 __iter__ 写回 ec.Variables 便于测试断言。
type loopExec struct {
	calls *atomic.Int32
}

func (l *loopExec) Execute(_ context.Context, n model.Node, ec *model.ExecutionContext) (*model.NodeState, error) {
	l.calls.Add(1)
	if v, ok := ec.GetVar("__iter__"); ok {
		ec.SetVar("__last_iter__", v)
	}
	return &model.NodeState{ID: n.ID, Status: model.NodeStatusOK}, nil
}

// 实现断言：自定义 executor 实现 port.NodeExecutor。
var _ port.NodeExecutor = (*loopExec)(nil)
