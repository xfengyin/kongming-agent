// Package modes 八卦阵调度算法集合 - Shepan 蛇蟠阵。
//
// Shepan = 循环迭代：节点含 `max_iterations` 参数，按该值循环执行。
//
// 设计要点：
//  1. 从 node.Params["max_iterations"] 读取 int 上限
//  2. 每次迭代用同一个 ctx，NodeState.CompletedAt 记最后一次
//  3. 循环变量通过 ExecutionContext.SetVar("__iter__", i) 暴露给节点
//  4. 达到上限 / 节点返回 error 即停
package modes

import (
	"context"
	"fmt"

	"github.com/zhuge/kongming/pkg/domain/model"
	"github.com/zhuge/kongming/pkg/domain/port"
)

// Shepan 蛇蟠阵：对每个节点按 max_iterations 循环执行。
//
// 行为：
//  1. 找到 type=NodeLoop 的节点；对每个 loop 节点按 Params["max_iterations"] 迭代
//  2. 其它节点按 Dizai 顺序执行一次
//  3. 循环体内可通过 ec.GetVar("__iter__") 读取当前迭代号（0-based）
//
// 返回最后一次执行的 NodeState。
func Shepan(ctx context.Context, wf *model.Workflow, ec *model.ExecutionContext,
	nodes map[model.NodeType]port.NodeExecutor) (*model.ExecutionContext, error) {
	for i := range wf.Nodes {
		node := wf.Nodes[i]
		exec, ok := nodes[node.Type]
		if !ok {
			return ec, fmt.Errorf("节点 %s 的执行器未注册: type=%s", node.ID, node.Type)
		}
		if node.Type == model.NodeLoop {
			max := maxIterations(node)
			var lastState *model.NodeState
			for iter := 0; iter < max; iter++ {
				if err := ctx.Err(); err != nil {
					return ec, fmt.Errorf("shepan 循环 ctx 取消: %w", err)
				}
				ec.SetVar("__iter__", iter)
				state, err := exec.Execute(ctx, node, ec)
				if err != nil {
					return ec, fmt.Errorf("shepan 节点 %s 第 %d 次迭代失败: %w", node.ID, iter, err)
				}
				lastState = state
			}
			if lastState != nil {
				ec.SetNodeState(node.ID, *lastState)
			}
			continue
		}
		// 非循环节点：Dizai 顺序一次
		state, err := exec.Execute(ctx, node, ec)
		if err != nil {
			return ec, fmt.Errorf("节点 %s 执行失败: %w", node.ID, err)
		}
		ec.SetNodeState(node.ID, *state)
	}
	return ec, nil
}

// maxIterations 从 Params["max_iterations"] 提取循环上限，默认 1。
//
// 支持 int / int32 / int64 / float64 四种数值类型（兼容 YAML/JSON 反序列化）。
func maxIterations(node model.Node) int {
	if node.Params == nil {
		return 1
	}
	v, ok := node.Params["max_iterations"]
	if !ok || v == nil {
		return 1
	}
	switch n := v.(type) {
	case int:
		if n < 1 {
			return 1
		}
		return n
	case int32:
		if n < 1 {
			return 1
		}
		return int(n)
	case int64:
		if n < 1 {
			return 1
		}
		return int(n)
	case float64:
		if n < 1 {
			return 1
		}
		return int(n)
	}
	return 1
}
