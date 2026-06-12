package resilience

import (
	"context"
	"errors"
	"time"
)

// ErrTimeout 弹性链中的统一超时哨兵错误。
//
// 用 errors.Is 判定；包装链路可能携带此错误以便上层识别"超时"分支。
var ErrTimeout = errors.New("resilience: operation timed out")

// WithTimeout 给 parent context 附加超时。
//
// d <= 0 时直接返回 parent + noop cancel，避免对短期任务误加 deadline。
// 返回的 cancel 始终需要调用（即使 noop），以保持调用方代码一致。
func WithTimeout(parent context.Context, d time.Duration) (context.Context, context.CancelFunc) {
	if d <= 0 {
		return parent, func() {}
	}
	return context.WithTimeout(parent, d)
}

// IsTimeout 判断 err 是否为超时（含 ctx.DeadlineExceeded / context.Canceled）。
//
// 建议在 application 层用 errors.Is(err, resilience.ErrTimeout) 替代此函数。
func IsTimeout(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, ErrTimeout) {
		return true
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return true
	}
	return false
}
