// 五虎将 - MoE 专家池
// 参考 kimi-k3 Stable LatentMoE：每个专家具备路由分数、容量、负载水位
// 关张赵马黄，各显神通；稀疏激活，负载均衡

package generals

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/zhuge/kongming/pkg/cmd_center"
)

// ExpertType 专家类型（对应 kimi-k3 中的专家分组）
type ExpertType string

const (
	ExpertGuanYu     ExpertType = "guanyu"     // 关羽 - 情报搜集专家
	ExpertZhangFei   ExpertType = "zhangfei"   // 张飞 - 数据工程专家
	ExpertZhaoYun    ExpertType = "zhaoyun"    // 赵云 - 分析可视化专家
	ExpertMaChao     ExpertType = "machao"     // 马超 - 报告撰写专家
	ExpertHuangZhong ExpertType = "huangzhong" // 黄忠 - 质量审核专家
)

// ExpertState 专家状态
type ExpertState int

const (
	ExpertIdle     ExpertState = iota // 待命
	ExpertBusy                        // 出征中
	ExpertResting                     // 休整中
	ExpertOffline                     // 离线
)

func (s ExpertState) String() string {
	switch s {
	case ExpertIdle:
		return "待命"
	case ExpertBusy:
		return "出征中"
	case ExpertResting:
		return "休整中"
	case ExpertOffline:
		return "离线"
	default:
		return "未知"
	}
}

// Expert 专家定义
// 对应 kimi-k3 中的路由专家：具备技能、容量、负载水位、路由统计
type Expert struct {
	ID          string                 `json:"id" yaml:"id"`
	Name        string                 `json:"name" yaml:"name"`
	Type        ExpertType             `json:"type" yaml:"type"`
	Title       string                 `json:"title" yaml:"title"`
	Description string                 `json:"description" yaml:"description"`
	Skills      []string               `json:"skills" yaml:"skills"`
	Traits      map[string]interface{} `json:"traits" yaml:"traits"`
	Stats       ExpertStats            `json:"stats" yaml:"stats"`
	State       ExpertState            `json:"state" yaml:"state"`
	CreatedAt   time.Time              `json:"created_at" yaml:"created_at"`

	// MoE 路由相关字段
	// Capacity 专家容量（最大并发负载，对应 kimi-k3 专家并行容量）
	Capacity int `json:"capacity" yaml:"capacity"`
	// RouteStats 路由统计（原子操作，线程安全）
	RouteStats ExpertRouteStats `json:"-" yaml:"-"`

	// 内部同步字段
	mu sync.RWMutex
}

// ExpertStats 专家战绩（历史聚合）
type ExpertStats struct {
	TotalMissions   int     `json:"total_missions" yaml:"total_missions"`
	SuccessCount    int     `json:"success_count" yaml:"success_count"`
	FailureCount    int     `json:"failure_count" yaml:"failure_count"`
	AvgResponseTime float64 `json:"avg_response_time_ms" yaml:"avg_response_time_ms"`
}

// ExpertHandler 专家处理器接口（对应 kimi-k3 专家的前向计算）
type ExpertHandler interface {
	Execute(ctx context.Context, order *cmd_center.MilitaryOrder) (*cmd_center.GeneralReport, error)
}

// ExpertFilter 专家过滤器
type ExpertFilter struct {
	Type   ExpertType
	State  ExpertState
	Skills []string
}

// ExpertPool 专家池接口
type ExpertPool interface {
	// Register 注册专家
	Register(expert *Expert) error
	// Unregister 注销专家
	Unregister(id string) error
	// Get 获取专家
	Get(id string) (*Expert, error)
	// List 列出专家
	List(filter ExpertFilter) []*Expert
	// Execute 派遣单个专家执行
	Execute(ctx context.Context, expertID string, order *cmd_center.MilitaryOrder) (*cmd_center.GeneralReport, error)
	// Count 获取专家数量
	Count() int
}

// MoEExpertPool MoE 专家池实现
// 对齐 kimi-k3 Stable LatentMoE：管理大量专家 + 路由统计 + 容量控制
type MoEExpertPool struct {
	mu       sync.RWMutex
	experts  map[string]*Expert
	handlers map[ExpertType]ExpertHandler
}

// NewMoEExpertPool 创建 MoE 专家池
func NewMoEExpertPool() *MoEExpertPool {
	pool := &MoEExpertPool{
		experts:  make(map[string]*Expert),
		handlers: make(map[ExpertType]ExpertHandler),
	}
	pool.initWuHu()
	return pool
}

