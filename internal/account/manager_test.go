package account

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/RoseKhlifa/FarmBot/internal/store"
)

func TestManagerRetriesYYBPermissionWithFreshCode(t *testing.T) {
	var (
		codeCalls  int
		startCalls int
		codes      []string
	)

	manager := NewManager(ManagerConfig{
		Load: func(context.Context, string) (store.Account, *store.AccountConfig, error) {
			return store.Account{ID: "account-1", LoginType: "yyb", YYBOpenID: "wx-openid"}, nil, nil
		},
		CodeProvider: func(context.Context, store.Account, *store.AccountConfig, string) (string, error) {
			codeCalls++
			return fmt.Sprintf("fresh-code-%d", codeCalls), nil
		},
		RuntimeFactory: func(spec RuntimeSpec) *Runtime {
			codes = append(codes, spec.RuntimeConfig.LoginCode)
			return NewRuntime(spec.RuntimeConfig, spec.Dependencies)
		},
		StartRuntime: func(context.Context, *Runtime) error {
			startCalls++
			if startCalls == 1 {
				return errors.New("game login handshake: code=1000016 权限不足，不能登录")
			}
			return nil
		},
		Reconnect: ReconnectConfig{PollInterval: time.Hour},
	})
	defer manager.Close()

	if err := manager.Start("account-1"); err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	if codeCalls != 2 || startCalls != 2 {
		t.Fatalf("refresh/start calls = %d/%d, want 2/2", codeCalls, startCalls)
	}
	if len(codes) != 2 || codes[0] != "fresh-code-1" || codes[1] != "fresh-code-2" {
		t.Fatalf("login codes = %#v, want two fresh codes", codes)
	}
	if manager.Get("account-1") == nil {
		t.Fatal("successful retry was not registered")
	}
}

func TestGameLoginPermissionErrorMatcher(t *testing.T) {
	if !isGameLoginPermissionError(errors.New("code=1000016 权限不足")) {
		t.Fatal("1000016 was not recognized")
	}
	if isGameLoginPermissionError(errors.New("code=1000017 other error")) {
		t.Fatal("different gateway error was recognized as 1000016")
	}
}
