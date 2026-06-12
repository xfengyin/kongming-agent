// Package kongming 是 Kongming 系统的顶层装配入口。
//
// 设计原则（对齐 12 条企业规则）：
//  1. 开闭原则（规则 1）：新增依赖只新增字段与装配步骤，不修改现有 API。
//  2. 依赖倒置（规则 2）：本包只引用 port.* 接口与具体实现的构造函数，
//     不感知 transport / infra 内部细节。
//  3. 单一职责（规则 3）：本包只负责「按配置把各子模块装配为 Kongming」，
//     不实现任何业务逻辑。
//  4. 接口隔离（规则 4）：New 暴露的 Options 字段按需扩展。
//  5. 高可用（规则 5）：所有依赖构造允许 nil-fallback；Run 失败时调用
//     Shutdown 保证资源回收。
//  6. 可观测（规则 6）：装配阶段不主动打日志；运行阶段通过 observer 注入 ctx。
//  7. 配置驱动（规则 7）：装配顺序固定，仅按 Options / config 路径微调。
//  8. 插件化（规则 8）：pluginReg 预留，stage 6/7 扩展时直接通过 Options 注入。
//  9. 幂等一致性（规则 9）：Shutdown 多次调用安全；New 重复调用互不影响。
//  10. 安全合规（规则 10）：默认不暴露内部端口；所有关闭路径都返回 error
//     列表便于上层审计。
//
// 当前实现（Stage 5.1）：
//   - Pool 的 executor 传 nil（业务层暂未注入五虎将 handler，待 stage 6 接入）；
//   - Vault 不接 pluginReg（保留接口，stage 7 扩展）；
//   - Dispatcher / Courier 用硬编码合理默认值（workers=4, buffer=256）。
//     后续 stage 在不破坏 New(cfgPath) 签名的前提下扩展这些点。
package kongming

import (
	"context"
	"errors"
	"fmt"

	"go.uber.org/zap"

	"github.com/zhuge/kongming/pkg/application/commander"
	"github.com/zhuge/kongming/pkg/application/courier"
	"github.com/zhuge/kongming/pkg/application/dispatcher"
	"github.com/zhuge/kongming/pkg/application/general"
	"github.com/zhuge/kongming/pkg/application/vault"
	"github.com/zhuge/kongming/pkg/application/workflow"
	"github.com/zhuge/kongming/pkg/infra/config"
	"github.com/zhuge/kongming/pkg/infra/observability"
	"github.com/zhuge/kongming/pkg/infra/persistence/memory"
	"github.com/zhuge/kongming/pkg/infra/plugin"
	"github.com/zhuge/kongming/pkg/infra/resilience"
	grpcsrv "github.com/zhuge/kongming/pkg/transport/grpc"
	httpsrv "github.com/zhuge/kongming/pkg/transport/http"
)

// 顶层装配的硬编码默认值（可由 Options 覆盖，未来可迁移到 cfg.Xxx 子段）。
const (
	defaultCourierBuffer = 256
	defaultWorkers       = 4
)

// Kongming 是 Kongming 系统的顶层对象；持有所有运行时依赖与传输层 server。
//
// 字段全为小写：外部通过 New / NewWithOptions 构造，通过 Run / Shutdown 控制生命周期。
//
// 字段分组：
//   - 配置：cfg（root config）/ serviceName（用于日志 tag）
//   - 横切：logger / observer / resilient
//   - 业务：commander / dispatcher / engine / pool / vault / courier
//   - 扩展：pluginReg（stage 6/7 接入）
//   - 传输：httpSrv / grpcSrv（Run 阶段填充）
type Kongming struct {
	cfg         *config.Config
	serviceName string
	logger      *zap.Logger
	observer    *observability.Observer
	resilient   *resilience.Runner
	commander   *commander.Service
	dispatcher  *dispatcher.Dispatcher
	engine      *workflow.Runner
	pool        *general.Pool
	vault       *vault.Vault
	courier     *courier.Courier
	pluginReg   *plugin.Registry
	httpSrv     *httpsrv.Server
	grpcSrv     *grpcsrv.Server
}

// New 是顶层装配的快捷入口，等价于 NewWithOptions(cfgPath, Options{})。
func New(cfgPath string) (*Kongming, error) {
	return NewWithOptions(cfgPath, Options{})
}

