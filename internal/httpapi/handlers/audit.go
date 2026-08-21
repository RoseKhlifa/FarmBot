package handlers

import (
	"net/http"
	"strconv"

	"github.com/RoseKhlifa/FarmBot/internal/audit"
	"github.com/RoseKhlifa/FarmBot/internal/httpapi/middleware"
	"github.com/gin-gonic/gin"
)

func (h *Handler) RegisterAudit(r gin.IRouter) {
	r.GET("/api/admin/audit-logs", h.auditLogs)
	r.GET("/api/admin/audit-logs/export", h.auditExport)
}

func (h *Handler) auditFilter(c *gin.Context) audit.Filter {
	return audit.Filter{Since: parseInt64(c.Query("since")), Until: parseInt64(c.Query("until")), ActorUser: c.Query("actor"), Action: c.Query("action"), TargetAccount: c.Query("accountId"), Limit: queryInt(c, "limit", 100), Offset: queryInt(c, "offset", 0)}
}
func parseInt64(value string) int64 { parsed, _ := strconv.ParseInt(value, 10, 64); return parsed }
func (h *Handler) auditLogs(c *gin.Context) {
	if !isAdmin(c) || h.app().Audit == nil {
		c.JSON(http.StatusForbidden, gin.H{"ok": false, "error": "需要管理员权限"})
		return
	}
	entries, err := h.app().Audit.List(c.Request.Context(), h.auditFilter(c))
	if err != nil {
		writeError(c, err)
		return
	}
	writeOK(c, entries)
}
func (h *Handler) auditExport(c *gin.Context) {
	if !isAdmin(c) || h.app().Audit == nil {
		c.JSON(http.StatusForbidden, gin.H{"ok": false, "error": "需要管理员权限"})
		return
	}
	data, err := h.app().Audit.ExportJSON(c.Request.Context(), h.auditFilter(c))
	if err != nil {
		writeError(c, err)
		return
	}
	c.Header("Content-Disposition", "attachment; filename=audit-log.json")
	c.Data(http.StatusOK, "application/json", data)
}
func isAdmin(c *gin.Context) bool {
	user, ok := middleware.CurrentUser(c)
	return ok && middleware.HasAdminRole(user.Role)
}
