package protocol

import "testing"

func TestExtractCodeFromNestedJSON(t *testing.T) {
	resp := []byte(`{"data":{"result":{"wx_code":"json-login-code-123"}}}`)
	if got := string(extractCode(resp)); got != "json-login-code-123" {
		t.Fatalf("extractCode(JSON) = %q", got)
	}
}

func TestExtractCodeFromNestedProtobuf(t *testing.T) {
	resp := pbLen(2, pbLen(3, []byte("protobuf-login-code-123")))
	if got := string(extractCode(resp)); got != "protobuf-login-code-123" {
		t.Fatalf("extractCode(protobuf) = %q", got)
	}
}

func TestExtractCodeFromPlainText(t *testing.T) {
	if got := string(extractCode([]byte("plain-login-code-123"))); got != "plain-login-code-123" {
		t.Fatalf("extractCode(plain) = %q", got)
	}
}
