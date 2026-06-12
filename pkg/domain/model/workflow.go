// Package model 领域模型 - 工作流（Workflow）聚合。
//
// 工作流是一张有向无环图（DAG），由 Node 与 Edge 组成；Engine 按 BaguaMode 选
// 用不同拓扑/调度算法执行。本文件只定义数据结构与静态校验（环、孤立节点），
// 不涉及运行时调度（由 application/workflow 实现）。
package model

import (
	"errors"
	"fmt"
	"sync"
	"time"
)

// Node 是工作流中的一个执行节点。
//
// 节点本身是「数据」，执行逻辑由 action 解析为对应的 NodeExecutor（plugin/SPI）。
// 这种「数据-行为」分离让工作流可序列化、跨版本兼容、动态加载。
type Node struct {
	// ID 节点唯一标识（工作流内唯一），用于 Edge.From/Edge.To 引用。
	ID string
	// Name 节点名，便于日志/战报展示。
	Name string
	// Type 节点类型（start/end/action/branch/loop），决定 Engine 如何调度该节点。
	// Stage 2 新增字段；旧 Workflow 若未设置，Runner 视作 NodeAction。
	Type NodeType
	// Action 节点动作名（指向具体锦囊或内置动作）。
	Action string
	// Params 节点参数，具体语义由 Action 解释。
	Params map[string]any
	// Timeout 节点执行超时时间（秒）；0 表示使用工作流全局默认。
	Timeout int
	// Retries 失败重试次数（不含首次执行）；0 表示不重试。
	Retries int
}

// Edge 是工作流中节点间的有向边，描述执行依赖/路由。
//
// Condition 用于 Huyi 条件分支：Edge 携带的布尔表达式评估为 true 才走该分支；
// 空 Condition 表示「无条件走」（即「前驱完成后必走」）。
type Edge struct {
	// From 边的源节点 ID（依赖上游）。
	From string
	// To 边的目标节点 ID（依赖下游）。
	To string
	// Condition 条件表达式（Huyi 阵专用），空字符串表示无条件。
	Condition string
}

// Workflow 是工作流聚合根。
//
// 一份工作流 = 节点集合 + 边集合 + 入口节点 + 执行阵型（BaguaMode）。
// Validate 负责静态校验（环/孤立），真正执行由 Engine 解析 Workflow +
// BaguaMode 后调度。Stage 2 新增 Mode 字段，Engine 据此选择 modes/* 中的
// 调度算法；零值 Dizai（顺序执行）作为默认兜底。
type Workflow struct {
	// ID 工作流唯一标识。
	ID string
	// Name 工作流名，便于 UI/CLI 展示。
	Name string
	// Nodes 节点集合，key 在工作流内必须唯一。
	Nodes []Node
	// Edges 边集合，From/To 必须引用已存在的节点。
	Edges []Edge
	// Entry 入口节点 ID（DAG 起点），空表示使用无入边的节点自动推断。
	Entry string
	// Mode 八卦阵执行模式（tiangai/dizai/...），决定 Engine 的调度算法。
	// Stage 2 新增字段；零值 Dizai（顺序执行）。
	Mode BaguaMode
}

