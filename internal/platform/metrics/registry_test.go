package metrics

import (
	"context"
	"strings"
	"testing"
	"time"
)

func TestRegistryCollectsCoreMetrics(t *testing.T) {
	registry := New(Config{
		Sources: Sources{
			Accounts: func(context.Context) ([]AccountSnapshot, error) {
				return []AccountSnapshot{{AccountID: "a-2", Online: false}, {AccountID: "a-1", Online: true, Operations: map[string]float64{"harvest": 3}}}, nil
			},
			WebSocketConnections:  func() int { return 2 },
			TSDKHeartbeatFailures: func(context.Context) (uint64, error) { return 4, nil },
		},
	})
	registry.RecordLoginSuccess()
	registry.RecordLoginFailure()
	registry.RecordTSDKHeartbeatFailure()
	registry.RecordACEHeartbeatFailure()
	registry.ObserveHTTP("get", "/api/farm", 20*time.Millisecond)

	payload, err := registry.Render(context.Background())
	if err != nil {
		t.Fatalf("Render() error = %v", err)
	}
	text := string(payload)
	for _, want := range []string{
		"farmbot_accounts_online 1\n",
		`farmbot_account_online{account_id="a-1"} 1`,
		`farmbot_account_operation_total{account_id="a-1",operation="harvest"} 3`,
		"farmbot_websocket_connections 2\n",
		`farmbot_logins_total{result="success"} 1`,
		`farmbot_logins_total{result="failure"} 1`,
		"farmbot_tsdk_heartbeat_failures_total 5\n",
		"farmbot_ace_heartbeat_failures_total 1\n",
		`farmbot_http_request_duration_seconds_count{method="GET",route="/api/farm"} 1`,
	} {
		if !strings.Contains(text, want) {
			t.Errorf("rendered metrics do not contain %q:\n%s", want, text)
		}
	}
	for _, metadata := range []string{
		"# HELP farmbot_account_online ",
		"# HELP farmbot_logins_total ",
	} {
		if count := strings.Count(text, metadata); count != 1 {
			t.Errorf("metadata %q occurred %d times, want once", metadata, count)
		}
	}
}

func TestRegistryDoesNotShareMutableState(t *testing.T) {
	first := New(Config{})
	second := New(Config{})
	first.RecordLoginSuccess()
	first.SetWebSocketConnections(9)
	first.ObserveHTTP("GET", "/", time.Second)

	snapshot, err := second.Collect(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if snapshot.LoginSuccesses != 0 || snapshot.WebSocketConnections != 0 || len(snapshot.HTTPHistograms) != 0 {
		t.Fatalf("registries share state: %+v", snapshot)
	}
}

func TestRegistryRejectsSourceErrors(t *testing.T) {
	registry := New(Config{AccountSource: func(context.Context) ([]AccountSnapshot, error) {
		return nil, context.Canceled
	}})
	if _, err := registry.Collect(context.Background()); err == nil {
		t.Fatal("Collect() unexpectedly succeeded")
	}
}
