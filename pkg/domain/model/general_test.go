// Package model 将领（General）单元测试。
package model

import (
	"sync"
	"testing"
)

// TestGeneral_SetState_Concurrent 验证 SetState/GetState 在 100 并发 goroutine 下
// 不会出现竞态或 panic。
//
// 验证点：
//  1. race detector 报告干净（-race 模式编译运行）
//  2. 最终状态必然是最后一次写入的某个值
func TestGeneral_SetState_Concurrent(t *testing.T) {
	g := &General{ID: "guanyu", Name: "关羽"}
	states := []GeneralState{GeneralIdle, GeneralBusy, GeneralResting, GeneralOffline}

	var wg sync.WaitGroup
	wg.Add(100)
	for i := 0; i < 100; i++ {
		go func(i int) {
			defer wg.Done()
			// 50 个写 + 50 个读交错
			if i%2 == 0 {
				g.SetState(states[i%len(states)])
			} else {
				_ = g.GetState()
			}
		}(i)
	}
	wg.Wait()

	// 最终状态必须是 states 中之一
	final := g.GetState()
	ok := false
	for _, s := range states {
		if final == s {
			ok = true
			break
		}
	}
	if !ok {
		t.Errorf("final state %v is not one of expected states", final)
	}
}

// TestGeneralStats 验证 GeneralStats 字段可正确赋值。
func TestGeneralStats(t *testing.T) {
	g := &General{
		ID:    "zhangfei",
		Name:  "张飞",
		State: int(GeneralIdle),
		Stats: GeneralStats{
			TotalMissions:   100,
			SuccessCount:    90,
			FailureCount:    10,
			AvgResponseTime: 1.5,
		},
	}
	if g.Stats.TotalMissions != 100 {
		t.Errorf("TotalMissions = %d, want 100", g.Stats.TotalMissions)
	}
	if g.Stats.SuccessCount+g.Stats.FailureCount != g.Stats.TotalMissions {
		t.Errorf("Success(%d) + Failure(%d) != Total(%d)",
			g.Stats.SuccessCount, g.Stats.FailureCount, g.Stats.TotalMissions)
	}
	if g.Stats.AvgResponseTime != 1.5 {
		t.Errorf("AvgResponseTime = %f, want 1.5", g.Stats.AvgResponseTime)
	}
}

// TestGeneralState_String 验证 GeneralState 的可读字符串。
func TestGeneralState_String(t *testing.T) {
	cases := map[GeneralState]string{
		GeneralIdle:     "idle",
		GeneralBusy:     "busy",
		GeneralResting:  "resting",
		GeneralOffline:  "offline",
		GeneralState(9): "unknown",
	}
	for s, want := range cases {
		if got := s.String(); got != want {
			t.Errorf("GeneralState(%d).String() = %q, want %q", s, got, want)
		}
	}
}

// TestGeneralType_Values 验证五虎将常量值与字符串一致。
func TestGeneralType_Values(t *testing.T) {
	cases := map[GeneralType]string{
		GeneralGuanYu:     "guanyu",
		GeneralZhangFei:   "zhangfei",
		GeneralZhaoYun:    "zhaoyun",
		GeneralMaChao:     "machao",
		GeneralHuangZhong: "huangzhong",
	}
	for gt, want := range cases {
		if string(gt) != want {
			t.Errorf("GeneralType %v: got %q, want %q", gt, string(gt), want)
		}
	}
	if len(cases) != 5 {
		t.Errorf("五虎将应有 5 位，实际 %d 位", len(cases))
	}
}
