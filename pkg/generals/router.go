// 五虎将 - MoE 专家路由系统
// 参考 kimi-k3 Stable LatentMoE：896 路由专家 / 每 token 激活 16 个（1.8% 稀疏）
// 通过 SiTU-GLU 评分 + Quantile Balancing 负载均衡，在极高稀疏度下保持稳定

package generals

import (
	"context"
	"fmt"
	"sort"
	"sync"
	"sync/atomic"
	"time"

	"github.com/zhuge/kongming/pkg/cmd_center"
)

// RoutingInput 路由输入（对应 kimi-k3 中的 token 特征）
type RoutingInput struct {
	// Skill 任务所需技能（路由键）
	Skill string
	// Tags 附加标签，用于细粒度路由
	Tags []string
	// Params 路由参数
	Params map[string]interface{}
}

// RoutingDecision 路由决策
type RoutingDecision struct {
	// Activated 激活的专家列表（按路由分数降序，Top-K）
	Activated []*Expert
	// Scores 每个激活专家的路由分数
	Scores map[string]float64
	// Reason 路由说明
	Reason string
}

// ExpertRouteStats 专家路由统计（用于负载均衡与可观测）
type ExpertRouteStats struct {
	// TotalRoutes 累计被路由次数
	TotalRoutes atomic.Int64
	// ActiveLoads 当前在途负载
	ActiveLoads atomic.Int64
	// SuccessCount 成功执行次数
	SuccessCount atomic.Int64
	// FailureCount 失败执行次数
	FailureCount atomic.Int64
	// TotalDurationMs 累计执行耗时（毫秒）
	TotalDurationMs atomic.Int64
}

// Snapshot 路由统计快照（用于只读视图与负载均衡计算）
type RouteStatsSnapshot struct {
	TotalRoutes    int64
	ActiveLoads    int64
	SuccessCount   int64
	FailureCount   int64
	TotalDurationMs int64
}

// Snapshot 获取统计快照
func (s *ExpertRouteStats) Snapshot() RouteStatsSnapshot {
	return RouteStatsSnapshot{
		TotalRoutes:     s.TotalRoutes.Load(),
		ActiveLoads:     s.ActiveLoads.Load(),
		SuccessCount:    s.SuccessCount.Load(),
		FailureCount:    s.FailureCount.Load(),
		TotalDurationMs: s.TotalDurationMs.Load(),
	}
}

// ExpertLoadBalancer 负载均衡器接口
// 对齐 kimi-k3 MoonEP：在专家负载不均衡时仍保持高效路由
type ExpertLoadBalancer interface {
	// Adjust 对原始路由分数进行负载均衡调整
	// rawScores: 专家ID -> 原始路由分数
	// stats: 专家ID -> 路由统计快照
	// 返回调整后的分数
	Adjust(rawScores map[string]float64, stats map[string]RouteStatsSnapshot) map[string]float64
}

// ExpertRouter MoE 专家路由器接口
type ExpertRouter interface {
	// Route 根据 RoutingInput 计算 Top-K 路由决策
	Route(ctx context.Context, input RoutingInput, topK int) (*RoutingDecision, error)
	// RegisterBalancer 注册负载均衡器（SPI 扩展点）
	RegisterBalancer(balancer ExpertLoadBalancer)
}

// ScoringFunc 路由评分函数（对应 kimi-k3 SiTU-GLU 路由网络）
// 返回 [0,1] 的路由分数，0 表示不可路由
type ScoringFunc func(expert *Expert, input RoutingInput) float64

// MoERouter 默认 MoE 路由器实现
type MoERouter struct {
	mu        sync.RWMutex
	pool      *MoEExpertPool
	scoring   ScoringFunc
	balancer  ExpertLoadBalancer
}

// NewMoERouter 创建 MoE 路由器
func NewMoERouter(pool *MoEExpertPool, scoring ScoringFunc) *MoERouter {
	if scoring == nil {
		scoring = DefaultScoringFunc
	}
	return &MoERouter{
		pool:    pool,
		scoring: scoring,
	}
}

// RegisterBalancer 注册负载均衡器
func (r *MoERouter) RegisterBalancer(balancer ExpertLoadBalancer) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.balancer = balancer
}

