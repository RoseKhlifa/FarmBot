package logger

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestNewRedactsSensitiveFieldsAndStrings(t *testing.T) {
	t.Setenv("LOG_LEVEL", "debug")
	t.Setenv("FARM_LOG_DIR", "")
	output := captureStdout(t, func() {
		log := New("auth")
		log.Info("request ?token=query-secret ?access_token=access-secret Bearer header-secret FARM_MASTER_KEY=master-secret",
			"token", "field-secret",
			"openid", "openid-secret",
			"nested", map[string]any{"password": "password-secret", "message": "?code=code-secret"},
		)
	})

	for _, secret := range []string{"query-secret", "access-secret", "header-secret", "master-secret", "field-secret", "openid-secret", "password-secret", "code-secret"} {
		if strings.Contains(output, secret) {
			t.Fatalf("log output contains secret %q: %s", secret, output)
		}
	}
	for _, marker := range []string{`"token":"***"`, `"openid":"***"`, `"password":"***"`, `"module":"auth"`} {
		if !strings.Contains(output, marker) {
			t.Fatalf("log output is missing %q: %s", marker, output)
		}
	}
}

func TestLogLevelConfiguration(t *testing.T) {
	t.Setenv("LOG_LEVEL", "error")
	t.Setenv("FARM_LOG_DIR", "")
	output := captureStdout(t, func() {
		log := New("level")
		log.Info("hidden")
		log.Error("visible")
	})
	if strings.Contains(output, "hidden") {
		t.Fatalf("info record was not filtered: %s", output)
	}
	if !strings.Contains(output, "visible") {
		t.Fatalf("error record was filtered: %s", output)
	}
}

func TestFarmLogDirWritesDailyFile(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("LOG_LEVEL", "info")
	t.Setenv("FARM_LOG_DIR", dir)
	captureStdout(t, func() { New("file").Info("written") })

	files, err := filepath.Glob(filepath.Join(dir, "combined-*.log"))
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 1 {
		t.Fatalf("expected one daily log file, got %d", len(files))
	}
	contents, err := os.ReadFile(files[0])
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(contents, []byte(`"msg":"written"`)) {
		t.Fatalf("file does not contain structured record: %s", contents)
	}
}

func captureStdout(t *testing.T, fn func()) string {
	t.Helper()
	previous := os.Stdout
	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stdout = writer
	defer func() {
		os.Stdout = previous
		_ = reader.Close()
	}()

	fn()
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	data, err := io.ReadAll(reader)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}
