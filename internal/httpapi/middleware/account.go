package middleware

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/RoseKhlifa/FarmBot/internal/store"
	"github.com/gin-gonic/gin"
)

const CurrentAccountIDKey = "currentAccountID"

var (
	ErrAccountReferenceMissing = errors.New("x-account-id header is required")
	ErrAccountAccessDenied     = errors.New("account access denied")
)

// AccountAccessConfig supplies account lookup and ownership persistence. The
// callback fields allow Runtime/provider account registries to be composed
// without importing those concrete implementations here.
type AccountAccessConfig struct {
	Repo            store.AccountRepo
	AccountRepo     store.AccountRepo
	Resolve         func(context.Context, string) (string, error)
	CanAccess       func(context.Context, store.User, string) (bool, error)
	AccountsForUser func(context.Context, string) ([]store.Account, error)
}

// AccountAccess checks x-account-id when present and injects its canonical ID.
// Routes that require a header should use RequireAccountAccess instead.
func AccountAccess(cfg AccountAccessConfig) gin.HandlerFunc {
	return func(c *gin.Context) {
		raw := strings.TrimSpace(c.GetHeader(AccountIDHeader))
		if raw == "" {
			c.Next()
			return
		}
		if !authorizeAccount(c, cfg, raw) {
			return
		}
		c.Next()
	}
}

// RequireAccountAccess is the account-scoped variant used by handlers that
// cannot operate without x-account-id, matching the Node 400/403 responses.
func RequireAccountAccess(cfg AccountAccessConfig) gin.HandlerFunc {
	return func(c *gin.Context) {
		raw := strings.TrimSpace(c.GetHeader(AccountIDHeader))
		if raw == "" {
			c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"ok": false, "error": "Missing x-account-id"})
			return
		}
		if !authorizeAccount(c, cfg, raw) {
			return
		}
		c.Next()
	}
}

// AccountID returns the canonical account ID injected by AccountAccess, or
// the normalized request header when the optional middleware was not used.
func AccountID(c *gin.Context) string {
	if c == nil {
		return ""
	}
	if value, ok := c.Get(CurrentAccountIDKey); ok {
		if id, ok := value.(string); ok {
			return strings.TrimSpace(id)
		}
	}
	return strings.TrimSpace(c.GetHeader(AccountIDHeader))
}

// ResolveAccountReference applies the same id/uin/qq lookup rules as Node.
func ResolveAccountReference(accounts []store.Account, reference string) string {
	target := strings.TrimSpace(reference)
	if target == "" {
		return ""
	}
	for _, account := range accounts {
		if target == strings.TrimSpace(account.ID) || target == strings.TrimSpace(account.UIN) || target == strings.TrimSpace(account.QQ) {
			return strings.TrimSpace(account.ID)
		}
	}
	return ""
}

func authorizeAccount(c *gin.Context, cfg AccountAccessConfig, raw string) bool {
	ctx := c.Request.Context()
	canonical, err := resolveAccount(ctx, cfg, raw)
	if err != nil {
		c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"ok": false, "error": err.Error()})
		return false
	}
	if canonical == "" {
		canonical = strings.TrimSpace(raw)
	}
	user, authenticated := CurrentUser(c)
	if !authenticated {
		unauthorized(c)
		return false
	}
	if !isElevatedRole(user.Role) {
		allowed, err := accountAllowed(ctx, cfg, user, canonical)
		if err != nil || !allowed {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"ok": false, "error": "无权访问此账号"})
			return false
		}
	}
	c.Set(CurrentAccountIDKey, canonical)
	return true
}

func resolveAccount(ctx context.Context, cfg AccountAccessConfig, raw string) (string, error) {
	if cfg.Resolve != nil {
		return cfg.Resolve(ctx, raw)
	}
	repo := cfg.Repo
	if repo == nil {
		repo = cfg.AccountRepo
	}
	if repo == nil {
		return strings.TrimSpace(raw), nil
	}
	accounts, err := repo.List(ctx)
	if err != nil {
		return "", fmt.Errorf("list accounts: %w", err)
	}
	return ResolveAccountReference(accounts, raw), nil
}

func accountAllowed(ctx context.Context, cfg AccountAccessConfig, user store.User, accountID string) (bool, error) {
	if cfg.CanAccess != nil {
		return cfg.CanAccess(ctx, user, accountID)
	}
	repo := cfg.Repo
	if repo == nil {
		repo = cfg.AccountRepo
	}
	if cfg.AccountsForUser != nil {
		accounts, err := cfg.AccountsForUser(ctx, user.Username)
		if err != nil {
			return false, err
		}
		return containsAccount(accounts, accountID), nil
	}
	if repo == nil {
		return false, nil
	}
	accounts, err := repo.GetByUser(ctx, user.Username)
	if err != nil {
		return false, err
	}
	return containsAccount(accounts, accountID), nil
}

func containsAccount(accounts []store.Account, accountID string) bool {
	for _, account := range accounts {
		if strings.TrimSpace(account.ID) == strings.TrimSpace(accountID) {
			return true
		}
	}
	return false
}
