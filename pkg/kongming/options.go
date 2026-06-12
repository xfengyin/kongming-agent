// Package kongming - Options 提供 NewWithOptions 的可配置参数。
//
// 设计原则：
//  1. 开闭原则：新增可选装配参数时只追加字段，不修改 New 签名；
//  2. 默认值收敛：ServiceName == "" 时 NewWithOptions 内部回退为 "kongming"；
//  3. 零值可用：未设置任何字段时 Options{} 等价于「全部走默认值」。

package kongming

// Options 控制 NewWithOptions 的可选装配行为。
//
// 字段按需扩展，未来可能新增：LoggerOverride、ObserverOverride、ExtraPlugins 等。
// 当前实现仅暴露业务侧可感知的服务名与两个硬编码替代项。
type Options struct {
	// ServiceName 服务名；用于日志 tag / metrics label。
	// 空字符串时由 NewWithOptions 内部回退为 "kongming"。
	ServiceName string

	// CourierBuffer 传令兵内部 channel 缓冲；<=0 时使用默认 256。
	CourierBuffer int

	// DispatcherWorkers 调度器 worker 数；<=0 时使用默认 4。
	DispatcherWorkers int
}
