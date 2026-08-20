package middleware

import (
	"path"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
)

const (
	// DefaultSecurityHSTSMaxAge is the one-year HSTS policy used for
	// production deployments. HSTS is opt-in so local HTTP development is not
	// accidentally pinned to HTTPS by a browser.
	DefaultSecurityHSTSMaxAge = 31536000

	// DefaultContentSecurityPolicy keeps the SPA's executable and connection
	// sources same-origin while retaining the inline styles used by Vue/UnoCSS
	// and the native WebSocket/SSE schemes used by the realtime client.
	DefaultContentSecurityPolicy = "default-src 'self'; base-uri 'self'; object-src 'none'; frame-ancestors 'none'; script-src 'self'; style-src 'self' 'unsafe-inline'; img-src 'self' data: blob: https:; font-src 'self' data:; connect-src 'self' ws: wss:"

	// LoginAssetSVGContentSecurityPolicy prevents an uploaded SVG from
	// executing active content when it is displayed by the login UI.
	LoginAssetSVGContentSecurityPolicy = "sandbox; default-src 'none'; style-src 'unsafe-inline'; img-src data:"
)

// SecurityHeadersConfig controls the policy emitted by SecurityHeaders.
//
// Production enables HSTS. EnableHSTS can be used by a composition root that
// does not label its deployment as production but still terminates HTTPS. The
// CSP fields are optional overrides; an empty value keeps the safe SPA
// default. HSTSMaxAge is expressed in seconds because that is the wire format
// of Strict-Transport-Security.
type SecurityHeadersConfig struct {
	Production bool
	EnableHSTS bool

	HSTSMaxAge            int
	HSTSIncludeSubDomains bool
	DisableHSTSSubDomains bool
	HSTSPreload           bool

	ContentSecurityPolicy string
	// CSP is a short alias for ContentSecurityPolicy.
	CSP string
}

// SecurityConfig is a concise compatibility name for SecurityHeadersConfig.
type SecurityConfig = SecurityHeadersConfig

// DefaultSecurityHeadersConfig returns the development policy. Production
// callers should set Production (or EnableHSTS) explicitly at the composition
// root rather than making this package inspect process environment variables.
func DefaultSecurityHeadersConfig() SecurityHeadersConfig {
	return SecurityHeadersConfig{
		HSTSMaxAge:            DefaultSecurityHSTSMaxAge,
		HSTSIncludeSubDomains: true,
	}
}

// SecurityHeaders returns a Gin middleware that applies common browser
// security headers to every response. The middleware is deliberately
// composable: callers can append it to the existing infrastructure chain
// without changing authentication, CORS, or realtime handlers.
func SecurityHeaders(config ...SecurityHeadersConfig) gin.HandlerFunc {
	cfg := DefaultSecurityHeadersConfig()
	if len(config) > 0 {
		cfg = normalizeSecurityHeadersConfig(config[0])
	}
	csp := strings.TrimSpace(cfg.ContentSecurityPolicy)
	if csp == "" {
		csp = strings.TrimSpace(cfg.CSP)
	}
	if csp == "" {
		csp = DefaultContentSecurityPolicy
	}

	hsts := ""
	if cfg.Production || cfg.EnableHSTS {
		maxAge := cfg.HSTSMaxAge
		if maxAge <= 0 {
			maxAge = DefaultSecurityHSTSMaxAge
		}
		hsts = "max-age=" + strconv.Itoa(maxAge)
		if cfg.HSTSIncludeSubDomains && !cfg.DisableHSTSSubDomains {
			hsts += "; includeSubDomains"
		}
		if cfg.HSTSPreload {
			hsts += "; preload"
		}
	}

	return func(c *gin.Context) {
		if c == nil || c.Request == nil {
			if c != nil {
				c.Next()
			}
			return
		}

		c.Header("X-Content-Type-Options", "nosniff")
		c.Header("X-Frame-Options", "DENY")
		c.Header("Referrer-Policy", "strict-origin-when-cross-origin")
		c.Header("Content-Security-Policy", csp)
		if hsts != "" {
			c.Header("Strict-Transport-Security", hsts)
		}

		// Login assets are user-provided files. Keep the legacy SVG sandbox
		// policy narrower than the SPA policy, while allowing the handler to
		// serve the file normally after this middleware has run.
		if isLoginAssetSVG(c.Request.URL.Path) {
			c.Header("Content-Security-Policy", LoginAssetSVGContentSecurityPolicy)
		}

		c.Next()
	}
}

// SecureHeaders is a descriptive alias for SecurityHeaders.
func SecureHeaders(config ...SecurityHeadersConfig) gin.HandlerFunc {
	return SecurityHeaders(config...)
}

// SecurityHeadersMiddleware is an explicit alias for composition roots that
// group all policy middleware under a common naming convention.
func SecurityHeadersMiddleware(config ...SecurityHeadersConfig) gin.HandlerFunc {
	return SecurityHeaders(config...)
}

// SecurityMiddleware is a concise alias for SecurityHeaders.
func SecurityMiddleware(config ...SecurityHeadersConfig) gin.HandlerFunc {
	return SecurityHeaders(config...)
}

func normalizeSecurityHeadersConfig(cfg SecurityHeadersConfig) SecurityHeadersConfig {
	if cfg.HSTSMaxAge <= 0 {
		cfg.HSTSMaxAge = DefaultSecurityHSTSMaxAge
	}
	// Subdomains are part of the HSTS default. A dedicated opt-out avoids the
	// usual bool zero-value ambiguity while preserving a convenient config
	// literal for production callers.
	if !cfg.DisableHSTSSubDomains {
		cfg.HSTSIncludeSubDomains = true
	}
	return cfg
}

func isLoginAssetSVG(requestPath string) bool {
	requestPath = strings.TrimSpace(requestPath)
	if requestPath == "" {
		return false
	}
	clean := path.Clean(requestPath)
	if clean != "/login-assets" && !strings.HasPrefix(clean, "/login-assets/") {
		return false
	}
	return strings.EqualFold(path.Ext(path.Base(clean)), ".svg")
}
