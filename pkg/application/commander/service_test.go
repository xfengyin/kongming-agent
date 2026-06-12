// Package commander commander 应用层用例的单元测试。
//
// 本文件覆盖 Service（commander 派单/查询/审核/列表）的核心 9 个用例。
// mock 策略：
//   - OrderRepository → 直接用 memory.OrderRepo（与生产同实现，零 Mock 偏离）；
//   - GeneralPool     → stubPool，generals/reports map 控制选将与执行结果；
//   - ResilientRunner → noopResilient：直接执行 fn，便于断言内层逻辑；
//   - Observer        → noopObserver：span/event/metric 全部 no-op，零观测副作用；
//   - Planner         → 优先复用 DefaultPlanner；需要模拟失败时用 errPlanner。
package commander

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"

	domerrs "github.com/zhuge/kongming/pkg/domain/errors"
	"github.com/zhuge/kongming/pkg/domain/model"
	"github.com/zhuge/kongming/pkg/domain/port"
	"github.com/zhuge/kongming/pkg/infra/persistence/memory"
)

// ---------------------------------------------------------------------------
// Stub: GeneralPool
// ---------------------------------------------------------------------------

// stubPool 满足 port.GeneralPool，generals/reports map 控制 SelectBest/Execute 行为。
type stubPool struct {
	generals map[model.GeneralID]*model.General
	reports  map[model.GeneralID]*model.GeneralReport
	// selectErr 强制 SelectBest 返回的 error（用于「无将」场景）。
	selectErr error
}

func (p *stubPool) Get(_ context.Context, _ model.GeneralID) (*model.General, error) {
	return nil, nil
}

func (p *stubPool) List(_ context.Context) ([]*model.General, error) {
	return nil, nil
}

func (p *stubPool) Register(_ context.Context, _ *model.General) error {
	return nil
}

func (p *stubPool) Unregister(_ context.Context, _ model.GeneralID) error {
	return nil
}

func (p *stubPool) SelectBest(skill string) (*model.General, error) {
	if p.selectErr != nil {
		return nil, p.selectErr
	}
	for _, g := range p.generals {
		for _, s := range g.Skills {
			if s == skill {
				return g, nil
			}
		}
	}
	return nil, errors.New("no general for skill: " + skill)
}

func (p *stubPool) Execute(_ context.Context, id model.GeneralID, _ *model.Order) (*model.GeneralReport, error) {
	if r, ok := p.reports[id]; ok {
		return r, nil
	}
	return &model.GeneralReport{GeneralID: id, Success: false, Error: "no stub report"}, nil
}

// 编译期断言
var _ port.GeneralPool = (*stubPool)(nil)

// ---------------------------------------------------------------------------
// Stub: ResilientRunner
// ---------------------------------------------------------------------------

type noopResilient struct{}

func (noopResilient) Run(_ context.Context, _ string, fn func(context.Context) error) error {
	return fn(context.Background())
}

func (noopResilient) RunWithResult(_ context.Context, _ string, fn func(context.Context) (any, error)) (any, error) {
	return fn(context.Background())
}

var _ port.ResilientRunner = noopResilient{}

// ---------------------------------------------------------------------------
// Stub: Observer
// ---------------------------------------------------------------------------

type noopObserver struct{}

func (noopObserver) StartSpan(ctx context.Context, _ string, _ ...attribute.KeyValue) (context.Context, trace.Span) {
	return ctx, trace.SpanFromContext(ctx)
}

func (noopObserver) RecordError(_ trace.Span, _ error) {}

func (noopObserver) RecordEvent(_ context.Context, _ string, _ ...attribute.KeyValue) {}

func (noopObserver) IncCounter(_ string, _ map[string]string) {}

func (noopObserver) ObserveHistogram(_ string, _ float64, _ map[string]string) {}

func (noopObserver) SetGauge(_ string, _ float64, _ map[string]string) {}

func (noopObserver) Shutdown(_ context.Context) error { return nil }

var _ port.Observer = noopObserver{}

// ---------------------------------------------------------------------------
// Stub: Planner（用于 PlannerError 测试）
// ---------------------------------------------------------------------------

type errPlanner struct{ err error }

func (p *errPlanner) Plan(_ context.Context, _ *model.Order) (*model.Strategy, error) {
	return nil, p.err
}

// ---------------------------------------------------------------------------
// 公共工具
// ---------------------------------------------------------------------------

// newSvc 构造一个标准测试 Service。
// engine / vault 在当前 Dispatch 主流程未被使用，传 nil。
func newSvc(t *testing.T, planner Planner, pool port.GeneralPool) (*Service, *memory.OrderRepo) {
	t.Helper()
	store := memory.NewStore()
	orders := memory.NewOrderRepo(store)
	s := New(planner, pool, nil, nil, orders, noopResilient{}, noopObserver{}, zap.NewNop())
	return s, orders
}