// initWuHu 初始化五虎将专家（每个专家默认容量为 1，可按需扩展）
func (p *MoEExpertPool) initWuHu() {
	defaults := []struct {
		expert  *Expert
		handler ExpertHandler
	}{
		{
			expert: &Expert{
				ID:          "guanyu",
				Name:        "关羽",
				Type:        ExpertGuanYu,
				Title:       "武圣",
				Description: "情报搜集专家，擅长数据收集与整理",
				Skills:      []string{"data_collection", "web_search", "info_gathering"},
				Traits:      map[string]interface{}{"precision": 0.95, "speed": 0.85},
				State:       ExpertIdle,
				CreatedAt:   time.Now(),
				Capacity:    2,
			},
			handler: &GuanYuHandler{},
		},
		{
			expert: &Expert{
				ID:          "zhangfei",
				Name:        "张飞",
				Type:        ExpertZhangFei,
				Title:       "猛将",
				Description: "数据工程专家，擅长数据清洗与结构化",
				Skills:      []string{"data_processing", "etl", "data_cleaning"},
				Traits:      map[string]interface{}{"power": 0.95, "speed": 0.90},
				State:       ExpertIdle,
				CreatedAt:   time.Now(),
				Capacity:    2,
			},
			handler: &ZhangFeiHandler{},
		},
		{
			expert: &Expert{
				ID:          "zhaoyun",
				Name:        "赵云",
				Type:        ExpertZhaoYun,
				Title:       "常胜将军",
				Description: "分析可视化专家，擅长数据分析与图表生成",
				Skills:      []string{"data_analysis", "visualization", "chart_generation"},
				Traits:      map[string]interface{}{"agility": 0.95, "accuracy": 0.92},
				State:       ExpertIdle,
				CreatedAt:   time.Now(),
				Capacity:    2,
			},
			handler: &ZhaoYunHandler{},
		},
		{
			expert: &Expert{
				ID:          "machao",
				Name:        "马超",
				Type:        ExpertMaChao,
				Title:       "锦马超",
				Description: "报告撰写专家，擅长文案与文档生成",
				Skills:      []string{"writing", "report_generation", "documentation"},
				Traits:      map[string]interface{}{"elegance": 0.95, "speed": 0.88},
				State:       ExpertIdle,
				CreatedAt:   time.Now(),
				Capacity:    2,
			},
			handler: &MaChaoHandler{},
		},
		{
			expert: &Expert{
				ID:          "huangzhong",
				Name:        "黄忠",
				Type:        ExpertHuangZhong,
				Title:       "老将",
				Description: "质量审核专家，擅长校验与把关",
				Skills:      []string{"quality_check", "review", "validation"},
				Traits:      map[string]interface{}{"experience": 0.98, "precision": 0.96},
				State:       ExpertIdle,
				CreatedAt:   time.Now(),
				Capacity:    1,
			},
			handler: &HuangZhongHandler{},
		},
	}
	for _, d := range defaults {
		_ = p.Register(d.expert)
		p.handlers[d.expert.Type] = d.handler
	}
}

// Register 注册专家
func (p *MoEExpertPool) Register(expert *Expert) error {
	if expert == nil {
		return fmt.Errorf("专家不能为空")
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	if expert.Capacity <= 0 {
		expert.Capacity = 1
	}
	p.experts[expert.ID] = expert
	return nil
}

// Unregister 注销专家
func (p *MoEExpertPool) Unregister(id string) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	delete(p.experts, id)
	return nil
}

// Get 获取专家
func (p *MoEExpertPool) Get(id string) (*Expert, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	expert, exists := p.experts[id]
	if !exists {
		return nil, fmt.Errorf("专家不存在: %s", id)
	}
	return expert, nil
}

// List 列出专家（按过滤条件）
func (p *MoEExpertPool) List(filter ExpertFilter) []*Expert {
	p.mu.RLock()
	defer p.mu.RUnlock()
	result := make([]*Expert, 0, len(p.experts))
	for _, e := range p.experts {
		if filter.Type != "" && e.Type != filter.Type {
			continue
		}
		// State 为负值时表示不过滤状态
		if filter.State >= 0 && e.State != filter.State {
			continue
		}
		if len(filter.Skills) > 0 && !expertHasAnySkill(e, filter.Skills) {
			continue
		}
		result = append(result, e)
	}
	return result
}

// Count 获取专家数量
func (p *MoEExpertPool) Count() int {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return len(p.experts)
}

