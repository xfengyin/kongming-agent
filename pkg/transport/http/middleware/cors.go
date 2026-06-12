package middleware

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// CORS 跨域中间件（dev 友好：allow-all）。
//
// 设计取舍：
//  1. 生产环境建议改为白名单（Origin 校验）；当前为开箱即用体验优先。
//  2. 预检请求（OPTIONS）直接 204，不再走业务 handler。
//  3. 暴露 X-Trace-Id header，方便跨服务排查。
//
// 注意：本中间件不修改 c.Request.Context()，是「纯旁路」中间件。
func CORS() gin.HandlerFunc {
	return func(c *gin.Context) {
		// 允许任意源。
		c.Header("Access-Control-Allow-Origin", "*")
		c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, PATCH, OPTIONS")
		c.Header("Access-Control-Allow-Headers", "Content-Type, Authorization, X-Trace-Id")
		c.Header("Access-Control-Expose-Headers", "X-Trace-Id")
		c.Header("Access-Control-Max-Age", "86400")

		// 预检请求直接 204。
		if c.Request.Method == http.MethodOptions {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}
		c.Next()
	}
}
