package middleware

import (
	"context"
	"time"

	"github.com/RoseKhlifa/FarmBot/internal/audit"
	"github.com/gin-gonic/gin"
)

type auditTimestampKey struct{}

func AuditTimestamp(ctx context.Context) (int64, bool) {
	value, ok := ctx.Value(auditTimestampKey{}).(int64)
	return value, ok
}

func AuditLog(repo *audit.Repository) gin.HandlerFunc {
	return func(c *gin.Context) {
		timestamp := time.Now().UnixMilli()
		c.Request = c.Request.WithContext(context.WithValue(c.Request.Context(), auditTimestampKey{}, timestamp))
		c.Next()
		if repo == nil {
			return
		}
		action := audit.ActionForRequest(c.Request.Method, c.Request.URL.Path, c.Writer.Status())
		if action == "" {
			return
		}
		user, _ := CurrentUser(c)
		if user.Username == "" {
			return
		}
		_ = repo.Append(c.Request.Context(), audit.Entry{ActorUser: user.Username, Action: action, TargetAccount: AccountID(c), IP: c.ClientIP(), UserAgent: c.GetHeader("User-Agent"), DetailJSON: []byte(`{"status":` + itoa(c.Writer.Status()) + `}`), Timestamp: timestamp})
	}
}

func itoa(value int) string {
	if value == 0 {
		return "0"
	}
	digits := [20]byte{}
	i := len(digits)
	for value > 0 {
		i--
		digits[i] = byte('0' + value%10)
		value /= 10
	}
	return string(digits[i:])
}
