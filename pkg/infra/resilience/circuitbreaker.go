package resilience

import (
	"errors"
	"sync"
	"sync/atomic"
	"time"
)

// ErrCircuitOpen 熔断器开启/试探中时拒绝调用方。
var ErrCircuitOpen = errors.New("circuit breaker is open")

// State 熔断器状态。
type State int

const (
	// StateClosed 关闭：正常放行所有请求；累计失败到 threshold 后转 Open。
	StateClosed State = iota
	// StateOpen 开启：直接拒绝所有请求；timeout 到期后转 HalfOpen。
	StateOpen
	// StateHalfOpen 半开：仅放行 1 个试探请求；其他并发请求拒绝。
	StateHalfOpen
)

// String 返回状态的可读名称（用于日志/指标）。
func (s State) String() string {
	switch s {
	case StateClosed:
		return "closed"
	case StateOpen:
		return "open"
	case StateHalfOpen:
		return "half-open"
	default:
		return "unknown"
	}
}

// CircuitBreaker 三态熔断器。
//
// 状态机：
//
//	Closed --(失败数 ≥ threshold)--> Open
//	Open   --(now-lastFailure > timeout)--> HalfOpen
//	HalfOpen --(试探成功)--> Closed
//	HalfOpen --(试探失败)--> Open
//
// ★ HalfOpen 缺陷修复：使用 atomic.Int32 + CompareAndSwap 严格限制
// "并发试探数 = 1"，避免大量 goroutine 同时打到下游造成二次雪崩。
// sync.Mutex 仍用于保护 state/failures 的复合写，
// atomic 单独控制"是否已有人持有试探位"以保证 Lock-Free 探测并发量。
type CircuitBreaker struct {
	mu          sync.Mutex
	state       State
	failures    int
	threshold   int
	timeout     time.Duration
	lastFailure time.Time
	// halfOpen 0=空闲/已恢复正常，1=有 goroutine 正在试探。
	// 读写用原子操作；状态机转换在 mu 保护下进行。
	halfOpen atomic.Int32
	// now 注入时钟，便于测试时加速。
	now func() time.Time
}

// NewCircuitBreaker 构造熔断器。
//
// threshold <= 0 退化为 1；timeout <= 0 表示 Open→HalfOpen 立即触发（仅在时间戳变化后）。
func NewCircuitBreaker(threshold int, timeout time.Duration) *CircuitBreaker {
	if threshold <= 0 {
		threshold = 1
	}
	return &CircuitBreaker{
		threshold: threshold,
		timeout:   timeout,
		state:     StateClosed,
		now:       time.Now,
	}
}

// setNow 注入时间源（仅测试用）。
func (cb *CircuitBreaker) setNow(now func() time.Time) {
	cb.mu.Lock()
	cb.now = now
	cb.mu.Unlock()
}

// Allow 判断是否放行一次调用。返回 nil 放行；返回 ErrCircuitOpen 拒绝。
//
// 关键逻辑：
//   - Closed：直接放行（失败计数在 Record 阶段累计）
//   - Open：若冷却时间到 → 转入 HalfOpen 并占用试探位（放行 1 个）
//   - HalfOpen：CAS(0→1) 抢试探位；抢到则放行，未抢到则拒绝
func (cb *CircuitBreaker) Allow() error {
	cb.mu.Lock()
	switch cb.state {
	case StateOpen:
		// 冷却时间到 → 转入 HalfOpen
		if cb.timeout <= 0 || cb.now().Sub(cb.lastFailure) > cb.timeout {
			cb.state = StateHalfOpen
			cb.halfOpen.Store(1)
			cb.mu.Unlock()
			return nil
		}
		cb.mu.Unlock()
		return ErrCircuitOpen

	case StateHalfOpen:
		cb.mu.Unlock()
		// CAS 抢试探位：只有 1 个 goroutine 能成功
		if cb.halfOpen.CompareAndSwap(0, 1) {
			return nil
		}
		return ErrCircuitOpen

	default: // StateClosed
		cb.mu.Unlock()
		return nil
	}
}

// Record 上报一次执行结果。
//
// err == nil 视为成功；否则视为失败。
//   - Closed：累加 failures；达阈值转 Open
//   - HalfOpen：成功 → 关闭熔断器；失败 → 立即 Open
//   - Open：忽略（不会进入此路径）
func (cb *CircuitBreaker) Record(err error) {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	if err != nil {
		cb.failures++
		cb.lastFailure = cb.now()
		// 无论当前状态是 Closed 还是 HalfOpen，失败都要触发 Open
		if cb.state == StateHalfOpen {
			cb.state = StateOpen
			cb.halfOpen.Store(0)
		} else if cb.failures >= cb.threshold {
			cb.state = StateOpen
			cb.halfOpen.Store(0)
		}
		return
	}

	// 成功路径
	if cb.state == StateHalfOpen {
		cb.state = StateClosed
		cb.failures = 0
		cb.halfOpen.Store(0)
		return
	}
	if cb.state == StateClosed {
		// 成功重置部分失败计数（连续 N 次成功再清零，避免抖动）
		// 这里采用"立即清零"策略，简单可控
		cb.failures = 0
	}
}

// GetState 读取当前状态（线程安全）。
func (cb *CircuitBreaker) GetState() State {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	return cb.state
}

// Failures 读取当前失败计数（线程安全，主要用于测试/指标）。
func (cb *CircuitBreaker) Failures() int {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	return cb.failures
}
