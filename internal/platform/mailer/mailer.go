// Package mailer provides the SMTP integration used by platform reminders.
package mailer

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"mime"
	"net"
	"net/mail"
	"net/smtp"
	"strconv"
	"strings"
	"time"
)

// Config contains SMTP connection and sender settings.
type Config struct {
	SMTPHost   string
	SMTPPort   int
	SMTPUser   string
	SMTPPass   string
	SenderName string
	// TLSConfig is optional. When nil, a config using SMTPHost as ServerName
	// is created. InsecureSkipVerify should only be used for local test servers.
	TLSConfig *tls.Config
	Timeout   time.Duration
}

// SMTPConfig is an explicit alias for callers configuring email delivery.
type SMTPConfig = Config

// Message describes a plain-text email. To and Body are aliases useful to
// callers that use conventional mail terminology; RecipientEmail and Content
// retain the names of the legacy Node options.
type Message struct {
	RecipientEmail string
	To             string
	Subject        string
	Content        string
	Body           string
}

// Email is an alias for Message.
type Email = Message

// Result mirrors the common result shape of the legacy integrations.
type Result struct {
	OK      bool
	Code    string
	Message string
	Raw     any
}

// Mailer sends messages using one SMTP connection per call.
type Mailer struct{ config Config }

// New creates an SMTP mailer.
func New(config Config) *Mailer { return &Mailer{config: config} }

// Send validates and sends a plain-text email.
func (m *Mailer) Send(ctx context.Context, message Message) (Result, error) {
	if m == nil {
		return Result{}, errors.New("mailer is nil")
	}
	return SendSMTPEmail(ctx, m.config, message)
}

// SendSMTPEmail sends an email with SMTP, supporting implicit TLS on port 465
// and STARTTLS on servers that advertise it for other ports.
func SendSMTPEmail(ctx context.Context, config Config, message Message) (Result, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	host := strings.TrimSpace(config.SMTPHost)
	if host == "" {
		return Result{}, errors.New("SMTP host is required")
	}
	port := config.SMTPPort
	if port == 0 {
		port = 465
	}
	if port < 1 || port > 65535 {
		return Result{}, fmt.Errorf("SMTP port %d is invalid", port)
	}
	user := strings.TrimSpace(config.SMTPUser)
	password := strings.TrimSpace(config.SMTPPass)
	if user == "" {
		return Result{}, errors.New("SMTP user is required")
	}
	if password == "" {
		return Result{}, errors.New("SMTP password is required")
	}
	recipient := strings.TrimSpace(message.RecipientEmail)
	if recipient == "" {
		recipient = strings.TrimSpace(message.To)
	}
	if recipient == "" {
		return Result{}, errors.New("recipient email is required")
	}
	if strings.TrimSpace(message.Content) == "" && strings.TrimSpace(message.Body) == "" {
		return Result{}, errors.New("email content is required")
	}
	if err := rejectHeaderInjection(host, user, config.SenderName, recipient, message.Subject); err != nil {
		return Result{}, err
	}
	recipients, err := splitRecipients(recipient)
	if err != nil {
		return Result{}, err
	}
	body := message.Content
	if strings.TrimSpace(body) == "" {
		body = message.Body
	}
	content := buildMessage(user, config.SenderName, recipients, message.Subject, body)

	address := net.JoinHostPort(host, strconv.Itoa(port))
	timeout := config.Timeout
	if timeout <= 0 {
		timeout = 30 * time.Second
	}
	dialer := &net.Dialer{Timeout: timeout}
	connection, err := dialer.DialContext(ctx, "tcp", address)
	if err != nil {
		return Result{}, fmt.Errorf("connect SMTP server: %w", err)
	}
	defer connection.Close()
	_ = connection.SetDeadline(time.Now().Add(timeout))

	var client *smtp.Client
	if port == 465 {
		tlsConfig := cloneTLSConfig(config.TLSConfig, host)
		tlsConnection := tls.Client(connection, tlsConfig)
		if err := tlsConnection.HandshakeContext(ctx); err != nil {
			return Result{}, fmt.Errorf("SMTP TLS handshake: %w", err)
		}
		client, err = smtp.NewClient(tlsConnection, host)
	} else {
		client, err = smtp.NewClient(connection, host)
	}
	if err != nil {
		return Result{}, fmt.Errorf("initialize SMTP client: %w", err)
	}
	defer client.Close()

	if ok, _ := client.Extension("STARTTLS"); ok && port != 465 {
		if err := client.StartTLS(cloneTLSConfig(config.TLSConfig, host)); err != nil {
			return Result{}, fmt.Errorf("SMTP STARTTLS: %w", err)
		}
	}
	if ok, _ := client.Extension("AUTH"); ok {
		if err := client.Auth(smtp.PlainAuth("", user, password, host)); err != nil {
			return Result{}, fmt.Errorf("SMTP authentication: %w", err)
		}
	}
	if err := client.Mail(user); err != nil {
		return Result{}, fmt.Errorf("SMTP sender: %w", err)
	}
	for _, address := range recipients {
		if err := client.Rcpt(address); err != nil {
			return Result{}, fmt.Errorf("SMTP recipient: %w", err)
		}
	}
	writer, err := client.Data()
	if err != nil {
		return Result{}, fmt.Errorf("SMTP data: %w", err)
	}
	if _, err := writer.Write([]byte(content)); err != nil {
		_ = writer.Close()
		return Result{}, fmt.Errorf("SMTP write: %w", err)
	}
	if err := writer.Close(); err != nil {
		return Result{}, fmt.Errorf("SMTP finalize: %w", err)
	}
	if err := client.Quit(); err != nil {
		return Result{}, fmt.Errorf("SMTP quit: %w", err)
	}
	return Result{OK: true, Code: "ok", Message: "email sent"}, nil
}