// NewWithOptions 基于配置文件路径 + Options 构造一个未启动的 Kongming 实例。
//
// 装配顺序（不可换序）：
//  1. config.Load（所有后续步骤的输入）
//  2. observability.NewLogger（日志先行）
//  3. observability.NewObserver（依赖 logger）
//  4. resilience.NewRunner（依赖 logger；cfg → resilience.Config 转换）
//  5. memory.NewStore / NewOrderRepo（共享 Store）
//  6. plugin.NewRegistry
//  7. general.NewPool（executor 暂传 nil）
//  8. vault.NewVault
//  9. courier.NewCourier
//
// 10. workflow.NewRunner
// 11. dispatcher.NewDispatcher
// 12. commander.New（依赖最多，必须最后）
//
// 任意步骤失败：直接返回 error，已构造的依赖由 Go GC 回收（无全局副作用）。
func NewWithOptions(cfgPath string, opts Options) (*Kongming, error) {
	// 默认 Options
	if opts.ServiceName == "" {
		opts.ServiceName = "kongming"
	}

	// 1. 配置
	cfg, err := config.Load(cfgPath)
	if err != nil {
		return nil, fmt.Errorf("config load %q: %w", cfgPath, err)
	}

	// 2. logger
	logger, err := observability.NewLogger(cfg.Observatory.Log)
	if err != nil {
		return nil, fmt.Errorf("logger: %w", err)
	}

	// 3. observer（ctx 仅作为 tracer 初始化占位；OTLP exporter 由 cfg.Tracing.Enabled 控制）
	obs, err := observability.NewObserver(context.Background(), cfg.Observatory, logger)
	if err != nil {
		return nil, fmt.Errorf("observer: %w", err)
	}

	// 4. 弹性执行器（resilience.Config 是其内部类型，需做字段映射）
	res := resilience.NewRunner(toResilienceConfig(cfg.Resilience), logger)

	// 5. 进程内仓库
	store := memory.NewStore()
	orderRepo := memory.NewOrderRepo(store)

	// 6. 插件注册中心（空起步；stage 6/7 扩展）
	pluginReg := plugin.NewRegistry()

	// 7. 将领池：executor 暂传 nil（service.New 内部 nil-check 已兼容）。
	pl := general.NewPool(logger, obs, res, nil)

	// 8. 锦囊库：当前实现未接 pluginReg（保持最小可用）。
	vlt := vault.NewVault(logger, obs)

	// 9. 传令兵：buffer 默认 256，可由 Options.CourierBuffer 覆盖。
	buffer := opts.CourierBuffer
	if buffer <= 0 {
		buffer = defaultCourierBuffer
	}
	cou := courier.NewCourier(buffer, logger)

	// 10. 工作流引擎
	eng := workflow.NewRunner(logger)

	// 11. 调度器：worker 数走 Options 默认值。
	workers := opts.DispatcherWorkers
	if workers <= 0 {
		workers = defaultWorkers
	}
	disp := dispatcher.NewDispatcher(workers, logger)

	// 12. 军师（依赖最广，必须最后）
	cmd := commander.New(
		commander.NewDefaultPlanner(),
		pl, eng, vlt, orderRepo,
		res, obs, logger,
	)

	return &Kongming{
		cfg:         cfg,
		serviceName: opts.ServiceName,
		logger:      logger,
		observer:    obs,
		resilient:   res,
		commander:   cmd,
		dispatcher:  disp,
		engine:      eng,
		pool:        pl,
		vault:       vlt,
		courier:     cou,
		pluginReg:   pluginReg,
	}, nil
}

