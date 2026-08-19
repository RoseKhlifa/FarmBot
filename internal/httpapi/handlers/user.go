package handlers

import (
	"github.com/RoseKhlifa/FarmBot/internal/httpapi/middleware"
	"github.com/RoseKhlifa/FarmBot/internal/store"
	"github.com/gin-gonic/gin"
)

func (h *Handler) RegisterUser(r gin.IRouter) {
	r.GET("/api/user/me", h.currentUser)
	r.GET("/api/user/wxlogin-config", h.wxConfig)
	r.POST("/api/user/wxlogin-config", h.saveWXConfig)
	r.POST("/api/user/device-protocol", func(c *gin.Context) { writeNotConfigured(c) })
	r.GET("/api/user/device-protocol", func(c *gin.Context) { writeNotConfigured(c) })
	r.GET("/api/admin/users", h.adminUsers)
	r.GET("/api/admin/users-with-password", h.adminUsersWithPassword)
	r.POST("/api/admin/users/clear-expired", func(c *gin.Context) { writeNotConfigured(c) })
	r.POST("/api/admin/users/:username", h.adminUserUpdate)
	r.POST("/api/admin/users/:username/edit", h.adminUserEdit)
	r.POST("/api/admin/users/:username/renew", h.adminUserRenew)
	r.DELETE("/api/admin/users/:username", h.adminUserDelete)
}
func (h *Handler) currentUser(c *gin.Context) {
	user, ok := middleware.CurrentUser(c)
	if !ok {
		c.JSON(401, gin.H{"ok": false, "error": "Unauthorized"})
		return
	}
	writeOK(c, user)
}
func (h *Handler) wxConfig(c *gin.Context) {
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

func (h *Handler) adminUsers(c *gin.Context) {
	if h.app().Users == nil {
		writeNotConfigured(c)
		return
	}
	users, err := h.app().Users.List(c.Request.Context())
	if err != nil {
		writeError(c, err)
		return
	}
	writeOK(c, users)
}

func (h *Handler) adminUsersWithPassword(c *gin.Context) { h.adminUsers(c) }

func (h *Handler) adminUserUpdate(c *gin.Context) {
	if h.app().Users == nil {
		writeNotConfigured(c)
		return
	}
	user, err := h.app().Users.Get(c.Request.Context(), c.Param("username"))
	if err != nil {
		writeError(c, err)
		return
	}
	var patch store.User
	if c.Request.ContentLength > 0 && !bindJSON(c, &patch) {
		return
	}
	patch.Username = user.Username
	if err := h.app().Users.Update(c.Request.Context(), patch); err != nil {
		writeError(c, err)
		return
	}
	updated, err := h.app().Users.Get(c.Request.Context(), user.Username)
	if err != nil {
		writeError(c, err)
		return
	}
	writeOK(c, updated)
}

func (h *Handler) adminUserEdit(c *gin.Context) { h.adminUserUpdate(c) }

func (h *Handler) adminUserRenew(c *gin.Context) {
	if h.app().Cards == nil {
		writeNotConfigured(c)
		return
	}
	var body struct {
		CardCode string `json:"cardCode"`
	}
	if !bindJSON(c, &body) {
		return
	}
	user, err := h.app().Cards.Renew(c.Request.Context(), c.Param("username"), body.CardCode)
	if err != nil {
		writeError(c, err)
		return
	}
	writeOK(c, user)
}

func (h *Handler) adminUserDelete(c *gin.Context) {
	if h.app().Users == nil {
		writeNotConfigured(c)
		return
	}
	if err := h.app().Users.Delete(c.Request.Context(), c.Param("username")); err != nil {
		writeError(c, err)
		return
	}
	writeOK(c, nil)
}
