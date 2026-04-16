// 八卦阵 - 工作流编排引擎
// 天覆、地载、风扬、云垂、龙飞、虎翼、鸟翔、蛇蟠

package bagua

import (
	"context"
	"fmt"
	"sync"

	"github.com/google/uuid"
	"github.com/zhuge/kongming/pkg/cmd_center"
)

// BaguaMode 八卦阵模式
type BaguaMode string

const (
	Tiangai   BaguaMode = "tiangai"   // 天覆阵 - 并行全攻
	Dizai     BaguaMode = "dizai"     // 地载阵 - 顺序执行
	Fengyang  BaguaMode = "fengyang"  // 风扬阵 - 快速响应
	Yunzhui   BaguaMode = "yunzhui"  // 云垂阵 - 容错重试
	Longfei   BaguaMode = "longfei"   // 龙飞阵 - 动态调度
	Huyi      BaguaMode = "huyi"      // 虎翼阵 - 条件分支
	Niaoxiang BaguaMode = "niaoxiang" // 鸟翔阵 - 扇形扩散
	Shepan    BaguaMode = "shepan"    // 蛇蟠阵 - 循环迭代
)

// NodeType 节点类型
type NodeType string

const (
	NodeStart    NodeType = "start"
	NodeEnd      NodeType = "end"
	NodeLLM      NodeType = "llm"
	NodeTool     NodeType = "tool"
	NodeCondition NodeType = "condition"
	NodeLoop     NodeType = "loop"
	NodeParallel NodeType = "parallel"
	NodeWait     NodeType = "wait"
)

// Node 工作流节点
type Node struct {
	ID       string                 `json:"id" yaml:"id"`
	Type     NodeType              `json:"type" yaml:"type"`
	Name     string                 `json:"name" yaml:"name"`
	Config   map[string]interface{} `json:"config" yaml:"config"`
	Inputs   []string               `json:"inputs" yaml:"inputs"`
	Outputs  []string              `json:"outputs" yaml:"outputs"`
	Position Position               `json:"position" yaml:"position"`
}

// Position 节点位置（用于可视化）
type Position struct {
	X float64 `json:"x" yaml:"x"`
	Y float64 `json:"y" yaml:"y"`
}

// Edge 工作流边
type Edge struct {
	ID        string `json:"id" yaml:"id"`
	From      string `json:"from" yaml:"from"`
	To        string `json:"to" yaml:"to"`
	Label     string `json:"label,omitempty" yaml:"label,omitempty"`
	Condition string `json:"condition,omitempty" yaml:"condition,omitempty"`
}

// Workflow 工作流定义
type Workflow struct {
	ID          string            `json:"id" yaml:"id"`
	Name        string            `json:"name" yaml:"name"`
	Description string            `json:"description" yaml:"description"`
	Mode        BaguaMode         `json:"mode" yaml:"mode"`
	Nodes       []Node            `json:"nodes" yaml:"nodes"`
	Edges       []Edge            `json:"edges" yaml:"edges"`
	Variables   map[string]string `json:"variables" yaml:"variables"`
}

// ExecutionContext 执行上下文
type ExecutionContext struct {
	WorkflowID  string
	RunID       string
	Variables   map[string]interface{}
	NodeStates  map[string]NodeState
	mu          sync.RWMutex
}

// NodeState 节点状态
type NodeState struct {
	Status    string      `json:"status"`
	Input     interface{} `json:"input"`
	Output    interface{} `json:"output"`
	Error     string      `json:"error,omitempty"`
	StartTime int64       `json:"start_time"`
	EndTime   int64       `json:"end_time"`
}

// Engine 八卦阵引擎
type Engine struct {
	workflows map[string]*Workflow
	nodes     map[NodeType]NodeExecutor
	mu        sync.RWMutex
}

// NodeExecutor 节点执行器接口
type NodeExecutor interface {
	Execute(ctx context.Context, node Node, ec *ExecutionContext) (*NodeState, error)
}

// NewEngine 创建八卦阵引擎
func NewEngine() *Engine {
	return &Engine{
		workflows: make(map[string]*Workflow),
		nodes:     make(map[NodeType]NodeExecutor),
	}
}