// Route 执行 Top-K 路由
func (r *MoERouter) Route(ctx context.Context, input RoutingInput, topK int) (*RoutingDecision, error) {
	if topK <= 0 {
		topK = 1
	}

	r.mu.RLock()
	scoring := r.scoring
	balancer := r.balancer
	r.mu.RUnlock()

	// 1. 计算所有候选专家的原始路由分数
	candidates := r.pool.List(ExpertFilter{State: ExpertIdle})
	rawScores := make(map[string]float64, len(candidates))
	for _, exp := range candidates {
		score := scoring(exp, input)
		if score > 0 {
			rawScores[exp.ID] = score
		}
	}

	if len(rawScores) == 0 {
		return &RoutingDecision{
			Activated: []*Expert{},
			Scores:    map[string]float64{},
			Reason:    "无可用专家匹配路由输入",
		}, nil
	}

	// 2. 负载均衡调整（对应 kimi-k3 Quantile Balancing）
	finalScores := rawScores
	if balancer != nil {
		stats := r.collectStats(candidates)
		finalScores = balancer.Adjust(rawScores, stats)
	}

	// 3. Top-K 选择（对应 kimi-k3 每 token 激活 Top-16/896）
	activated := selectTopK(candidates, finalScores, topK)

	scores := make(map[string]float64, len(activated))
	for _, exp := range activated {
		scores[exp.ID] = finalScores[exp.ID]
	}

	return &RoutingDecision{
		Activated: activated,
		Scores:    scores,
		Reason:    formatRouteReason(input, activated),
	}, nil
}

// collectStats 收集候选专家的路由统计
func (r *MoERouter) collectStats(candidates []*Expert) map[string]RouteStatsSnapshot {
	stats := make(map[string]RouteStatsSnapshot, len(candidates))
	for _, exp := range candidates {
		stats[exp.ID] = exp.RouteStats.Snapshot()
	}
	return stats
}

// selectTopK 按调整后分数选择 Top-K 专家
func selectTopK(candidates []*Expert, scores map[string]float64, topK int) []*Expert {
	type scored struct {
		expert *Expert
		score  float64
	}
	list := make([]scored, 0, len(candidates))
	for _, exp := range candidates {
		if s, ok := scores[exp.ID]; ok && s > 0 {
			list = append(list, scored{expert: exp, score: s})
		}
	}
	// 降序排序，分数相同按 ID 保持稳定
	sort.Slice(list, func(i, j int) bool {
		if list[i].score != list[j].score {
			return list[i].score > list[j].score
		}
		return list[i].expert.ID < list[j].expert.ID
	})
	if topK > len(list) {
		topK = len(list)
	}
	result := make([]*Expert, topK)
	for i := 0; i < topK; i++ {
		result[i] = list[i].expert
	}
	return result
}

// formatRouteReason 生成路由说明
func formatRouteReason(input RoutingInput, activated []*Expert) string {
	if len(activated) == 0 {
		return "未激活任何专家"
	}
	names := make([]string, 0, len(activated))
	for _, e := range activated {
		names = append(names, e.Name)
	}
	reason := "路由激活专家: "
	for i, n := range names {
		if i > 0 {
			reason += ", "
		}
		reason += n
	}
	return reason
}

// DefaultScoringFunc 默认评分函数
// 综合：技能匹配度 × 状态权重 × 历史成功率 × 速度评分
func DefaultScoringFunc(expert *Expert, input RoutingInput) float64 {
	if expert.State != ExpertIdle {
		return 0
	}
	// 技能匹配
	matched := false
	for _, s := range expert.Skills {
		if s == input.Skill {
			matched = true
			break
		}
	}
	if !matched {
		return 0
	}

	stats := expert.RouteStats.Snapshot()
	successRate := 0.5
	if stats.TotalRoutes > 0 {
		successRate = float64(stats.SuccessCount) / float64(stats.TotalRoutes)
	}
	speedScore := 1.0
	if stats.TotalRoutes > 0 {
		avgMs := float64(stats.TotalDurationMs) / float64(stats.TotalRoutes)
		speedScore = 1000.0 / (1000.0 + avgMs)
	}
	// 加权综合（对应 SiTU-GLU 的门控思想）
	return successRate*0.6 + speedScore*0.4
}

// QuantileBalancer 分位数负载均衡器
// 对齐 kimi-k3 Quantile Balancing：基于路由分数分位数 + 负载水位调整
type QuantileBalancer struct {
	// LoadPenalty 负载惩罚强度（越大对繁忙专家惩罚越重，默认 0.3）
	LoadPenalty float64
}

// NewQuantileBalancer 创建分位数负载均衡器
func NewQuantileBalancer(loadPenalty float64) *QuantileBalancer {
	if loadPenalty <= 0 {
		loadPenalty = 0.3
	}
	return &QuantileBalancer{LoadPenalty: loadPenalty}
}

