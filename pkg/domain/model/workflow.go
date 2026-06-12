// Package model 领域模型 - 工作流（Workflow）聚合。
//
// 工作流是一张有向无环图（DAG），由 Node 与 Edge 组成；Engine 按 BaguaMode 选
// 用不同拓扑/调度算法执行。本文件只定义数据结构与静态校验（环、孤立节点），
// 不涉及运行时调度（由 application/workflow 实现）。
package model

import (
	"errors"
	"fmt"
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
// 一份工作流 = 节点集合 + 边集合 + 入口节点。Validate 负责静态校验（环/孤立），
// 真正执行由 Engine 解析 Workflow + BaguaMode 后调度。
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
