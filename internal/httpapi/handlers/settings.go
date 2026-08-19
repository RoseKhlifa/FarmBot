package handlers

import (
	"encoding/json"
	"github.com/gin-gonic/gin"
)

func (h *Handler) RegisterSettings(r gin.IRouter) {
	r.GET("/api/settings/default-plan", h.setting("default_plan"))
	r.PUT("/api/settings/default-plan", h.saveSetting("default_plan"))
	r.POST("/api/settings/default-plan/import", h.saveSetting("default_plan"))
	r.POST("/api/settings/default-plan/apply", h.saveSetting("default_plan"))
	r.POST("/api/settings/default-plan/reset", h.deleteSetting("default_plan"))
	r.POST("/api/settings/save", h.saveSetting("settings"))
	r.POST("/api/settings/theme", h.theme)
	r.POST("/api/settings/auto-code-refresh", h.saveSetting("auto_code_refresh"))
	r.POST("/api/settings/auto-code-refresh/run", func(c *gin.Context) { writeNotConfigured(c) })
	r.POST("/api/settings/offline-reminder", h.saveSetting("offline_reminder"))
	r.POST("/api/settings/offline-reminder/test", func(c *gin.Context) { writeNotConfigured(c) })
	r.GET("/api/settings", h.settings)
	r.GET("/api/settings/default", h.setting("settings_default"))
}
func (h *Handler) setting(key string) gin.HandlerFunc {
	return func(c *gin.Context) {
		if h.app().Config == nil {
			writeNotConfigured(c)
			return
		}
		value, err := h.app().Config.GetGlobal(c.Request.Context(), key)
		if err != nil {
			writeError(c, err)
			return
		}
		writeOK(c, value)
	}
}
func (h *Handler) saveSetting(key string) gin.HandlerFunc {
	return func(c *gin.Context) {
		if h.app().Config == nil {
			writeNotConfigured(c)
			return
		}
		var value json.RawMessage
		if !bindJSON(c, &value) {
			return
		}
		if err := h.app().Config.SetGlobal(c.Request.Context(), key, value); err != nil {
			writeError(c, err)
			return
		}
		writeOK(c, value)
	}
}
func (h *Handler) deleteSetting(key string) gin.HandlerFunc {
	return func(c *gin.Context) {
		if h.app().Config == nil {
			writeNotConfigured(c)
			return
		}
		if err := h.app().Config.DeleteGlobal(c.Request.Context(), key); err != nil {
			writeError(c, err)
			return
		}
		writeOK(c, nil)
	}
}
func (h *Handler) theme(c *gin.Context) {
	if h.app().Config == nil {
		writeNotConfigured(c)
		return
	}
	var body struct {
		Theme string `json:"theme"`
	}
	if !bindJSON(c, &body) {
		return
	}
	if err := h.app().Config.SetTheme(c.Request.Context(), body.Theme); err != nil {
		writeError(c, err)
		return
	}
	writeOK(c, body.Theme)
}
func (h *Handler) settings(c *gin.Context) {
	if h.app().Config == nil {
		writeNotConfigured(c)
		return
	}
	entries, err := h.app().Config.List(c.Request.Context())
	if err != nil {
		writeError(c, err)
		return
	}
	writeOK(c, entries)
}
