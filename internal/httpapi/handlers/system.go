package handlers

import (
	"context"
	"crypto/subtle"
	"database/sql"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/RoseKhlifa/FarmBot/internal/httpapi/middleware"
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
	r.POST("/api/admin/login-logo", h.saveLoginLogo)
	r.GET("/api/super-admin-announcement", h.publicValue("super-admin-announcement"))
	r.POST("/api/super-admin/announcement", h.savePublicValue("super-admin-announcement"))
	r.GET("/api/super-admin/anti-resale-config", h.publicValue("anti-resale-config"))
	r.POST("/api/super-admin/anti-resale-config", h.savePublicValue("anti-resale-config"))
	r.POST("/api/super-admin/check-account-limit", h.checkAccountLimit)
	r.POST("/api/super-admin/clear-data", h.clearData)
	r.POST("/api/super-admin-announcement/verify", h.verifySuperAdminAnnouncement)
	r.GET("/api/announcement", h.publicValue("announcement"))
	r.POST("/api/announcement/read", h.announcementRead)
	r.POST("/api/system/restart", middleware.RequireAdminRole, h.restart)
	r.GET("/api/public/login-links", h.publicValue("login-links"))
	r.POST("/api/announcement/clear", middleware.RequireAdminRole, h.clearAnnouncement)
	r.GET("/api/system/info", h.publicValue("system-info"))
	r.POST("/api/system/config", middleware.RequireAdminRole, h.saveSystemConfig)
}

func (h *Handler) verifySuperAdminAnnouncement(c *gin.Context) {
	if h.app().Config == nil {
		writeNotConfigured(c)
		return
	}
	var body struct {
		Password string `json:"password"`
	}
	if !bindJSON(c, &body) {
		return
	}
	raw, err := h.app().Config.GetGlobal(c.Request.Context(), "super-admin-announcement")
	if errors.Is(err, sql.ErrNoRows) {
		raw, err = h.app().Config.GetGlobal(c.Request.Context(), "legacy:superAdminAnnouncement")
	}
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			writeOK(c, gin.H{"valid": false})
			return
		}
		writeError(c, err)
		return
	}
	var stored struct {
		Password string `json:"password"`
	}
	_ = json.Unmarshal(raw, &stored)
	valid := stored.Password != "" && subtle.ConstantTimeCompare([]byte(strings.TrimSpace(body.Password)), []byte(stored.Password)) == 1
	writeOK(c, gin.H{"valid": valid})
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
	if errors.Is(err, sql.ErrNoRows) {
		writeOK(c, map[string]any{})
		return
	}
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

func (h *Handler) saveLoginLogo(c *gin.Context) {
	if h.app().Config == nil {
		writeNotConfigured(c)
		return
	}
	var value json.RawMessage
	if !bindJSON(c, &value) {
		return
	}
	if err := h.app().Config.SetGlobal(c.Request.Context(), "loginLogo", value); err != nil {
		writeError(c, err)
		return
	}
	writeOK(c, value)
}

func (h *Handler) checkAccountLimit(c *gin.Context) {
	if h.app().Accounts == nil || h.app().Users == nil {
		writeNotConfigured(c)
		return
	}
	user, ok := middleware.CurrentUser(c)
	if !ok {
		c.JSON(401, gin.H{"ok": false, "error": "Unauthorized"})
		return
	}
	accounts, err := h.app().Accounts.List(c.Request.Context())
	if err != nil {
		writeError(c, err)
		return
	}
	owned := 0
	for _, account := range accounts {
		if account.OwnerUser == user.Username {
			owned++
		}
	}
	writeOK(c, gin.H{"allowed": user.AccountLimit <= 0 || owned < user.AccountLimit, "used": owned, "limit": user.AccountLimit})
}

func (h *Handler) clearData(c *gin.Context) {
	if h.app().Accounts == nil {
		writeNotConfigured(c)
		return
	}
	accounts, err := h.app().Accounts.List(c.Request.Context())
	if err != nil {
		writeError(c, err)
		return
	}
	cleared := 0
	for _, account := range accounts {
		if h.app().Logs != nil {
			_ = h.app().Logs.ClearLogs(c.Request.Context(), account.ID)
		}
		if h.app().Cache != nil {
			_ = h.app().Cache.DeleteAccountCaches(c.Request.Context(), account.ID)
		}
		cleared++
	}
	writeOK(c, gin.H{"cleared": cleared})
}

func (h *Handler) restart(c *gin.Context) {
	if h.app().Accounts == nil {
		writeNotConfigured(c)
		return
	}
	var body struct {
		AccountID string `json:"accountId"`
	}
	if c.Request.ContentLength > 0 && !bindJSON(c, &body) {
		return
	}
	if strings.TrimSpace(body.AccountID) == "" {
		body.AccountID = middleware.AccountID(c)
	}
	body.AccountID = strings.TrimSpace(body.AccountID)
	if body.AccountID == "" {
		writeError(c, errors.New("accountId is required"))
		return
	}
	if !h.requireAccountOwner(c, body.AccountID) {
		return
	}
	accounts, err := h.app().Accounts.List(c.Request.Context())
	if err != nil {
		writeError(c, err)
		return
	}
	found := false
	for _, account := range accounts {
		if strings.EqualFold(strings.TrimSpace(account.ID), body.AccountID) {
			found = true
			break
		}
	}
	if !found {
		c.JSON(404, gin.H{"ok": false, "error": "account not found"})
		return
	}
	if restarter, ok := h.app().Accounts.(interface {
		Restart(context.Context, string) error
	}); ok {
		if err := restarter.Restart(c.Request.Context(), body.AccountID); err != nil {
			writeError(c, err)
			return
		}
		writeOK(c, nil)
		return
	}
	if err := h.app().Accounts.Stop(c.Request.Context(), body.AccountID); err != nil {
		writeError(c, err)
		return
	}
	if err := h.app().Accounts.Start(c.Request.Context(), body.AccountID); err != nil {
		writeError(c, err)
		return
	}
	writeOK(c, nil)
}

func (h *Handler) clearAnnouncement(c *gin.Context) {
	if h.app().Config == nil {
		writeNotConfigured(c)
		return
	}
	if err := h.app().Config.DeleteGlobal(c.Request.Context(), "announcement"); err != nil {
		writeError(c, err)
		return
	}
	// Migrations may have left the value under the legacy key. Removing both
	// makes the clear action durable across old and new databases.
	_ = h.app().Config.DeleteGlobal(c.Request.Context(), "legacy:announcement")
	writeOK(c, nil)
}

func (h *Handler) announcementRead(c *gin.Context) {
	username, ok := currentUsername(c)
	if !ok || h.app().Config == nil {
		if ok {
			writeNotConfigured(c)
		}
		return
	}
	const readKey = "announcement_read_records"
	records := map[string]int64{}
	if raw, err := h.app().Config.GetGlobal(c.Request.Context(), readKey); err == nil {
		_ = json.Unmarshal(raw, &records)
	}
	readAt := time.Now().UnixMilli()
	records[username] = readAt
	raw, err := json.Marshal(records)
	if err != nil {
		writeError(c, err)
		return
	}
	if err := h.app().Config.SetGlobal(c.Request.Context(), readKey, raw); err != nil {
		writeError(c, err)
		return
	}
	writeOK(c, gin.H{"username": username, "readAt": readAt})
}
