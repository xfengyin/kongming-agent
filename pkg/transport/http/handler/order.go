package handler

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/zhuge/kongming/pkg/domain/errors"
	"github.com/zhuge/kongming/pkg/domain/model"
	"github.com/zhuge/kongming/pkg/transport/http/dto"
)

// CreateOrder 处理 POST /api/v1/orders 请求。
//
// 入参：dto.CreateOrderRequest（JSON body）。
// 出参：200 + {order_id, report} / 4xx-5xx + ErrorResponse。
// 错误码：
//   - INVALID_ARGUMENT（400）：body 非法 / 必填字段缺失
//   - INVALID_STATE    （409）：Commander 状态机非法
//   - STRATEGY_FAILED  （500）：战略规划失败
//   - PERSIST_FAILED   （500）：持久化失败
//   - INTERNAL         （500）：兜底
func (h *Handler) CreateOrder(c *gin.Context) {
	h.observeRequest(c)

	var req dto.CreateOrderRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, errorResp(errors.INVALID_ARGUMENT, err.Error()))
		return
	}

	// Priority 0 表示未指定，handler 兜底为 PriorityNormal。
	prio := model.Priority(req.Priority)
	if prio == 0 {
		prio = model.PriorityNormal
	}

	order := &model.Order{
		ID:          model.OrderID(uuid.NewString()),
		Name:        req.Name,
		Description: req.Description,
		State:       model.StatePending,
		Priority:    prio,
		Strategy: model.Strategy{
			Objectives: req.Objectives,
		},
		Context: map[string]any{},
	}

	report, err := h.c.Dispatch(c.Request.Context(), order)
	if err != nil {
		h.writeError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"order_id": order.ID,
		"report":   report,
	})
}

// GetOrder 处理 GET /api/v1/orders/:id 请求。
//
// 入参：路径参数 id（OrderID）。
// 出参：200 + Order JSON / 4xx-5xx + ErrorResponse。
// 错误码：NOT_FOUND（404）/ INVALID_ARGUMENT（400）/ INTERNAL（500）。
func (h *Handler) GetOrder(c *gin.Context) {
	h.observeRequest(c)

	id := model.OrderID(c.Param("id"))
	if id == "" {
		c.JSON(http.StatusBadRequest, errorResp(errors.INVALID_ARGUMENT, "id is required"))
		return
	}

	order, err := h.c.GetOrder(c.Request.Context(), id)
	if err != nil {
		h.writeError(c, err)
		return
	}
	c.JSON(http.StatusOK, order)
}

// ListOrders 处理 GET /api/v1/orders 请求。
//
// 入参：可选 query state（State 枚举的整数值，0 表示不过滤）。
// 出参：200 + {"orders": [...]} / 4xx-5xx + ErrorResponse。
// 错误码：INVALID_ARGUMENT（400）/ INTERNAL（500）。
func (h *Handler) ListOrders(c *gin.Context) {
	h.observeRequest(c)

	state := model.StateNone
	if raw := c.Query("state"); raw != "" {
		v, err := strconv.Atoi(raw)
		if err != nil {
			c.JSON(http.StatusBadRequest, errorResp(errors.INVALID_ARGUMENT, "invalid state query: "+err.Error()))
			return
		}
		state = model.State(v)
	}

	orders, err := h.c.ListOrders(c.Request.Context(), state)
	if err != nil {
		h.writeError(c, err)
		return
	}
	// 防止 nil 反序列化得到 null，统一给空切片。
	if orders == nil {
		orders = []*model.Order{}
	}
	c.JSON(http.StatusOK, gin.H{"orders": orders})
}
