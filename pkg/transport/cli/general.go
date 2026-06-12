// Package cli 提供 Kongming 系统的命令行前端（cobra）。
//
// 本文件实现 `kongming general` 子命令：将领池查询/管理。
//
// 子命令：
//
//	list    列出五虎将（dry-run 模式打印 5 个占位将领）
//	get <id> 查询单个将领
//
// stage 4 阶段无真实 GeneralPool 注入，依赖 --dry-run 验证参数解析。
package cli

import (
	"github.com/spf13/cobra"

	"github.com/zhuge/kongming/pkg/domain/model"
)

// newGeneralCmd 构造 `kongming general` 子命令。
func newGeneralCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "general",
		Short: "General pool operations (五虎将池)",
		Long: `将领池管理：
  list         列出全部将领
  get <id>     查询指定 ID 的将领`,
	}

	cmd.AddCommand(newGeneralListCmd())
	cmd.AddCommand(newGeneralGetCmd())
	return cmd
}

// newGeneralListCmd 实现 `kongming general list`。
//
// dry-run：输出 5 个占位将领（对应五虎将）；
// 真实模式：调用 GeneralPool.List 并以 JSON 形式打印。
func newGeneralListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List all generals",
		RunE: func(cmd *cobra.Command, _ []string) error {
			dryRun, _ := cmd.Flags().GetBool("dry-run")

			if dryRun {
				return printJSON(cmd.OutOrStdout(), map[string]any{
					"action":  "general.list",
					"dry_run": true,
					"generals": []model.General{
						{ID: model.GeneralID("guanyu"), Name: "关羽", Type: model.GeneralGuanYu, State: int(model.GeneralIdle)},
						{ID: model.GeneralID("zhangfei"), Name: "张飞", Type: model.GeneralZhangFei, State: int(model.GeneralIdle)},
						{ID: model.GeneralID("zhaoyun"), Name: "赵云", Type: model.GeneralZhaoYun, State: int(model.GeneralIdle)},
						{ID: model.GeneralID("machao"), Name: "马超", Type: model.GeneralMaChao, State: int(model.GeneralIdle)},
						{ID: model.GeneralID("huangzhong"), Name: "黄忠", Type: model.GeneralHuangZhong, State: int(model.GeneralIdle)},
					},
				})
			}

			svc, ok := ServiceFromContext(cmd.Context())
			if !ok || svc == nil || svc.Pool == nil {
				return ErrServiceNotWired
			}
			generals, err := svc.Pool.List(cmd.Context())
			if err != nil {
				return err
			}
			return printJSON(cmd.OutOrStdout(), map[string]any{
				"action":   "general.list",
				"generals": generals,
			})
		},
	}
}

// newGeneralGetCmd 实现 `kongming general get <id>`。
func newGeneralGetCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "get <id>",
		Short: "Get a general by id",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			id := args[0]
			dryRun, _ := cmd.Flags().GetBool("dry-run")

			if dryRun {
				return printJSON(cmd.OutOrStdout(), map[string]any{
					"action":  "general.get",
					"dry_run": true,
					"general": model.General{
						ID:    model.GeneralID(id),
						Name:  id,
						State: int(model.GeneralIdle),
					},
				})
			}

			svc, ok := ServiceFromContext(cmd.Context())
			if !ok || svc == nil || svc.Pool == nil {
				return ErrServiceNotWired
			}
			general, err := svc.Pool.Get(cmd.Context(), model.GeneralID(id))
			if err != nil {
				return err
			}
			return printJSON(cmd.OutOrStdout(), map[string]any{
				"action":  "general.get",
				"general": general,
			})
		},
	}
}
