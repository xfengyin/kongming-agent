// Package model 战略与八卦阵单元测试。
package model

import "testing"

// TestStrategy_BaguaMode 验证八卦阵 8 种模式的字符串值，
// 避免无意改动造成配置/YAML 解析失败。
func TestStrategy_BaguaMode(t *testing.T) {
	cases := map[BaguaMode]string{
		Tiangai:   "tiangai",
		Dizai:     "dizai",
		Fengyang:  "fengyang",
		Yunzhui:   "yunzhui",
		Longfei:   "longfei",
		Huyi:      "huyi",
		Niaoxiang: "niaoxiang",
		Shepan:    "shepan",
	}
	for mode, want := range cases {
		if string(mode) != want {
			t.Errorf("BaguaMode %v: got %q, want %q", mode, string(mode), want)
		}
	}
	if len(cases) != 8 {
		t.Errorf("八卦阵应有 8 种模式，实际 %d 种", len(cases))
	}
}

// TestTactic_Dependency 验证 Tactic 的依赖关系可正确表达。
//
// 场景：3 个战术 T1/T2/T3，T2 与 T3 都依赖 T1（典型 Niaoxiang 扇形）。
func TestTactic_Dependency(t *testing.T) {
	s := Strategy{
		Type:      "offensive",
		BaguaMode: Niaoxiang,
		Tactics: []Tactic{
			{Order: 1, Name: "fire", Action: "huogong"},
			{Order: 2, Name: "flood", Action: "shuibo", DependsOn: []int{1}},
			{Order: 3, Name: "ambush", Action: "xieji", DependsOn: []int{1}},
		},
		Generals:   []GeneralID{"guanyu", "zhangfei"},
		JinnangIDs: []string{"huogong-001", "shuibo-002", "xieji-003"},
	}
	if len(s.Tactics) != 3 {
		t.Fatalf("expected 3 tactics, got %d", len(s.Tactics))
	}
	if s.BaguaMode != Niaoxiang {
		t.Errorf("BaguaMode = %v, want %v", s.BaguaMode, Niaoxiang)
	}
	if len(s.Tactics[1].DependsOn) != 1 || s.Tactics[1].DependsOn[0] != 1 {
		t.Errorf("Tactic[1].DependsOn = %v, want [1]", s.Tactics[1].DependsOn)
	}
	if len(s.Generals) != 2 {
		t.Errorf("Generals len = %d, want 2", len(s.Generals))
	}
	if len(s.JinnangIDs) != 3 {
		t.Errorf("JinnangIDs len = %d, want 3", len(s.JinnangIDs))
	}
}
