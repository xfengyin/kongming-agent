// 八卦阵 - 混合执行引擎
// 参考 kimi-k3 架构：
//   - KDA-like 轻量节点（3/4）：线性注意力思想，快速执行，无聚合
//   - MLA-like 重型节点（1/4）：全多头潜在注意力 + 输出门控，精确执行 + 残差聚合
//   - Attention Residuals (AttnRes)：节点从早期节点选择性检索表示（α 算子）
// 天覆、地载、风扬、云垂、龙飞、虎翼、鸟翔、蛇蟠 八种阵法为上层调度策略

package bagua

import (
	"context"
	"fmt"
	"sync"

	"github.com/google/uuid"
	"github.com/zhuge/kongming/pkg/cmd_center"
)

// BaguaMode 八卦阵模式（上层调度策略）
type BaguaMode string

const (
	Tiangai   BaguaMode = "tiangai"   // 天覆阵 - 并行全攻
	Dizai     BaguaMode = "dizai"     // 地载阵 - 顺序执行
	Fengyang  BaguaMode = "fengyang"  // 风扬阵 - 快速响应
	Yunzhui   BaguaMode = "yunzhui"   // 云垂阵 - 容错重试
	Longfei   BaguaMode = "longfei"   // 龙飞阵 - 动态调度
	Huyi      BaguaMode = "huyi"      // 虎翼阵 - 条件分支
	Niaoxiang BaguaMode = "niaoxiang" // 鸟翔阵 - 扇形扩散
	Shepan    BaguaMode = "shepan"    // 蛇蟠阵 - 循环迭代
)

// ExecutionMode 节点执行模式
// 对齐 kimi-k3 的 KDA(3/4) + Gated MLA(1/4) 混合注意力结构
type ExecutionMode string

const (
	// ModeKDA KDA-like 轻量执行模式
	// 对应 kimi-k3 Kimi Delta Attention：线性注意力，快速执行，不做残差聚合
	ModeKDA ExecutionMode = "kda"
	// ModeMLA MLA-like 重型执行模式
	// 对应 kimi-k3 Gated MLA：全多头潜在注意力 + 输出门控，精确执行 + 残差聚合
	ModeMLA ExecutionMode = "mla"
)

// ResidualSource AttnRes 残差源
// 对齐 kimi-k3 Attention Residuals 的 α 算子：从早期节点选择性检索表示
type ResidualSource struct {
	// NodeID 源节点 ID
	NodeID string `json:"node_id" yaml:"node_id"`
	// Alpha 检索权重（α 算子，[0,1]）
	Alpha float64 `json:"alpha" yaml:"alpha"`
}

// NodeType 节点类型
type NodeType string

const (
	NodeStart     NodeType = "start"
	NodeEnd       NodeType = "end"
	NodeLLM       NodeType = "llm"
	NodeTool      NodeType = "tool"
	NodeCondition NodeType = "condition"
	NodeLoop      NodeType = "loop"
	NodeParallel  NodeType = "parallel"
	NodeWait      NodeType = "wait"
)

// Node 工作流节点
type Node struct {
	ID      string                 `json:"id" yaml:"id"`
	Type    NodeType               `json:"type" yaml:"type"`
	Name    string                 `json:"name" yaml:"name"`
	Mode    ExecutionMode          `json:"mode" yaml:"mode"` // 执行模式（KDA/MLA），空则自动按 3:1 分配
	Config  map[string]interface{} `json:"config" yaml:"config"`
	Inputs  []string               `json:"inputs" yaml:"inputs"`
	Outputs []string               `json:"outputs" yaml:"outputs"`
	// ResidualSources AttnRes 残差源列表
	// 节点执行前从这些源节点检索表示，按 Alpha 加权聚合注入
	ResidualSources []ResidualSource `json:"residual_sources,omitempty" yaml:"residual_sources,omitempty"`
	Position        Position         `json:"position" yaml:"position"`
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

// NodeRepresentation 节点表示（对应 kimi-k3 中每层的隐藏表示）
type NodeRepresentation struct {
	Output  interface{}            `json:"output"`
	Meta    map[string]interface{} `json:"meta,omitempty"`
}

// NodeState 节点状态
type NodeState struct {
	Status        string             `json:"status"`
	Input         interface{}        `json:"input"`
	Output        interface{}        `json:"output"`
	Representation NodeRepresentation `json:"representation"` // 节点表示，供 AttnRes 检索
	Error         string             `json:"error,omitempty"`
	StartTime     int64              `json:"start_time"`
	EndTime       int64              `json:"end_time"`
	Mode          ExecutionMode      `json:"mode"` // 实际执行模式
}

// ExecutionContext 执行上下文
type ExecutionContext struct {
	WorkflowID string
	RunID      string
	Variables  map[string]interface{}
	NodeStates map[string]NodeState
	mu         sync.RWMutex
}

// GetNodeRepresentation 获取节点表示（供 AttnRes 检索，线程安全）
func (ec *ExecutionContext) GetNodeRepresentation(nodeID string) (NodeRepresentation, bool) {
	ec.mu.RLock()
	defer ec.mu.RUnlock()
	state, ok := ec.NodeStates[nodeID]
	if !ok {
		return NodeRepresentation{}, false
	}
	return state.Representation, true
}

// NodeExecutor 节点执行器接口
type NodeExecutor interface {
	Execute(ctx context.Context, node Node, ec *ExecutionContext) (*NodeState, error)
}

// Engine 八卦阵混合执行引擎
type Engine struct {
	workflows map[string]*Workflow
	nodes     map[NodeType]NodeExecutor
	mu        sync.RWMutex
}

// NewEngine 创建八卦阵引擎
func NewEngine() *Engine {
	return &Engine{
		workflows: make(map[string]*Workflow),
		nodes:     make(map[NodeType]NodeExecutor),
	}
}

// RegisterWorkflow 注册工作流
// 对齐 kimi-k3 3:1 混合比例：未显式指定 Mode 的节点自动按 3 KDA : 1 MLA 分配
func (e *Engine) RegisterWorkflow(wf *Workflow) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if err := e.validateWorkflow(wf); err != nil {
		return fmt.Errorf("工作流验证失败: %w", err)
	}
	if wf.ID == "" {
		wf.ID = uuid.New().String()
	}
	// 自动分配 3:1 KDA/MLA 模式（对齐 kimi-k3 KDA:MLA = 3:1）
	e.assignExecutionModes(wf)
	e.workflows[wf.ID] = wf
	return nil
}

