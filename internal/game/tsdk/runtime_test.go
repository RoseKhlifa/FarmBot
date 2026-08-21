package tsdk

import (
	"bytes"
	"context"
	"encoding/hex"
	"os"
	"path/filepath"
	stdruntime "runtime"
	"strings"
	"testing"
	"time"

	"github.com/RoseKhlifa/FarmBot/assets"
)

func TestVerifyOfficialAssetsAndModule(t *testing.T) {
	if err := VerifyAssets(); err != nil {
		t.Fatal(err)
	}
	if err := VerifyOfficialModule(context.Background()); err != nil {
		t.Fatal(err)
	}
}

func TestRuntimeLifecycleCryptoAndIsolation(t *testing.T) {
	fixedWallTime := func() time.Time { return time.UnixMilli(1700000000123) }
	fixedMonotonicTime := func() time.Duration { return 12345678 * time.Microsecond }
	device := DeviceInfo{Model: "FarmBot Test", Platform: "win32", System: "test-os", Brand: "FarmBot"}
	first := New(context.Background(), Options{
		AccountID:     "tsdk-test-a",
		DataDir:       filepath.Join(t.TempDir(), "a"),
		Device:        device,
		WallTime:      fixedWallTime,
		MonotonicTime: fixedMonotonicTime,
	})
	second := New(context.Background(), Options{
		AccountID:     "tsdk-test-b",
		DataDir:       filepath.Join(t.TempDir(), "b"),
		Device:        device,
		WallTime:      fixedWallTime,
		MonotonicTime: fixedMonotonicTime,
	})
	if err := first.Init(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err := second.Init(context.Background()); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = first.Destroy()
		_ = second.Destroy()
	})

	plain := []byte("qq-farm-tsdk-offline-roundtrip")
	result, err := first.GetResult()
	if err != nil {
		t.Fatal(err)
	}
	if len(result) != 64 {
		t.Fatalf("result length = %d, want 64", len(result))
	}
	wantResult := make([]byte, 64)
	wantResult[1] = 1
	if !bytes.Equal(result, wantResult) {
		t.Fatalf("result mismatch with Node fixture: got %x want %x", result, wantResult)
	}
	encrypted, err := first.Encrypt(plain)
	if err != nil {
		t.Fatal(err)
	}
	wantEncrypted, err := hex.DecodeString("b52e407a40ec2e1aa7c2f6e24f9720bf0e6d2325bc586b16c775cce4cd92")
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(encrypted, wantEncrypted) {
		t.Fatalf("encrypt mismatch with Node fixture: got %x want %x", encrypted, wantEncrypted)
	}
	decrypted, err := first.Decrypt(encrypted)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(decrypted, plain) {
		t.Fatalf("decrypt mismatch: got %x want %x", decrypted, plain)
	}

	token, err := first.GenerateToken([]byte("request-sequence-1"))
	if err != nil {
		t.Fatal(err)
	}
	if token.Token == "" {
		t.Fatal("empty generated token")
	}
	if token.Token != "Oi7EIAC124d63b7a7d764d0f" || token.Nonce != "" {
		t.Fatalf("token mismatch with Node fixture: got %+v", token)
	}

	v2Encrypted, err := second.EncryptV2(plain)
	if err != nil {
		t.Fatal(err)
	}
	wantV2Encrypted, err := hex.DecodeString("ace0da7a5453444b1e000000020000004c2c362fccf4c8cf56d91562624a9b44b66daefd90e81f125ae8120fd18d")
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(v2Encrypted, wantV2Encrypted) {
		t.Fatalf("encrypt v2 mismatch with Node fixture: got %x want %x", v2Encrypted, wantV2Encrypted)
	}
	v2Decrypted, err := second.DecryptV2(v2Encrypted)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(v2Decrypted, plain) {
		t.Fatalf("decrypt v2 mismatch: got %x want %x", v2Decrypted, plain)
	}
	if err := first.HeartbeatTick(); err != nil {
		t.Fatal(err)
	}
	if err := first.ProcessReceivedData(); err != nil {
		t.Fatal(err)
	}
	if err := first.SendStatus(); err != nil {
		t.Fatal(err)
	}
	if err := first.CheckFunctionArray([]string{"function fixture() { return 1; }"}, 0); err != nil {
		t.Fatal(err)
	}
	if first.GetStatus().HeartbeatTicks != 1 || second.GetStatus().HeartbeatTicks != 0 {
		t.Fatalf("runtime counters are not isolated: first=%+v second=%+v", first.GetStatus(), second.GetStatus())
	}
	status := first.GetStatus()
	if status.ProcessTicks != 1 || status.StatusReports != 1 || status.FunctionChecks != 1 {
		t.Fatalf("runtime lifecycle counters were not updated: %+v", status)
	}
}

func TestRuntimeRejectsWrongWASMAndUseAfterDestroy(t *testing.T) {
	legacy, err := assets.ReadWASM("tsdk-legacy.wasm")
	if err != nil {
		t.Fatal(err)
	}
	runtime := New(context.Background(), Options{WASMData: legacy, DataDir: filepath.Join(t.TempDir(), "bad")})
	if err := runtime.Init(context.Background()); err == nil {
		t.Fatal("legacy WASM unexpectedly initialized")
	}

	official, err := assets.ReadWASM("tsdk-v3.8.2.wasm")
	if err != nil {
		t.Fatal(err)
	}
	runtime = New(context.Background(), Options{WASMData: official, DataDir: filepath.Join(t.TempDir(), "destroy")})
	if err := runtime.Init(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err := runtime.Destroy(); err != nil {
		t.Fatal(err)
	}
	if _, err := runtime.GenerateToken([]byte("x")); err == nil {
		t.Fatal("use after destroy unexpectedly succeeded")
	}
}

func TestRuntimeCreatesAccountDirectory(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "nested", "account")
	runtime := New(context.Background(), Options{DataDir: dir})
	if err := runtime.Init(context.Background()); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = runtime.Destroy() }()
	if _, err := os.Stat(dir); err != nil {
		t.Fatal(err)
	}
}

func TestRuntimeReinitializesWithPersistentAccountDirectory(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "persistent")
	for i := 0; i < 3; i++ {
		runtime := New(context.Background(), Options{AccountID: "persistent", DataDir: dir})
		if err := runtime.Init(context.Background()); err != nil {
			t.Fatalf("initialization %d failed: %v", i+1, err)
		}
		if err := runtime.Destroy(); err != nil {
			t.Fatalf("destroy %d failed: %v", i+1, err)
		}
	}
}

func TestRuntimeCanBeDisabledExplicitly(t *testing.T) {
	enabled := false
	runtime := New(context.Background(), Options{Enabled: &enabled, DataDir: filepath.Join(t.TempDir(), "disabled")})
	if err := runtime.Init(context.Background()); err == nil {
		t.Fatal("disabled TSDK runtime unexpectedly initialized")
	}
	if runtime.GetStatus().Enabled {
		t.Fatal("disabled TSDK runtime reports enabled")
	}
}

func TestDeviceStringMatchesNodeHostShape(t *testing.T) {
	runtime := New(context.Background(), Options{})
	got := runtime.deviceString()
	if !strings.HasSuffix(got, ";Node.js;") {
		t.Fatalf("device string = %q, want Node-compatible brand", got)
	}
	if stdruntime.GOOS == "windows" && !strings.HasPrefix(got, "Windows_NT x64;win32;") {
		t.Fatalf("Windows device string = %q", got)
	}
}
