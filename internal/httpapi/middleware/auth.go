package middleware

import (
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/RoseKhlifa/FarmBot/internal/store"
	"github.com/gin-gonic/gin"
)

const (
	CurrentUserKey    = "currentUser"
	CurrentSessionKey = "adminSession"
	CurrentTokenKey   = "adminToken"
)

// ChainConfig describes the infrastructure middleware order. Account and
// role gates are route-specific and should be appended by business handlers.
type ChainConfig struct {
	Sessions       *SessionManager
	CORS           CORSConfig
	RequestTimeout time.Duration
}

// MiddlewareChain returns the fixed order required by the admin server:
// CORS, public-path/token authentication, then request timeout protection.
func MiddlewareChain(cfg ChainConfig) []gin.HandlerFunc {
	return []gin.HandlerFunc{
		CORS(cfg.CORS),
		AuthGate(cfg.Sessions),
		Timeout(cfg.RequestTimeout),
	}
}

var publicAPIPaths = map[string]struct{}{
	"/login":                           {},
	"/qr/create":                       {},
	"/qr/check":                        {},
	"/proxy":                           {},
	"/card-claim/status":               {},
	"/card-claim/claim":                {},
	"/game-version":                    {},
	"/public/login-links":              {},
	"/user-count":                      {},
	"/super-admin-announcement":        {},
	"/super-admin-announcement/verify": {},
	"/changelog":                       {},
	"/public/renew":                    {},
	"/public/reset-password/verify":    {},
	"/public/reset-password/confirm":   {},
	"/health":                          {},
}

// PublicAPIPaths returns the fixed legacy allowlist. The returned slice is a
// copy so callers cannot widen the security boundary by mutation.
func PublicAPIPaths() []string {
	paths := make([]string, 0, len(publicAPIPaths))
	for path := range publicAPIPaths {
		paths = append(paths, path)
	}
	sortStrings(paths)
	return paths
}

// IsPublicAPIPath reports whether an API path bypasses token authentication.
// The capture-certificate prefix is the only wildcard exception in Node.
func IsPublicAPIPath(path string) bool {
	relative, ok := apiPath(path)
	if !ok {
		// The exported helper also accepts the relative paths used by the
		// legacy allowlist; AuthGate itself still only calls it for /api/*.
		path = strings.TrimSpace(path)
		if strings.HasPrefix(path, "/") && !strings.HasPrefix(path, "/api/") {
			relative, ok = path, true
		}
		if !ok {
			return false
		}
	}
	if _, exists := publicAPIPaths[relative]; exists {
		return true
	}
	return strings.HasPrefix(relative, "/public/capture-certificate/")
}

// AuthGate enforces x-admin-token for every /api path not in the fixed public
// allowlist. The authenticated user and session are injected into Gin values.
func AuthGate(sessions *SessionManager) gin.HandlerFunc {
	return func(c *gin.Context) {
		if !isAPIRequest(c.Request.URL.Path) || IsPublicAPIPath(c.Request.URL.Path) {
			c.Next()
			return
		}
		token := strings.TrimSpace(c.GetHeader(AdminTokenHeader))
		if token == "" || sessions == nil {
			unauthorized(c)
			return
		}
		session, err := sessions.Lookup(c.Request.Context(), token)
		if err != nil {
			if errors.Is(err, ErrSessionBanned) {
				c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"ok": false, "error": "账号已被封禁，请联系管理员"})
				return
			}
			if errors.Is(err, ErrSessionExpired) {
				c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"ok": false, "error": "账号已过期，请续费后重新登录"})
				return
			}
			unauthorized(c)
			return
		}
		c.Set(CurrentTokenKey, token)
		c.Set(CurrentSessionKey, session)
		c.Set(CurrentUserKey, session.User)
		c.Next()
	}
}

// RequireAdminToken is a descriptive alias for AuthGate.
func RequireAdminToken(sessions *SessionManager) gin.HandlerFunc { return AuthGate(sessions) }

// CurrentUser extracts the user injected by AuthGate.
func CurrentUser(c *gin.Context) (store.User, bool) {
	if c == nil {
		return store.User{}, false
	}
	value, ok := c.Get(CurrentUserKey)
	if !ok {
		return store.User{}, false
	}
	switch user := value.(type) {
	case store.User:
		return user, true
	case *store.User:
		if user != nil {
			return *user, true
		}
	}
	return store.User{}, false
}

func CurrentToken(c *gin.Context) string {
	if c == nil {
		return ""
	}
	if value, ok := c.Get(CurrentTokenKey); ok {
		if token, ok := value.(string); ok {
			return token
		}
	}
	return strings.TrimSpace(c.GetHeader(AdminTokenHeader))
}

func unauthorized(c *gin.Context) {
	c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"ok": false, "error": "Unauthorized"})
}

func apiPath(path string) (string, bool) {
	path = strings.TrimSpace(path)
	if path == "/api" {
		return "/", true
	}
	if !strings.HasPrefix(path, "/api/") {
		return "", false
	}
	return strings.TrimSuffix(strings.TrimPrefix(path, "/api"), "/"), true
}

func isAPIRequest(path string) bool {
	_, ok := apiPath(path)
	return ok
}

func sortStrings(values []string) {
	for i := 1; i < len(values); i++ {
		for j := i; j > 0 && values[j] < values[j-1]; j-- {
			values[j], values[j-1] = values[j-1], values[j]
		}
	}
}
