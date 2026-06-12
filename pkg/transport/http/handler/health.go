package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// Healthz 处理 GET /healthz 请求：进程存活探针，永远返回 200。
//
// Kubernetes/Consul 等探针会调用此端点；只要进程未僵死就 OK，
// 不检查下游依赖（避免雪崩时 Pod 被全量重启）。
//
// 出参：200 + {"status": "ok"}。
func (h *Handler) Healthz(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

// Readyz 处理 GET /readyz 请求：就绪探针。
//
// 简化实现：默认返回 200；如未来需要 readiness 检查（如依赖未就绪返回 503），
// 可以在此扩展（不破坏当前契约）。
//
// 出参：200 + {"status": "ready"}。
func (h *Handler) Readyz(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"status": "ready"})
}
