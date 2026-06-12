package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/zhuge/kongming/pkg/domain/errors"
	"github.com/zhuge/kongming/pkg/domain/model"
	"github.com/zhuge/kongming/pkg/transport/http/dto"
)

// PlanStrategy 处理 POST /api/v1/strategies 请求。
//
// 仅做战略规划，不执行战术（区别于 CreateOrder → Dispatch）。
// 适用场景：派单前预览、Reviewer 评估方案。
//
// 入参：dto.CreateOrderRequest（复用 Order 创建入参的 DTO）。
// 出参：200 + Strategy / 4xx-5xx + ErrorResponse。
// 错误码：INVALID_ARGUMENT（400）/ STRATEGY_FAILED（500）/ INTERNAL（500）。
func (h *Handler) PlanStrategy(c *gin.Context) {
	h.observeRequest(c)

	var req dto.CreateOrderRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, errorResp(errors.INVALID_ARGUMENT, err.Error()))
		return
	}

	// 构造最小 Order（无 ID 也可，planner 不依赖 ID；这里仍生成 UUID
	// 便于日志/审计能反查到原始 Order）。
	order := &model.Order{
		ID:          model.OrderID(uuid.NewString()),
		Name:        req.Name,
		Description: req.Description,
		Priority:    model.Priority(req.Priority),
		Strategy: model.Strategy{
			Objectives: req.Objectives,
		},
	}

	strat, err := h.c.PlanStrategy(c.Request.Context(), order)
	if err != nil {
		h.writeError(c, err)
		return
	}
	c.JSON(http.StatusOK, strat)
}
