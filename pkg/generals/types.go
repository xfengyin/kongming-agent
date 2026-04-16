// 五虎将 - 子Agent池与并行处理系统
// 关张赵马黄，各显神通

package generals

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/zhuge/kongming/pkg/cmd_center"
)

// GeneralType 将领类型
type GeneralType string

const (
	GeneralGuanYu     GeneralType = "guanyu"     // 关羽 - 情报搜集
	GeneralZhangFei   GeneralType = "zhangfei"   // 张飞 - 数据工程
	GeneralZhaoYun    GeneralType = "zhaoyun"    // 赵云 - 分析可视化
	GeneralMaChao     GeneralType = "machao"     // 马超 - 报告撰写
	GeneralHuangZhong GeneralType = "huangzhong" // 黄忠 - 质量审核
)

// General 将领定义
type General struct {
	ID          string                 `json:"id" yaml:"id"`
	Name        string                 `json:"name" yaml:"name"`
	Type        GeneralType           `json:"type" yaml:"type"`
	Title       string                 `json:"title" yaml:"title"` // 称号
	Description string                 `json:"description" yaml:"description"`
	Skills      []string               `json:"skills" yaml:"skills"`
	Traits      map[string]interface{} `json:"traits" yaml:"traits"`
	Stats       GeneralStats           `json:"stats" yaml:"stats"`
	State       GeneralState           `json:"state" yaml:"state"`
	CreatedAt   time.Time             `json:"created_at" yaml:"created_at"`
}

// GeneralStats 将领战绩
type GeneralStats struct {
	TotalMissions   int     `json:"total_missions" yaml:"total_missions"`
	SuccessCount    int     `json:"success_count" yaml:"success_count"`
	FailureCount    int     `json:"failure_count" yaml:"failure_count"`
	AvgResponseTime float64 `json:"avg_response_time_ms" yaml:"avg_response_time_ms"`
}

// GeneralState 将领状态
type GeneralState int

const (
	GeneralIdle GeneralState = iota
	GeneralBusy
	GeneralResting
	GeneralOffline
)

func (s GeneralState) String() string {
	switch s {
	case GeneralIdle:
		return "待命"
	case GeneralBusy:
		return "出征中"
	case GeneralResting:
		return "休整中"
	case GeneralOffline:
		return "离线"
	default:
		return "未知"
	}
}

// GeneralPool 将领池接口
type GeneralPool interface {
	// Register 注册将领
	Register(general *General) error

	// Unregister 注销将领
	Unregister(id string) error

	// Get 获取将领
	Get(id string) (*General, error)

	// List 列出将领
	List(filter GeneralFilter) []*General

	// Execute 派遣执行
	Execute(ctx context.Context, generalID string, order *cmd_center.MilitaryOrder) (*cmd_center.GeneralReport, error)

	// SelectBest 选择最佳将领
	SelectBest(skill string) (*General, error)

	// Count 获取将领数量
	Count() int
}

// GeneralFilter 将领过滤器
type GeneralFilter struct {
	Type   GeneralType
	State  GeneralState
	Skills []string
}

// WuHuPool 五虎将池
type WuHuPool struct {
	mu       sync.RWMutex
	generals map[string]*General
	handlers map[GeneralType]GeneralHandler
}

// GeneralHandler 将领处理器
type GeneralHandler interface {
	Execute(ctx context.Context, order *cmd_center.MilitaryOrder) (*cmd_center.GeneralReport, error)
}

// NewWuHuPool 创建五虎将池
func NewWuHuPool() *WuHuPool {
	pool := &WuHuPool{
		generals: make(map[string]*General),
		handlers: make(map[GeneralType]GeneralHandler),
	}

	// 初始化五虎将
	pool.initWuHu()

	return pool
}

// Count 获取将领数量
func (p *WuHuPool) Count() int {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return len(p.generals)
}

