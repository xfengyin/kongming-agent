// 锦囊：计算器工具（安全表达式求值）
// 运筹帷幄，算无遗策
//
// 设计约束：
//   - 仅支持数字 / 四则运算（+ - * /）/ 括号 / 小数点 / 一元负号；
//   - 表达式求值基于 github.com/Knetic/govaluate，不自己实现解析器；
//   - govaluate 只做纯算术解析与求值，绝不 eval 任意代码（无函数、无变量、无字符串）；
//   - 除零、非法字符、空表达式返回明确错误。

package tools

import (
	"context"
	"fmt"
	"math"
	"strings"
	"unicode"

	"github.com/Knetic/govaluate"
)

// Calculator 计算器工具
type Calculator struct{}

// NewCalculator 创建计算器
func NewCalculator() *Calculator { return &Calculator{} }

// Name 工具名
func (c *Calculator) Name() string { return "calc" }

// Try 判断 question 是否为计算请求（命中中文/英文前缀或裸表达式）。
// 命中且求值成功返回 (true, 结果, nil)；命中但表达式非法（如除零）
// 返回 (true, "", err)，由 Agent 决定回落 LLM；不命中返回 (false, "", nil)。
func (c *Calculator) Try(ctx context.Context, question string) (bool, string, error) {
	expr, ok := extractCalcExpr(question)
	if !ok {
		return false, "", nil
	}
	val, err := Evaluate(expr)
	if err != nil {
		return true, "", err
	}
	return true, fmt.Sprintf("🧮 计算结果：%s = %s", expr, formatNumber(val)), nil
}

// Evaluate 求值表达式，返回结果字符串
func (c *Calculator) Evaluate(expr string) (string, error) {
	val, err := Evaluate(expr)
	if err != nil {
		return "", err
	}
	return formatNumber(val), nil
}

// Evaluate 求值表达式，返回数值
func Evaluate(expr string) (float64, error) {
	trimmed := strings.TrimSpace(expr)
	if trimmed == "" {
		return 0, fmt.Errorf("表达式为空")
	}
	// govaluate 支持 ** 幂运算且不支持一元正号；为保持既有行为，这里做兼容处理：
	// 1) 拒绝 **（原实现不支持幂运算）
	// 2) 剥离前导一元正号（如 "+7"）
	if strings.Contains(trimmed, "**") {
		return 0, fmt.Errorf("表达式包含无法解析的内容")
	}
	trimmed = strings.TrimLeft(trimmed, "+")
	if trimmed == "" {
		return 0, fmt.Errorf("表达式为空")
	}
	expression, err := govaluate.NewEvaluableExpression(trimmed)
	if err != nil {
		return 0, fmt.Errorf("表达式解析失败: %w", err)
	}
	val, err := expression.Evaluate(map[string]interface{}{})
	if err != nil {
		msg := strings.ToLower(err.Error())
		if strings.Contains(msg, "division by zero") || strings.Contains(msg, "divide by zero") {
			return 0, fmt.Errorf("除数为零")
		}
		return 0, fmt.Errorf("表达式求值失败: %w", err)
	}
	f, ok := val.(float64)
	if !ok {
		return 0, fmt.Errorf("表达式求值结果不是数字")
	}
	if math.IsInf(f, 0) || math.IsNaN(f) {
		return 0, fmt.Errorf("除数为零")
	}
	return f, nil
}

// formatNumber 数字格式化：整数不带小数点
func formatNumber(v float64) string {
	if v == float64(int64(v)) {
		return fmt.Sprintf("%d", int64(v))
	}
	return fmt.Sprintf("%g", v)
}

// extractCalcExpr 从问题文本中提取计算表达式。
// 支持中文前缀（计算/算一下/帮我算）、英文前缀（calc/calculate），
// 以及裸表达式（如 "123*456"）。返回表达式与是否匹配。
func extractCalcExpr(question string) (string, bool) {
	q := strings.TrimSpace(question)
	if q == "" {
		return "", false
	}
	// 去尾部提问语气词（循环剥到稳定为止，兼容"…等于多少？"这类叠加）
	for changed := true; changed; {
		changed = false
		for _, suf := range []string{"等于多少", "是多少", "等于几", "的结果", "等于", "= ?", "=?", "？", "?"} {
			if strings.HasSuffix(q, suf) {
				q = strings.TrimSpace(strings.TrimSuffix(q, suf))
				changed = true
			}
		}
	}
	// 去前缀
	for _, pre := range []string{"计算", "算一下", "帮我算", "帮我计算", "请问计算", "calculate", "calc", "compute"} {
		if strings.HasPrefix(strings.ToLower(q), pre) {
			q = strings.TrimSpace(q[len(pre):])
			break
		}
	}
	// 去可能残留的冒号/等号
	q = strings.Trim(q, "：:＝=，, ")
	if q == "" {
		return "", false
	}
	// 必须是纯表达式字符：数字、四则、括号、小数点、空白
	for _, r := range q {
		if unicode.IsDigit(r) || strings.ContainsRune("+-*/(). ", r) {
			continue
		}
		return "", false
	}
	// 至少要有一个数字
	hasDigit := false
	for _, r := range q {
		if unicode.IsDigit(r) {
			hasDigit = true
			break
		}
	}
	if !hasDigit {
		return "", false
	}
	return q, true
}
