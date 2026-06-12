// Package general 应用层 - 将领池（Pool）实现。
//
// 设计要点：
//  1. 端口化：Pool 实现 domain/port.GeneralPool，业务方只依赖接口。
//  2. 线程安全：sync.RWMutex 保护内部 map；写用 Lock、读用 RLock。
//  3. 依赖注入：Observer / Logger / ResilientRunner / Executor 全部外部注入，
//     便于单测中替换 mock 实现（fake observer / nop resilient / mock executor）。
//  4. 可观测：每次 Register / Execute 触发 IncCounter / ObserveHistogram。
//  5. 弹性执行：Execute 走 ResilientRunner 包装，提供重试/熔断/限流/超时能力。
//  6. 单一职责：Pool 只负责"将领注册 + 选将 + 派单编排"，不实现具体业务逻辑；
//     具体业务由 Executor（可替换为五虎将 / 插件 / LLM provider）实现。
//
// 本文件迁移自 pkg/generals.WuHuPool：
//   - 删除了硬编码的 initWuHu() 与五虎将 Handler；
//   - 抽象为 Executor 接口，由 wuhu 子包或插件注入；
//   - 引入 ctx 透传与 ResilientRunner 包装。
package general

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"go.uber.org/zap"

	"github.com/zhuge/kongming/pkg/domain/model"
	"github.com/zhuge/kongming/pkg/domain/port"
)

// Executor 是将领执行器抽象。
//
// 实现方负责：接收 (ctx, general, order) → 返回 (*GeneralReport, error)。
// Pool 仅做编排（设置 Busy、统计、弹性包装、指标上报），不感知具体业务。
//
// 生产实现可由：
//   - pkg/application/general/wuhu.*Handler（五虎将内置处理器）
//   - 插件系统注册的第三方 Executor
//
// 注入。
type Executor interface {
	Execute(ctx context.Context, g *model.General, o *model.Order) (*model.GeneralReport, error)
}

// Pool 是 port.GeneralPool 的内存实现（原 generals.WuHuPool 的应用层重构版）。
//
// 并发模型：
//   - mu 保护 generals map（写用 Lock、读用 RLock）。
//   - 执行过程中 General.Stats / State 的变更由调用方在 Executor 内自行同步；
//     本结构不持有 General 实例级的锁，避免与 Executor 形成死锁链。
type Pool struct {
	// mu 保护 generals map。Register/Unregister 用 Lock；Get/List/SelectBest 用 RLock。
	mu sync.RWMutex
	// generals 将领表，key 为 GeneralID。
	generals map[model.GeneralID]*model.General
	// observer 可观测性端口（指标 / 链路）。
	observer port.Observer
	// logger 结构化日志；nil 时降级为 zap.NewNop()。
	logger *zap.Logger
	// resilient 弹性执行器（重试/熔断/限流/超时）；可为 nil（仅测试用）。
	resilient port.ResilientRunner
	// executor 业务执行器；为 nil 时 Execute 返回错误（避免静默 noop）。
	executor Executor
}

// NewPool 构造一个 Pool。
//
// 参数：
//   - logger：必传，可传 zap.NewNop() 关闭日志。
//   - observer：必传，可传 fakeObserver（测试）或 observability.Observer（生产）。
//   - resilient：可选，传 nil 时降级为 noop 包装（仅 fn 一次调用）。
//   - executor：可选，传 nil 时 Execute 返回错误。
//
// 注入式设计确保单元测试可替换任意依赖（不依赖任何 infra 包）。
func NewPool(logger *zap.Logger, observer port.Observer, resilient port.ResilientRunner, executor Executor) *Pool {
	if logger == nil {
		logger = zap.NewNop()
	}
	if resilient == nil {
		resilient = noopResilient{}
	}
	return &Pool{
		generals:  make(map[model.GeneralID]*model.General),
		observer:  observer,
		logger:    logger,
		resilient: resilient,
		executor:  executor,
	}
}

