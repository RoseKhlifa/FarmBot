package middleware

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

const DefaultRequestTimeout = 120 * time.Second

const RequestTimedOutKey = "requestTimedOut"

// Timeout installs a request context deadline and returns the legacy 503
// response when a context-aware handler finishes after that deadline.
func Timeout(timeout ...time.Duration) gin.HandlerFunc {
	limit := DefaultRequestTimeout
	if len(timeout) > 0 && timeout[0] > 0 {
		limit = timeout[0]
	}
	return func(c *gin.Context) {
		if c.Request.URL.Path == "/api/health" {
			c.Next()
			return
		}
		ctx, cancel := context.WithTimeout(c.Request.Context(), limit)
		defer cancel()
		c.Request = c.Request.WithContext(ctx)
		timer := time.AfterFunc(limit, func() {
			if c.Writer.Written() {
				return
			}
			c.Set(RequestTimedOutKey, true)
			c.AbortWithStatusJSON(http.StatusServiceUnavailable, gin.H{"ok": false, "error": "Request Timeout"})
		})
		defer timer.Stop()
		c.Next()
		if errors.Is(ctx.Err(), context.DeadlineExceeded) && !c.Writer.Written() {
			c.Set(RequestTimedOutKey, true)
			c.AbortWithStatusJSON(http.StatusServiceUnavailable, gin.H{"ok": false, "error": "Request Timeout"})
		}
	}
}

// RequestTimeoutGuard is a descriptive alias for Timeout.
func RequestTimeoutGuard(timeout ...time.Duration) gin.HandlerFunc { return Timeout(timeout...) }

func RequestTimedOut(c *gin.Context) bool {
	if c == nil {
		return false
	}
	if value, ok := c.Get(RequestTimedOutKey); ok {
		if timedOut, ok := value.(bool); ok {
			return timedOut
		}
	}
	return c.Request.Context().Err() == context.DeadlineExceeded
}
