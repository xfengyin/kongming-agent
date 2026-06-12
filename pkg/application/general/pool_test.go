// Package general 应用层 - 将领池（Pool）单元测试。
//
// 覆盖：
//  1. 基本 CRUD：Register/Get/List/Unregister
//  2. 选将：SelectBest 按 SuccessCount 降序
//  3. 执行：Execute 通过注入的 Executor 拿到 GeneralReport
//  4. 并发：-race 下 10 goroutine 并发 Register 无数据竞争
package general

import (
	"context"
	stderrors "errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"

	"github.com/zhuge/kongming/pkg/domain/model"
	"github.com/zhuge/kongming/pkg/domain/port"
)

// fakeObserver 是一个无副作用的 port.Observer 实现，避免在单测中拉起
// prom registry / OTLP exporter 等重组件。
// 同时记录 IncCounter / ObserveHistogram / SetGauge 调用次数用于断言。
type fakeObserver struct {
	counterCalls atomic.Int32
	histCalls    atomic.Int32
	gaugeCalls   atomic.Int32
}

func (f *fakeObserver) StartSpan(ctx context.Context, _ string, _ ...attribute.KeyValue) (context.Context, trace.Span) {
	// 返回 ctx 与一个 noop span；测试不需要真实链路。
	return ctx, trace.SpanFromContext(ctx)
}
func (f *fakeObserver) RecordError(_ trace.Span, _ error) {}
func (f *fakeObserver) RecordEvent(_ context.Context, _ string, _ ...attribute.KeyValue) {
}
func (f *fakeObserver) IncCounter(_ string, _ map[string]string) { f.counterCalls.Add(1) }
func (f *fakeObserver) ObserveHistogram(_ string, _ float64, _ map[string]string) {
	f.histCalls.Add(1)
}
func (f *fakeObserver) SetGauge(_ string, _ float64, _ map[string]string) {
	f.gaugeCalls.Add(1)
}
func (f *fakeObserver) Shutdown(_ context.Context) error { return nil }

// nopResilient 是一个 noop 的 ResilientRunner，直接调用 fn，不做重试/熔断/限流。
// 测试用它简化 Execute 行为。
type nopResilient struct{}

func (nopResilient) Run(ctx context.Context, _ string, fn func(ctx context.Context) error) error {
	return fn(ctx)
}
func (nopResilient) RunWithResult(ctx context.Context, _ string, fn func(ctx context.Context) (any, error)) (any, error) {
	return fn(ctx)
}

// mockExecutor 用于测试 Execute 路径。
type mockExecutor struct {
	resp  *model.GeneralReport
	err   error
	calls atomic.Int32
}

func (m *mockExecutor) Execute(_ context.Context, g *model.General, _ *model.Order) (*model.GeneralReport, error) {
	m.calls.Add(1)
	if m.resp != nil {
		// 拷贝模板并填上 ID/Name，使 report 与 general 关联（真实 Executor 的行为）。
		r := *m.resp
		r.GeneralID = g.ID
		r.Name = g.Name
		return &r, m.err
	}
	return &model.GeneralReport{
		GeneralID: g.ID,
		Name:      g.Name,
		Success:   m.err == nil,
	}, m.err
}

// makeGeneral 构造一个用于测试的 *model.General，简化 fixture。
func makeGeneral(id model.GeneralID, skills []string, success int) *model.General {
	return &model.General{
		ID:     id,
		Name:   string(id),
		Type:   model.GeneralType(id),
		Skills: skills,
		Stats: model.GeneralStats{
			TotalMissions: success,
			SuccessCount:  success,
		},
		CreatedAt: time.Now(),
	}
}

// newTestPool 构造一个便于测试的 *Pool，使用 fake observer + noop resilient + nil executor。
func newTestPool(t *testing.T) (*Pool, *fakeObserver) {
	t.Helper()
	obs := &fakeObserver{}
	p := NewPool(zap.NewNop(), obs, nopResilient{}, nil)
	return p, obs
}

