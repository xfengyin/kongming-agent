// Kongming 快速开始
// 运筹帷幄之中，决胜千里之外

package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/zhuge/kongming/pkg/cmd_center"
	"github.com/zhuge/kongming/pkg/generals"
	"github.com/zhuge/kongming/pkg/strategy_vault"
	"go.uber.org/zap"
)

func main() {
	logger, _ := zap.NewDevelopment()
	defer logger.Sync()
	_ = logger

	ctx := context.Background()

	fmt.Println("=== 诸葛孔明系统 - 快速开始 ===")
	fmt.Println()

	// 1. 初始化军师府
	fmt.Println("📜 初始化军师府...")
	commander := cmd_center.NewCommander(logger)
	fmt.Println("✓ 军师府已开张")

	// 2. 查看五虎将
	fmt.Println()
	fmt.Println("⚔️  五虎将待命中...")
	pool := generals.NewWuHuPool()
	wuHu := pool.List(generals.GeneralFilter{})
	for _, g := range wuHu {
		fmt.Printf("  • %s（%s）- %s\n", g.Name, g.Title, g.Description)
	}

	// 3. 查看锦囊库
	fmt.Println()
	fmt.Println("🎁 锦囊库已备好...")

	// 4. 颁布军令
	fmt.Println()
	fmt.Println("📋 颁布军令：市场调研任务")

	order := cmd_center.NewMilitaryOrder(
		"市场调研",
		"调研智能硬件市场现状",
		cmd_center.PriorityNormal,
	)
	order.Strategy.Objectives = []string{
		"收集竞品信息",
		"分析用户需求",
		"输出调研报告",
	}

	// 派遣执行
	fmt.Println("⚔️  调兵遣将中...")
	report, err := commander.Dispatch(ctx, order)
	if err != nil {
		log.Fatalf("任务执行失败: %v", err)
	}

	// 5. 输出战报
	fmt.Println()
	fmt.Println("=== 战报 ===")
	fmt.Printf("任务: %s\n", order.Name)
	fmt.Printf("状态: %v\n", report.Success)
	fmt.Printf("执行时间: %v\n", report.CompletedAt.Sub(report.StartedAt))
	fmt.Println()
	fmt.Println("将领战功:")
	for _, gr := range report.Generals {
		status := "✓"
		if !gr.Success {
			status = "✗"
		}
		fmt.Printf("  %s %s: %s\n", status, gr.GeneralName, gr.Message)
	}

	fmt.Println()
	fmt.Println("=== 演示完成 ===")

	// 锦囊演示
	fmt.Println()
	fmt.Println("🎁 锦囊演示...")
	vault := strategy_vault.NewVault()
	vault.RegisterSkill("data_analysis", &DataAnalysisSkill{})

	result, err := vault.Execute(ctx, "data_analysis", strategy_vault.JinnangInput{
		Params: map[string]interface{}{
			"data": []int{1, 2, 3, 4, 5},
		},
	})
	if err != nil {
		log.Printf("锦囊执行失败: %v", err)
	} else {
		fmt.Printf("✓ 锦囊执行成功: %v\n", result.Data)
	}

	time.Sleep(100 * time.Millisecond)
}

// DataAnalysisSkill 数据分析技能
type DataAnalysisSkill struct{}

func (s *DataAnalysisSkill) Name() string { return "data_analysis" }

func (s *DataAnalysisSkill) Execute(ctx context.Context, input map[string]interface{}) (map[string]interface{}, error) {
	data, ok := input["data"].([]int)
	if !ok {
		return nil, fmt.Errorf("需要整数数组")
	}
	sum := 0
	for _, v := range data {
		sum += v
	}
	return map[string]interface{}{
		"count": len(data),
		"sum":   sum,
		"avg":   float64(sum) / float64(len(data)),
	}, nil
}
