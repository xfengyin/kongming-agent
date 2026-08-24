// 计算器工具测试

package tools

import (
	"strings"
	"testing"
)

func TestEvaluateBasic(t *testing.T) {
	cases := []struct {
		expr string
		want string
	}{
		{"123*456", "56088"},
		{"1+2", "3"},
		{"10-3", "7"},
		{"20/4", "5"},
		{"2+3*4", "14"},                 // 乘除优先
		{"(2+3)*4", "20"},               // 括号优先
		{"100/3", "33.333333333333336"}, // 浮点
		{"-5+10", "5"},                  // 一元负号
		{"+7", "7"},                     // 一元正号
		{"1.5*2", "3"},                  // 小数
		{"((1+2))", "3"},                // 嵌套括号
		{"2*(3+4)-5", "9"},
		{"  7 * 8  ", "56"}, // 空白容忍
	}
	for _, c := range cases {
		got, err := Evaluate(c.expr)
		if err != nil {
			t.Errorf("Evaluate(%q) 报错: %v", c.expr, err)
			continue
		}
		if formatNumber(got) != c.want {
			t.Errorf("Evaluate(%q) = %s，期望 %s", c.expr, formatNumber(got), c.want)
		}
	}
}

func TestEvaluateErrors(t *testing.T) {
	bad := []string{
		"",        // 空
		"abc",     // 非法字符
		"1+",      // 尾部运算符
		"1/0",     // 除零
		"(1+2",    // 缺右括号
		"1+2)",    // 多余右括号
		"1..2",    // 多个小数点
		"2**3",    // 连续运算符
		"1+2+",    // 尾部运算符
		"sqrt(4)", // 函数调用（不支持）
	}
	for _, e := range bad {
		_, err := Evaluate(e)
		if err == nil {
			t.Errorf("Evaluate(%q) 应报错，实际成功", e)
		}
	}
}

func TestEvaluateZeroDivisionMessage(t *testing.T) {
	_, err := Evaluate("1/0")
	if err == nil || !strings.Contains(err.Error(), "除数为零") {
		t.Errorf("除零错误信息不明确: %v", err)
	}
}

func TestCalculatorToolInterface(t *testing.T) {
	c := NewCalculator()
	if c.Name() != "calc" {
		t.Errorf("工具名应为 calc，实际 %s", c.Name())
	}
	got, err := c.Evaluate("123*456")
	if err != nil {
		t.Fatalf("Evaluate 失败: %v", err)
	}
	if got != "56088" {
		t.Errorf("期望 56088，实际 %s", got)
	}
	_, err = c.Evaluate("1/0")
	if err == nil {
		t.Error("除零应报错")
	}
}

func TestEvaluateNoCodeExecution(t *testing.T) {
	// 不得执行任意代码：函数调用/变量/字符串全部拒绝
	for _, e := range []string{"os.Exit(1)", "`ls`", "len([])", "2+2;rm -rf /", "a=1", "exec('x')"} {
		if _, err := Evaluate(e); err == nil {
			t.Errorf("危险表达式 %q 不应被求值", e)
		}
	}
}
