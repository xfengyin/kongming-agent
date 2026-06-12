// Package cli 提供 Kongming 系统的命令行前端（cobra）。
//
// 本文件实现 `kongming dispatch` 子命令：从 flags 构造 Order 并派单。
//
// 关键设计：CLI 在 stage 4 阶段没有真实 Commander 注入，
// 通过 --dry-run 跳过 service 调用，仅打印待派送的 Order JSON，
// 方便用户在 stage 5 装配前调试与验证参数解析正确性。
package cli

import (
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/spf13/cobra"

	"github.com/zhuge/kongming/pkg/domain/model"
)

// ErrDispatchInvalidOrder 当必填字段缺失时返回。
var ErrDispatchInvalidOrder = errors.New("dispatch: order name is required (use --name)")

// newDispatchCmd 构造 `kongming dispatch` 子命令。
//
// Flags:
//
//	--name        必填，订单名
//	--priority    选填，1..4，默认 PriorityNormal
//	--objective   可重复，战略目标列表
//	--description 选填，订单描述
//	--dry-run     跳过 service 调用
func newDispatchCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "dispatch",
		Short: "Dispatch a new order to the commander",
		Long: `派发一次 Order 给军师（Commander）。

示例：
  kongming dispatch --name attack-chengdu --priority 4 --objective "夺城" --dry-run
  kongming dispatch -c configs/prod.yaml --name routine-task`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			name, _ := cmd.Flags().GetString("name")
			if strings.TrimSpace(name) == "" {
				return ErrDispatchInvalidOrder
			}
			priority, _ := cmd.Flags().GetInt("priority")
			description, _ := cmd.Flags().GetString("description")
			objectives, _ := cmd.Flags().GetStringSlice("objective")
			dryRun, _ := cmd.Flags().GetBool("dry-run")

			// 构造 Order：ID 自动用 uuid 保证全局唯一。
			order := &model.Order{
				ID:          model.OrderID(uuid.NewString()),
				Name:        name,
				Description: description,
				State:       model.StatePending,
				Priority:    model.Priority(priority),
				Strategy: model.Strategy{
					Objectives: objectives,
				},
				CreatedAt: time.Now(),
				UpdatedAt: time.Now(),
			}

			// dry-run 模式：仅打印 payload。
			if dryRun {
				return printJSON(cmd.OutOrStdout(), map[string]any{
					"action":  "dispatch",
					"order":   order,
					"dry_run": true,
				})
			}

			// 真实派单：依赖 service 注入；stage 4 阶段未装配则降级。
			svc, ok := ServiceFromContext(cmd.Context())
			if !ok || svc == nil || svc.Commander == nil {
				return ErrServiceNotWired
			}
			report, err := svc.Commander.Dispatch(cmd.Context(), order)
			if err != nil {
				return err
			}
			return printJSON(cmd.OutOrStdout(), map[string]any{
				"action": "dispatch",
				"order":  order,
				"report": report,
			})
		},
	}

	// Flags：name 必填，priority 默认 2（normal）。
	cmd.Flags().String("name", "", "order name (required)")
	cmd.Flags().Int("priority", int(model.PriorityNormal), "order priority (1=low, 2=normal, 3=high, 4=urgent)")
	cmd.Flags().String("description", "", "order description")
	// StringSliceP 支持 --objective a --objective b 多次声明。
	cmd.Flags().StringSlice("objective", nil, "strategic objective (repeatable)")
	return cmd
}
