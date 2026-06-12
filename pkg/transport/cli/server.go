// Package cli 提供 Kongming 系统的命令行前端（cobra）。
//
// 本文件实现 `kongming server` 子命令：启动 HTTP + gRPC 服务。
//
// Stage 4.3 状态：server 启动依赖 pkg/kongming.Kongming.Run，
// 后者在 stage 5（顶层装配）才会创建。本任务先建立 cobra.Command 框架，
// RunE 在 service 未注入时返回明确错误信息，避免在 stage 4 阶段误启动。
package cli

import (
	"errors"

	"github.com/spf13/cobra"
)

// ErrServiceNotWired 当 CLI 调用了 server 但 Service 容器未注入时返回。
//
// 让用户明确知道 stage 5 才会完成接入。
var ErrServiceNotWired = errors.New("kongming server: service not wired, see stage 5")

// newServerCmd 构造 `kongming server` 子命令。
//
// 当前实现仅做 dry-run 打印 + service 注入校验；
// 真实启动逻辑在 stage 5 接入 kongming.New(cfg).Run(ctx) 时替换。
func newServerCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "server",
		Short: "Start HTTP+gRPC server",
		Long: `启动 Kongming 服务（HTTP + gRPC）。

配置由 --config 指定，默认 configs/kongming.yaml。
当前 stage 4.3 仅提供命令框架，真实装配将在 stage 5 接入。`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			cfgPath, _ := cmd.Flags().GetString("config")
			dryRun, _ := cmd.Flags().GetBool("dry-run")

			// dry-run 模式：仅打印将要启动的 service 配置，便于调试。
			if dryRun {
				return printJSON(cmd.OutOrStdout(), map[string]any{
					"action":  "server.start",
					"config":  cfgPath,
					"dry_run": true,
				})
			}

			// service 注入检查：stage 4 阶段未装配，返回明确错误。
			svc, ok := ServiceFromContext(cmd.Context())
			if !ok || svc == nil {
				return ErrServiceNotWired
			}
			// 真实启动逻辑将在 stage 5 接入，TODO 提示位置。
			// 当前实现：返回错误以提示用户。
			_ = svc
			return ErrServiceNotWired
		},
	}
}
