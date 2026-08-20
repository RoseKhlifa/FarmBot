package app

import (
	"context"
	"errors"
	"sync"

	"github.com/RoseKhlifa/FarmBot/internal/account"
	"github.com/RoseKhlifa/FarmBot/internal/config"
	"github.com/RoseKhlifa/FarmBot/internal/httpapi"
	"github.com/RoseKhlifa/FarmBot/internal/httpapi/middleware"
	"github.com/RoseKhlifa/FarmBot/internal/httpapi/realtime"
	platformmetrics "github.com/RoseKhlifa/FarmBot/internal/platform/metrics"
	"github.com/RoseKhlifa/FarmBot/internal/store"
	"github.com/RoseKhlifa/FarmBot/internal/yyb"
)

var ErrApplicationClosed = errors.New("application is closed")

// Application is the process-wide composition root. It owns every resource
// created during startup and releases them in reverse dependency order.
type Application struct {
	Config config.Config

	DB       *store.DB
	Accounts *store.SQLiteAccountRepo
	Cache    *store.SQLiteCacheRepo
	ConfigDB *store.SQLiteConfigRepo
	Stats    *store.SQLiteStatsRepo
	Users    *store.SQLiteUserRepo
	Cards    *store.SQLiteCardRepo

	YybDB          *yyb.DB
	Yyb            yyb.Service
	AccountManager *account.Manager
	Sessions       *middleware.SessionManager
	Realtime       *realtime.Hub
	Metrics        *platformmetrics.Registry
	Server         *httpapi.Server

	mu     sync.Mutex
	closed bool
}

// New opens the shared database and wires the in-process services. It does
// not start a listener; callers control the lifecycle through Run/Shutdown.
func New(cfg config.Config) (*Application, error) {
	return newApplication(cfg)
}

// Run starts the HTTP server and returns when ctx is cancelled or the server
// exits. The server itself drains active requests on context cancellation.
func (a *Application) Run(ctx context.Context) error {
	if a == nil || a.Server == nil {
		return errors.New("application server is not configured")
	}
	a.mu.Lock()
	closed := a.closed
	a.mu.Unlock()
	if closed {
		return ErrApplicationClosed
	}
	return a.Server.Run(ctx)
}

// Shutdown stops account runtimes, realtime clients, sessions, HTTP, and the
// SQLite connection. It is safe to call repeatedly.
func (a *Application) Shutdown(ctx context.Context) error {
	if a == nil {
		return nil
	}
	a.mu.Lock()
	if a.closed {
		a.mu.Unlock()
		return nil
	}
	a.closed = true
	a.mu.Unlock()

	var firstErr error
	if a.AccountManager != nil {
		if err := a.AccountManager.Close(); err != nil {
			firstErr = err
		}
	}
	if a.Realtime != nil {
		if err := a.Realtime.Close(); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	if a.Sessions != nil {
		a.Sessions.Close()
	}
	if a.Server != nil {
		if err := a.Server.Shutdown(ctx); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	if a.DB != nil {
		if err := a.DB.Close(); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}

// Close is the conventional lifecycle alias.
func (a *Application) Close() error { return a.Shutdown(context.Background()) }
