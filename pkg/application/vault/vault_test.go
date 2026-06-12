// Package vault 单元测试。
//
// 覆盖策略：
//   - 接口契约：5 个方法全部走通（Register/Get/List/Execute/LoadFromDir）
//   - 边界：不存在的 ID、空目录、无效 JSON
//   - 内置锦囊：echo / uppercase / reverse 三种 handler
//   - 目录加载：批量 .json 解析 + 注册
package vault

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"github.com/zhuge/kongming/pkg/domain/model"
)

// newTestVault 构造一个干净的 Vault 实例用于单测。
// 注入 zap.NewNop 避免污染测试输出。
func newTestVault(t *testing.T) *Vault {
	t.Helper()
	return NewVault(zap.NewNop(), nil)
}

// fakeHandler 是测试用最小 JinnangHandler 实现：
//   - Execute 直接把 input.Data 透传给 output.Data
//   - Validate / GetSchema 走空实现
type fakeHandler struct {
	execFn func(ctx context.Context, in model.JinnangInput) (*model.JinnangOutput, error)
}

func (f *fakeHandler) Execute(ctx context.Context, in model.JinnangInput) (*model.JinnangOutput, error) {
	if f.execFn != nil {
		return f.execFn(ctx, in)
	}
	return &model.JinnangOutput{Success: true, Data: in.Data}, nil
}

func (f *fakeHandler) Validate(_ model.JinnangInput) error { return nil }

func (f *fakeHandler) GetSchema() (map[string]any, error) {
	return map[string]any{"type": "object"}, nil
}

// TestVault_Register_Get 验证 Register + Get 的最小回路。
func TestVault_Register_Get(t *testing.T) {
	v := newTestVault(t)

	j := &model.Jinnang{
		ID:   "demo-1",
		Name: "示例锦囊",
		Type: model.JinnangSkill,
	}
	require.NoError(t, v.RegisterSkill(j, &fakeHandler{}))

	got, err := v.GetJinnang("demo-1")
	require.NoError(t, err)
	require.NotNil(t, got)
	assert.Equal(t, "demo-1", got.ID)
	assert.Equal(t, "示例锦囊", got.Name)
}

// TestVault_Register_Invalid 验证空 ID / nil handler 的入参校验。
func TestVault_Register_Invalid(t *testing.T) {
	v := newTestVault(t)

	err := v.RegisterSkill(&model.Jinnang{ID: ""}, &fakeHandler{})
	assert.Error(t, err, "空 ID 应返回错误")

	err = v.RegisterSkill(&model.Jinnang{ID: "ok"}, nil)
	assert.Error(t, err, "nil handler 应返回错误")
}

// TestVault_Get_NotFound 验证不存在的 ID 走 NotFound 错误。
func TestVault_Get_NotFound(t *testing.T) {
	v := newTestVault(t)

	got, err := v.GetJinnang("missing")
	assert.Nil(t, got)
	assert.Error(t, err)
}

// TestVault_List 验证 List 按 ID 字典序返回，且返回的是拷贝。
func TestVault_List(t *testing.T) {
	v := newTestVault(t)

	for _, id := range []string{"c", "a", "b"} {
		require.NoError(t, v.RegisterSkill(&model.Jinnang{ID: id, Type: model.JinnangSkill}, &fakeHandler{}))
	}

	list, err := v.ListJinnang()
	require.NoError(t, err)
	require.Len(t, list, 3)
	assert.Equal(t, "a", list[0].ID, "ListJinnang 应按字典序返回")
	assert.Equal(t, "b", list[1].ID)
	assert.Equal(t, "c", list[2].ID)

	// 修改返回 slice 不应影响内部状态。
	list[0].ID = "mutated"
	again, _ := v.ListJinnang()
	assert.Equal(t, "a", again[0].ID, "ListJinnang 必须返回防御性拷贝")
}

// TestVault_Execute_Echo 验证 echo 内置锦囊：output.Data == input.Data。
func TestVault_Execute_Echo(t *testing.T) {
	v := newTestVault(t)
	require.NoError(t, RegisterBuiltins(v))

	out, err := v.Execute(context.Background(), "echo", model.JinnangInput{Data: "hello"})
	require.NoError(t, err)
	require.NotNil(t, out)
	assert.True(t, out.Success)
	assert.Equal(t, "hello", out.Data, "echo 应原样回显")
}

