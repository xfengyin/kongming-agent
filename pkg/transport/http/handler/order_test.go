package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"

	"github.com/zhuge/kongming/pkg/domain/errors"
	"github.com/zhuge/kongming/pkg/domain/model"
)

// mockCommander 实现 port.Commander，便于 handler 单测。
//
// 通过函数字段注入行为；未注入时返回零值，便于「只关心一个方法」的测试。
type mockCommander struct {
	dispatchFn     func(ctx context.Context, o *model.Order) (*model.BattleReport, error)
	planStrategyFn func(ctx context.Context, o *model.Order) (*model.Strategy, error)
	reviewFn       func(ctx context.Context, r *model.BattleReport) error
	getOrderFn     func(ctx context.Context, id model.OrderID) (*model.Order, error)
	listOrdersFn   func(ctx context.Context, s model.State) ([]*model.Order, error)
	dispatchCalls  int
	lastOrder      *model.Order
}

func (m *mockCommander) Dispatch(ctx context.Context, o *model.Order) (*model.BattleReport, error) {
	m.dispatchCalls++
	m.lastOrder = o
	if m.dispatchFn != nil {
		return m.dispatchFn(ctx, o)
	}
	return &model.BattleReport{OrderID: o.ID, Success: true}, nil
}

func (m *mockCommander) PlanStrategy(ctx context.Context, o *model.Order) (*model.Strategy, error) {
	if m.planStrategyFn != nil {
		return m.planStrategyFn(ctx, o)
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
	return nil, nil
}

func (m *mockCommander) ListOrders(ctx context.Context, s model.State) ([]*model.Order, error) {
	if m.listOrdersFn != nil {
		return m.listOrdersFn(ctx, s)
	}
	return nil, nil
}

// noopObserver 满足 port.Observer 全部方法，调用全部 no-op，
// 避免 handler 内 observer==nil 时 nil 指针解引用。
type noopObserver struct{}

func (n *noopObserver) IncCounter(string, map[string]string)                {}
func (n *noopObserver) ObserveHistogram(string, float64, map[string]string) {}
func (n *noopObserver) SetGauge(string, float64, map[string]string)         {}
func (n *noopObserver) StartSpan(ctx context.Context, _ string, _ ...attribute.KeyValue) (context.Context, trace.Span) {
	return ctx, trace.SpanFromContext(ctx)
}
func (n *noopObserver) RecordError(trace.Span, error)                              {}
func (n *noopObserver) RecordEvent(context.Context, string, ...attribute.KeyValue) {}
func (n *noopObserver) Shutdown(context.Context) error                             { return nil }

// newTestHandler 构造带 mock commander 的 Handler；observer 用 noop，logger 用 Nop。
func newTestHandler(c *mockCommander) *Handler {
	return New(c, nil, nil, nil, nil, &noopObserver{}, zap.NewNop())
}

// newTestEngine 构造只挂载特定 handler 的 gin 路由，避免中间件干扰。
func newTestEngine(method, path string, h gin.HandlerFunc) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Handle(method, path, h)
	return r
}

// =============================================================================
// CreateOrder 测试
// =============================================================================

// TestCreateOrder_Success 验证正常入参时返回 200 + BattleReport。
func TestCreateOrder_Success(t *testing.T) {
	mock := &mockCommander{}
	h := newTestHandler(mock)
	r := newTestEngine("POST", "/api/v1/orders", h.CreateOrder)

	body := `{"name":"test-order","description":"d","priority":2,"objectives":["o1","o2"]}`
	req := httptest.NewRequest("POST", "/api/v1/orders", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code, "正常入参应返回 200")
	require.Equal(t, 1, mock.dispatchCalls, "Dispatch 应当被调用 1 次")
	require.NotNil(t, mock.lastOrder, "Dispatch 入参 order 不应为 nil")
	assert.Equal(t, "test-order", mock.lastOrder.Name)
	assert.Equal(t, model.Priority(2), mock.lastOrder.Priority)
	assert.Equal(t, model.StatePending, mock.lastOrder.State)
	assert.Equal(t, []string{"o1", "o2"}, mock.lastOrder.Strategy.Objectives)
	assert.NotEmpty(t, mock.lastOrder.ID, "Handler 应自动生成 OrderID")

	// 响应体包含 order_id 与 report。
	var resp map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Contains(t, resp, "order_id")
	assert.Contains(t, resp, "report")
}

// TestCreateOrder_BadRequest 验证非法 JSON 入参时返回 400 + INVALID_ARGUMENT。
func TestCreateOrder_BadRequest(t *testing.T) {
	h := newTestHandler(&mockCommander{})
	r := newTestEngine("POST", "/api/v1/orders", h.CreateOrder)

	// 非法 JSON 缺少右括号。
	req := httptest.NewRequest("POST", "/api/v1/orders", bytes.NewBufferString(`{"name":"x"`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusBadRequest, w.Code, "非法 JSON 应返回 400")

	var resp dtoErrorResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, "INVALID_ARGUMENT", string(resp.Code))
}