// Execute 派遣单个专家执行
func (p *MoEExpertPool) Execute(ctx context.Context, expertID string, order *cmd_center.MilitaryOrder) (*cmd_center.GeneralReport, error) {
	expert, err := p.Get(expertID)
	if err != nil {
		return nil, err
	}
	handler, exists := p.handlers[expert.Type]
	if !exists {
		return nil, fmt.Errorf("专家处理器不存在: %s", expert.Type)
	}
	return p.executeWithStats(ctx, expert, handler, order)
}

// ExecuteWithDecision 按路由决策并行执行多个专家
// 对应 kimi-k3 中"Top-K 专家激活后并行计算 -> 结果聚合"
func (p *MoEExpertPool) ExecuteWithDecision(ctx context.Context, decision *RoutingDecision, order *cmd_center.MilitaryOrder) (*cmd_center.BattleReport, error) {
	if decision == nil || len(decision.Activated) == 0 {
		return &cmd_center.BattleReport{
			OrderID: order.ID,
			Success: false,
			Message: "无激活专家",
		}, nil
	}

	report := &cmd_center.BattleReport{
		OrderID:   order.ID,
		StartedAt: time.Now(),
		Generals:  make([]cmd_center.GeneralReport, 0, len(decision.Activated)),
	}

	// 单专家直接执行，避免不必要的并发开销
	if len(decision.Activated) == 1 {
		expert := decision.Activated[0]
		gr, err := p.Execute(ctx, expert.ID, order)
		if err != nil {
			report.Success = false
			report.Message = err.Error()
			report.CompletedAt = time.Now()
			return report, nil
		}
		report.Generals = append(report.Generals, *gr)
		report.Success = gr.Success
		report.Message = gr.Message
		report.CompletedAt = time.Now()
		return report, nil
	}

	// 多专家并行执行（对应 kimi-k3 专家并行计算）
	type execResult struct {
		report *cmd_center.GeneralReport
		err    error
	}
	resultCh := make(chan execResult, len(decision.Activated))
	for _, expert := range decision.Activated {
		go func(e *Expert) {
			gr, err := p.Execute(ctx, e.ID, order)
			resultCh <- execResult{report: gr, err: err}
		}(expert)
	}

	// 收集结果
	allSuccess := true
	for i := 0; i < len(decision.Activated); i++ {
		r := <-resultCh
		if r.err != nil {
			allSuccess = false
			report.Generals = append(report.Generals, cmd_center.GeneralReport{
				GeneralID:   "",
				GeneralName: "",
				Success:     false,
				Message:     r.err.Error(),
			})
			continue
		}
		if !r.report.Success {
			allSuccess = false
		}
		report.Generals = append(report.Generals, *r.report)
	}
	report.Success = allSuccess
	report.CompletedAt = time.Now()
	if allSuccess {
		report.Message = fmt.Sprintf("Top-%d 专家协同执行成功", len(decision.Activated))
	} else {
		report.Message = "部分专家执行失败"
	}
	return report, nil
}

// executeWithStats 带统计的执行（对应 kimi-k3 推理时的专家负载追踪）
func (p *MoEExpertPool) executeWithStats(ctx context.Context, expert *Expert, handler ExpertHandler, order *cmd_center.MilitaryOrder) (*cmd_center.GeneralReport, error) {
	// 容量检查（对应 kimi-k3 专家容量上限），tryAcquire 已通过 CAS 增加 ActiveLoads
	if !tryAcquire(expert) {
		return nil, fmt.Errorf("专家 %s 负载已达容量上限 %d", expert.Name, expert.Capacity)
	}
	defer release(expert)

	expert.mu.Lock()
	expert.State = ExpertBusy
	expert.mu.Unlock()

	startTime := time.Now()
	report, err := handler.Execute(ctx, order)
	duration := time.Since(startTime).Milliseconds()

	// 更新路由统计（ActiveLoads 由 release 负责）
	expert.RouteStats.TotalRoutes.Add(1)
	expert.RouteStats.TotalDurationMs.Add(duration)

	expert.mu.Lock()
	expert.Stats.TotalMissions++
	if err != nil || (report != nil && !report.Success) {
		expert.Stats.FailureCount++
		expert.RouteStats.FailureCount.Add(1)
	} else {
		expert.Stats.SuccessCount++
		expert.RouteStats.SuccessCount.Add(1)
	}
	// 更新平均响应时间（滑动平均）
	if expert.Stats.TotalMissions > 0 {
		expert.Stats.AvgResponseTime =
			(expert.Stats.AvgResponseTime*float64(expert.Stats.TotalMissions-1) + float64(duration)) /
				float64(expert.Stats.TotalMissions)
	}
	expert.State = ExpertIdle
	expert.mu.Unlock()

	return report, err
}

