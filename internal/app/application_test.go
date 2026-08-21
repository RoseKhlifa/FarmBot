package app

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/RoseKhlifa/FarmBot/internal/config"
	secretcrypto "github.com/RoseKhlifa/FarmBot/internal/crypto"
	"github.com/RoseKhlifa/FarmBot/internal/httpapi/middleware"
	"github.com/RoseKhlifa/FarmBot/internal/store"
)

func TestNewApplicationWiresAndClosesIdempotently(t *testing.T) {
	application, err := New(testApplicationConfig(t))
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	if application.DB == nil || application.Accounts == nil || application.Users == nil || application.Cards == nil {
		t.Fatal("application repositories were not wired")
	}
	if application.Yyb == nil || application.AccountManager == nil || application.Sessions == nil || application.Realtime == nil || application.Metrics == nil || application.Server == nil {
		t.Fatal("application services were not wired")
	}

	if err := application.Shutdown(context.Background()); err != nil {
		t.Fatalf("first Shutdown() error = %v", err)
	}
	if err := application.Shutdown(context.Background()); err != nil {
		t.Fatalf("second Shutdown() error = %v", err)
	}
	if err := application.Run(context.Background()); err != ErrApplicationClosed {
		t.Fatalf("Run() after Shutdown() error = %v, want %v", err, ErrApplicationClosed)
	}
}

func TestNewApplicationRequiresMasterKey(t *testing.T) {
	cfg := testApplicationConfig(t)
	cfg.MasterKey = ""
	if _, err := New(cfg); !errors.Is(err, secretcrypto.ErrMasterKeyMissing) {
		t.Fatalf("New() without FARM_MASTER_KEY error = %v, want %v", err, secretcrypto.ErrMasterKeyMissing)
	}
	cfg.MasterKey = "invalid-master-key"
	if _, err := New(cfg); !errors.Is(err, secretcrypto.ErrMasterKeyInvalid) {
		t.Fatalf("New() with invalid FARM_MASTER_KEY error = %v, want %v", err, secretcrypto.ErrMasterKeyInvalid)
	}
}

func TestApplicationReturnsEmptyWXConfigWhenUnset(t *testing.T) {
	application, err := New(testApplicationConfig(t))
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	defer func() { _ = application.Shutdown(context.Background()) }()

	admin, err := application.Users.Get(context.Background(), "admin")
	if err != nil {
		t.Fatalf("get admin: %v", err)
	}
	token, err := application.Sessions.Create(context.Background(), *admin)
	if err != nil {
		t.Fatalf("create session: %v", err)
	}
	request := httptest.NewRequest(http.MethodGet, "/api/admin/wx-config", nil)
	request.Header.Set("x-admin-token", token)
	response := httptest.NewRecorder()
	application.Server.Handler().ServeHTTP(response, request)
	if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), `"ok":true`) || !strings.Contains(response.Body.String(), `"data":{}`) {
		t.Fatalf("empty wx config status=%d body=%s", response.Code, response.Body.String())
	}
}

