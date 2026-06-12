package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/zhuge/kongming/pkg/domain/errors"
	"github.com/zhuge/kongming/pkg/domain/model"
)

// ListGenerals 处理 GET /api/v1/generals 请求。
//
// 入参：无。
// 出参：200 + {"generals": [...]} / 4xx-5xx + ErrorResponse。
// 错误码：INTERNAL（500）。
func (h *Handler) ListGenerals(c *gin.Context) {
	h.observeRequest(c)

	generals, err := h.p.List(c.Request.Context())
	if err != nil {
		h.writeError(c, err)
		return
	}
	if generals == nil {
		generals = []*model.General{}
	}
	c.JSON(http.StatusOK, gin.H{"generals": generals})
}

// GetGeneral 处理 GET /api/v1/generals/:id 请求。
//
// 入参：路径参数 id（GeneralID）。
// 出参：200 + General / 4xx-5xx + ErrorResponse。
// 错误码：NOT_FOUND（404）/ INVALID_ARGUMENT（400）/ INTERNAL（500）。
func (h *Handler) GetGeneral(c *gin.Context) {
	h.observeRequest(c)

	id := model.GeneralID(c.Param("id"))
	if id == "" {
		c.JSON(http.StatusBadRequest, errorResp(errors.INVALID_ARGUMENT, "id is required"))
		return
	}

	g, err := h.p.Get(c.Request.Context(), id)
	if err != nil {
		h.writeError(c, err)
		return
	}
	c.JSON(http.StatusOK, g)
}
