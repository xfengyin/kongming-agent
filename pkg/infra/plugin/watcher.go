// Package plugin watcher.go 提供基于 fsnotify 的目录热更新监听。
//
// 设计：
//   - Watch 在后台 goroutine 中运行；ctx 取消时优雅退出。
//   - 仅处理 .yaml 与 .so 两种文件后缀；其它事件直接跳过。
//   - Reload 是占位实现：只记录日志。具体的"加载 → 解析 → 注入 registry"由 application 层
//     通过 ReloadHook 注入，避免本包反向依赖 application。
package plugin

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/fsnotify/fsnotify"
	"go.uber.org/zap"
)

// 关注的文件后缀。其它类型文件的事件会被忽略。
var (
	watchedExtYAML = ".yaml"
	watchedExtSO   = ".so"
)

// reloadHook 是 application 层注入的业务处理回调。
// 默认实现仅记录日志，application 层在启动时通过 SetReloadHook 替换。
var (
	reloadHookMu sync.RWMutex
	reloadHook   = defaultReloadHook
)

// defaultReloadHook 是 Reload 的占位实现：仅打 INFO 日志，不做实际加载。
// 这样可观测性先到位，真实业务（解析 yaml / dlopen .so / 注入 registry）由调用方接入。
func defaultReloadHook(path string, logger *zap.Logger) error {
	ext := filepath.Ext(path)
	logger.Info("plugin reload placeholder",
		zap.String("file", path),
		zap.String("ext", ext),
	)
	return nil
}

// SetReloadHook 注入自定义 reload 处理函数。常用于 application 层接入真实注册逻辑。
// 传 nil 可恢复默认占位实现。该函数是并发安全的。
func SetReloadHook(fn func(path string, logger *zap.Logger) error) {
	reloadHookMu.Lock()
	defer reloadHookMu.Unlock()
	if fn == nil {
		reloadHook = defaultReloadHook
		return
	}
	reloadHook = fn
}

// invokeReload 内部调用当前 hook；并发安全。
func invokeReload(path string, logger *zap.Logger) error {
	reloadHookMu.RLock()
	fn := reloadHook
	reloadHookMu.RUnlock()
	return fn(path, logger)
}

// Watch 在 dir 上启动文件监听并阻塞监听事件直到 ctx 取消。
//
// 行为契约：
//   - dir 不存在：返回 error（不启动 goroutine）。
//   - 启动后台 goroutine 后立即返回 nil；调用方通过 ctx 控制生命周期。
//   - ctx 取消后 goroutine 会在下一次 select 唤醒时退出，并关闭底层 fsnotify watcher。
//   - fsnotify 自身的错误会通过 logger.Warn 暴露，不影响主流程。
func (r *Registry) Watch(ctx context.Context, dir string, logger *zap.Logger) error {
	if r == nil {
		return errors.New("plugin: Watch called on nil registry")
	}
	if dir == "" {
		return errors.New("plugin: empty watch dir")
	}
	if ctx == nil {
		return errors.New("plugin: nil context")
	}
	if logger == nil {
		return errors.New("plugin: nil logger")
	}
	// dir 必须是已存在的目录；否则立刻失败以便启动期 fail-fast。
	info, err := os.Stat(dir)
	if err != nil {
		return fmt.Errorf("plugin: stat watch dir: %w", err)
	}
	if !info.IsDir() {
		return fmt.Errorf("plugin: watch path is not a directory: %s", dir)
	}

	w, err := fsnotify.NewWatcher()
	if err != nil {
		return fmt.Errorf("plugin: create fsnotify watcher: %w", err)
	}
	if err := w.Add(dir); err != nil {
		_ = w.Close()
		return fmt.Errorf("plugin: watch dir %s: %w", dir, err)
	}

	go r.watchLoop(ctx, dir, w, logger)
	return nil
}

// watchLoop 是后台 goroutine 的主体；独立函数便于测试。
func (r *Registry) watchLoop(ctx context.Context, dir string, w *fsnotify.Watcher, logger *zap.Logger) {
	defer w.Close()

	logger.Info("plugin watcher started", zap.String("dir", dir))
	for {
		select {
		case <-ctx.Done():
			logger.Info("plugin watcher stopped", zap.String("dir", dir), zap.String("reason", ctx.Err().Error()))
			return
		case ev, ok := <-w.Events:
			if !ok {
				logger.Warn("plugin watcher events channel closed", zap.String("dir", dir))
				return
			}
			if !isWatchedOp(ev.Op) {
				continue
			}
			if !isWatchedExt(ev.Name) {
				continue
			}
			if err := r.Reload(ev.Name, logger); err != nil {
				logger.Error("plugin reload failed",
					zap.String("file", ev.Name),
					zap.Error(err),
				)
			}
		case err, ok := <-w.Errors:
			if !ok {
				logger.Warn("plugin watcher errors channel closed", zap.String("dir", dir))
				return
			}
			logger.Warn("plugin watcher error", zap.Error(err))
		}
	}
}

// isWatchedOp 判断事件是否值得处理：Write / Create / Rename 后落入的 Write 都需要 reload。
// Rename / Remove 不主动 reload（应用层可选实现缓存失效）。
func isWatchedOp(op fsnotify.Op) bool {
	return op&(fsnotify.Write|fsnotify.Create) != 0
}

// isWatchedExt 判断文件后缀是否被本 watcher 关注。
func isWatchedExt(name string) bool {
	ext := strings.ToLower(filepath.Ext(name))
	return ext == watchedExtYAML || ext == watchedExtSO
}

// Reload 是 Watch 触发后的处理函数占位。application 层可通过 SetReloadHook 注入真实逻辑。
func (r *Registry) Reload(path string, logger *zap.Logger) error {
	if path == "" {
		return errors.New("plugin: empty reload path")
	}
	if logger == nil {
		// 测试或非生产调用可能不传 logger；此处降级到 no-op logger，避免 nil 解引用。
		logger = zap.NewNop()
	}
	return invokeReload(path, logger)
}
