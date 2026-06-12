// Package cli 的测试。
//
// 测试目标：
//  1. 根命令注册 6 个子命令（kongming --help 包含子命令名）；
//  2. 子命令参数解析 + dry-run 路径；
//  3. 错误路径（缺参数/未注入 service）；
//  4. 通用输出工具（printJSON / printYAML）正确序列化。
//
// 测试模式：捕获 cobra.Command 的 stdout/stderr 到 bytes.Buffer。
// 注意：cobra 会在 os.Args 解析时调用 SetArgs，测试用局部 SetArgs 隔离。
package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestRootCmd_Help 验证根命令 --help 包含 6 个子命令名。
func TestRootCmd_Help(t *testing.T) {
	cmd := NewRootCmd()
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	cmd.SetArgs([]string{"--help"})

	err := cmd.Execute()
	require.NoError(t, err)

	out := buf.String()
	// 根命令应至少包含 6 个子命令。
	for _, name := range []string{"server", "dispatch", "strategy", "general", "vault", "plugin"} {
		assert.Contains(t, out, name, "root help should mention subcommand %q", name)
	}
	// 持久化 flag 也要出现。
	assert.Contains(t, out, "--config")
	assert.Contains(t, out, "--dry-run")
}

// TestRootCmd_PersistentFlags 验证 --config / --dry-run 默认值正确。
func TestRootCmd_PersistentFlags(t *testing.T) {
	cmd := NewRootCmd()
	cfg, err := cmd.PersistentFlags().GetString("config")
	require.NoError(t, err)
	assert.Equal(t, "configs/kongming.yaml", cfg)

	dryRun, err := cmd.PersistentFlags().GetBool("dry-run")
	require.NoError(t, err)
	assert.False(t, dryRun)
}

// TestDispatchCmd_DryRun 验证 --dry-run 模式输出 JSON 包含期望字段。
func TestDispatchCmd_DryRun(t *testing.T) {
	cmd := NewRootCmd()
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	cmd.SetArgs([]string{"dispatch", "--name", "attack-chengdu", "--priority", "4", "--dry-run"})

	err := cmd.Execute()
	require.NoError(t, err, "dry-run should not error")

	out := buf.String()
	// 输出必须是合法 JSON。
	var payload map[string]any
	require.NoError(t, json.Unmarshal([]byte(strings.TrimSpace(out)), &payload), "output must be JSON: %s", out)

	// dry-run 标记 + 订单名应出现在输出中。
	assert.Equal(t, true, payload["dry_run"])
	assert.Equal(t, "dispatch", payload["action"])

	order, ok := payload["order"].(map[string]any)
	require.True(t, ok, "order field should be an object")
	// Go 默认使用 struct 字段名大写作为 JSON key（无 json tag）。
	assert.Equal(t, "attack-chengdu", order["Name"])
	// 优先级 4 = urgent。
	assert.Equal(t, float64(4), order["Priority"])
}

// TestDispatchCmd_MissingName 验证未传 --name 时返回错误。
func TestDispatchCmd_MissingName(t *testing.T) {
	cmd := NewRootCmd()
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	cmd.SetArgs([]string{"dispatch"})

	err := cmd.Execute()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "name is required")
}

// TestGeneralCmd_NoArgs 验证 `kongming general` 不带子命令时不报错（打印 help）。
//
// cobra 默认行为：父命令无 RunE 时会打印 help 并返回 nil。
func TestGeneralCmd_NoArgs(t *testing.T) {
	cmd := NewRootCmd()
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	cmd.SetArgs([]string{"general"})

	err := cmd.Execute()
	// general 父命令没有 RunE，应回退到 help。
	require.NoError(t, err)
	out := buf.String()
	assert.Contains(t, out, "general", "should show help mentioning general")
}

// TestGeneralCmd_List_DryRun 验证 `kongming general list --dry-run` 输出五虎将占位数据。
func TestGeneralCmd_List_DryRun(t *testing.T) {
	cmd := NewRootCmd()
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	cmd.SetArgs([]string{"general", "list", "--dry-run"})

	err := cmd.Execute()
	require.NoError(t, err)
	out := buf.String()
	assert.Contains(t, out, "guanyu")
	assert.Contains(t, out, "zhangfei")
	assert.Contains(t, out, "dry_run", true)
}

// TestVaultCmd_Exec_MissingID 验证 exec 不带 id 时返回错误。
//
// cobra ExactArgs(1) 会在 RunE 之前拦截并返回 "accepts 1 arg(s)" 错误，
// 任意包含 "id" 或 "accepts" 关键字的错误都视为通过。
func TestVaultCmd_Exec_MissingID(t *testing.T) {
	cmd := NewRootCmd()
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	cmd.SetArgs([]string{"vault", "exec"})

	err := cmd.Execute()
	require.Error(t, err)
	errStr := err.Error()
	assert.True(t,
		strings.Contains(errStr, "id is required") || strings.Contains(errStr, "accepts 1 arg"),
		"error should mention missing id or cobra arg validation, got: %q", errStr)
}

// TestVaultCmd_Exec_MissingData 验证 exec 缺 --data 时返回错误。
func TestVaultCmd_Exec_MissingData(t *testing.T) {
	cmd := NewRootCmd()
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	cmd.SetArgs([]string{"vault", "exec", "fire-attack"})

	err := cmd.Execute()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "--data")
}

// TestVaultCmd_Exec_InvalidData 验证 --data 非法 JSON 时返回错误。
func TestVaultCmd_Exec_InvalidData(t *testing.T) {
	cmd := NewRootCmd()
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	cmd.SetArgs([]string{"vault", "exec", "fire-attack", "--data", "not-json"})

	err := cmd.Execute()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid --data JSON")
}

