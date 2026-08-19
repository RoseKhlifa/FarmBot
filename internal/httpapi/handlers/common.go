package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/RoseKhlifa/FarmBot/internal/domain/activity"
	"github.com/RoseKhlifa/FarmBot/internal/domain/career"
	"github.com/RoseKhlifa/FarmBot/internal/domain/farm"
	"github.com/RoseKhlifa/FarmBot/internal/domain/friend"
	"github.com/RoseKhlifa/FarmBot/internal/domain/illustrated"
	"github.com/RoseKhlifa/FarmBot/internal/domain/mall"
	"github.com/RoseKhlifa/FarmBot/internal/domain/social"
	"github.com/RoseKhlifa/FarmBot/internal/domain/task"
	"github.com/RoseKhlifa/FarmBot/internal/domain/warehouse"
	"github.com/RoseKhlifa/FarmBot/internal/httpapi/middleware"
	"github.com/RoseKhlifa/FarmBot/internal/store"
	"github.com/gin-gonic/gin"
)

var ErrApplicationDependency = errors.New("application dependency is not configured")

// DomainProviders is the only domain boundary used by handlers. The resolver
// returns the account-local service, keeping Runtime maps out of HTTP code.
type DomainProviders struct {
	Farm        func(context.Context, string) (farm.Service, error)
	Friend      func(context.Context, string) (friend.Service, error)
	Warehouse   func(context.Context, string) (warehouse.Service, error)
	Mall        func(context.Context, string) (*mall.Domains, error)
	Task        func(context.Context, string) (task.Service, error)
	Activity    func(context.Context, string) (activity.Service, error)
	Career      func(context.Context, string) (career.Service, error)
	Illustrated func(context.Context, string) (illustrated.Service, error)
	Social      func(context.Context, string) (social.Service, error)
}

type AccountProvider interface {
	List(context.Context) ([]store.Account, error)
	Start(context.Context, string) error
	Stop(context.Context, string) error
	Delete(context.Context, string) error
	SetRemark(context.Context, string, string) error
}

// Application is the handler-facing composition boundary. P6-05 can adapt
// its concrete Application to this value without changing route handlers.
type Application struct {
	Domains  DomainProviders
	Accounts AccountProvider
	Cache    store.CacheRepo
	Config   store.ConfigRepo
	Users    store.UserRepo
	Cards    store.CardRepo
	Sessions *middleware.SessionManager
	Auth     AuthProvider
	Runtime  RuntimeProvider
	Logs     LogProvider
	Yyb      YybProvider
	Public   PublicProvider
	Capture  CaptureProvider
	QR       QRProvider
	Proxy    ProxyProvider
}

type AuthProvider interface {
	Login(context.Context, string, string) (store.User, error)
	Register(context.Context, string, string) (store.User, error)
}
type RuntimeProvider interface {
	Status(context.Context, string) (any, error)
	Automation(context.Context, string, json.RawMessage) (any, error)
	Scheduler(context.Context, string) (any, error)
}
type LogProvider interface {
	Logs(context.Context, string, int) (any, error)
	AccountLogs(context.Context, string, int) (any, error)
	ClearLogs(context.Context, string) error
}
type YybProvider interface {
	Handle(context.Context, string, map[string]any) (any, error)
}
type PublicProvider interface {
	Value(context.Context, string) (any, error)
}
type CaptureProvider interface {
	Handle(context.Context, string, map[string]any) (any, error)
}
type QRProvider interface {
	Handle(context.Context, string, map[string]any) (any, error)
}
type ProxyProvider interface {
	Handle(context.Context, map[string]any) (any, error)
}

type Handler struct{ App *Application }

func New(app *Application) *Handler { return &Handler{App: app} }

func (h *Handler) app() *Application {
	if h == nil || h.App == nil {
		return &Application{}
	}
	return h.App
}

func accountID(c *gin.Context, required bool) (string, bool) {
	id := strings.TrimSpace(middleware.AccountID(c))
	if id == "" && required {
		c.JSON(http.StatusBadRequest, gin.H{"ok": false, "error": "Missing x-account-id"})
		return "", false
	}
	return id, true
}

func resolve[T any](c *gin.Context, resolver func(context.Context, string) (T, error), id string) (T, bool) {
	var zero T
	if resolver == nil {
		c.JSON(http.StatusNotImplemented, gin.H{"ok": false, "error": ErrApplicationDependency.Error()})
		return zero, false
	}
	service, err := resolver(c.Request.Context(), id)
	if err != nil {
		writeError(c, err)
		return zero, false
	}
	return service, true
}

func writeOK(c *gin.Context, data any) {
	if data == nil {
		c.JSON(http.StatusOK, gin.H{"ok": true})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true, "data": data})
}
func writeError(c *gin.Context, err error) {
	if err == nil {
		return
	}
	c.JSON(http.StatusInternalServerError, gin.H{"ok": false, "error": err.Error()})
}
func writeNotConfigured(c *gin.Context) {
	c.JSON(http.StatusNotImplemented, gin.H{"ok": false, "error": ErrApplicationDependency.Error()})
}
func bindJSON(c *gin.Context, target any) bool {
	if err := c.ShouldBindJSON(target); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"ok": false, "error": err.Error()})
		return false
	}
	return true
}
func int64Param(c *gin.Context, name string) (int64, bool) {
	value, err := strconv.ParseInt(strings.TrimSpace(c.Param(name)), 10, 64)
	if err != nil || value <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"ok": false, "error": fmt.Sprintf("invalid %s", name)})
		return 0, false
	}
	return value, true
}
func queryInt(c *gin.Context, name string, fallback int) int {
	value, err := strconv.Atoi(c.Query(name))
	if err != nil || value < 0 {
		return fallback
	}
	return value
}

// RequireAccountAccess is kept as a handler-local helper so every account
// route has identical 400 semantics even when the middleware was omitted in
// a focused test router.
func RequireAccountAccess(c *gin.Context) (string, bool) { return accountID(c, true) }
