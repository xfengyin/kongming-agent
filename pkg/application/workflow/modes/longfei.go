// Package modes 八卦阵调度算法集合 - Longfei 龙飞阵。
//
// Longfei = 「关键路径优先」调度：
//  1. 拓扑排序后第一层（入口层）并发执行
//  2. 后续层按 Dizai 顺序执行
//
// 这是简化版实现（plan 文档建议的方案），真实 critical path 需要在每个分支
// 动态选择最长链；本版本假设入口层后的依赖已线性化，足够测试场景。
//
// 与 Tiangai 区别：Tiangai 整图全并行；Longfei 强调「入口冲刺 + 后续稳态」。
package modes

import (
	"context"
	"fmt"
	"sync"

	"github.com/zhuge/kongming/pkg/domain/model"
	"github.com/zhuge/kongming/pkg/domain/port"
)

// Longfei 龙飞阵：入口层 Tiangai 并行 + 后续 Dizai 顺序。
//
// 行为：
//  1. 拓扑分层后第一层用 Tiangai 并行
//  2. 剩余层按 Dizai 顺序
//  3. 任一层失败立即终止
func Longfei(ctx context.Context, wf *model.Workflow, ec *model.ExecutionContext,
	nodes map[model.NodeType]port.NodeExecutor) (*model.ExecutionContext, error) {
	graph := buildDAGAdapter(wf)
	levels := topologicalLevelsAdapter(graph)
	if len(levels) == 0 {
		return ec, nil
	}

	// 第 1 层：Tiangai 并行
	if err := runLevelParallelLongfei(ctx, levels[0], wf, ec, nodes); err != nil {
		return ec, err
	}

	// 后续层：Dizai 顺序
	for _, level := range levels[1:] {
		if err := runLevelSequential(ctx, level, wf, ec, nodes); err != nil {
			return ec, err
		}
	}
	return ec, nil
}

// runLevelParallelLongfei Longfei 专用并行层执行（独立于 Tiangai 减少耦合）。
func runLevelParallelLongfei(ctx context.Context, level []string, wf *model.Workflow,
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
	for e := range errCh {
		if e != nil {
			return e
		}
	}
	return nil
}

// runLevelSequential 单层顺序执行（用于 Longfei 第二层起）。
func runLevelSequential(ctx context.Context, level []string, wf *model.Workflow,
	ec *model.ExecutionContext, nodes map[model.NodeType]port.NodeExecutor) error {
	for _, id := range level {
		node, ok := findNode(wf, id)
		if !ok {
			return fmt.Errorf("节点未找到: %s", id)
		}
		exec, ok := nodes[node.Type]
		if !ok {
			return fmt.Errorf("节点 %s 的执行器未注册: type=%s", id, node.Type)
		}
		state, err := exec.Execute(ctx, *node, ec)
		if err != nil {
			return fmt.Errorf("节点 %s 执行失败: %w", id, err)
		}
		ec.SetNodeState(id, *state)
	}
	return nil
}
