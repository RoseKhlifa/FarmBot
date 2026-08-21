package handlers

import (
	"context"
	"net/http"
	"strings"
	"time"

	"github.com/RoseKhlifa/FarmBot/internal/httpapi/middleware"
	platformmetrics "github.com/RoseKhlifa/FarmBot/internal/platform/metrics"
	"github.com/RoseKhlifa/FarmBot/internal/store"
	"github.com/gin-gonic/gin"
)

const metricsContentType = "text/plain; version=0.0.4; charset=utf-8"

// MetricsRouteConfig contains the two dependencies needed by /metrics. The
// route intentionally does not use the normal /api AuthGate because /metrics
// lives outside /api and must remain protected independently.
type MetricsRouteConfig struct {
	Registry *platformmetrics.Registry
	Sessions *middleware.SessionManager
}

// MetricsHandler serves the Prometheus text exposition for one application.
type MetricsHandler struct {
	Registry *platformmetrics.Registry
	Sessions *middleware.SessionManager
}

// NewMetricsHandler creates a protected metrics handler.
func NewMetricsHandler(cfg MetricsRouteConfig) *MetricsHandler {
	return &MetricsHandler{Registry: cfg.Registry, Sessions: cfg.Sessions}
}

// RegisterMetricsRoutes installs the independent, protected /metrics route.
// The returned handler can be retained by a composition root for diagnostics.
func RegisterMetricsRoutes(router gin.IRouter, cfg MetricsRouteConfig) *MetricsHandler {
	handler := NewMetricsHandler(cfg)
	if router != nil {
		router.GET("/metrics", handler.ServeHTTP)
	}
	return handler
}

// RegisterMetrics is a convenience alias for composition roots.
func RegisterMetrics(router gin.IRouter, registry *platformmetrics.Registry, sessions *middleware.SessionManager) *MetricsHandler {
	return RegisterMetricsRoutes(router, MetricsRouteConfig{Registry: registry, Sessions: sessions})
}

// MetricsHTTPMiddleware records each Gin request in the registry's fixed
// histogram. A normalized route template is used when Gin has one, avoiding
// unbounded account IDs and other path parameters in Prometheus labels.
func MetricsHTTPMiddleware(registry *platformmetrics.Registry) gin.HandlerFunc {
	return func(c *gin.Context) {
		if registry == nil {
			c.Next()
			return
		}
		started := time.Now()
		c.Next()
		route := c.FullPath()
		if route == "" {
			route = "unmatched"
		}
		registry.ObserveHTTP(c.Request.Method, route, time.Since(started))
	}
}

type metricsAuthProvider struct {
	next     AuthProvider
	registry *platformmetrics.Registry
}

// InstrumentAuth decorates the existing authentication interface so login
// outcomes update the metrics registry without duplicating authentication.
func InstrumentAuth(next AuthProvider, registry *platformmetrics.Registry) AuthProvider {
	if next == nil || registry == nil {
		return next
	}
	return metricsAuthProvider{next: next, registry: registry}
}

func (p metricsAuthProvider) Login(ctx context.Context, username, password, ip string) (store.User, error) {
	user, err := p.next.Login(ctx, username, password, ip)
	if err != nil {
		p.registry.RecordLoginFailure()
		return user, err
	}
	p.registry.RecordLoginSuccess()
	return user, nil
}

func (p metricsAuthProvider) Register(ctx context.Context, username, password string) (store.User, error) {
	return p.next.Register(ctx, username, password)
}

// RegisterRoutes installs this handler on a Gin router.
func (h *MetricsHandler) RegisterRoutes(router gin.IRouter) {
	if h != nil && router != nil {
		router.GET("/metrics", h.ServeHTTP)
	}
}

// ServeHTTP authenticates an administrator using x-admin-token and writes
// Prometheus text. Ordinary user sessions are rejected even when their token
// is otherwise valid.
func (h *MetricsHandler) ServeHTTP(c *gin.Context) {
	if h == nil {
		c.String(http.StatusServiceUnavailable, "metrics are not configured\n")
		return
	}
	if h.Sessions == nil {
		writeMetricsUnauthorized(c)
		return
	}
	token := strings.TrimSpace(c.GetHeader(middleware.AdminTokenHeader))
	if token == "" {
		writeMetricsUnauthorized(c)
		return
	}
	session, err := h.Sessions.Lookup(c.Request.Context(), token)
	if err != nil {
		writeMetricsUnauthorized(c)
		return
	}
	if !middleware.HasAdminRole(session.User.Role) {
		c.String(http.StatusForbidden, "metrics require an administrator session\n")
		return
	}
	if h.Registry == nil {
		c.String(http.StatusServiceUnavailable, "metrics are not configured\n")
		return
	}
	payload, err := h.Registry.Render(c.Request.Context())
	if err != nil {
		c.String(http.StatusInternalServerError, "metrics collection failed: %s\n", err.Error())
		return
	}
	c.Data(http.StatusOK, metricsContentType, payload)
}

func writeMetricsUnauthorized(c *gin.Context) {
	c.Header("WWW-Authenticate", "x-admin-token")
	c.String(http.StatusUnauthorized, "metrics require an administrator token\n")
}
