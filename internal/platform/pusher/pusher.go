// Package pusher sends notifications through the HTTP endpoints used by the
// legacy pushoo integration.
package pusher

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// Payload is the common cross-channel notification contract.
type Payload struct {
	Channel  string `json:"channel"`
	Endpoint string `json:"endpoint,omitempty"`
	Token    string `json:"token,omitempty"`
	Title    string `json:"title"`
	Content  string `json:"content"`
}

// Notification is an alias for Payload.
type Notification = Payload

// Result normalizes provider-specific response shapes.
type Result struct {
	OK      bool   `json:"ok"`
	Code    string `json:"code"`
	Message string `json:"msg"`
	Raw     any    `json:"raw,omitempty"`
}

// Client is an HTTP pusher. Endpoint is supplied per payload because the
// configured channels commonly have different provider URLs.
type Client struct {
	HTTPClient *http.Client
	Timeout    time.Duration
}

// Pusher is an alias for Client.
type Pusher = Client

// New creates a pusher client with a bounded default HTTP timeout.
func New(httpClient *http.Client) *Client {
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 30 * time.Second}
	}
	return &Client{HTTPClient: httpClient, Timeout: 30 * time.Second}
}

// NewPusher is an explicit constructor alias.
func NewPusher(httpClient *http.Client) *Client { return New(httpClient) }

// Send validates and posts a notification to the configured provider endpoint.
func (c *Client) Send(ctx context.Context, payload Payload) (Result, error) {
	if c == nil {
		c = New(nil)
	}
	if ctx == nil {
		ctx = context.Background()
	}
	rawChannel := strings.TrimSpace(payload.Channel)
	channel := strings.ToLower(rawChannel)
	if channel == "" {
		return Result{}, errors.New("push channel is required")
	}
	endpoint := strings.TrimSpace(payload.Endpoint)
	webhookMethod := http.MethodPost
	// pushoo also accepts a webhook URL directly as the channel name.
	if strings.HasPrefix(channel, "http://") || strings.HasPrefix(channel, "https://") {
		if strings.HasSuffix(channel, ":get") {
			webhookMethod = http.MethodGet
		}
		endpoint = strings.TrimSuffix(rawChannel, ":GET")
		channel = "webhook"
	}
	if endpoint == "" {
		return Result{}, errors.New("push endpoint is required")
	}
	parsedURL, err := url.Parse(endpoint)
	if err != nil || (parsedURL.Scheme != "http" && parsedURL.Scheme != "https") || parsedURL.Host == "" {
		return Result{}, fmt.Errorf("push endpoint must be an absolute HTTP(S) URL")
	}
	title := strings.TrimSpace(payload.Title)
	if title == "" {
		return Result{}, errors.New("push title is required")
	}
	content := strings.TrimSpace(payload.Content)
	if content == "" {
		return Result{}, errors.New("push content is required")
	}
	token := strings.TrimSpace(payload.Token)
	if channel != "webhook" && token == "" {
		return Result{}, errors.New("push token is required")
	}

	body := map[string]any{"title": title, "content": content}
	if token != "" {
		body["token"] = token
	}
	encoded, err := json.Marshal(body)
	if err != nil {
		return Result{}, fmt.Errorf("encode push request: %w", err)
	}
	requestURL := parsedURL.String()
	var requestBody io.Reader = bytes.NewReader(encoded)
	if channel == "webhook" && webhookMethod == http.MethodGet {
		query := parsedURL.Query()
		query.Set("content", content)
		if title != "" {
			query.Set("title", title)
		}
		if token != "" {
			query.Set("token", token)
		}
		parsedURL.RawQuery = query.Encode()
		requestURL = parsedURL.String()
		requestBody = nil
	}
	request, err := http.NewRequestWithContext(ctx, webhookMethod, requestURL, requestBody)
	if err != nil {
		return Result{}, fmt.Errorf("create push request: %w", err)
	}
	if requestBody != nil {
		request.Header.Set("Content-Type", "application/json")
	}
	request.Header.Set("Accept", "application/json, text/plain;q=0.9")
	httpClient := c.HTTPClient
	if httpClient == nil {
		httpClient = &http.Client{Timeout: c.Timeout}
	}
	response, err := httpClient.Do(request)
	if err != nil {
		return Result{}, fmt.Errorf("send push request: %w", err)
	}
	defer response.Body.Close()
	responseBody, err := io.ReadAll(io.LimitReader(response.Body, 1<<20))
	if err != nil {
		return Result{}, fmt.Errorf("read push response: %w", err)
	}
	result := ParseResult(response.StatusCode, responseBody)
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		result.OK = false
		if result.Code == "" || result.Code == "ok" {
			result.Code = strconv.Itoa(response.StatusCode)
		}
		if result.Message == "" {
			result.Message = response.Status
		}
	}
	if !result.OK {
		return result, fmt.Errorf("push provider rejected notification: %s", result.Message)
	}
	return result, nil
}