// TestPool_Register_Get 验证 Register 后能通过 Get 拿到同一对象。
func TestPool_Register_Get(t *testing.T) {
	p, _ := newTestPool(t)
	ctx := context.Background()

	g := makeGeneral("guanyu", []string{"data_collection"}, 0)
	if err := p.Register(ctx, g); err != nil {
		t.Fatalf("register: %v", err)
	}

	got, err := p.Get(ctx, "guanyu")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.ID != g.ID {
		t.Errorf("id: want %q, got %q", g.ID, got.ID)
	}
	if got.Name != g.Name {
		t.Errorf("name: want %q, got %q", g.Name, got.Name)
	}
}

// TestPool_List 验证注册 3 个后 List 返回 3 个。
func TestPool_List(t *testing.T) {
	p, _ := newTestPool(t)
	ctx := context.Background()

	for _, id := range []model.GeneralID{"guanyu", "zhangfei", "zhaoyun"} {
		if err := p.Register(ctx, makeGeneral(id, []string{"x"}, 0)); err != nil {
			t.Fatalf("register %s: %v", id, err)
		}
	}

	got, err := p.List(ctx)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(got) != 3 {
		t.Errorf("list len: want 3, got %d", len(got))
	}
}

// TestPool_Unregister 验证 Unregister 后 Get 返回错误；重复 Unregister 幂等。
func TestPool_Unregister(t *testing.T) {
	p, _ := newTestPool(t)
	ctx := context.Background()

	if err := p.Register(ctx, makeGeneral("machao", []string{"writing"}, 0)); err != nil {
		t.Fatalf("register: %v", err)
	}
	if err := p.Unregister(ctx, "machao"); err != nil {
		t.Fatalf("unregister: %v", err)
	}

	if _, err := p.Get(ctx, "machao"); err == nil {
		t.Error("get after unregister: want error, got nil")
	}
	// 重复 Unregister 幂等，不应报错。
	if err := p.Unregister(ctx, "machao"); err != nil {
		t.Errorf("re-unregister: want nil, got %v", err)
	}
}

// TestPool_SelectBest 验证 SelectBest 按 Stats.SuccessCount 降序选第一。
func TestPool_SelectBest(t *testing.T) {
	p, _ := newTestPool(t)
	ctx := context.Background()

	// 两位都具备 "data_collection" skill；huangzhong 成功数更高，应被选中。
	if err := p.Register(ctx, makeGeneral("guanyu", []string{"data_collection"}, 3)); err != nil {
		t.Fatalf("register guanyu: %v", err)
	}
	if err := p.Register(ctx, makeGeneral("huangzhong", []string{"data_collection"}, 9)); err != nil {
		t.Fatalf("register huangzhong: %v", err)
	}

	best, err := p.SelectBest("data_collection")
	if err != nil {
		t.Fatalf("select best: %v", err)
	}
	if best.ID != "huangzhong" {
		t.Errorf("best: want huangzhong, got %s", best.ID)
	}

	// 选不存在的 skill，应返回错误。
	if _, err := p.SelectBest("nonexistent_skill"); err == nil {
		t.Error("select best unknown: want error, got nil")
	}
}

// TestPool_Execute_Success 验证 Execute 通过 mock executor 返回 Success=true 的 GeneralReport。
func TestPool_Execute_Success(t *testing.T) {
	exec := &mockExecutor{
		resp: &model.GeneralReport{
			Success: true,
			Output:  "ok",
		},
	}
	p := NewPool(zap.NewNop(), &fakeObserver{}, nopResilient{}, exec)
	ctx := context.Background()

	if err := p.Register(ctx, makeGeneral("zhaoyun", []string{"analysis"}, 0)); err != nil {
		t.Fatalf("register: %v", err)
	}

	order := &model.Order{ID: "o-1", Name: "test"}
	report, err := p.Execute(ctx, "zhaoyun", order)
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if report == nil {
		t.Fatal("report: want non-nil, got nil")
	}
	if !report.Success {
		t.Errorf("success: want true, got false (error=%s)", report.Error)
	}
	if report.GeneralID != "zhaoyun" {
		t.Errorf("general id: want zhaoyun, got %s", report.GeneralID)
	}
	if report.Duration <= 0 {
		t.Errorf("duration: want > 0, got %f", report.Duration)
	}
	if exec.calls.Load() != 1 {
		t.Errorf("executor calls: want 1, got %d", exec.calls.Load())
	}
}