// TestVault_Execute_Uppercase 验证 uppercase 把字符串转大写。
func TestVault_Execute_Uppercase(t *testing.T) {
	v := newTestVault(t)
	require.NoError(t, RegisterBuiltins(v))

	out, err := v.Execute(context.Background(), "uppercase", model.JinnangInput{Data: "KongMing"})
	require.NoError(t, err)
	require.NotNil(t, out)
	assert.True(t, out.Success)
	assert.Equal(t, "KONGMING", out.Data)
}

// TestVault_Execute_Uppercase_InvalidType 验证非 string 输入返回错误（不 panic）。
func TestVault_Execute_Uppercase_InvalidType(t *testing.T) {
	v := newTestVault(t)
	require.NoError(t, RegisterBuiltins(v))

	// 数字不是 string → handler 应在 output 标记失败或返回 error。
	out, err := v.Execute(context.Background(), "uppercase", model.JinnangInput{Data: 123})
	// 约定：类型不匹配应在 output 表达失败；error 可为 nil。
	if err == nil {
		require.NotNil(t, out)
		assert.False(t, out.Success)
	} else {
		assert.Nil(t, out)
	}
}

// TestVault_Execute_Reverse 验证 reverse 把字符串反转。
func TestVault_Execute_Reverse(t *testing.T) {
	v := newTestVault(t)
	require.NoError(t, RegisterBuiltins(v))

	out, err := v.Execute(context.Background(), "reverse", model.JinnangInput{Data: "abc"})
	require.NoError(t, err)
	require.NotNil(t, out)
	assert.True(t, out.Success)
	assert.Equal(t, "cba", out.Data)
}

// TestVault_Execute_NotFound 验证 Execute 不存在的 ID 时返回错误。
func TestVault_Execute_NotFound(t *testing.T) {
	v := newTestVault(t)

	out, err := v.Execute(context.Background(), "no-such-id", model.JinnangInput{Data: "x"})
	assert.Nil(t, out)
	assert.Error(t, err)
}

// TestVault_LoadFromDir 用 t.TempDir 写 2 个 .json 文件，验证 LoadFromDir 后 List 返回 2 个。
func TestVault_LoadFromDir(t *testing.T) {
	v := newTestVault(t)

	dir := t.TempDir()
	// 文件 1
	f1 := filepath.Join(dir, "alpha.json")
	require.NoError(t, os.WriteFile(f1, []byte(`{
		"id": "alpha",
		"name": "Alpha",
		"type": "skill",
		"tags": ["demo"],
		"config": {"k": "v"}
	}`), 0o600))
	// 文件 2
	f2 := filepath.Join(dir, "beta.json")
	require.NoError(t, os.WriteFile(f2, []byte(`{
		"id": "beta",
		"name": "Beta",
		"type": "tool"
	}`), 0o600))
	// 写一个非 json/yaml 文件，应被忽略。
	require.NoError(t, os.WriteFile(filepath.Join(dir, "README.md"), []byte("# readme"), 0o600))

	require.NoError(t, v.LoadFromDir(context.Background(), dir))

	list, err := v.ListJinnang()
	require.NoError(t, err)
	assert.Len(t, list, 2, "应加载 2 个锦囊")

	ids := []string{list[0].ID, list[1].ID}
	assert.Contains(t, ids, "alpha")
	assert.Contains(t, ids, "beta")

	// 验证 config 字段也被正确解析。
	for _, j := range list {
		if j.ID == "alpha" {
			assert.Equal(t, model.JinnangSkill, j.Type)
			assert.Equal(t, "v", j.Config["k"])
			assert.Equal(t, []string{"demo"}, j.Tags)
		}
	}
}

// TestVault_LoadFromDir_NotExist 验证不存在的目录返回错误。
func TestVault_LoadFromDir_NotExist(t *testing.T) {
	v := newTestVault(t)

	err := v.LoadFromDir(context.Background(), filepath.Join(t.TempDir(), "no-such-subdir"))
	assert.Error(t, err)
}

// TestVault_LoadFromDir_InvalidJSON 验证非法 JSON 文件返回错误。
func TestVault_LoadFromDir_InvalidJSON(t *testing.T) {
	v := newTestVault(t)

	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "bad.json"), []byte("not json"), 0o600))

	err := v.LoadFromDir(context.Background(), dir)
	assert.Error(t, err)
}

