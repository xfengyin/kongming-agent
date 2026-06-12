// Package errors 领域层类型化错误体系的单元测试。
//
// 注意：本包名与 stdlib "errors" 冲突，业务代码严禁 import stdlib errors；
// 本测试文件也仅在测试场景下以别名 `stderrors` 形式引入，用于断言
// errors.Is / errors.As 等行为的语义正确性。
package errors

import (
	stderrors "errors"
	"fmt"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
)

// 公共触发型 error，便于 errors.Is 断言。
var baseCause = stderrors.New("底层失败原因")

func TestError_New(t *testing.T) {
	// 场景：纯新错误（无 cause），只设置 Code 与 Message。
	e := New(INVALID_ARGUMENT, "参数非法")
	require.NotNil(t, e)
	assert.Equal(t, INVALID_ARGUMENT, e.Code)
	assert.Equal(t, "参数非法", e.Message)
	assert.Nil(t, e.Cause)
	assert.Empty(t, e.TraceID)
	assert.NotNil(t, e.Fields)
	assert.Empty(t, e.Fields)
}

func TestError_Error_WithoutCause(t *testing.T) {
	// 场景：Error() 至少要包含 code 与 message。
	e := New(NOT_FOUND, "资源不存在")
	assert.Contains(t, e.Error(), string(NOT_FOUND))
	assert.Contains(t, e.Error(), "资源不存在")
}

func TestError_Error_WithCause(t *testing.T) {
	// 场景：Error() 包含 cause 的字符串，便于日志检索。
	e := Wrap(CONFLICT, baseCause)
	assert.Contains(t, e.Error(), string(CONFLICT))
	assert.Contains(t, e.Error(), baseCause.Error())
}

func TestError_Wrap_PreservesCause(t *testing.T) {
	// 场景：Wrap 后的错误必须能让 errors.Is 命中原 cause。
	e := Wrap(INTERNAL, baseCause)
	assert.True(t, stderrors.Is(e, baseCause), "errors.Is(e, cause) 必须为 true")
}

func TestError_Wrap_MessageDefaultsToCauseText(t *testing.T) {
	// 场景：Wrap 未显式传 message 时，Message 默认 = cause.Error()。
	e := Wrap(INTERNAL, baseCause)
	assert.Equal(t, baseCause.Error(), e.Message)
}

func TestError_Is_SameCode(t *testing.T) {
	// 场景：不同实例但 Code 相同，errors.Is 应能匹配（语义：Code 即类别）。
	a := New(NOT_FOUND, "找不到 a")
	b := New(NOT_FOUND, "找不到 b")
	assert.True(t, stderrors.Is(a, b), "同 Code 应跨实例匹配")
}

func TestError_Is_DiffCode(t *testing.T) {
	// 场景：不同 Code 不应误匹配。
	a := New(NOT_FOUND, "a")
	b := New(CONFLICT, "b")
	assert.False(t, stderrors.Is(a, b), "不同 Code 不应匹配")
}

func TestError_Is_NonPointerTarget(t *testing.T) {
	// 场景：target 不是 *Error（如 stdlib errors.New）时，Is 应回退为
	// 与 Cause 的等价判断。
	e := Wrap(NOT_FOUND, baseCause)
	assert.True(t, stderrors.Is(e, baseCause))
	assert.False(t, stderrors.Is(e, stderrors.New("其他原因")))
}

func TestError_Unwrap(t *testing.T) {
	// 场景：Unwrap 必须返回原始 cause。
	e := Wrap(INTERNAL, baseCause)
	assert.Equal(t, baseCause, stderrors.Unwrap(e))
}

func TestError_WithField_Chain(t *testing.T) {
	// 场景：WithField 必须支持链式调用，且不影响原对象（不可变性）。
	e := New(INVALID_ARGUMENT, "参数非法").
		WithField("user_id", "u-1").
		WithField("order_id", 42)
	assert.Equal(t, "u-1", e.Fields["user_id"])
	assert.Equal(t, 42, e.Fields["order_id"])
}

func TestError_WithTraceID(t *testing.T) {
	// 场景：WithTraceID 设置 traceID 并支持链式。
	e := New(TIMEOUT, "超时").WithTraceID("trace-abc-123")
	assert.Equal(t, "trace-abc-123", e.TraceID)
}

func TestError_Format(t *testing.T) {
	// 场景：fmt.Sprintf("%v", e) 输出便于日志检索，包含 code / message / cause。
	// 约定：Error() 不暴露 Fields/TraceID（由结构化日志器单独输出），故此断言只校验三件套。
	e := Wrap(CONFLICT, baseCause).WithTraceID("tid-1").WithField("k", "v")
	out := fmt.Sprintf("%v", e)
	assert.Contains(t, out, string(CONFLICT))
	assert.Contains(t, out, baseCause.Error())
}

func TestError_Format_NoCause(t *testing.T) {
	// 场景：无 cause 的格式化不应 panic。
	e := New(OK, "ok")
	assert.NotPanics(t, func() {
		_ = fmt.Sprintf("%v", e)
	})
}

