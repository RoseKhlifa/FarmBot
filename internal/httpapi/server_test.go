package httpapi

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/RoseKhlifa/FarmBot/internal/config"
)

func TestHealthReadyAndDrain(t *testing.T) {
	server, err := NewServer(config.Config{AdminPort: 3007}, ServerOptions{Ready: func(context.Context) error { return nil }})
	if err != nil {
		t.Fatal(err)
	}
	request := httptest.NewRequest(http.MethodGet, "/api/health", nil)
	response := httptest.NewRecorder()
	server.Handler().ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("health status=%d", response.Code)
	}
	request = httptest.NewRequest(http.MethodGet, "/api/ready", nil)
	response = httptest.NewRecorder()
	server.Handler().ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("ready status=%d", response.Code)
	}
	server.BeginDrain()
	request = httptest.NewRequest(http.MethodGet, "/api/ready", nil)
	response = httptest.NewRecorder()
	server.Handler().ServeHTTP(response, request)
	if response.Code != http.StatusServiceUnavailable {
		t.Fatalf("draining ready status=%d", response.Code)
	}
	request = httptest.NewRequest(http.MethodGet, "/api/health", nil)
	response = httptest.NewRecorder()
	server.Handler().ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("draining health status=%d", response.Code)
	}
}
