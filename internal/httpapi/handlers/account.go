package handlers

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"net/http"
	"strings"

	"github.com/RoseKhlifa/FarmBot/internal/httpapi/middleware"
	"github.com/RoseKhlifa/FarmBot/internal/store"
	"github.com/gin-gonic/gin"
)

func (h *Handler) RegisterAccount(r gin.IRouter) {
	r.GET("/api/accounts", h.accounts)
	r.POST("/api/accounts/refresh-wx-codes", h.refreshWXCodes)
	r.POST("/api/account/remark", h.accountRemark)
	r.POST("/api/accounts", h.accountCreate)
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
	if user, ok := middleware.CurrentUser(c); ok && !middleware.HasAdminRole(user.Role) {
		owned := make([]store.Account, 0, len(data))
		for _, row := range data {
			if strings.EqualFold(strings.TrimSpace(row.OwnerUser), strings.TrimSpace(user.Username)) {
				owned = append(owned, row)
			}
		}
		data = owned
	}
	writeOK(c, data)
}

func (h *Handler) accountCreate(c *gin.Context) {
	creator, ok := h.app().Accounts.(interface {
		Create(context.Context, store.Account) (store.Account, error)
	})
	if !ok {
		writeNotConfigured(c)
		return
	}
	var account store.Account
	if !bindJSON(c, &account) {
		return
	}
	if strings.TrimSpace(account.ID) == "" {
		var random [8]byte
		if _, err := rand.Read(random[:]); err != nil {
			writeError(c, err)
			return
		}
		account.ID = "account-" + hex.EncodeToString(random[:])
	}
	if user, authenticated := middleware.CurrentUser(c); authenticated {
		// Resolve the existing row before applying ownership and quota rules. A
		// normal user may update only their own account, while an admin may edit
		// any account without accidentally transferring ownership when the
		// partial update omits ownerUser.
		var existing *store.Account
		rows, err := h.app().Accounts.List(c.Request.Context())
		if err != nil {
			writeError(c, err)
			return
		} else {
			for i := range rows {
				if strings.EqualFold(strings.TrimSpace(rows[i].ID), strings.TrimSpace(account.ID)) {
					existing = &rows[i]
					break
				}
			}
		}
		if middleware.HasAdminRole(user.Role) {
			if strings.TrimSpace(account.OwnerUser) == "" && existing != nil {
				account.OwnerUser = existing.OwnerUser
			}
			if strings.TrimSpace(account.OwnerUser) == "" {
				account.OwnerUser = user.Username
			}
		} else {
			if existing != nil && !strings.EqualFold(strings.TrimSpace(existing.OwnerUser), strings.TrimSpace(user.Username)) {
				c.JSON(http.StatusForbidden, gin.H{"ok": false, "error": "无权访问此账号"})
				return
			}
			if existing == nil && user.AccountLimit > 0 {
				owned := 0
				for _, row := range rows {
					if strings.EqualFold(strings.TrimSpace(row.OwnerUser), strings.TrimSpace(user.Username)) {
						owned++
					}
				}
				if owned >= user.AccountLimit {
					c.JSON(http.StatusForbidden, gin.H{"ok": false, "error": "账号数量已达到配额上限"})
					return
				}
			}
			account.OwnerUser = user.Username
		}
	}
	created, err := creator.Create(c.Request.Context(), account)
	if err != nil {
		writeError(c, err)
		return
	}
	writeOK(c, created)
}

func (h *Handler) refreshWXCodes(c *gin.Context) {
	if h.app().Accounts == nil {
		writeNotConfigured(c)
		return
	}
	accounts, err := h.app().Accounts.List(c.Request.Context())
	if err != nil {
		writeError(c, err)
		return
	}
	if user, ok := middleware.CurrentUser(c); ok && !middleware.HasAdminRole(user.Role) {
		owned := make([]store.Account, 0, len(accounts))
		for _, row := range accounts {
			if strings.EqualFold(strings.TrimSpace(row.OwnerUser), strings.TrimSpace(user.Username)) {
				owned = append(owned, row)
			}
		}
		accounts = owned
	}
	restarter, canRestart := h.app().Accounts.(interface {
		Restart(context.Context, string) error
	})
	eligible := 0
	refreshed := 0
	failed := make([]gin.H, 0)
	for _, account := range accounts {
		if strings.EqualFold(account.LoginType, "yyb") || account.YYBOpenID != "" {
			eligible++
			if canRestart {
				if err := restarter.Restart(c.Request.Context(), account.ID); err != nil {
					failed = append(failed, gin.H{"accountId": account.ID, "error": err.Error()})
				} else {
					refreshed++
				}
			}
		}
	}
	if !canRestart {
		writeNotConfigured(c)
		return
	}
	writeOK(c, gin.H{"accounts": accounts, "eligible": eligible, "refreshed": refreshed, "failed": failed})
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
	if !h.requireAccountOwner(c, body.AccountID) {
		return
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
	if !h.requireAccountOwner(c, id) {
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
	if !h.requireAccountOwner(c, c.Param("id")) {
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
	if !h.requireAccountOwner(c, c.Param("id")) {
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
	if user, authenticated := middleware.CurrentUser(c); authenticated && !middleware.HasAdminRole(user.Role) {
		if id == "" || !h.requireAccountOwner(c, id) {
			return
		}
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
	if user, authenticated := middleware.CurrentUser(c); authenticated && !middleware.HasAdminRole(user.Role) {
		if id == "" || !h.requireAccountOwner(c, id) {
			return
		}
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
	if !h.requireAccountOwner(c, id) {
		return
	}
	if err := h.app().Logs.ClearLogs(c.Request.Context(), id); err != nil {
		writeError(c, err)
		return
	}
	writeOK(c, nil)
}