// SendSmtpEmail preserves the spelling used by the legacy JavaScript module.
func SendSmtpEmail(ctx context.Context, config Config, message Message) (Result, error) {
	return SendSMTPEmail(ctx, config, message)
}

// Send is a convenience wrapper for one-off messages.
func Send(config Config, message Message) (Result, error) {
	return SendSMTPEmail(context.Background(), config, message)
}

func cloneTLSConfig(config *tls.Config, host string) *tls.Config {
	if config == nil {
		return &tls.Config{ServerName: host, MinVersion: tls.VersionTLS12}
	}
	clone := config.Clone()
	if clone.ServerName == "" {
		clone.ServerName = host
	}
	if clone.MinVersion == 0 {
		clone.MinVersion = tls.VersionTLS12
	}
	return clone
}

func splitRecipients(raw string) ([]string, error) {
	parts := strings.FieldsFunc(raw, func(r rune) bool { return r == ',' || r == ';' })
	if len(parts) == 0 {
		return nil, errors.New("recipient email is required")
	}
	recipients := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		parsed, err := mail.ParseAddress(part)
		if err != nil || parsed.Address == "" {
			return nil, fmt.Errorf("invalid recipient email %q", part)
		}
		recipients = append(recipients, parsed.Address)
	}
	if len(recipients) == 0 {
		return nil, errors.New("recipient email is required")
	}
	return recipients, nil
}

func buildMessage(user, senderName string, recipients []string, subject, content string) string {
	from := (&mail.Address{Name: strings.TrimSpace(senderName), Address: user}).String()
	if strings.TrimSpace(senderName) == "" {
		from = user
	}
	encodedSubject := mime.QEncoding.Encode("UTF-8", strings.TrimSpace(subject))
	if encodedSubject == "" {
		encodedSubject = "下线提醒"
	}
	return "From: " + from + "\r\n" +
		"To: " + strings.Join(recipients, ", ") + "\r\n" +
		"Subject: " + encodedSubject + "\r\n" +
		"MIME-Version: 1.0\r\n" +
		"Content-Type: text/plain; charset=UTF-8\r\n" +
		"Content-Transfer-Encoding: 8bit\r\n" +
		"\r\n" + strings.ReplaceAll(strings.ReplaceAll(content, "\r\n", "\n"), "\n", "\r\n") + "\r\n"
}

func rejectHeaderInjection(values ...string) error {
	for _, value := range values {
		if strings.ContainsAny(value, "\r\n") {
			return errors.New("SMTP header contains newline")
		}
	}
	return nil
}
