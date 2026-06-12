package interceptor

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	"github.com/zhuge/kongming/pkg/infra/observability"
)

// TestChain_NoInterceptors 验证：传入空链时返回 noop 拦截器（不 panic）。
func TestChain_NoInterceptors(t *testing.T) {
	// 入参 nil。
	c1 := Chain()
	require.NotNil(t, c1)
	info := &grpc.UnaryServerInfo{FullMethod: "/test/Method"}
	called := false
	resp, err := c1(context.Background(), nil, info, func(_ context.Context, _ any) (any, error) {
		called = true
		return "ok", nil
	})
	require.NoError(t, err)
	assert.Equal(t, "ok", resp)
	assert.True(t, called, "handler 必须被调用")

	// 入参仅含 nil 拦截器。
	c2 := Chain(nil, nil)
	require.NotNil(t, c2)
	called = false
	_, err = c2(context.Background(), nil, info, func(_ context.Context, _ any) (any, error) {
		called = true
		return nil, nil
	})
	require.NoError(t, err)
	assert.True(t, called, "全部为 nil 的链仍应调用 handler")
}

// TestChain_SingleInterceptor 验证：单个拦截器时直接透传，无多余包装。
func TestChain_SingleInterceptor(t *testing.T) {
	seen := false
	one := func(_ context.Context, _ any, _ *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		seen = true
		return handler(context.Background(), "x")
	}
	c := Chain(one)
	resp, err := c(context.Background(), "in", &grpc.UnaryServerInfo{}, func(_ context.Context, r any) (any, error) {
		assert.Equal(t, "x", r)
		return "out", nil
	})
	require.NoError(t, err)
	assert.Equal(t, "out", resp)
	assert.True(t, seen, "单拦截器必须被执行")
}

// TestChain_OrderOutermostFirst 验证：拦截器按数组顺序：第一个最外层、最后一个最内层。
func TestChain_OrderOutermostFirst(t *testing.T) {
	// 记录调用顺序。
	var order []string
	mk := func(name string) grpc.UnaryServerInterceptor {
		return func(ctx context.Context, _ any, _ *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
			order = append(order, name+":in")
			resp, err := handler(ctx, nil)
			order = append(order, name+":out")
			return resp, err
		}
	}
	c := Chain(mk("A"), mk("B"), mk("C"))
	resp, err := c(context.Background(), nil, &grpc.UnaryServerInfo{FullMethod: "/t/M"}, func(_ context.Context, _ any) (any, error) {
		order = append(order, "handler")
		return "ok", nil
	})
	require.NoError(t, err)
	assert.Equal(t, "ok", resp)
	// 期望：A→B→C→handler→C→B→A。
	assert.Equal(t, []string{"A:in", "B:in", "C:in", "handler", "C:out", "B:out", "A:out"}, order)
}

// TestChain_PropagatesError 验证：拦截器链会把 handler 的 error 一路冒泡。
func TestChain_PropagatesError(t *testing.T) {
	want := status.Error(codes.NotFound, "missing")
	c := Chain(func(_ context.Context, _ any, _ *grpc.UnaryServerInfo, h grpc.UnaryHandler) (any, error) {
		return h(context.Background(), nil)
	})
	_, err := c(context.Background(), nil, &grpc.UnaryServerInfo{}, func(_ context.Context, _ any) (any, error) {
		return nil, want
	})
	assert.Equal(t, want, err)
}

// TestTraceID_InjectFromMetadata 验证：metadata 存在 x-trace-id 时直接复用。
func TestTraceID_InjectFromMetadata(t *testing.T) {
	md := metadata.Pairs(TraceIDKey, "trace-abc")
	ctx := metadata.NewIncomingContext(context.Background(), md)
	var got string
	inter := TraceID()
	resp, err := inter(ctx, nil, &grpc.UnaryServerInfo{FullMethod: "/t/M"}, func(ctx context.Context, _ any) (any, error) {
		got = observability.FromTraceIDContext(ctx)
		return "ok", nil
	})
	require.NoError(t, err)
	assert.Equal(t, "ok", resp)
	assert.Equal(t, "trace-abc", got, "应从 metadata 透传 traceId")
}