// TestCreateOrder_MissingName 验证必填字段缺失时返回 400。
func TestCreateOrder_MissingName(t *testing.T) {
	h := newTestHandler(&mockCommander{})
	r := newTestEngine("POST", "/api/v1/orders", h.CreateOrder)

	// name 为空（binding:"required"）。
	req := httptest.NewRequest("POST", "/api/v1/orders", bytes.NewBufferString(`{"description":"d"}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusBadRequest, w.Code)
	var resp dtoErrorResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, "INVALID_ARGUMENT", string(resp.Code))
}

// TestCreateOrder_InvalidState 验证 Commander 返回 INVALID_STATE 错误时
// handler 写入 409 + INVALID_STATE 响应。
func TestCreateOrder_InvalidState(t *testing.T) {
	mock := &mockCommander{
		dispatchFn: func(_ context.Context, o *model.Order) (*model.BattleReport, error) {
			return nil, errors.New(errors.INVALID_STATE, "invalid state transition")
		},
	}
	h := newTestHandler(mock)
	r := newTestEngine("POST", "/api/v1/orders", h.CreateOrder)

	body := `{"name":"o1","priority":1}`
	req := httptest.NewRequest("POST", "/api/v1/orders", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusConflict, w.Code, "INVALID_STATE 应映射为 409")
	var resp dtoErrorResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, "INVALID_STATE", string(resp.Code))
}

// =============================================================================
// GetOrder 测试
// =============================================================================

// TestGetOrder_NotFound 验证订单不存在时返回 404 + NOT_FOUND。
func TestGetOrder_NotFound(t *testing.T) {
	mock := &mockCommander{
		getOrderFn: func(_ context.Context, _ model.OrderID) (*model.Order, error) {
			return nil, errors.New(errors.NOT_FOUND, "order o1 not found")
		},
	}
	h := newTestHandler(mock)
	r := newTestEngine("GET", "/api/v1/orders/:id", h.GetOrder)

	req := httptest.NewRequest("GET", "/api/v1/orders/o1", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusNotFound, w.Code)
	var resp dtoErrorResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, "NOT_FOUND", string(resp.Code))
	assert.Contains(t, resp.Message, "o1")
}

// TestGetOrder_OK 验证订单存在时返回 200。
func TestGetOrder_OK(t *testing.T) {
	now := time.Now().UTC()
	mock := &mockCommander{
		getOrderFn: func(_ context.Context, id model.OrderID) (*model.Order, error) {
			return &model.Order{
				ID:        id,
				Name:      "n1",
				State:     model.StateCompleted,
				Priority:  model.PriorityNormal,
				CreatedAt: now,
				UpdatedAt: now,
			}, nil
		},
	}
	h := newTestHandler(mock)
	r := newTestEngine("GET", "/api/v1/orders/:id", h.GetOrder)

	req := httptest.NewRequest("GET", "/api/v1/orders/o-1", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
}

// =============================================================================
// ListOrders 测试
// =============================================================================

// TestListOrders_OK 验证 ListOrders 返回 200。
func TestListOrders_OK(t *testing.T) {
	mock := &mockCommander{
		listOrdersFn: func(_ context.Context, _ model.State) ([]*model.Order, error) {
			return []*model.Order{
				{ID: "o1", Name: "a", State: model.StatePending},
				{ID: "o2", Name: "b", State: model.StateCompleted},
			}, nil
		},
	}
	h := newTestHandler(mock)
	r := newTestEngine("GET", "/api/v1/orders", h.ListOrders)

	req := httptest.NewRequest("GET", "/api/v1/orders", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	var resp map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Contains(t, resp, "orders")
}

// TestListOrders_InvalidStateQuery 验证 query 参数非法状态被 handler 拒绝。
func TestListOrders_InvalidStateQuery(t *testing.T) {
	h := newTestHandler(&mockCommander{})
	r := newTestEngine("GET", "/api/v1/orders", h.ListOrders)

	// state=notanumber 解析失败。
	req := httptest.NewRequest("GET", "/api/v1/orders?state=notanumber", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusBadRequest, w.Code)
	var resp dtoErrorResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, "INVALID_ARGUMENT", string(resp.Code))
}

// =============================================================================
// Health 测试
// =============================================================================

// TestHealthz_OK 验证 /healthz 总返回 200。
func TestHealthz_OK(t *testing.T) {
	h := newTestHandler(&mockCommander{})
	r := newTestEngine("GET", "/healthz", h.Healthz)

	req := httptest.NewRequest("GET", "/healthz", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	var resp map[string]string
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, "ok", resp["status"])
}

// TestReadyz_OK 验证 /readyz 默认返回 200。
func TestReadyz_OK(t *testing.T) {
	h := newTestHandler(&mockCommander{})
	r := newTestEngine("GET", "/readyz", h.Readyz)

	req := httptest.NewRequest("GET", "/readyz", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
}

// =============================================================================
// 公共类型：与 dto.ErrorResponse 保持一致，避免引入 dto 循环依赖
// =============================================================================

type dtoErrorResponse struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}
