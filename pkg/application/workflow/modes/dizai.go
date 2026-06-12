// Package modes 八卦阵调度算法集合 - Dizai 地载阵。
//
// Dizai = 顺序执行：按节点出现顺序逐个执行，前一节点成功才执行后一节点。
// 是 8 阵中的「默认兜底」与「最简单调度」实现。
//
// 与 Tiangai 区别：完全串行，不并发；用于强顺序语义场景（如状态机迁移、事务链）。
package modes

import (
	"context"
	"fmt"

	"github.com/zhuge/kongming/pkg/domain/model"
	"github.com/zhuge/kongming/pkg/domain/port"
)

// Dizai 地载阵：按 Workflow.Nodes 顺序逐个执行。
//
// 行为：
//  1. 严格按 Nodes 切片的物理顺序执行（依赖 Workflow 自身的 DAG 顺序合法性）
//  2. 任一节点返回 error → 立即终止，返回该 error
//  3. 成功时所有节点状态写入 ec.NodeStates
//
// 时间复杂度 O(N)，无并发开销。
func Dizai(ctx context.Context, wf *model.Workflow, ec *model.ExecutionContext,
	nodes map[model.NodeType]port.NodeExecutor) (*model.ExecutionContext, error) {
	for i := range wf.Nodes {
		node := wf.Nodes[i]
		exec, ok := nodes[node.Type]
		if !ok {
			return ec, fmt.Errorf("节点 %s 的执行器未注册: type=%s", node.ID, node.Type)
		}
		state, err := exec.Execute(ctx, node, ec)
		if err != nil {
			return ec, fmt.Errorf("节点 %s 执行失败: %w", node.ID, err)
		}
		ec.SetNodeState(node.ID, *state)
	}
	return ec, nil
}
