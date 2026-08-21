package tsdk

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sort"

	"github.com/RoseKhlifa/FarmBot/assets"
	"github.com/tetratelabs/wazero"
)

const (
	// OfficialVersion is the version string exposed by the official host shim.
	OfficialVersion = "v3.8.2.1783066265"
	// OfficialSHA256 is the checksum of the only supported production TSDK.
	OfficialSHA256 = "705e326caad538d6cccb40cb1bd54573525a42d12215c9da9c9c513ec4850a5f"
	// LegacySHA256 identifies the two legacy compatibility assets shipped with
	// the Node application. They are retained for explicit asset validation but
	// are not accepted by Runtime.Init.
	LegacySHA256         = "748ca5e807a41e26ee81e21d8f71e231c0832c1252bf3f4814ba0cd92d5aedff"
	DefaultAppID         = "1112386029"
	DefaultGameID uint32 = 3167
	DefaultAppKey        = "0"
)

var officialExports = map[string]string{
	"memory":             "w",
	"createBuffer":       "A",
	"destroyBuffer":      "B",
	"getResult":          "C",
	"initRuntime":        "G",
	"heartbeat":          "M",
	"getDataToServer":    "N",
	"sendDataFromServer": "O",
	"generateToken":      "aa",
	"encrypt":            "ba",
	"decrypt":            "ca",
	"encryptV2":          "da",
	"decryptV2":          "ea",
}

var requiredExports = []string{
	"w", "A", "B", "C", "G", "M", "N", "O", "aa", "ba", "ca", "da", "ea",
}

var expectedImports = func() map[string]struct{} {
	imports := make(map[string]struct{}, 22)
	for c := 'a'; c <= 'v'; c++ {
		imports[string(c)] = struct{}{}
	}
	return imports
}()

var mergedDataSegments = [][2]uint32{
	{1024, 5541}, {6580, 8989}, {15585, 33}, {15643, 1}, {15655, 21},
	{15701, 1}, {15713, 21}, {15759, 1}, {15771, 30}, {15826, 14},
	{15875, 1}, {15887, 21}, {15933, 1}, {15945, 671}, {16632, 400},
	{17040, 103}, {67371008, 404},
}

const mergedDataKey uint32 = 1871261153

// OfficialExports returns a copy of the symbolic export mapping used by the
// official wrapper. The copy prevents callers from mutating verification state.
func OfficialExports() map[string]string {
	result := make(map[string]string, len(officialExports))
	for name, symbol := range officialExports {
		result[name] = symbol
	}
	return result
}

// SHA256Hex computes a lower-case SHA-256 digest for an asset or fixture.
func SHA256Hex(data []byte) string {
	digest := sha256.Sum256(data)
	return hex.EncodeToString(digest[:])
}

// VerifyWASM checks the production binary checksum without instantiating it.
func VerifyWASM(data []byte) error {
	actual := SHA256Hex(data)
	if actual != OfficialSHA256 {
		return fmt.Errorf("TSDK WASM checksum mismatch: expected %s, got %s", OfficialSHA256, actual)
	}
	return nil
}

// VerifyAssets validates all three embedded WASM assets. Only the official
// v3.8.2 asset is accepted by Runtime.Init; the other two are checked so a
// packaged release cannot silently ship a corrupted compatibility asset.
func VerifyAssets() error {
	checks := map[string]string{
		"tsdk-v3.8.2.wasm": OfficialSHA256,
		"tsdk.wasm":        LegacySHA256,
		"tsdk-legacy.wasm": LegacySHA256,
	}
	for name, expected := range checks {
		data, err := assets.ReadWASM(name)
		if err != nil {
			return fmt.Errorf("read WASM asset %s: %w", name, err)
		}
		if actual := SHA256Hex(data); actual != expected {
			return fmt.Errorf("WASM asset %s checksum mismatch: expected %s, got %s", name, expected, actual)
		}
	}
	return nil
}

// VerifyCompiledModule validates the import/export ABI before any host module
// is instantiated. This keeps a swapped or truncated official binary from
// running against a subtly incompatible host implementation.
func VerifyCompiledModule(module wazero.CompiledModule) error {
	imports := module.ImportedFunctions()
	if len(imports) != len(expectedImports) {
		return fmt.Errorf("TSDK import count mismatch: expected %d, got %d", len(expectedImports), len(imports))
	}
	seen := make(map[string]struct{}, len(imports))
	for _, definition := range imports {
		moduleName, name, isImport := definition.Import()
		if !isImport || moduleName != "a" {
			return fmt.Errorf("TSDK unexpected import %s.%s", moduleName, name)
		}
		if _, ok := expectedImports[name]; !ok {
			return fmt.Errorf("TSDK unexpected import a.%s", name)
		}
		if _, duplicate := seen[name]; duplicate {
			return fmt.Errorf("TSDK duplicate import a.%s", name)
		}
		seen[name] = struct{}{}
	}
	if len(seen) != len(expectedImports) {
		missing := make([]string, 0)
		for name := range expectedImports {
			if _, ok := seen[name]; !ok {
				missing = append(missing, name)
			}
		}
		sort.Strings(missing)
		return fmt.Errorf("TSDK missing imports: %v", missing)
	}

	functions := module.ExportedFunctions()
	for _, name := range requiredExports {
		if _, ok := functions[name]; !ok {
			if name == "w" {
				if _, memoryOK := module.ExportedMemories()[name]; memoryOK {
					continue
				}
			}
			return fmt.Errorf("TSDK missing required export %s", name)
		}
	}
	if _, ok := module.ExportedFunctions()["__mergewasm_shared____wasm_decrypt_strings"]; !ok {
		return fmt.Errorf("TSDK merged-data decryptor is missing")
	}
	return nil
}

// VerifyOfficialModule compiles and validates the official embedded module.
// It is useful in packaging checks without creating a live account runtime.
func VerifyOfficialModule(ctx context.Context) error {
	data, err := assets.ReadWASM("tsdk-v3.8.2.wasm")
	if err != nil {
		return err
	}
	if err := VerifyWASM(data); err != nil {
		return err
	}
	runtime := wazero.NewRuntime(ctx)
	defer func() { _ = runtime.Close(ctx) }()
	compiled, err := runtime.CompileModule(ctx, data)
	if err != nil {
		return fmt.Errorf("compile TSDK WASM: %w", err)
	}
	defer func() { _ = compiled.Close(ctx) }()
	return VerifyCompiledModule(compiled)
}
