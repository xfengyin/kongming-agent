package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/zhuge/kongming/pkg/domain/errors"
	"github.com/zhuge/kongming/pkg/domain/model"
	"github.com/zhuge/kongming/pkg/transport/http/dto"
)

// ListJinnang 处理 GET /api/v1/vault 请求。
//
// 入参：无。
// 出参：200 + {"jinnang": [...]} / 4xx-5xx + ErrorResponse。
// 错误码：INTERNAL（500）。
func (h *Handler) ListJinnang(c *gin.Context) {
	h.observeRequest(c)

	js, err := h.v.ListJinnang()
	if err != nil {
		h.writeError(c, err)
		return
	}
	if js == nil {
		js = []*model.Jinnang{}
	}
	c.JSON(http.StatusOK, gin.H{"jinnang": js})
}

// ExecuteJinnang 处理 POST /api/v1/vault/:id/exec 请求。
//
// 入参：路径参数 id（JinnangID）；body 为 dto.ExecuteJinnangRequest。
// 出参：200 + JinnangOutput / 4xx-5xx + ErrorResponse。
// 错误码：NOT_FOUND（404）/ INVALID_ARGUMENT（400）/ INTERNAL（500）。
func (h *Handler) ExecuteJinnang(c *gin.Context) {
	h.observeRequest(c)

	id := c.Param("id")
	if id == "" {
		c.JSON(http.StatusBadRequest, errorResp(errors.INVALID_ARGUMENT, "id is required"))
		return
	}

	// body 可选（GET 风格：空 body 视为零值入参）。
	var req dto.ExecuteJinnangRequest
	if c.Request.ContentLength > 0 {
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, errorResp(errors.INVALID_ARGUMENT, err.Error()))
			return
		}
	}

	input := model.JinnangInput{
		Params: req.Params,
		Data:   req.Data,
	}

	out, err := h.v.Execute(c.Request.Context(), id, input)
	if err != nil {
		h.writeError(c, err)
		return
	}
	c.JSON(http.StatusOK, out)
}