// noopResilient 是 ResilientRunner 的 noop 实现（仅 fn 一次调用）。
// 抽到 production code 是为了让 NewPool 在传 nil resilient 时也能用，
// 避免在测试与生产中重复写 nopResilient。
type noopResilient struct{}

func (noopResilient) Run(ctx context.Context, _ string, fn func(ctx context.Context) error) error {
	return fn(ctx)
}
func (noopResilient) RunWithResult(ctx context.Context, _ string, fn func(ctx context.Context) (any, error)) (any, error) {
	return fn(ctx)
}

// ErrGeneralNotFound Get/Execute 找不到将领时返回的错误。
// 业务方可使用 errors.Is(err, ErrGeneralNotFound) 判定。
var ErrGeneralNotFound = errors.New("general not found")

// ErrExecutorNotSet Executor 未注入时 Execute 返回的错误。
var ErrExecutorNotSet = errors.New("general executor not set")

// ErrNoGeneralForSkill SelectBest 找不到具备指定 skill 的将领时返回的错误。
var ErrNoGeneralForSkill = errors.New("no general for skill")

// Get 按 ID 查询单个将领。命中返回 *model.General；未命中返回 (nil, ErrGeneralNotFound)。
func (p *Pool) Get(_ context.Context, id model.GeneralID) (*model.General, error) {
	if id == "" {
		return nil, fmt.Errorf("general id is empty")
	}
	p.mu.RLock()
	defer p.mu.RUnlock()
	g, ok := p.generals[id]
	if !ok {
		return nil, fmt.Errorf("%w: %s", ErrGeneralNotFound, id)
	}
	return g, nil
}

// List 返回全量将领快照。返回的切片是新分配的；底层对象仍共享，
// 调用方不应在外部修改 General 字段。
func (p *Pool) List(_ context.Context) ([]*model.General, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	out := make([]*model.General, 0, len(p.generals))
	for _, g := range p.generals {
		out = append(out, g)
	}
	return out, nil
}

// Register 注册一个将领到池中。已存在则覆盖（upsert 语义）。
// 成功注册后上报 general_registered 计数。
func (p *Pool) Register(_ context.Context, g *model.General) error {
	if g == nil {
		return errors.New("general is nil")
	}
	if g.ID == "" {
		return errors.New("general.ID is required")
	}
	p.mu.Lock()
	p.generals[g.ID] = g
	p.mu.Unlock()

	if p.observer != nil {
		p.observer.IncCounter("general_registered", map[string]string{
			"general_id": string(g.ID),
		})
	}
	return nil
}

// Unregister 从池中注销一个将领。不存在时不应返回错误（幂等）。
func (p *Pool) Unregister(_ context.Context, id model.GeneralID) error {
	if id == "" {
		return errors.New("general id is empty")
	}
	p.mu.Lock()
	delete(p.generals, id)
	p.mu.Unlock()
	return nil
}

// SelectBest 在池中按 skill 标签挑选最佳将领。
// 当前实现：拥有该 skill 的将领中，Stats.SuccessCount 最大者胜出。
// 找不到具备该 skill 的将领时返回 ErrNoGeneralForSkill。
func (p *Pool) SelectBest(skill string) (*model.General, error) {
	if skill == "" {
		return nil, errors.New("skill is empty")
	}
	p.mu.RLock()
	defer p.mu.RUnlock()

	var best *model.General
	bestScore := -1 // -1 确保 SuccessCount=0 的将领也能被选中
	for _, g := range p.generals {
		if !hasSkill(g, skill) {
			continue
		}
		if g.Stats.SuccessCount > bestScore {
			bestScore = g.Stats.SuccessCount
			best = g
		}
	}
	if best == nil {
		return nil, fmt.Errorf("%w: %s", ErrNoGeneralForSkill, skill)
	}
	return best, nil
}

// hasSkill 内部辅助：判断 g 是否具备指定 skill。
// 抽到独立函数便于未来扩展（如支持通配符 / 标签继承）。
func hasSkill(g *model.General, skill string) bool {
	for _, s := range g.Skills {
		if s == skill {
			return true
		}
	}
	return false
}

