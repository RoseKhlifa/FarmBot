package handlers

import (
	"strings"

	"github.com/RoseKhlifa/FarmBot/internal/httpapi/middleware"
	"github.com/gin-gonic/gin"
)

func (h *Handler) RegisterPublicInfo(r gin.IRouter) {
	r.GET("/api/ping", func(c *gin.Context) { writeOK(c, gin.H{"ok": true}) })
	r.GET("/api/game-version", func(c *gin.Context) { writeOK(c, gin.H{"version": "go"}) })
	r.GET("/api/user-count", h.publicValue("user-count"))
	r.GET("/api/anti-resale-config", h.publicValue("anti-resale-config"))
	r.GET("/api/changelog", h.publicValue("changelog"))
	r.GET("/api/auth/validate", func(c *gin.Context) { writeOK(c, gin.H{"valid": true}) })
	r.GET("/api/scheduler", h.schedulerInfo)
	r.GET("/api/debug/item-config", h.publicValue("debug-item-config"))
}

func (h *Handler) schedulerInfo(c *gin.Context) {
	id := strings.TrimSpace(middleware.AccountID(c))
	if id == "" || h.app().Runtime == nil {
		writeOK(c, gin.H{"accountId": id, "running": false, "tasks": []any{}})
		return
	}
	if !h.requireAccountOwner(c, id) {
		return
	}
	data, err := h.app().Runtime.Scheduler(c.Request.Context(), id)
	if err != nil {
		writeError(c, err)
		return
	}
	writeOK(c, data)
}
