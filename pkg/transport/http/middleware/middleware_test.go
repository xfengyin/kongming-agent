package middleware

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

// newMiddlewareEngine 构造只挂载中间件 + 一个 echo handler 的 gin 引擎。
func newMiddlewareEngine(t *testing.T, mws ...gin.HandlerFunc) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)
	r := gin.New()
	for _, m := range mws {
		r.Use(m)
	}
	r.GET("/echo", func(c *gin.Context) {
		c.String(http.StatusOK, "ok")
	})
	// 一个会 panic 的端点，供 Recovery 测试用。
	r.GET("/panic", func(_ *gin.Context) {
		panic("boom")
	})
	return r
}

// =============================================================================
// TraceID 测试
// =============================================================================

// TestTraceID_GeneratesNewWhenMissing 验证：客户端未传 X-Trace-Id 时，
// 中间件生成新的 UUID 并回写到 response header。
func TestTraceID_GeneratesNewWhenMissing(t *testing.T) {
	r := newMiddlewareEngine(t, TraceID())

	req := httptest.NewRequest("GET", "/echo", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	got := w.Header().Get(HeaderTraceID)
	assert.NotEmpty(t, got, "中间件应生成并回写 traceId")
	// UUID v4 长度 = 36（含连字符）。
	assert.Len(t, got, 36, "traceId 应为 UUID 格式")
}

// TestTraceID_PreservesClientProvided 验证：客户端传了 X-Trace-Id 时，
// 中间件透传而不替换。
func TestTraceID_PreservesClientProvided(t *testing.T) {
	r := newMiddlewareEngine(t, TraceID())

	req := httptest.NewRequest("GET", "/echo", nil)
	req.Header.Set(HeaderTraceID, "client-trace-123")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "client-trace-123", w.Header().Get(HeaderTraceID))
}

// =============================================================================
// Recovery 测试
// =============================================================================

// TestRecovery_ReturnsInternalServerError 验证 panic 被恢复且返回 500。
func TestRecovery_ReturnsInternalServerError(t *testing.T) {
	r := newMiddlewareEngine(t, Recovery(zap.NewNop()))

	req := httptest.NewRequest("GET", "/panic", nil)
	w := httptest.NewRecorder()
	require.NotPanics(t, func() {
		r.ServeHTTP(w, req)
	}, "panic 必须被中间件捕获")

	assert.Equal(t, http.StatusInternalServerError, w.Code)
	// 响应体形如 {"code":"INTERNAL","message":"internal server error: boom"}。
	assert.Contains(t, w.Body.String(), "INTERNAL")
}

// TestRecovery_PassesThroughOnNoPanic 验证无 panic 时中间件不干扰。
func TestRecovery_PassesThroughOnNoPanic(t *testing.T) {
	r := newMiddlewareEngine(t, Recovery(zap.NewNop()))

	req := httptest.NewRequest("GET", "/echo", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "ok", strings.TrimSpace(w.Body.String()))
}

// =============================================================================
// Logging 测试
// =============================================================================

// TestLogging_DoesNotBreakRequest 验证 Logging 中间件不破坏正常请求。
func TestLogging_DoesNotBreakRequest(t *testing.T) {
	r := newMiddlewareEngine(t, Logging(zap.NewNop()))

	req := httptest.NewRequest("GET", "/echo", nil)
	w := httptest.NewRecorder()
	require.NotPanics(t, func() { r.ServeHTTP(w, req) })

	assert.Equal(t, http.StatusOK, w.Code)
}

// TestLogging_NilLoggerSafe 验证 logger 为 nil 时不 panic。
func TestLogging_NilLoggerSafe(t *testing.T) {
	r := newMiddlewareEngine(t, Logging(nil))

	req := httptest.NewRequest("GET", "/echo", nil)
	w := httptest.NewRecorder()
	require.NotPanics(t, func() { r.ServeHTTP(w, req) })

	assert.Equal(t, http.StatusOK, w.Code)
}

// =============================================================================
// CORS 测试
// =============================================================================

// TestCORS_SetsHeaders 验证 CORS 头正确设置。
func TestCORS_SetsHeaders(t *testing.T) {
	r := newMiddlewareEngine(t, CORS())

	req := httptest.NewRequest("GET", "/echo", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "*", w.Header().Get("Access-Control-Allow-Origin"))
	assert.Equal(t, "X-Trace-Id", w.Header().Get("Access-Control-Expose-Headers"))
}

// TestCORS_HandlesPreflight 验证 OPTIONS 预检请求直接 204。
func TestCORS_HandlesPreflight(t *testing.T) {
	r := newMiddlewareEngine(t, CORS())

	req := httptest.NewRequest("OPTIONS", "/echo", nil)
	req.Header.Set("Origin", "http://example.com")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNoContent, w.Code)
}

// =============================================================================
// 中间件链集成测试
// =============================================================================

// TestMiddlewareChain_Order 验证完整链（Recovery → TraceID → Logging → CORS）
// 组合后能正确处理 panic 并保留 traceId。
func TestMiddlewareChain_Order(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(Recovery(zap.NewNop()))
	r.Use(TraceID())
	r.Use(Logging(zap.NewNop()))
	r.Use(CORS())
	r.GET("/echo", func(c *gin.Context) {
		c.String(http.StatusOK, "ok")
	})
	r.GET("/panic", func(_ *gin.Context) {
		panic("chain-boom")
	})

	// 正常路径应回写 traceId。
	req := httptest.NewRequest("GET", "/echo", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
	assert.NotEmpty(t, w.Header().Get(HeaderTraceID))

	// panic 路径返回 500，且 CORS 头在响应中。
	req2 := httptest.NewRequest("GET", "/panic", nil)
	w2 := httptest.NewRecorder()
	require.NotPanics(t, func() { r.ServeHTTP(w2, req2) })
	assert.Equal(t, http.StatusInternalServerError, w2.Code)
	assert.Equal(t, "*", w2.Header().Get("Access-Control-Allow-Origin"))
}
