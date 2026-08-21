package app

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/RoseKhlifa/FarmBot/internal/httpapi/handlers"
	"github.com/RoseKhlifa/FarmBot/internal/store"
	"github.com/RoseKhlifa/FarmBot/internal/yyb"
)

type codeServiceStub struct {
	yyb.Service
	called bool
}

func (s *codeServiceStub) GetCode(context.Context, string, string) (string, error) {
	s.called = true
	return "builtin-code", nil
}

type qrConfirmServiceStub struct{ yyb.Service }

func (qrConfirmServiceStub) QRConfirm(context.Context, string) (yyb.QRConfirmResult, error) {
	secret := "login-buffer-secret"
	return yyb.QRConfirmResult{Account: &yyb.WechatAccount{
		OpenID:      "openid-qr",
		LoginBuffer: secret,
		Credentials: map[string]any{"session": "secret"},
	}}, nil
}

func TestYYBProviderQRConfirmReturnsOnlyPublicAccount(t *testing.T) {
	provider := yybProvider{service: qrConfirmServiceStub{}}
	result, err := provider.Handle(context.Background(), "/api/yyb/qr/confirm", map[string]any{"sessionId": "session-1"})
	if err != nil {
		t.Fatalf("QR confirm error = %v", err)
	}
	data, ok := result.(map[string]any)
	if !ok {
		t.Fatalf("QR confirm result = %#v", result)
	}
	account, ok := data["account"].(yyb.AccountPublic)
	if !ok || account.OpenID != "openid-qr" {
		t.Fatalf("public account = %#v", data["account"])
	}
	if _, leaked := data["login_buffer"]; leaked {
		t.Fatal("QR confirm leaked login buffer")
	}
}

func TestYYBProviderBuiltinGetCodeReturnsObject(t *testing.T) {
	provider := yybProvider{service: &codeServiceStub{}}
	result, err := provider.Handle(context.Background(), "/api/yyb/getcode", map[string]any{
		"openid": "openid-builtin",
	})
	if err != nil {
		t.Fatalf("builtin getCode error = %v", err)
	}
	data, ok := result.(map[string]any)
	if !ok || data["code"] != "builtin-code" || data["openid"] != "openid-builtin" {
		t.Fatalf("builtin getCode result = %#v", result)
	}
}

func TestYYBProviderExternalAccounts(t *testing.T) {
	var gotAuth string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		if r.Method != http.MethodGet || r.URL.Path != "/accounts" {
			t.Fatalf("unexpected accounts request: %s %s", r.Method, r.URL.RequestURI())
		}
		_, _ = io.WriteString(w, `{"ok":true,"data":{"accounts":[{"openid":"openid-1"}]}}`)
	}))
	defer server.Close()

	provider := yybProvider{client: server.Client()}
	result, err := provider.Handle(context.Background(), "/api/yyb/accounts", map[string]any{
		"apiBase": server.URL,
		"apiKey":  "secret-token",
	})
	if err != nil {
		t.Fatalf("external accounts error = %v", err)
	}
	if gotAuth != "Bearer secret-token" {
		t.Fatalf("Authorization = %q, want bearer token", gotAuth)
	}
	data, ok := result.(map[string]any)
	if !ok || data["accounts"] == nil {
		t.Fatalf("external accounts result = %#v", result)
	}
}

func TestYYBProviderExternalGetCodeNormalizesEndpointSuffix(t *testing.T) {
	var gotBody map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/wxapp/getCode" {
			t.Fatalf("unexpected getCode request: %s %s", r.Method, r.URL.RequestURI())
		}
		if err := json.NewDecoder(r.Body).Decode(&gotBody); err != nil {
			t.Fatalf("decode getCode body: %v", err)
		}
		_, _ = io.WriteString(w, `{"code":0,"data":{"code":"wx-code"}}`)
	}))
	defer server.Close()

	provider := yybProvider{client: server.Client()}
	result, err := provider.Handle(context.Background(), "/api/yyb/getcode", map[string]any{
		"apiBase": server.URL + "/wxapp/getCode",
		"apiKey":  "secret-token",
		"openid":  "openid-1",
		"appId":   "app-1",
	})
	if err != nil {
		t.Fatalf("external getCode error = %v", err)
	}
	if gotBody["ref"] != "openid-1" || gotBody["app_id"] != "app-1" {
		t.Fatalf("getCode body = %#v", gotBody)
	}
	data, ok := result.(map[string]any)
	if !ok || data["code"] != "wx-code" {
		t.Fatalf("external getCode result = %#v", result)
	}
}

