// Kongming 五虎将测试

package generals

import (
	"context"
	"testing"
)

func TestWuHuPoolCount(t *testing.T) {
	pool := NewWuHuPool()
	count := pool.Count()
	if count != 5 {
		t.Errorf("期望5位将领，实际有 %d 位", count)
	}
}

func TestWuHuPoolList(t *testing.T) {
	pool := NewWuHuPool()

	// 列出所有
	all := pool.List(GeneralFilter{})
	if len(all) != 5 {
		t.Errorf("期望5位将领，实际有 %d 位", len(all))
	}

	// 按类型筛选
	guanyu := pool.List(GeneralFilter{Type: GeneralGuanYu})
	if len(guanyu) != 1 {
		t.Errorf("期望1位关羽，实际有 %d 位", len(guanyu))
	}
}

func TestWuHuPoolSelectBest(t *testing.T) {
	pool := NewWuHuPool()

	general, err := pool.SelectBest("data_collection")
	if err != nil {
		t.Errorf("选择将领失败: %v", err)
	}
	if general.ID != "guanyu" {
		t.Errorf("期望选择关羽，实际为 %s", general.ID)
	}
}

func TestWuHuPoolExecute(t *testing.T) {
	pool := NewWuHuPool()
	ctx := context.Background()

	order := &MilitaryOrder{
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
}