func TestApplicationLoginUsesConfiguredAdminPassword(t *testing.T) {
	cfg := testApplicationConfig(t)
	cfg.AdminPassword = "configured-admin-password"
	application, err := New(cfg)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	runErr := make(chan error, 1)
	go func() { runErr <- application.Run(ctx) }()

	client := &http.Client{Timeout: 250 * time.Millisecond}
	baseURL := "http://127.0.0.1:" + strconv.Itoa(application.Config.AdminPort)
	deadline := time.Now().Add(3 * time.Second)
	for {
		response, requestErr := client.Get(baseURL + "/api/health")
		if requestErr == nil {
			_ = response.Body.Close()
			break
		}
		if time.Now().After(deadline) {
			cancel()
			<-runErr
			_ = application.Shutdown(context.Background())
			t.Fatalf("server did not become ready: %v", requestErr)
		}
		time.Sleep(20 * time.Millisecond)
	}

	loginClient := &http.Client{Timeout: 5 * time.Second}
	response, err := loginClient.Post(baseURL+"/api/login", "application/json", strings.NewReader(`{"username":"admin","password":"configured-admin-password"}`))
	if err != nil {
		cancel()
		<-runErr
		_ = application.Shutdown(context.Background())
		t.Fatalf("login request error = %v", err)
	}
	if response.StatusCode != http.StatusOK {
		if response.Body != nil {
			_ = response.Body.Close()
		}
		cancel()
		<-runErr
		_ = application.Shutdown(context.Background())
		t.Fatalf("configured admin login status = %d, want %d", response.StatusCode, http.StatusOK)
	}
	var loginPayload struct {
		OK   bool `json:"ok"`
		Data struct {
			Token        string         `json:"token"`
			Role         string         `json:"role"`
			Card         map[string]any `json:"card"`
			AccountLimit int            `json:"accountLimit"`
			User         struct {
				Username string `json:"username"`
			} `json:"user"`
			Password string `json:"Password"`
			PwdHash  string `json:"PwdHash"`
			Salt     string `json:"Salt"`
		} `json:"data"`
	}
	loginRaw, readErr := io.ReadAll(response.Body)
	_ = response.Body.Close()
	if readErr != nil {
		cancel()
		<-runErr
		_ = application.Shutdown(context.Background())
		t.Fatalf("read login response: %v", readErr)
	}
	if err := json.Unmarshal(loginRaw, &loginPayload); err != nil {
		cancel()
		<-runErr
		_ = application.Shutdown(context.Background())
		t.Fatalf("decode login response: %v", err)
	}
	if !loginPayload.OK || loginPayload.Data.Token == "" || loginPayload.Data.Role != "admin" || loginPayload.Data.User.Username != "admin" || loginPayload.Data.AccountLimit != 2 {
		cancel()
		<-runErr
		_ = application.Shutdown(context.Background())
		t.Fatalf("login response contract = %s", loginRaw)
	}
	if loginPayload.Data.Password != "" || loginPayload.Data.PwdHash != "" || loginPayload.Data.Salt != "" {
		cancel()
		<-runErr
		_ = application.Shutdown(context.Background())
		t.Fatalf("login response leaked credential fields: %s", loginRaw)
	}
	currentRequest, _ := http.NewRequest(http.MethodGet, baseURL+"/api/user/me", nil)
	currentRequest.Header.Set("x-admin-token", loginPayload.Data.Token)
	currentResponse, currentErr := loginClient.Do(currentRequest)
	if currentErr != nil {
		cancel()
		<-runErr
		_ = application.Shutdown(context.Background())
		t.Fatalf("current user request error = %v", currentErr)
	}
	currentRaw, _ := io.ReadAll(currentResponse.Body)
	_ = currentResponse.Body.Close()
	if currentResponse.StatusCode != http.StatusOK || strings.Contains(string(currentRaw), `"PwdHash"`) || strings.Contains(string(currentRaw), `"Password"`) || strings.Contains(string(currentRaw), `"Salt"`) {
		cancel()
		<-runErr
		_ = application.Shutdown(context.Background())
		t.Fatalf("current user response status=%d body=%s", currentResponse.StatusCode, currentRaw)
	}

	cancel()
	select {
	case runErrValue := <-runErr:
		if runErrValue != nil {
			t.Fatalf("Run() after login cancellation error = %v", runErrValue)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("Run() did not stop after login test cancellation")
	}
	if err := application.Shutdown(context.Background()); err != nil {
		t.Fatalf("Shutdown() error = %v", err)
	}
}

func TestApplicationRunStopsOnContextCancellation(t *testing.T) {
	application, err := New(testApplicationConfig(t))
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	runErr := make(chan error, 1)
	go func() { runErr <- application.Run(ctx) }()

	client := &http.Client{Timeout: 250 * time.Millisecond}
	healthURL := "http://127.0.0.1:" + strconv.Itoa(application.Config.AdminPort) + "/api/health"
	deadline := time.Now().Add(3 * time.Second)
	for {
		response, requestErr := client.Get(healthURL)
		if requestErr == nil {
			_ = response.Body.Close()
			break
		}
		if time.Now().After(deadline) {
			cancel()
			<-runErr
			t.Fatalf("server did not become ready: %v", requestErr)
		}
		time.Sleep(20 * time.Millisecond)
	}

	cancel()
	select {
	case err := <-runErr:
		if err != nil {
			t.Fatalf("Run() after cancellation error = %v", err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("Run() did not stop after context cancellation")
	}

	if err := application.Shutdown(context.Background()); err != nil {
		t.Fatalf("Shutdown() error = %v", err)
	}
}

func TestApplicationProtectsAdminRoutesAndSupportsRecoveryLogout(t *testing.T) {
	application, err := New(testApplicationConfig(t))
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	defer func() { _ = application.Shutdown(context.Background()) }()

	ctx := context.Background()
	card, err := application.Cards.Create(ctx, store.CardSpec{Description: "recovery", Days: 2, Type: "time"})
	if err != nil {
		t.Fatal(err)
	}
	user, err := application.Cards.RegisterWithCard(ctx, "recoveruser", "Aa123456", card.Code)
	if err != nil {
		t.Fatal(err)
	}
	userToken, err := application.Sessions.Create(ctx, *user)
	if err != nil {
		t.Fatal(err)
	}
	request := httptest.NewRequest(http.MethodGet, "/api/admin/users", nil)
	request.Header.Set("x-admin-token", userToken)
	response := httptest.NewRecorder()
	application.Server.Handler().ServeHTTP(response, request)
	if response.Code != http.StatusForbidden {
		t.Fatalf("normal user admin route status=%d, want 403", response.Code)
	}

	request = httptest.NewRequest(http.MethodPost, "/api/public/reset-password/verify", strings.NewReader(fmt.Sprintf(`{"username":"recoveruser","cardCode":"%s"}`, card.Code)))
	request.Header.Set("Content-Type", "application/json")
	response = httptest.NewRecorder()
	application.Server.Handler().ServeHTTP(response, request)
	if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), `"valid":true`) {
		t.Fatalf("recovery verification status=%d body=%s", response.Code, response.Body.String())
	}

	request = httptest.NewRequest(http.MethodPost, "/api/public/reset-password/confirm", strings.NewReader(fmt.Sprintf(`{"username":"recoveruser","cardCode":"%s","newPassword":"Bb123456"}`, card.Code)))
	request.Header.Set("Content-Type", "application/json")
	response = httptest.NewRecorder()
	application.Server.Handler().ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("recovery confirmation status=%d body=%s", response.Code, response.Body.String())
	}
	if _, err := application.Sessions.Lookup(ctx, userToken); !errors.Is(err, middleware.ErrSessionNotFound) {
		t.Fatalf("recovery should invalidate old session, error=%v", err)
	}
	if _, err := application.Users.Authenticate(ctx, "recoveruser", "Bb123456", "127.0.0.1"); err != nil {
		t.Fatalf("new password login error=%v", err)
	}

	admin, err := application.Users.Get(ctx, "admin")
	if err != nil {
		t.Fatal(err)
	}
	adminToken, err := application.Sessions.Create(ctx, *admin)
	if err != nil {
		t.Fatal(err)
	}
	request = httptest.NewRequest(http.MethodPost, "/api/logout", strings.NewReader(`{}`))
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("x-admin-token", adminToken)
	response = httptest.NewRecorder()
	application.Server.Handler().ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("logout status=%d body=%s", response.Code, response.Body.String())
	}
	if _, err := application.Sessions.Lookup(ctx, adminToken); !errors.Is(err, middleware.ErrSessionNotFound) {
		t.Fatalf("logout should invalidate token, error=%v", err)
	}
}

func testApplicationConfig(t *testing.T) config.Config {
	t.Helper()
	port := freeApplicationPort(t)
	return config.Config{
		AdminPort: port,
		DataDir:   t.TempDir(),
		Paths:     config.NewPaths(t.TempDir(), "", nil),
		MasterKey: "01234567890123456789012345678901",
		TSDK:      config.TSDKConfig{AppKey: "0"},
	}
}

func freeApplicationPort(t *testing.T) int {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("reserve test port: %v", err)
	}
	defer func() { _ = listener.Close() }()
	_, portText, err := net.SplitHostPort(listener.Addr().String())
	if err != nil {
		t.Fatalf("split test address: %v", err)
	}
	port := 0
	if _, err := fmt.Sscanf(portText, "%d", &port); err != nil {
		t.Fatalf("parse test port: %v", err)
	}
	return port
}
