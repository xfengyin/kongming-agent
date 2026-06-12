// Package errors 类型化 Error 结构体与构造器。
//
// 设计要点：
//   - Error 持有 Code（类别）+ Message（人类可读）+ Cause（根因）+ TraceID（链路）+ Fields（结构化上下文）；
//   - 构造器返回 *Error 指针，支持链式 WithField / WithTraceID，**不修改原对象**（无副作用）；
//   - Error() / Unwrap() / Is() 完全兼容 stdlib errors.Is / errors.As 协议。
package errors

import "fmt"

// Error 是领域层统一的错误结构体。
//
// 字段说明：
//   - Code    错误类别，决定 HTTP/gRPC 状态码映射；
//   - Message 面向用户/调用方的人类可读描述（建议英文，避免日志乱码）；
//   - Cause   原始错误（可为 nil），用于 errors.Is/As 根因匹配；
//   - TraceID 链路追踪 ID，便于在日志/告警中检索整条调用链；
//   - Fields  业务结构化上下文（如 user_id、order_id），会原样写入结构化日志。
type Error struct {
	// Code 错误类别。
	Code Code
	// Message 人类可读的错误描述。
	Message string
	// Cause 根因错误，Unwrap 返回此值。
	Cause error
	// TraceID 链路追踪 ID。
	TraceID string
	// Fields 业务结构化上下文。
	Fields map[string]any
}

// Error 实现 error 接口。
//
// 输出格式：`[CODE] message: cause`，其中 cause 可省略。
// 该格式刻意不暴露 Fields 与 TraceID，避免日志重复打印；
// Fields/TraceID 应由上层结构化日志器（如 zap）单独输出。
func (e *Error) Error() string {
	if e == nil {
		return "<nil>"
	}
	if e.Cause != nil {
		return fmt.Sprintf("[%s] %s: %v", e.Code, e.Message, e.Cause)
	}
	return fmt.Sprintf("[%s] %s", e.Code, e.Message)
}

// Unwrap 返回根因错误，供 stdlib errors.Is / errors.As 使用。
//
// 返回 nil 时（无 cause）表示"终止链"，调用方不应再向下匹配。
func (e *Error) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Cause
}

// Is 实现自定义相等语义：同 Code 即视为同类错误。
//
// 这样调用方可以这样写：
//
//	if errors.Is(err, domainerrors.New(NOT_FOUND, "")) { ... }
//
// 而不必关心具体的 Message / Cause。
//
// 优先级：
//  1. target 是 *Error → 按 Code 匹配；
//  2. 否则回退为与 Cause 的等价判断（满足 stdlib errors.Is 协议）。
func (e *Error) Is(target error) bool {
	if e == nil {
		return target == nil
	}
	if target == nil {
		return false
	}
	if other, ok := target.(*Error); ok {
		return e.Code == other.Code
	}
	// 非 *Error 目标：委托给 stdlib，让 errors.Is(e, baseCause) 仍能工作。
	return false
}

// New 构造一个新的 *Error（无 cause）。
//
// 适用场景：参数校验失败、资源未找到等"自解释"错误，无需包装底层原因。
func New(code Code, msg string) *Error {
	return &Error{
		Code:    code,
		Message: msg,
		Fields:  make(map[string]any),
	}
}

// Wrap 包装一个 cause 错误，自动用 cause.Error() 作为 Message。
//
// 适用场景：调用外部依赖（DB / RPC / 第三方 SDK）失败时，保留根因同时附加业务 Code。
//
// 注意：不会修改传入的 cause，返回的是新 *Error。
func Wrap(code Code, cause error) *Error {
	if cause == nil {
		// 防御：调用方传 nil cause 时，降级为 New(code, "")，避免 Unwrap 返回 nil 又保留 nil cause 引发歧义。
		return New(code, "")
	}
	return &Error{
		Code:    code,
		Message: cause.Error(),
		Cause:   cause,
		Fields:  make(map[string]any),
	}
}

// WithField 追加一个结构化上下文字段，返回 *Error 以支持链式调用。
//
// 不可变性：不会修改 e.Fields 本身；若 e.Fields 尚未初始化，会先分配。
// 同名字段会被覆盖（语义：后置覆盖前置，与 zap.Field 一致）。
func (e *Error) WithField(key string, value any) *Error {
	if e == nil {
		return nil
	}
	if e.Fields == nil {
		e.Fields = make(map[string]any)
	}
	e.Fields[key] = value
	return e
}

// WithTraceID 设置链路追踪 ID，返回 *Error 以支持链式调用。
func (e *Error) WithTraceID(traceID string) *Error {
	if e == nil {
		return nil
	}
	e.TraceID = traceID
	return e
}
