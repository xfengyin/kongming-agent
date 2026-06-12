// Package workflow 工作流应用层 - Engine 端口实现（Runner）。
//
// Runner 是 application/workflow 的核心：实现 domain/port.Engine 接口，
// 持有「工作流注册表」与「节点执行器注册表」，按 Workflow.Mode 把执行派发到
// 8 阵对应的 modes/* 函数。
//
// 设计原则（对齐 12 条企业规则）：
//  1. 开闭原则：新增阵型只需 switch 一个 case，不改既有调度代码
//  2. 依赖倒置：依赖 port.Engine / port.NodeExecutor 抽象接口
//  3. 单一职责：Runner 只负责「注册 + 派发」；具体调度算法由 modes/* 实现
//  4. 线程安全：mu 保护 workflows 与 nodes 注册表
//  5. 幂等：RegisterWorkflow / RegisterNodeExecutor 重复注册视为覆盖
//  6. 可观测：每次 Execute 生成新 RunID，便于链路追踪
package workflow

import (
	"context"
	"fmt"
	"sync"

	"github.com/google/uuid"
	"go.uber.org/zap"

	"github.com/zhuge/kongming/pkg/application/workflow/modes"
	"github.com/zhuge/kongming/pkg/domain/model"
	"github.com/zhuge/kongming/pkg/domain/port"
)

// Runner 是工作流引擎的默认实现，实现 port.Engine。
type Runner struct {
	logger    *zap.Logger
	mu        sync.RWMutex
	workflows map[string]*model.Workflow
	nodes     map[model.NodeType]port.NodeExecutor
}

// NewRunner 构造 Runner 并注册内置 start/end 节点执行器。
//
// 构造完成后即可调用 RegisterWorkflow 注册业务工作流，
// 以及 RegisterNodeExecutor 注入业务节点类型（Action/Branch/Loop）。
func NewRunner(logger *zap.Logger) *Runner {
	if logger == nil {
		logger = zap.NewNop()
	}
	r := &Runner{
		logger:    logger,
		workflows: make(map[string]*model.Workflow),
		nodes:     make(map[model.NodeType]port.NodeExecutor),
	}
	// 注册内置节点
	r.RegisterNodeExecutor(model.NodeStart, &startExecutor{})
	r.RegisterNodeExecutor(model.NodeEnd, &endExecutor{})
	return r
}

// RegisterWorkflow 注册一个工作流到引擎。
//
// 流程：
//  1. 静态校验（model.Workflow.Validate）：环、孤立、节点引用
//  2. 必含校验：至少一个 start 节点 + 一个 end 节点（兜底 Engine 调度假设）
//  3. ID 为空时分配 UUID
//  4. 同一 ID 重复注册视为覆盖
func (r *Runner) RegisterWorkflow(wf *model.Workflow) error {
	if wf == nil {
		return fmt.Errorf("workflow 不能为空")
	}
	if err := wf.Validate(); err != nil {
		return fmt.Errorf("工作流静态校验失败: %w", err)
	}
	if err := validateStartEnd(wf); err != nil {
		return fmt.Errorf("工作流锚点校验失败: %w", err)
	}
	if wf.ID == "" {
		wf.ID = uuid.NewString()
	}
	r.mu.Lock()
	r.workflows[wf.ID] = wf
	r.mu.Unlock()
	r.logger.Info("工作流已注册",
		zap.String("workflow_id", wf.ID),
		zap.String("mode", string(wf.Mode)),
		zap.Int("nodes", len(wf.Nodes)))
	return nil
}

// GetWorkflow 按 ID 查询工作流；不存在时返回 error。
func (r *Runner) GetWorkflow(id string) (*model.Workflow, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	wf, ok := r.workflows[id]
	if !ok {
		return nil, fmt.Errorf("工作流未找到: %s", id)
	}
	return wf, nil
}

// RegisterNodeExecutor 注册一个 NodeType 的执行器；同类型重复注册视为覆盖。
func (r *Runner) RegisterNodeExecutor(t model.NodeType, exec port.NodeExecutor) {
	r.mu.Lock()
	r.nodes[t] = exec
	r.mu.Unlock()
}

