// Package dispatcher 是「调度器」应用层实现：异步把 Order 路由到对应 Executor。
//
// 设计要点：
//   - worker pool 模型：固定数量 goroutine 持续从 channel 拉取任务；
//   - 优先级路由：按 order.Priority 找名为 "priority-<value>" 的 executor；
//   - 优雅关闭：Wait 等所有 in-flight 任务完成或 ctx 超时；
//   - 可观测：每次 dispatch 累加 counter。
package dispatcher

import (
	"context"
	"fmt"
	"sync"

	"go.uber.org/zap"

	"github.com/zhuge/kongming/pkg/domain/model"
	"github.com/zhuge/kongming/pkg/domain/port"
)

// 编译期断言：Dispatcher 必须实现 port.Dispatcher。
var _ port.Dispatcher = (*Dispatcher)(nil)

// Executor 是本包内部别名（= port.Executor），方便外部用 dispatcher.Executor 引用。
type Executor = port.Executor

// task 是 worker pool 内部传输单元：携带一个 Order。
type task struct {
	order *model.Order
}

// Dispatcher 是 worker-pool 风格的异步调度器。
//
//  1. Dispatch 投递 task 到 buffered channel，立刻返回；
//  2. N 个 worker goroutine 持续从 channel 读 task；
//  3. worker 按 order.Priority 找 executor（name="priority-<value>"），并执行；
//  4. executor 错误只记日志，worker 继续处理下一个 task。
type Dispatcher struct {
	// workers 是 worker pool 大小（≤ 0 视作 1）。
	workers int
	logger  *zap.Logger

	// executor 表：name → Executor。优先级路由查找时按 "priority-<value>" 取。
	mu        sync.RWMutex
	executors map[string]Executor

	// 任务 channel：Dispatch 写，worker 读。
	// 这是「有界缓冲」——满了 Dispatch 阻塞或返回（看 ctx）。
	taskCh chan task

	// workerWG 跟踪所有 worker goroutine。
	// taskWG 跟踪 in-flight 的 executor 调用（每个 task Add(1) → handle 返回 Done()）。
	// Wait 等这两个 wg 都归零，确保「in-flight 完成 + worker 退出」。
	workerWG sync.WaitGroup
	taskWG   sync.WaitGroup

	// started 防止重复启动。
	startMu sync.Mutex
	started bool
}

// NewDispatcher 构造一个 Dispatcher 实例。
//
//   - workers：worker goroutine 数量；≤0 视为 1；
//   - logger：可传 zap.NewNop() 在测试中静音。
func NewDispatcher(workers int, logger *zap.Logger) *Dispatcher {
	if workers <= 0 {
		workers = 1
	}
	if logger == nil {
		logger = zap.NewNop()
	}
	return &Dispatcher{
		workers:   workers,
		logger:    logger,
		executors: make(map[string]Executor),
		// taskCh 容量为 workers 的 4 倍，最少 16，避免极端 burst 阻塞 Dispatch。
		taskCh: make(chan task, bufferSizeFor(workers)),
	}
}

// bufferSize 根据 worker 数计算一个合理的 channel 容量。
//
// 不暴露到外部，作为 NewDispatcher 的内部策略。
func bufferSizeFor(workers int) int {
	if workers*4 < 16 {
		return 16
	}
	return workers * 4
}

