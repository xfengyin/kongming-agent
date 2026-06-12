// Package modes 八卦阵调度算法集合 - Huyi 虎翼阵。
//
// Huyi = 条件分支：评估 Edge.Condition 决定下游走向。
//
// 极简条件评估器（够测试即可，**不实现完整 expression engine**）：
//   - 条件形如 `var.x=="value"`（精确等于字符串）
//   - 条件形如 `var.x!=`（不等于）
//   - 条件形如 `true` / `false`（字面量）
//   - 空 Condition = 无条件（永远为 true）
//
// 设计取舍：完整 DSL（CEL/expr-lang）依赖较大；当前实现覆盖 80% 业务场景。
// 后续如需扩展只需替换 evalCondition 函数。
package modes

import (
	"context"
	"fmt"
	"strings"

	"github.com/zhuge/kongming/pkg/domain/model"
	"github.com/zhuge/kongming/pkg/domain/port"
)

// Huyi 虎翼阵：按 Edge.Condition 选择下游分支。
//
// 行为：
//  1. 拓扑排序逐层执行
//  2. 当前节点的「下游」由每条出边 Condition 决定：评估为 true 的边才走
//  3. 当前实现按 Workflow.Nodes 顺序执行（与 Dizai 一致），但跳过没有「被选中入边」的目标节点
//
// 注：当前实现不真正构建动态子图（plan 文档允许的简化）；按层执行 + 跳过未命中节点即可。
func Huyi(ctx context.Context, wf *model.Workflow, ec *model.ExecutionContext,
	nodes map[model.NodeType]port.NodeExecutor) (*model.ExecutionContext, error) {
	// 预计算「每条边的命中结果」（true 的边才走其 To 节点）
	edgeHit := make(map[string]bool, len(wf.Edges))
	hitNodes := make(map[string]bool, len(wf.Nodes))
	for _, e := range wf.Edges {
		hit := evalCondition(e.Condition, ec)
		edgeHit[e.From+"->"+e.To] = hit
		if hit {
			hitNodes[e.To] = true
		}
	}

	for i := range wf.Nodes {
		node := wf.Nodes[i]
		// 节点是否「被上游任意一条边命中」？入口节点（无入边）也允许执行
		if !isReachable(node.ID, wf) {
			continue
		}
		// 如果节点有入边且全部未命中，则跳过（条件全 false）
		if hasIncoming(node.ID, wf) && !anyEdgeHitsTarget(node.ID, wf, edgeHit) {
			ec.SetNodeState(node.ID, model.NodeState{
				ID:     node.ID,
				Status: model.NodeStatusSkipped,
			})
			continue
		}
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

// evalCondition 极简条件评估（够测试即可）。
//
// 支持：
//   - ""        → true（无条件永远走）
//   - "true"    → true
//   - "false"   → false
//   - `var.x=="v"`  → 变量 x 字符串值等于 "v"
//   - `var.x!="v"`  → 变量 x 字符串值不等于 "v"
//   - `var.x`        → 变量 x 存在且非 nil/non-false
func evalCondition(cond string, ec *model.ExecutionContext) bool {
	cond = strings.TrimSpace(cond)
	if cond == "" || cond == "true" {
		return true
	}
	if cond == "false" {
		return false
	}
	// var.x=="v"
	if strings.HasPrefix(cond, "var.") {
		rest := strings.TrimPrefix(cond, "var.")
		// 拆分 == / !=
		if idx := strings.Index(rest, "=="); idx > 0 {
			key := strings.TrimSpace(rest[:idx])
			val := strings.Trim(strings.TrimSpace(rest[idx+2:]), `"`)
			if v, ok := ec.GetVar(key); ok {
				return fmt.Sprintf("%v", v) == val
			}
			return false
		}
		if idx := strings.Index(rest, "!="); idx > 0 {
			key := strings.TrimSpace(rest[:idx])
			val := strings.Trim(strings.TrimSpace(rest[idx+2:]), `"`)
			if v, ok := ec.GetVar(key); ok {
				return fmt.Sprintf("%v", v) != val
			}
			return true // 变量不存在 ⇒ 不等于 val
		}
		// 纯 var.x 存在性检查
		_, ok := ec.GetVar(strings.TrimSpace(rest))
		return ok
	}
	// 兜底：无法识别视为 false（fail-closed）
	return false
}

// isReachable 节点是否在 DAG 中（即使无入边也视为可达）。
func isReachable(id string, wf *model.Workflow) bool {
	for _, n := range wf.Nodes {
		if n.ID == id {
			return true
		}
	}
	return false
}

// hasIncoming 节点是否至少有一条入边。
func hasIncoming(id string, wf *model.Workflow) bool {
	for _, e := range wf.Edges {
		if e.To == id {
			return true
		}
	}
	return false
}

// anyEdgeHitsTarget 节点的「任意一条入边」是否被命中。
func anyEdgeHitsTarget(target string, wf *model.Workflow, edgeHit map[string]bool) bool {
	for _, e := range wf.Edges {
		if e.To != target {
			continue
		}
		if edgeHit[e.From+"->"+e.To] {
			return true
		}
	}
	return false
}
