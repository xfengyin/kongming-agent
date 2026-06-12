package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/zhuge/kongming/pkg/domain/errors"
	"github.com/zhuge/kongming/pkg/transport/http/dto"
)

// RunWorkflow 处理 POST /api/v1/workflows/:id/run 请求。
//
// 入参：路径参数 id（WorkflowID）；body 为 dto.RunWorkflowRequest。
// 出参：200 + ExecutionContext / 4xx-5xx + ErrorResponse。
// 错误码：NOT_FOUND（404）/ INVALID_ARGUMENT（400）/ INTERNAL（500）。
func (h *Handler) RunWorkflow(c *gin.Context) {
	h.observeRequest(c)

	id := c.Param("id")
	if id == "" {
		c.JSON(http.StatusBadRequest, errorResp(errors.INVALID_ARGUMENT, "id is required"))
		return
	}

	// body 可选（无 body 时 inputs=nil）。
	var req dto.RunWorkflowRequest
	if c.Request.ContentLength > 0 {
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, errorResp(errors.INVALID_ARGUMENT, err.Error()))
			return
		}
	}

	ec, err := h.e.Execute(c.Request.Context(), id, req.Inputs)
	if err != nil {
		h.writeError(c, err)
		return
	}
	c.JSON(http.StatusOK, ec)
}
