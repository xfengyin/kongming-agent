// Package model 战报（BattleReport）单元测试。
package model

import (
	"testing"
	"time"
)

// TestBattleReport_Duration 验证 StartedAt + CompletedAt 计算 Duration 的正确性。
func TestBattleReport_Duration(t *testing.T) {
	started := time.Date(2026, 6, 12, 10, 0, 0, 0, time.UTC)
	completed := started.Add(2*time.Second + 500*time.Millisecond)

	r := BattleReport{
		OrderID:     "ord-1",
		Success:     true,
		StartedAt:   started,
		CompletedAt: completed,
	}

	// 业务层负责：Duration = CompletedAt.Sub(StartedAt).Seconds()
	r.Duration = r.CompletedAt.Sub(r.StartedAt).Seconds()

	if got, want := r.Duration, 2.5; got != want {
		t.Errorf("Duration = %v, want %v", got, want)
	}
}

// TestBattleReport_Generals 验证多将领子报告聚合语义。
func TestBattleReport_Generals(t *testing.T) {
	r := BattleReport{
		OrderID: "ord-2",
		Generals: []GeneralReport{
			{GeneralID: "guanyu", Name: "关羽", Success: true, Output: "ok", Duration: 1.0},
			{GeneralID: "zhangfei", Name: "张飞", Success: false, Error: "timeout", Duration: 30.0},
		},
		Success:     false, // 一名失败则整体失败
		StartedAt:   time.Now(),
		CompletedAt: time.Now().Add(31 * time.Second),
		Duration:    31.0,
	}
	if len(r.Generals) != 2 {
		t.Fatalf("Generals len = %d, want 2", len(r.Generals))
	}
	if r.Success {
		t.Error("Success = true, want false (one general failed)")
	}
	if r.Generals[1].Error != "timeout" {
		t.Errorf("Error = %q, want \"timeout\"", r.Generals[1].Error)
	}
}

// TestGeneralReport_Fields 验证 GeneralReport 字段可正确赋值。
func TestGeneralReport_Fields(t *testing.T) {
	gr := GeneralReport{
		GeneralID: "zhaoyun",
		Name:      "赵云",
		Success:   true,
		Output: map[string]any{
			"text": "longzhong plan drafted",
		},
		Duration: 3.14,
	}
	if gr.GeneralID != "zhaoyun" {
		t.Errorf("GeneralID = %q, want \"zhaoyun\"", gr.GeneralID)
	}
	if gr.Output.(map[string]any)["text"] != "longzhong plan drafted" {
		t.Errorf("Output.text mismatch: %v", gr.Output)
	}
	if gr.Duration != 3.14 {
		t.Errorf("Duration = %v, want 3.14", gr.Duration)
	}
}
