// Package errors Code → HTTP / gRPC 状态码分类器。
//
// 设计原则：
//   - 单一职责：分类器只负责映射，不做协议编解码；
//   - 集中策略：所有"Code 变状态码"的逻辑收敛到本文件，避免散落各适配器；
//   - 兜底降级：未注册 Code 一律返回 500 / Internal，绝不返回 0 或 panic。
package errors

import (
	"net/http"

	"google.golang.org/grpc/codes"
)

// HTTPStatus 将业务 Code 映射到 HTTP 状态码。
//
// 映射表（按 spec §4.1）：
//
//	INVALID_ARGUMENT → 400 BadRequest
//	UNAUTHORIZED     → 401 Unauthorized
//	NOT_FOUND        → 404 NotFound
//	CONFLICT         → 409 Conflict
//	INVALID_STATE    → 409 Conflict
//	TIMEOUT          → 504 GatewayTimeout
//	UNAVAILABLE      → 503 ServiceUnavailable
//	CIRCUIT_OPEN     → 503 ServiceUnavailable
//	INTERNAL / 其他   → 500 InternalServerError
//
// 未注册的 Code 一律降级为 500，绝不返回 0（避免框架误判为成功）。
func (c Code) HTTPStatus() int {
	switch c {
	case INVALID_ARGUMENT:
		return http.StatusBadRequest
	case UNAUTHORIZED:
		return http.StatusUnauthorized
	case NOT_FOUND:
		return http.StatusNotFound
	case CONFLICT, INVALID_STATE:
		return http.StatusConflict
	case TIMEOUT:
		return http.StatusGatewayTimeout
	case UNAVAILABLE, CIRCUIT_OPEN:
		return http.StatusServiceUnavailable
	default:
		// 兜底：INTERNAL / PLUGIN_LOAD_FAIL / STRATEGY_FAILED / PERSIST_FAILED / OK / 未知值。
		return http.StatusInternalServerError
	}
}

// GRPCCode 将业务 Code 映射到 gRPC 标准 codes.Code。
//
// 映射表（与 HTTP 保持一致的语义，遵循 gRPC 官方推荐）：
//
//	INVALID_ARGUMENT → InvalidArgument
//	UNAUTHORIZED     → Unauthenticated
//	NOT_FOUND        → NotFound
//	CONFLICT         → FailedPrecondition
//	INVALID_STATE    → FailedPrecondition
//	TIMEOUT          → DeadlineExceeded
//	UNAVAILABLE      → Unavailable
//	CIRCUIT_OPEN     → Unavailable
//	INTERNAL / 其他   → Internal
//
// 未注册的 Code 一律降级为 codes.Internal，避免调用方收到 Unknown 而无法分类。
func (c Code) GRPCCode() codes.Code {
	switch c {
	case INVALID_ARGUMENT:
		return codes.InvalidArgument
	case UNAUTHORIZED:
		return codes.Unauthenticated
	case NOT_FOUND:
		return codes.NotFound
	case CONFLICT, INVALID_STATE:
		return codes.FailedPrecondition
	case TIMEOUT:
		return codes.DeadlineExceeded
	case UNAVAILABLE, CIRCUIT_OPEN:
		return codes.Unavailable
	default:
		// 兜底：INTERNAL / PLUGIN_LOAD_FAIL / STRATEGY_FAILED / PERSIST_FAILED / OK / 未知值。
		return codes.Internal
	}
}
