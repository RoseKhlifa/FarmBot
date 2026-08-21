package app

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestCaptureFlowUsesRemoteServiceAndPersistsAccount(t *testing.T) {
	remote := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/health":
			_, _ = io.WriteString(w, `{"ok":true,"uptime":12,"sessions":0,"portPool":[1,2]}`)
		case r.Method == http.MethodPost && r.URL.Path == "/api/sessions":
			_, _ = io.WriteString(w, `{"ok":true,"data":{}}`)
		case r.Method == http.MethodPost && r.URL.Path == "/api/capture/start":
			_, _ = io.WriteString(w, `{"ok":true,"data":{"channels":{"qq":{"status":"running","codes":[{"code":"captured-code","gid":"12345"}]}},"publicInfo":{"host":"127.0.0.1","mitmPort":18080,"certUrl":"/cert/ca.cer"},"proxy":{"running":true}}}`)
		case r.Method == http.MethodGet && strings.HasSuffix(r.URL.Path, "/state"):
			_, _ = io.WriteString(w, `{"ok":true,"data":{"channels":{"qq":{"status":"running","codes":[{"code":"captured-code","gid":"12345"}]}},"friends":{"items":[{"gid":67890}]}}}`)
		case r.Method == http.MethodGet && r.URL.Path == "/cert/ca.cer":
			w.Header().Set("Content-Type", "application/x-x509-ca-cert")
			_, _ = w.Write([]byte("certificate"))
		case r.Method == http.MethodDelete && strings.HasPrefix(r.URL.Path, "/api/sessions/"):
			_, _ = io.WriteString(w, `{"ok":true}`)
		default:
			http.NotFound(w, r)
		}
	}))
	defer remote.Close()

	cfg := testApplicationConfig(t)
	application, err := New(cfg)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	defer func() { _ = application.Shutdown(context.Background()) }()
	if err := application.ConfigDB.SetGlobal(context.Background(), "captureConfig", mustJSON(map[string]any{
		"enabled": true, "apiBase": remote.URL, "apiToken": "test-token", "autoImportQqGids": true,
	})); err != nil {
		t.Fatalf("save capture config: %v", err)
	}

	admin, err := application.Users.Get(context.Background(), "admin")
	if err != nil {
		t.Fatalf("get admin: %v", err)
	}
	token, err := application.Sessions.Create(context.Background(), *admin)
	if err != nil {
		t.Fatalf("create session: %v", err)
	}
	create := captureRequest(t, application.Server.Handler(), http.MethodPost, "/api/capture/sessions", token, map[string]any{"platform": "qq"})
	if create.Code != http.StatusOK {
		t.Fatalf("create capture status=%d body=%s", create.Code, create.Body.String())
	}
	var created struct {
		Data struct {
			ID           string `json:"id"`
			CodeCaptured bool   `json:"codeCaptured"`
			PublicInfo   struct {
				CertificateURL string `json:"certificateUrl"`
			} `json:"publicInfo"`
		} `json:"data"`
	}
	if err := json.Unmarshal(create.Body.Bytes(), &created); err != nil {
		t.Fatalf("decode capture response: %v", err)
	}
	if created.Data.ID == "" || !created.Data.CodeCaptured || created.Data.PublicInfo.CertificateURL == "" {
		t.Fatalf("capture response = %s", create.Body.String())
	}
	complete := captureRequest(t, application.Server.Handler(), http.MethodPost, "/api/capture/sessions/"+created.Data.ID+"/complete", token, map[string]any{"name": "Captured"})
	if complete.Code != http.StatusOK {
		t.Fatalf("complete capture status=%d body=%s", complete.Code, complete.Body.String())
	}
	accounts, err := application.Accounts.List(context.Background())
	if err != nil || len(accounts) != 1 || accounts[0].Code != "captured-code" || accounts[0].OwnerUser != "admin" {
		t.Fatalf("captured accounts=%+v err=%v", accounts, err)
	}
	certificate := captureRequest(t, application.Server.Handler(), http.MethodGet, "/api/public"+strings.TrimPrefix(created.Data.PublicInfo.CertificateURL, "/api/public"), "", nil)
	if certificate.Code != http.StatusOK || certificate.Body.String() != "certificate" {
		t.Fatalf("certificate status=%d body=%q", certificate.Code, certificate.Body.String())
	}
	duplicate := captureRequest(t, application.Server.Handler(), http.MethodPost, "/api/capture/sessions", token, map[string]any{"platform": "qq"})
	if duplicate.Code != http.StatusOK {
		t.Fatalf("duplicate flow create status=%d body=%s", duplicate.Code, duplicate.Body.String())
	}
	var duplicateFlow struct {
		Data struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	if err := json.Unmarshal(duplicate.Body.Bytes(), &duplicateFlow); err != nil || duplicateFlow.Data.ID == "" {
		t.Fatalf("duplicate flow response=%s err=%v", duplicate.Body.String(), err)
	}
	duplicateComplete := captureRequest(t, application.Server.Handler(), http.MethodPost, "/api/capture/sessions/"+duplicateFlow.Data.ID+"/complete", token, map[string]any{"name": "Duplicate"})
	if duplicateComplete.Code != http.StatusConflict || !strings.Contains(duplicateComplete.Body.String(), "DUPLICATE_CAPTURE_ACCOUNT") {
		t.Fatalf("duplicate completion status=%d body=%s", duplicateComplete.Code, duplicateComplete.Body.String())
	}
}

func mustJSON(value any) []byte {
	raw, _ := json.Marshal(value)
	return raw
}

func captureRequest(t *testing.T, handler http.Handler, method, path, token string, payload map[string]any) *httptest.ResponseRecorder {
	t.Helper()
	var body io.Reader
	if payload != nil {
		body = bytes.NewReader(mustJSON(payload))
	}
	request := httptest.NewRequest(method, path, body)
	if payload != nil {
		request.Header.Set("Content-Type", "application/json")
	}
	if token != "" {
		request.Header.Set("x-admin-token", token)
	}
	response := httptest.NewRecorder()
	gin.SetMode(gin.TestMode)
	handler.ServeHTTP(response, request)
	return response
}
