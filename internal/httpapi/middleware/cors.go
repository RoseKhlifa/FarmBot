package middleware

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
)

const (
	AdminTokenHeader   = "x-admin-token"
	AccountIDHeader    = "x-account-id"
	ProxyAPIKeyHeader  = "x-proxy-api-key"
	ProxyAPIURLHeader  = "x-proxy-api-url"
	ProxyAppIDHeader   = "x-proxy-app-id"
	DefaultAllowMethod = "GET, POST, DELETE, OPTIONS, PUT"
	DefaultAllowHeader = "Content-Type, x-account-id, x-admin-token, x-proxy-api-key, x-proxy-api-url, x-proxy-app-id"
)

var defaultAllowedOrigins = []string{
	"http://localhost:5173",
	"http://localhost:3000",
	"http://127.0.0.1:5173",
}

// CORSConfig contains the browser-facing CORS policy. An empty origin list
// uses the same local development origins as the legacy admin server.
type CORSConfig struct {
	AllowedOrigins   []string
	AllowMethods     string
	AllowHeaders     string
	AllowCredentials bool
	MaxAge           int
}

func defaultCORSConfig() CORSConfig {
	return CORSConfig{
		AllowedOrigins:   append([]string(nil), defaultAllowedOrigins...),
		AllowMethods:     DefaultAllowMethod,
		AllowHeaders:     DefaultAllowHeader,
		AllowCredentials: true,
		MaxAge:           86400,
	}
}

// CORS returns a Gin middleware implementing the legacy admin CORS contract.
// It accepts an optional config so the composition root can inject deployment
// origins without making this package read process environment variables.
func CORS(config ...CORSConfig) gin.HandlerFunc {
	cfg := defaultCORSConfig()
	if len(config) > 0 {
		cfg = normalizeCORSConfig(config[0])
	}
	origins := make(map[string]struct{}, len(cfg.AllowedOrigins))
	for _, origin := range cfg.AllowedOrigins {
		if value := strings.TrimSpace(origin); value != "" {
			origins[value] = struct{}{}
		}
	}

	return func(c *gin.Context) {
		origin := strings.TrimSpace(c.GetHeader("Origin"))
		if origin == "" {
			c.Header("Access-Control-Allow-Origin", "*")
		} else if _, ok := origins[origin]; ok {
			c.Header("Access-Control-Allow-Origin", origin)
		}
		c.Header("Access-Control-Allow-Methods", cfg.AllowMethods)
		c.Header("Access-Control-Allow-Headers", cfg.AllowHeaders)
		if cfg.AllowCredentials {
			c.Header("Access-Control-Allow-Credentials", "true")
		}
		if cfg.MaxAge > 0 {
			c.Header("Access-Control-Max-Age", strconv.Itoa(cfg.MaxAge))
		}
		if origin != "" {
			c.Header("Vary", "Origin")
		}
		if c.Request.Method == http.MethodOptions {
			c.AbortWithStatus(http.StatusOK)
			return
		}
		c.Next()
	}
}

// CORSMiddleware is a descriptive alias for CORS.
func CORSMiddleware(config ...CORSConfig) gin.HandlerFunc { return CORS(config...) }

func normalizeCORSConfig(cfg CORSConfig) CORSConfig {
	if len(cfg.AllowedOrigins) == 0 {
		cfg.AllowedOrigins = append([]string(nil), defaultAllowedOrigins...)
	}
	if strings.TrimSpace(cfg.AllowMethods) == "" {
		cfg.AllowMethods = DefaultAllowMethod
	}
	if strings.TrimSpace(cfg.AllowHeaders) == "" {
		cfg.AllowHeaders = DefaultAllowHeader
	}
	// The legacy admin server always enabled credentials and advertised a
	// one-day preflight cache. Keep those defaults when callers only override
	// AllowedOrigins.
	cfg.AllowCredentials = true
	if cfg.MaxAge <= 0 {
		cfg.MaxAge = 86400
	}
	return cfg
}
