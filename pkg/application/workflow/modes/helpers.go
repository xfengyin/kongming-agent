// Package modes 八卦阵调度算法集合 - 共享工具。
//
// 提供 8 阵共同依赖的辅助函数：
//   - findNode              从 Workflow.Nodes 中按 ID 查找节点
//   - buildDAGAdapter       把 Workflow 转为邻接表（modes 内部用，与 application/workflow 同名）
//   - topologicalLevelsAdapter 拓扑分层（同上）
//
// 这些函数与 application/workflow 包内的同名函数语义一致；为了保持
// 「modes 是独立可测试单元」，这里提供简化版（不依赖外层 package）。
package modes

import "github.com/zhuge/kongming/pkg/domain/model"

// findNode 从 wf.Nodes 中按 ID 查找节点；返回 (节点指针, 是否找到)。
//
// 注意返回指针避免对 Node（值类型）做大量字段复制。
func findNode(wf *model.Workflow, id string) (*model.Node, bool) {
	for i := range wf.Nodes {
		if wf.Nodes[i].ID == id {
			return &wf.Nodes[i], true
		}
	}
	return nil, false
}

// buildDAGAdapter 把 Workflow 转为「节点 ID → 后继 ID 列表」的邻接表。
//
// 与 application/workflow.buildDAG 行为完全一致；此处独立实现以保证
// modes 包可独立测试、不强依赖 application/workflow。
func buildDAGAdapter(wf *model.Workflow) map[string][]string {
	graph := make(map[string][]string, len(wf.Nodes))
	for _, n := range wf.Nodes {
		if _, ok := graph[n.ID]; !ok {
			graph[n.ID] = nil
		}
	}
	for _, e := range wf.Edges {
		graph[e.From] = append(graph[e.From], e.To)
	}
	return graph
}

// topologicalLevelsAdapter 按入度法做 BFS 拓扑分层。
//
// 与 application/workflow.topologicalLevels 行为一致。
func topologicalLevelsAdapter(graph map[string][]string) [][]string {
	inDeg := make(map[string]int, len(graph))
	for _, succs := range graph {
		for _, to := range succs {
			inDeg[to]++
		}
	}
	for id := range graph {
		if _, ok := inDeg[id]; !ok {
			inDeg[id] = 0
		}
	}
	var levels [][]string
	for {
		var level []string
		for id, d := range inDeg {
			if d == 0 {
				level = append(level, id)
			}
		}
		if len(level) == 0 {
			break
		}
		for _, id := range level {
			delete(inDeg, id)
			for _, to := range graph[id] {
				inDeg[to]--
			}
		}
		levels = append(levels, level)
	}
	return levels
}
