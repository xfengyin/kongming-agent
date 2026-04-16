// 传令兵 - 消息传递系统
// 千里传音，使命必达

package courier

import (
	"context"
	"fmt"
	"sync"
	"time"

	"go.uber.org/zap"
)

// MessageType 消息类型
type MessageType string

const (
	MessageTask    MessageType = "task"    // 任务消息
	MessageEvent   MessageType = "event"   // 事件消息
	MessageCommand MessageType = "command" // 命令消息
	MessageResult  MessageType = "result"  // 结果消息
)

// Message 消息
type Message struct {
	ID        string                 `json:"id"`
	Type      MessageType           `json:"type"`
	From      string                 `json:"from"`
	To        string                 `json:"to"`
	Payload   interface{}            `json:"payload"`
	Timestamp time.Time             `json:"timestamp"`
	TTL       time.Duration         `json:"ttl"`
	Headers   map[string]string     `json:"headers"`
}

// DeliveryStatus 投递状态
type DeliveryStatus int

const (
	StatusPending DeliveryStatus = iota
	StatusDelivered
	StatusFailed
	StatusTimeout
)

// Courier 传令兵服务
type Courier struct {
	logger    *zap.Logger
	inbox     chan *Message
	outbox    chan *Message
	handlers  map[MessageType][]MessageHandler
	delivery  map[string]*DeliveryStatus
	mu        sync.RWMutex
	running   bool
}

// MessageHandler 消息处理器
type MessageHandler interface {
	Handle(ctx context.Context, msg *Message) error
}

// DeliveryStatusHandler 投递状态处理器
type DeliveryStatusHandler interface {
	OnDelivered(msg *Message)
	OnFailed(msg *Message, err error)
	OnTimeout(msg *Message)
}

// NewCourier 创建传令兵服务
func NewCourier(logger *zap.Logger) *Courier {
	return &Courier{
		logger:   logger,
		inbox:    make(chan *Message, 1000),
		outbox:   make(chan *Message, 1000),
		handlers: make(map[MessageType][]MessageHandler),
		delivery: make(map[string]*DeliveryStatus),
	}
}

// RegisterHandler 注册消息处理器
func (c *Courier) RegisterHandler(msgType MessageType, handler MessageHandler) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.handlers[msgType] = append(c.handlers[msgType], handler)
}

// Send 发送消息
func (c *Courier) Send(ctx context.Context, msg *Message) error {
	if msg.ID == "" {
		return fmt.Errorf("消息ID不能为空")
	}
	if msg.Timestamp.IsZero() {
		msg.Timestamp = time.Now()
	}

	c.logger.Info("传令兵出发",
		zap.String("msg_id", msg.ID),
		zap.String("from", msg.From),
		zap.String("to", msg.To),
		zap.String("type", string(msg.Type)),
	)

	// 放入投递队列
	select {
	case c.outbox <- msg:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	default:
		return fmt.Errorf("投递队列已满")
	}
}

// Start 启动服务
func (c *Courier) Start(ctx context.Context) {
	c.running = true
	go c.processOutbox(ctx)
	go c.processInbox(ctx)
	c.logger.Info("传令兵服务启动")
}

// Stop 停止服务
func (c *Courier) Stop() {
	c.running = false
	close(c.outbox)
	close(c.inbox)
	c.logger.Info("传令兵服务停止")
}

// processOutbox 处理发送队列
func (c *Courier) processOutbox(ctx context.Context) {
	for msg := range c.outbox {
		// 模拟网络投递
		go func(m *Message) {
			// 实际实现中，这里会通过网络发送消息
			c.mu.Lock()
			status := StatusPending
			c.delivery[m.ID] = &status
			c.mu.Unlock()

			// 模拟投递
			select {
			case <-ctx.Done():
				c.mu.Lock()
				s := StatusTimeout
				c.delivery[m.ID] = &s
				c.mu.Unlock()
				return
			case <-time.After(100 * time.Millisecond):
				c.mu.Lock()
				s := StatusDelivered
				c.delivery[m.ID] = &s
				c.mu.Unlock()
				c.inbox <- m
			}
		}(msg)
	}
}

// processInbox 处理接收队列
func (c *Courier) processInbox(ctx context.Context) {
	for msg := range c.inbox {
		c.mu.RLock()
		handlers := c.handlers[msg.Type]
		c.mu.RUnlock()

		for _, handler := range handlers {
			if err := handler.Handle(ctx, msg); err != nil {
				c.logger.Error("消息处理失败",
					zap.String("msg_id", msg.ID),
					zap.Error(err),
				)
			}
		}
	}
}

// GetDeliveryStatus 获取投递状态
func (c *Courier) GetDeliveryStatus(msgID string) (DeliveryStatus, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	status, exists := c.delivery[msgID]
	if !exists {
		return StatusPending, fmt.Errorf("消息不存在: %s", msgID)
	}
	return *status, nil
}
