package middleware

import (
	"fmt"
	"net/http"
	"runtime/debug"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// Recovery panic 恢复中间件。
//
// 行为：
//  1. 捕获 c.Next() 链路中任何 panic；
//  2. 记录 stack + error 到 zap；
//  3. 返回 500 + ErrorResponse（与 spec §5.1 一致）。
//
// 注意：必须保证 panic 不会冒泡到 gin 默认的 recovery（我们替换了它），
// 因此 server.go 中使用 gin.New() 而非 gin.Default()。
func Recovery(logger *zap.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			if r := recover(); r != nil {
				// 记录 stack 与 panic 值。
				if logger != nil {
					logger.Error("panic recovered in http handler",
						zap.Any("panic", r),
						zap.String("path", c.Request.URL.Path),
						zap.String("method", c.Request.Method),
						zap.String("stack", string(debug.Stack())),
					)
				}
				// 终止后续 handler，写统一 500 响应。
				c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{
					"code":    "INTERNAL",
					"message": fmt.Sprintf("internal server error: %v", r),
				})
			}
		}()
		c.Next()
	}
}