// Run 启动 HTTP + gRPC server 并阻塞，直到 ctx 取消或某 server 异常退出。
//
// 行为：
//  1. 装配 HTTP server（同步注册路由，不绑定端口）；
//  2. 装配 gRPC server（含 net.Listen，启动期失败立即返回）；
//  3. 后台 goroutine 跑 ListenAndServe / Serve；
//  4. 阻塞等待 ctx.Done() 或 HTTP server 异常；
//  5. 退出时调用 Shutdown(context.Background()) 释放资源。
func (k *Kongming) Run(ctx context.Context) error {
	httpAddr := fmt.Sprintf("%s:%d", k.cfg.Server.Host, k.cfg.Server.Port)
	k.httpSrv = httpsrv.NewServer(httpsrv.Deps{
		Commander:  k.commander,
		Dispatcher: k.dispatcher,
		Engine:     k.engine,
		Pool:       k.pool,
		Vault:      k.vault,
		Observer:   k.observer,
		Logger:     k.logger,
		Addr:       httpAddr,
	})

	httpErrCh := make(chan error, 1)
	go func() { httpErrCh <- k.httpSrv.ListenAndServe() }()

	grpcAddr := fmt.Sprintf("%s:%d", k.cfg.Server.Host, k.cfg.Server.GRPCPort)
	grpcSrv, err := grpcsrv.NewServer(grpcsrv.Deps{
		Commander:  k.commander,
		Dispatcher: k.dispatcher,
		Engine:     k.engine,
		Pool:       k.pool,
		Vault:      k.vault,
		Logger:     k.logger,
		Addr:       grpcAddr,
	})
	if err != nil {
		// gRPC 启动期失败时也要尽力关闭已启动的 HTTP。
		_ = k.httpSrv.Shutdown(context.Background())
		return fmt.Errorf("grpc server: %w", err)
	}
	k.grpcSrv = grpcSrv
	go func() { _ = k.grpcSrv.Serve() }()

	k.logger.Info("kongming started",
		zap.String("service", k.serviceName),
		zap.String("http_addr", httpAddr),
		zap.String("grpc_addr", grpcAddr),
	)

	// 阻塞：ctx 取消 或 HTTP 异常。
	select {
	case <-ctx.Done():
		k.logger.Info("kongming shutting down by signal",
			zap.String("service", k.serviceName))
		return k.Shutdown(context.Background())
	case err := <-httpErrCh:
		// HTTP 异常也要把 gRPC 一起停掉。
		_ = k.Shutdown(context.Background())
		if err != nil && !errors.Is(err, context.Canceled) {
			return fmt.Errorf("http server: %w", err)
		}
		return nil
	}
}

// Shutdown 优雅停机：按 httpSrv → grpcSrv → observer 顺序释放资源。
//
// 关闭错误用 errors.Join 聚合向上返回（Go 1.20+），便于上层审计。
// Run 之前调用安全（httpSrv / grpcSrv 为 nil 时直接跳过）。
// 多次调用安全：observer.Shutdown 内部幂等。
func (k *Kongming) Shutdown(ctx context.Context) error {
	var errs []error

	if k.httpSrv != nil {
		if err := k.httpSrv.Shutdown(ctx); err != nil {
			errs = append(errs, fmt.Errorf("http shutdown: %w", err))
		}
	}
	if k.grpcSrv != nil {
		k.grpcSrv.GracefulStop()
	}
	if k.observer != nil {
		if err := k.observer.Shutdown(ctx); err != nil {
			errs = append(errs, fmt.Errorf("observer shutdown: %w", err))
		}
	}
	if k.logger != nil {
		// logger.Sync 失败常见于非 TTY 场景（stderr 写入 "invalid argument"），
		// 按规范忽略：进程退出时日志已落 stdout，无须再 flush。
		_ = k.logger.Sync()
	}
	if len(errs) > 0 {
		return errors.Join(errs...)
	}
	return nil
}

// ServiceName 返回当前实例的服务名（用于日志 tag / metrics label）。
func (k *Kongming) ServiceName() string { return k.serviceName }

// toResilienceConfig 把 config.ResilienceConfig 映射到 resilience.Config。
//
// resilience 包定义了内部 Config 类型以避免跨子包循环依赖，
// 故此处做手动字段拷贝。新增子配置时需同步更新本函数。
func toResilienceConfig(in config.ResilienceConfig) resilience.Config {
	return resilience.Config{
		Retry: resilience.RetryConfig{
			MaxAttempts:    in.Retry.MaxAttempts,
			InitialBackoff: in.Retry.InitialBackoff,
			MaxBackoff:     in.Retry.MaxBackoff,
			BackoffFactor:  in.Retry.BackoffFactor,
			Jitter:         in.Retry.Jitter,
		},
		CircuitBreaker: resilience.CircuitBreakerConfig{
			Threshold: in.CircuitBreaker.Threshold,
			Timeout:   in.CircuitBreaker.Timeout,
		},
		RateLimit: resilience.RateLimitConfig{
			RPS:   in.RateLimit.RPS,
			Burst: in.RateLimit.Burst,
		},
	}
}