// ParseResult applies the same broad success/failure interpretation as the
// legacy pushoo wrapper. It accepts JSON objects and plain-text responses.
func ParseResult(httpStatus int, body []byte) Result {
	text := strings.TrimSpace(string(body))
	var raw any
	if text != "" && json.Unmarshal(body, &raw) != nil {
		raw = text
	}
	object, _ := raw.(map[string]any)
	status := lowerString(firstValue(object, "status", "Status"))
	codeValue := firstValue(object, "code", "errcode", "errCode", "retcode", "errorCode", "statusCode")
	code := valueString(codeValue)
	message := firstValueString(object, "msg", "message", "errmsg", "errMsg", "error_description", "description")
	if message == "" {
		if nested, ok := object["error"].(map[string]any); ok {
			message = firstValueString(nested, "message", "msg", "errmsg")
		}
	}
	if message == "" && object == nil {
		message = text
	}
	allText := strings.ToLower(message + " " + flatten(raw))
	successText := strings.Contains(allText, "成功") || strings.Contains(allText, "success") || strings.Contains(allText, "delivered successfully") || strings.Contains(allText, " ok") || strings.HasPrefix(allText, "ok")
	failureText := strings.Contains(allText, "失败") || strings.Contains(allText, "错误") || strings.Contains(allText, "error") || strings.Contains(allText, "fail") || strings.Contains(allText, "invalid") || strings.Contains(allText, "unauthorized") || strings.Contains(allText, "forbidden") || strings.Contains(allText, "denied")
	explicitError := object != nil && (object["error"] != nil || status == "error" || status == "failed" || boolValue(object["ok"]) == boolFalse || boolValue(object["success"]) == boolFalse)
	successFlag := object != nil && (boolValue(object["ok"]) == boolTrue || boolValue(object["success"]) == boolTrue || valueString(object["result"]) == "success")
	successCode := code == "" || code == "0" || strings.EqualFold(code, "ok") || strings.EqualFold(code, "success") || code == "200" || code == "204"
	ok := httpStatus >= 200 && httpStatus < 300 && !explicitError && !failureText && (successFlag || successCode || successText || status == "success" || object == nil)
	if message == "" {
		if ok {
			message = "push sent"
		} else {
			message = "push failed"
		}
	}
	if code == "" {
		if ok {
			code = "ok"
		} else if httpStatus > 0 {
			code = strconv.Itoa(httpStatus)
		} else {
			code = "error"
		}
	}
	return Result{OK: ok, Code: code, Message: message, Raw: raw}
}

// SendPushooMessage is the migration-friendly spelling of Client.Send.
func SendPushooMessage(ctx context.Context, payload Payload) (Result, error) {
	return New(nil).Send(ctx, payload)
}

// Send is a convenience wrapper for one-off notifications.
func Send(payload Payload) (Result, error) {
	return SendPushooMessage(context.Background(), payload)
}

func firstValue(object map[string]any, keys ...string) any {
	for _, key := range keys {
		if value, ok := object[key]; ok && value != nil {
			return value
		}
	}
	return nil
}

func firstValueString(object map[string]any, keys ...string) string {
	return valueString(firstValue(object, keys...))
}

func valueString(value any) string {
	switch value := value.(type) {
	case nil:
		return ""
	case string:
		return value
	case float64:
		return strconv.FormatFloat(value, 'f', -1, 64)
	case bool:
		return strconv.FormatBool(value)
	default:
		return fmt.Sprint(value)
	}
}

const (
	boolTrue  = "true"
	boolFalse = "false"
)

func boolValue(value any) string {
	switch value := value.(type) {
	case bool:
		return strconv.FormatBool(value)
	case string:
		return strings.ToLower(strings.TrimSpace(value))
	default:
		return ""
	}
}

func lowerString(value any) string { return strings.ToLower(strings.TrimSpace(valueString(value))) }

func flatten(value any) string {
	switch value := value.(type) {
	case nil:
		return ""
	case map[string]any:
		parts := make([]string, 0, len(value))
		for _, item := range value {
			parts = append(parts, flatten(item))
		}
		return strings.Join(parts, " ")
	case []any:
		parts := make([]string, 0, len(value))
		for _, item := range value {
			parts = append(parts, flatten(item))
		}
		return strings.Join(parts, " ")
	default:
		return valueString(value)
	}
}