// RegisterWorkflow 注册工作流
func (e *Engine) RegisterWorkflow(wf *Workflow) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if err := e.validateWorkflow(wf); err != nil {
		return fmt.Errorf("工作流验证失败: %w", err)
	}
	if wf.ID == "" {
		wf.ID = uuid.New().String()
	}
	e.workflows[wf.ID] = wf
	return nil
}

// GetWorkflow 获取工作流
func (e *Engine) GetWorkflow(id string) (*Workflow, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()
	wf, exists := e.workflows[id]
	if !exists {
		return nil, fmt.Errorf("工作流不存在: %s", id)
	}
	return wf, nil
}

// Execute 执行工作流
func (e *Engine) Execute(ctx context.Context, workflowID string, inputs map[string]interface{}) (*ExecutionContext, error) {
	wf, err := e.GetWorkflow(workflowID)
	if err != nil {
		return nil, err
	}
	ec := &ExecutionContext{
		WorkflowID: workflowID,
		RunID:      uuid.New().String(),
		Variables:  inputs,
		NodeStates: make(map[string]NodeState),
	}
	switch wf.Mode {
	case Tiangai:
		return e.executeTiangai(ctx, wf, ec)
	case Dizai:
		return e.executeDizai(ctx, wf, ec)
	case Fengyang:
		ctx, cancel := context.WithTimeout(ctx, cmd_center.DefaultTimeout)
		defer cancel()
		return e.executeTiangai(ctx, wf, ec)
	case Yunzhui:
		return e.executeYunzhui(ctx, wf, ec)
	case Longfei:
		return e.executeLongfei(ctx, wf, ec)
	default:
		return e.executeDizai(ctx, wf, ec)
	}
}

// RegisterNodeExecutor 注册节点执行器
func (e *Engine) RegisterNodeExecutor(nodeType NodeType, executor NodeExecutor) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.nodes[nodeType] = executor
}

// ===== 八卦阵执行模式 =====

// 天覆阵 - 并行全攻
func (e *Engine) executeTiangai(ctx context.Context, wf *Workflow, ec *ExecutionContext) (*ExecutionContext, error) {
	graph := buildDAG(wf)
	levels := topologicalLevels(graph)
	for _, level := range levels {
		var wg sync.WaitGroup
		errChan := make(chan error, len(level))
		for _, nodeID := range level {
			wg.Add(1)
			go func(id string) {
				defer wg.Done()
				node := findNode(wf, id)
				if node == nil {
					errChan <- fmt.Errorf("节点不存在: %s", id)
					return
				}
				executor, exists := e.nodes[node.Type]
				if !exists {
					errChan <- nil
					return
				}
				state, err := executor.Execute(ctx, *node, ec)
				if err != nil {
					errChan <- err
					return
				}
				ec.mu.Lock()
				ec.NodeStates[id] = *state
				ec.mu.Unlock()
			}(nodeID)
		}
		wg.Wait()
		close(errChan)
		for err := range errChan {
			if err != nil {
				return ec, err
			}
		}
	}
	return ec, nil
}

// 地载阵 - 顺序执行
func (e *Engine) executeDizai(ctx context.Context, wf *Workflow, ec *ExecutionContext) (*ExecutionContext, error) {
	startNode := findStartNode(wf)
	if startNode == nil {
		return nil, fmt.Errorf("工作流缺少开始节点")
	}
	current := startNode
	visited := make(map[string]bool)
	for current != nil {
		if visited[current.ID] {
			return nil, fmt.Errorf("检测到循环: %s", current.ID)
		}
		visited[current.ID] = true
		executor, exists := e.nodes[current.Type]
		if exists {
			state, err := executor.Execute(ctx, *current, ec)
			if err != nil {
				return ec, fmt.Errorf("节点执行失败 %s: %w", current.ID, err)
			}
			ec.mu.Lock()
			ec.NodeStates[current.ID] = *state
			ec.mu.Unlock()
		}
		current = findNextNode(wf, current.ID, ec)
	}
	return ec, nil
}

// 风扬阵 - 快速响应（带超时）
func (e *Engine) executeFengyang(ctx context.Context, wf *Workflow, ec *ExecutionContext) (*ExecutionContext, error) {
	ctx, cancel := context.WithTimeout(ctx, cmd_center.DefaultTimeout)
	defer cancel()
	return e.executeTiangai(ctx, wf, ec)
}

