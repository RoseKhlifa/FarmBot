package handlers

import (
	"encoding/json"

	"github.com/RoseKhlifa/FarmBot/internal/store"
	"github.com/gin-gonic/gin"
)

func (h *Handler) RegisterSystem(r gin.IRouter) {
	r.POST("/api/admin/announcement", h.adminAnnouncement)
	r.GET("/api/admin/system-config", h.systemConfig)
	r.POST("/api/admin/system-config", h.saveSystemConfig)
	r.POST("/api/admin/system-config/reset", h.resetSystemConfig)
	r.GET("/api/admin/wx-config", h.adminWXConfig)
	r.POST("/api/admin/wx-config", h.saveWXConfig)
	r.GET("/api/admin/login-links", h.publicValue("login-links"))
	r.POST("/api/admin/login-links", h.savePublicValue("login-links"))
	r.POST("/api/admin/login-links/reset", h.deletePublicValue("login-links"))
	r.POST("/api/admin/login-logo", func(c *gin.Context) { writeNotConfigured(c) })
	r.GET("/api/super-admin-announcement", h.publicValue("super-admin-announcement"))
	r.POST("/api/super-admin/announcement", h.savePublicValue("super-admin-announcement"))
	r.GET("/api/super-admin/anti-resale-config", h.publicValue("anti-resale-config"))
	r.POST("/api/super-admin/anti-resale-config", h.savePublicValue("anti-resale-config"))
	r.POST("/api/super-admin/check-account-limit", func(c *gin.Context) { writeNotConfigured(c) })
	r.POST("/api/super-admin/clear-data", func(c *gin.Context) { writeNotConfigured(c) })
	r.POST("/api/super-admin-announcement", h.publicValue("super-admin-announcement"))
	r.POST("/api/super-admin-announcement/verify", h.publicValue("super-admin-announcement-verify"))
	r.GET("/api/announcement", h.publicValue("announcement"))
	r.POST("/api/announcement/read", h.publicValue("announcement-read"))
	r.POST("/api/system/restart", func(c *gin.Context) { writeNotConfigured(c) })
	r.GET("/api/public/login-links", h.publicValue("login-links"))
	r.POST("/api/announcement/clear", func(c *gin.Context) { writeNotConfigured(c) })
	r.GET("/api/system/info", h.publicValue("system-info"))
	r.POST("/api/system/config", func(c *gin.Context) { writeNotConfigured(c) })
}
func (h *Handler) publicValue(key string) gin.HandlerFunc {
	return func(c *gin.Context) {
		if h.app().Public == nil {
			writeNotConfigured(c)
			return
		}
		data, err := h.app().Public.Value(c.Request.Context(), key)
		if err != nil {
			writeError(c, err)
			return
		}
		writeOK(c, data)
	}
}

func (h *Handler) adminAnnouncement(c *gin.Context) {
	h.savePublicValue("announcement")(c)
}

func (h *Handler) systemConfig(c *gin.Context) {
	if h.app().Config == nil {
		writeNotConfigured(c)
		return
	}
	value, err := h.app().Config.GetSystemConfig(c.Request.Context())
	if err != nil {
		writeError(c, err)
		return
	}
	writeOK(c, gin.H{"saved": value, "current": value})
}

func (h *Handler) saveSystemConfig(c *gin.Context) {
	if h.app().Config == nil {
		writeNotConfigured(c)
		return
	}
	var value map[string]any
	if !bindJSON(c, &value) {
		return
	}
	payload, err := json.Marshal(value)
	if err != nil {
		writeError(c, err)
		return
	}
	if err := h.app().Config.SetSystemConfig(c.Request.Context(), payload); err != nil {
		writeError(c, err)
		return
	}
	writeOK(c, gin.H{"saved": value, "current": value})
}

func (h *Handler) resetSystemConfig(c *gin.Context) {
	if h.app().Config == nil {
		writeNotConfigured(c)
		return
	}
	if err := h.app().Config.DeleteGlobal(c.Request.Context(), store.ConfigKeySystem); err != nil {
		writeError(c, err)
		return
	}
	writeOK(c, gin.H{"saved": map[string]any{}, "current": map[string]any{}})
}

func (h *Handler) adminWXConfig(c *gin.Context) {
	if h.app().Config == nil {
		writeNotConfigured(c)
		return
	}
	value, err := h.app().Config.GetWXConfig(c.Request.Context())
	if err != nil {
		writeError(c, err)
		return
	}
	writeOK(c, value)
}

func (h *Handler) saveWXConfig(c *gin.Context) {
	if h.app().Config == nil {
		writeNotConfigured(c)
		return
	}
	var value map[string]any
	if !bindJSON(c, &value) {
		return
	}
	payload, err := json.Marshal(value)
	if err != nil {
		writeError(c, err)
		return
	}
	if err := h.app().Config.SetWXConfig(c.Request.Context(), payload); err != nil {
		writeError(c, err)
		return
	}
	writeOK(c, value)
}

func (h *Handler) savePublicValue(key string) gin.HandlerFunc {
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

func (h *Handler) deletePublicValue(key string) gin.HandlerFunc {
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