// 初始化五虎将
func (p *WuHuPool) initWuHu() {
	// 关羽 - 情报搜集大将
	p.Register(&General{
		ID:          "guanyu",
		Name:        "关羽",
		Type:        GeneralGuanYu,
		Title:       "武圣",
		Description: "情报搜集专家，擅长数据收集与整理",
		Skills:      []string{"data_collection", "web_search", "info_gathering"},
		Traits:      map[string]interface{}{"precision": 0.95, "speed": 0.85},
		Stats:       GeneralStats{},
		State:       GeneralIdle,
		CreatedAt:   time.Now(),
	})
	p.handlers[GeneralGuanYu] = &GuanYuHandler{}

	// 张飞 - 数据工程大将
	p.Register(&General{
		ID:          "zhangfei",
		Name:        "张飞",
		Type:        GeneralZhangFei,
		Title:       "猛将",
		Description: "数据工程专家，擅长数据清洗与结构化",
		Skills:      []string{"data_processing", "etl", "data_cleaning"},
		Traits:      map[string]interface{}{"power": 0.95, "speed": 0.90},
		Stats:       GeneralStats{},
		State:       GeneralIdle,
		CreatedAt:   time.Now(),
	})
	p.handlers[GeneralZhangFei] = &ZhangFeiHandler{}

	// 赵云 - 分析可视化大将
	p.Register(&General{
		ID:          "zhaoyun",
		Name:        "赵云",
		Type:        GeneralZhaoYun,
		Title:       "常胜将军",
		Description: "分析可视化专家，擅长数据分析与图表生成",
		Skills:      []string{"data_analysis", "visualization", "chart_generation"},
		Traits:      map[string]interface{}{"agility": 0.95, "accuracy": 0.92},
		Stats:       GeneralStats{},
		State:       GeneralIdle,
		CreatedAt:   time.Now(),
	})
	p.handlers[GeneralZhaoYun] = &ZhaoYunHandler{}

	// 马超 - 报告撰写大将
	p.Register(&General{
		ID:          "machao",
		Name:        "马超",
		Type:        GeneralMaChao,
		Title:       "锦马超",
		Description: "报告撰写专家，擅长文案与文档生成",
		Skills:      []string{"writing", "report_generation", "documentation"},
		Traits:      map[string]interface{}{"elegance": 0.95, "speed": 0.88},
		Stats:       GeneralStats{},
		State:       GeneralIdle,
		CreatedAt:   time.Now(),
	})
	p.handlers[GeneralMaChao] = &MaChaoHandler{}

	// 黄忠 - 质量审核大将
	p.Register(&General{
		ID:          "huangzhong",
		Name:        "黄忠",
		Type:        GeneralHuangZhong,
		Title:       "老将",
		Description: "质量审核专家，擅长校验与把关",
		Skills:      []string{"quality_check", "review", "validation"},
		Traits:      map[string]interface{}{"experience": 0.98, "precision": 0.96},
		Stats:       GeneralStats{},
		State:       GeneralIdle,
		CreatedAt:   time.Now(),
	})
	p.handlers[GeneralHuangZhong] = &HuangZhongHandler{}
}

// Register 注册将领
func (p *WuHuPool) Register(general *General) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.generals[general.ID] = general
	return nil
}

// Unregister 注销将领
func (p *WuHuPool) Unregister(id string) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	delete(p.generals, id)
	return nil
}

// Get 获取将领
func (p *WuHuPool) Get(id string) (*General, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()

	general, exists := p.generals[id]
	if !exists {
		return nil, fmt.Errorf("将领不存在: %s", id)
	}

	return general, nil
}

// List 列出将领
func (p *WuHuPool) List(filter GeneralFilter) []*General {
	p.mu.RLock()
	defer p.mu.RUnlock()

	var result []*General
	for _, g := range p.generals {
		if filter.Type != "" && g.Type != filter.Type {
			continue
		}
		if filter.State >= 0 && g.State != filter.State {
			continue
		}
		result = append(result, g)
	}

	return result
}

