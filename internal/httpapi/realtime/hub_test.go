package realtime

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/RoseKhlifa/FarmBot/internal/httpapi/middleware"
	"github.com/RoseKhlifa/FarmBot/internal/store"
	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

type realtimeFrame struct {
	Event string                 `json:"event"`
	Data  map[string]interface{} `json:"data"`
}

func TestNativeWebSocketSubscribesSnapshotsAndBroadcasts(t *testing.T) {
	gin.SetMode(gin.TestMode)
	sessions, err := middleware.NewSessionManager(middleware.SessionManagerConfig{})
	if err != nil {
		t.Fatal(err)
	}
	token, err := sessions.Create(context.Background(), store.User{Username: "admin", Role: "admin", Status: "active"})
	if err != nil {
		t.Fatal(err)
	}
	hub := NewHub(Config{
		Sessions: sessions,
		Snapshot: SnapshotProvider{
			Status:      func(context.Context, string) (any, error) { return map[string]any{"online": true}, nil },
			Logs:        func(context.Context, string, int) (any, error) { return []string{"one"}, nil },
			AccountLogs: func(context.Context, int) (any, error) { return []string{"account"}, nil },
		},
	})
	defer func() { _ = hub.Close() }()
	server := httptest.NewServer(http.HandlerFunc(hub.HandleWebSocket))
	defer server.Close()
	wsURL := "ws" + server.URL[len("http"):]
	query := url.Values{"accountId": {"a-1"}}
	conn, _, err := websocket.DefaultDialer.Dial(wsURL+"?"+query.Encode(), http.Header{middleware.AdminTokenHeader: []string{token}})
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = conn.Close() }()

	assertEvent := func(want string) realtimeFrame {
		t.Helper()
		_ = conn.SetReadDeadline(time.Now().Add(time.Second))
		_, data, err := conn.ReadMessage()
		if err != nil {
			t.Fatal(err)
		}
		var frame realtimeFrame
		if err := json.Unmarshal(data, &frame); err != nil {
			t.Fatal(err)
		}
		if frame.Event != want {
			t.Fatalf("event = %q, want %q (data=%s)", frame.Event, want, data)
		}
		return frame
	}
	assertEvent(SubscribedEvent)
	assertEvent(StatusUpdateEvent)
	assertEvent(LogsSnapshotEvent)
	assertEvent(AccountLogsSnapshotEvent)
	assertEvent(ReadyEvent)
	if got := hub.SubscriberCount("a-1"); got != 1 {
		t.Fatalf("subscriber count = %d, want 1", got)
	}
	if got := hub.BroadcastStatus("a-1", map[string]any{"phase": "online"}); got != 1 {
		t.Fatalf("broadcast count = %d, want 1", got)
	}
	frame := assertEvent(StatusUpdateEvent)
	if frame.Data["accountId"] != "a-1" {
		t.Fatalf("status data = %#v", frame.Data)
	}
	if got := hub.BroadcastLog(map[string]any{"accountId": "a-1", "message": "hello"}); got != 1 {
		t.Fatalf("log broadcast count = %d, want 1", got)
	}
	assertEvent(LogNewEvent)
}

func TestNonElevatedCannotSubscribeUnauthorizedAccount(t *testing.T) {
	sessions, err := middleware.NewSessionManager(middleware.SessionManagerConfig{})
	if err != nil {
		t.Fatal(err)
	}
	token, err := sessions.Create(context.Background(), store.User{Username: "user", Role: "user", Status: "active"})
	if err != nil {
		t.Fatal(err)
	}
	hub := NewHub(Config{
		Sessions: sessions,
		AuthorizeAccount: func(_ context.Context, user store.User, accountID string) (bool, error) {
			return user.Username == "user" && accountID == "a-1", nil
		},
	})
	defer func() { _ = hub.Close() }()
	server := httptest.NewServer(http.HandlerFunc(hub.HandleWebSocket))
	defer server.Close()
	wsURL := "ws" + server.URL[len("http"):]
	query := url.Values{"accountId": {"a-2"}}
	_, response, err := websocket.DefaultDialer.Dial(wsURL+"?"+query.Encode(), http.Header{middleware.AdminTokenHeader: []string{token}})
	if err == nil {
		t.Fatal("unauthorized subscription unexpectedly upgraded")
	}
	if response == nil || response.StatusCode != http.StatusForbidden {
		t.Fatalf("handshake response = %#v, err = %v", response, err)
	}
}

