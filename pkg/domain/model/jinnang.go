// Package model 领域模型 - 锦囊（Jinnang）聚合。
//
// 「锦囊」是 Kongming 系统的可复用作战单元（skill/tool/wisdom 三类），
// 由 JinnangHandler 接口暴露 Execute/Validate/GetSchema 三个方法。
// 应用层通过 JinnangInput/JinnangOutput 与 handler 通信，不直接依赖具体实现。
package model

import (
	"context"
	"time"
)

// JinnangType 锦囊类型。
//
//   - JinnangSkill  技能型锦囊（如「舌战」），由将领直接调用
//   - JinnangTool   工具型锦囊（如「火攻」），调用外部 API/二进制
//   - JinnangWisdom 智略型锦囊（如「空城」），组合多个 skill/tool 的策略
type JinnangType string

const (
	// JinnangSkill 技能型锦囊，将领内置能力。
	JinnangSkill JinnangType = "skill"
	// JinnangTool 工具型锦囊，调用外部资源。
	JinnangTool JinnangType = "tool"
	// JinnangWisdom 智略型锦囊，组合多个原子能力。
	JinnangWisdom JinnangType = "wisdom"
)

// Jinnang 是锦囊的元数据/描述实体。
//
// 不包含具体执行逻辑（由 JinnangHandler 实现），只描述「它是什么 / 怎么调用」。
// 这种「数据 + 行为」分离让锦囊可以独立被序列化、存储、版本化与热更新。
type Jinnang struct {
	// ID 唯一标识一个锦囊。
	ID string
	// Name 锦囊名（如「火攻」「空城」），便于 UI/日志引用。
	Name string
	// Type 锦囊类型，决定 JinnangHandler 的选型。
	Type JinnangType
	// Description 锦囊详细描述，用于 UI/文档/审计。
	Description string
	// Version 语义化版本号（如 "1.2.0"），用于热更新时的兼容性判断。
	Version string
	// Tags 标签列表（如 ["offensive", "fire", "siege"]），用于 Commander 检索。
	Tags []string
	// Config 锦囊配置（如 API key、端点、默认值），handler 自行解释。
	Config map[string]any
	// CreatedAt 锦囊注册时间。
	CreatedAt time.Time
	// UpdatedAt 锦囊最近一次更新时间。
	UpdatedAt time.Time
}

// JinnangInput 锦囊执行的入参。
//
// 三段式设计（Context/Params/Data）让 handler 可同时接收「跨调用上下文」
// （trace_id、用户身份）、「本次调用参数」（action、超时）与「业务数据」（如 LLM prompt）。
type JinnangInput struct {
	// Context 跨调用透传上下文（trace_id/user_id/...），handler 不应修改。
	Context map[string]any
	// Params 本次调用的可变参数（timeouts/retries/...）。
	Params map[string]any
	// Data 业务载荷（如 LLM prompt、查询语句），handler 自行解释。
	Data any
}

// JinnangOutput 锦囊执行的出参。
//
// Success + Error 二选一：Success=true 时 Error 应为空。
// Meta 字段用于透传 handler 自定义元数据（如 token 用量、调用耗时）。
type JinnangOutput struct {
	// Success 是否执行成功。
	Success bool
	// Data 业务结果（LLM response、查询结果集等）。
	Data any
	// Error 错误描述（仅在 Success=false 时有值）。
	Error string
	// Meta handler 自定义元数据（性能指标、计费信息等）。
	Meta map[string]any
}

// JinnangHandler 锦囊执行接口。
//
// 由 application/vault 通过 SPI 注册（进程内 / .so 动态加载 / fsnotify 热更新）。
// 三个方法的职责分离：
//   - Execute   执行锦囊，返回结果
//   - Validate  在执行前校验入参，避免无效调用进入核心路径
//   - GetSchema 暴露锦囊的 JSON Schema，供 UI 动态生成表单
type JinnangHandler interface {
	// Execute 执行锦囊。ctx 用于超时/取消/链路追踪。
	// 出错时建议同时填充 output.Error 与 error 字段，便于上游做差异化处理。
	Execute(ctx context.Context, input JinnangInput) (*JinnangOutput, error)
	// Validate 校验入参合法性。
	// 返回 nil 表示入参可被本 handler 接受。
	Validate(input JinnangInput) error
	// GetSchema 返回锦囊的 JSON Schema（map 表示），用于 UI 动态渲染。
	GetSchema() (map[string]any, error)
}
