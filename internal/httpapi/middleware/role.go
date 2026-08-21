package middleware

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

// RequireAdminRole allows both admin and super_admin users.
func RequireAdminRole(c *gin.Context) {
	user, ok := CurrentUser(c)
	if !ok || !isElevatedRole(user.Role) {
		c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"ok": false, "error": "需要管理员权限"})
		return
	}
	c.Next()
}

// RequireSuperAdminRole allows only the super administrator role.
func RequireSuperAdminRole(c *gin.Context) {
	user, ok := CurrentUser(c)
	if !ok || !isSuperAdminRole(user.Role) {
		c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"ok": false, "error": "需要超级管理员权限"})
		return
	}
	c.Next()
}

func AdminRole() gin.HandlerFunc { return RequireAdminRole }

func SuperAdminRole() gin.HandlerFunc { return RequireSuperAdminRole }

// RequireAdminAPI protects every /api/admin endpoint when installed on the
// application router. Keeping this gate at the prefix prevents a newly added
// administrative route from accidentally becoming available to normal users.
func RequireAdminAPI() gin.HandlerFunc {
	return func(c *gin.Context) {
		if c == nil || c.Request == nil {
			return
		}
		path := strings.TrimSuffix(c.Request.URL.Path, "/")
		if path == "/api/admin" || strings.HasPrefix(path, "/api/admin/") {
			RequireAdminRole(c)
			return
		}
		c.Next()
	}
}

// RequireSuperAdminAPI applies the equivalent prefix gate to platform-wide
// operations under /api/super-admin.
func RequireSuperAdminAPI() gin.HandlerFunc {
	return func(c *gin.Context) {
		if c == nil || c.Request == nil {
			return
		}
		path := strings.TrimSuffix(c.Request.URL.Path, "/")
		if path == "/api/super-admin" || strings.HasPrefix(path, "/api/super-admin/") {
			RequireSuperAdminRole(c)
			return
		}
		c.Next()
	}
}

func HasAdminRole(role string) bool { return isElevatedRole(role) }

func HasSuperAdminRole(role string) bool { return isSuperAdminRole(role) }

func isElevatedRole(role string) bool {
	role = strings.ToLower(strings.TrimSpace(strings.ReplaceAll(role, "-", "_")))
	return role == "admin" || role == "super_admin"
}

func isSuperAdminRole(role string) bool {
	role = strings.ToLower(strings.TrimSpace(strings.ReplaceAll(role, "-", "_")))
	return role == "super_admin"
}
