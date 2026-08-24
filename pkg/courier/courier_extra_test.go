// 传令兵扩展测试：处理器分发与投递状态

package courier

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"go.uber.org/zap"
)

// recordingHandler 记录收到的消息（原子计数 + 原子指针，避免竞态）
type recordingHandler struct {
	count int32
	msg   atomic.Pointer[Message]
}

func (h *recordingHandler) Handle(ctx context.Context, msg *Message) error {
	atomic.AddInt32(&h.count, 1)
	h.msg.Store(msg)
	return nil
}

func TestCourierHandlerDispatch(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	c := NewCourier(logger)
	ctx := context.Background()
	c.Start(ctx)
	defer c.Stop()

	h := &recordingHandler{}
	c.RegisterHandler(MessageTask, h)

	msg := &Message{
		ID:   "test-dispatch-001",
		Type: MessageTask,
		From: "commander",
		To:   "general",
	}
	if err := c.Send(ctx, msg); err != nil {
		t.Fatalf("发送失败: %v", err)
	}

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if atomic.LoadInt32(&h.count) >= 1 {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if atomic.LoadInt32(&h.count) != 1 {
		t.Errorf("期望处理器被调用1次，实际 %d", h.count)
	}
	got := h.msg.Load()
	if got == nil || got.ID != "test-dispatch-001" {
		t.Errorf("处理器应收到对应消息")
	}
}

func TestCourierSendAfterStop(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	c := NewCourier(logger)
	ctx := context.Background()
	c.Start(ctx)
	c.Stop()

	msg := &Message{ID: "test-after-stop", Type: MessageEvent}
	err := c.Send(ctx, msg)
	if err == nil {
		t.Errorf("服务停止后发送应报错")
	}
}

func TestCourierStopIdempotent(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	c := NewCourier(logger)
	ctx := context.Background()
	c.Start(ctx)
	c.Stop()
	c.Stop() // 不应 panic
}