// TestTraceID_GenerateWhenMissing 验证：metadata 缺失 x-trace-id 时自动生成。
func TestTraceID_GenerateWhenMissing(t *testing.T) {
	ctx := context.Background()
	var got string
	inter := TraceID()
	_, err := inter(ctx, nil, &grpc.UnaryServerInfo{FullMethod: "/t/M"}, func(ctx context.Context, _ any) (any, error) {
		got = observability.FromTraceIDContext(ctx)
		return nil, nil
	})
	require.NoError(t, err)
	assert.NotEmpty(t, got, "metadata 缺失时必须自动生成 traceId")
	assert.True(t, len(got) >= 16, "应至少为 UUID 长度")
}

// TestTraceID_BlankInMetadata 验证：metadata 中是空字符串时也走"生成"分支。
func TestTraceID_BlankInMetadata(t *testing.T) {
	md := metadata.Pairs(TraceIDKey, "")
	ctx := metadata.NewIncomingContext(context.Background(), md)
	var got string
	_, _ = TraceID()(ctx, nil, &grpc.UnaryServerInfo{FullMethod: "/t/M"}, func(ctx context.Context, _ any) (any, error) {
		got = observability.FromTraceIDContext(ctx)
		return nil, nil
	})
	assert.NotEqual(t, "", got, "metadata 中是空字符串时必须重新生成")
}

// TestRecovery_Panic 验证：panic 被捕获并返回 codes.Internal。
func TestRecovery_Panic(t *testing.T) {
	inter := Recovery(zap.NewNop())
	resp, err := inter(context.Background(), nil, &grpc.UnaryServerInfo{FullMethod: "/t/M"}, func(_ context.Context, _ any) (any, error) {
		panic("boom")
	})
	require.Error(t, err)
	assert.Nil(t, resp)
	st, ok := status.FromError(err)
	require.True(t, ok, "必须是 status.Error")
	assert.Equal(t, codes.Internal, st.Code())
}

// TestRecovery_NoPanicPasses 验证：handler 正常返回时拦截器透传结果。
func TestRecovery_NoPanicPasses(t *testing.T) {
	inter := Recovery(zap.NewNop())
	resp, err := inter(context.Background(), nil, &grpc.UnaryServerInfo{FullMethod: "/t/M"}, func(_ context.Context, _ any) (any, error) {
		return "ok", nil
	})
	require.NoError(t, err)
	assert.Equal(t, "ok", resp)
}

// TestRecovery_NilLoggerSafe 验证：传入 nil logger 也不 panic（zap.NewNop 兜底）。
func TestRecovery_NilLoggerSafe(t *testing.T) {
	assert.NotPanics(t, func() {
		inter := Recovery(nil)
		_, _ = inter(context.Background(), nil, &grpc.UnaryServerInfo{}, func(_ context.Context, _ any) (any, error) {
			panic("x")
		})
	})
}

// TestLogging_NilLoggerSafe 验证：Logging 传入 nil logger 也不 panic。
func TestLogging_NilLoggerSafe(t *testing.T) {
	assert.NotPanics(t, func() {
		inter := Logging(nil)
		_, err := inter(context.Background(), nil, &grpc.UnaryServerInfo{FullMethod: "/t/M"}, func(_ context.Context, _ any) (any, error) {
			return "ok", nil
		})
		assert.NoError(t, err)
	})
}

// TestLogging_PropagatesHandlerError 验证：handler 报错时仍把 error 冒泡回去。
func TestLogging_PropagatesHandlerError(t *testing.T) {
	want := errors.New("downstream fail")
	inter := Logging(zap.NewNop())
	resp, err := inter(context.Background(), nil, &grpc.UnaryServerInfo{FullMethod: "/t/M"}, func(_ context.Context, _ any) (any, error) {
		return nil, want
	})
	assert.Equal(t, want, err)
	assert.Nil(t, resp)
}