// Validate 对工作流做静态校验：环检测与孤立节点检查。
//
// 返回 nil 表示工作流结构合法；否则返回具体错误（中文），便于上层包装为
// CodeInvalidArgument 上报给调用方。
//
// 校验规则：
//  1. 至少包含 1 个节点
//  2. Entry 字段（若非空）必须指向已存在节点
//  3. 所有边的 From/To 必须指向已存在节点
//  4. 工作流中除 Entry 外的节点必须至少有一条入边（否则为「孤立节点」）
//  5. 不能存在环（DFS 三色标记法检测）
func (w *Workflow) Validate() error {
	if len(w.Nodes) == 0 {
		return errors.New("workflow must contain at least 1 node")
	}

	// 索引：节点 ID → 是否存在
	nodeSet := make(map[string]struct{}, len(w.Nodes))
	for _, n := range w.Nodes {
		if n.ID == "" {
			return errors.New("node.ID is required")
		}
		if _, dup := nodeSet[n.ID]; dup {
			return fmt.Errorf("duplicate node id: %s", n.ID)
		}
		nodeSet[n.ID] = struct{}{}
	}

	// Entry 必须存在
	if w.Entry != "" {
		if _, ok := nodeSet[w.Entry]; !ok {
			return fmt.Errorf("entry node %q not found", w.Entry)
		}
	}

	// 边端点必须存在
	indeg := make(map[string]int, len(w.Nodes))
	for _, n := range w.Nodes {
		indeg[n.ID] = 0
	}
	adj := make(map[string][]string, len(w.Nodes))
	for _, e := range w.Edges {
		if _, ok := nodeSet[e.From]; !ok {
			return fmt.Errorf("edge.From %q references unknown node", e.From)
		}
		if _, ok := nodeSet[e.To]; !ok {
			return fmt.Errorf("edge.To %q references unknown node", e.To)
		}
		adj[e.From] = append(adj[e.From], e.To)
		indeg[e.To]++
	}

	// 孤立节点：indegree==0 且不是 Entry
	entry := w.Entry
	if entry == "" {
		// 选一个 indegree=0 节点作默认入口；若没有则第一个节点
		for id, d := range indeg {
			if d == 0 {
				entry = id
				break
			}
		}
		if entry == "" && len(w.Nodes) > 0 {
			entry = w.Nodes[0].ID
		}
	}
	for id, d := range indeg {
		if d == 0 && id != entry {
			return fmt.Errorf("orphan node %q (no incoming edge and not entry)", id)
		}
	}

	// 环检测：DFS 三色标记（0=未访问 1=在栈中 2=已完成）
	const (
		white = 0
		gray  = 1
		black = 2
	)
	color := make(map[string]int, len(w.Nodes))
	var dfs func(id string) error
	dfs = func(id string) error {
		color[id] = gray
		for _, next := range adj[id] {
			switch color[next] {
			case gray:
				return fmt.Errorf("cycle detected at %s -> %s", id, next)
			case white:
				if err := dfs(next); err != nil {
					return err
				}
			}
		}
		color[id] = black
		return nil
	}
	for id := range nodeSet {
		if color[id] == white {
			if err := dfs(id); err != nil {
				return err
			}
		}
	}
	return nil
}

// ============================================================================
// Stage 2 扩展：运行时数据结构（NodeType / NodeState / ExecutionContext）
// ============================================================================
//
// 上述「静态」Workflow 仅描述「该执行什么」，并不关心「执行到哪一步」。
// Stage 2 引入以下三类「运行时」数据结构，配套 application/workflow.Runner
// 使用：
//   - NodeType          节点类型（start/end/action/branch/loop）的字符串别名
//   - NodeState         节点执行结果（status/output/error/时间戳）
//   - ExecutionContext  一次完整执行的全局状态（runId/变量/各节点结果）
//
// 这种「静态 + 运行时」分层遵循 CQRS 思想：建模与执行解耦，便于序列化、
// 跨节点数据传递、审计与重放。

// NodeType 节点类型，决定 Engine 如何调度与解释。
//
// 使用 string 别名便于配置/YAML/JSON 直接反序列化；与 BaguaMode 风格一致。
// 与 Node.Action 的区别：Action 描述「做什么」，NodeType 描述「节点本身的
// 角色」（如 start/end 是控制流锚点，branch/loop 是结构化节点）。
type NodeType string

const (
	// NodeStart 起始节点：把外部输入注入 ExecutionContext。
	NodeStart NodeType = "start"
	// NodeEnd 结束节点：把 ExecutionContext 的结果汇总并写入 NodeState。
	NodeEnd NodeType = "end"
	// NodeAction 业务动作节点：执行具体 Action（如 LLM 调用、工具调用）。
	NodeAction NodeType = "action"
	// NodeBranch 条件分支节点：评估 Edge.Condition 决定下游。
	NodeBranch NodeType = "branch"
	// NodeLoop 循环节点：按 Params["max_iterations"] 迭代。
	NodeLoop NodeType = "loop"
)

// NodeStatus 节点执行状态枚举。
//
// 与 Strategy/Order 的 State 字段不同：NodeStatus 是「单次节点执行」的
// 结果状态，不参与状态机迁移；Engine 据此判断是否进入下一节点。
type NodeStatus string

