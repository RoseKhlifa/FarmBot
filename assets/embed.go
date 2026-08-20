package assets

import (
	"embed"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"strings"
)

// embedded contains the immutable resources shipped with the binary. The
// resource directories are populated from the checked-in web and game
// artifacts before a release build.
//
//go:embed wasm/*.wasm all:gameConfig/** webdist/**
var embedded embed.FS

// WasmFS exposes the embedded files through the standard filesystem interface.
var WasmFS fs.FS = mustSubFS("wasm")

// ReadWASM returns one embedded WASM binary by its basename.
func ReadWASM(name string) ([]byte, error) {
	return WASM(name)
}

// WASM returns one WASM asset. FARM_WASM_DIR or FARM_RESOURCE_DIR/wasm may be
// used to override the embedded copy during development.
func WASM(name string) ([]byte, error) {
	name = strings.TrimSpace(name)
	if name == "" || path.Base(name) != name || !strings.HasSuffix(name, ".wasm") {
		return nil, fmt.Errorf("invalid WASM asset name %q", name)
	}
	if file, ok := diskAsset("FARM_WASM_DIR", "wasm", name); ok {
		return os.ReadFile(file)
	}
	return fs.ReadFile(embedded, path.Join("wasm", name))
}

// EmbeddedFS returns the complete immutable resource tree for composition
// roots that need to inject multiple resource kinds into another package.
func EmbeddedFS() fs.FS { return embedded }

// GameConfigFS returns the game configuration tree, preferring an explicit
// FARM_GAME_CONFIG_DIR or FARM_RESOURCE_DIR/gameConfig override.
func GameConfigFS() fs.FS {
	return overrideOrEmbedded("FARM_GAME_CONFIG_DIR", "gameConfig", "gameConfig")
}

// WebDistFS returns the built SPA tree, preferring FARM_WEB_DIR or
// FARM_RESOURCE_DIR/webdist as a disk override.
func WebDistFS() fs.FS { return overrideOrEmbedded("FARM_WEB_DIR", "webdist", "webdist") }

func overrideOrEmbedded(envName, resourceSubdir, embeddedSubdir string) fs.FS {
	if directory := strings.TrimSpace(os.Getenv(envName)); directory != "" {
		if info, err := os.Stat(directory); err == nil && info.IsDir() {
			return os.DirFS(directory)
		}
	}
	if resourceRoot := strings.TrimSpace(os.Getenv("FARM_RESOURCE_DIR")); resourceRoot != "" {
		directory := filepath.Join(resourceRoot, resourceSubdir)
		if info, err := os.Stat(directory); err == nil && info.IsDir() {
			return os.DirFS(directory)
		}
	}
	return mustSubFS(embeddedSubdir)
}

func diskAsset(envName, resourceSubdir, name string) (string, bool) {
	candidates := make([]string, 0, 2)
	if directory := strings.TrimSpace(os.Getenv(envName)); directory != "" {
		candidates = append(candidates, filepath.Join(directory, name))
	}
	if resourceRoot := strings.TrimSpace(os.Getenv("FARM_RESOURCE_DIR")); resourceRoot != "" {
		candidates = append(candidates, filepath.Join(resourceRoot, resourceSubdir, name))
	}
	for _, candidate := range candidates {
		if info, err := os.Stat(candidate); err == nil && !info.IsDir() {
			return candidate, true
		}
	}
	return "", false
}

func mustSubFS(name string) fs.FS {
	result, err := fs.Sub(embedded, name)
	if err != nil {
		panic(fmt.Sprintf("embedded resource %q is missing: %v", name, err))
	}
	return result
}