// Execute 按 Workflow.Mode 分发到对应阵型；默认走 Dizai。
//
// 流程：
//  1. 查工作流
//  2. 构造 ExecutionContext（RunID/Variables/NodeStates）
//  3. switch wf.Mode → 调 modes.Xxx
//  4. 写回 ec.CompletedAt
func (r *Runner) Execute(ctx context.Context, id string, inputs map[string]any) (*model.ExecutionContext, error) {
	wf, err := r.GetWorkflow(id)
	if err != nil {
		return nil, err
	}
	if inputs == nil {
		inputs = map[string]any{}
	}
	ec := &model.ExecutionContext{
		WorkflowID: wf.ID,
		RunID:      uuid.NewString(),
		Variables:  copyVariables(inputs),
		NodeStates: make(map[string]model.NodeState),
		StartedAt:  nowFn(),
	}

	r.mu.RLock()
	// 拷贝 nodes 快照（避免在执行期间被并发注册覆盖）
	nodes := make(map[model.NodeType]port.NodeExecutor, len(r.nodes))
	for t, e := range r.nodes {
		nodes[t] = e
	}
	r.mu.RUnlock()

	r.logger.Debug("工作流开始执行",
		zap.String("workflow_id", wf.ID),
		zap.String("run_id", ec.RunID),
		zap.String("mode", string(wf.Mode)))

	out, execErr := dispatchMode(ctx, wf, ec, nodes)
	out.CompletedAt = nowFn()
	if execErr != nil {
		r.logger.Warn("工作流执行失败",
			zap.String("workflow_id", wf.ID),
			zap.String("run_id", ec.RunID),
			zap.Error(execErr))
		return out, execErr
	}
	r.logger.Info("工作流执行成功",
		zap.String("workflow_id", wf.ID),
		zap.String("run_id", ec.RunID))
	return out, nil
}

// dispatchMode 按 BaguaMode 派发到对应阵型。
//
// 注意：switch 默认走 Dizai，确保「未知 mode」不会 panic。
func dispatchMode(ctx context.Context, wf *model.Workflow, ec *model.ExecutionContext,
	nodes map[model.NodeType]port.NodeExecutor) (*model.ExecutionContext, error) {
	switch wf.Mode {
	case model.Tiangai:
		return modes.Tiangai(ctx, wf, ec, nodes)
	case model.Dizai:
		return modes.Dizai(ctx, wf, ec, nodes)
	case model.Fengyang:
		return modes.Fengyang(ctx, wf, ec, nodes)
	case model.Yunzhui:
		return modes.Yunzhui(ctx, wf, ec, nodes)
	case model.Longfei:
		return modes.Longfei(ctx, wf, ec, nodes)
	case model.Huyi:
		return modes.Huyi(ctx, wf, ec, nodes)
	case model.Niaoxiang:
		return modes.Niaoxiang(ctx, wf, ec, nodes)
	case model.Shepan:
		return modes.Shepan(ctx, wf, ec, nodes)
	default:
		// 零值/未知 mode → 默认顺序执行（兜底）
		return modes.Dizai(ctx, wf, ec, nodes)
	}
}

// validateStartEnd 校验工作流至少含一个 start 与一个 end 节点。
//
// 这是 application/workflow 的额外约束（model.Workflow.Validate 不强制）；
// 缺锚点会让 Tiangai/Niaoxiang 等「无 start 上下文」语义混乱。
func validateStartEnd(wf *model.Workflow) error {
	hasStart, hasEnd := false, false
	for _, n := range wf.Nodes {
		if n.Type == model.NodeStart {
			hasStart = true
		}
		if n.Type == model.NodeEnd {
			hasEnd = true
		}
	}
	if !hasStart {
		return fmt.Errorf("工作流必须包含至少一个 start 节点")
	}
	if !hasEnd {
		return fmt.Errorf("工作流必须包含至少一个 end 节点")
	}
	return nil
}

// copyVariables 浅拷贝一份 Variables（避免外部 map 被执行期修改影响）。
func copyVariables(in map[string]any) map[string]any {
	out := make(map[string]any, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}

// 编译期断言：Runner 实现 port.Engine。
var _ port.Engine = (*Runner)(nil)
