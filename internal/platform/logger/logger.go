// Package logger provides structured application logging with credential
// redaction at the handler boundary.
package logger

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"strings"
	"sync"
	"time"
)

const (
	// Redacted is used for values that must not be written to a log sink.
	Redacted = "***"
	// truncatedValue prevents unexpectedly deep values from producing a very
	// large log record while retaining the shape of the original metadata.
	truncatedValue = "[Truncated]"
)

var (
	sensitiveKeyRE     = regexp.MustCompile(`(?i)(?:code|token|password|passwd|auth|authorization|ticket|cookie|session|openid|login[_-]?buffer|farm[_-]?master[_-]?key|master[_-]?key|secret|credential)`)
	querySecretRE      = regexp.MustCompile(`(?i)([?&](?:access[_-]?token|refresh[_-]?token|id[_-]?token|code|token|password|passwd|auth|authorization|ticket|cookie|session|openid|login[_-]?buffer|farm[_-]?master[_-]?key|master[_-]?key|secret|credential)=)[^&\s]+`)
	assignmentSecretRE = regexp.MustCompile(`(?i)(\b(?:access[_-]?token|refresh[_-]?token|id[_-]?token|code|token|password|passwd|auth|authorization|ticket|cookie|session|openid|login[_-]?buffer|farm[_-]?master[_-]?key|master[_-]?key|secret|credential)\b\s*[:=]\s*)[^\s,;&]+`)
	bearerTokenRE      = regexp.MustCompile(`(?i)(Bearer\s+)[\w.-]+`)
)

// New creates a module-scoped structured logger. Configuration is read from
// LOG_LEVEL and FARM_LOG_DIR when the logger is created. Every logger writes
// JSON records to stdout; setting FARM_LOG_DIR additionally writes a combined
// file whose name changes once per calendar day.
func New(module string) *slog.Logger {
	module = strings.TrimSpace(module)
	if module == "" {
		module = "app"
	}
	return slog.New(newHandler()).With("module", module)
}

// RedactString removes credentials from URLs, authorization headers, and
// common key/value strings before they reach a log sink.
func RedactString(raw string) string {
	result := raw
	result = querySecretRE.ReplaceAllString(result, `${1}`+Redacted)
	result = bearerTokenRE.ReplaceAllString(result, `${1}`+Redacted)
	result = assignmentSecretRE.ReplaceAllString(result, `${1}`+Redacted)
	return result
}

// SanitizeMeta returns a recursively sanitized copy of metadata. Maps,
// slices, arrays, and exported struct fields are traversed up to four levels.
func SanitizeMeta(meta any) any { return sanitizeAny(meta, 0) }

func newHandler() slog.Handler {
	writers := []io.Writer{os.Stdout}
	if dir := strings.TrimSpace(os.Getenv("FARM_LOG_DIR")); dir != "" {
		if fileWriter, err := newDailyFileWriter(dir); err == nil {
			writers = append(writers, fileWriter)
		}
	}

	var output io.Writer = writers[0]
	if len(writers) > 1 {
		output = io.MultiWriter(writers...)
	}
	base := slog.NewJSONHandler(output, &slog.HandlerOptions{Level: configuredLevel()})
	return &redactingHandler{Handler: base}
}

