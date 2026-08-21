package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/RoseKhlifa/FarmBot/internal/audit"
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

// HTTPError lets providers preserve the intended API status while keeping
// the handler boundary independent from concrete provider implementations.
type HTTPError struct {
	Status int
	Code   string
	Err    error
}

func (e *HTTPError) Error() string {
	if e == nil || e.Err == nil {
		return ""
	}
	return e.Err.Error()
}

func (e *HTTPError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Err
}

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
	Domains       DomainProviders
	Accounts      AccountProvider
	Cache         store.CacheRepo
	Config        store.ConfigRepo
	Users         store.UserRepo
	Cards         store.CardRepo
	Sessions      *middleware.SessionManager
	Auth          AuthProvider
	Runtime       RuntimeProvider
	Logs          LogProvider
	Yyb           YybProvider
	Public        PublicProvider
	Capture       CaptureProvider
	QR            QRProvider
	Proxy         ProxyProvider
	Audit         *audit.Repository
	ExportAccount func(context.Context, string) ([]byte, error)
}

type AuthProvider interface {
	Login(context.Context, string, string, string) (store.User, error)
	Register(context.Context, string, string) (store.User, error)
}

type AdminUserUpdater interface {
	UpdateAdminUser(context.Context, string, store.AdminUserPatch) (*store.User, error)
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

// BinaryResponse is returned by providers that proxy a binary download
// (currently the capture-service CA certificate) instead of a JSON envelope.
type BinaryResponse struct {
	Data        []byte
	ContentType string
	Filename    string
}

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
	status := http.StatusInternalServerError
	var httpErr *HTTPError
	if errors.As(err, &httpErr) && httpErr != nil && httpErr.Status >= 400 && httpErr.Status <= 599 {
		status = httpErr.Status
	}
	switch {
	case status != http.StatusInternalServerError:
		// Provider supplied an explicit API status.
	case errors.Is(err, store.ErrRateLimited):
		status = http.StatusTooManyRequests
	case errors.Is(err, store.ErrUserLocked):
		status = http.StatusLocked
	case errors.Is(err, store.ErrUserExpired):
		status = http.StatusForbidden
	case errors.Is(err, store.ErrUserDisabled):
		status = http.StatusForbidden
	case errors.Is(err, store.ErrUserNotFound), errors.Is(err, store.ErrCardNotFound):
		status = http.StatusNotFound
	case errors.Is(err, store.ErrUserExists), errors.Is(err, store.ErrInvalidUsername), errors.Is(err, store.ErrInvalidPassword),
		errors.Is(err, store.ErrCardExists), errors.Is(err, store.ErrCardUnavailable), errors.Is(err, store.ErrCardAlreadyBound), errors.Is(err, store.ErrInvalidCard):
		status = http.StatusBadRequest
	}
	response := gin.H{"ok": false, "error": err.Error()}
	if httpErr != nil && strings.TrimSpace(httpErr.Code) != "" {
		response["code"] = strings.TrimSpace(httpErr.Code)
	}
	c.JSON(status, response)
}
func writeNotConfigured(c *gin.Context) {
	c.JSON(http.StatusNotImplemented, gin.H{"ok": false, "error": ErrApplicationDependency.Error()})
}

// publicUser is the stable user shape consumed by the web client. store.User
// intentionally contains password compatibility fields for persistence and
// authentication; those fields must never cross the HTTP boundary.
func publicUser(user store.User) gin.H {
	return gin.H{
		"username":           user.Username,
		"role":               user.Role,
		"card":               publicCard(user.CardJSON),
		"accountLimit":       publicAccountLimit(user.AccountLimit),
		"mustChangePassword": user.MustChangePassword,
	}
}

func publicUsers(users []store.User) []gin.H {
	result := make([]gin.H, 0, len(users))
	for _, user := range users {
		result = append(result, publicUser(user))
	}
	return result
}

func publicCard(raw string) any {
	raw = strings.TrimSpace(raw)
	if raw == "" || raw == "{}" || raw == "null" {
		return nil
	}
	var card map[string]any
	if err := json.Unmarshal([]byte(raw), &card); err != nil || len(card) == 0 {
		return nil
	}
	return card
}

func publicAccountLimit(limit int) int {
	if limit <= 0 {
		return 2
	}
	return limit
}

func publicRenewal(user *store.User) gin.H {
	if user == nil {
		return gin.H{"card": nil, "accountLimit": 2, "cardType": "time"}
	}
	return gin.H{
		"card":         publicCard(user.CardJSON),
		"accountLimit": publicAccountLimit(user.AccountLimit),
		"cardType":     "time",
	}
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

// requireAccountOwner protects path-based account mutations. The global
// AccountAccess middleware validates the selected x-account-id header, but a
// mutation also carries an account ID in its URL and must validate that exact
// target.
func (h *Handler) requireAccountOwner(c *gin.Context, id string) bool {
	id = strings.TrimSpace(id)
	if id == "" {
		c.JSON(http.StatusBadRequest, gin.H{"ok": false, "error": "invalid account id"})
		return false
	}
	user, ok := middleware.CurrentUser(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"ok": false, "error": "Unauthorized"})
		return false
	}
	if middleware.HasAdminRole(user.Role) {
		return true
	}
	if h.app().Accounts == nil {
		writeNotConfigured(c)
		return false
	}
	accounts, err := h.app().Accounts.List(c.Request.Context())
	if err != nil {
		writeError(c, err)
		return false
	}
	for _, account := range accounts {
		if strings.TrimSpace(account.ID) == id && strings.EqualFold(strings.TrimSpace(account.OwnerUser), strings.TrimSpace(user.Username)) {
			return true
		}
	}
	c.JSON(http.StatusForbidden, gin.H{"ok": false, "error": "无权访问此账号"})
	return false
}
