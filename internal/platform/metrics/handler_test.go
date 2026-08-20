package metrics_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/RoseKhlifa/FarmBot/internal/httpapi/handlers"
	"github.com/RoseKhlifa/FarmBot/internal/httpapi/middleware"
	"github.com/RoseKhlifa/FarmBot/internal/platform/metrics"
	"github.com/RoseKhlifa/FarmBot/internal/store"
	"github.com/gin-gonic/gin"
)

func TestMetricsRouteRequiresAdministratorSession(t *testing.T) {
	gin.SetMode(gin.TestMode)
	sessions, err := middleware.NewSessionManager(middleware.SessionManagerConfig{})
	if err != nil {
		t.Fatal(err)
	}
	defer sessions.Close()
	userToken, err := sessions.Create(context.Background(), store.User{Username: "user", Role: "user", Status: "active"})
	if err != nil {
		t.Fatal(err)
	}
	adminToken, err := sessions.Create(context.Background(), store.User{Username: "admin", Role: "admin", Status: "active"})
	if err != nil {
		t.Fatal(err)
	}

	router := gin.New()
	handlers.RegisterMetricsRoutes(router, handlers.MetricsRouteConfig{Registry: metrics.New(metrics.Config{}), Sessions: sessions})

	for name, test := range map[string]struct {
		token  string
		status int
	}{
		"missing": {status: http.StatusUnauthorized},
		"user":    {token: userToken, status: http.StatusForbidden},
		"admin":   {token: adminToken, status: http.StatusOK},
	} {
		t.Run(name, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodGet, "/metrics", nil)
			if test.token != "" {
				request.Header.Set(middleware.AdminTokenHeader, test.token)
			}
			response := httptest.NewRecorder()
			router.ServeHTTP(response, request)
			if response.Code != test.status {
				t.Fatalf("status = %d, want %d; body=%q", response.Code, test.status, response.Body.String())
			}
			if name == "admin" && response.Header().Get("Content-Type") != "text/plain; version=0.0.4; charset=utf-8" {
				t.Fatalf("unexpected Content-Type %q", response.Header().Get("Content-Type"))
			}
		})
	}
}
