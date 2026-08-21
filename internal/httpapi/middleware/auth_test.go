package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/RoseKhlifa/FarmBot/internal/store"
	"github.com/gin-gonic/gin"
)

func TestPublicAuthPathsIncludeRegistrationAndCardInfo(t *testing.T) {
	for _, path := range []string{"/api/login", "/api/register", "/api/card/info/ABC123", "/api/public/renew"} {
		if !IsPublicAPIPath(path) {
			t.Fatalf("%s should bypass token auth", path)
		}
	}
	for _, path := range []string{"/api/accounts", "/api/user/renew", "/api/admin/users"} {
		if IsPublicAPIPath(path) {
			t.Fatalf("%s should require token auth", path)
		}
	}
}

func TestAdministrativePrefixGates(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(RequireAdminAPI(), RequireSuperAdminAPI())
	router.GET("/api/admin/users", func(c *gin.Context) { c.Status(http.StatusNoContent) })
	router.GET("/api/super-admin/config", func(c *gin.Context) { c.Status(http.StatusNoContent) })

	request := httptest.NewRequest(http.MethodGet, "/api/admin/users", nil)
	request.Header.Set(AdminTokenHeader, "test")
	response := httptest.NewRecorder()
	routerWithUser := gin.New()
	routerWithUser.Use(func(c *gin.Context) {
		c.Set(CurrentUserKey, store.User{Username: "user", Role: "user"})
		c.Next()
	}, RequireAdminAPI())
	routerWithUser.GET("/api/admin/users", func(c *gin.Context) { c.Status(http.StatusNoContent) })
	routerWithUser.ServeHTTP(response, request)
	if response.Code != http.StatusForbidden {
		t.Fatalf("normal user admin status=%d, want 403", response.Code)
	}

	response = httptest.NewRecorder()
	routerWithUser = gin.New()
	routerWithUser.Use(func(c *gin.Context) {
		c.Set(CurrentUserKey, store.User{Username: "admin", Role: "admin"})
		c.Next()
	}, RequireAdminAPI(), RequireSuperAdminAPI())
	routerWithUser.GET("/api/admin/users", func(c *gin.Context) { c.Status(http.StatusNoContent) })
	routerWithUser.ServeHTTP(response, request)
	if response.Code != http.StatusNoContent {
		t.Fatalf("admin user admin status=%d, want 204", response.Code)
	}

	response = httptest.NewRecorder()
	routerWithUser = gin.New()
	routerWithUser.Use(func(c *gin.Context) {
		c.Set(CurrentUserKey, store.User{Username: "admin", Role: "admin"})
		c.Next()
	}, RequireAdminAPI(), RequireSuperAdminAPI())
	routerWithUser.GET("/api/super-admin/config", func(c *gin.Context) { c.Status(http.StatusNoContent) })
	routerWithUser.ServeHTTP(response, request)
	// The request path above is intentionally changed to the super-admin route
	// so the ordinary admin role is rejected by the second prefix gate.
	request = httptest.NewRequest(http.MethodGet, "/api/super-admin/config", nil)
	response = httptest.NewRecorder()
	routerWithUser.ServeHTTP(response, request)
	if response.Code != http.StatusForbidden {
		t.Fatalf("admin user super-admin status=%d, want 403", response.Code)
	}
}
