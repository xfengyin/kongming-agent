// Package http 提供基于 gin 的 HTTP 传输层实现。
//
// 设计原则（spec §5.1）：
//  1. 依赖倒置：server 只依赖 domain/port.* 端口接口，不直接 import application/*。
//  2. 中间件链：Recovery → TraceID → Logging → CORS（顺序敏感）。
//  3. 路由表：/healthz /readyz /metrics 走顶层，/api/v1/* 走子 group。
//  4. 优雅停机：Shutdown 透传到 http.Server.Shutdown。
package http

import (
	"context"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.uber.org/zap"

	"github.com/zhuge/kongming/pkg/domain/port"
	"github.com/zhuge/kongming/pkg/transport/http/handler"
	"github.com/zhuge/kongming/pkg/transport/http/middleware"
)

// Server 封装 gin.Engine + 标准 http.Server，提供 ListenAndServe / Shutdown 能力。
//
// 持有 engine 是为了测试可绕过 ListenAndServe，直接用 httptest 调用 engine.ServeHTTP。
type Server struct {
	engine *gin.Engine
	srv    *http.Server
	logger *zap.Logger
}

// Deps 是 server 装配所需的全部依赖。
//
// 任一字段为 nil 时 NewServer 不报错（按需 mock；handler 内对 nil 端口做了防御），
// 但 listener 启动后调用对应路由会 panic——属预期：测试用 Server.Engine() 直接打。
type Deps struct {
	// Commander 军师端口（派单/战略/订单查询）。
	Commander port.Commander
	// Dispatcher 调度器端口（异步派发）；本任务未使用 handler 暴露，保留装配位。
	Dispatcher port.Dispatcher
	// Engine 工作流引擎端口。
	Engine port.Engine
	// Pool 将领池端口。
	Pool port.GeneralPool
	// Vault 锦囊库端口。
	Vault port.Vault
	// Observer 可观测性端口（注入 ctx span/metrics）。
	Observer port.Observer
	// Logger zap 日志器。
	Logger *zap.Logger
	// Addr 监听地址（如 ":8080"）。
	Addr string
}

// NewServer 构造 HTTP server，注册全部路由 + 中间件。
//
// 中间件顺序：Recovery（最外）→ TraceID → Logging → CORS（最里）。
// - Recovery 必须最外层，才能捕获后续所有 panic。
// - TraceID 必须在 Logging 之前，Logging 才能从 ctx 读 traceId。
// - CORS 在最里层，预检请求可被前置中间件处理（实际 OPTIONS 由 CORS 自身 short-circuit）。
func NewServer(d Deps) *Server {
	gin.SetMode(gin.ReleaseMode)
	e := gin.New()
	e.Use(middleware.Recovery(d.Logger))
	e.Use(middleware.TraceID())
	e.Use(middleware.Logging(d.Logger))
	e.Use(middleware.CORS())

	// 构造 handler 聚合。
	h := handler.New(d.Commander, d.Dispatcher, d.Engine, d.Pool, d.Vault, d.Observer, d.Logger)

	// 业务路由（/api/v1）。
	api := e.Group("/api/v1")
	{
		api.POST("/orders", h.CreateOrder)
		api.GET("/orders", h.ListOrders)
		api.GET("/orders/:id", h.GetOrder)
		api.POST("/strategies", h.PlanStrategy)
		api.GET("/generals", h.ListGenerals)
		api.GET("/generals/:id", h.GetGeneral)
		api.GET("/vault", h.ListJinnang)
		api.POST("/vault/:id/exec", h.ExecuteJinnang)
		api.POST("/workflows/:id/run", h.RunWorkflow)
	}

	// 健康/就绪/metrics 路由（顶层）。
	e.GET("/healthz", h.Healthz)
	e.GET("/readyz", h.Readyz)
	// /metrics 直接挂 promhttp.Handler：使用 default registry 暴露所有
	// 已注册的指标（go_* / process_* / 自定义 metric）。
	// 注意：本任务不依赖 port.Observer.Handler()（Observer 接口未暴露 Handler）。
	e.GET("/metrics", gin.WrapH(promhttp.Handler()))

	return &Server{
		engine: e,
		logger: d.Logger,
		srv: &http.Server{
			Addr:         d.Addr,
			Handler:      e,
			ReadTimeout:  30 * time.Second,
			WriteTimeout: 30 * time.Second,
			IdleTimeout:  60 * time.Second,
		},
	}
}

// Engine 返回内部 gin.Engine，便于 httptest 直接打（不走 ListenAndServe）。
func (s *Server) Engine() *gin.Engine { return s.engine }

// ListenAndServe 阻塞监听并服务 HTTP 请求；返回的 error 在 Listen 失败时非 nil。
func (s *Server) ListenAndServe() error { return s.srv.ListenAndServe() }

// Shutdown 优雅停机，等待 in-flight 请求完成（受 ctx 超时约束）。
func (s *Server) Shutdown(ctx context.Context) error { return s.srv.Shutdown(ctx) }
