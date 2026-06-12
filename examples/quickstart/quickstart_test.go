// Package quickstart_test 是 quickstart 示例的烟雾测试。
//
// 测试目标：跑一遍 quickstart.Run(ctx)，验证不 panic 且能拿到 BattleReport。
// 不校验业务结果（避免与 plan 阶段示例失败原因强耦合），仅做「主流程不挂」断言。
package quickstart_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/zhuge/kongming/examples/quickstart"
)

// minimalYAML 是一个最小可用的 kongming yaml：覆盖所有 required 字段。
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

// TestQuickstart_Run 验证 Run(ctx) 在最小 yaml 下能完成派单并返回 BattleReport。
func TestQuickstart_Run(t *testing.T) {
	// 准备临时配置：复制最小 yaml 到 t.TempDir，并通过 KONGMING_CONFIG 注入。
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "kongming.yaml")
	require.NoError(t, os.WriteFile(cfgPath, []byte(minimalYAML), 0o600))
	t.Setenv("KONGMING_CONFIG", cfgPath)

	// 跑 Run：给 5s 上限避免 CI 抖动。
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	report, err := quickstart.Run(ctx)
	require.NoError(t, err, "Run should not return error")
	require.NotNil(t, report, "report should not be nil")

	// 战报最小化断言：OrderID 非空、Success=true（demo 不注入 executor 时
	// runTactics 把无将的 tactic 跳过但整体 Success 仍为 true）。
	assert.NotEmpty(t, string(report.OrderID), "OrderID should be set")
	assert.True(t, report.Success, "demo run should be success (no tactics executed)")
	assert.False(t, report.StartedAt.IsZero(), "StartedAt should be set")
	assert.False(t, report.CompletedAt.IsZero(), "CompletedAt should be set")
	assert.GreaterOrEqual(t, report.Duration, 0.0, "Duration should be non-negative")
}
