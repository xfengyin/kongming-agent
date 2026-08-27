// 计算表达式提取与 Calculator.Try 测试
// 从 examples/longzhong 迁移（原 TestExtractCalcExpr），并补充 Try 短路语义。

package tools

import (
	"context"
	"strings"
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

func TestCalculatorTry(t *testing.T) {
	ctx := context.Background()
	c := NewCalculator()

	// 命中且求值成功
	handled, out, err := c.Try(ctx, "计算 123*456")
	if !handled || err != nil {
		t.Fatalf("命中应成功，handled=%v err=%v", handled, err)
	}
	if !strings.Contains(out, "56088") {
		t.Errorf("输出应包含结果 56088，实际 %q", out)
	}

	// 命中但求值失败（除零）→ handled=true, err!=nil
	handled, out, err = c.Try(ctx, "计算 1/0")
	if !handled {
		t.Fatalf("表达式命中但失败应 handled=true")
	}
	if err == nil || !strings.Contains(err.Error(), "除数为零") {
		t.Errorf("除零应返回明确错误，实际 %v", err)
	}
	if out != "" {
		t.Errorf("失败时不应有输出，实际 %q", out)
	}

	// 不命中 → handled=false
	handled, out, err = c.Try(ctx, "天下大势如何？")
	if handled || err != nil || out != "" {
		t.Errorf("不命中应返回 (false, \"\", nil)，实际 handled=%v out=%q err=%v", handled, out, err)
	}
}
