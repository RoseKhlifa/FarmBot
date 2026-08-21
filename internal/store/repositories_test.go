package store

import (
	"context"
	"encoding/json"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/RoseKhlifa/FarmBot/internal/config"
)

func openRepositoryTestDB(t *testing.T) *DB {
	t.Helper()
	db, err := Open(config.Config{DataDir: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return db
}

func TestRepositoriesRoundTripAndConcurrentStats(t *testing.T) {
	ctx := context.Background()
	db := openRepositoryTestDB(t)
	accounts, configs := NewAccountRepo(db), NewConfigRepo(db)
	account := Account{ID: "a1", Name: "测试", Code: "code", OwnerUser: "admin", Platform: "qq"}
	if err := accounts.Upsert(ctx, account); err != nil {
		t.Fatal(err)
	}
	configRow := AccountConfig{AccountID: "a1", AutomationJSON: json.RawMessage(`{"farm":true}`), PlantingStrategy: "level", PreferredSeedID: 42, MysteryAutoBuyCurrenciesJSON: json.RawMessage(`[1001]`)}
	if err := accounts.ApplyConfigSnapshot(ctx, "a1", configRow); err != nil {
		t.Fatal(err)
	}
	loaded, err := accounts.GetConfig(ctx, "a1")
	if err != nil || loaded.PlantingStrategy != "level" || loaded.PreferredSeedID != 42 {
		t.Fatalf("config=%+v err=%v", loaded, err)
	}
	if err := configs.SetTheme(ctx, "dark"); err != nil {
		t.Fatal(err)
	}
	if theme, err := configs.GetTheme(ctx); err != nil || theme != "dark" {
		t.Fatalf("theme=%q err=%v", theme, err)
	}

	cache := NewCacheRepo(db)
	if err := cache.PutKnownFriendGIDs(ctx, "a1", CacheValue{Payload: json.RawMessage(`[1,2]`)}); err != nil {
		t.Fatal(err)
	}
	if got, err := cache.GetKnownFriendGIDs(ctx, "a1"); err != nil || string(got.Payload) != `[1,2]` {
		t.Fatalf("cache=%+v err=%v", got, err)
	}

	stats := NewStatsRepo(db)
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 10; j++ {
				_, _ = stats.Increment(ctx, "a1", "harvest", "2026-08-20", 1)
			}
		}()
	}
	wg.Wait()
	stat, err := stats.Get(ctx, "a1", "harvest", "2026-08-20")
	if err != nil || stat.Value != 80 {
		t.Fatalf("stat=%+v err=%v", stat, err)
	}
}

func TestUserCardStateMachineRoundTrip(t *testing.T) {
	ctx := context.Background()
	db := openRepositoryTestDB(t)
	users, cards := NewUserRepo(db), NewCardRepo(db)
	if err := users.InitializeDefaultAdmin(ctx, ""); err != nil {
		t.Fatal(err)
	}
	spec := CardSpec{Description: "test", Days: 2, Type: "time"}
	card, err := cards.Create(ctx, spec)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := cards.RegisterWithCard(ctx, "user1", "Aa123456", card.Code); err != nil {
		t.Fatal(err)
	}
	user, err := users.Get(ctx, "user1")
	if err != nil || user.CardCode != card.Code {
		t.Fatalf("user=%+v err=%v", user, err)
	}
	if _, err := cards.Renew(ctx, "user1", card.Code); err == nil {
		t.Fatal("renewed an already claimed card")
	}
	if !VerifyPassword("Aa123456", user.Password) {
		t.Fatal("password hash did not verify")
	}
}

func TestDefaultAdminUsesConfiguredPasswordOnlyOnFirstInsert(t *testing.T) {
	ctx := context.Background()
	db := openRepositoryTestDB(t)
	users := NewUserRepo(db)
	if err := users.InitializeDefaultAdmin(ctx, "configured-admin-password"); err != nil {
		t.Fatal(err)
	}
	admin, err := users.Get(ctx, "admin")
	if err != nil || admin == nil || !VerifyPassword("configured-admin-password", admin.Password) {
		t.Fatalf("configured admin password was not applied: user=%+v err=%v", admin, err)
	}
	if err := users.InitializeDefaultAdmin(ctx, "replacement-password"); err != nil {
		t.Fatal(err)
	}
	admin, err = users.Get(ctx, "admin")
	if err != nil || admin == nil || !VerifyPassword("configured-admin-password", admin.Password) {
		t.Fatalf("existing admin password was unexpectedly overwritten: user=%+v err=%v", admin, err)
	}
}

func TestAdminUserPatchUpdatesMembershipAndIdentityAtomically(t *testing.T) {
	ctx := context.Background()
	db := openRepositoryTestDB(t)
	users, cards := NewUserRepo(db), NewCardRepo(db)
	if err := users.InitializeDefaultAdmin(ctx, ""); err != nil {
		t.Fatal(err)
	}
	card, err := cards.Create(ctx, CardSpec{Description: "test", Days: 2, Type: "time"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := cards.RegisterWithCard(ctx, "user1", "Aa123456", card.Code); err != nil {
		t.Fatal(err)
	}
	expires := time.Now().Add(48 * time.Hour).UnixMilli()
	updated, err := users.UpdateAdminUser(ctx, "user1", AdminUserPatch{
		NewUsername:  "user2",
		Password:     "Bb123456",
		AccountLimit: ptrInt(5),
		ExpiresAt:    &expires,
		ExpiresAtSet: true,
		Enabled:      false,
		EnabledSet:   true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if updated.Username != "user2" || updated.Status != "banned" || updated.AccountLimit != 5 || !VerifyPassword("Bb123456", updated.Password) {
		t.Fatalf("updated user=%+v", updated)
	}
	if updated.ExpireAt == nil || *updated.ExpireAt != expires || !strings.Contains(updated.CardJSON, `"enabled":false`) {
		t.Fatalf("membership state=%+v", updated)
	}
	if _, err := users.Authenticate(ctx, "user2", "Bb123456", "127.0.0.1"); err != ErrUserDisabled {
		t.Fatalf("disabled login error=%v", err)
	}
	if _, err := users.Get(ctx, "user1"); err != ErrUserNotFound {
		t.Fatalf("old username lookup error=%v", err)
	}
	accounts := NewAccountRepo(db)
	if err := accounts.Upsert(ctx, Account{ID: "owned", Name: "owned", OwnerUser: "user2"}); err != nil {
		t.Fatal(err)
	}
	// The rename above happened before this account existed; verify the same
	// transaction path on a second user rename with an existing account.
	card2, err := cards.Create(ctx, CardSpec{Description: "test", Days: 2, Type: "time"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := cards.RegisterWithCard(ctx, "user3", "Cc123456", card2.Code); err != nil {
		t.Fatal(err)
	}
	if err := accounts.Upsert(ctx, Account{ID: "owned-user3", Name: "owned", OwnerUser: "user3"}); err != nil {
		t.Fatal(err)
	}
	if _, err := users.UpdateAdminUser(ctx, "user3", AdminUserPatch{NewUsername: "user4"}); err != nil {
		t.Fatal(err)
	}
	owned, err := accounts.GetByUser(ctx, "user4")
	if err != nil || len(owned) != 1 || owned[0].ID != "owned-user3" {
		t.Fatalf("renamed account ownership=%+v err=%v", owned, err)
	}
}

func ptrInt(value int) *int { return &value }