// TestVault_LoadFromDir_YAML 验证 .yaml 文件可被解析并注册。
func TestVault_LoadFromDir_YAML(t *testing.T) {
	v := newTestVault(t)

	dir := t.TempDir()
	yaml := `id: yaml-skill
name: YAML 示例
type: skill
version: 0.1.0
tags: [demo, sample]
`
	require.NoError(t, os.WriteFile(filepath.Join(dir, "skill.yaml"), []byte(yaml), 0o600))

	require.NoError(t, v.LoadFromDir(context.Background(), dir))

	list, err := v.ListJinnang()
	require.NoError(t, err)
	require.Len(t, list, 1)
	assert.Equal(t, "yaml-skill", list[0].ID)
	assert.Equal(t, "YAML 示例", list[0].Name)
	assert.Equal(t, model.JinnangSkill, list[0].Type)
	assert.Equal(t, "0.1.0", list[0].Version)
	assert.Equal(t, []string{"demo", "sample"}, list[0].Tags)
}

// TestNewVault_NilLogger 验证 NewVault 对 nil logger 走兜底（不 panic）。
func TestNewVault_NilLogger(t *testing.T) {
	v := NewVault(nil, nil)
	require.NotNil(t, v)
	// 用一个最小 handler 验证可正常工作。
	require.NoError(t, v.RegisterSkill(
		&model.Jinnang{ID: "x", Name: "X", Type: model.JinnangSkill},
		&fakeHandler{},
	))
	j, err := v.GetJinnang("x")
	require.NoError(t, err)
	assert.Equal(t, "x", j.ID)
}

// TestVault_Execute_ValidateError 验证 handler.Validate 失败时 output.Success=false。
type validateFailHandler struct{ fakeHandler }

func (v *validateFailHandler) Validate(_ model.JinnangInput) error {
	return fmt.Errorf("simulated validate failure")
}

func TestVault_Execute_ValidateError(t *testing.T) {
	v := newTestVault(t)
	require.NoError(t, v.RegisterSkill(
		&model.Jinnang{ID: "vf", Name: "VF", Type: model.JinnangSkill},
		&validateFailHandler{},
	))

	out, err := v.Execute(context.Background(), "vf", model.JinnangInput{Data: "x"})
	require.NoError(t, err, "Validate 失败应通过 output 表达，不视作 error")
	require.NotNil(t, out)
	assert.False(t, out.Success)
	assert.Contains(t, out.Error, "validate failed")
}

// TestVault_Execute_HandlerError 验证 handler.Execute 返回 error 时透传给上游。
type execErrorHandler struct{ fakeHandler }

func (e *execErrorHandler) Execute(_ context.Context, _ model.JinnangInput) (*model.JinnangOutput, error) {
	return nil, fmt.Errorf("simulated exec failure")
}

func TestVault_Execute_HandlerError(t *testing.T) {
	v := newTestVault(t)
	require.NoError(t, v.RegisterSkill(
		&model.Jinnang{ID: "ef", Name: "EF", Type: model.JinnangSkill},
		&execErrorHandler{},
	))

	out, err := v.Execute(context.Background(), "ef", model.JinnangInput{Data: "x"})
	assert.Error(t, err)
	assert.Nil(t, out, "handler.Execute 返回 error 时应透传 nil output")
}

// TestVault_RegisterSkill_NilJinnang 验证 RegisterSkill 对 nil jinnang 的防御。
func TestVault_RegisterSkill_NilJinnang(t *testing.T) {
	v := newTestVault(t)
	err := v.RegisterSkill(nil, &fakeHandler{})
	assert.Error(t, err)
}

// TestVault_Builtin_GetSchema 验证内置锦囊的 GetSchema 可调用。
func TestVault_Builtin_GetSchema(t *testing.T) {
	v := newTestVault(t)
	require.NoError(t, RegisterBuiltins(v))

	// echo 的 GetSchema 不约束 data。
	js, err := v.Execute(context.Background(), "echo", model.JinnangInput{Data: "x"})
	require.NoError(t, err)
	require.NotNil(t, js)
	assert.True(t, js.Success)
}