// TestVaultCmd_Exec_DryRun 验证合法 --data + --dry-run 路径。
func TestVaultCmd_Exec_DryRun(t *testing.T) {
	cmd := NewRootCmd()
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	cmd.SetArgs([]string{"vault", "exec", "fire-attack", "--data", `{"target":"chengdu"}`, "--dry-run"})

	err := cmd.Execute()
	require.NoError(t, err)
	out := buf.String()
	assert.Contains(t, out, "fire-attack")
	assert.Contains(t, out, "dry_run", true)
}

// TestPluginCmd_List 验证 plugin list 子命令。
func TestPluginCmd_List(t *testing.T) {
	cmd := NewRootCmd()
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	cmd.SetArgs([]string{"plugin", "list"})

	err := cmd.Execute()
	// 未注入 service 但未传 --dry-run，应返回 ErrServiceNotWired。
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrServiceNotWired)
}

// TestPluginCmd_List_DryRun 验证 plugin list --dry-run 正常输出。
func TestPluginCmd_List_DryRun(t *testing.T) {
	cmd := NewRootCmd()
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	cmd.SetArgs([]string{"plugin", "list", "--dry-run"})

	err := cmd.Execute()
	require.NoError(t, err)
	assert.Contains(t, buf.String(), "plugin.list")
}

// TestPluginCmd_Load_MissingPath 验证 plugin load 缺 --path 时报错。
func TestPluginCmd_Load_MissingPath(t *testing.T) {
	cmd := NewRootCmd()
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	cmd.SetArgs([]string{"plugin", "load"})

	err := cmd.Execute()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "--path")
}

// TestStrategyCmd_Plan_MissingID 验证 plan 缺 order_id 时报错。
func TestStrategyCmd_Plan_MissingID(t *testing.T) {
	cmd := NewRootCmd()
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	cmd.SetArgs([]string{"strategy", "plan"})

	err := cmd.Execute()
	require.Error(t, err)
}

// TestStrategyCmd_Plan_DryRun 验证 plan --dry-run 输出 YAML 包含 strategy 字段。
func TestStrategyCmd_Plan_DryRun(t *testing.T) {
	cmd := NewRootCmd()
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	cmd.SetArgs([]string{"strategy", "plan", "order-001", "--dry-run"})

	err := cmd.Execute()
	require.NoError(t, err)
	out := buf.String()
	// YAML 输出应包含 strategy 关键字段。
	assert.Contains(t, out, "strategy:")
	assert.Contains(t, out, "tiangai")
	assert.Contains(t, out, "dry_run")
}

// TestServerCmd_DryRun 验证 server --dry-run 输出 JSON 包含 config path。
func TestServerCmd_DryRun(t *testing.T) {
	cmd := NewRootCmd()
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	cmd.SetArgs([]string{"server", "--dry-run", "--config", "testdata/x.yaml"})

	err := cmd.Execute()
	require.NoError(t, err)
	out := buf.String()
	var payload map[string]any
	require.NoError(t, json.Unmarshal([]byte(strings.TrimSpace(out)), &payload))
	assert.Equal(t, "testdata/x.yaml", payload["config"])
	assert.Equal(t, true, payload["dry_run"])
}

// TestServerCmd_NotWired 验证 server 非 dry-run 且无 service 时返回 ErrServiceNotWired。
func TestServerCmd_NotWired(t *testing.T) {
	cmd := NewRootCmd()
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	cmd.SetArgs([]string{"server"})

	err := cmd.Execute()
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrServiceNotWired)
}

// TestOutputJSON 单元测试 printJSON：合法 map 应输出可解析 JSON。
func TestOutputJSON(t *testing.T) {
	var buf bytes.Buffer
	payload := map[string]any{"a": 1, "b": "hello"}
	require.NoError(t, printJSON(&buf, payload))

	var got map[string]any
	require.NoError(t, json.Unmarshal([]byte(strings.TrimSpace(buf.String())), &got))
	assert.Equal(t, float64(1), got["a"])
	assert.Equal(t, "hello", got["b"])
}

// TestOutputYAML 单元测试 printYAML：合法 struct 应输出含字段名的 YAML。
func TestOutputYAML(t *testing.T) {
	var buf bytes.Buffer
	payload := map[string]any{"mode": "tiangai", "ok": true}
	require.NoError(t, printYAML(&buf, payload))
	out := buf.String()
	assert.Contains(t, out, "mode:")
	assert.Contains(t, out, "tiangai")
}

// TestOutputError 单元测试 printError：nil 不输出，err 时输出。
func TestOutputError(t *testing.T) {
	var buf bytes.Buffer
	printError(&buf, nil)
	assert.Empty(t, buf.String())

	buf.Reset()
	printError(&buf, assertError("boom"))
	assert.Contains(t, buf.String(), "boom")
}

// TestWithService_AndServiceFromContext 验证 ctx 注入与取出。
func TestWithService_AndServiceFromContext(t *testing.T) {
	svc := &Service{ConfigPath: "x.yaml"}
	ctx := WithService(context.Background(), svc)

	got, ok := ServiceFromContext(ctx)
	require.True(t, ok)
	assert.Equal(t, "x.yaml", got.ConfigPath)

	// nil ctx 走 fallback。
	_, ok = ServiceFromContext(nil)
	assert.False(t, ok)
}

// assertError 是小工具：构造一个 error 字面量用于测试 printError。
func assertError(s string) error {
	return &strErr{s: s}
}

type strErr struct{ s string }

func (e *strErr) Error() string { return e.s }
