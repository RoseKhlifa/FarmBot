package handlers

import (
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/RoseKhlifa/FarmBot/internal/account"
	"github.com/RoseKhlifa/FarmBot/internal/domain/farm"
	"github.com/RoseKhlifa/FarmBot/internal/domain/friend"
	"github.com/RoseKhlifa/FarmBot/internal/store"
	"github.com/gin-gonic/gin"
)

type missingDeviceProtocolConfig struct{ store.ConfigRepo }

func (missingDeviceProtocolConfig) GetGlobal(context.Context, string) (json.RawMessage, error) {
	return nil, sql.ErrNoRows
}

func TestRegisterRoutesContractSnapshot(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	RegisterRoutes(router, &Application{})
	routes := make(map[string]map[string]bool)
	for _, route := range router.Routes() {
		if routes[route.Path] == nil {
			routes[route.Path] = map[string]bool{}
		}
		routes[route.Path][route.Method] = true
	}
	expected := map[string]string{
		"/api/accounts": "GET", "/api/accounts/:id/start": "POST", "/api/accounts/:id/stop": "POST",
		"/api/friends": "GET", "/api/friend/:gid/lands": "GET", "/api/friend/:gid/op": "POST", "/api/friend-blacklist": "GET",
		"/api/status": "GET", "/api/lands": "GET", "/api/farm/operate": "POST", "/api/land/fertilize": "POST",
		"/api/bag": "GET", "/api/bag/use": "POST", "/api/bag/sell": "POST", "/api/shop/mall": "GET", "/api/shop/mall/buy": "POST",
		"/api/tasks": "GET", "/api/tasks/claim": "POST", "/api/activity/list": "GET", "/api/activity/group/:id": "GET",
		"/api/illustrated": "GET", "/api/career": "GET", "/api/analytics": "GET", "/api/settings": "GET",
		"/api/login": "POST", "/api/logout": "POST", "/api/user/me": "GET", "/api/admin/cards": "GET", "/api/admin/login-logs": "GET",
		"/api/yyb/getcode": "POST", "/api/capture/sessions": "POST", "/api/qr/create": "POST", "/api/proxy": "POST",
		"/api/ping": "GET", "/api/game-version": "GET", "/api/shop/seed": "GET", "/api/shop/pet": "GET",
		"/api/shop/decoration": "GET", "/api/shop/mystery": "GET",
		"/api/admin/capture-config": "GET", "/api/admin/capture-config/test": "POST",
		"/api/capture/config": "GET", "/api/capture/sessions/:flowId": "GET",
		"/api/public/capture-certificate/:flowId/:token": "GET", "/api/capture/sessions/:flowId/complete": "POST",
		"/api/yyb/accounts": "POST", "/api/yyb/thirdparty-code": "POST",
		"/api/yyb/qr/create": "POST", "/api/yyb/qr/poll": "POST", "/api/yyb/qr/confirm": "POST",
		"/api/admin/login-links": "GET", "/api/admin/login-links/reset": "POST",
		"/api/admin/system-config": "GET", "/api/admin/system-config/reset": "POST",
		"/api/admin/users": "GET", "/api/admin/users-with-password": "GET", "/api/admin/users/clear-expired": "POST",
		"/api/admin/users/:username": "POST", "/api/admin/users/:username/edit": "POST", "/api/admin/users/:username/renew": "POST",
		"/api/admin/wx-config": "GET", "/api/admin/announcement": "POST", "/api/admin/login-logo": "POST",
		"/api/admin/cards/:code": "POST", "/api/admin/card-claim/status": "POST",
		"/api/super-admin/announcement": "POST", "/api/super-admin/anti-resale-config": "GET",
		"/api/super-admin/check-account-limit": "POST", "/api/super-admin/clear-data": "POST", "/api/user/wxlogin-config": "POST",
		"/api/debug/item-config": "GET",
	}
	for path, method := range expected {
		if !routes[path][method] {
			t.Errorf("missing route %s %s", method, path)
		}
	}
	for _, route := range []struct{ path, method string }{
		{path: "/api/admin/capture-config", method: "POST"},
		{path: "/api/capture/sessions/:flowId", method: "DELETE"},
		{path: "/api/yyb/getcode", method: "POST"},
		{path: "/api/admin/login-links", method: "POST"},
		{path: "/api/admin/system-config", method: "POST"},
		{path: "/api/admin/users/:username", method: "DELETE"},
		{path: "/api/admin/wx-config", method: "POST"},
		{path: "/api/super-admin/anti-resale-config", method: "POST"},
	} {
		if !routes[route.path][route.method] {
			t.Errorf("missing route %s %s", route.method, route.path)
		}
	}
}

func TestAccountRouteRequiresHeader(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	New(&Application{}).RegisterFriend(router)
	req := httptest.NewRequest(http.MethodGet, "/api/friends", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
}

func TestDomainResolverIsApplicationBoundary(t *testing.T) {
	called := false
	app := &Application{Domains: DomainProviders{Friend: func(context.Context, string) (friend.Service, error) {
		called = true
		return nil, ErrApplicationDependency
	}}}
	gin.SetMode(gin.TestMode)
	router := gin.New()
	New(app).RegisterFriend(router)
	req := httptest.NewRequest(http.MethodGet, "/api/friends", nil)
	req.Header.Set("x-account-id", "account-1")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if !called {
		t.Fatal("friend handler bypassed Application domain resolver")
	}
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500", rec.Code)
	}
}

func TestSeedListReturnsEmptySnapshotWhenAccountOffline(t *testing.T) {
	app := &Application{Domains: DomainProviders{Farm: func(context.Context, string) (farm.Service, error) {
		return nil, account.ErrAccountOffline
	}}}
	gin.SetMode(gin.TestMode)
	router := gin.New()
	New(app).RegisterSeedShop(router)
	req := httptest.NewRequest(http.MethodGet, "/api/seeds", nil)
	req.Header.Set("x-account-id", "account-1")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", rec.Code, rec.Body.String())
	}
	var body struct {
		OK   bool  `json:"ok"`
		Data []any `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if !body.OK || len(body.Data) != 0 {
		t.Fatalf("body = %s, want empty seed snapshot", rec.Body.String())
	}
}

func TestDeviceProtocolReturnsDefaultsWhenUnset(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	New(&Application{Config: missingDeviceProtocolConfig{}}).RegisterUser(router)
	req := httptest.NewRequest(http.MethodGet, "/api/user/device-protocol", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", rec.Code, rec.Body.String())
	}
	var body struct {
		OK   bool           `json:"ok"`
		Data map[string]any `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if !body.OK || body.Data == nil {
		t.Fatalf("body = %s, want default config object", rec.Body.String())
	}
}
