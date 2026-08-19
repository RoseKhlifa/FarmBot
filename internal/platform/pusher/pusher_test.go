package pusher

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestSendPostsPayloadAndNormalizesSuccess(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		var body map[string]string
		if err := json.NewDecoder(request.Body).Decode(&body); err != nil {
			t.Errorf("decode request: %v", err)
		}
		if body["token"] != "secret" || body["title"] != "title" || body["content"] != "content" {
			t.Errorf("unexpected request body: %#v", body)
		}
		writer.Header().Set("Content-Type", "application/json")
		_, _ = writer.Write([]byte(`{"code":0,"msg":"success"}`))
	}))
	defer server.Close()

	result, err := New(server.Client()).Send(context.Background(), Payload{
		Channel: "telegram", Endpoint: server.URL, Token: "secret", Title: "title", Content: "content",
	})
	if err != nil || !result.OK || result.Code != "0" {
		t.Fatalf("unexpected push result: %+v, %v", result, err)
	}
}

func TestSendRejectsProviderFailure(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writer.WriteHeader(http.StatusBadRequest)
		_, _ = writer.Write([]byte(`{"code":400,"msg":"invalid token"}`))
	}))
	defer server.Close()

	result, err := New(server.Client()).Send(context.Background(), Payload{
		Channel: "telegram", Endpoint: server.URL, Token: "secret", Title: "title", Content: "content",
	})
	if err == nil || result.OK || !strings.Contains(err.Error(), "rejected") {
		t.Fatalf("unexpected provider failure: %+v, %v", result, err)
	}
}

func TestWebhookDoesNotRequireToken(t *testing.T) {
	if _, err := New(nil).Send(context.Background(), Payload{Channel: "webhook", Endpoint: "not a URL", Title: "title", Content: "content"}); err == nil {
		t.Fatal("webhook accepted an invalid endpoint")
	}
}
