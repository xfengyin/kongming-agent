// Package workflow 工作流应用层 - 内部共享工具。
//
// 提供 Runner 内部使用的辅助函数：
//   - nowFn   返回当前时间（变量便于测试时注入 mock；当前实现 = time.Now）
package workflow

import "time"

// nowFn 返回当前时间。
//
// 设计为变量（而非直接 time.Now）便于在单测中通过赋值注入 mock；
// 当前实现为 time.Now，符合「production code 用真值」原则。
var nowFn = func() time.Time { return time.Now() }
