// 锦囊：简单计算器工具（安全表达式求值）
// 运筹帷幄，算无遗策
//
// 设计约束：
//   - 仅支持数字 / 四则运算（+ - * /）/ 括号 / 小数点 / 一元负号；
//   - 递归下降解析 + 直接求值，绝不 eval 任意代码（无函数、无变量、无字符串）；
//   - 除零、非法字符、空表达式返回明确错误。

package tools

import (
	"context"
	"fmt"
	"strings"
	"unicode"
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
	p := &parser{input: strings.TrimSpace(expr)}
	if p.input == "" {
		return 0, fmt.Errorf("表达式为空")
	}
	val, err := p.parseExpression()
	if err != nil {
		return 0, err
	}
	if !p.atEnd() {
		return 0, fmt.Errorf("表达式包含无法解析的内容: %q", p.remaining())
	}
	return val, nil
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

// parser 递归下降解析器
type parser struct {
	input string
	pos   int
}

// atEnd 是否已到末尾
func (p *parser) atEnd() bool { return p.pos >= len(p.input) }

// remaining 剩余输入
func (p *parser) remaining() string { return p.input[p.pos:] }

// peek 当前字符（可能跳过空白）
func (p *parser) peek() rune {
	for p.pos < len(p.input) {
		r := rune(p.input[p.pos])
		if unicode.IsSpace(r) {
			p.pos++
			continue
		}
		return r
	}
	return 0
}

// parseExpression 表达式：expr = term (('+'|'-') term)*
func (p *parser) parseExpression() (float64, error) {
	left, err := p.parseTerm()
	if err != nil {
		return 0, err
	}
	for {
		r := p.peek()
		if r == '+' || r == '-' {
			p.pos++
			right, err := p.parseTerm()
			if err != nil {
				return 0, err
			}
			if r == '+' {
				left += right
			} else {
				left -= right
			}
			continue
		}
		break
	}
	return left, nil
}

// parseTerm 项：term = factor (('*'|'/') factor)*
func (p *parser) parseTerm() (float64, error) {
	left, err := p.parseFactor()
	if err != nil {
		return 0, err
	}
	for {
		r := p.peek()
		if r == '*' || r == '/' {
			p.pos++
			right, err := p.parseFactor()
			if err != nil {
				return 0, err
			}
			if r == '*' {
				left *= right
			} else {
				if right == 0 {
					return 0, fmt.Errorf("除数为零")
				}
				left /= right
			}
			continue
		}
		break
	}
	return left, nil
}

// parseFactor 因子：factor = ('-' factor) | number | '(' expression ')'
func (p *parser) parseFactor() (float64, error) {
	r := p.peek()
	switch {
	case r == '-':
		p.pos++
		val, err := p.parseFactor()
		if err != nil {
			return 0, err
		}
		return -val, nil
	case r == '+':
		p.pos++ // 一元正号（如 "+123"）
		return p.parseFactor()
	case r == '(':
		p.pos++
		val, err := p.parseExpression()
		if err != nil {
			return 0, err
		}
		if p.peek() != ')' {
			return 0, fmt.Errorf("缺少右括号")
		}
		p.pos++
		return val, nil
	default:
		return p.parseNumber()
	}
}

// parseNumber 数字：digits ['.' digits]
func (p *parser) parseNumber() (float64, error) {
	start := p.pos
	seenDot := false
	for p.pos < len(p.input) {
		c := p.input[p.pos]
		if unicode.IsDigit(rune(c)) {
			p.pos++
			continue
		}
		if c == '.' && !seenDot {
			seenDot = true
			p.pos++
			continue
		}
		break
	}
	if p.pos == start {
		return 0, fmt.Errorf("此处应为数字: %q", p.input[p.pos:])
	}
	// 解析为 float
	var val float64
	if _, err := fmt.Sscanf(p.input[start:p.pos], "%g", &val); err != nil {
		return 0, fmt.Errorf("数字解析失败: %q", p.input[start:p.pos])
	}
	return val, nil
}
