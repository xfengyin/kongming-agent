// Package kongming 是 Kongming 系统的顶层装配入口。
//
// 本文件实现集成测试：覆盖 New 的三种典型场景（合法配置 / 缺失配置 / 临时配置）。
//
// 设计目标：
//  1. 装配正确性：New 必须能基于 configs/kongming.yaml 成功构造；
//  2. 错误路径：缺失文件时必须返回 error（不静默）；
//  3. 临时配置：动态生成的最小 yaml 也必须可被 New 正确解析。
package kongming

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// minimalYAML 是一个最小可用的 kongming yaml：覆盖所有 required 字段（server / observatory）。
const minimalYAML = `server:
  host: "127.0.0.1"
  port: 18080
  grpc_port: 18081
observatory:
  tracing:
    enabled: false
  log:
    level: "info"
    encoding: "json"
commander:
  default_timeout: 5s
  max_concurrent_orders: 10
`

// TestNew_ValidConfig 用仓库内 configs/kongming.yaml 启动装配，断言所有依赖非 nil。
func TestNew_ValidConfig(t *testing.T) {
	// 找到仓库根（kongming_test.go 在 pkg/kongming/ 下，相对路径 ../../configs/kongming.yaml）。
	wd, err := os.Getwd()
	require.NoError(t, err)
	cfgPath := filepath.Join(wd, "..", "..", "configs", "kongming.yaml")
	// 用 EvalSymlinks 兼容软链/相对路径，让错误信息更直观。
	if real, err := filepath.EvalSymlinks(cfgPath); err == nil {
		cfgPath = real
	}

	k, err := New(cfgPath)
	require.NoError(t, err)
	require.NotNil(t, k)

	// 核心依赖必须非 nil。
	assert.NotNil(t, k.cfg, "cfg")
	assert.NotNil(t, k.logger, "logger")
	assert.NotNil(t, k.observer, "observer")
	assert.NotNil(t, k.resilient, "resilient")
	assert.NotNil(t, k.commander, "commander")
	assert.NotNil(t, k.dispatcher, "dispatcher")
	assert.NotNil(t, k.engine, "engine")
	assert.NotNil(t, k.pool, "pool")
	assert.NotNil(t, k.vault, "vault")
	assert.NotNil(t, k.courier, "courier")
	assert.NotNil(t, k.pluginReg, "pluginReg")

	// 不应在装配阶段启动服务。
	assert.Nil(t, k.httpSrv, "httpSrv must be nil before Run")
	assert.Nil(t, k.grpcSrv, "grpcSrv must be nil before Run")
}

// TestNew_MissingConfig 期望 New 在路径不存在时返回 error。
func TestNew_MissingConfig(t *testing.T) {
	_, err := New("/tmp/this-path-does-not-exist-zzz-9c1f.yaml")
	assert.Error(t, err, "missing config must return error")
}

// TestNew_InvalidConfig 用最小临时 yaml 验证 parse 路径全通：
// 覆盖到 server / observatory / commander 三个 required 段。
func TestNew_InvalidConfig(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "kongming.yaml")
	require.NoError(t, os.WriteFile(cfgPath, []byte(minimalYAML), 0o600))

	k, err := New(cfgPath)
	require.NoError(t, err)
	require.NotNil(t, k)
	// 关键字段非 nil 即视为「parse 成功」。
	assert.NotNil(t, k.cfg)
	assert.NotNil(t, k.logger)
	assert.NotNil(t, k.commander)
}

// TestNewWithOptions_Defaults 验证 NewWithOptions 接受自定义 Options 不报错。
func TestNewWithOptions_Defaults(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "kongming.yaml")
	require.NoError(t, os.WriteFile(cfgPath, []byte(minimalYAML), 0o600))

	k, err := NewWithOptions(cfgPath, Options{ServiceName: "kongming-test"})
	require.NoError(t, err)
	require.NotNil(t, k)
	assert.Equal(t, "kongming-test", k.serviceName)
}

// TestShutdown_NotStarted 验证 Shutdown 在 Run 之前调用是安全的（幂等）。
func TestShutdown_NotStarted(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "kongming.yaml")
	require.NoError(t, os.WriteFile(cfgPath, []byte(minimalYAML), 0o600))

	k, err := New(cfgPath)
	require.NoError(t, err)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	// Run 之前 Shutdown 必须不 panic、不返回非 nil error。
	assert.NoError(t, k.Shutdown(ctx))
}

// TestRun_StartStop 集成测试：起服务 → ctx 取消 → 优雅关闭。
//
// 使用 18080/18081 高位端口避免与默认 8080/8081 冲突；
// 测试结束后 Shutdown 自动释放，不残留进程。
func TestRun_StartStop(t *testing.T) {
	dir := t.TempDir()
	// 替换为测试专用端口（避免与默认 8080/8081 冲突）。
	yamlContent := strings.ReplaceAll(minimalYAML, "18080", "38080")
	yamlContent = strings.ReplaceAll(yamlContent, "18081", "38081")
	cfgPath := filepath.Join(dir, "kongming.yaml")
	require.NoError(t, os.WriteFile(cfgPath, []byte(yamlContent), 0o600))

	k, err := New(cfgPath)
	require.NoError(t, err)

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()
	// Run 会因 ctx 超时主动 Shutdown，应返回 nil。
	assert.NoError(t, k.Run(ctx))
}
