package handlers

import (
	"database/sql"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/RoseKhlifa/FarmBot/internal/httpapi/middleware"
	"github.com/RoseKhlifa/FarmBot/internal/store"
	"github.com/gin-gonic/gin"
)

func (h *Handler) RegisterUser(r gin.IRouter) {
	r.GET("/api/user/me", h.currentUser)
	r.GET("/api/user/wxlogin-config", h.wxConfig)
	r.POST("/api/user/wxlogin-config", h.saveWXConfig)
	r.POST("/api/user/device-protocol", h.saveDeviceProtocol)
	r.GET("/api/user/device-protocol", h.deviceProtocol)
	r.GET("/api/admin/users", h.adminUsers)
	r.GET("/api/admin/users-with-password", h.adminUsersWithPassword)
	r.POST("/api/admin/users/clear-expired", h.clearExpiredUsers)
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
	writeOK(c, publicUser(user))
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

func (h *Handler) deviceProtocol(c *gin.Context) {
	if h.app().Config == nil {
		writeNotConfigured(c)
		return
	}
	value, err := h.app().Config.GetGlobal(c.Request.Context(), "deviceProtocol")
	if err != nil {
		// Device protocol settings are optional. A fresh installation has no
		// row yet, so return an empty object and let the client apply defaults.
		if errors.Is(err, sql.ErrNoRows) {
			writeOK(c, map[string]any{})
			return
		}
		writeError(c, err)
		return
	}
	writeOK(c, value)
}
func (h *Handler) saveDeviceProtocol(c *gin.Context) {
	if h.app().Config == nil {
		writeNotConfigured(c)
		return
	}
	var value json.RawMessage
	if !bindJSON(c, &value) {
		return
	}
	if err := h.app().Config.SetGlobal(c.Request.Context(), "deviceProtocol", value); err != nil {
		writeError(c, err)
		return
	}
	writeOK(c, value)
}
func (h *Handler) clearExpiredUsers(c *gin.Context) {
	if h.app().Users == nil {
		writeNotConfigured(c)
		return
	}
	users, err := h.app().Users.List(c.Request.Context())
	if err != nil {
		writeError(c, err)
		return
	}
	now := time.Now().UnixMilli()
	cleared := 0
	for _, user := range users {
		if user.Role == "super_admin" || user.Role == "admin" || user.ExpireAt == nil || *user.ExpireAt > now {
			continue
		}
		if err := h.app().Users.Delete(c.Request.Context(), user.Username); err != nil {
			writeError(c, err)
			return
		}
		if h.app().Sessions != nil {
			_ = h.app().Sessions.InvalidateAll(c.Request.Context(), func(session middleware.Session) bool {
				return session.User.Username == user.Username
			})
		}
		cleared++
	}
	writeOK(c, gin.H{"cleared": cleared})
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
	writeOK(c, publicUsers(users))
}

func (h *Handler) adminUsersWithPassword(c *gin.Context) { h.adminUsers(c) }

func (h *Handler) adminUserUpdate(c *gin.Context) {
	if h.app().Users == nil {
		writeNotConfigured(c)
		return
	}
	username := strings.TrimSpace(c.Param("username"))
	var body struct {
		NewUsername  string          `json:"newUsername"`
		Password     string          `json:"password"`
		AccountLimit *int            `json:"accountLimit"`
		ExpiresAt    json.RawMessage `json:"expiresAt"`
		IsPermanent  *bool           `json:"isPermanent"`
		Enabled      *bool           `json:"enabled"`
	}
	if c.Request.ContentLength > 0 && !bindJSON(c, &body) {
		return
	}
	adminPatch := store.AdminUserPatch{
		NewUsername:  strings.TrimSpace(body.NewUsername),
		Password:     body.Password,
		AccountLimit: body.AccountLimit,
	}
	if body.IsPermanent != nil {
		adminPatch.IsPermanentSet = true
		adminPatch.IsPermanent = *body.IsPermanent
	}
	if body.Enabled != nil {
		adminPatch.EnabledSet = true
		adminPatch.Enabled = *body.Enabled
	}
	if body.ExpiresAt != nil {
		adminPatch.ExpiresAtSet = true
		if strings.TrimSpace(string(body.ExpiresAt)) != "null" {
			var expiresAt int64
			if err := json.Unmarshal(body.ExpiresAt, &expiresAt); err != nil || expiresAt < 0 {
				c.JSON(400, gin.H{"ok": false, "error": "invalid expiresAt"})
				return
			}
			adminPatch.ExpiresAt = &expiresAt
		}
	}
	if updater, ok := h.app().Users.(AdminUserUpdater); ok {
		updated, err := updater.UpdateAdminUser(c.Request.Context(), username, adminPatch)
		if err != nil {
			writeError(c, err)
			return
		}
		if h.app().Sessions != nil {
			if adminPatch.EnabledSet && !adminPatch.Enabled {
				_ = h.app().Sessions.InvalidateAll(c.Request.Context(), func(session middleware.Session) bool {
					return session.User.Username == username
				})
			} else {
				_ = h.app().Sessions.UpdateSessions(c.Request.Context(), func(session middleware.Session) bool {
					return session.User.Username == username
				}, func(session *middleware.Session) {
					session.User.Username = updated.Username
					session.User.Role = updated.Role
					session.User.Status = updated.Status
					session.User.ExpireAt = updated.ExpireAt
					session.User.AccountLimit = updated.AccountLimit
					session.User.CardCode = updated.CardCode
					session.User.CardJSON = updated.CardJSON
					session.User.MustChangePassword = updated.MustChangePassword
				})
			}
		}
		writeOK(c, publicUser(*updated))
		return
	}
	user, err := h.app().Users.Get(c.Request.Context(), username)
	if err != nil {
		writeError(c, err)
		return
	}
	user.Username = username
	if err := h.app().Users.Update(c.Request.Context(), *user); err != nil {
		writeError(c, err)
		return
	}
	updated, err := h.app().Users.Get(c.Request.Context(), username)
	if err != nil {
		writeError(c, err)
		return
	}
	writeOK(c, publicUser(*updated))
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
	writeOK(c, publicRenewal(user))
}

func (h *Handler) adminUserDelete(c *gin.Context) {
	if h.app().Users == nil {
		writeNotConfigured(c)
		return
	}
	username := strings.TrimSpace(c.Param("username"))
	if current, ok := middleware.CurrentUser(c); ok && strings.EqualFold(current.Username, username) {
		c.JSON(400, gin.H{"ok": false, "error": "不能删除自己的账号"})
		return
	}
	if err := h.app().Users.Delete(c.Request.Context(), username); err != nil {
		writeError(c, err)
		return
	}
	if h.app().Sessions != nil {
		_ = h.app().Sessions.InvalidateAll(c.Request.Context(), func(session middleware.Session) bool {
			return session.User.Username == username
		})
	}
	writeOK(c, nil)
}
