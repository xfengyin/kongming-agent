package http

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	domerrs "github.com/zhuge/kongming/pkg/domain/errors"
	"github.com/zhuge/kongming/pkg/domain/model"
)

// mockCommander 实现 port.Commander（与 handler 包内的等价）。
type mockCommander struct {
	dispatchFn   func(ctx context.Context, o *model.Order) (*model.BattleReport, error)
	planStrategy func(ctx context.Context, o *model.Order) (*model.Strategy, error)
	reviewFn     func(ctx context.Context, r *model.BattleReport) error
	getOrderFn   func(ctx context.Context, id model.OrderID) (*model.Order, error)
	listOrdersFn func(ctx context.Context, s model.State) ([]*model.Order, error)
}

func (m *mockCommander) Dispatch(ctx context.Context, o *model.Order) (*model.BattleReport, error) {
	if m.dispatchFn != nil {
		return m.dispatchFn(ctx, o)
	}
	return &model.BattleReport{OrderID: o.ID, Success: true}, nil
}
func (m *mockCommander) PlanStrategy(ctx context.Context, o *model.Order) (*model.Strategy, error) {
	if m.planStrategy != nil {
		return m.planStrategy(ctx, o)
	}
	return &model.Strategy{BaguaMode: model.Tiangai}, nil
}
func (m *mockCommander) Review(ctx context.Context, r *model.BattleReport) error {
	if m.reviewFn != nil {
		return m.reviewFn(ctx, r)
	}
	return nil
}
func (m *mockCommander) GetOrder(ctx context.Context, id model.OrderID) (*model.Order, error) {
	if m.getOrderFn != nil {
		return m.getOrderFn(ctx, id)
	}
	return nil, domerrs.New(domerrs.NOT_FOUND, "order not found: "+string(id))
}
func (m *mockCommander) ListOrders(ctx context.Context, s model.State) ([]*model.Order, error) {
	if m.listOrdersFn != nil {
		return m.listOrdersFn(ctx, s)
	}
	return []*model.Order{}, nil
}

// newTestServer 构造 Server；observer/engine/pool/vault/dispatcher 全部 nil（按需补）。
func newTestServer(c *mockCommander) *Server {
	return NewServer(Deps{
		Commander: c,
		Logger:    zap.NewNop(),
		Addr:      ":0",
	})
}

// TestServer_Healthz 端到端：GET /healthz 返回 200。
func TestServer_Healthz(t *testing.T) {
	srv := newTestServer(&mockCommander{})
	ts := httptest.NewServer(srv.Engine())
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/healthz")
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)
}

// TestServer_Readyz 端到端：GET /readyz 返回 200。
func TestServer_Readyz(t *testing.T) {
	srv := newTestServer(&mockCommander{})
	ts := httptest.NewServer(srv.Engine())
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/readyz")
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)
}

// TestServer_CreateOrder_Success 端到端：POST /api/v1/orders 成功路径。
func TestServer_CreateOrder_Success(t *testing.T) {
	mock := &mockCommander{
		dispatchFn: func(_ context.Context, o *model.Order) (*model.BattleReport, error) {
			assert.Equal(t, "test-order", o.Name)
			return &model.BattleReport{OrderID: o.ID, Success: true}, nil
		},
	}
	srv := newTestServer(mock)
	ts := httptest.NewServer(srv.Engine())
	defer ts.Close()

	body := `{"name":"test-order","priority":2}`
	resp, err := http.Post(ts.URL+"/api/v1/orders", "application/json", bytes.NewBufferString(body))
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	// 验证 traceId 回写到 response header。
	assert.NotEmpty(t, resp.Header.Get("X-Trace-Id"))
	// 验证 CORS 头在响应中（中间件链生效）。
	assert.Equal(t, "*", resp.Header.Get("Access-Control-Allow-Origin"))
}

// TestServer_CreateOrder_NotFound 端到端：mock 返回 NOT_FOUND 时路径返回 404。
func TestServer_CreateOrder_NotFound(t *testing.T) {
	mock := &mockCommander{
		dispatchFn: func(_ context.Context, o *model.Order) (*model.BattleReport, error) {
			return nil, domerrs.New(domerrs.NOT_FOUND, "order o1 not found")
		},
	}
	srv := newTestServer(mock)
	ts := httptest.NewServer(srv.Engine())
	defer ts.Close()

	body := `{"name":"o1"}`
	resp, err := http.Post(ts.URL+"/api/v1/orders", "application/json", bytes.NewBufferString(body))
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
}

// TestServer_Metrics 端到端：GET /metrics 返回 200 + prom 文本。
func TestServer_Metrics(t *testing.T) {
	srv := newTestServer(&mockCommander{})
	ts := httptest.NewServer(srv.Engine())
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/metrics")
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)
}

// TestServer_EngineReturnsGin 验证 Engine() 暴露 gin 引擎。
func TestServer_EngineReturnsGin(t *testing.T) {
	srv := newTestServer(&mockCommander{})
	require.NotNil(t, srv.Engine(), "Engine() 必须返回非 nil gin.Engine")
}

// TestServer_AllRoutesRegistered 验证全部 11 个路由（5 group 路由 + 5
// 顶层 + /metrics）已注册到 gin 引擎。
func TestServer_AllRoutesRegistered(t *testing.T) {
	srv := newTestServer(&mockCommander{})
	routes := srv.Engine().Routes()
	paths := make(map[string]bool, len(routes))
	for _, ri := range routes {
		paths[ri.Method+" "+ri.Path] = true
	}

	// 期望注册的路由集合（method+path）。
	expected := []string{
		"GET /healthz",
		"GET /readyz",
		"GET /metrics",
		"POST /api/v1/orders",
		"GET /api/v1/orders",
		"GET /api/v1/orders/:id",
		"POST /api/v1/strategies",
		"GET /api/v1/generals",
		"GET /api/v1/generals/:id",
		"GET /api/v1/vault",
		"POST /api/v1/vault/:id/exec",
		"POST /api/v1/workflows/:id/run",
	}
	for _, e := range expected {
		assert.True(t, paths[e], "路由未注册: %s", e)
	}
}