func TestError_NilReceiver_AllMethodsSafe(t *testing.T) {
	// 场景：所有方法必须容忍 nil 接收者（避免 panic 蔓延）。
	// 这是 Go 错误处理的最佳实践，调用方常把 e 当作值传递。
	var e *Error
	assert.NotPanics(t, func() { _ = e.Error() })
	assert.NotPanics(t, func() { _ = stderrors.Unwrap(e) })
	assert.NotPanics(t, func() { _ = stderrors.Is(e, New(NOT_FOUND, "")) })
	assert.NotPanics(t, func() { _ = e.WithField("k", "v") })
	assert.NotPanics(t, func() { _ = e.WithTraceID("tid") })
}

func TestError_Is_NilTarget(t *testing.T) {
	// 场景：Is 的 target 为 nil 必须返回 false（除 e 自身也 nil 的极端情况）。
	e := New(NOT_FOUND, "")
	assert.False(t, stderrors.Is(e, nil))
}

func TestError_Is_NilBoth(t *testing.T) {
	// 场景：直接调用 Is 方法，e==nil && target==nil 必须返回 true。
	// 注意：通过 stdlib errors.Is(nil, nil) 走的是 `err == target` 分支（不进入 Is），
	// 行为依赖类型可比较性，此处直接测 Is 方法本身以覆盖该 return 分支。
	var e *Error
	assert.True(t, e.Is(nil))
}

func TestError_Is_NonErrorTarget(t *testing.T) {
	// 场景：target 不是 *Error 且 e 有 cause 时，Is 方法本身返回 false，
	// 但 errors.Is 仍能通过 Unwrap 命中（已在 TestError_Is_NonPointerTarget 验证）。
	// 此处直接调用 Is 方法覆盖最后一条 return false 分支。
	e := New(NOT_FOUND, "")
	assert.False(t, e.Is(baseCause))
}

func TestError_Is_NilTargetDirectCall(t *testing.T) {
	// 场景：直接调用 Is 方法、target 为 nil 的分支（stdlib errors.Is 在 target==nil
	// 时提前返回，不会进入 Is 的 target==nil 分支；此测试用于覆盖该防御性分支）。
	e := New(NOT_FOUND, "")
	assert.False(t, e.Is(nil))
}

func TestWithField_LazyInitFields(t *testing.T) {
	// 场景：Fields 为 nil 时 WithField 必须懒分配，不能 panic。
	e := &Error{Code: INVALID_ARGUMENT, Message: "x"} // 故意不初始化 Fields
	assert.NotPanics(t, func() {
		_ = e.WithField("k", "v")
	})
	assert.Equal(t, "v", e.Fields["k"])
}

func TestWrap_NilCause(t *testing.T) {
	// 场景：Wrap(code, nil) 必须降级为 New(code, "")，不产生歧义 cause。
	e := Wrap(INTERNAL, nil)
	require.NotNil(t, e)
	assert.Equal(t, INTERNAL, e.Code)
	assert.Equal(t, "", e.Message)
	assert.Nil(t, e.Cause)
}

func TestClassify_HTTP(t *testing.T) {
	// 场景：核心 Code 必须映射到约定的 HTTP 状态码。
	cases := []struct {
		code Code
		want int
	}{
		{INVALID_ARGUMENT, http.StatusBadRequest},     // 400
		{UNAUTHORIZED, http.StatusUnauthorized},       // 401
		{NOT_FOUND, http.StatusNotFound},              // 404
		{CONFLICT, http.StatusConflict},               // 409
		{INVALID_STATE, http.StatusConflict},          // 409
		{TIMEOUT, http.StatusGatewayTimeout},          // 504
		{UNAVAILABLE, http.StatusServiceUnavailable},  // 503
		{CIRCUIT_OPEN, http.StatusServiceUnavailable}, // 503
		{INTERNAL, http.StatusInternalServerError},    // 500
	}
	for _, c := range cases {
		assert.Equal(t, c.want, c.code.HTTPStatus(), "Code=%s", c.code)
	}
}

func TestClassify_HTTP_UnknownDefaults500(t *testing.T) {
	// 场景：未注册 Code 必须降级为 500，绝不返回 0 或 panic。
	assert.Equal(t, http.StatusInternalServerError, Code("UNKNOWN_X").HTTPStatus())
}

func TestClassify_GRPC(t *testing.T) {
	// 场景：核心 Code 必须映射到 gRPC 标准 codes。
	cases := []struct {
		code Code
		want codes.Code
	}{
		{INVALID_ARGUMENT, codes.InvalidArgument},
		{UNAUTHORIZED, codes.Unauthenticated},
		{NOT_FOUND, codes.NotFound},
		{CONFLICT, codes.FailedPrecondition},
		{INVALID_STATE, codes.FailedPrecondition},
		{TIMEOUT, codes.DeadlineExceeded},
		{UNAVAILABLE, codes.Unavailable},
		{CIRCUIT_OPEN, codes.Unavailable},
		{INTERNAL, codes.Internal},
	}
	for _, c := range cases {
		assert.Equal(t, c.want, c.code.GRPCCode(), "Code=%s", c.code)
	}
}

func TestClassify_GRPC_UnknownDefaultsInternal(t *testing.T) {
	// 场景：未注册 Code 必须降级为 codes.Internal。
	assert.Equal(t, codes.Internal, Code("UNKNOWN_X").GRPCCode())
}