func configuredLevel() slog.Level {
	switch strings.ToLower(strings.TrimSpace(os.Getenv("LOG_LEVEL"))) {
	case "debug":
		return slog.LevelDebug
	case "warn", "warning":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}

// redactingHandler sanitizes records after slog has evaluated the call but
// before the wrapped handler serializes them. WithAttrs is sanitized too so
// values attached through Logger.With cannot bypass the boundary.
type redactingHandler struct{ slog.Handler }

func (h *redactingHandler) Handle(ctx context.Context, record slog.Record) error {
	sanitized := slog.NewRecord(record.Time, record.Level, RedactString(record.Message), record.PC)
	record.Attrs(func(attr slog.Attr) bool {
		sanitized.AddAttrs(sanitizeAttr(attr, 0))
		return true
	})
	return h.Handler.Handle(ctx, sanitized)
}

func (h *redactingHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	sanitized := make([]slog.Attr, len(attrs))
	for i, attr := range attrs {
		sanitized[i] = sanitizeAttr(attr, 0)
	}
	return &redactingHandler{Handler: h.Handler.WithAttrs(sanitized)}
}

func (h *redactingHandler) WithGroup(name string) slog.Handler {
	return &redactingHandler{Handler: h.Handler.WithGroup(name)}
}

func sanitizeAttr(attr slog.Attr, depth int) slog.Attr {
	if attr.Key != "" && sensitiveKeyRE.MatchString(attr.Key) {
		return slog.String(attr.Key, Redacted)
	}
	attr.Value = sanitizeValue(attr.Value, depth)
	return attr
}

func sanitizeValue(value slog.Value, depth int) slog.Value {
	if depth > 4 {
		return slog.StringValue(truncatedValue)
	}
	switch value.Kind() {
	case slog.KindString:
		return slog.StringValue(RedactString(value.String()))
	case slog.KindGroup:
		attrs := value.Group()
		for i := range attrs {
			attrs[i] = sanitizeAttr(attrs[i], depth+1)
		}
		return slog.GroupValue(attrs...)
	case slog.KindAny:
		return slog.AnyValue(sanitizeAny(value.Any(), depth+1))
	case slog.KindLogValuer:
		return sanitizeValue(value.Resolve(), depth+1)
	default:
		return value
	}
}

func sanitizeAny(value any, depth int) any {
	if depth > 4 || value == nil {
		if depth > 4 {
			return truncatedValue
		}
		return nil
	}
	return sanitizeReflect(reflect.ValueOf(value), depth)
}

func sanitizeReflect(value reflect.Value, depth int) any {
	if !value.IsValid() {
		return nil
	}
	if depth > 4 {
		return truncatedValue
	}

	switch value.Kind() {
	case reflect.Interface, reflect.Pointer:
		if value.IsNil() {
			return nil
		}
		return sanitizeReflect(value.Elem(), depth+1)
	case reflect.String:
		return RedactString(value.String())
	case reflect.Bool, reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr,
		reflect.Float32, reflect.Float64:
		if value.CanInterface() {
			return value.Interface()
		}
		return nil
	case reflect.Slice, reflect.Array:
		result := make([]any, value.Len())
		for i := 0; i < value.Len(); i++ {
			result[i] = sanitizeReflect(value.Index(i), depth+1)
		}
		return result
	case reflect.Map:
		if value.IsNil() {
			return nil
		}
		if value.Type().Key().Kind() == reflect.String {
			result := make(map[string]any, value.Len())
			iter := value.MapRange()
			for iter.Next() {
				key := iter.Key().String()
				if sensitiveKeyRE.MatchString(key) {
					result[key] = Redacted
					continue
				}
				result[key] = sanitizeReflect(iter.Value(), depth+1)
			}
			return result
		}
		result := make(map[any]any, value.Len())
		iter := value.MapRange()
		for iter.Next() {
			key := iter.Key()
			if key.Kind() == reflect.String && sensitiveKeyRE.MatchString(key.String()) {
				result[key.Interface()] = Redacted
				continue
			}
			if key.CanInterface() {
				result[key.Interface()] = sanitizeReflect(iter.Value(), depth+1)
			}
		}
		return result
	case reflect.Struct:
		if value.CanInterface() {
			if err, ok := value.Interface().(error); ok {
				return RedactString(err.Error())
			}
		}
		result := make(map[string]any)
		typ := value.Type()
		for i := 0; i < value.NumField(); i++ {
			field := typ.Field(i)
			if field.PkgPath != "" { // unexported
				continue
			}
			name := field.Name
			if tag := field.Tag.Get("json"); tag != "" {
				if comma := strings.IndexByte(tag, ','); comma >= 0 {
					tag = tag[:comma]
				}
				if tag == "-" {
					continue
				}
				if tag != "" {
					name = tag
				}
			}
			if sensitiveKeyRE.MatchString(name) {
				result[name] = Redacted
				continue
			}
			result[name] = sanitizeReflect(value.Field(i), depth+1)
		}
		return result
	default:
		if value.CanInterface() {
			return fmt.Sprint(value.Interface())
		}
		return nil
	}
}

type dailyFileWriter struct {
	mu  sync.Mutex
	dir string
}

func newDailyFileWriter(dir string) (*dailyFileWriter, error) {
	dir, err := filepath.Abs(dir)
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	return &dailyFileWriter{dir: filepath.Clean(dir)}, nil
}

func (w *dailyFileWriter) Write(data []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()

	day := time.Now().Format("2006-01-02")
	file, err := os.OpenFile(filepath.Join(w.dir, "combined-"+day+".log"), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		return 0, err
	}
	defer file.Close()
	return file.Write(data)
}

func (w *dailyFileWriter) Close() error { return nil }
