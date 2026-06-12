// Package cli 提供 Kongming 系统的命令行前端（cobra）。
//
// 本文件实现 `kongming strategy` 子命令：战略规划预览。
//
// 子命令：
//
//	plan <order_id>   为指定 order 制定战略（输出 YAML）
//	list              列出已有战略（占位，stage 5 接入后实现）
//
// 与 dispatch 一致：stage 4 阶段没有真实 Commander 注入，
// --dry-run 跳过 service 调用，仅打印构造的占位 Order。
package cli

import (
	"errors"

	"github.com/spf13/cobra"

	"github.com/zhuge/kongming/pkg/domain/model"
)

// ErrStrategyMissingID 当 plan 子命令未传 order_id 时返回。
var ErrStrategyMissingID = errors.New("strategy plan: order_id is required")

// newStrategyCmd 构造 `kongming strategy` 子命令。
func newStrategyCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "strategy",
		Short: "Strategy planning operations",
		Long: `战略规划相关操作：
  plan <order_id>   为指定 order 制定战略（输出 YAML）
  list              列出已存在战略（stage 5 接入）`,
	}

	cmd.AddCommand(newStrategyPlanCmd())
	cmd.AddCommand(newStrategyListCmd())
	return cmd
}

// newStrategyPlanCmd 实现 `kongming strategy plan`。
//
// 实际语义：调用 Commander.PlanStrategy(ctx, order) 并把 *model.Strategy
// 以 YAML 形式打印，便于人工 review。
func newStrategyPlanCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "plan <order_id>",
		Short: "Plan a strategy for the given order",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			orderID := args[0]
			if orderID == "" {
				return ErrStrategyMissingID
			}
			dryRun, _ := cmd.Flags().GetBool("dry-run")

			// 构造占位 Order：真实场景应通过 OrderRepository.Get 拉取已存在的 order。
			// CLI 在 stage 4 没有 OrderRepository 注入，因此用零值 + 传入 ID 模拟。
			order := &model.Order{
				ID:   model.OrderID(orderID),
				Name: "strategy-plan",
			}

			if dryRun {
				// dry-run：打印占位 strategy YAML。
				return printYAML(cmd.OutOrStdout(), map[string]any{
					"action":  "strategy.plan",
					"order":   order,
					"dry_run": true,
					"strategy": model.Strategy{
						Type:       "offensive",
						Objectives: []string{"plan preview"},
						BaguaMode:  model.Tiangai,
					},
				})
			}

			svc, ok := ServiceFromContext(cmd.Context())
			if !ok || svc == nil || svc.Commander == nil {
				return ErrServiceNotWired
			}
			strategy, err := svc.Commander.PlanStrategy(cmd.Context(), order)
			if err != nil {
				return err
			}
			return printYAML(cmd.OutOrStdout(), map[string]any{
				"action":   "strategy.plan",
				"order_id": order.ID,
				"strategy": strategy,
			})
		},
	}
}

// newStrategyListCmd 实现 `kongming strategy list`（占位）。
//
// Stage 5 接入 OrderRepository 后可返回持久化的 strategy 列表。
// 当前 stage 4：直接返回空列表 + JSON 包裹。
func newStrategyListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List known strategies (stage 5 wired)",
		RunE: func(cmd *cobra.Command, _ []string) error {
			return printJSON(cmd.OutOrStdout(), map[string]any{
				"action":     "strategy.list",
				"strategies": []any{},
				"note":       "wired in stage 5",
			})
		},
	}
}
