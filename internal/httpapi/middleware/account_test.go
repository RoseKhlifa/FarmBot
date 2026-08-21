package middleware

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"testing"

	"github.com/RoseKhlifa/FarmBot/internal/store"
	"github.com/gin-gonic/gin"
)

type accountRepoFake struct{ rows []store.Account }

func (f accountRepoFake) List(context.Context) ([]store.Account, error) {
	return append([]store.Account(nil), f.rows...), nil
}
func (f accountRepoFake) Get(_ context.Context, id string) (*store.Account, error) {
	for _, row := range f.rows {
		if row.ID == id {
			copy := row
			return &copy, nil
		}
	}
	return nil, context.Canceled
}
func (f accountRepoFake) Upsert(context.Context, store.Account) error { return nil }
func (f accountRepoFake) Delete(context.Context, string) error        { return nil }
func (f accountRepoFake) GetByUser(_ context.Context, username string) ([]store.Account, error) {
	result := []store.Account{}
	for _, row := range f.rows {
		if row.OwnerUser == username {
			result = append(result, row)
		}
	}
	return result, nil
}
func (f accountRepoFake) GetConfig(context.Context, string) (*store.AccountConfig, error) {
	return &store.AccountConfig{}, nil
}
func (f accountRepoFake) ApplyConfigSnapshot(context.Context, string, store.AccountConfig) error {
	return nil
}

func TestPublicAllowlistAndWildcard(t *testing.T) {
	for _, path := range []string{"/api/login", "/api/public/reset-password/verify", "/api/public/capture-certificate/x/y"} {
		if !IsPublicAPIPath(path) {
			t.Fatalf("%s should be public", path)
		}
	}
	if IsPublicAPIPath("/api/accounts") {
		t.Fatal("account route must require auth")
	}
}

func TestRequireAccountAccessSemantics(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(RequireAccountAccess(AccountAccessConfig{Repo: accountRepoFake{rows: []store.Account{{ID: "a1", OwnerUser: "u1"}}}}))
	r.GET("/", func(c *gin.Context) { c.Status(204) })
	request := httptest.NewRequest("GET", "/", nil)
	response := httptest.NewRecorder()
	r.ServeHTTP(response, request)
	if response.Code != 400 {
		t.Fatalf("missing account header status=%d", response.Code)
	}

	r = gin.New()
	r.Use(AccountAccess(AccountAccessConfig{Repo: accountRepoFake{rows: []store.Account{{ID: "a1", OwnerUser: "u1"}}}}))
	r.GET("/", func(c *gin.Context) { c.Status(204) })
	request = httptest.NewRequest("GET", "/", nil)
	request.Header.Set(AccountIDHeader, "a1")
	response = httptest.NewRecorder()
	r.ServeHTTP(response, request)
	if response.Code != 401 {
		t.Fatalf("unauthenticated account status=%d", response.Code)
	}

	r = gin.New()
	r.Use(func(c *gin.Context) { c.Set(CurrentUserKey, store.User{Username: "u2", Role: "user"}); c.Next() })
	r.Use(AccountAccess(AccountAccessConfig{Repo: accountRepoFake{rows: []store.Account{{ID: "a1", OwnerUser: "u1"}}}}))
	r.GET("/", func(c *gin.Context) { c.Status(204) })
	request = httptest.NewRequest("GET", "/", nil)
	request.Header.Set(AccountIDHeader, "a1")
	response = httptest.NewRecorder()
	r.ServeHTTP(response, request)
	if response.Code != 403 {
		t.Fatalf("forbidden account status=%d", response.Code)
	}
}

func TestAdminAccountAccessAndCanonicalReference(t *testing.T) {
	if got := ResolveAccountReference([]store.Account{{ID: "a1", UIN: "100"}}, "100"); got != "a1" {
		t.Fatalf("canonical id=%q", got)
	}
	r := gin.New()
	r.Use(func(c *gin.Context) { c.Set(CurrentUserKey, store.User{Username: "admin", Role: "admin"}); c.Next() })
	r.Use(AccountAccess(AccountAccessConfig{Repo: accountRepoFake{rows: []store.Account{{ID: "a1"}}}}))
	r.GET("/", func(c *gin.Context) { c.JSON(200, gin.H{"id": AccountID(c)}) })
	request := httptest.NewRequest("GET", "/", nil)
	request.Header.Set(AccountIDHeader, "a1")
	response := httptest.NewRecorder()
	r.ServeHTTP(response, request)
	if response.Code != 200 {
		t.Fatalf("admin account status=%d", response.Code)
	}
	var body map[string]any
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil || body["id"] != "a1" {
		t.Fatalf("body=%s", response.Body.String())
	}
}
