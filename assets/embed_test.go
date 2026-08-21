package assets

import (
	"io/fs"
	"os"
	"path/filepath"
	"testing"
)

func TestEmbeddedReleaseAssets(t *testing.T) {
	for _, name := range []string{"tsdk.wasm", "tsdk-v3.8.2.wasm", "tsdk-legacy.wasm"} {
		data, err := WASM(name)
		if err != nil || len(data) == 0 {
			t.Fatalf("WASM(%q) = %d bytes, %v", name, len(data), err)
		}
	}
	if GameConfigFS() == nil || WebDistFS() == nil {
		t.Fatal("embedded resource filesystems must not be nil")
	}
	if data, err := fs.ReadFile(WebDistFS(), "index.html"); err != nil || len(data) == 0 {
		t.Fatalf("embedded webdist/index.html unavailable: %v", err)
	}
	foundConfig := false
	if err := fs.WalkDir(GameConfigFS(), ".", func(_ string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !entry.IsDir() {
			foundConfig = true
		}
		return nil
	}); err != nil || !foundConfig {
		t.Fatalf("embedded game config is empty: %v", err)
	}
}

func TestAssetDiskOverrides(t *testing.T) {
	root := t.TempDir()
	wasmDir := filepath.Join(root, "wasm")
	webDir := filepath.Join(root, "webdist")
	configDir := filepath.Join(root, "gameConfig")
	for _, dir := range []string{wasmDir, webDir, configDir} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(wasmDir, "custom.wasm"), []byte("custom"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(webDir, "index.html"), []byte("override"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(configDir, "test.json"), []byte("{}"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("FARM_RESOURCE_DIR", root)
	if got, err := WASM("custom.wasm"); err != nil || string(got) != "custom" {
		t.Fatalf("WASM override = %q, %v", got, err)
	}
	if got, err := fs.ReadFile(WebDistFS(), "index.html"); err != nil || string(got) != "override" {
		t.Fatalf("web override = %q, %v", got, err)
	}
	if got, err := fs.ReadFile(GameConfigFS(), "test.json"); err != nil || string(got) != "{}" {
		t.Fatalf("config override = %q, %v", got, err)
	}
}
