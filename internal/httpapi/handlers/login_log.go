package handlers

import "github.com/gin-gonic/gin"

func (h *Handler) RegisterLoginLog(r gin.IRouter) {
	r.GET("/api/admin/login-logs", h.loginLogs)
	r.DELETE("/api/admin/login-logs", h.clearLoginLogs)
}
func (h *Handler) loginLogs(c *gin.Context) {
	if h.app().Users == nil {
		writeNotConfigured(c)
		return
	}
	data, total, err := h.app().Users.GetLoginLogs(c.Request.Context(), queryInt(c, "limit", 100), queryInt(c, "offset", 0))
	if err != nil {
		writeError(c, err)
		return
	}
	writeOK(c, gin.H{"items": data, "total": total})
}
func (h *Handler) clearLoginLogs(c *gin.Context) {
	if h.app().Users == nil {
		writeNotConfigured(c)
		return
	}
	if err := h.app().Users.ClearLoginLogs(c.Request.Context()); err != nil {
		writeError(c, err)
		return
	}
	writeOK(c, nil)
}
