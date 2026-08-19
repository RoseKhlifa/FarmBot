package handlers

import "github.com/gin-gonic/gin"

func (h *Handler) RegisterAccount(r gin.IRouter) {
	r.GET("/api/accounts", h.accounts)
	r.POST("/api/accounts/refresh-wx-codes", func(c *gin.Context) { writeNotConfigured(c) })
	r.POST("/api/account/remark", h.accountRemark)
	r.POST("/api/accounts", func(c *gin.Context) { writeNotConfigured(c) })
	r.DELETE("/api/accounts/:id", h.accountDelete)
	r.GET("/api/account-logs", h.accountLogs)
	r.GET("/api/logs", h.logs)
	r.DELETE("/api/logs", h.clearLogs)
	r.POST("/api/accounts/:id/start", h.accountStart)
	r.POST("/api/accounts/:id/stop", h.accountStop)
}
func (h *Handler) accounts(c *gin.Context) {
	if h.app().Accounts == nil {
		writeNotConfigured(c)
		return
	}
	data, err := h.app().Accounts.List(c.Request.Context())
	if err != nil {
		writeError(c, err)
		return
	}
	writeOK(c, data)
}
func (h *Handler) accountRemark(c *gin.Context) {
	if h.app().Accounts == nil {
		writeNotConfigured(c)
		return
	}
	var body struct{ AccountID, Remark string }
	if !bindJSON(c, &body) {
		return
	}
	if body.AccountID == "" {
		body.AccountID, _ = accountID(c, true)
	}
	if err := h.app().Accounts.SetRemark(c.Request.Context(), body.AccountID, body.Remark); err != nil {
		writeError(c, err)
		return
	}
	writeOK(c, nil)
}
func (h *Handler) accountDelete(c *gin.Context) {
	if h.app().Accounts == nil {
		writeNotConfigured(c)
		return
	}
	id := c.Param("id")
	if id == "" {
		c.JSON(400, gin.H{"ok": false, "error": "invalid id"})
		return
	}
	if err := h.app().Accounts.Delete(c.Request.Context(), id); err != nil {
		writeError(c, err)
		return
	}
	writeOK(c, nil)
}
func (h *Handler) accountStart(c *gin.Context) {
	if h.app().Accounts == nil {
		writeNotConfigured(c)
		return
	}
	if err := h.app().Accounts.Start(c.Request.Context(), c.Param("id")); err != nil {
		writeError(c, err)
		return
	}
	writeOK(c, nil)
}
func (h *Handler) accountStop(c *gin.Context) {
	if h.app().Accounts == nil {
		writeNotConfigured(c)
		return
	}
	if err := h.app().Accounts.Stop(c.Request.Context(), c.Param("id")); err != nil {
		writeError(c, err)
		return
	}
	writeOK(c, nil)
}
func (h *Handler) accountLogs(c *gin.Context) {
	id, ok := accountID(c, false)
	if !ok || h.app().Logs == nil {
		if ok {
			writeNotConfigured(c)
		}
		return
	}
	data, err := h.app().Logs.AccountLogs(c.Request.Context(), id, queryInt(c, "limit", 100))
	if err != nil {
		writeError(c, err)
		return
	}
	writeOK(c, data)
}
func (h *Handler) logs(c *gin.Context) {
	id, ok := accountID(c, false)
	if !ok || h.app().Logs == nil {
		if ok {
			writeNotConfigured(c)
		}
		return
	}
	data, err := h.app().Logs.Logs(c.Request.Context(), id, queryInt(c, "limit", 100))
	if err != nil {
		writeError(c, err)
		return
	}
	writeOK(c, data)
}
func (h *Handler) clearLogs(c *gin.Context) {
	id, ok := accountID(c, true)
	if !ok || h.app().Logs == nil {
		if ok {
			writeNotConfigured(c)
		}
		return
	}
	if err := h.app().Logs.ClearLogs(c.Request.Context(), id); err != nil {
		writeError(c, err)
		return
	}
	writeOK(c, nil)
}
