// Package grpc 是 Kongming gRPC 传输层入口。
//
// 职责：
//  1. 装配 grpc.Server、注册 Kongming service、绑定 TCP 端口；
//  2. 注入 3 个核心拦截器（TraceID / Logging / Recovery），保证全链路可观测；
//  3. 暴露 Serve / GracefulStop 供 cmd/kongming-server 调用。
//
// 设计原则（六边形架构 + 依赖倒置）：
//   - Server 只通过 port.* 接口与业务交互，不直接依赖 application 实现；
//   - Deps 结构体作为「装配契约」，所有依赖都显式注入，避免隐式全局状态；
//   - 拦截器链顺序固定：TraceID（最外层）→ Logging → Recovery（最内层），
//     这样 panic 必然被 Recovery 兜底、日志包含 traceId、traceId 必先生成。
package grpc

import (
	"fmt"
	"net"

	"go.uber.org/zap"
	"google.golang.org/grpc"

	pb "github.com/zhuge/kongming/api/proto/kongming/v1"
	"github.com/zhuge/kongming/pkg/domain/port"
	"github.com/zhuge/kongming/pkg/transport/grpc/interceptor"
	"github.com/zhuge/kongming/pkg/transport/grpc/service"
)

// Server 封装 gRPC.Server 与监听器，对外提供 Serve / GracefulStop 能力。
//
// 字段全为小写：调用方通过 NewServer 构造后只使用方法，不应直接读写内部状态。
type Server struct {
	srv *grpc.Server
	lis net.Listener
}

// Deps 是 Server 装配阶段所需的所有依赖。
//
// 字段顺序与 NewServer 入参顺序一致，便于 IDE 提示与代码 review 时核对。
type Deps struct {
	// Commander 军师用例端口（派单 / 查询订单 / 审核）。
	Commander port.Commander
	// Dispatcher 调度器端口（异步派发 Order）。当前 Service 未直接使用，
	// 保留以备 RunWorkflow 内部异步化时直接接入。
	Dispatcher port.Dispatcher
	// Engine 工作流引擎端口（八卦阵执行）。
	Engine port.Engine
	// Pool 将领池端口（将领 CRUD / 派单 / 执行）。
	Pool port.GeneralPool
	// Vault 锦囊库端口（锦囊注册 / 查询 / 执行）。
	Vault port.Vault
	// Logger 结构化日志器；为 nil 时由拦截器降级为 zap.NewNop()。
	Logger *zap.Logger
	// Addr 监听地址（如 ":9090"），传给 net.Listen。
	Addr string
}

// NewServer 创建并装配一个 gRPC server。
//
// 行为：
//  1. net.Listen(d.Addr) 失败时返回 error（启动期就暴露问题，避免静默失败）；
//  2. grpc.NewServer 时按 TraceID→Logging→Recovery 顺序链式注册拦截器；
//  3. 注册 KongmingService（service.New）到 server；
//  4. 返回 *Server，调用方负责 Serve 与 GracefulStop。
//
// 注意：本函数不启动 server（不调用 Serve），由调用方在合适时机启动，
// 便于 main 流程先注册信号处理再 Serve。
func NewServer(d Deps) (*Server, error) {
	if d.Addr == "" {
		return nil, fmt.Errorf("grpc.NewServer: Addr is required")
	}
	lis, err := net.Listen("tcp", d.Addr)
	if err != nil {
		return nil, fmt.Errorf("grpc.NewServer: listen %q: %w", d.Addr, err)
	}

	logger := d.Logger
	if logger == nil {
		logger = zap.NewNop()
	}

	srv := grpc.NewServer(
		grpc.UnaryInterceptor(interceptor.Chain(
			interceptor.TraceID(),
			interceptor.Logging(logger),
			interceptor.Recovery(logger),
		)),
	)

	pb.RegisterKongmingServer(srv, service.New(
		d.Commander,
		d.Dispatcher,
		d.Engine,
		d.Pool,
		d.Vault,
	))

	return &Server{srv: srv, lis: lis}, nil
}

// Serve 启动 gRPC server 并阻塞直到 listener 关闭或返回非 nil error。
//
// 退出码语义：
//   - 返回 nil：理论上不会发生（Serve 成功会一直阻塞）；
//   - 返回非 nil：listener 关闭后返回的 error（通常是 http.ErrServerClosed 之外）。
//
// 一般在 main goroutine 中启动：
//
//	go func() { _ = grpcSrv.Serve() }()
func (s *Server) Serve() error {
	if s.srv == nil || s.lis == nil {
		return fmt.Errorf("grpc.Server.Serve: server is not initialized")
	}
	return s.srv.Serve(s.lis)
}

// GracefulStop 优雅停止 server：等待所有 in-flight RPC 完成后退出。
//
// 与 Stop 的区别：
//   - GracefulStop：阻塞等所有 RPC 完成；
//   - Stop：立即关闭，丢弃 in-flight 请求。
//
// 通常在收到 SIGTERM/SIGINT 时调用 GracefulStop，保证数据完整性。
func (s *Server) GracefulStop() {
	if s.srv == nil {
		return
	}
	s.srv.GracefulStop()
}

// Stop 立即停止 server（不等待 in-flight RPC）。
//
// 测试中常用；生产中建议用 GracefulStop。
func (s *Server) Stop() {
	if s.srv == nil {
		return
	}
	s.srv.Stop()
}
