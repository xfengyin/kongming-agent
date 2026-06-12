// Package cli 提供 Kongming 系统的命令行前端（cobra）。
//
// 本文件实现 `kongming plugin` 子命令：插件管理。
//
// 子命令：
//
//	list      列出已注册插件
//	load      从指定路径加载插件（占位，stage 5/6 接入 plugin registry）
//
// Stage 4 阶段 plugin registry 尚未注入；--dry-run 验证参数解析。
// 真实插件加载/热更新能力由 pkg/infra/plugin 在 stage 5/6 接入。
package cli

import (
	"errors"

	"github.com/spf13/cobra"
)

// ErrPluginMissingPath 当 load 子命令未传 --path 时返回。
var ErrPluginMissingPath = errors.New("plugin load: --path is required")

// newPluginCmd 构造 `kongming plugin` 子命令。
func newPluginCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "plugin",
		Short: "Plugin management (SPI registry)",
		Long: `插件管理：
  list           列出已注册插件
  load --path    从指定路径加载插件（stage 5/6 接入）`,
	}

	cmd.AddCommand(newPluginListCmd())
	cmd.AddCommand(newPluginLoadCmd())
	return cmd
}

// newPluginListCmd 实现 `kongming plugin list`。
//
// Stage 4：dry-run 返回空列表；真实模式通过 Service.Plugin.ListFn 读取。
func newPluginListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List registered plugins",
		RunE: func(cmd *cobra.Command, _ []string) error {
			dryRun, _ := cmd.Flags().GetBool("dry-run")

			if dryRun {
				return printJSON(cmd.OutOrStdout(), map[string]any{
					"action":  "plugin.list",
					"dry_run": true,
					"plugins": []string{},
				})
			}

			svc, ok := ServiceFromContext(cmd.Context())
			if !ok || svc == nil || svc.Plugin == nil || svc.Plugin.ListFn == nil {
				return ErrServiceNotWired
			}
			return printJSON(cmd.OutOrStdout(), map[string]any{
				"action":  "plugin.list",
				"plugins": svc.Plugin.ListFn(),
			})
		},
	}
}

// newPluginLoadCmd 实现 `kongming plugin load --path <path>`（占位）。
//
// Stage 4：dry-run 打印 path；真实模式调用 Service.Plugin.LoadFn。
// 真实热更新由 pkg/infra/plugin.Watcher 负责。
func newPluginLoadCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "load",
		Short: "Load a plugin from the given path",
		RunE: func(cmd *cobra.Command, _ []string) error {
			path, _ := cmd.Flags().GetString("path")
			if path == "" {
				return ErrPluginMissingPath
			}
			dryRun, _ := cmd.Flags().GetBool("dry-run")

			if dryRun {
				return printJSON(cmd.OutOrStdout(), map[string]any{
					"action":  "plugin.load",
					"dry_run": true,
					"path":    path,
				})
			}

			svc, ok := ServiceFromContext(cmd.Context())
			if !ok || svc == nil || svc.Plugin == nil || svc.Plugin.LoadFn == nil {
				return ErrServiceNotWired
			}
			if err := svc.Plugin.LoadFn(path); err != nil {
				return err
			}
			return printJSON(cmd.OutOrStdout(), map[string]any{
				"action": "plugin.load",
				"path":   path,
				"status": "ok",
			})
		},
	}
	cmd.Flags().String("path", "", "plugin path (required)")
	return cmd
}
