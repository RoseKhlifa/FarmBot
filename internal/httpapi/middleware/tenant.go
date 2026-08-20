package middleware

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"github.com/RoseKhlifa/FarmBot/internal/tenant"
	"github.com/gin-gonic/gin"
)

// TenantQuotaConfig wires the tenant policy into HTTP without coupling the
// middleware to a concrete account manager. ResolveTenant should return the
// tenant for the authenticated user; account paths are classified by method
// and route shape below.
type TenantQuotaConfig struct {
	Manager              *tenant.Manager
	ResolveTenant        func(context.Context, string) (string, error)
	ResolveAccountTenant func(context.Context, string) (string, error)
	Operation            func(*gin.Context) TenantOperation
}

type TenantOperation uint8

const (
	TenantOperationNone TenantOperation = iota
	TenantOperationCreate
	TenantOperationStart
)

// TenantQuota gates account creation and account starts. It fails closed when
// a protected operation has no tenant resolver or manager configured.
func TenantQuota(cfg TenantQuotaConfig) gin.HandlerFunc {
	return func(c *gin.Context) {
		op := classifyTenantOperation(c)
		if cfg.Operation != nil {
			op = cfg.Operation(c)
		}
		if op == TenantOperationNone {
			c.Next()
			return
		}
		user, ok := CurrentUser(c)
		if !ok {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"ok": false, "error": "Unauthorized"})
			return
		}
		if HasSuperAdminRole(user.Role) {
			c.Next()
			return
		}
		if cfg.Manager == nil || cfg.ResolveTenant == nil {
			c.AbortWithStatusJSON(http.StatusServiceUnavailable, gin.H{"ok": false, "error": "tenant quota is not configured"})
			return
		}
		tenantID, err := cfg.ResolveTenant(c.Request.Context(), user.Username)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"ok": false, "error": err.Error()})
			return
		}
		if op == TenantOperationStart && cfg.ResolveAccountTenant != nil {
			accountTenant, resolveErr := cfg.ResolveAccountTenant(c.Request.Context(), strings.TrimSpace(c.Param("id")))
			if resolveErr != nil || strings.TrimSpace(accountTenant) != strings.TrimSpace(tenantID) {
				if resolveErr == nil {
					resolveErr = tenant.ErrTenantRequired
				}
				c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"ok": false, "error": resolveErr.Error()})
				return
			}
		}
		var checkErr error
		switch op {
		case TenantOperationCreate:
			checkErr = cfg.Manager.CheckAccountCreate(c.Request.Context(), strings.TrimSpace(tenantID))
		case TenantOperationStart:
			checkErr = cfg.Manager.CheckAccountStart(c.Request.Context(), strings.TrimSpace(tenantID), strings.TrimSpace(c.Param("id")))
		}
		if checkErr != nil {
			if errors.Is(checkErr, tenant.ErrAccountQuotaExceeded) || errors.Is(checkErr, tenant.ErrConcurrentQuotaExceeded) {
				c.AbortWithStatusJSON(http.StatusPaymentRequired, gin.H{"ok": false, "error": checkErr.Error()})
				return
			}
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"ok": false, "error": checkErr.Error()})
			return
		}
		c.Set(CurrentTenantIDKey, strings.TrimSpace(tenantID))
		c.Next()
	}
}

// TenantMiddleware and QuotaMiddleware are descriptive aliases used by
// composition roots that group all policy middleware under one constructor.
func TenantMiddleware(cfg TenantQuotaConfig) gin.HandlerFunc { return TenantQuota(cfg) }
func QuotaMiddleware(cfg TenantQuotaConfig) gin.HandlerFunc  { return TenantQuota(cfg) }

const CurrentTenantIDKey = "currentTenantID"

func CurrentTenantID(c *gin.Context) string {
	if c == nil {
		return ""
	}
	value, _ := c.Get(CurrentTenantIDKey)
	if id, ok := value.(string); ok {
		return strings.TrimSpace(id)
	}
	return ""
}

func classifyTenantOperation(c *gin.Context) TenantOperation {
	if c == nil || c.Request == nil {
		return TenantOperationNone
	}
	path := strings.TrimSuffix(c.Request.URL.Path, "/")
	if c.Request.Method == http.MethodPost && (path == "/api/accounts" || path == "/accounts") {
		return TenantOperationCreate
	}
	if c.Request.Method == http.MethodPost && (strings.HasSuffix(path, "/start") || strings.HasSuffix(path, "/accounts/start")) {
		return TenantOperationStart
	}
	return TenantOperationNone
}