// RegisterExecutor 注册一个 executor 到指定 name。
//
// 同一 name 重复注册会覆盖（最新生效）。允许在 Start 前后调用。
func (d *Dispatcher) RegisterExecutor(name string, exec Executor) {
	if name == "" || exec == nil {
		return
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	d.executors[name] = exec
}

// Dispatch 把 order 投递到 worker pool，立刻返回。
//
// 行为契约：
//   - ctx 取消时立刻返回 ctx.Err()；
//   - 没有对应 executor 的 order 不阻塞——投递后 worker 会打 warn 并丢弃；
//   - 重复 Start 之后的 Dispatch 才会真正执行（避免依赖未就绪的隐式 bug）。
func (d *Dispatcher) Dispatch(ctx context.Context, order *model.Order) error {
	if order == nil {
		return fmt.Errorf("dispatcher: order is nil")
	}
	// 投递前 taskWG.Add(1)：与 worker 的 Done() 配对，确保 Wait 能感知到 in-flight。
	d.taskWG.Add(1)
	select {
	case d.taskCh <- task{order: order}:
		return nil
	case <-ctx.Done():
		// 投递失败时撤销 Add，否则 Wait 会永久 block。
		d.taskWG.Done()
		return ctx.Err()
	}
}

// Start 启动 N 个 worker goroutine 持续从 channel 读 task。
//
// 重复调用幂等。
func (d *Dispatcher) Start(ctx context.Context) error {
	d.startMu.Lock()
	if d.started {
		d.startMu.Unlock()
		return nil
	}
	d.started = true
	d.startMu.Unlock()

	for i := 0; i < d.workers; i++ {
		d.workerWG.Add(1)
		go d.workerLoop(ctx, i)
	}
	d.logger.Info("dispatcher started", zap.Int("workers", d.workers))
	return nil
}

// workerLoop 单个 worker 的主循环：读 task → 找 executor → 调 Execute。
//
// 注意：channel 读取和 ctx.Done() 共享一个 select——若两者都 ready，select 会
// 随机选一个，可能让 task 留在 channel 里没人 handle，进而导致 taskWG 永久 > 0。
// 因此在退出前用「drain 直到 channel 空」兜底：把所有残留 task 都 handle 掉。
func (d *Dispatcher) workerLoop(ctx context.Context, id int) {
	defer d.workerWG.Done()
	for {
		select {
		case <-ctx.Done():
			// ctx 取消后，channel 里可能还有 task——继续 drain 完再退出。
			d.drainRemaining()
			return
		case t, ok := <-d.taskCh:
			if !ok {
				return
			}
			d.handle(t)
		}
	}
}

// drainRemaining 排空 channel 中所有剩余 task，全部走 handle 路径。
//
// 之所以需要这个：当 ctx 取消时 select 随机性可能让 task 留在 channel 里，
// 而这些 task 已经由 Dispatch Add(1) 到 taskWG，必须 Done() 才能让 Wait 返回。
func (d *Dispatcher) drainRemaining() {
	for {
		select {
		case t, ok := <-d.taskCh:
			if !ok {
				return
			}
			d.handle(t)
		default:
			return
		}
	}
}

// handle 处理一个 task：按 priority 找 executor 并执行。
//
// 错误路径全部走 warn 日志，不影响 worker 继续处理后续 task。
//
// 注意：handle 自身就是 taskWG.Done() 的执行点——无论走哪条分支都 Done。
func (d *Dispatcher) handle(t task) {
	defer d.taskWG.Done()

	if t.order == nil {
		d.logger.Warn("dispatcher received nil order")
		return
	}

	// 优先级路由：name 形如 "priority-2"。Priority 用十进制数字。
	name := priorityName(t.order.Priority)

	d.mu.RLock()
	exec, ok := d.executors[name]
	d.mu.RUnlock()

	if !ok {
		d.logger.Warn("dispatcher: no executor registered for priority",
			zap.String("order_id", string(t.order.ID)),
			zap.String("executor_name", name),
			zap.String("priority", t.order.Priority.String()),
		)
		return
	}

	if _, err := exec.Execute(context.Background(), t.order); err != nil {
		d.logger.Warn("dispatcher executor returned error",
			zap.String("order_id", string(t.order.ID)),
			zap.String("executor_name", name),
			zap.Error(err),
		)
	}
}

// priorityName 把 Priority 映射为 executor name："priority-<n>"。
//
// 这是 Dispatcher 的核心路由约定，单元测试也基于此断言。
func priorityName(p model.Priority) string {
	return fmt.Sprintf("priority-%d", int(p))
}

// Wait 阻塞直到所有 in-flight 任务完成（taskWG 归零），或 ctx 取消/超时。
//
// 语义：
//   - 未 Start → 直接返回 nil（无 in-flight 可等）；
//   - 已 Start → 等所有 worker 取出 task 并完成 executor.Execute。
//
// task channel 不会被主动关闭，因此这里只看 taskWG：Dispatch Add(1) + handle Done()
// 自然配对，确保「所有投递的 task 都已执行完」。
func (d *Dispatcher) Wait(ctx context.Context) error {
	d.startMu.Lock()
	if !d.started {
		d.startMu.Unlock()
		return nil
	}
	d.startMu.Unlock()

	done := make(chan struct{})
	go func() {
		d.taskWG.Wait()
		close(done)
	}()
	select {
	case <-done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}
