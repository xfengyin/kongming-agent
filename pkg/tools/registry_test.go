// 工具注册表测试

package tools

import (
	"context"
	"errors"
	"strings"
	"testing"
)

// errDivZero 模拟求值失败（除零）
var errDivZero = errors.New("除数为零")

// fakeTool 可编程假工具：由 match 决定是否命中
type fakeTool struct {
	name  string
	match bool
	output string
	err   error
}

func (f *fakeTool) Name() string { return f.name }

func (f *fakeTool) Try(ctx context.Context, question string) (bool, string, error) {
	if !f.match {
		return false, "", nil
	}
	if f.err != nil {
		return true, "", f.err
	}
	return true, f.output, nil
}

func TestRegistryTryNoTools(t *testing.T) {
	r := NewRegistry()
	handled, name, out, err := r.Try(context.Background(), "任何问题")
	if handled || name != "" || out != "" || err != nil {
		t.Errorf("空注册表应返回全零，实际 handled=%v name=%q out=%q err=%v", handled, name, out, err)
	}
}

func TestRegistryTryFirstMatchWins(t *testing.T) {
	ctx := context.Background()
	// 两个工具都可能命中，按注册顺序取第一个
	r := NewRegistry(
		&fakeTool{name: "first", match: true, output: "第一个命中"},
		&fakeTool{name: "second", match: true, output: "第二个命中"},
	)
	handled, name, out, err := r.Try(ctx, "问题")
	if !handled || name != "first" || out != "第一个命中" || err != nil {
		t.Errorf("应取第一个命中工具，实际 name=%q out=%q err=%v", name, out, err)
	}
}

func TestRegistryTrySecondMatches(t *testing.T) {
	ctx := context.Background()
	r := NewRegistry(
		&fakeTool{name: "first", match: false},
		&fakeTool{name: "second", match: true, output: "二号"},
	)
	handled, name, out, err := r.Try(ctx, "问题")
	if !handled || name != "second" || out != "二号" || err != nil {
		t.Errorf("应落到第二个工具，实际 name=%q out=%q err=%v", name, out, err)
	}
}

func TestRegistryTryMatchButError(t *testing.T) {
	ctx := context.Background()
	// 工具命中但执行失败：handled=true 且透出错误，由 Agent 决定回落 LLM
	r := NewRegistry(&fakeTool{name: "calc", match: true, err: errDivZero})
	handled, name, out, err := r.Try(ctx, "计算 1/0")
	if !handled || name != "calc" {
		t.Errorf("命中但失败也应 handled=true 且报告工具名，实际 handled=%v name=%q", handled, name)
	}
	if err == nil || !strings.Contains(err.Error(), "除数为零") {
		t.Errorf("应透出求值错误，实际 %v", err)
	}
	if out != "" {
		t.Errorf("失败时不应有输出，实际 %q", out)
	}
}

func TestRegistryAdd(t *testing.T) {
	ctx := context.Background()
	r := NewRegistry()
	r.Add(&fakeTool{name: "calc", match: true, output: "ok"})
	handled, name, _, _ := r.Try(ctx, "计算 1+1")
	if !handled || name != "calc" {
		t.Errorf("Add 后应能命中，实际 handled=%v name=%q", handled, name)
	}
}

func TestRegistryWithCalculator(t *testing.T) {
	ctx := context.Background()
	r := NewRegistry(NewCalculator())
	handled, name, out, err := r.Try(ctx, "帮我算 7*8 等于多少？")
	if !handled || name != "calc" || err != nil {
		t.Fatalf("计算器应命中，handled=%v name=%q err=%v", handled, name, err)
	}
	if !strings.Contains(out, "56") {
		t.Errorf("输出应包含 56，实际 %q", out)
	}
}
