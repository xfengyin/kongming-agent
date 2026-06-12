// Package workflow 工作流应用层 - end 内置节点执行器。
//
// end 节点是工作流的「出口锚点」，负责把 ExecutionContext.Variables["__result__"]
// 写入节点结果，便于 commander/审计模块读最终结果。
package workflow

import (
	"context"
	"time"

	"github.com/zhuge/kongming/pkg/domain/model"
)

// endExecutor 实现 port.NodeExecutor：把 ec.Variables["__result__"] 汇总到 state。
//
// 行为：
//  1. 从 ec.Variables["__result__"] 读取「最终结果」（可能为 nil）
//  2. 写 NodeState.Status = ok + Output = result
//  3. 同时把 ec.Variables 全量快照写 Output（便于审计）
type endExecutor struct{}

// Execute 实现 port.NodeExecutor 接口。
func (e *endExecutor) Execute(_ context.Context, node model.Node, ec *model.ExecutionContext) (*model.NodeState, error) {
	now := time.Now()
	var result any
	if v, ok := ec.GetVar("__result__"); ok {
		result = v
	}
	// 审计快照
	snap := make(map[string]any, len(ec.Variables))
	for k, v := range ec.Variables {
		snap[k] = v
	}
	// 写回以触发锁路径（保持与 startExecutor 对称）
	for k, v := range snap {
		ec.SetVar(k, v)
	}
	return &model.NodeState{
		ID:          node.ID,
		Status:      model.NodeStatusOK,
		Output:      map[string]any{"result": result, "variables": snap},
		StartedAt:   now,
		CompletedAt: now,
	}, nil
}
