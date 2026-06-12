// Package courier 是「传令兵」应用层实现：基于 Topic 的发布/订阅消息分发。
//
// 设计要点：
//   - channel-based 异步分发（Publish 不阻塞订阅者处理）；
//   - fan-out：同一 topic 多个订阅者都收到；
//   - 优雅关闭：Stop 等 in-flight 任务完成或 ctx 超时。
package courier

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/zhuge/kongming/pkg/domain/model"
	"go.uber.org/zap"
)

// 计数订阅者收到消息的次数。
type counter struct {
	n int32
}

func (c *counter) handle(_ context.Context, _ *model.Message) error {
	atomic.AddInt32(&c.n, 1)
	return nil
}

// TestCourier_Publish_Subscribe 验证：订阅者能收到 Publish 的消息。
func TestCourier_Publish_Subscribe(t *testing.T) {
	logger := zap.NewNop()
	c := NewCourier(16, logger)
	ct := &counter{}
	if err := c.Subscribe("order.created", ct.handle); err != nil {
		t.Fatalf("subscribe: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	if err := c.Start(ctx); err != nil {
		t.Fatalf("start: %v", err)
	}

	msg := &model.Message{ID: "m1", Topic: "order.created", Payload: map[string]any{"k": "v"}}
	if err := c.Publish(ctx, msg); err != nil {
		t.Fatalf("publish: %v", err)
	}

	// 等待 fan-out 完成（用 deadline 而非固定 sleep 避免偶发抖动）。
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if atomic.LoadInt32(&ct.n) >= 1 {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if got := atomic.LoadInt32(&ct.n); got != 1 {
		t.Fatalf("expected 1 received, got %d", got)
	}

	stopCtx, stopCancel := context.WithTimeout(context.Background(), time.Second)
	defer stopCancel()
	if err := c.Stop(stopCtx); err != nil {
		t.Fatalf("stop: %v", err)
	}
}

// TestCourier_MultipleSubscribers 验证：同一 topic 多个订阅者都收到（fan-out）。
func TestCourier_MultipleSubscribers(t *testing.T) {
	logger := zap.NewNop()
	c := NewCourier(16, logger)

	a, b := &counter{}, &counter{}
	if err := c.Subscribe("battle.report", a.handle); err != nil {
		t.Fatalf("subscribe a: %v", err)
	}
	if err := c.Subscribe("battle.report", b.handle); err != nil {
		t.Fatalf("subscribe b: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	if err := c.Start(ctx); err != nil {
		t.Fatalf("start: %v", err)
	}

	for i := 0; i < 3; i++ {
		msg := &model.Message{ID: "m", Topic: "battle.report"}
		if err := c.Publish(ctx, msg); err != nil {
			t.Fatalf("publish: %v", err)
		}
	}

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if atomic.LoadInt32(&a.n) == 3 && atomic.LoadInt32(&b.n) == 3 {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if got := atomic.LoadInt32(&a.n); got != 3 {
		t.Fatalf("subscriber a expected 3, got %d", got)
	}
	if got := atomic.LoadInt32(&b.n); got != 3 {
		t.Fatalf("subscriber b expected 3, got %d", got)
	}

	stopCtx, stopCancel := context.WithTimeout(context.Background(), time.Second)
	defer stopCancel()
	_ = c.Stop(stopCtx)
}

// TestCourier_StartStop 验证：Start/Stop 生命周期幂等 + Stop 能等待任务完成。
func TestCourier_StartStop(t *testing.T) {
	logger := zap.NewNop()
	c := NewCourier(16, logger)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// 重复 Start 幂等。
	if err := c.Start(ctx); err != nil {
		t.Fatalf("first start: %v", err)
	}
	if err := c.Start(ctx); err != nil {
		t.Fatalf("second start (idempotent): %v", err)
	}

	// 重复 Stop 不会 panic，且第二次为 noop。
	stopCtx, stopCancel := context.WithTimeout(context.Background(), time.Second)
	defer stopCancel()
	if err := c.Stop(stopCtx); err != nil {
		t.Fatalf("first stop: %v", err)
	}
	if err := c.Stop(stopCtx); err != nil {
		t.Fatalf("second stop (idempotent): %v", err)
	}
}

// TestCourier_Publish_NoSubscriber 验证：没人订阅时不阻塞、不 panic。
func TestCourier_Publish_NoSubscriber(t *testing.T) {
	logger := zap.NewNop()
	c := NewCourier(16, logger)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	if err := c.Start(ctx); err != nil {
		t.Fatalf("start: %v", err)
	}

	// Publish 一个没人订阅的 topic，必须立刻返回。
	publishCtx, publishCancel := context.WithTimeout(context.Background(), time.Second)
	defer publishCancel()
	if err := c.Publish(publishCtx, &model.Message{ID: "orphan", Topic: "no.subscriber"}); err != nil {
		t.Fatalf("publish with no subscriber: %v", err)
	}

	stopCtx, stopCancel := context.WithTimeout(context.Background(), time.Second)
	defer stopCancel()
	_ = c.Stop(stopCtx)
}

// TestCourier_StopWaitsForInFlight 验证：Stop 会等待订阅者处理完在飞任务。
func TestCourier_StopWaitsForInFlight(t *testing.T) {
	logger := zap.NewNop()
	c := NewCourier(16, logger)

	var done sync.WaitGroup
	done.Add(1)
	slow := func(_ context.Context, _ *model.Message) error {
		time.Sleep(100 * time.Millisecond)
		done.Done()
		return nil
	}
	if err := c.Subscribe("slow", slow); err != nil {
		t.Fatalf("subscribe: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	if err := c.Start(ctx); err != nil {
		t.Fatalf("start: %v", err)
	}
	if err := c.Publish(ctx, &model.Message{ID: "x", Topic: "slow"}); err != nil {
		t.Fatalf("publish: %v", err)
	}

	stopCtx, stopCancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer stopCancel()
	if err := c.Stop(stopCtx); err != nil {
		t.Fatalf("stop: %v", err)
	}
	// 关闭后 handler 必已跑完，否则 done.Wait 永不返回 → 死锁。
	finished := make(chan struct{})
	go func() {
		done.Wait()
		close(finished)
	}()
	select {
	case <-finished:
		// OK
	case <-time.After(time.Second):
		t.Fatal("handler did not finish before Stop returned")
	}
}

// TestCourier_HandlerError 验证：单个订阅者返回 error 不影响其他订阅者。
func TestCourier_HandlerError(t *testing.T) {
	logger := zap.NewNop()
	c := NewCourier(16, logger)

	good := &counter{}
	failHandler := func(_ context.Context, _ *model.Message) error {
		return errors.New("boom")
	}
	if err := c.Subscribe("evt", failHandler); err != nil {
		t.Fatalf("subscribe fail: %v", err)
	}
	if err := c.Subscribe("evt", good.handle); err != nil {
		t.Fatalf("subscribe good: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	if err := c.Start(ctx); err != nil {
		t.Fatalf("start: %v", err)
	}
	if err := c.Publish(ctx, &model.Message{ID: "m", Topic: "evt"}); err != nil {
		t.Fatalf("publish: %v", err)
	}

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if atomic.LoadInt32(&good.n) >= 1 {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if got := atomic.LoadInt32(&good.n); got != 1 {
		t.Fatalf("good handler should still receive, got %d", got)
	}

	stopCtx, stopCancel := context.WithTimeout(context.Background(), time.Second)
	defer stopCancel()
	_ = c.Stop(stopCtx)
}
