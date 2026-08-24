// 隆中对 --tool calc 表达式提取测试

package main

import (
	"testing"
)

func TestExtractCalcExpr(t *testing.T) {
	cases := []struct {
		q    string
		want string
		ok   bool
	}{
		{"计算 123*456", "123*456", true},
		{"算一下 (2+3)*4", "(2+3)*4", true},
		{"帮我算 100/4 等于多少？", "100/4", true},
		{"123*456", "123*456", true},
		{"(2+3)*4 等于多少？", "(2+3)*4", true},
		{"calc 1.5*2", "1.5*2", true},
		{"calculate 7*8 是多少", "7*8", true},
		{"-5+10 等于几", "-5+10", true},
		// 非计算问题 → 不匹配，走 LLM
		{"天下大势如何？", "", false},
		{"如何提升团队执行力？", "", false},
		{"什么是三国？", "", false},
		{"", "", false},
		// 含非法字符 → 不匹配
		{"计算 123 的平方", "", false},
		{"计算 sqrt(4)", "", false},
	}
	for _, c := range cases {
		got, ok := extractCalcExpr(c.q)
		if ok != c.ok {
			t.Errorf("extractCalcExpr(%q) ok=%v，期望 %v", c.q, ok, c.ok)
			continue
		}
		if ok && got != c.want {
			t.Errorf("extractCalcExpr(%q) = %q，期望 %q", c.q, got, c.want)
		}
	}
}
