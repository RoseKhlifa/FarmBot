package audit

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/RoseKhlifa/FarmBot/internal/store"
)

type Entry struct {
	ID            int64           `json:"id"`
	ActorUser     string          `json:"actorUser"`
	Action        string          `json:"action"`
	TargetAccount string          `json:"targetAccount,omitempty"`
	IP            string          `json:"ip,omitempty"`
	UserAgent     string          `json:"ua,omitempty"`
	DetailJSON    json.RawMessage `json:"detail,omitempty"`
	Timestamp     int64           `json:"ts"`
}

type Filter struct {
	Since, Until                     int64
	ActorUser, Action, TargetAccount string
	Limit, Offset                    int
}
type Repository struct{ DB *store.DB }

func (r *Repository) Append(ctx context.Context, entry Entry) error {
	if r == nil || r.DB == nil || r.DB.DB == nil {
		return errors.New("audit database is nil")
	}
	if strings.TrimSpace(entry.Action) == "" {
		return errors.New("audit action is required")
	}
	if entry.Timestamp <= 0 {
		return errors.New("audit timestamp must be supplied by caller")
	}
	detail := redact(entry.DetailJSON)
	_, err := r.DB.ExecContext(ctx, `INSERT INTO audit_log(actor_user,action,target_account,ip,ua,detail_json,ts) VALUES(?,?,?,?,?,?,?)`, entry.ActorUser, entry.Action, entry.TargetAccount, entry.IP, entry.UserAgent, string(detail), entry.Timestamp)
	if err != nil {
		return fmt.Errorf("append audit log: %w", err)
	}
	return nil
}

func (r *Repository) List(ctx context.Context, f Filter) ([]Entry, error) {
	if r == nil || r.DB == nil || r.DB.DB == nil {
		return nil, errors.New("audit database is nil")
	}
	if f.Limit <= 0 || f.Limit > 500 {
		f.Limit = 100
	}
	if f.Offset < 0 {
		f.Offset = 0
	}
	query := `SELECT id,actor_user,action,target_account,ip,ua,detail_json,ts FROM audit_log WHERE 1=1`
	args := []any{}
	if f.Since > 0 {
		query += " AND ts >= ?"
		args = append(args, f.Since)
	}
	if f.Until > 0 {
		query += " AND ts <= ?"
		args = append(args, f.Until)
	}
	if f.ActorUser != "" {
		query += " AND actor_user = ?"
		args = append(args, f.ActorUser)
	}
	if f.Action != "" {
		query += " AND action = ?"
		args = append(args, f.Action)
	}
	if f.TargetAccount != "" {
		query += " AND target_account = ?"
		args = append(args, f.TargetAccount)
	}
	query += " ORDER BY ts DESC,id DESC LIMIT ? OFFSET ?"
	args = append(args, f.Limit, f.Offset)
	rows, err := r.DB.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	result := make([]Entry, 0)
	for rows.Next() {
		var e Entry
		var detail string
		if err := rows.Scan(&e.ID, &e.ActorUser, &e.Action, &e.TargetAccount, &e.IP, &e.UserAgent, &detail, &e.Timestamp); err != nil {
			return nil, err
		}
		e.DetailJSON = json.RawMessage(detail)
		result = append(result, e)
	}
	return result, rows.Err()
}

func (r *Repository) ExportJSON(ctx context.Context, f Filter) ([]byte, error) {
	rows, err := r.List(ctx, f)
	if err != nil {
		return nil, err
	}
	return json.MarshalIndent(rows, "", "  ")
}

func redact(raw json.RawMessage) []byte {
	if len(raw) == 0 {
		return []byte("{}")
	}
	var value any
	if json.Unmarshal(raw, &value) != nil {
		return []byte("{}")
	}
	redactValue(value)
	out, err := json.Marshal(value)
	if err != nil {
		return []byte("{}")
	}
	return out
}
func redactValue(value any) {
	sensitive := map[string]bool{"password": true, "pwd": true, "token": true, "apikey": true, "api_key": true, "apitoken": true, "api_token": true, "secret": true, "code": true, "openid": true}
	switch typed := value.(type) {
	case map[string]any:
		for key, child := range typed {
			normalized := strings.ToLower(strings.ReplaceAll(key, "-", "_"))
			if sensitive[normalized] || strings.Contains(normalized, "password") || strings.Contains(normalized, "token") || strings.Contains(normalized, "secret") {
				typed[key] = "[redacted]"
			} else {
				redactValue(child)
			}
		}
	case []any:
		for _, child := range typed {
			redactValue(child)
		}
	}
}

func ActionForRequest(method, path string, status int) string {
	if method == http.MethodGet || method == http.MethodHead || status >= 500 {
		return ""
	}
	normalized := strings.Trim(path, "/")
	if normalized == "" {
		normalized = "root"
	}
	normalized = strings.ReplaceAll(normalized, "/", ".")
	return strings.ToLower(method) + " " + normalized
}
