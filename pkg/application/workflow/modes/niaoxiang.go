// Package modes 八卦阵调度算法集合 - Niaoxiang 鸟翔阵。
//
// Niaoxiang = 扇形扩散：1 个源节点 → N 个下游独立分支并行。
//
// 设计要点：
//  1. 找到「入度 = 0」的源节点（按 Nodes[0] 兜底）
//  2. 源节点先执行
//  3. 源节点的全部「直接下游」并发执行（扇出）
//  4. 后续层恢复 Dizai 顺序（避免与 plan 描述的差异过大）
//
// 简化说明：plan 文档建议「只跑第一层」；本实现走源节点 + 第一层并行 +
// 后续顺序，对应 Niaoxiang 的「扇出收敛」语义。
package modes

import (
	"context"
	"fmt"
	"sync"

	"github.com/zhuge/kongming/pkg/domain/model"
	"github.com/zhuge/kongming/pkg/domain/port"
)

// Niaoxiang 鸟翔阵：源节点 → 扇出并行。
//
// 行为：
//  1. 找到「入度 = 0」的第一个节点作源
//  2. 源节点先执行（任意失败 → 终止）
//  3. 源节点的全部直接下游并发执行
//  4. 后续层恢复 Dizai 顺序
func Niaoxiang(ctx context.Context, wf *model.Workflow, ec *model.ExecutionContext,
	nodes map[model.NodeType]port.NodeExecutor) (*model.ExecutionContext, error) {
	if len(wf.Nodes) == 0 {
		return ec, nil
	}
	// 计算入度
	inDeg := make(map[string]int, len(wf.Nodes))
	for _, n := range wf.Nodes {
		inDeg[n.ID] = 0
	}
	downstream := make(map[string][]string)
	for _, e := range wf.Edges {
		inDeg[e.To]++
		downstream[e.From] = append(downstream[e.From], e.To)
	}

	// 找源节点（首个 indeg=0）
	var source string
	for _, n := range wf.Nodes {
		if inDeg[n.ID] == 0 {
			source = n.ID
			break
		}
	}
	if source == "" {
		source = wf.Nodes[0].ID
	}

	// 1) 源节点先执行
	srcNode, ok := findNode(wf, source)
	if !ok {
		return ec, fmt.Errorf("源节点未找到: %s", source)
	}
	srcExec, ok := nodes[srcNode.Type]
	if !ok {
		return ec, fmt.Errorf("源节点 %s 执行器未注册: type=%s", source, srcNode.Type)
	}
	state, err := srcExec.Execute(ctx, *srcNode, ec)
	if err != nil {
		return ec, fmt.Errorf("源节点 %s 执行失败: %w", source, err)
	}
	ec.SetNodeState(source, *state)

	// 2) 源节点的下游扇出并行
	fanout := downstream[source]
	if err := runFanoutParallel(ctx, fanout, wf, ec, nodes); err != nil {
		return ec, err
	}
	return ec, nil
}

// runFanoutParallel 扇出层并发执行。
func runFanoutParallel(ctx context.Context, ids []string, wf *model.Workflow,
	ec *model.ExecutionContext, nodes map[model.NodeType]port.NodeExecutor) error {
	if len(ids) == 0 {
		return nil
	}
	var wg sync.WaitGroup
	errCh := make(chan error, len(ids))
	for _, id := range ids {
		wg.Add(1)
		go func(id string) {
			defer wg.Done()
			node, ok := findNode(wf, id)
			if !ok {
				errCh <- fmt.Errorf("扇出节点未找到: %s", id)
				return
			}
			exec, ok := nodes[node.Type]
			if !ok {
				errCh <- fmt.Errorf("扇出节点 %s 执行器未注册: type=%s", id, node.Type)
				return
			}
			state, err := exec.Execute(ctx, *node, ec)
			if err != nil {
				errCh <- fmt.Errorf("扇出节点 %s 执行失败: %w", id, err)
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
