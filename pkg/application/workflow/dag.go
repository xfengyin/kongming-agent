// Package workflow 工作流应用层 - DAG 拓扑工具。
//
// 本文件聚焦于「静态 DAG → 调度原语」的转换：
//   - buildDAG          从 Workflow 构造邻接表（后继映射）
//   - topologicalLevels 按 BFS 把节点分成「可并行层」，供 Tiangai/Longfei 等
//     并行阵直接按层并发
//
// 设计要点：
//  1. 不做环检测（由 model.Workflow.Validate 负责；这里假定传入合法 DAG）
//  2. 算法复杂度 O(V+E)，适合中小规模工作流（百节点以内）
//  3. 不依赖任何第三方库，零外部依赖
package workflow

import "github.com/zhuge/kongming/pkg/domain/model"

// buildDAG 把 Workflow 转换为「节点 ID → 后继节点 ID 列表」的邻接表。
//
// 行为细节：
//  1. 每条 Edge.From → Edge.To 形成一条有向边
//  2. 出现在 Edges 中但不在 Nodes 中的端点被忽略（Validate 已保证合法性）
//  3. 出现在 Nodes 中但没有任何出边的节点也出现在结果中（值为 nil），
//     便于后续 BFS/拓扑层判定「汇点节点」
//
// 时间复杂度 O(V + E)。
//
// 单元测试覆盖：TestBuildDAG_Linear / TestBuildDAG_Diamond / TestBuildDAG_Empty。
func buildDAG(wf *model.Workflow) map[string][]string {
	// 预分配容量：节点数为上界
	graph := make(map[string][]string, len(wf.Nodes))

	// 第一遍：把所有节点都登记进邻接表（哪怕暂时没有出边）
	for _, n := range wf.Nodes {
		if _, ok := graph[n.ID]; !ok {
			graph[n.ID] = nil
		}
	}

	// 第二遍：按 Edge 填充后继
	for _, e := range wf.Edges {
		graph[e.From] = append(graph[e.From], e.To)
	}
	return graph
}

// topologicalLevels 按 BFS（入度法）把 DAG 节点分成「可并行层」。
//
// 每一层内的节点相互独立（无依赖关系），可以并发执行（Tiangai 阵核心算法）。
// 每一层顺序执行（前一层全部完成后才进入下一层），保证依赖正确性。
//
// 返回 [][]string：外层是层序号（从 0 开始 = 最顶层入口），内层是该层节点 ID 列表。
//
// 实现细节：使用「剩余入度」而非 visited 集合，避免对同一节点重复计数。
// 时间复杂度 O(V + E)，空间 O(V)。
//
// 单元测试覆盖：TestTopologicalLevels_Linear / TestTopologicalLevels_Diamond /
// TestTopologicalLevels_Single。
func topologicalLevels(graph map[string][]string) [][]string {
	// 计算每个节点的入度（基于「反向邻接表」）
	inDegree := make(map[string]int, len(graph))
	for _, succs := range graph {
		for _, to := range succs {
			inDegree[to]++
		}
	}
	// 节点没有出现在 inDegree 中 → 入度默认为 0
	for id := range graph {
		if _, ok := inDegree[id]; !ok {
			inDegree[id] = 0
		}
	}

	var levels [][]string
	// 反复挑选「当前入度 = 0」的节点入下一层；挑完后再把这些节点的后继入度 -1
	for {
		var level []string
		for id, d := range inDegree {
			if d == 0 {
				level = append(level, id)
			}
		}
		if len(level) == 0 {
			break
		}
		// 把这一层节点标记为「已消费」（删除以避免重复挑选）
		// 同时让它们的后继入度 -1
		for _, id := range level {
			delete(inDegree, id)
			for _, to := range graph[id] {
				inDegree[to]--
			}
		}
		levels = append(levels, level)
	}
	return levels
}
