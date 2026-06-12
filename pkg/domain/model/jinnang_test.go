// Package model 锦囊（Jinnang）单元测试。
package model

import (
	"context"
	"errors"
	"testing"
)

// mockHandler 用于编译期校验 JinnangHandler 接口实现的最小 mock。
// 不导出，避免污染 model 包对外 API。
type mockHandler struct{}

func (m *mockHandler) Execute(_ context.Context, _ JinnangInput) (*JinnangOutput, error) {
	return &JinnangOutput{Success: true, Data: "ok"}, nil
}

func (m *mockHandler) Validate(_ JinnangInput) error { return nil }

func (m *mockHandler) GetSchema() (map[string]any, error) {
	return map[string]any{"type": "object"}, nil
}

// errorHandler 用于测试 Validate 失败路径。
type errorHandler struct{ mockHandler }

func (e *errorHandler) Validate(_ JinnangInput) error { return errors.New("invalid input") }

// 编译期断言：mockHandler / errorHandler 必须实现 JinnangHandler。
// 任何接口签名变动都会在编译期被捕获。
var _ JinnangHandler = (*mockHandler)(nil)
var _ JinnangHandler = (*errorHandler)(nil)

// TestJinnangType_String 验证锦囊类型常量值。
func TestJinnangType_String(t *testing.T) {
	cases := map[JinnangType]string{
		JinnangSkill:  "skill",
		JinnangTool:   "tool",
		JinnangWisdom: "wisdom",
	}
	for jt, want := range cases {
		if string(jt) != want {
			t.Errorf("JinnangType %v: got %q, want %q", jt, string(jt), want)
		}
	}
}

// TestJinnangHandler_Execute 验证 mock handler 可正常执行并返回结果。
func TestJinnangHandler_Execute(t *testing.T) {
	h := &mockHandler{}
	in := JinnangInput{Data: "hello"}
	out, err := h.Execute(context.Background(), in)
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	if !out.Success {
		t.Errorf("Success = false, want true")
	}
	if out.Data != "ok" {
		t.Errorf("Data = %v, want \"ok\"", out.Data)
	}
}

// TestJinnangHandler_Validate 验证 errorHandler 正确返回错误。
func TestJinnangHandler_Validate(t *testing.T) {
	h := &errorHandler{}
	if err := h.Validate(JinnangInput{}); err == nil {
		t.Error("expected error from Validate, got nil")
	}
}

// TestJinnangHandler_GetSchema 验证 schema 返回。
func TestJinnangHandler_GetSchema(t *testing.T) {
	h := &mockHandler{}
	schema, err := h.GetSchema()
	if err != nil {
		t.Fatalf("GetSchema failed: %v", err)
	}
	if schema["type"] != "object" {
		t.Errorf("schema[type] = %v, want \"object\"", schema["type"])
	}
}

// TestJinnang_StructFields 验证 Jinnang 字段可正确赋值。
func TestJinnang_StructFields(t *testing.T) {
	j := Jinnang{
		ID:          "huogong-001",
		Name:        "火攻",
		Type:        JinnangTool,
		Description: "调用火焰 API",
		Version:     "1.0.0",
		Tags:        []string{"offensive", "fire"},
		Config:      map[string]any{"api": "http://fire.local"},
	}
	if j.Type != JinnangTool {
		t.Errorf("Type = %v, want %v", j.Type, JinnangTool)
	}
	if j.Version != "1.0.0" {
		t.Errorf("Version = %q, want \"1.0.0\"", j.Version)
	}
	if len(j.Tags) != 2 {
		t.Errorf("Tags len = %d, want 2", len(j.Tags))
	}
	if j.Config["api"] != "http://fire.local" {
		t.Errorf("Config[api] = %v", j.Config["api"])
	}
}
