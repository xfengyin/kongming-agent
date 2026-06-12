// Package model 领域模型 - 战略与八卦阵定义。
//
// 本文件定义 Strategy 聚合（军师派单前制定的作战方案）以及八卦阵（BaguaMode）枚举。
// 八卦阵是 Kongming 系统的核心抽象：8 种编排模式对应 8 种执行语义。
package model

// BaguaMode 八卦阵执行模式。
//
// 八卦阵来源于诸葛亮的「八阵图」，在 Kongming 中被抽象为 8 种工作流编排模式：
//   - Tiangai  天覆（天盖）：并行（DAG 拓扑层级）
//   - Dizai    地载（地载）：顺序
//   - Fengyang 风扬（风扬）：快速响应（带超时）
//   - Yunzhui  云垂（云垂）：容错重试
//   - Longfei  龙飞（龙飞）：动态调度（critical path）
//   - Huyi     虎翼（虎翼）：条件分支【Stage 1 后补齐】
//   - Niaoxiang 鸟翔（鸟翔）：扇形扩散【Stage 1 后补齐】
//   - Shepan   蛇蟠（蛇蟠）：循环迭代【Stage 1 后补齐】
//
// 用 string 别名而非枚举 int，便于配置/YAML/JSON 直接反序列化。
type BaguaMode string

const (
	// Tiangai 盖阵：并行拓扑，按 DAG 层级并发执行。
	Tiangai BaguaMode = "tiangai"
	// Dizai 载阵：顺序执行，前一节点成功才执行下一节点。
	Dizai BaguaMode = "dizai"
	// Fengyang 扬阵：快速响应，每节点带独立超时（短超时优先）。
	Fengyang BaguaMode = "fengyang"
	// Yunzhui 垂阵：容错重试，失败后按策略重试。
	Yunzhui BaguaMode = "yunzhui"
	// Longfei 飞阵：动态调度，沿 critical path 优先派单。
	Longfei BaguaMode = "longfei"
	// Huyi 翼阵：条件分支，按 Edge.Condition 路由。
	Huyi BaguaMode = "huyi"
	// Niaoxiang 翔阵：扇形扩散，单源多目标广播。
	Niaoxiang BaguaMode = "niaoxiang"
	// Shepan 蟠阵：循环迭代，按 max_iterations 循环。
	Shepan BaguaMode = "shepan"
)

// Tactic 是 Strategy 内的一个具体战术步骤。
//
// 战术间通过 DependsOn 声明依赖（基于 Order 序号），Dispatcher 据此构建 DAG。
// 这种「显式依赖」比「隐式顺序」更灵活，支持 Niaoxiang/Tiangai 等并行阵型。
type Tactic struct {
	// Order 战术在 Strategy 内的序号（1-based），仅用于展示/排序，
	// 真实依赖关系以 DependsOn 为准。
	Order int
	// Name 战术名（如「水淹」「火攻」），便于日志/战报引用。
	Name string
	// Description 战术描述，运维/审计场景使用。
	Description string
	// Action 战术动作名（指向具体锦囊或工作流节点）。
	Action string
	// Params 战术参数（如目标、阈值），具体语义由 Action 解释。
	Params map[string]any
	// DependsOn 依赖的其他战术 Order 列表；空表示无依赖，可立即执行。
	DependsOn []int
}

// Strategy 是军师派单前制定的完整战略。
//
// Strategy 既是 Order 的子聚合（随 Order 持久化），又可独立被 Commander 缓存复用。
// BaguaMode 决定 Engine 选用的工作流模式，Generals/JinnangIDs 决定参与方。
type Strategy struct {
	// Type 战略类型（如 offensive/defensive/exploration），可被 Commander 用作二级路由。
	Type string
	// Objectives 战略目标列表（自然语言），便于 Reviewer 校验战报。
	Objectives []string
	// Tactics 战术步骤列表，Dispatcher 按 BaguaMode + DependsOn 编排。
	Tactics []Tactic
	// BaguaMode 八卦阵模式，决定 Engine 的执行语义。
	BaguaMode BaguaMode
	// Generals 参与本战略的将领 ID 列表。
	Generals []GeneralID
	// JinnangIDs 引用的锦囊 ID 列表（可被 Tactic.Action 解析）。
	JinnangIDs []string
}
