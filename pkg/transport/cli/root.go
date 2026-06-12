// Package cli 提供 Kongming 系统的命令行前端（cobra）。
//
// 本文件实现根命令 NewRootCmd：创建 `kongming` 根命令，注册 6 个子命令
// （server / dispatch / strategy / general / vault / plugin）以及
// 持久化 flag `--config` / `-c`。
//
// 设计要点：
//  1. 依赖倒置：Service 容器由调用方通过 SetService 或 WithService 注入，
//     CLI 层不直接构造 application/infra 对象，方便 stage 5 顶层装配接入；
//  2. 配置驱动：所有子命令统一通过 --config 读配置，根命令持久化；
//  3. dry-run 友好：每个子命令支持 --dry-run，跳过真实 service 调用。
package cli

import (
	"context"

	"github.com/spf13/cobra"

	"github.com/zhuge/kongming/pkg/domain/port"
)

// serviceKey 是 ctx 中存储 Service 容器所用的非导出键类型。
//
// 使用类型作为 key 避免与外部 ctx value 冲突。
type serviceKey struct{}

// Service 是 CLI 子命令依赖的应用层端口容器。
//
// Stage 4.3 时 commander / pool / vault 字段可能为 nil（service 尚未装配），
// 子命令应通过 cli.ServiceFromContext 检查并降级到 dry-run 模式；
// stage 5 接入 kongming.New 后由 cmd/kongming/main.go 注入真实实现。
//
// 字段分组：
//   - 业务依赖：Commander / Pool / Vault / Plugin
//   - 配置：ConfigPath（--config 解析后的最终路径）
type Service struct {
	// Commander 军师用例端口（派单/战报/查询）。
	Commander port.Commander
	// Pool 将领池端口。
	Pool port.GeneralPool
	// Vault 锦囊库端口。
	Vault port.Vault
	// Plugin 插件注册中心（来自 infra/plugin.Registry，由 stage 5 接入）。
	Plugin *PluginAdapter
	// ConfigPath 已解析的配置文件绝对路径。
	ConfigPath string
}

// PluginAdapter 是 CLI 层对 plugin.Registry 的薄包装。
//
// CLI 不直接依赖 infra/plugin 包（避免 transport → infra 倒置违反），
// 而是定义一个最小接口，由 stage 5 顶层装配时把 *plugin.Registry 适配进来。
type PluginAdapter struct {
	// List 返回已注册插件名列表。
	ListFn func() []string
	// Load 从指定路径加载插件。
	LoadFn func(path string) error
}

// NewRootCmd 构造根命令并注册 6 个子命令。
//
// 工厂函数：每次调用都生成新 cobra.Command，避免测试间共享状态。
// persistent flag `--config` / `-c` 默认指向 configs/kongming.yaml，
// 与 kongming.New(cfgPath) 的约定保持一致。
func NewRootCmd() *cobra.Command {
	root := &cobra.Command{
		Use:   "kongming",
		Short: "Kongming 孔明军师系统（多 Agent 编排框架）",
		Long: `Kongming 是面向企业级场景的多 Agent 编排框架。

子命令：
  server      启动 HTTP+gRPC 服务
  dispatch    派发一次 Order（派单）
  strategy    战略规划/查询
  general     将领池管理（list/get）
  vault       锦囊库管理（list/exec）
  plugin      插件管理（list/load）`,
		// SilenceUsage/Errors: 错误时不打印 usage 干扰（cobra 默认会刷 usage）。
		SilenceUsage:  true,
		SilenceErrors: true,
	}

	// 持久化 flag：所有子命令都能拿到。
	root.PersistentFlags().StringP("config", "c", "configs/kongming.yaml", "config file path")

	// 全局 --dry-run：跳过真实 service 调用，仅打印 payload。
	// 子命令可自行覆盖语义。
	root.PersistentFlags().Bool("dry-run", false, "skip real service calls, just print payload")

	// 注册 6 个子命令。
	root.AddCommand(
		newServerCmd(),
		newDispatchCmd(),
		newStrategyCmd(),
		newGeneralCmd(),
		newVaultCmd(),
		newPluginCmd(),
	)
	return root
}

// WithService 返回一个新 ctx，把 svc 注入其中。
//
// 使用方式：kongming CLI 入口在 main() 派生 ctx 时调用一次，
// 所有子命令通过 ServiceFromContext(cmd.Context()) 拿到依赖。
func WithService(ctx context.Context, svc *Service) context.Context {
	return context.WithValue(ctx, serviceKey{}, svc)
}

// ServiceFromContext 从 ctx 取出 CLI 注入的 Service 容器。
//
// 返回 (nil, false) 表示 ctx 中无 Service（CLI 入口未注入）。
// 子命令应据此判断是否降级到 dry-run。
func ServiceFromContext(ctx context.Context) (*Service, bool) {
	if ctx == nil {
		return nil, false
	}
	svc, ok := ctx.Value(serviceKey{}).(*Service)
	return svc, ok && svc != nil
}
