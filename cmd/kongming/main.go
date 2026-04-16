// Kongming 孔明军师系统
// 运筹帷幄之中，决胜千里之外

package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/zhuge/kongming/internal/memory"
	"github.com/zhuge/kongming/pkg/bagua"
	"github.com/zhuge/kongming/pkg/cmd_center"
	"github.com/zhuge/kongming/pkg/courier"
	"github.com/zhuge/kongming/pkg/dispatch"
	"github.com/zhuge/kongming/pkg/generals"
	"github.com/zhuge/kongming/pkg/observatory"
	"github.com/zhuge/kongming/pkg/strategy_vault"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// Config 应用配置
type Config struct {
	Server   ServerConfig
	Features FeaturesConfig
}

type ServerConfig struct {
	Host string
	Port int
}

type FeaturesConfig struct {
	EnableMetrics    bool
	EnableTracing    bool
	EnableObservatory bool
}

var logger *zap.Logger

func main() {
	initLogger()
	defer logger.Sync()

	logger.Info("诸葛孔明系统启动")
	logger.Info("运筹帷幄之中，决胜千里之外")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	zhuge := NewZhugeKongming(ctx)
	go zhuge.Start(ctx)

	gracefulShutdown(zhuge)
}

func initLogger() {
	cfg := zap.Config{
		Level:            zap.NewAtomicLevelAt(zap.InfoLevel),
		Development:      false,
		Encoding:         "json",
		EncoderConfig:     zapcore.EncoderConfig{
			TimeKey:        "ts",
			LevelKey:       "level",
			NameKey:        "logger",
			CallerKey:      "caller",
			FunctionKey:    zapcore.OmitKey,
			MessageKey:     "msg",
			StacktraceKey:  "stacktrace",
			LineEnding:     zapcore.DefaultLineEnding,
			EncodeLevel:    zapcore.LowercaseLevelEncoder,
			EncodeTime:     zapcore.ISO8601TimeEncoder,
			EncodeDuration: zapcore.SecondsDurationEncoder,
			EncodeCaller:    zapcore.ShortCallerEncoder,
		},
		OutputPaths:      []string{"stdout"},
		ErrorOutputPaths: []string{"stderr"},
	}
	logger, _ = cfg.Build()
}

// ZhugeKongming 诸葛孔明
type ZhugeKongming struct {
	ctx             context.Context
	cmdCenter       cmd_center.Commander
	generalPool     generals.GeneralPool
	strategyVault   strategy_vault.Vault
	baguaEngine     *bagua.Engine
	courierService  *courier.Courier
	dispatcher      *dispatch.Dispatcher
	observatory     *observatory.Observatory
	metricsServer   *http.Server
	shutdownFuncs   []func() error
}

// NewZhugeKongming 创建诸葛孔明实例
func NewZhugeKongming(ctx context.Context) *ZhugeKongming {
	zk := &ZhugeKongming{
		ctx:             ctx,
		cmdCenter:       cmd_center.NewCommander(logger),
		generalPool:     generals.NewWuHuPool(),
		strategyVault:   strategy_vault.NewVault(),
		baguaEngine:     bagua.NewEngine(),
		courierService: courier.NewCourier(logger),
		dispatcher:      dispatch.NewDispatcher(logger),
		observatory:     observatory.NewObservatory(),
	}

	zk.shutdownFuncs = append(zk.shutdownFuncs,
		func() error {
			logger.Info("关闭军师府")
			return nil
		},
	)

	return zk
}

// Start 启动服务
func (zk *ZhugeKongming) Start(ctx context.Context) {
	go zk.startMetrics()
	go zk.startObservatory()
	logger.Info("军师府开张")
}

// startMetrics 启动指标服务
func (zk *ZhugeKongming) startMetrics() {
	mux := http.NewServeMux()
	mux.Handle("/metrics", promhttp.Handler())
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, "OK")
	})
	mux.HandleFunc("/ready", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, "Ready")
	})

	addr := ":9090"
	zk.metricsServer = &http.Server{
		Addr:         addr,
		Handler:      mux,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
	}

	logger.Info("指标服务启动", zap.String("addr", addr))
	if err := zk.metricsServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		logger.Error("指标服务异常", zap.Error(err))
	}
}

// startObservatory 启动观测服务
func (zk *ZhugeKongming) startObservatory() {
	if err := zk.observatory.Start(zk.ctx); err != nil {
		logger.Error("观测服务启动失败", zap.Error(err))
	}
}

// gracefulShutdown 优雅退出
func gracefulShutdown(zhuge *ZhugeKongming) {
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	<-sigChan

	logger.Info("收到退出信号，开始优雅关闭")

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	for _, fn := range zhuge.shutdownFuncs {
		if err := fn(); err != nil {
			logger.Error("清理失败", zap.Error(err))
		}
	}

	if zhuge.metricsServer != nil {
		zhuge.metricsServer.Shutdown(ctx)
	}

	logger.Info("军师府打烊，后会有期")
}