// TestPool_Execute_ExecutorError 验证 Executor 报错时 Pool 把错误透传并填充 report.Error。
func TestPool_Execute_ExecutorError(t *testing.T) {
	wantErr := stderrors.New("executor boom")
	exec := &mockExecutor{err: wantErr}
	p := NewPool(zap.NewNop(), &fakeObserver{}, nopResilient{}, exec)
	ctx := context.Background()

	if err := p.Register(ctx, makeGeneral("machao", []string{"writing"}, 0)); err != nil {
		t.Fatalf("register: %v", err)
	}

	report, err := p.Execute(ctx, "machao", &model.Order{ID: "o-1"})
	if err == nil {
		t.Fatal("execute: want error, got nil")
	}
	if !stderrors.Is(err, wantErr) {
		t.Errorf("error: want %v, got %v", wantErr, err)
	}
	if report == nil {
		t.Fatal("report: want non-nil even on error, got nil")
	}
	if report.Success {
		t.Error("success: want false on error")
	}
	if report.Error == "" {
		t.Error("error string: want non-empty")
	}
}

// TestPool_Execute_GeneralNotFound 验证 Execute 在将领不存在时返回错误且不调 Executor。
func TestPool_Execute_GeneralNotFound(t *testing.T) {
	exec := &mockExecutor{}
	p := NewPool(zap.NewNop(), &fakeObserver{}, nopResilient{}, exec)
	ctx := context.Background()

	_, err := p.Execute(ctx, "ghost", &model.Order{ID: "o-1"})
	if err == nil {
		t.Fatal("execute unknown general: want error, got nil")
	}
	if exec.calls.Load() != 0 {
		t.Errorf("executor should not be called, got %d calls", exec.calls.Load())
	}
}

// TestPool_Concurrent_Register 验证 10 goroutine 并发 Register 无 race。
func TestPool_Concurrent_Register(t *testing.T) {
	p, _ := newTestPool(t)
	ctx := context.Background()

	var wg sync.WaitGroup
	const n = 10
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			id := model.GeneralID(string(rune('a' + i)))
			_ = p.Register(ctx, makeGeneral(id, []string{"x"}, 0))
		}(i)
	}
	wg.Wait()

	got, err := p.List(ctx)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(got) != n {
		t.Errorf("list len after concurrent register: want %d, got %d", n, len(got))
	}
}

// TestPool_Register_IncCounter 验证 Register 上报 general_registered 计数。
func TestPool_Register_IncCounter(t *testing.T) {
	obs := &fakeObserver{}
	p := NewPool(zap.NewNop(), obs, nopResilient{}, nil)
	ctx := context.Background()

	for i := 0; i < 3; i++ {
		id := model.GeneralID(string(rune('a' + i)))
		if err := p.Register(ctx, makeGeneral(id, []string{"x"}, 0)); err != nil {
			t.Fatalf("register: %v", err)
		}
	}
	if got := obs.counterCalls.Load(); got != 3 {
		t.Errorf("counter calls: want 3, got %d", got)
	}
}

// TestPool_Execute_ObserveHistogram 验证 Execute 上报 Duration 直方图。
func TestPool_Execute_ObserveHistogram(t *testing.T) {
	exec := &mockExecutor{resp: &model.GeneralReport{Success: true}}
	obs := &fakeObserver{}
	p := NewPool(zap.NewNop(), obs, nopResilient{}, exec)
	ctx := context.Background()

	if err := p.Register(ctx, makeGeneral("guanyu", []string{"x"}, 0)); err != nil {
		t.Fatalf("register: %v", err)
	}
	if _, err := p.Execute(ctx, "guanyu", &model.Order{ID: "o-1"}); err != nil {
		t.Fatalf("execute: %v", err)
	}
	if got := obs.histCalls.Load(); got != 1 {
		t.Errorf("histogram calls: want 1, got %d", got)
	}
}

// 编译期断言：Pool 必须实现 port.GeneralPool。
// 这里额外做运行期 sanity check，避免接口漂移。
func TestPool_ImplementsPort(t *testing.T) {
	var _ port.GeneralPool = (*Pool)(nil)
}
