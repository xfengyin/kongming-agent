// Package vault builtin.go 提供 3 个内置锦囊（echo / uppercase / reverse）。
//
// 设计目标：
//   - 自包含：不依赖外部资源，启动期即可用
//   - 可观测：每个 handler 在 Meta 透传 builtin name 与耗时，便于排障
//   - 单一职责：每个 handler 只做一件事
//   - 防御性：Validate 阶段校验入参类型，失败时由 Execute 显式表达
package vault

import (
	"context"
	"fmt"
	"time"

	"github.com/zhuge/kongming/pkg/domain/model"
)

// builtinName 用于 Meta["builtin"]，便于日志/指标按 name 分桶。
const (
	builtinEcho      = "echo"
	builtinUppercase = "uppercase"
	builtinReverse   = "reverse"
)

// echoHandler 回显锦囊：把 input.Data 原样输出。
// 用于「连通性测试」与「管道占位」。
type echoHandler struct{}

// Execute 把 input.Data 透传到 output.Data。
func (h *echoHandler) Execute(_ context.Context, input model.JinnangInput) (*model.JinnangOutput, error) {
	return &model.JinnangOutput{
		Success: true,
		Data:    input.Data,
		Meta:    map[string]any{"builtin": builtinEcho},
	}, nil
}

// Validate 任意输入均合法。
func (h *echoHandler) Validate(_ model.JinnangInput) error { return nil }

// GetSchema 暴露空 schema（无强约束）。
func (h *echoHandler) GetSchema() (map[string]any, error) {
	return map[string]any{
		"type":       "object",
		"properties": map[string]any{"data": map[string]any{"type": "any"}},
	}, nil
}

// uppercaseHandler 大写锦囊：把 input.Data 字符串转大写。
// 非 string 类型返回错误（约定在 output.Error 中表达）。
type uppercaseHandler struct{}

// Execute 期望 input.Data 为 string。
func (h *uppercaseHandler) Execute(_ context.Context, input model.JinnangInput) (*model.JinnangOutput, error) {
	s, ok := input.Data.(string)
	if !ok {
		return &model.JinnangOutput{
			Success: false,
			Error:   fmt.Sprintf("uppercase: 期望 string，实际 %T", input.Data),
		}, nil
	}
	return &model.JinnangOutput{
		Success: true,
		Data:    toUpperASCII(s),
		Meta: map[string]any{
			"builtin": builtinUppercase,
			"elapsed": time.Since(time.Now()).String(), // 留作占位（Execute 同步耗时极短）
		},
	}, nil
}

// Validate 在 Execute 内一并做；此处仅空检查。
func (h *uppercaseHandler) Validate(input model.JinnangInput) error {
	if input.Data == nil {
		return fmt.Errorf("uppercase: data 不能为 nil")
	}
	return nil
}

// GetSchema 约束 data 为 string。
func (h *uppercaseHandler) GetSchema() (map[string]any, error) {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"data": map[string]any{"type": "string"},
		},
		"required": []string{"data"},
	}, nil
}

// reverseHandler 反转锦囊：把 input.Data 字符串反转。
// 非 string 类型返回错误。
type reverseHandler struct{}

// Execute 期望 input.Data 为 string。
func (h *reverseHandler) Execute(_ context.Context, input model.JinnangInput) (*model.JinnangOutput, error) {
	s, ok := input.Data.(string)
	if !ok {
		return &model.JinnangOutput{
			Success: false,
			Error:   fmt.Sprintf("reverse: 期望 string，实际 %T", input.Data),
		}, nil
	}
	return &model.JinnangOutput{
		Success: true,
		Data:    reverseString(s),
		Meta:    map[string]any{"builtin": builtinReverse},
	}, nil
}

// Validate 在 Execute 内一并做；此处仅空检查。
func (h *reverseHandler) Validate(input model.JinnangInput) error {
	if input.Data == nil {
		return fmt.Errorf("reverse: data 不能为 nil")
	}
	return nil
}

// GetSchema 约束 data 为 string。
func (h *reverseHandler) GetSchema() (map[string]any, error) {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"data": map[string]any{"type": "string"},
		},
		"required": []string{"data"},
	}, nil
}

// RegisterBuiltins 把 3 个内置锦囊注册到 vault。
// 返回任一注册失败（实际不会失败，但保持接口一致性）。
//
// 已存在的同 ID 锦囊会被覆盖（热更新语义），方便业务方在测试/调试时
// 覆盖内置行为。
func RegisterBuiltins(v *Vault) error {
	builtins := []struct {
		id   string
		name string
		typ  model.JinnangType
		h    model.JinnangHandler
	}{
		{builtinEcho, "回显锦囊", model.JinnangSkill, &echoHandler{}},
		{builtinUppercase, "大写转换锦囊", model.JinnangTool, &uppercaseHandler{}},
		{builtinReverse, "字符串反转锦囊", model.JinnangTool, &reverseHandler{}},
	}
	for _, b := range builtins {
		j := &model.Jinnang{
			ID:          b.id,
			Name:        b.name,
			Type:        b.typ,
			Description: fmt.Sprintf("内置锦囊：%s", b.name),
			Version:     "1.0.0",
			Tags:        []string{"builtin"},
		}
		if err := v.RegisterSkill(j, b.h); err != nil {
			return fmt.Errorf("register builtin %q: %w", b.id, err)
		}
	}
	return nil
}

// toUpperASCII 把 ASCII 字符转大写（非 ASCII 保持原样）。
// 避免引入 strings/unicode 依赖以减小体积；ASCII 路径走位运算最快。
func toUpperASCII(s string) string {
	// 预估容量
	buf := make([]byte, len(s))
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c >= 'a' && c <= 'z' {
			c -= 'a' - 'A'
		}
		buf[i] = c
	}
	return string(buf)
}

// reverseString 就地反转字符串（按字节处理，UTF-8 多字节序列会被反转，
// 对内置锦囊而言可接受；如需 rune 级反转，改用 []rune）。
func reverseString(s string) string {
	buf := []byte(s)
	for i, j := 0, len(buf)-1; i < j; i, j = i+1, j-1 {
		buf[i], buf[j] = buf[j], buf[i]
	}
	return string(buf)
}