func TestRegisterRoutesAndAccountLogPublisher(t *testing.T) {
	hub := NewHub(Config{})
	router := gin.New()
	hub.RegisterRoutes(router)
	found := false
	for _, route := range router.Routes() {
		if route.Method == http.MethodGet && route.Path == "/ws" {
			found = true
		}
	}
	if !found {
		t.Fatal("/ws route was not registered")
	}
	if got := hub.BroadcastAccountLog(map[string]any{"account_id": "a-1"}); got != 0 {
		t.Fatalf("broadcast without subscribers = %d", got)
	}
	if got := accountIDFromPayload(struct {
		AccountID string `json:"account_id"`
	}{AccountID: "a-2"}); got != "a-2" {
		t.Fatalf("account id extraction = %q", got)
	}
}

func TestClientCanChangeAccountSubscription(t *testing.T) {
	sessions, err := middleware.NewSessionManager(middleware.SessionManagerConfig{})
	if err != nil {
		t.Fatal(err)
	}
	token, err := sessions.Create(context.Background(), store.User{Username: "admin", Role: "admin", Status: "active"})
	if err != nil {
		t.Fatal(err)
	}
	hub := NewHub(Config{Sessions: sessions})
	defer func() { _ = hub.Close() }()
	server := httptest.NewServer(http.HandlerFunc(hub.HandleWS))
	defer server.Close()
	wsURL := "ws" + server.URL[len("http"):]
	query := url.Values{"accountId": {"a-1"}}
	conn, _, err := websocket.DefaultDialer.Dial(wsURL+"?"+query.Encode(), http.Header{middleware.AdminTokenHeader: []string{token}})
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = conn.Close() }()
	readEvent := func() string {
		t.Helper()
		_ = conn.SetReadDeadline(time.Now().Add(time.Second))
		_, data, err := conn.ReadMessage()
		if err != nil {
			t.Fatal(err)
		}
		var frame realtimeFrame
		if err := json.Unmarshal(data, &frame); err != nil {
			t.Fatal(err)
		}
		return frame.Event
	}
	if event := readEvent(); event != SubscribedEvent {
		t.Fatalf("initial event = %q", event)
	}
	if event := readEvent(); event != ReadyEvent {
		t.Fatalf("ready event = %q", event)
	}
	if err := conn.WriteJSON(map[string]any{"event": "subscribe", "data": map[string]string{"accountId": "a-2"}}); err != nil {
		t.Fatal(err)
	}
	if event := readEvent(); event != SubscribedEvent {
		t.Fatalf("resubscribe event = %q", event)
	}
	if got := hub.SubscriberCount("a-1"); got != 0 {
		t.Fatalf("old subscriber count = %d", got)
	}
	if got := hub.SubscriberCount("a-2"); got != 1 {
		t.Fatalf("new subscriber count = %d", got)
	}
	if got := hub.BroadcastStatus("a-1", map[string]any{"phase": "old"}); got != 0 {
		t.Fatalf("old account broadcast count = %d", got)
	}
	if got := hub.BroadcastStatus("a-2", map[string]any{"phase": "new"}); got != 1 {
		t.Fatalf("new account broadcast count = %d", got)
	}
	if event := readEvent(); event != StatusUpdateEvent {
		t.Fatalf("new account event = %q", event)
	}
}
