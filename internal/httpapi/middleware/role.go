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