// tryAcquire 尝试获取专家容量（非阻塞，CAS 保证并发安全）
func tryAcquire(expert *Expert) bool {
	for {
		current := expert.RouteStats.ActiveLoads.Load()
		if current >= int64(expert.Capacity) {
			return false
		}
		if expert.RouteStats.ActiveLoads.CompareAndSwap(current, current+1) {
			return true
		}
	}
}

// release 释放专家容量
func release(expert *Expert) {
	for {
		current := expert.RouteStats.ActiveLoads.Load()
		if current <= 0 {
			return
		}
		if expert.RouteStats.ActiveLoads.CompareAndSwap(current, current-1) {
			return
		}
	}
}

// expertHasAnySkill 判断专家是否拥有任一指定技能
func expertHasAnySkill(expert *Expert, skills []string) bool {
	skillSet := make(map[string]struct{}, len(expert.Skills))
	for _, s := range expert.Skills {
		skillSet[s] = struct{}{}
	}
	for _, s := range skills {
		if _, ok := skillSet[s]; ok {
			return true
		}
	}
	return false
}

// 确保 MoEExpertPool 实现 ExpertPool 接口
var _ ExpertPool = (*MoEExpertPool)(nil)

// ===== 五虎将处理器实现（对应 kimi-k3 中各专家的前向计算逻辑）=====

// GuanYuHandler 关羽处理器 - 情报搜集
type GuanYuHandler struct{}

func (h *GuanYuHandler) Execute(ctx context.Context, order *cmd_center.MilitaryOrder) (*cmd_center.GeneralReport, error) {
	return &cmd_center.GeneralReport{
		GeneralID:   "guanyu",
		GeneralName: "关羽",
		Success:     true,
		Message:     "关某不辱使命，情报已收集完毕",
		Data: map[string]interface{}{
			"source":     "web_search",
			"data_count": 100,
			"quality":    "high",
		},
	}, nil
}

// ZhangFeiHandler 张飞处理器 - 数据工程
type ZhangFeiHandler struct{}

func (h *ZhangFeiHandler) Execute(ctx context.Context, order *cmd_center.MilitaryOrder) (*cmd_center.GeneralReport, error) {
	return &cmd_center.GeneralReport{
		GeneralID:   "zhangfei",
		GeneralName: "张飞",
		Success:     true,
		Message:     "燕人张飞在此，数据处理完成！",
		Data: map[string]interface{}{
			"records_processed": 1000,
			"quality_score":     0.95,
		},
	}, nil
}

// ZhaoYunHandler 赵云处理器 - 分析可视化
type ZhaoYunHandler struct{}

func (h *ZhaoYunHandler) Execute(ctx context.Context, order *cmd_center.MilitaryOrder) (*cmd_center.GeneralReport, error) {
	return &cmd_center.GeneralReport{
		GeneralID:   "zhaoyun",
		GeneralName: "赵云",
		Success:     true,
		Message:     "常山赵子龙，七进七出，分析完成！",
		Data: map[string]interface{}{
			"charts_generated": 10,
			"insights_count":   5,
		},
	}, nil
}

// MaChaoHandler 马超处理器 - 报告撰写
type MaChaoHandler struct{}

func (h *MaChaoHandler) Execute(ctx context.Context, order *cmd_center.MilitaryOrder) (*cmd_center.GeneralReport, error) {
	return &cmd_center.GeneralReport{
		GeneralID:   "machao",
		GeneralName: "马超",
		Success:     true,
		Message:     "西凉锦马超，报告已成！",
		Data: map[string]interface{}{
			"document_url": "https://example.com/report",
			"pages":        20,
			"word_count":   5000,
		},
	}, nil
}

// HuangZhongHandler 黄忠处理器 - 质量审核
type HuangZhongHandler struct{}

func (h *HuangZhongHandler) Execute(ctx context.Context, order *cmd_center.MilitaryOrder) (*cmd_center.GeneralReport, error) {
	return &cmd_center.GeneralReport{
		GeneralID:   "huangzhong",
		GeneralName: "黄忠",
		Success:     true,
		Message:     "老将黄忠，百步穿杨，审核完毕！",
		Data: map[string]interface{}{
			"issues_found":   0,
			"quality_passed": true,
			"score":          98,
		},
	}, nil
}
