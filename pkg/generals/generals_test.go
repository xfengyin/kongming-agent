// Kongming MoE 专家池测试

package generals

import (
	"context"
	"testing"

	"github.com/zhuge/kongming/pkg/cmd_center"
)

func TestMoEExpertPoolCount(t *testing.T) {
	pool := NewMoEExpertPool()
	if count := pool.Count(); count != 5 {
		t.Errorf("期望5位专家，实际有 %d 位", count)
	}
}

func TestMoEExpertPoolList(t *testing.T) {
	pool := NewMoEExpertPool()

	all := pool.List(ExpertFilter{})
	if len(all) != 5 {
		t.Errorf("期望5位专家，实际有 %d 位", len(all))
	}

	guanyu := pool.List(ExpertFilter{Type: ExpertGuanYu})
	if len(guanyu) != 1 {
		t.Errorf("期望1位关羽，实际有 %d 位", len(guanyu))
	}

	// 按技能过滤
	collectors := pool.List(ExpertFilter{Skills: []string{"data_collection"}})
	if len(collectors) != 1 || collectors[0].ID != "guanyu" {
		t.Errorf("期望按技能过滤出关羽，实际: %v", collectors)
	}
}

func TestMoERouterTopK(t *testing.T) {
	pool := NewMoEExpertPool()
	router := NewMoERouter(pool, nil)

	// Top-1 路由：应激活关羽（唯一匹配 data_collection）
	decision, err := router.Route(context.Background(), RoutingInput{Skill: "data_collection"}, 1)
	if err != nil {
		t.Fatalf("路由失败: %v", err)
	}
	if len(decision.Activated) != 1 {
		t.Errorf("期望激活1位专家，实际 %d 位", len(decision.Activated))
	}
	if decision.Activated[0].ID != "guanyu" {
		t.Errorf("期望激活关羽，实际为 %s", decision.Activated[0].ID)
	}
}

func TestMoERouterWithBalancer(t *testing.T) {
	pool := NewMoEExpertPool()
	router := NewMoERouter(pool, nil)
	router.RegisterBalancer(NewQuantileBalancer(0.3))

	// 多个专家都有 writing 类技能时，应能选出 Top-1
	// 这里所有专家技能不同，所以用 data_processing 测试单匹配
	decision, err := router.Route(context.Background(), RoutingInput{Skill: "data_processing"}, 1)
	if err != nil {
		t.Fatalf("路由失败: %v", err)
	}
	if len(decision.Activated) != 1 || decision.Activated[0].ID != "zhangfei" {
		t.Errorf("期望激活张飞，实际: %v", decision.Activated)
	}
}

func TestMoERouterNoMatch(t *testing.T) {
	pool := NewMoEExpertPool()
	router := NewMoERouter(pool, nil)

	decision, err := router.Route(context.Background(), RoutingInput{Skill: "nonexistent_skill"}, 2)
	if err != nil {
		t.Fatalf("路由不应返回错误: %v", err)
	}
	if len(decision.Activated) != 0 {
		t.Errorf("无匹配时应激活0位专家，实际 %d 位", len(decision.Activated))
	}
}

func TestMoEExpertPoolExecute(t *testing.T) {
	pool := NewMoEExpertPool()
	ctx := context.Background()

	order := &cmd_center.MilitaryOrder{
		ID:   "test-order",
		Name: "测试任务",
	}

	report, err := pool.Execute(ctx, "guanyu", order)
	if err != nil {
		t.Errorf("执行失败: %v", err)
	}
	if !report.Success {
		t.Errorf("执行应成功")
	}

	// 验证路由统计已更新
	guanyu, _ := pool.Get("guanyu")
	stats := guanyu.RouteStats.Snapshot()
	if stats.TotalRoutes != 1 {
		t.Errorf("期望 TotalRoutes=1，实际 %d", stats.TotalRoutes)
	}
	if stats.SuccessCount != 1 {
		t.Errorf("期望 SuccessCount=1，实际 %d", stats.SuccessCount)
	}
}

func TestMoEExpertPoolExecuteWithDecision(t *testing.T) {
	pool := NewMoEExpertPool()
	router := NewMoERouter(pool, nil)
	ctx := context.Background()

	order := &cmd_center.MilitaryOrder{
		ID:   "test-decision-order",
		Name: "决策测试任务",
	}

	report, err := RouteAndExecute(ctx, router, pool, RoutingInput{Skill: "data_collection"}, 1, order)
	if err != nil {
		t.Fatalf("路由并执行失败: %v", err)
	}
	if !report.Success {
		t.Errorf("期望执行成功，消息: %s", report.Message)
	}
	if len(report.Generals) != 1 {
		t.Errorf("期望1位专家战报，实际 %d 位", len(report.Generals))
	}
}

func TestQuantileBalancer(t *testing.T) {
	balancer := NewQuantileBalancer(0.3)
	stats := map[string]RouteStatsSnapshot{
		"expert-a": {ActiveLoads: 0},
		"expert-b": {ActiveLoads: 5},
		"expert-c": {ActiveLoads: 10},
	}
	rawScores := map[string]float64{
		"expert-a": 0.9,
		"expert-b": 0.9,
		"expert-c": 0.9,
	}
	adjusted := balancer.Adjust(rawScores, stats)
	// expert-a 负载最低，应得分最高
	if adjusted["expert-a"] <= adjusted["expert-c"] {
		t.Errorf("负载低的专家应得分更高: a=%v, c=%v", adjusted["expert-a"], adjusted["expert-c"])
	}
}

func TestExpertCapacity(t *testing.T) {
	pool := NewMoEExpertPool()
	// 注册一个容量为 1 的专家
	lowCapExpert := &Expert{
		ID:       "low-cap",
		Name:     "低容专家",
		Type:     ExpertGuanYu,
		Skills:   []string{"data_collection"},
		State:    ExpertIdle,
		Capacity: 1,
	}
	_ = pool.Register(lowCapExpert)
	// 复用关羽的 handler
	// 注意：此处仅验证容量控制，handler 复用不影响测试

	expert, _ := pool.Get("low-cap")
	// 模拟占满容量
	expert.RouteStats.ActiveLoads.Store(1)
	if tryAcquire(expert) {
		t.Errorf("容量已满时应获取失败")
	}
	// 释放后应可获取
	expert.RouteStats.ActiveLoads.Store(0)
	if !tryAcquire(expert) {
		t.Errorf("容量可用时应获取成功")
	}
	release(expert)
}
