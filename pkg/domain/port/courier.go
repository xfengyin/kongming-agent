// Package port 定义领域层对外暴露的端口（接口）契约。
//
// 本文件定义「传令兵」（Courier）的端口契约：基于 Topic 的发布/订阅消息通道。
// 实现位于 pkg/application/courier，遵循依赖倒置原则。
package port

import (
	"context"

	"github.com/zhuge/kongming/pkg/domain/model"
)

// HandlerFunc 是订阅者的回调函数签名。
//
// 返回 nil 视为处理成功；返回 error 由 Courier 记录日志但不中断其他订阅者
// （fan-out 隔离：单个订阅者失败不应影响其他订阅者）。
type HandlerFunc func(ctx context.Context, msg *model.Message) error

// Courier 是「传令兵」消息分发端口，抽象出 4 个能力。
//
//  设计要点：
//   - Topic 路由：同一 topic 可有多个订阅者（fan-out）；
//   - 异步分发：Publish 不等待订阅者处理完成；
//   - 优雅关闭：Stop 等待 in-flight 处理完成或 ctx 超时；
//   - 弱类型负载：Payload 用 map[string]any 让业务方扩展。
type Courier interface {
	// Publish 发布一条消息到内部通道，返回前会等待通道可写或 ctx 取消。
	// 没有订阅者时仍然成功返回（不阻塞、不 panic）。
	Publish(ctx context.Context, msg *model.Message) error

	// Subscribe 订阅一个 topic，注册一个 handler。
	// 同一 topic 多次调用会追加 handler（fan-out），不会覆盖。
	Subscribe(topic string, handler HandlerFunc) error

	// Start 启动后台分发 loop，将消息 fan-out 到该 topic 的所有订阅者。
	// 重复调用幂等（已启动则 noop）。
	Start(ctx context.Context) error

	// Stop 优雅关闭：停止接收新消息并等待 in-flight 消息处理完成；
	// ctx 超时则强制返回。
	Stop(ctx context.Context) error
}
