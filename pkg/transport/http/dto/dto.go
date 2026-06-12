// Package dto 定义 HTTP 传输层与外部交互的请求/响应数据结构。
//
// 设计原则：
//  1. DTO 与领域 model 解耦：领域层不感知 HTTP，反序列化不直接绑定 model.* 字段。
//  2. 字段可选：未提供的字段在 handler 中按业务规则取零值或默认值。
//  3. 中文注释：标明每个字段的业务语义，便于 reviewer 阅读。
package dto

import (
	domerrs "github.com/zhuge/kongming/pkg/domain/errors"
)

// CreateOrderRequest 是 POST /api/v1/orders 的请求体。
//
// 字段说明：
//   - Name         订单名（必填，下游展示用）。
//   - Description  订单详细描述（可选）。
//   - Priority     优先级（1-4 整数；0 表示未指定，handler 兜底为 PriorityNormal）。
//   - Objectives   战略目标列表（自然语言），会写入 Strategy.Objectives。
type CreateOrderRequest struct {
	Name        string   `json:"name" binding:"required"`
	Description string   `json:"description"`
	Priority    int      `json:"priority"`
	Objectives  []string `json:"objectives"`
}

// ExecuteJinnangRequest 是 POST /api/v1/vault/:id/exec 的请求体。
//
// 字段说明：
//   - Params 本次调用的可变参数（timeouts/retries/...）。
//   - Data   业务载荷（如 LLM prompt、查询语句），handler 直接透传。
type ExecuteJinnangRequest struct {
	Params map[string]any `json:"params"`
	Data   any            `json:"data"`
}

// RunWorkflowRequest 是 POST /api/v1/workflows/:id/run 的请求体。
//
// Inputs 会被注入到 ExecutionContext.Variables，作为工作流级共享变量。
type RunWorkflowRequest struct {
	Inputs map[string]any `json:"inputs"`
}

// ErrorResponse 是统一的错误响应结构。
//
// 与 spec §5.1 一致：{"code": "NOT_FOUND", "message": "order o1 not found"}。
// HTTP 状态码由 Code.HTTPStatus() 计算，与 Body.Code 保持语义一致。
type ErrorResponse struct {
	Code    domerrs.Code `json:"code"`
	Message string       `json:"message"`
}