// assignExecutionModes 自动分配执行模式（3:1 KDA/MLA）
// 对齐 kimi-k3：每 4 个执行节点中，3 个为 KDA 轻量模式，1 个为 MLA 重型模式
func (e *Engine) assignExecutionModes(wf *Workflow) {
	execIndex := 0
	for i := range wf.Nodes {
		node := &wf.Nodes[i]
		// start/end 节点不参与混合执行
		if node.Type == NodeStart || node.Type == NodeEnd {
			continue
		}
		if node.Mode != "" {
			// 已显式指定，保留
			continue
		}
		// 3:1 分配：第 4、8、12... 个执行节点为 MLA，其余为 KDA
		if (execIndex+1)%4 == 0 {
			node.Mode = ModeMLA
		} else {
			node.Mode = ModeKDA
		}
		execIndex++
	}
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

// Execute 执行工作流（混合调度）
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

// ===== AttnRes 残差聚合（对应 kimi-k3 α 算子）=====

// AttnResAggregation AttnRes 聚合结果
// 对齐 kimi-k3 Attention Residuals 的 α 算子输出：归一化权重 + 融合表示
type AttnResAggregation struct {
	// Sources 参与聚合的源节点（按 Alpha 归一化后的权重）
	Sources []AttnResSource `json:"sources"`
	// FusedOutput 融合后的表示（按归一化 Alpha 加权合并）
	FusedOutput interface{} `json:"fused_output"`
	// TotalAlpha 原始 Alpha 总和（归一化前）
	TotalAlpha float64 `json:"total_alpha"`
}

// AttnResSource 单个残差源的聚合信息
type AttnResSource struct {
	NodeID        string  `json:"node_id"`
	Alpha         float64 `json:"alpha"`          // 原始 α 权重
	NormalizedAlpha float64 `json:"normalized_alpha"` // 归一化后的权重
}

// residualCollected 已检索到的残差源（内部中间结构）
type residualCollected struct {
	source ResidualSource
	rep    NodeRepresentation
}

// gatherResiduals 收集并聚合残差表示（对应 kimi-k3 α 算子）
// 从节点的 ResidualSources 检索早期节点表示，按 Alpha 加权归一化后融合
// 返回 AttnResAggregation，供 MLA 节点作为门控输入
func gatherResiduals(node Node, ec *ExecutionContext) *AttnResAggregation {
	if len(node.ResidualSources) == 0 {
		return nil
	}

	// 第一遍：检索所有可用源节点的表示，累计 Alpha
	available := make([]residualCollected, 0, len(node.ResidualSources))
	totalAlpha := 0.0
	for _, src := range node.ResidualSources {
		if src.Alpha <= 0 {
			continue
		}
		rep, ok := ec.GetNodeRepresentation(src.NodeID)
		if !ok {
			continue
		}
		available = append(available, residualCollected{source: src, rep: rep})
		totalAlpha += src.Alpha
	}
	if totalAlpha <= 0 || len(available) == 0 {
		return nil
	}

	// 第二遍：归一化 Alpha，构造聚合结果
	sources := make([]AttnResSource, 0, len(available))
	for _, c := range available {
		sources = append(sources, AttnResSource{
			NodeID:          c.source.NodeID,
			Alpha:           c.source.Alpha,
			NormalizedAlpha: c.source.Alpha / totalAlpha,
		})
	}

	// 融合输出：对数值型输出做加权平均，非数值型做加权列表
	fused := fuseRepresentations(available, totalAlpha)

	return &AttnResAggregation{
		Sources:     sources,
		FusedOutput: fused,
		TotalAlpha:  totalAlpha,
	}
}

// fuseRepresentations 融合多个源节点表示
// 数值型字段做加权平均；其他字段以最高权重源为准
func fuseRepresentations(available []residualCollected, totalAlpha float64) interface{} {
	// 尝试提取 map[string]interface{} 类型输出做加权融合
	mapOutputs := make([]map[string]interface{}, 0, len(available))
	for _, c := range available {
		if m, ok := c.rep.Output.(map[string]interface{}); ok {
			// 注入 α 权重供下游识别
			weighted := make(map[string]interface{}, len(m))
			for k, v := range m {
				weighted[k] = v
			}
			weighted["_alpha"] = c.source.Alpha / totalAlpha
			mapOutputs = append(mapOutputs, weighted)
		}
	}
	if len(mapOutputs) == 0 {
		// 非数值型：返回按权重排序的源输出列表
		list := make([]map[string]interface{}, 0, len(available))
		for _, c := range available {
			list = append(list, map[string]interface{}{
				"node_id": c.source.NodeID,
				"alpha":   c.source.Alpha / totalAlpha,
				"output":  c.rep.Output,
			})
		}
		return list
	}
	// 数值型字段加权融合
	fused := make(map[string]interface{})
	// 收集所有数值型字段
	numFields := make(map[string][]float64)
	for _, m := range mapOutputs {
		for k, v := range m {
			if k == "_alpha" {
				continue
			}
			if num, ok := toFloat(v); ok {
				numFields[k] = append(numFields[k], num)
			}
		}
	}
	// 对每个数值字段做加权平均
	for field, values := range numFields {
		if len(values) != len(mapOutputs) {
			continue // 字段未在所有源出现，跳过加权
		}
		weightedSum := 0.0
		for i, m := range mapOutputs {
			alpha, _ := m["_alpha"].(float64)
			weightedSum += values[i] * alpha
		}
		fused[field] = weightedSum
	}
	fused["_fused_by"] = "attnres_alpha"
	fused["_source_count"] = len(mapOutputs)
	return fused
}

// toFloat 将 interface{} 转为 float64
func toFloat(v interface{}) (float64, bool) {
	switch n := v.(type) {
	case int:
		return float64(n), true
	case int64:
		return float64(n), true
	case float64:
		return n, true
	case float32:
		return float64(n), true
	}
	return 0, false
}

// executeNodeWithMix 混合执行单个节点
// KDA 模式：轻量执行，不做残差聚合
// MLA 模式：重型执行，先聚合 AttnRes 残差，再执行（对应 Gated MLA 的门控输入）
func (e *Engine) executeNodeWithMix(ctx context.Context, node Node, ec *ExecutionContext) (*NodeState, error) {
	executor, exists := e.nodes[node.Type]
	if !exists {
		// 无执行器的节点（如 start/end）直接跳过
		return &NodeState{
			Status: "skipped",
			Mode:   node.Mode,
		}, nil
	}

	// MLA 模式：先执行 AttnRes 残差聚合，注入到节点 Config（对应 Gated MLA 门控输入）
	if node.Mode == ModeMLA {
		if agg := gatherResiduals(node, ec); agg != nil {
			if node.Config == nil {
				node.Config = make(map[string]interface{})
			}
			node.Config["_attnres"] = agg
		}
	}

	state, err := executor.Execute(ctx, node, ec)
	if err != nil {
		return state, err
	}
	// 标记实际执行模式
	if state != nil {
		state.Mode = node.Mode
		// 构造节点表示，供后续节点 AttnRes 检索
		state.Representation = NodeRepresentation{
			Output: state.Output,
			Meta:   map[string]interface{}{"mode": string(node.Mode)},
		}
	}
	return state, nil
}

// ===== 八卦阵执行模式 =====

// 天覆阵 - 并行全攻（按拓扑层级并行，层内混合执行）
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
				state, err := e.executeNodeWithMix(ctx, *node, ec)
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

// 地载阵 - 顺序执行（混合 KDA/MLA 模式）
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
		state, err := e.executeNodeWithMix(ctx, *current, ec)
		if err != nil {
			return ec, fmt.Errorf("节点执行失败 %s: %w", current.ID, err)
		}
		ec.mu.Lock()
		ec.NodeStates[current.ID] = *state
		ec.mu.Unlock()
		current = findNextNode(wf, current.ID, ec)
	}
	return ec, nil
}

// 云垂阵 - 容错重试（KDA 快速重试，MLA 严格验证）
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

// 龙飞阵 - 动态调度（先关键路径，后并行补全）
func (e *Engine) executeLongfei(ctx context.Context, wf *Workflow, ec *ExecutionContext) (*ExecutionContext, error) {
	graph := buildDAG(wf)
	criticalPath := calculateCriticalPath(graph, wf)
	for _, nodeID := range criticalPath {
		node := findNode(wf, nodeID)
		if node == nil {
			continue
		}
		state, err := e.executeNodeWithMix(ctx, *node, ec)
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