func TestYYBProviderExternalThirdPartyNormalizesEndpointSuffix(t *testing.T) {
	var gotBody map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/api/open/v1/farm/code" {
			t.Fatalf("unexpected third-party request: %s %s", r.Method, r.URL.RequestURI())
		}
		if !strings.HasPrefix(r.Header.Get("Authorization"), "Bearer ") {
			t.Fatalf("missing third-party authorization header")
		}
		if err := json.NewDecoder(r.Body).Decode(&gotBody); err != nil {
			t.Fatalf("decode third-party body: %v", err)
		}
		_, _ = io.WriteString(w, `{"code":"third-party-code"}`)
	}))
	defer server.Close()

	provider := yybProvider{client: server.Client()}
	result, err := provider.Handle(context.Background(), "/api/yyb/thirdparty-code", map[string]any{
		"apiBase":      server.URL + "/api/open/v1/farm/code",
		"apiToken":     "third-party-token",
		"openid":       "openid-1",
		"forceRefresh": true,
	})
	if err != nil {
		t.Fatalf("external third-party error = %v", err)
	}
	if gotBody["openid"] != "openid-1" || gotBody["forceRefresh"] != true {
		t.Fatalf("third-party body = %#v", gotBody)
	}
	data, ok := result.(map[string]any)
	if !ok || data["code"] != "third-party-code" {
		t.Fatalf("external third-party result = %#v", result)
	}
}

func TestAccountCodeProviderKeepsBuiltinAndThirdPartyPathsSeparate(t *testing.T) {
	builtinService := &codeServiceStub{}
	provider := accountCodeProvider(builtinService)
	builtinCode, err := provider(context.Background(), store.Account{
		LoginType:      "yyb",
		Provider:       "builtin",
		YYBOpenID:      "builtin-openid",
		ThirdPartyJSON: mustJSON(map[string]any{"apiBase": "https://stale.example"}),
	}, nil, "app-1")
	if err != nil || builtinCode != "builtin-code" || !builtinService.called {
		t.Fatalf("builtin code = %q, err=%v, called=%v", builtinCode, err, builtinService.called)
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/open/v1/farm/code" {
			t.Fatalf("unexpected third-party path: %s", r.URL.Path)
		}
		_, _ = io.WriteString(w, `{"success":true,"data":{"code":"external-code"}}`)
	}))
	defer server.Close()
	thirdPartyCode, err := provider(context.Background(), store.Account{
		LoginType: "yyb",
		Provider:  "thirdparty",
		YYBOpenID: "external-openid",
		ThirdPartyJSON: mustJSON(map[string]any{
			"apiBase":  server.URL,
			"apiToken": "external-token",
		}),
	}, nil, "app-1")
	if err != nil || thirdPartyCode != "external-code" {
		t.Fatalf("third-party code = %q, err=%v", thirdPartyCode, err)
	}
}

func TestYYBProviderExternalErrorsPreserveHTTPStatus(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = io.WriteString(w, `{"success":false,"error":"invalid token"}`)
	}))
	defer server.Close()

	provider := yybProvider{client: server.Client()}
	_, err := provider.Handle(context.Background(), "/api/yyb/accounts", map[string]any{
		"apiBase": server.URL,
		"apiKey":  "bad-token",
	})
	var httpErr *handlers.HTTPError
	if !errors.As(err, &httpErr) || httpErr.Status != http.StatusUnauthorized {
		t.Fatalf("external error = %v, want HTTP 401", err)
	}
}

func TestMergeThirdPartyJSONPreservesMaskedTokenOnEdit(t *testing.T) {
	merged := objectJSON(mergeThirdPartyJSON(
		json.RawMessage(`{"apiToken":"stored-secret","autoReconnect":true}`),
		json.RawMessage(`{"apiToken":"","autoReconnect":false}`),
	))
	if merged["apiToken"] != "stored-secret" || merged["autoReconnect"] != false {
		t.Fatalf("merged third-party config = %#v", merged)
	}
}
