package mailer

import (
	"context"
	"strings"
	"testing"
)

func TestSendValidatesLegacySMTPOptions(t *testing.T) {
	_, err := SendSMTPEmail(context.Background(), Config{}, Message{})
	if err == nil || !strings.Contains(err.Error(), "SMTP host") {
		t.Fatalf("expected host validation error, got %v", err)
	}
	_, err = SendSMTPEmail(context.Background(), Config{SMTPHost: "smtp.example.test", SMTPUser: "user", SMTPPass: "pass"}, Message{Content: "body"})
	if err == nil || !strings.Contains(err.Error(), "recipient") {
		t.Fatalf("expected recipient validation error, got %v", err)
	}
}

func TestBuildMessageUsesSenderNameAndPlainText(t *testing.T) {
	body := buildMessage("sender@example.test", "FarmBot", []string{"to@example.test"}, "subject", "line 1\nline 2")
	for _, want := range []string{
		"From: \"FarmBot\" <sender@example.test>",
		"To: to@example.test",
		"Subject: subject",
		"Content-Type: text/plain; charset=UTF-8",
		"line 1\r\nline 2",
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("message missing %q: %s", want, body)
		}
	}
}