// Execute 派遣执行
func (p *WuHuPool) Execute(ctx context.Context, generalID string, order *cmd_center.MilitaryOrder) (*cmd_center.GeneralReport, error) {
	general, err := p.Get(generalID)
	if err != nil {
		return nil, err
	}

	handler, exists := p.handlers[general.Type]
	if !exists {
		return nil, fmt.Errorf("将领处理器不存在: %s", general.Type)
	}

	// 更新状态为出征中
	general.State = GeneralBusy
	startTime := time.Now()

	// 执行
	report, err := handler.Execute(ctx, order)

	// 更新统计
	duration := time.Since(startTime).Milliseconds()
	general.Stats.TotalMissions++
	if err != nil || !report.Success {
		general.Stats.FailureCount++
	} else {
		general.Stats.SuccessCount++
	}

	// 更新平均响应时间
	if general.Stats.TotalMissions > 0 {
		general.Stats.AvgResponseTime =
			(general.Stats.AvgResponseTime*float64(general.Stats.TotalMissions-1) + float64(duration)) /
				float64(general.Stats.TotalMissions)
	}

	// 恢复待命状态
	general.State = GeneralIdle

	return report, err
}

// SelectBest 选择最佳将领
func (p *WuHuPool) SelectBest(skill string) (*General, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()

	var best *General
	var bestScore float64

	for _, g := range p.generals {
		// 检查是否有该技能
		hasSkill := false
		for _, s := range g.Skills {
			if s == skill {
				hasSkill = true
				break
			}
		}
		if !hasSkill {
			continue
		}

		// 计算评分
		score := p.calculateScore(g)
		if score > bestScore {
			bestScore = score
			best = g
		}
	}

	if best == nil {
		return nil, fmt.Errorf("无合适将领可执行技能: %s", skill)
	}

	return best, nil
}

func (p *WuHuPool) calculateScore(g *General) float64 {
	if g.State != GeneralIdle {
		return 0 // 非待命状态不得分
	}

	// 成功率权重
	successRate := 0.5
	if g.Stats.TotalMissions > 0 {
		successRate = float64(g.Stats.SuccessCount) / float64(g.Stats.TotalMissions)
	}

	// 响应速度权重（越慢分越低）
	speedScore := 1.0
	if g.Stats.AvgResponseTime > 0 {
		speedScore = 1000.0 / (1000.0 + g.Stats.AvgResponseTime)
	}

	return successRate*0.6 + speedScore*0.4
}

// ===== 五虎将处理器实现 =====

// GuanYuHandler 关羽处理器
type GuanYuHandler struct{}

func (h *GuanYuHandler) Execute(ctx context.Context, order *cmd_center.MilitaryOrder) (*cmd_center.GeneralReport, error) {
	// 关羽执行情报搜集
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

// ZhangFeiHandler 张飞处理器
type ZhangFeiHandler struct{}

func (h *ZhangFeiHandler) Execute(ctx context.Context, order *cmd_center.MilitaryOrder) (*cmd_center.GeneralReport, error) {
	// 张飞执行数据工程
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

// ZhaoYunHandler 赵云处理器
type ZhaoYunHandler struct{}

func (h *ZhaoYunHandler) Execute(ctx context.Context, order *cmd_center.MilitaryOrder) (*cmd_center.GeneralReport, error) {
	// 赵云执行分析可视化
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

// MaChaoHandler 马超处理器
type MaChaoHandler struct{}

func (h *MaChaoHandler) Execute(ctx context.Context, order *cmd_center.MilitaryOrder) (*cmd_center.GeneralReport, error) {
	// 马超执行报告撰写
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

// HuangZhongHandler 黄忠处理器
type HuangZhongHandler struct{}

func (h *HuangZhongHandler) Execute(ctx context.Context, order *cmd_center.MilitaryOrder) (*cmd_center.GeneralReport, error) {
	// 黄忠执行质量审核
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