// 云垂阵 - 容错重试
func (e *Engine) executeYunzhui(ctx context.Context, wf *Workflow, ec *ExecutionContext) (*ExecutionContext, error) {
	maxRetries := 3
	for attempt := 1; attempt <= maxRetries; attempt++ {
		result, err := e.executeDizai(ctx, wf, ec)
		if err == nil {
			return result, nil
		}
		if attempt < maxRetries {
			fmt.Printf("云垂阵重试 %d/%d\n", attempt, maxRetries)
		}
	}
	return ec, fmt.Errorf("云垂阵重试%d次后仍失败", maxRetries)
}

// 龙飞阵 - 动态调度
func (e *Engine) executeLongfei(ctx context.Context, wf *Workflow, ec *ExecutionContext) (*ExecutionContext, error) {
	graph := buildDAG(wf)
	criticalPath := calculateCriticalPath(graph, wf)
	for _, nodeID := range criticalPath {
		node := findNode(wf, nodeID)
		if node == nil {
			continue
		}
		executor, exists := e.nodes[node.Type]
		if !exists {
			continue
		}
		state, err := executor.Execute(ctx, *node, ec)
		if err != nil {
			return ec, err
		}
		ec.mu.Lock()
		ec.NodeStates[nodeID] = *state
		ec.mu.Unlock()
	}
	return e.executeTiangai(ctx, wf, ec)
}

// ===== 辅助函数 =====

func (e *Engine) validateWorkflow(wf *Workflow) error {
	hasStart, hasEnd := false, false
	for _, node := range wf.Nodes {
		if node.Type == NodeStart {
			hasStart = true
		}
		if node.Type == NodeEnd {
			hasEnd = true
		}
	}
	if !hasStart {
		return fmt.Errorf("缺少开始节点")
	}
	if !hasEnd {
		return fmt.Errorf("缺少结束节点")
	}
	return nil
}

func buildDAG(wf *Workflow) map[string][]string {
	graph := make(map[string][]string)
	for _, edge := range wf.Edges {
		graph[edge.From] = append(graph[edge.From], edge.To)
	}
	for _, node := range wf.Nodes {
		if _, exists := graph[node.ID]; !exists {
			graph[node.ID] = []string{}
		}
	}
	return graph
}

func topologicalLevels(graph map[string][]string) [][]string {
	levels := make([][]string, 0)
	visited := make(map[string]bool)
	nodeList := make([]string, 0, len(graph))
	for node := range graph {
		nodeList = append(nodeList, node)
	}
	iterations := 0
	maxIterations := len(nodeList) * len(nodeList)
	for len(visited) < len(nodeList) && iterations < maxIterations {
		iterations++
		level := make([]string, 0)
		for _, node := range nodeList {
			if visited[node] {
				continue
			}
			ready := true
			for from, tos := range graph {
				for _, to := range tos {
					if to == node && !visited[from] {
						ready = false
						break
					}
				}
				if !ready {
					break
				}
			}
			if ready {
				level = append(level, node)
			}
		}
		for _, node := range level {
			visited[node] = true
		}
		if len(level) > 0 {
			levels = append(levels, level)
		}
	}
	return levels
}

func findNode(wf *Workflow, id string) *Node {
	for i := range wf.Nodes {
		if wf.Nodes[i].ID == id {
			return &wf.Nodes[i]
		}
	}
	return nil
}

func findStartNode(wf *Workflow) *Node {
	for i := range wf.Nodes {
		if wf.Nodes[i].Type == NodeStart {
			return &wf.Nodes[i]
		}
	}
	return nil
}

func findNextNode(wf *Workflow, currentID string, ec *ExecutionContext) *Node {
	for _, edge := range wf.Edges {
		if edge.From == currentID {
			return findNode(wf, edge.To)
		}
	}
	return nil
}

func calculateCriticalPath(graph map[string][]string, wf *Workflow) []string {
	path := make([]string, 0)
	visited := make(map[string]bool)
	var dfs func(node string)
	dfs = func(node string) {
		if visited[node] {
			return
		}
		visited[node] = true
		path = append(path, node)
		for _, next := range graph[node] {
			dfs(next)
		}
	}
	for node := range graph {
		hasIncoming := false
		for _, tos := range graph {
			for _, to := range tos {
				if to == node {
					hasIncoming = true
					break
				}
			}
		}
		if !hasIncoming {
			dfs(node)
		}
	}
	return path
}