// ---------------------------------------------------------------------------
// 测试用例
// ---------------------------------------------------------------------------

// TestService_Dispatch_HappyPath 验证派单主流程：tactic 匹配关羽 → 报告成功 → 落 StateCompleted。
func TestService_Dispatch_HappyPath(t *testing.T) {
	pool := &stubPool{
		generals: map[model.GeneralID]*model.General{
			"guanyu": {
				ID:     "guanyu",
				Name:   "关羽",
				Skills: []string{"execute"},
				State:  int(model.GeneralIdle),
			},
		},
		reports: map[model.GeneralID]*model.GeneralReport{
			"guanyu": {GeneralID: "guanyu", Name: "关羽", Success: true, Output: "ok"},
		},
	}
	s, orders := newSvc(t, &DefaultPlanner{}, pool)

	order := &model.Order{
		ID:        "o1",
		Name:      "test",
		State:     model.StatePending,
		Priority:  model.PriorityNormal,
		Strategy:  model.Strategy{Objectives: []string{"obj1"}},
		CreatedAt: time.Now(),
	}
	report, err := s.Dispatch(context.Background(), order)
	require.NoError(t, err)
	require.NotNil(t, report)
	assert.True(t, report.Success, "整体应视为成功")
	require.Len(t, report.Generals, 1, "应有一个将的子战报")
	assert.Equal(t, model.GeneralID("guanyu"), report.Generals[0].GeneralID)

	saved, err := orders.Get(context.Background(), "o1")
	require.NoError(t, err)
	assert.Equal(t, model.StateCompleted, saved.State, "落库应为 StateCompleted")
}

// TestService_Dispatch_Idempotent 验证幂等路径：仓库中已存在 StateCompleted 订单 → 走 replayReport。
func TestService_Dispatch_Idempotent(t *testing.T) {
	pool := &stubPool{} // 本次不会调用（走 replay）
	s, orders := newSvc(t, &DefaultPlanner{}, pool)

	// 先在仓库里放一份已完成订单（模拟「首次派单已落库」）。
	require.NoError(t, orders.Save(context.Background(), &model.Order{
		ID:        "o1",
		Name:      "replay",
		State:     model.StateCompleted,
		Priority:  model.PriorityNormal,
		CreatedAt: time.Now().Add(-time.Minute),
		UpdatedAt: time.Now(),
	}))

	report, err := s.Dispatch(context.Background(), &model.Order{
		ID:    "o1",
		State: model.StateCompleted, // 调用方传 Completed 也不会触发 TransitionTo
	})
	require.NoError(t, err)
	require.NotNil(t, report)
	msg, ok := report.Result["message"].(string)
	require.True(t, ok, "Result.message 必须存在")
	assert.Contains(t, msg, "idempotent", "幂等 replay 战报应包含 idempotent 标识")
	assert.True(t, report.Success, "幂等 replay 始终成功")
}

// TestService_Dispatch_NoGeneral 验证「无将」场景：tactic 找不到匹配的将时
// runTactics 内部 continue，最终 report.Generals 为空、Success 仍为 true。
func TestService_Dispatch_NoGeneral(t *testing.T) {
	pool := &stubPool{
		// 故意不预置任何将领 → SelectBest 永远返回 error
		selectErr: errors.New("no general available"),
	}
	s, orders := newSvc(t, &DefaultPlanner{}, pool)

	order := &model.Order{
		ID:        "o-empty",
		State:     model.StatePending,
		Priority:  model.PriorityNormal,
		Strategy:  model.Strategy{Objectives: []string{"obj1"}},
		CreatedAt: time.Now(),
	}
	report, err := s.Dispatch(context.Background(), order)
	require.NoError(t, err)
	require.NotNil(t, report)
	assert.Empty(t, report.Generals, "无将时 Generals 应为空")
	assert.True(t, report.Success, "runTactics 内部 continue 不影响整体成功标志")

	saved, err := orders.Get(context.Background(), "o-empty")
	require.NoError(t, err)
	assert.Equal(t, model.StateCompleted, saved.State, "整体仍应落 StateCompleted")
}

