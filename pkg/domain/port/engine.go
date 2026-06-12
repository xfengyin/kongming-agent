// Package port 定义领域层端口（接口）契约。
//
// engine.go 聚焦「工作流引擎」端口：Engine 抽象工作流执行入口，NodeExecutor
// 抽象单个节点的执行行为。两类接口由 application/workflow.Runner 整体实现，
// 由具体节点（LLM / Tool / Branch / ...）以 SPI 方式注入，符合「接口隔离 +
// 依赖倒置」原则。
package port

import (
	"context"

	"github.com/zhuge/kongming/pkg/domain/model"
)

// Engine 是工作流引擎的核心端口。
//
// 设计要点：
//  1. 4 个方法按「注册-查询-执行」三类职责正交切分（接口隔离）
//  2. RegisterWorkflow 不立即 Validate（由 Engine 实现决定；Runner 内部校验）
//  3. Execute 返回 ExecutionContext，调用方按需读取 NodeStates / Variables
//  4. RegisterNodeExecutor 允许动态注入新节点类型（插件化 / 热更新）
type Engine interface {
	// RegisterWorkflow 注册一个工作流到引擎。
	// 实现方负责静态校验（环/孤立/必含 start-end），返回错误时工作流未被注册。
	// 同一 ID 重复注册视为覆盖（幂等）。
	RegisterWorkflow(wf *model.Workflow) error

	// GetWorkflow 按 ID 查询工作流。
	// 返回 (nil, ErrNotFound) 表示不存在；调用方据此做存在性检查。
	GetWorkflow(id string) (*model.Workflow, error)

	// Execute 按工作流 ID 执行一次完整流程。
	// inputs 作为初始变量注入 ExecutionContext.Variables；ctx 用于超时/取消/traceId。
	// 返回 ExecutionContext（含各节点 NodeStates），用于审计/重放/调试。
	Execute(ctx context.Context, id string, inputs map[string]any) (*model.ExecutionContext, error)

	// RegisterNodeExecutor 注册一个节点执行器。
	// 同一类型重复注册视为覆盖（幂等）；类型为 NodeType（如 NodeStart/NodeAction）。
	// 典型调用：Runner 内置 start/end，外部注入 LLM/Tool 等。
	RegisterNodeExecutor(t model.NodeType, exec NodeExecutor)
}

// NodeExecutor 是单个节点的执行器契约。
//
// 单一职责：一个实现只负责一种 NodeType（start/end/action/branch/loop）。
// Execute 在 Engine 派发时被调用，Engine 不感知具体执行细节（依赖倒置）。
//
// 错误语义：返回 error 即视为失败，Engine 据此触发重试/熔断/短路等。
// 状态写入：返回的 NodeState 由 Engine 写入 ExecutionContext.NodeStates。
type NodeExecutor interface {
	// Execute 执行单个节点。
	//  - ctx   来自 Engine 的派发上下文（已附加超时/traceId）
	//  - node  当前节点（只读）
	//  - ec    工作流执行上下文（可读：Variables/NodeStates；可写：SetVar）
	//
	// 返回值：
	//  - state.Status = NodeStatusOK + nil error     → 成功
	//  - state.Status = NodeStatusFailed + non-nil error → 失败
	Execute(ctx context.Context, node model.Node, ec *model.ExecutionContext) (*model.NodeState, error)
}
