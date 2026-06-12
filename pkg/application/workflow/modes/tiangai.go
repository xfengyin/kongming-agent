// Package modes 八卦阵调度算法集合。
//
// 本文件实现「Tiangai 天盖阵」：按 DAG 拓扑层级并发执行，每层内节点并行。
//
// 设计要点：
//  1. 一层全部完成后才进入下一层（保留依赖语义）
//  2. 遇第一个 error 立即终止本层（其它 goroutine 通过 ctx/errCh 退出）
//  3. 使用 errCh 容量 = 本层节点数，避免发送方阻塞
//  4. NodeStates 由 ExecutionContext 内置锁保护，可并发写
package modes

import (
	"context"
	"fmt"
	"sync"

	"github.com/zhuge/kongming/pkg/domain/model"
	"github.com/zhuge/kongming/pkg/domain/port"
)

// Tiangai 天盖阵：按 DAG 拓扑层级并行执行。
//
// 参数：
//   - ctx         工作流级 context（用于外部取消 / 整体超时）
//   - wf          待执行工作流
//   - ec          执行上下文（变量 + 节点状态聚合点）
//   - nodes       NodeType → NodeExecutor 注册表
//
// 返回：执行后的 ExecutionContext 与首个非 nil error（任一层失败即终止）。
func Tiangai(ctx context.Context, wf *model.Workflow, ec *model.ExecutionContext,
	nodes map[model.NodeType]port.NodeExecutor) (*model.ExecutionContext, error) {
	graph := buildDAGAdapter(wf)
	levels := topologicalLevelsAdapter(graph)

	for _, level := range levels {
		if err := runLevelParallel(ctx, level, wf, ec, nodes); err != nil {
			return ec, err
		}
	}
	return ec, nil
}

// runLevelParallel 在「单层」内并发执行所有节点，遇首个 error 终止。
func runLevelParallel(ctx context.Context, level []string, wf *model.Workflow,
	ec *model.ExecutionContext, nodes map[model.NodeType]port.NodeExecutor) error {
	if len(level) == 0 {
		return nil
	}
	var wg sync.WaitGroup
	errCh := make(chan error, len(level))

	for _, id := range level {
		wg.Add(1)
		go func(id string) {
			defer wg.Done()
			node, ok := findNode(wf, id)
			if !ok {
				errCh <- fmt.Errorf("节点未找到: %s", id)
				return
			}
			exec, ok := nodes[node.Type]
			if !ok {
				errCh <- fmt.Errorf("节点 %s 的执行器未注册: type=%s", id, node.Type)
				return
			}
			state, err := exec.Execute(ctx, *node, ec)
			if err != nil {
				errCh <- fmt.Errorf("节点 %s 执行失败: %w", id, err)
				return
			}
			ec.SetNodeState(id, *state)
		}(id)
	}

	wg.Wait()
	close(errCh)

	// 收集错误：返回首个非 nil error（保持确定性）
	for e := range errCh {
		if e != nil {
			return e
		}
	}
	return nil
}
