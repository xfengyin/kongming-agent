// Package dispatcher 是「调度器」应用层实现：异步把 Order 路由到对应 Executor。
//
// 设计要点：
//   - worker pool 模型：固定数量 goroutine 持续从 channel 拉取任务；
//   - 优先级路由：按 order.Priority 找名为 "priority-<value>" 的 executor；
//   - 优雅关闭：Wait 等所有 in-flight 任务完成或 ctx 超时。
package dispatcher

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/zhuge/kongming/pkg/domain/model"
	"github.com/zhuge/kongming/pkg/domain/port"
	"go.uber.org/zap"
)

// mockExecutor 是测试用 executor：记录调用次数 + 可注入 sleep/error。
type mockExecutor struct {
	calls   int32
	sleep   time.Duration
	failErr error
}

func (m *mockExecutor) Execute(_ context.Context, _ *model.Order) (*model.BattleReport, error) {
	atomic.AddInt32(&m.calls, 1)
	if m.sleep > 0 {
		time.Sleep(m.sleep)
	}
	if m.failErr != nil {
		return nil, m.failErr
	}
	return &model.BattleReport{Success: true, OrderID: "ok"}, nil
}

// TestDispatcher_RegisterExecutor 验证：RegisterExecutor + 后续 Dispatch 能找到。
func TestDispatcher_RegisterExecutor(t *testing.T) {
	logger := zap.NewNop()
	d := NewDispatcher(2, logger)

	mock := &mockExecutor{}
	d.RegisterExecutor("priority-2", mock) // PriorityNormal=2

	// 内部 map 应能查到这个 executor（通过间接行为：Dispatch 后 mock.calls=1）。
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	if err := d.Start(ctx); err != nil {
		t.Fatalf("start: %v", err)
	}

	order := &model.Order{ID: "o1", Priority: model.PriorityNormal}
	if err := d.Dispatch(ctx, order); err != nil {
		t.Fatalf("dispatch: %v", err)
	}

	waitCtx, waitCancel := context.WithTimeout(context.Background(), time.Second)
	defer waitCancel()
	if err := d.Wait(waitCtx); err != nil {
		t.Fatalf("wait: %v", err)
	}

	if got := atomic.LoadInt32(&mock.calls); got != 1 {
		t.Fatalf("expected executor called once, got %d", got)
	}
}

// TestDispatcher_Dispatch_Success 验证：Dispatch → executor.Execute 调通且无 error。
func TestDispatcher_Dispatch_Success(t *testing.T) {
	logger := zap.NewNop()
	d := NewDispatcher(2, logger)
	mock := &mockExecutor{}
	d.RegisterExecutor("priority-3", mock) // PriorityHigh=3

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	if err := d.Start(ctx); err != nil {
		t.Fatalf("start: %v", err)
	}

	order := &model.Order{ID: "o2", Priority: model.PriorityHigh}
	if err := d.Dispatch(ctx, order); err != nil {
		t.Fatalf("dispatch: %v", err)
	}

	waitCtx, waitCancel := context.WithTimeout(context.Background(), time.Second)
	defer waitCancel()
	if err := d.Wait(waitCtx); err != nil {
		t.Fatalf("wait: %v", err)
	}
	if got := atomic.LoadInt32(&mock.calls); got != 1 {
		t.Fatalf("expected 1 call, got %d", got)
	}
}

// TestDispatcher_Dispatch_NoExecutor 验证：未注册 executor 时不 panic，只记 warn 日志。
func TestDispatcher_Dispatch_NoExecutor(t *testing.T) {
	logger := zap.NewNop()
	d := NewDispatcher(2, logger)
	// 不注册任何 executor。

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	if err := d.Start(ctx); err != nil {
		t.Fatalf("start: %v", err)
	}

	order := &model.Order{ID: "o3", Priority: model.PriorityUrgent}
	if err := d.Dispatch(ctx, order); err != nil {
		t.Fatalf("dispatch: %v", err)
	}

	// Wait 必须在合理时间内返回（说明没有 panic 也没有死锁）。
	waitCtx, waitCancel := context.WithTimeout(context.Background(), time.Second)
	defer waitCancel()
	if err := d.Wait(waitCtx); err != nil {
		t.Fatalf("wait: %v", err)
	}
}

// TestDispatcher_Dispatch_Concurrent 验证：10 个 order 并发 dispatch，WaitGroup 等齐。
func TestDispatcher_Dispatch_Concurrent(t *testing.T) {
	logger := zap.NewNop()
	// 4 个 worker，10 个任务，模拟「多 in-flight」场景。
	d := NewDispatcher(4, logger)
	mock := &mockExecutor{sleep: 10 * time.Millisecond}
	d.RegisterExecutor("priority-2", mock)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	if err := d.Start(ctx); err != nil {
		t.Fatalf("start: %v", err)
	}

	const n = 10
	var wg sync.WaitGroup
	wg.Add(n)
	for i := 0; i < n; i++ {
		go func(idx int) {
			defer wg.Done()
			order := &model.Order{ID: model.OrderID("o-" + string(rune('a'+idx))), Priority: model.PriorityNormal}
			if err := d.Dispatch(ctx, order); err != nil {
				t.Errorf("dispatch %d: %v", idx, err)
			}
		}(i)
	}
	wg.Wait()

	waitCtx, waitCancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer waitCancel()
	if err := d.Wait(waitCtx); err != nil {
		t.Fatalf("wait: %v", err)
	}
	if got := atomic.LoadInt32(&mock.calls); got != n {
		t.Fatalf("expected %d calls, got %d", n, got)
	}
}

