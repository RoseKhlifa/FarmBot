package yyb

import (
	"context"
	"database/sql"
	"testing"
)

type fakePool struct{}

func (fakePool) GetCode(context.Context, string, string, int64, string) (map[string]any, error) {
	return map[string]any{"code": "code-1"}, nil
}

func (fakePool) GetPhoneNumber(context.Context, string, string, int64, string) (map[string]any, error) {
	return map[string]any{"phone": "13800000000"}, nil
}

func (fakePool) OperateWXData(context.Context, string, string, map[string]any, int64, string) (map[string]any, error) {
	return map[string]any{"ok": true}, nil
}

func TestServiceGetCodeUsesInjectableProtocolPool(t *testing.T) {
	_, db := openSharedTestDB(t)
	ctx := context.Background()
	if _, err := db.UpsertAccount(ctx, "openid-code", "buffer", nil, nil, nil, nil, nil, nil); err != nil {
		t.Fatal(err)
	}
	service, err := NewService(db, ServiceConfig{Pool: fakePool{}})
	if err != nil {
		t.Fatal(err)
	}
	code, err := service.GetCode(ctx, "openid-code", "")
	if err != nil || code != "code-1" {
		t.Fatalf("GetCode() = %q, %v", code, err)
	}
}

func TestServiceSharesStoreAndResolvesAccounts(t *testing.T) {
	_, db := openSharedTestDB(t)
	ctx := context.Background()
	account, err := db.UpsertAccount(ctx, "openid-service", "buffer", nil, nil, nil, nil, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	service, err := NewService(db, ServiceConfig{})
	if err != nil {
		t.Fatal(err)
	}
	accounts, err := service.ListAccounts(ctx)
	if err != nil || len(accounts) != 1 || accounts[0].ID != account.ID {
		t.Fatalf("ListAccounts() = %#v, %v", accounts, err)
	}
	removed, err := service.DeleteAccount(ctx, "openid-service")
	if err != nil || removed.ID != account.ID {
		t.Fatalf("DeleteAccount() = %#v, %v", removed, err)
	}
	if _, err := service.DeleteAccount(ctx, "openid-service"); err != sql.ErrNoRows {
		t.Fatalf("missing account delete = %v, want sql.ErrNoRows", err)
	}
}

func TestServiceRejectsInvalidQRSession(t *testing.T) {
	_, db := openSharedTestDB(t)
	service, err := NewService(db, ServiceConfig{})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.QRPoll(context.Background(), ""); err == nil {
		t.Fatal("QRPoll accepted an empty session id")
	}
	if _, err := service.QRConfirm(context.Background(), "missing"); err == nil {
		t.Fatal("QRConfirm accepted an unknown session id")
	}
}
