// Kongming 传令兵测试

package courier

import (
	"context"
	"testing"
	"time"

	"go.uber.org/zap"
)

func TestCourierSend(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	courier := NewCourier(logger)

	ctx := context.Background()
	courier.Start(ctx)
	defer courier.Stop()

	msg := &Message{
		ID:   "test-001",
		Type: MessageTask,
		From: "commander",
		To:   "general",
		Payload: map[string]interface{}{
			"task": "市场调研",
		},
	}

	err := courier.Send(ctx, msg)
	if err != nil {
		t.Errorf("发送消息失败: %v", err)
	}

	// 等待投递
	time.Sleep(200 * time.Millisecond)

	status, err := courier.GetDeliveryStatus("test-001")
	if err != nil {
		t.Errorf("获取状态失败: %v", err)
	}
	if status != StatusDelivered {
		t.Errorf("期望状态为 Delivered，实际为 %v", status)
	}
}
