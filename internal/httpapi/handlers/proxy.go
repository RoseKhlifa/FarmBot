package handlers

import "github.com/gin-gonic/gin"

func (h *Handler) RegisterProxy(r gin.IRouter) { r.POST("/api/proxy", h.proxy) }
func (h *Handler) proxy(c *gin.Context) {
	if h.app().Proxy == nil {
		writeNotConfigured(c)
		return
	}
	var body map[string]any
	if !bindJSON(c, &body) {
		return
	}
	data, err := h.app().Proxy.Handle(c.Request.Context(), body)
	if err != nil {
		writeError(c, err)
		return
	}
	writeOK(c, data)
}
