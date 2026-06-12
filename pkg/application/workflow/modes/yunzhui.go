// Package modes 八卦阵调度算法集合 - Yunzhui 云垂阵。
//
// Yunzhui = Dizai 包 retry：单节点失败时按 Node.Retries 次数重试。
//
// 设计要点：
//  1. 重试次数 = Node.Retries（不含首次执行），所以总尝试次数 = 1 + Retries
//  2. 每次重试沿用同一个 ctx（由调用方决定是否要退避：当前实现不内置退避）
//  3. 重试用尽后仍返回最后一次的 error
//  4. 不重试 start/end 节点（Retries=0），避免重做副作用
package modes

import (
	"context"
	"fmt"

	"github.com/zhuge/kongming/pkg/domain/model"
	"github.com/zhuge/kongming/pkg/domain/port"
)

// Yunzhui 云垂阵：Dizai + 单节点按 Retries 重试。
//
// 行为：
//  1. 按 Workflow.Nodes 顺序逐个执行
//  2. 每个节点失败时按 Node.Retries 次数重试（同步、不退避）
//  3. 最终失败时返回包装后的 error
func Yunzhui(ctx context.Context, wf *model.Workflow, ec *model.ExecutionContext,
	nodes map[model.NodeType]port.NodeExecutor) (*model.ExecutionContext, error) {
	for i := range wf.Nodes {
		node := wf.Nodes[i]
		exec, ok := nodes[node.Type]
		if !ok {
			return ec, fmt.Errorf("节点 %s 的执行器未注册: type=%s", node.ID, node.Type)
		}
		state, err := executeWithRetry(ctx, node, ec, exec)
		if err != nil {
			return ec, fmt.Errorf("节点 %s 重试 %d 次后仍失败: %w", node.ID, node.Retries, err)
		}
		ec.SetNodeState(node.ID, *state)
	}
	return ec, nil
}

// executeWithRetry 对单个节点执行最多 1+Retries 次。
func executeWithRetry(ctx context.Context, node model.Node, ec *model.ExecutionContext,
	exec port.NodeExecutor) (*model.NodeState, error) {
	var lastErr error
	for attempt := 0; attempt <= node.Retries; attempt++ {
		// 每次尝试前检查 ctx 状态
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		state, err := exec.Execute(ctx, node, ec)
		if err == nil {
			return state, nil
		}
		lastErr = err
	}
	return nil, lastErr
}