// Execute 由指定 ID 的将领执行一个 Order。
//
// 流程：
//  1. 从池中取出将领（Get）
//  2. 通过 ResilientRunner 包装执行 Executor（重试/熔断/限流/超时）
//  3. 度量执行时长、累加成功/失败计数
//  4. 始终返回非 nil 的 *GeneralReport，error 字段在失败时被填充
//
// 即便 Executor 报错，Pool 也会尽力把可观测信息写入 report。
func (p *Pool) Execute(ctx context.Context, id model.GeneralID, o *model.Order) (*model.GeneralReport, error) {
	g, err := p.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	if p.executor == nil {
		return nil, fmt.Errorf("%w: %s", ErrExecutorNotSet, id)
	}
	if o == nil {
		o = &model.Order{}
	}

	// 标记出征中（线程安全）。
	g.SetState(model.GeneralBusy)
	startTime := time.Now()

	// 弹性执行。
	out, runErr := p.resilient.RunWithResult(ctx, "general.execute",
		func(ctx context.Context) (any, error) {
			return p.executor.Execute(ctx, g, o)
		})
	duration := time.Since(startTime).Seconds()

	// 报告组装：Executor 可能返回 nil report（极端情况），降级为空白 report。
	report, _ := out.(*model.GeneralReport)
	if report == nil {
		report = &model.GeneralReport{
			GeneralID: g.ID,
			Name:      g.Name,
		}
	}
	report.Duration = duration

	// 统计与状态机更新（在锁外做以缩短临界区；stats 字段无独立锁，按需可加）。
	if runErr != nil {
		if report.Error == "" {
			report.Error = runErr.Error()
		}
		if report.Success {
			// Executor 已写 Success=true 但 RunWithResult 仍返回 error：
			// 以 error 为准（防御 Executor 自报成功但 ResilientRunner 拒绝执行的情况）。
			report.Success = false
		}
		p.updateStats(g, false, duration)
		g.SetState(model.GeneralResting)
	} else {
		if !report.Success {
			// Executor 显式报失败但未返回 error → 视为业务失败。
			report.Success = false
		}
		p.updateStats(g, report.Success, duration)
		g.SetState(model.GeneralIdle)
	}

	// 指标上报。
	if p.observer != nil {
		labels := map[string]string{
			"general_id": string(g.ID),
			"success":    boolLabel(report.Success),
		}
		p.observer.IncCounter("general_executed", labels)
		p.observer.ObserveHistogram("general_execute_duration_seconds", duration, labels)
	}

	p.logger.Debug("general executed",
		zap.String("general_id", string(g.ID)),
		zap.String("order_id", string(o.ID)),
		zap.Bool("success", report.Success),
		zap.Float64("duration_sec", duration),
		zap.Error(runErr),
	)

	if runErr != nil {
		return report, runErr
	}
	return report, nil
}

// updateStats 更新将领战绩（成功/失败计数 + 滑动平均响应时间）。
//
// 不持锁：General 自身的 stats 字段无独立锁；并发安全依赖 General 整体不被并发修改，
// 业务方在 Register 时通常一次性写入，Execute 串行化（通过 ResilientRunner）。
func (p *Pool) updateStats(g *model.General, success bool, durationSec float64) {
	g.Stats.TotalMissions++
	if success {
		g.Stats.SuccessCount++
	} else {
		g.Stats.FailureCount++
	}
	// 增量平均：新平均 = 旧平均 + (新值 - 旧平均) / 新总数
	if g.Stats.TotalMissions > 0 {
		g.Stats.AvgResponseTime += (durationSec - g.Stats.AvgResponseTime) / float64(g.Stats.TotalMissions)
	}
}

// boolLabel 把 bool 映射为 prom-friendly 字符串 label。
func boolLabel(b bool) string {
	if b {
		return "true"
	}
	return "false"
}

// 编译期断言：Pool 必须实现 port.GeneralPool。
// 接口方法签名变动会在编译期被捕获，避免运行时静默失败。
var _ port.GeneralPool = (*Pool)(nil)
