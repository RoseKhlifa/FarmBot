package handlers

import "github.com/gin-gonic/gin"

func (h *Handler) RegisterQRLogin(r gin.IRouter) {
	r.POST("/api/qr/create", h.qr)
	r.POST("/api/qr/check", h.qr)
}
func (h *Handler) qr(c *gin.Context) {
	if h.app().QR == nil {
		writeNotConfigured(c)
		return
	}
	var body map[string]any
	if c.Request.ContentLength > 0 && !bindJSON(c, &body) {
		return
	}
	data, err := h.app().QR.Handle(c.Request.Context(), c.Request.URL.Path, body)
	if err != nil {
		writeError(c, err)
		return
	}
	writeOK(c, data)
}
