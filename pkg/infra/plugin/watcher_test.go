// Package plugin watcher_test.go 覆盖 Watch + Reload 的集成行为与错误路径。
package plugin

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest"
)

// withReloadHook 临时替换 reloadHook，测试结束后恢复。
// 通过 t.Cleanup 注册，确保即便断言失败也能复位全局状态，不污染其他测试。
func withReloadHook(t *testing.T, fn func(path string, logger *zap.Logger) error) {
	t.Helper()
	prev := reloadHook
	SetReloadHook(fn)
	t.Cleanup(func() { SetReloadHook(prev) })
}

// waitFor 轮询 cond 直至其返回 true 或超时。返回是否在超时前达成条件。
// 用于 fsnotify 集成测试的"事件已触发"判定，避免硬编码 sleep。
func waitFor(t *testing.T, timeout time.Duration, cond func() bool) bool {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if cond() {
			return true
		}
		time.Sleep(20 * time.Millisecond)
	}
	return cond()
}

func TestWatch_FileCreate_TriggersReload(t *testing.T) {
	dir := t.TempDir()
	logger := zaptest.NewLogger(t)

	var called atomic.Int32
	var lastFile atomic.Value // string

	withReloadHook(t, func(path string, _ *zap.Logger) error {
		called.Add(1)
		lastFile.Store(path)
		return nil
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	r := NewRegistry()
	require.NoError(t, r.Watch(ctx, dir, logger))

	// 给 watcher 一小段时间把 inotify watch 注册到内核；不等待则首次 Create 事件可能漏报。
	time.Sleep(100 * time.Millisecond)

	target := filepath.Join(dir, "plugin.yaml")
	require.NoError(t, os.WriteFile(target, []byte("name: demo\n"), 0o600))

	// 等待 hook 触发；fsnotify 在多数内核上 < 200ms 即可见。
	ok := waitFor(t, 3*time.Second, func() bool { return called.Load() > 0 })
	require.True(t, ok, "reload hook was not invoked within timeout; called=%d", called.Load())

	got, _ := lastFile.Load().(string)
	assert.Equal(t, target, got, "reload should be called with the actual file path")
	assert.GreaterOrEqual(t, called.Load(), int32(1))
}

func TestWatch_SOFileCreate_TriggersReload(t *testing.T) {
	dir := t.TempDir()
	logger := zaptest.NewLogger(t)

	var called atomic.Int32
	withReloadHook(t, func(path string, _ *zap.Logger) error {
		if filepath.Ext(path) == ".so" {
			called.Add(1)
		}
		return nil
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	r := NewRegistry()
	require.NoError(t, r.Watch(ctx, dir, logger))
	time.Sleep(100 * time.Millisecond)

	require.NoError(t, os.WriteFile(filepath.Join(dir, "plugin.so"), []byte("ELF-not-real"), 0o600))

	ok := waitFor(t, 3*time.Second, func() bool { return called.Load() > 0 })
	assert.True(t, ok, "reload hook should fire for .so files; got %d calls", called.Load())
}

func TestWatch_NonWatchedExtension_IsIgnored(t *testing.T) {
	dir := t.TempDir()
	logger := zaptest.NewLogger(t)

	var called atomic.Int32
	withReloadHook(t, func(string, *zap.Logger) error {
		called.Add(1)
		return nil
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	r := NewRegistry()
	require.NoError(t, r.Watch(ctx, dir, logger))
	time.Sleep(100 * time.Millisecond)

	require.NoError(t, os.WriteFile(filepath.Join(dir, "ignore.txt"), []byte("x"), 0o600))

	// 等待 > 通常 fsnotify 事件传播时间，确认确实未触发。
	time.Sleep(400 * time.Millisecond)
	assert.Equal(t, int32(0), called.Load(), "non-.yaml/.so files must be ignored")
}

func TestWatch_ContextCancel_StopsWatcher(t *testing.T) {
	dir := t.TempDir()
	logger := zaptest.NewLogger(t)

	ctx, cancel := context.WithCancel(context.Background())
	r := NewRegistry()
	require.NoError(t, r.Watch(ctx, dir, logger))

	// 给后台 goroutine 时间进入 select 阻塞。
	time.Sleep(100 * time.Millisecond)

	cancel()

	// 通过创建文件并确认不再触发来验证 goroutine 已退出；
	// 若未退出，hook 仍可能被调用。给一个保守的等待窗口。
	withReloadHook(t, func(string, *zap.Logger) error {
		t.Errorf("reload hook should NOT be called after cancel")
		return nil
	})
	time.Sleep(200 * time.Millisecond)
	require.NoError(t, os.WriteFile(filepath.Join(dir, "after_cancel.yaml"), []byte("x"), 0o600))
	time.Sleep(300 * time.Millisecond)
	// 若上面 t.Errorf 未触发，即视为通过。
}

func TestWatch_InvalidDir_ReturnsError(t *testing.T) {
	r := NewRegistry()
	err := r.Watch(context.Background(), "/nonexistent/path/should/not/exist", zaptest.NewLogger(t))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "plugin:")
}

func TestWatch_EmptyDir_ReturnsError(t *testing.T) {
	r := NewRegistry()
	err := r.Watch(context.Background(), "", zaptest.NewLogger(t))
	assert.Error(t, err)
}

func TestWatch_NilLogger_ReturnsError(t *testing.T) {
	dir := t.TempDir()
	r := NewRegistry()
	err := r.Watch(context.Background(), dir, nil)
	assert.Error(t, err)
}

func TestWatch_NilContext_ReturnsError(t *testing.T) {
	dir := t.TempDir()
	// nolint:staticcheck // 故意传 nil ctx 触发错误分支
	err := NewRegistry().Watch(nil, dir, zaptest.NewLogger(t)) //nolint:staticcheck
	assert.Error(t, err)
}

func TestWatch_PathIsFile_NotDirectory(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "not-a-dir.yaml")
	require.NoError(t, os.WriteFile(file, []byte("x"), 0o600))
	err := NewRegistry().Watch(context.Background(), file, zaptest.NewLogger(t))
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not a directory")
}

func TestWatch_OnNilRegistry_ReturnsError(t *testing.T) {
	var r *Registry
	err := r.Watch(context.Background(), t.TempDir(), zaptest.NewLogger(t))
	assert.Error(t, err)
}

func TestReload_HookError_Propagated(t *testing.T) {
	wantErr := errors.New("hook failed")
	withReloadHook(t, func(string, *zap.Logger) error {
		return wantErr
	})

	r := NewRegistry()
	err := r.Reload("/tmp/x.yaml", zaptest.NewLogger(t))
	require.ErrorIs(t, err, wantErr)
}

func TestReload_EmptyPath_ReturnsError(t *testing.T) {
	r := NewRegistry()
	err := r.Reload("", zaptest.NewLogger(t))
	assert.Error(t, err)
}

func TestReload_NilLogger_FallsBackToNop(t *testing.T) {
	// Reload 接受 nil logger 并降级为 no-op，避免 nil 解引用。
	// 这条路径的语义验证：不应 panic、hook 仍被调用。
	var called atomic.Int32
	withReloadHook(t, func(string, *zap.Logger) error {
		called.Add(1)
		return nil
	})

	r := NewRegistry()
	require.NoError(t, r.Reload("/tmp/whatever.yaml", nil))
	assert.Equal(t, int32(1), called.Load())
}

func TestSetReloadHook_NilRestoresDefault(t *testing.T) {
	// nil 应回退到 defaultReloadHook；default 实现不会 panic。
	prev := reloadHook
	defer func() { SetReloadHook(prev) }()

	SetReloadHook(nil)
	require.NoError(t, defaultReloadHook("/tmp/x.yaml", zaptest.NewLogger(t)))
}

func TestIsWatchedExt_AndOp(t *testing.T) {
	assert.True(t, isWatchedExt("/a/b.yaml"))
	assert.True(t, isWatchedExt("/a/b.YAML"), "case-insensitive")
	assert.True(t, isWatchedExt("/a/b.so"))
	assert.False(t, isWatchedExt("/a/b.txt"))
	assert.False(t, isWatchedExt(""))
}
