package assets

import (
	"embed"
	"fmt"
	"io/fs"
	"path"
	"strings"
)

// WASM contains the official TSDK binaries shipped with FarmBot.
//
// The variable is intentionally exported so the TSDK package and future
// composition roots can choose between the embedded bytes and a test fixture.
//
//go:embed wasm/*.wasm
var WASM embed.FS

// WasmFS exposes the embedded files through the standard filesystem interface.
var WasmFS fs.FS = WASM

// ReadWASM returns one embedded WASM binary by its basename.
func ReadWASM(name string) ([]byte, error) {
	name = strings.TrimSpace(name)
	if name == "" || path.Base(name) != name || !strings.HasSuffix(name, ".wasm") {
		return nil, fmt.Errorf("invalid WASM asset name %q", name)
	}
	return fs.ReadFile(WASM, path.Join("wasm", name))
}