const (
	// NodeStatusPending 节点尚未开始执行。
	NodeStatusPending NodeStatus = "pending"
	// NodeStatusRunning 节点正在执行中（用于异步节点）。
	NodeStatusRunning NodeStatus = "running"
	// NodeStatusOK 节点执行成功。
	NodeStatusOK NodeStatus = "ok"
	// NodeStatusFailed 节点执行失败（Error 字段非空）。
	NodeStatusFailed NodeStatus = "failed"
	// NodeStatusSkipped 节点被跳过（条件分支未命中 / 循环未到 / 上游失败）。
	NodeStatusSkipped NodeStatus = "skipped"
)

// NodeState 记录单次节点执行的结果。
//
// 由 NodeExecutor.Execute 返回并写入 ExecutionContext.NodeStates。
// 字段最小化原则：只保留「执行结果」+ 「可观测性」所需的元数据；
// 业务自定义输出应放在 Output（map[string]any）。
type NodeState struct {
	// ID 节点 ID（与 Node.ID 对应），便于按 ID 检索。
	ID string
	// Status 节点状态。
	Status NodeStatus
	// Output 节点输出（任意类型），由具体 Action 解释（如 LLM 文本、工具返回）。
	Output any
	// Error 节点执行错误，成功时为 nil。
	Error string
	// StartedAt 节点开始时间。
	StartedAt time.Time
	// CompletedAt 节点完成时间（含失败 / 跳过）。
	CompletedAt time.Time
}

// ExecutionContext 一次工作流执行的完整上下文。
//
// 每次 Runner.Execute 都会生成一个新的 ExecutionContext（RunID 唯一）。
// 节点之间通过 ec.Variables（共享变量）传递数据；ec.NodeStates 记录
// 各节点执行结果（用于审计/重放/调试）。
//
// 线程安全：Variables 与 NodeStates 在并行阵（Tiangai/Niaoxiang）下
// 可能被多 goroutine 并发写入，建议使用 SetVar/GetVar 等带锁的辅助方法。
type ExecutionContext struct {
	// WorkflowID 关联的工作流 ID。
	WorkflowID string
	// RunID 本次执行唯一标识（UUID）。
	RunID string
	// Variables 工作流级共享变量，节点间通过它传值。
	Variables map[string]any
	// NodeStates 节点 ID → 执行结果。
	NodeStates map[string]NodeState
	// StartedAt 执行开始时间。
	StartedAt time.Time
	// CompletedAt 执行完成时间（含失败 / 取消）。
	CompletedAt time.Time

	// mu 保护 Variables 与 NodeStates 的并发读写。
	// 嵌入匿名，与 General.mu 风格一致（导出 API 仍干净）。
	mu sync.Mutex
}

// SetVar 线程安全地写入一个工作流级变量。
//
// 建议在 NodeExecutor.Execute 内部使用（节点代码自己写自己的结果），
// 避免外部直接改 map。
func (ec *ExecutionContext) SetVar(key string, value any) {
	ec.mu.Lock()
	ec.Variables[key] = value
	ec.mu.Unlock()
}

// GetVar 线程安全地读取一个工作流级变量。
//
// 返回 (value, ok)：不存在时 ok=false、value=nil。
func (ec *ExecutionContext) GetVar(key string) (any, bool) {
	ec.mu.Lock()
	defer ec.mu.Unlock()
	v, ok := ec.Variables[key]
	return v, ok
}

// SetNodeState 线程安全地写入/覆盖一个节点的执行结果。
//
// Engine 内部用于 Tiangai/Niaoxiang 等并行场景下聚合多 goroutine 的结果。
func (ec *ExecutionContext) SetNodeState(id string, state NodeState) {
	ec.mu.Lock()
	ec.NodeStates[id] = state
	ec.mu.Unlock()
}

// GetNodeState 线程安全地读取一个节点的执行结果。
func (ec *ExecutionContext) GetNodeState(id string) (NodeState, bool) {
	ec.mu.Lock()
	defer ec.mu.Unlock()
	s, ok := ec.NodeStates[id]
	return s, ok
}
