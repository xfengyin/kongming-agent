// Package errors 领域层类型化错误体系。
//
// 设计目标：
//  1. 业务侧只关心"错误码 + 上下文"（Fields + TraceID），不再关心 string 拼接；
//  2. 适配层（HTTP / gRPC）通过 classify.go 集中映射 Code → 状态码 / gRPC codes；
//  3. 配合 errors.Is / errors.As 实现跨层语义匹配（Code 即类别，cause 即根因）。
//
// 注意：本包名与 stdlib "errors" 重名，业务代码严禁 import stdlib errors；
// 唯一可能用到 stdlib errors.Is / errors.As 的场景是测试文件（alias 为 stderrors）。
package errors

// Code 是错误码的类型化字符串别名。
//
// 使用 string 而非 int 的原因：
//   - 日志/链路追踪中可读性更高（无需维护"错误码字典"）；
//   - 前后兼容性强（增删 Code 不影响线上告警/统计）；
//   - 与 gRPC status / OpenTelemetry semantic conventions 对齐。
type Code string

// 业务错误码全集。命名遵循 snake_case + 大写常量风格，跨包可读。
//
// 注意：所有 Code 字符串值是契约的一部分，**禁止随意重命名**，否则会破坏
// 已落盘的日志、告警、Prometheus 指标的语义。
const (
	// OK 操作成功（理论上不在错误流中出现，保留以便"零值即成功"语义）。
	OK Code = "OK"

	// INVALID_ARGUMENT 客户端参数非法（HTTP 400 / gRPC InvalidArgument）。
	INVALID_ARGUMENT Code = "INVALID_ARGUMENT"

	// UNAUTHORIZED 未认证或凭证失效（HTTP 401 / gRPC Unauthenticated）。
	UNAUTHORIZED Code = "UNAUTHORIZED"

	// NOT_FOUND 资源不存在（HTTP 404 / gRPC NotFound）。
	NOT_FOUND Code = "NOT_FOUND"

	// CONFLICT 资源状态冲突（如唯一索引冲突、版本号不匹配）（HTTP 409 / gRPC FailedPrecondition）。
	CONFLICT Code = "CONFLICT"

	// INVALID_STATE 业务状态机非法（HTTP 409 / gRPC FailedPrecondition）。
	INVALID_STATE Code = "INVALID_STATE"

	// TIMEOUT 操作超时（HTTP 504 / gRPC DeadlineExceeded）。
	TIMEOUT Code = "TIMEOUT"

	// UNAVAILABLE 上游/依赖不可用（HTTP 503 / gRPC Unavailable）。
	UNAVAILABLE Code = "UNAVAILABLE"

	// CIRCUIT_OPEN 熔断器打开（HTTP 503 / gRPC Unavailable）。
	CIRCUIT_OPEN Code = "CIRCUIT_OPEN"

	// INTERNAL 内部未预期错误（HTTP 500 / gRPC Internal）。
	INTERNAL Code = "INTERNAL"

	// PLUGIN_LOAD_FAIL 插件加载失败（HTTP 500 / gRPC Internal）。
	PLUGIN_LOAD_FAIL Code = "PLUGIN_LOAD_FAIL"

	// STRATEGY_FAILED 策略执行失败（如 AI 决策引擎返回无效结果）（HTTP 500 / gRPC Internal）。
	STRATEGY_FAILED Code = "STRATEGY_FAILED"

	// PERSIST_FAILED 持久化失败（HTTP 500 / gRPC Internal）。
	PERSIST_FAILED Code = "PERSIST_FAILED"
)

// 业务侧高频使用的核心 Code 集合（编译期静态检查）。
var _ = []Code{
	OK,
	INVALID_ARGUMENT,
	UNAUTHORIZED,
	NOT_FOUND,
	CONFLICT,
	INVALID_STATE,
	TIMEOUT,
	UNAVAILABLE,
	CIRCUIT_OPEN,
	INTERNAL,
	PLUGIN_LOAD_FAIL,
	STRATEGY_FAILED,
	PERSIST_FAILED,
}
