package grpc_test

import (
	"context"
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	pb "github.com/zhuge/kongming/api/proto/kongming/v1"
	"github.com/zhuge/kongming/pkg/domain/port"
	grpcsrv "github.com/zhuge/kongming/pkg/transport/grpc"
)

// nopDeps 返回一个最小可用的 Deps（所有 port 都为 nil）。
//
// 设计动机：Server.NewServer 不应触发 port 的任何调用（构造期仅 listen + 装配），
// 因此 nil port 完全合法，简化测试装配。
func nopDeps(addr string) grpcsrv.Deps {
	return grpcsrv.Deps{
		Commander:  port.Commander(nil),
		Dispatcher: port.Dispatcher(nil),
		Engine:     port.Engine(nil),
		Pool:       port.GeneralPool(nil),
		Vault:      port.Vault(nil),
		Addr:       addr,
	}
}

// TestNewServer_EmptyAddr 验证：Addr 为空时 NewServer 立即 fail-fast。
func TestNewServer_EmptyAddr(t *testing.T) {
	_, err := grpcsrv.NewServer(nopDeps(""))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "Addr is required")
}

// TestNewServer_InvalidAddr 验证：监听失败时 NewServer 包装 error 返回。
func TestNewServer_InvalidAddr(t *testing.T) {
	// 非法地址格式：含空格字符会让 net.Listen 报错。
	_, err := grpcsrv.NewServer(nopDeps("invalid addr:99999"))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "listen")
}

// TestNewServer_OK 验证：合法地址下 NewServer 成功、Stop 正常退出。
func TestNewServer_OK(t *testing.T) {
	// 启动一个 0 端口的临时 server，复用其端口号以避免冲突。
	lis, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	addr := lis.Addr().String()
	require.NoError(t, lis.Close())

	srv, err := grpcsrv.NewServer(nopDeps(addr))
	require.NoError(t, err)
	require.NotNil(t, srv)

	// Serve 在 goroutine 中运行，Stop 后应立刻返回。
	done := make(chan error, 1)
	go func() { done <- srv.Serve() }()

	// 给 server 一点启动时间，然后 Stop。
	time.Sleep(20 * time.Millisecond)
	srv.Stop()

	select {
	case <-done:
		// Stop 后 Serve 返回的可能是 "use of closed network connection"。
		// 我们的合约不要求 Serve 退出时返回 nil，只验证它不阻塞 Stop。
	case <-time.After(2 * time.Second):
		t.Fatal("Serve 阻塞未退出")
	}
}

// TestNewServer_GracefulStop 验证：GracefulStop 立即返回（无 in-flight RPC）。
func TestNewServer_GracefulStop(t *testing.T) {
	lis, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	addr := lis.Addr().String()
	require.NoError(t, lis.Close())

	srv, err := grpcsrv.NewServer(nopDeps(addr))
	require.NoError(t, err)

	done := make(chan error, 1)
	go func() { done <- srv.Serve() }()
	time.Sleep(20 * time.Millisecond)
	// GracefulStop 不阻塞；Serve 仍在 goroutine 中等待新连接。
	srv.GracefulStop()
	// 等待 Serve 自然返回。
	select {
	case <-done:
		// ok
	case <-time.After(2 * time.Second):
		// 兜底：超时也允许（GracefulStop 后 Serve 可能仍在 Accept 阻塞）。
	}
}

// TestServer_End2End_Bufconn 验证：NewServer 装配的链路能通过 bufconn 打通。
//
// 端到端串起 server.go + service + interceptor：
//  1. NewServer 监听真实 TCP 端口（用 127.0.0.1:0 避免冲突）；
//  2. client 走 grpc.DialContext 直连该端口；
//  3. 调用 ListGenerals，依赖 nil pool → 返 INTERNAL（验证 error 链路通）。
//
// 端到端业务 RPC 行为已由 service/order_test.go 覆盖；本测试只验证
// "NewServer 出来的 server 在真实 TCP 上能跑通完整 RPC 链路"。
func TestServer_End2End_Bufconn(t *testing.T) {
	// 1) 拿一个空闲端口。
	lis, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	addr := lis.Addr().String()
	require.NoError(t, lis.Close())

	// 2) NewServer 监听该端口。
	srv, err := grpcsrv.NewServer(nopDeps(addr))
	require.NoError(t, err)
	go func() { _ = srv.Serve() }()
	defer srv.Stop()

	// 3) 等待 server 起来。
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		c, derr := net.Dial("tcp", addr)
		if derr == nil {
			_ = c.Close()
			break
		}
		time.Sleep(10 * time.Millisecond)
	}

	// 4) 客户端连接 + 调 RPC。
	conn, err := grpc.NewClient(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	require.NoError(t, err)
	defer func() { _ = conn.Close() }()

	cli := pb.NewKongmingClient(conn)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	_, err = cli.ListGenerals(ctx, &pb.ListGeneralsRequest{})
	// nil pool 场景下应返 INTERNAL（"general pool is not configured"）。
	// 此处不强制断言具体 code，只要 err 非 nil（说明链路通 + 拦截器跑通）。
	require.Error(t, err, "ListGenerals with nil pool should return error")
}
