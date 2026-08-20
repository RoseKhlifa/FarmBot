package yyb

import (
	"bytes"
	"context"
	"errors"
	"os"
	"testing"

	secretcrypto "github.com/RoseKhlifa/FarmBot/internal/crypto"
)

func TestNewDBFromEnvFailsClosedWithoutMasterKey(t *testing.T) {
	old, present := os.LookupEnv(secretcrypto.MasterKeyEnv)
	t.Cleanup(func() {
		if present {
			_ = os.Setenv(secretcrypto.MasterKeyEnv, old)
		} else {
			_ = os.Unsetenv(secretcrypto.MasterKeyEnv)
		}
	})
	_ = os.Unsetenv(secretcrypto.MasterKeyEnv)
	if _, err := NewDBFromEnv(nil); !errors.Is(err, secretcrypto.ErrMasterKeyMissing) {
		t.Fatalf("missing master key error = %v", err)
	}
}

func TestSecureStoreEncryptsCredentialsAndSessions(t *testing.T) {
	mainDB, plain := openSharedTestDB(t)
	ctx := context.Background()
	account, err := plain.UpsertAccount(ctx, "openid-secure", "legacy-buffer", nil, nil, nil, nil, map[string]any{"refresh": "secret"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	box, err := secretcrypto.NewSecretBox(bytes.Repeat([]byte{9}, secretcrypto.AES256KeySize))
	if err != nil {
		t.Fatal(err)
	}
	secure, err := NewDBWithSecretBox(mainDB, box)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := secure.GetAccount(ctx, account.ID); err != nil {
		t.Fatal(err)
	}
	var storedBuffer, storedCredentials string
	if err := mainDB.QueryRowContext(ctx, "SELECT login_buffer, credentials FROM wechat_accounts WHERE id = ?", account.ID).Scan(&storedBuffer, &storedCredentials); err != nil {
		t.Fatal(err)
	}
	if storedBuffer == "legacy-buffer" || storedCredentials == `{"refresh":"secret"}` {
		t.Fatal("legacy account values were not upgraded")
	}
	if _, err := secure.UpsertAccount(ctx, "openid-secure-2", "new-buffer", nil, nil, nil, nil, map[string]any{"access": "secret"}, nil); err != nil {
		t.Fatal(err)
	}
	var encrypted string
	if err := mainDB.QueryRowContext(ctx, "SELECT login_buffer FROM wechat_accounts WHERE openid = ?", "openid-secure-2").Scan(&encrypted); err != nil {
		t.Fatal(err)
	}
	if encrypted == "new-buffer" {
		t.Fatal("new login_buffer was stored in plaintext")
	}
	if err := secure.PutSession(ctx, account.ID, nil, map[string]any{"session": "secret"}, 4102444800, ""); err != nil {
		t.Fatal(err)
	}
	var sessionBlob string
	if err := mainDB.QueryRowContext(ctx, "SELECT session_blob FROM sessions WHERE wechat_account_id = ?", account.ID).Scan(&sessionBlob); err != nil {
		t.Fatal(err)
	}
	if sessionBlob == `{"session":"secret"}` {
		t.Fatal("session blob was stored in plaintext")
	}
	session, err := secure.GetSession(ctx, account.ID, "")
	if err != nil || session.SessionBlob["session"] != "secret" {
		t.Fatalf("secure session round trip = %+v, %v", session, err)
	}
}
