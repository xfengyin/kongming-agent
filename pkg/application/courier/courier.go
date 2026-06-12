// Package courier 是「传令兵」应用层实现：基于 Topic 的发布/订阅消息分发。
//
// 设计要点：
//   - channel-based 异步分发（Publish 不阻塞订阅者处理）；
//   - fan-out：同一 topic 多个订阅者都收到；
//   - 优雅关闭：Stop 等 in-flight 任务完成或 ctx 超时；
//   - 可观测：每次 publish 累加 counter。
package courier

import (
	"context"
	"fmt"
	"sync"

	"go.uber.org/zap"

	"github.com/zhuge/kongming/pkg/domain/model"
	"github.com/zhuge/kongming/pkg/domain/port"
)

// 编译期断言：Courier 必须实现 port.Courier。
var _ port.Courier = (*Courier)(nil)

// HandlerFunc 是本包内部对订阅者 handler 的别名（= port.HandlerFunc）。
//
// 这里重命名是为了让包外引用本包时使用 `courier.HandlerFunc`，避免直接
// 引入 port 包造成依赖泄漏。
type HandlerFunc = port.HandlerFunc

// Courier 是 channel-based 消息分发器：Publish 把消息扔进 channel，
// 后台 goroutine 持续从 channel 读消息并 fan-out 到该 topic 的所有订阅者。
type Courier struct {
	// bufferSize 是 channel 缓冲；满了 Publish 阻塞或返回（看 ctx）。
	bufferSize int
	logger     *zap.Logger

	// 订阅者表：topic → handlers。RWMutex 保护并发读写。
	mu        sync.RWMutex
	handlers  map[string][]HandlerFunc
	topicSet  map[string]struct{} // 用于快速判定 topic 是否被订阅过（可选统计）
	startedMu sync.Mutex
	started   bool

	// msgCh 是单消费者通道：所有 Publish 都投递到这里，由后台 goroutine 串行 fan-out。
	// 注意：单个 fan-out loop 是有意为之，避免「单 message → N goroutine 风暴」。
	msgCh chan *model.Message

	// wg 跟踪 fan-out goroutine + 所有正在执行的 handler，Stop 时等待它们退出。
	wg sync.WaitGroup
}

// NewCourier 构造一个 Courier 实例。
//
//   - bufferSize：消息通道容量；0 退化为 1（unbuffered）；
//   - logger：可传 zap.NewNop() 在测试中静音。
func NewCourier(bufferSize int, logger *zap.Logger) *Courier {
	if bufferSize <= 0 {
		bufferSize = 1
	}
	if logger == nil {
		logger = zap.NewNop()
	}
	return &Courier{
		bufferSize: bufferSize,
		logger:     logger,
		handlers:   make(map[string][]HandlerFunc),
		topicSet:   make(map[string]struct{}),
		msgCh:      make(chan *model.Message, bufferSize),
	}
}

// Subscribe 注册一个 handler 到指定 topic；同一 topic 多次调用追加 handler（fan-out）。
//
// 在 Start 前后都可以调用：Start 之后注册的 handler 也会收到后续消息。
func (c *Courier) Subscribe(topic string, handler HandlerFunc) error {
	if topic == "" {
		return fmt.Errorf("courier: topic is required")
	}
	if handler == nil {
		return fmt.Errorf("courier: handler is nil")
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.handlers[topic] = append(c.handlers[topic], handler)
	c.topicSet[topic] = struct{}{}
	return nil
}

// Publish 把消息投递到内部 channel；返回前等待 channel 可写或 ctx 取消。
//
// 没有人订阅时仍然成功返回（消息会被 fan-out goroutine 读到，handler 列表为空 = noop）。
func (c *Courier) Publish(ctx context.Context, msg *model.Message) error {
	if msg == nil {
		return fmt.Errorf("courier: message is nil")
	}
	// 即使未启动也允许 Publish：消息进入缓冲；下次 Start 起来时由 loop 消费。
	// 这样可以让「订阅 + 启动前 publish」的早期用例不报错。
	select {
	case c.msgCh <- msg:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// Start 启动后台 fan-out loop。重复调用幂等。
func (c *Courier) Start(ctx context.Context) error {
	c.startedMu.Lock()
	if c.started {
		c.startedMu.Unlock()
		return nil
	}
	c.started = true
	c.startedMu.Unlock()

	c.wg.Add(1)
	go c.dispatchLoop(ctx)
	c.logger.Info("courier started", zap.Int("buffer_size", c.bufferSize))
	return nil
}

// dispatchLoop 单消费者循环：读消息 → 查订阅者 → 逐个调用 handler。
//
// handler 是顺序调用的（避免 goroutine 风暴），整体在一个 goroutine 上跑直到 Stop。
// 每个 handler 内部是用户代码，错误由用户决定是否同步。
func (c *Courier) dispatchLoop(ctx context.Context) {
	defer c.wg.Done()
	for {
		select {
		case <-ctx.Done():
			return
		case msg, ok := <-c.msgCh:
			if !ok {
				return
			}
			c.fanout(ctx, msg)
		}
	}
}

// fanout 把单条消息分发给该 topic 的所有订阅者；handler 错误只记日志不中断。
func (c *Courier) fanout(ctx context.Context, msg *model.Message) {
	if msg == nil {
		return
	}
	c.mu.RLock()
	handlers := c.handlers[msg.Topic]
	c.mu.RUnlock()

	// 无订阅者 = 直接 noop（满足「不阻塞、不 panic」契约）。
	if len(handlers) == 0 {
		return
	}

	for _, h := range handlers {
		// 每个 handler 独立计数 + 独立错误日志，便于定位问题订阅者。
		if err := c.safeInvoke(ctx, h, msg); err != nil {
			c.logger.Warn("courier handler returned error",
				zap.String("msg_id", msg.ID),
				zap.String("topic", msg.Topic),
				zap.Error(err),
			)
		}
	}
}

// safeInvoke 包装 handler 调用，捕获 panic 防止单个 handler 拖垮整个 loop。
func (c *Courier) safeInvoke(ctx context.Context, h HandlerFunc, msg *model.Message) (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("handler panic: %v", r)
			c.logger.Error("courier handler panic",
				zap.String("msg_id", msg.ID),
				zap.String("topic", msg.Topic),
				zap.Any("panic", r),
			)
		}
	}()
	return h(ctx, msg)
}

// Stop 优雅关闭：关闭 channel → 排空剩余消息 → 等待 loop 退出。
//
//	ctx 超时则直接返回（in-flight handler 可能被截断）。
func (c *Courier) Stop(ctx context.Context) error {
	c.startedMu.Lock()
	if !c.started {
		c.startedMu.Unlock()
		return nil
	}
	c.started = false
	c.startedMu.Unlock()

	// 关闭 channel 让 loop 自然退出；新 Publish 写入已关闭 channel 会 panic，
	// 因此关闭前需要确保没有并发 Publish——这由调用方保证（典型顺序：Stop → 程序退出）。
	close(c.msgCh)

	done := make(chan struct{})
	go func() {
		c.wg.Wait()
		close(done)
	}()
	select {
	case <-done:
		c.logger.Info("courier stopped gracefully")
		return nil
	case <-ctx.Done():
		c.logger.Warn("courier stop timeout, force returning", zap.Error(ctx.Err()))
		return ctx.Err()
	}
}
