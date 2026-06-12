// Package handler 提供 HTTP handler 实现，将 HTTP 请求转换为对 application
// 端口（Commander / GeneralPool / Vault / Engine）的调用。
//
// 设计原则：
//  1. 依赖倒置：handler 只依赖 domain/port.* 接口，不直接 import application/*。
//  2. 单一职责：每个 handler 方法只处理一条 HTTP 路径，函数体内不再做业务编排。
//  3. 错误统一：所有错误经 fromError 归一化后写入统一 ErrorResponse，HTTP 状态
//     码由 Code.HTTPStatus() 决定。
//  4. 可观测：每个 handler 调用 observer.IncCounter 上报 http_request_total。
package handler

import (
	stderrors "errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"github.com/zhuge/kongming/pkg/domain/errors"
	"github.com/zhuge/kongming/pkg/domain/port"
	"github.com/zhuge/kongming/pkg/transport/http/dto"
)

// Handler 聚合所有 HTTP handler 方法所需的依赖。
//
// Commander / Pool / Vault / Engine / Observer 任意一个为 nil 时构造时不报错
// （便于测试用例只 mock 关心的端口），但调用对应 handler 时会 panic —— 行为
// 与 application layer 的 service 风格一致：装配时强约束，调用时放行。
type Handler struct {
	// c 军师端口（派单/战略/订单查询）。
	c port.Commander
	// d 调度器端口（异步派发）。
	d port.Dispatcher
	// e 工作流引擎端口（RegisterWorkflow/Execute）。
	e port.Engine
	// p 将领池端口（List/Get）。
	p port.GeneralPool
	// v 锦囊库端口（List/Execute）。
	v port.Vault
	// obs 可观测性端口（IncCounter 计数）。
	obs port.Observer
	// logger zap 日志器（handler 内部需要记录关键事件时使用）。
	logger *zap.Logger
}

// New 构造 Handler；c/d/e/p/v/obs/logger 任意一个为 nil 时不报错（按需 mock）。
func New(c port.Commander, d port.Dispatcher, e port.Engine, p port.GeneralPool, v port.Vault, obs port.Observer, logger *zap.Logger) *Handler {
	return &Handler{c: c, d: d, e: e, p: p, v: v, obs: obs, logger: logger}
}

// errorResp 构造统一的 ErrorResponse。
//
// 入参：code（错误类别）、msg（人类可读消息）。
// 出参：*dto.ErrorResponse，可直接 c.JSON(..., errorResp(...))。
//
// 注意：errorResp 只构造响应体，不负责写 HTTP 状态码（由调用方根据
// fromError(err).HTTPStatus() 决定）。
func errorResp(code errors.Code, msg string) *dto.ErrorResponse {
	return &dto.ErrorResponse{Code: code, Message: msg}
}

// fromError 把任意 error 归一化为 *errors.Error，便于 HTTP 状态码与响应体一致。
//
// 实现策略：
//  1. 已是 *errors.Error → 直接返回；
//  2. 否则用 errors.INTERNAL 包裹 err.Error()，作为兜底。
//
// 不修改 domain 层（避免动 stage 2），仅在 transport 内提供。
func fromError(err error) *errors.Error {
	if err == nil {
		return nil
	}
	var de *errors.Error
	if stderrors.As(err, &de) {
		return de
	}
	return errors.New(errors.INTERNAL, err.Error())
}

// writeError 统一写错误响应：HTTP 状态码 = Code.HTTPStatus()，Body = ErrorResponse。
//
// 任何 handler 在 catch 到 err 时都应调用 writeError(c, err)，避免每个 handler
// 重复写 c.JSON(..., errorResp(...)) 的样板代码。
func (h *Handler) writeError(c *gin.Context, err error) {
	de := fromError(err)
	c.JSON(de.Code.HTTPStatus(), errorResp(de.Code, de.Message))
}

// observeRequest 上报 HTTP 请求计数器。
//
// 命名约定：metric 名固定为 "http_request_total"，labels 包含 path/method，
// 让 Prometheus 侧可按 endpoint 维度统计 QPS。
func (h *Handler) observeRequest(c *gin.Context) {
	if h.obs == nil {
		return
	}
	h.obs.IncCounter("http_request_total", map[string]string{
		"path":   c.FullPath(),
		"method": c.Request.Method,
	})
}

// 不导出但需在 gin handler 中复用：把状态码数字转成 http.StatusXxx 文本（用于
// 日志）。仅当 logger 非 nil 时调用，避免 nil 解引用。
func statusText(code int) string {
	return http.StatusText(code)
}
