package handlers

import (
	"net/http"
	"strings"

	"github.com/RoseKhlifa/FarmBot/internal/httpapi/middleware"
	"github.com/gin-gonic/gin"
)

func (h *Handler) RegisterExport(r gin.IRouter) {
	r.GET("/api/admin/accounts/:accountId/export", h.exportAccount)
}

func (h *Handler) exportAccount(c *gin.Context) {
	if h.app().ExportAccount == nil {
		writeNotConfigured(c)
		return
	}
	id := strings.TrimSpace(c.Param("accountId"))
	if id == "" {
		c.JSON(http.StatusBadRequest, gin.H{"ok": false, "error": "accountId is required"})
		return
	}
	user, ok := middleware.CurrentUser(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"ok": false, "error": "Unauthorized"})
		return
	}
	if h.app().Accounts != nil && !middleware.HasAdminRole(user.Role) {
		accounts, err := h.app().Accounts.List(c.Request.Context())
		if err != nil {
			writeError(c, err)
			return
		}
		allowed := false
		for _, row := range accounts {
			if row.ID == id && row.OwnerUser == user.Username {
				allowed = true
				break
			}
		}
		if !allowed {
			c.JSON(http.StatusForbidden, gin.H{"ok": false, "error": "无权导出此账号"})
			return
		}
	}
	data, err := h.app().ExportAccount(c.Request.Context(), id)
	if err != nil {
		writeError(c, err)
		return
	}
	c.Header("Content-Disposition", "attachment; filename=farmbot-account-"+id+".json")
	c.Data(http.StatusOK, "application/json", data)
}
