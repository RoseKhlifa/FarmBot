package yyb

import (
	"context"
	"database/sql"
	"errors"
	"testing"

	"github.com/RoseKhlifa/FarmBot/internal/yyb/protocol"
	"github.com/RoseKhlifa/FarmBot/internal/yyb/qr"
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

type retryPool struct {
	calls       int
	loginBuffer []string
}

type nestedCodePool struct{}

func (nestedCodePool) GetCode(context.Context, string, string, int64, string) (map[string]any, error) {
	return map[string]any{"data": map[string]any{
		"result": map[string]any{"wx_code": "nested-code-1"},
	}}, nil
}

func (nestedCodePool) GetPhoneNumber(context.Context, string, string, int64, string) (map[string]any, error) {
	return nil, errors.New("not implemented")
}

func (nestedCodePool) OperateWXData(context.Context, string, string, map[string]any, int64, string) (map[string]any, error) {
	return nil, errors.New("not implemented")
}

func (p *retryPool) GetCode(_ context.Context, loginBuffer, _ string, _ int64, _ string) (map[string]any, error) {
	p.calls++
	p.loginBuffer = append(p.loginBuffer, loginBuffer)
	if p.calls == 1 {
		return nil, errors.New("stale session")
	}
	return map[string]any{"code": "fresh-code"}, nil
}

func (*retryPool) GetPhoneNumber(context.Context, string, string, int64, string) (map[string]any, error) {
	return nil, errors.New("not implemented")
}

func (*retryPool) OperateWXData(context.Context, string, string, map[string]any, int64, string) (map[string]any, error) {
	return nil, errors.New("not implemented")
}

type refreshQRClient struct {
	refreshed bool
}

func (*refreshQRClient) GetQRCodeImage(context.Context) (qr.ImageResult, error) {
	return qr.ImageResult{}, errors.New("not implemented")
}

func (*refreshQRClient) PollQRCode(context.Context, *qr.Session) (qr.PollResult, error) {
	return qr.PollResult{}, errors.New("not implemented")
}

func (*refreshQRClient) GetLoginBuffer(context.Context, *qr.Session) (protocol.LoginBufferResult, error) {
	return protocol.LoginBufferResult{}, errors.New("not implemented")
}

func (c *refreshQRClient) RefreshLoginBuffer(_ context.Context, creds protocol.LoginBufferCredentials) (protocol.LoginBufferResult, error) {
	c.refreshed = true
	creds.AccessToken = "fresh-access-token"
	return protocol.LoginBufferResult{LoginBuffer: "fresh-buffer", Credentials: creds}, nil
}

func (*refreshQRClient) LoginBuffers() *protocol.LoginBufferClient { return nil }

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

func TestServiceGetCodeAcceptsNestedResponse(t *testing.T) {
	_, db := openSharedTestDB(t)
	ctx := context.Background()
	if _, err := db.UpsertAccount(ctx, "openid-nested", "buffer", nil, nil, nil, nil, nil, nil); err != nil {
		t.Fatal(err)
	}
	service, err := NewService(db, ServiceConfig{Pool: nestedCodePool{}})
	if err != nil {
		t.Fatal(err)
	}
	code, err := service.GetCode(ctx, "openid-nested", "")
	if err != nil || code != "nested-code-1" {
		t.Fatalf("GetCode() = %q, %v", code, err)
	}
}

func TestServiceGetCodeInvalidatesSessionBeforeRefreshingBuffer(t *testing.T) {
	_, db := openSharedTestDB(t)
	ctx := context.Background()
	account, err := db.UpsertAccount(ctx, "openid-retry", "oauth-buffer", nil, nil, nil, nil, map[string]any{
		"openid":       "openid-retry",
		"accesstoken":  "access-token",
		"refreshtoken": "refresh-token",
		"logintype":    "WX",
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := db.PutSession(ctx, account.ID, nil, map[string]any{"stale": true}, 4102444800, ""); err != nil {
		t.Fatal(err)
	}

	pool := &retryPool{}
	qrClient := &refreshQRClient{}
	service, err := NewService(db, ServiceConfig{Pool: pool, QRClient: qrClient})
	if err != nil {
		t.Fatal(err)
	}
	code, err := service.GetCode(ctx, account.OpenID, "")
	if err != nil || code != "fresh-code" {
		t.Fatalf("GetCode() = %q, %v", code, err)
	}
	if pool.calls != 2 || len(pool.loginBuffer) != 2 || pool.loginBuffer[0] != "oauth-buffer" || pool.loginBuffer[1] != "fresh-buffer" {
		t.Fatalf("unexpected retry sequence: calls=%d buffers=%q", pool.calls, pool.loginBuffer)
	}
	if !qrClient.refreshed {
		t.Fatal("GetCode did not refresh the login buffer")
	}
	if _, err := db.GetSession(ctx, account.ID, ""); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("stale session was not invalidated: %v", err)
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