// TestService_Dispatch_OrderStateConflict 验证状态机冲突：StateCompleted 不可迁移到 StatePlanning。
//
// 关键：订单未在仓库中（Get 命中 nil），于是进入主流程；TransitionTo 拒绝并返回 INVALID_STATE。
func TestService_Dispatch_OrderStateConflict(t *testing.T) {
	pool := &stubPool{
		generals: map[model.GeneralID]*model.General{
			"guanyu": {ID: "guanyu", Name: "关羽", Skills: []string{"execute"}, State: int(model.GeneralIdle)},
		},
		reports: map[model.GeneralID]*model.GeneralReport{
			"guanyu": {GeneralID: "guanyu", Success: true},
		},
	}
	s, _ := newSvc(t, &DefaultPlanner{}, pool)

	// StateCompleted → StatePlanning 非法（StateCompleted 是终态，无任何迁出）
	order := &model.Order{
		ID:    "o-conflict",
		State: model.StateCompleted,
		// 注意：故意不 Save → Get 命中 nil → 不会走 replay
	}
	_, err := s.Dispatch(context.Background(), order)
	require.Error(t, err)
	var de *domerrs.Error
	require.True(t, errors.As(err, &de), "错误应可转为 *domerrs.Error")
	assert.Equal(t, domerrs.INVALID_STATE, de.Code, "应返回 INVALID_STATE")
}

// TestService_Dispatch_PlannerError 验证 Planner 失败被正确包装为 STRATEGY_FAILED。
func TestService_Dispatch_PlannerError(t *testing.T) {
	pool := &stubPool{
		generals: map[model.GeneralID]*model.General{
			"guanyu": {ID: "guanyu", Skills: []string{"execute"}},
		},
	}
	planErr := errors.New("planner boom")
	s, _ := newSvc(t, &errPlanner{err: planErr}, pool)

	order := &model.Order{
		ID:       "o-plan-fail",
		State:    model.StatePending,
		Strategy: model.Strategy{Objectives: []string{"obj1"}},
	}
	_, err := s.Dispatch(context.Background(), order)
	require.Error(t, err)
	var de *domerrs.Error
	require.True(t, errors.As(err, &de), "错误应可转为 *domerrs.Error")
	assert.Equal(t, domerrs.STRATEGY_FAILED, de.Code, "应返回 STRATEGY_FAILED")
	// 根因应被保留
	assert.True(t, errors.Is(err, planErr), "根因应可经 errors.Is 匹配")
}

// TestService_GetOrder_Found 验证 GetOrder 命中时原样透传。
func TestService_GetOrder_Found(t *testing.T) {
	s, orders := newSvc(t, &DefaultPlanner{}, &stubPool{})

	stored := &model.Order{
		ID:    "o-get",
		Name:  "to-fetch",
		State: model.StatePending,
	}
	require.NoError(t, orders.Save(context.Background(), stored))

	got, err := s.GetOrder(context.Background(), "o-get")
	require.NoError(t, err)
	require.NotNil(t, got)
	assert.Equal(t, stored.ID, got.ID)
	assert.Equal(t, stored.Name, got.Name)
}

// TestService_GetOrder_NotFound 验证 GetOrder 未命中时透传底层错误（NOT_FOUND）。
func TestService_GetOrder_NotFound(t *testing.T) {
	s, _ := newSvc(t, &DefaultPlanner{}, &stubPool{})

	got, err := s.GetOrder(context.Background(), "o-missing")
	assert.Nil(t, got)
	require.Error(t, err)
	// 底层 memory.OrderRepo.Get 返回的 error 含 ErrOrderNotFound 哨兵
	assert.True(t, errors.Is(err, memory.ErrOrderNotFound), "应透传 ErrOrderNotFound")
}

// TestService_ListOrders_AllStates 验证 ListOrders(StateNone) 返回全量。
func TestService_ListOrders_AllStates(t *testing.T) {
	s, orders := newSvc(t, &DefaultPlanner{}, &stubPool{})

	// 落 3 个不同状态的订单
	for _, st := range []model.State{model.StatePending, model.StateExecuting, model.StateCompleted} {
		require.NoError(t, orders.Save(context.Background(), &model.Order{
			ID:    model.OrderID("o-" + st.String()),
			State: st,
		}))
	}

	all, err := s.ListOrders(context.Background(), model.StateNone)
	require.NoError(t, err)
	assert.Len(t, all, 3, "StateNone 视为不过滤")
}

// TestService_PlanStrategy_Delegates 验证 PlanStrategy 直接代理 Planner.Plan。
func TestService_PlanStrategy_Delegates(t *testing.T) {
	pool := &stubPool{
		generals: map[model.GeneralID]*model.General{
			"guanyu": {ID: "guanyu", Skills: []string{"execute"}},
		},
	}
	s, _ := newSvc(t, &DefaultPlanner{}, pool)

	order := &model.Order{
		ID:       "o-plan",
		State:    model.StatePending,
		Priority: model.PriorityHigh,
		Strategy: model.Strategy{Objectives: []string{"obj1", "obj2"}},
	}
	strategy, err := s.PlanStrategy(context.Background(), order)
	require.NoError(t, err)
	require.NotNil(t, strategy)
	assert.Equal(t, "default", strategy.Type)
	assert.Equal(t, model.Tiangai, strategy.BaguaMode, "PriorityHigh → Tiangai")
	assert.Len(t, strategy.Tactics, 2)
}
