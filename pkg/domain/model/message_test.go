// Package model 消息（Message）单元测试。
package model

import (
	"testing"
	"time"
)

// TestMessage_Payload_NilSafe 验证访问 nil Payload 不会 panic。
//
// 真实场景：Subscriber 收到一条消息后不判断 Payload 是否为空就直接遍历，
// 必须保证 nil map 的「安全 nil」语义（Go 内置特性）。
func TestMessage_Payload_NilSafe(t *testing.T) {
	m := Message{
		ID:      "msg-1",
		Topic:   "order.created",
		Payload: nil, // 故意不初始化
		Headers: map[string]string{"trace_id": "t-1"},
	}
	// 遍历 nil map：合法且 0 次迭代
	count := 0
	for k, v := range m.Payload {
		count++
		_ = k
		_ = v
	}
	if count != 0 {
		t.Errorf("expected 0 iterations on nil map, got %d", count)
	}

	// 写入 nil map：会 panic，验证我们的辅助语义
	defer func() {
		if r := recover(); r == nil {
			t.Error("expected panic on write to nil map")
		}
	}()
	m.Payload["late"] = "init" // 应当 panic
}

// TestMessage_StructFields 验证 Message 字段可正确赋值与读取。
func TestMessage_StructFields(t *testing.T) {
	now := time.Now()
	m := Message{
		ID:    "msg-2",
		Topic: "battle.report",
		Payload: map[string]any{
			"order_id": "ord-1",
			"success":  true,
			"duration": 1.23,
		},
		Headers:     map[string]string{"trace_id": "t-2"},
		PublishedAt: now,
	}
	if m.ID != "msg-2" {
		t.Errorf("ID = %q, want \"msg-2\"", m.ID)
	}
	if m.Topic != "battle.report" {
		t.Errorf("Topic = %q, want \"battle.report\"", m.Topic)
	}
	if m.Payload["order_id"] != "ord-1" {
		t.Errorf("Payload[order_id] = %v, want \"ord-1\"", m.Payload["order_id"])
	}
	if m.Payload["success"] != true {
		t.Errorf("Payload[success] = %v, want true", m.Payload["success"])
	}
	if m.Headers["trace_id"] != "t-2" {
		t.Errorf("Headers[trace_id] = %v", m.Headers["trace_id"])
	}
	if !m.PublishedAt.Equal(now) {
		t.Errorf("PublishedAt mismatch")
	}
}