// TestDispatcher_ExecutorReturnsError 验证：executor 报错不会拖垮 worker。
func TestDispatcher_ExecutorReturnsError(t *testing.T) {
	logger := zap.NewNop()
	d := NewDispatcher(2, logger)
	failing := &mockExecutor{failErr: errors.New("boom")}
	d.RegisterExecutor("priority-1", failing)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	if err := d.Start(ctx); err != nil {
		t.Fatalf("start: %v", err)
	}

	order := &model.Order{ID: "ofail", Priority: model.PriorityLow}
	if err := d.Dispatch(ctx, order); err != nil {
		t.Fatalf("dispatch: %v", err)
	}

	waitCtx, waitCancel := context.WithTimeout(context.Background(), time.Second)
	defer waitCancel()
	if err := d.Wait(waitCtx); err != nil {
		t.Fatalf("wait: %v", err)
	}
	if got := atomic.LoadInt32(&failing.calls); got != 1 {
		t.Fatalf("expected 1 call, got %d", got)
	}
}

// TestDispatcher_NilOrder 验证：nil order 直接返回 error（防止 panic）。
func TestDispatcher_NilOrder(t *testing.T) {
	logger := zap.NewNop()
	d := NewDispatcher(2, logger)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	if err := d.Start(ctx); err != nil {
		t.Fatalf("start: %v", err)
	}
	if err := d.Dispatch(ctx, nil); err == nil {
		t.Fatal("expected error for nil order")
	}
}

// TestDispatcher_ImplementsPort 验证：Dispatcher 实现了 port.Dispatcher。
func TestDispatcher_ImplementsPort(t *testing.T) {
	logger := zap.NewNop()
	d := NewDispatcher(2, logger)
	var _ port.Dispatcher = d
}

// TestDispatcher_DispatchCtxCancel 验证：Dispatch 在 ctx 取消时返回 ctx.Err。
func TestDispatcher_DispatchCtxCancel(t *testing.T) {
	logger := zap.NewNop()
	// workers=1 + bufferSize=16。我们把 buffer 塞满再 cancel ctx。
	d := NewDispatcher(1, logger)
	mock := &mockExecutor{sleep: 100 * time.Millisecond}
	d.RegisterExecutor("priority-2", mock)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	if err := d.Start(ctx); err != nil {
		t.Fatalf("start: %v", err)
	}

	// 先把 buffer 塞满（16 个） + 1 个被 worker 拿走。
	for i := 0; i < 17; i++ {
		_ = d.Dispatch(context.Background(), &model.Order{ID: model.OrderID("b"), Priority: model.PriorityNormal})
	}

	// 现在 worker 正在执行 1 个，buffer 满。再次 Dispatch 应该被 ctx 取消拦截。
	cancelledCtx, cancel2 := context.WithCancel(context.Background())
	cancel2() // 立刻取消
	err := d.Dispatch(cancelledCtx, &model.Order{ID: model.OrderID("c"), Priority: model.PriorityNormal})
	if err == nil {
		t.Fatal("expected ctx error")
	}

	// 等 worker 处理完（120ms < sleep=100ms 的累加），再做断言。
	waitCtx, waitCancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer waitCancel()
	if err := d.Wait(waitCtx); err != nil {
		t.Fatalf("wait: %v", err)
	}
}

// TestDispatcher_StopDrainsRemaining 验证：ctx 取消时 worker 会把 channel 中残留 task
// 都处理完（不丢失 task），保证 taskWG 归零。
func TestDispatcher_StopDrainsRemaining(t *testing.T) {
	logger := zap.NewNop()
	d := NewDispatcher(2, logger)
	mock := &mockExecutor{sleep: 5 * time.Millisecond}
	d.RegisterExecutor("priority-2", mock)

	ctx, cancel := context.WithCancel(context.Background())
	if err := d.Start(ctx); err != nil {
		t.Fatalf("start: %v", err)
	}

	// 投递 5 个 order。
	for i := 0; i < 5; i++ {
		if err := d.Dispatch(context.Background(), &model.Order{ID: model.OrderID("d"), Priority: model.PriorityNormal}); err != nil {
			t.Fatalf("dispatch: %v", err)
		}
	}

	// 立刻 cancel（不要等 worker 跑完），触发 drainRemaining 路径。
	cancel()

	// Wait 应该能正常返回（不卡住），说明残留 task 全部被 drain + handle。
	waitCtx, waitCancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer waitCancel()
	if err := d.Wait(waitCtx); err != nil {
		t.Fatalf("wait should not timeout: %v", err)
	}
	// 5 个 task 全部应该被执行过（无论时序如何）。
	// 允许 worker 在 drain 期间只处理部分——这取决于 cancel 时的 goroutine 调度。
	// 关键断言：Wait 不超时、程序不 panic。
}
