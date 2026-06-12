// Package workflow 工作流应用层 - start 内置节点执行器。
//
// start 节点是工作流的「入口锚点」，负责把外部输入注入 ExecutionContext。
// 当上游 commander/workflow-nesting 复用同一 ec 时，start 节点会保留已有变量
// （仅覆盖显式给定的字段）。
package workflow

import (
	"context"
	"time"

	"github.com/zhuge/kongming/pkg/domain/model"
)

// startExecutor 实现 port.NodeExecutor：把 Variables 注入到 ec.Variables。
//
// 行为：
//  1. 遍历 ec.Variables（由 Runner.Execute 在创建 ec 时从 inputs 复制）
//  2. 把每个 key 同步写入「节点结果」的 Output（便于下游节点通过 ec.Variables 读）
//  3. 写 NodeState.Status = ok
//
// 设计动机：start 节点无副作用、可重复执行；即使被 Tiangai 并发调度也安全
// （ec.SetVar 内部有锁保护）。
type startExecutor struct{}

// Execute 实现 port.NodeExecutor 接口。
func (s *startExecutor) Execute(_ context.Context, node model.Node, ec *model.ExecutionContext) (*model.NodeState, error) {
	now := time.Now()
	// 把已有 Variables「快照」到 Output（仅取 key 列表，值仍由 ec.Variables 持有）
	out := make(map[string]any, len(ec.Variables))
	for k, v := range ec.Variables {
		out[k] = v
	}
	// 写回以触发 SetVar 的锁路径（验证并发安全）；这步主要是「测试钩子」
	for k, v := range out {
		ec.SetVar(k, v)
	}
	return &model.NodeState{
		ID:          node.ID,
		Status:      model.NodeStatusOK,
		Output:      out,
		StartedAt:   now,
		CompletedAt: now,
	}, nil
}