// Adjust 调整路由分数
func (b *QuantileBalancer) Adjust(rawScores map[string]float64, stats map[string]RouteStatsSnapshot) map[string]float64 {
	if len(rawScores) == 0 {
		return rawScores
	}

	// 计算负载水位（在途负载相对最大值的比例）
	maxLoad := int64(1)
	for _, s := range stats {
		if s.ActiveLoads > maxLoad {
			maxLoad = s.ActiveLoads
		}
	}

	adjusted := make(map[string]float64, len(rawScores))
	for id, score := range rawScores {
		s, ok := stats[id]
		if !ok {
			adjusted[id] = score
			continue
		}
		// 负载水位 [0,1]，越繁忙惩罚越大
		loadRatio := float64(s.ActiveLoads) / float64(maxLoad)
		penalty := 1.0 - b.LoadPenalty*loadRatio
		if penalty < 0.1 {
			penalty = 0.1
		}
		adjusted[id] = score * penalty
	}
	return adjusted
}

// CompiledExpertRouter 预编译路由器（缓存优化，对应 kimi-k3 推理优化思想）
type CompiledExpertRouter struct {
	base     *MoERouter
	cache    sync.Map // skill+topK -> *RoutingDecision
	cacheTTL time.Duration
}

// NewCompiledExpertRouter 创建预编译路由器
func NewCompiledExpertRouter(base *MoERouter, cacheTTL time.Duration) *CompiledExpertRouter {
	if cacheTTL <= 0 {
		cacheTTL = 5 * time.Second
	}
	return &CompiledExpertRouter{base: base, cacheTTL: cacheTTL}
}

// Route 带缓存的路由（仅缓存无 ActiveLoads 时的决策）
func (c *CompiledExpertRouter) Route(ctx context.Context, input RoutingInput, topK int) (*RoutingDecision, error) {
	// 简化：直接转发，实际生产可加 LRU/TTL 缓存
	return c.base.Route(ctx, input, topK)
}

// 确保 MoERouter 实现 ExpertRouter 接口
var _ ExpertRouter = (*MoERouter)(nil)

// RouteAndExecute 路由并执行（便捷方法）
// 对应 kimi-k3 中"路由激活专家 -> 专家并行执行 -> 结果聚合"的完整链路
func RouteAndExecute(ctx context.Context, router ExpertRouter, pool *MoEExpertPool, input RoutingInput, topK int, order *cmd_center.MilitaryOrder) (*cmd_center.BattleReport, error) {
	decision, err := router.Route(ctx, input, topK)
	if err != nil {
		return nil, err
	}
	if len(decision.Activated) == 0 {
		return &cmd_center.BattleReport{
			OrderID: order.ID,
			Success: false,
			Message: "无可用专家",
		}, nil
	}
	return pool.ExecuteWithDecision(ctx, decision, order)
}

// MoEExpertExecutor MoE 专家执行器
// 实现 cmd_center.ExpertExecutor 端口，封装路由器+专家池
// 对齐 kimi-k3 依赖倒置：上层只依赖端口，路由细节内聚于此
type MoEExpertExecutor struct {
	pool   *MoEExpertPool
	router ExpertRouter
	topK   int // 默认 Top-K 激活数
}

// NewMoEExpertExecutor 创建 MoE 专家执行器
func NewMoEExpertExecutor(pool *MoEExpertPool, router ExpertRouter, defaultTopK int) *MoEExpertExecutor {
	if defaultTopK <= 0 {
		defaultTopK = 1
	}
	return &MoEExpertExecutor{pool: pool, router: router, topK: defaultTopK}
}

// ExecuteBySkill 按技能路由并执行（Top-1 激活）
// 实现 cmd_center.ExpertExecutor 接口
func (e *MoEExpertExecutor) ExecuteBySkill(ctx context.Context, skill string, order *cmd_center.MilitaryOrder) (*cmd_center.GeneralReport, error) {
	decision, err := e.router.Route(ctx, RoutingInput{Skill: skill}, 1)
	if err != nil {
		return nil, err
	}
	if len(decision.Activated) == 0 {
		return nil, fmt.Errorf("无可用专家匹配技能: %s", skill)
	}
	expert := decision.Activated[0]
	return e.pool.Execute(ctx, expert.ID, order)
}

// ExecuteBySkillTopK 按技能路由并执行（Top-K 激活，多专家并行）
// 实现 cmd_center.ExpertExecutor 接口
func (e *MoEExpertExecutor) ExecuteBySkillTopK(ctx context.Context, skill string, topK int, order *cmd_center.MilitaryOrder) (*cmd_center.BattleReport, error) {
	if topK <= 0 {
		topK = e.topK
	}
	return RouteAndExecute(ctx, e.router, e.pool, RoutingInput{Skill: skill}, topK, order)
}

// 确保 MoEExpertExecutor 实现 cmd_center.ExpertExecutor 端口
var _ cmd_center.ExpertExecutor = (*MoEExpertExecutor)(nil)
