// Package modes 八卦阵调度算法集合 - Fengyang 风扬阵。
//
// Fengyang = Dizai（顺序）+ 全局超时（30s context）。
// 用于「快进快出」场景：宁可超时失败也不要拖死整条链路。
//
// 超时实现：
//   - 用 workflow 级 ctx 派生 30s context
//   - 节点执行器内部应响应 ctx.Done()（port.NodeExecutor 契约）
//   - 整体超时返回 context.DeadlineExceeded 包装错误
package modes

import (
	"context"
	"fmt"
	"time"

	"github.com/zhuge/kongming/pkg/domain/model"
	"github.com/zhuge/kongming/pkg/domain/port"
)

// fengyangDefaultTimeout Fengyang 阵默认全局超时（30 秒）。
//
// 设计权衡：30s 足够大多数 LLM/工具调用；超时由调用方按需通过 ctx
// 覆盖（外层 ctx 优先级高于内部 WithTimeout）。
const fengyangDefaultTimeout = 30 * time.Second

// Fengyang 风扬阵：Dizai + 30s 全局超时。
//
// 行为：
//  1. 派生 30s context（外层 ctx 取消仍会立即终止）
//  2. 沿用 Dizai 的顺序执行语义
//  3. 超时后返回带 context.DeadlineExceeded 包装的错误
func Fengyang(ctx context.Context, wf *model.Workflow, ec *model.ExecutionContext,
	nodes map[model.NodeType]port.NodeExecutor) (*model.ExecutionContext, error) {
	timedCtx, cancel := context.WithTimeout(ctx, fengyangDefaultTimeout)
	defer cancel()

	for i := range wf.Nodes {
		// 先检查 ctx 状态：被外层取消 / 已被超时 都不应继续
		if err := timedCtx.Err(); err != nil {
			return ec, fmt.Errorf("fengyang 全局超时/取消: %w", err)
		}
		node := wf.Nodes[i]
		exec, ok := nodes[node.Type]
		if !ok {
			return ec, fmt.Errorf("节点 %s 的执行器未注册: type=%s", node.ID, node.Type)
		}
		state, err := exec.Execute(timedCtx, node, ec)
		if err != nil {
			return ec, fmt.Errorf("节点 %s 执行失败: %w", node.ID, err)
		}
		ec.SetNodeState(node.ID, *state)
	}
	return ec, nil
}
