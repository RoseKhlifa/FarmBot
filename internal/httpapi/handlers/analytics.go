package handlers

import "github.com/gin-gonic/gin"

func (h *Handler) RegisterAnalytics(r gin.IRouter) { r.GET("/api/analytics", h.analytics) }
func (h *Handler) analytics(c *gin.Context) {
	id, ok := accountID(c, true)
	if !ok {
		return
	}
	if h.app().Runtime == nil {
		writeNotConfigured(c)
		return
	}
	data, err := h.app().Runtime.Status(c.Request.Context(), id)
	if err != nil {
		writeError(c, err)
		return
	}
	writeOK(c, data)
}
