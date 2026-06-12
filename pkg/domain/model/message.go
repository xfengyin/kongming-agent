// Package model 领域模型 - 消息（Message）实体。
//
// 「传令兵」（Courier）模块的传输单元：Commander/General/Vault 之间通过
// Topic + Payload 异步解耦，Message 即是这个发布/订阅流的载体。
//
// 关键设计：Payload 用 map[string]any 而非结构体，匹配传令场景下「业务载荷
// 形态高度可变」的现实（如「军令」「战报」「告警」「通知」共用同一通道）。
package model

import "time"

// Message 是 Courier 传递的消息实体。
//
// 不绑定具体消息类型（命令/事件/通知），订阅方通过 Topic 与 Payload 自解释。
// 这种「弱类型」设计让新增消息类型不需要修改 Message 本身（开闭原则）。
type Message struct {
	// ID 消息唯一标识（UUID/ULID 均可），用于消费端去重/幂等。
	ID string
	// Topic 消息主题（如 "order.created"/"battle.report"），订阅方按 Topic 过滤。
	Topic string
	// Payload 消息载荷，业务方可任意 JSON 序列化/反序列化。
	// 字段值可为 string/int/bool/[]any/map[string]any 等基础类型。
	Payload map[string]any
	// Headers 消息头（traceId/spanId/...），与 Payload 分离避免业务污染链路元数据。
	Headers map[string]string
	// PublishedAt 发布时间（发送方本地时间）。
	PublishedAt time.Time
}
