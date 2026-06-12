package resilience

import (
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// TestCircuitBreaker_OpenAfterThreshold 验证连续 N 次失败后转 Open。
func TestCircuitBreaker_OpenAfterThreshold(t *testing.T) {
	cb := NewCircuitBreaker(3, 100*time.Millisecond)
	for i := 0; i < 3; i++ {
		if err := cb.Allow(); err != nil {
			t.Fatalf("closed 状态应放行: %v", err)
		}
		cb.Record(errors.New("fail"))
	}
	assert.Equal(t, StateOpen, cb.GetState(), "达到阈值应转 Open")
	assert.Equal(t, ErrCircuitOpen, cb.Allow(), "Open 状态应拒绝")
}

// TestCircuitBreaker_HalfOpen_OnlyOneProbe 验证 HalfOpen 仅放行 1 个试探。
func TestCircuitBreaker_HalfOpen_OnlyOneProbe(t *testing.T) {
	cb := NewCircuitBreaker(1, 0) // 1 次失败即开；timeout=0 表示只要时间戳变化就转 HalfOpen
	_ = cb.Allow()
	cb.Record(errors.New("fail"))
	// 等待时间戳变化（>0 即可）
	time.Sleep(time.Millisecond)
	// 第一次 Allow：应转入 HalfOpen 并放行
	assert.NoError(t, cb.Allow(), "冷却到期后第一个请求应放行")
	// 第二次：应被拒绝（试探位已被占用）
	assert.Equal(t, ErrCircuitOpen, cb.Allow(), "HalfOpen 第二个并发应拒绝")
}

// TestCircuitBreaker_Recover 验证 HalfOpen 试探成功 → 关闭熔断器。
func TestCircuitBreaker_Recover(t *testing.T) {
	cb := NewCircuitBreaker(1, 0)
	_ = cb.Allow()
	cb.Record(errors.New("fail"))
	time.Sleep(time.Millisecond)
	assert.NoError(t, cb.Allow(), "冷却到期应放行试探")
	cb.Record(nil) // 试探成功
	assert.Equal(t, StateClosed, cb.GetState(), "试探成功后应回到 Closed")
	assert.Equal(t, 0, cb.Failures(), "Closed 状态失败计数应清零")
	// 后续请求都应放行
	assert.NoError(t, cb.Allow())
	assert.NoError(t, cb.Allow())
}

// TestCircuitBreaker_HalfOpen_FailureReopen 验证 HalfOpen 试探失败 → 立即重新 Open。
func TestCircuitBreaker_HalfOpen_FailureReopen(t *testing.T) {
	cb := NewCircuitBreaker(1, 50*time.Millisecond)
	_ = cb.Allow()
	cb.Record(errors.New("fail"))
	time.Sleep(60 * time.Millisecond) // 越过 50ms 冷却
	_ = cb.Allow()                     // 转入 HalfOpen 占用试探位
	cb.Record(errors.New("fail"))      // 试探失败
	assert.Equal(t, StateOpen, cb.GetState(), "试探失败应立即转 Open")
	// 立即请求：仍在 50ms 冷却内 → 拒绝
	assert.Equal(t, ErrCircuitOpen, cb.Allow())
}

// ★ TestCircuitBreaker_ConcurrentProbes 关键并发测试。
//
// 100 个 goroutine 同时请求 Allow()，断言：
//   - 恰好 1 个被放行
//   - 其余 99 个被拒绝
//
// 这是修复 HalfOpen 缺陷的核心验证：必须严格串行化试探。
func TestCircuitBreaker_ConcurrentProbes(t *testing.T) {
	cb := NewCircuitBreaker(1, 0)
	_ = cb.Allow()
	cb.Record(errors.New("fail"))
	time.Sleep(time.Millisecond) // 让冷却时间到

	const N = 100
	var (
		wg       sync.WaitGroup
		allowed  atomic.Int32
		denied   atomic.Int32
		start    = make(chan struct{}) // 让所有 goroutine 同时起跑
	)
	for i := 0; i < N; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			if err := cb.Allow(); err == nil {
				allowed.Add(1)
			} else {
				denied.Add(1)
			}
		}()
	}
	close(start)
	wg.Wait()
	assert.Equal(t, int32(1), allowed.Load(), "HalfOpen 必须只放行 1 个试探")
	assert.Equal(t, int32(N-1), denied.Load(), "其余请求必须被拒绝")
}

// TestCircuitBreaker_Open_StaysOpenWithinTimeout 验证冷却期内不会进入 HalfOpen。
func TestCircuitBreaker_Open_StaysOpenWithinTimeout(t *testing.T) {
	cb := NewCircuitBreaker(1, 10*time.Second)
	_ = cb.Allow()
	cb.Record(errors.New("fail"))
	// 立即请求：仍在 Open 冷却内
	assert.Equal(t, ErrCircuitOpen, cb.Allow())
}

// TestCircuitBreaker_OpenToHalfOpenAfterTimeout 验证 Open 冷却到期后能转 HalfOpen。
func TestCircuitBreaker_OpenToHalfOpenAfterTimeout(t *testing.T) {
	cb := NewCircuitBreaker(1, 5*time.Millisecond)
	_ = cb.Allow()
	cb.Record(errors.New("fail"))
	time.Sleep(20 * time.Millisecond) // 超出冷却
	assert.NoError(t, cb.Allow(), "冷却到期后第一个请求应放行并转 HalfOpen")
}

// TestCircuitBreaker_SuccessInClosedResetsCounters 验证 Closed 状态成功会清零失败计数。
func TestCircuitBreaker_SuccessInClosedResetsCounters(t *testing.T) {
	cb := NewCircuitBreaker(3, time.Second)
	_ = cb.Allow()
	cb.Record(errors.New("e1"))
	cb.Record(errors.New("e2"))
	assert.Equal(t, 2, cb.Failures())
	_ = cb.Allow()
	cb.Record(nil) // 成功
	assert.Equal(t, 0, cb.Failures(), "Closed 状态成功应清零失败计数")
	assert.Equal(t, StateClosed, cb.GetState())
}
