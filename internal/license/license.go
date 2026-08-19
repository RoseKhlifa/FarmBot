// Package license implements the legacy single-machine license contract.
//
// The license format and cryptographic derivation intentionally match the
// Node implementation: scrypt(secret, "salt", 32), AES-256-CBC with a zero
// IV, and an uppercase MD5 prefix formatted as four-character groups.
package license

import (
	"bufio"
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/md5" //nolint:gosec -- MD5 is part of the existing license contract.
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/RoseKhlifa/FarmBot/internal/config"
	"github.com/RoseKhlifa/FarmBot/internal/platform/machineid"
)

const (
	// LicenseSecret is the fixed seed retained for compatibility with existing
	// license files and administrator-generated keys.
	LicenseSecret      = "zDJp2Wv7cLawful5w7rTF8mB5foq1EUsU"
	licenseSalt        = "salt"
	licenseKeyLen      = 16
	defaultMaxAttempts = 5
)

// LICENSE_SECRET is retained as an exported compatibility alias for tooling
// that used the JavaScript constant name during the migration.
const LICENSE_SECRET = LicenseSecret

// Record is the on-disk license.json shape used by the Node application.
type Record struct {
	MachineID  string `json:"machineId"`
	LicenseKey string `json:"licenseKey"`
	VerifiedAt int64  `json:"verifiedAt"`
}

// Status is the result of checking the installed license.
type Status struct {
	Valid     bool   `json:"valid"`
	Reason    string `json:"reason,omitempty"`
	MachineID string `json:"machineId"`
}

// Options controls a Validator. Enabled is deliberately false by default;
// callers must explicitly opt in with FARM_LICENSE_ENABLED=true or a true
// value in Options.
type Options struct {
	Enabled     bool
	DataDir     string
	LicenseFile string
	Secret      string
	MachineID   func() string
	Input       io.Reader
	Output      io.Writer
	MaxAttempts int
}

// Validator checks and, when interactive input is available, enrolls a
// machine license.
type Validator struct {
	options Options
}

// New creates a validator with safe defaults.
func New(options Options) *Validator {
	if options.Secret == "" {
		options.Secret = LicenseSecret
	}
	if options.MachineID == nil {
		options.MachineID = machineid.GenerateMachineID
	}
	if options.Input == nil {
		options.Input = os.Stdin
	}
	if options.Output == nil {
		options.Output = os.Stdout
	}
	if options.MaxAttempts <= 0 {
		options.MaxAttempts = defaultMaxAttempts
	}
	if options.DataDir == "" && options.LicenseFile == "" {
		options.DataDir = config.ResolvePaths().DataDir
	}
	return &Validator{options: options}
}

// EnabledFromEnv preserves the default-off gate. Only the explicit value
// "true" enables license enforcement.
func EnabledFromEnv() bool { return strings.TrimSpace(os.Getenv("FARM_LICENSE_ENABLED")) == "true" }

// LicenseEnabled is an alias for EnabledFromEnv.
func LicenseEnabled() bool { return EnabledFromEnv() }

// DefaultOptions returns options configured from the process environment.
func DefaultOptions() Options { return Options{Enabled: EnabledFromEnv()} }

// GenerateLicenseKey creates the administrator-facing XXXX-XXXX-XXXX-XXXX
// key for a machine ID. An optional secret exists for offline tooling and
// tests; production callers should use the default compatibility secret.
func GenerateLicenseKey(machineID string, secret ...string) string {
	seed := LicenseSecret
	if len(secret) > 0 && secret[0] != "" {
		seed = secret[0]
	}
	normalizedID := normalize(machineID)
	key, err := deriveAESKey(seed)
	if err != nil {
		return ""
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return ""
	}
	plain := []byte(normalizedID + seed)
	plain = pkcs7Pad(plain, block.BlockSize())
	encrypted := make([]byte, len(plain))
	cipher.NewCBCEncrypter(block, make([]byte, block.BlockSize())).CryptBlocks(encrypted, plain)
	digest := md5.Sum(encrypted)
	encoded := strings.ToUpper(hex.EncodeToString(digest[:]))[:licenseKeyLen]
	return groupKey(encoded)
}

// VerifyLicenseKey compares a user-supplied key to the derived key in
// constant time after removing legacy separators and normalizing case.
func VerifyLicenseKey(machineID, licenseKey string, secret ...string) bool {
	expected := normalize(strings.ReplaceAll(GenerateLicenseKey(machineID, secret...), "-", ""))
	actual := normalize(licenseKey)
	if expected == "" || actual == "" {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(expected), []byte(actual)) == 1
}

// Load reads and validates a license.json record. Missing or malformed files
// are returned as errors so callers can distinguish them from valid records.
func Load(path string) (Record, error) {
	var record Record
	data, err := os.ReadFile(path)
	if err != nil {
		return record, err
	}
	if err := json.Unmarshal(data, &record); err != nil {
		return record, fmt.Errorf("decode license file: %w", err)
	}
	record.MachineID = normalize(record.MachineID)
	record.LicenseKey = normalize(record.LicenseKey)
	if record.MachineID == "" || record.LicenseKey == "" || record.VerifiedAt <= 0 {
		return Record{}, errors.New("license file is incomplete")
	}
	return record, nil
}

// LoadLicense is an explicit alias for Load.
func LoadLicense(path string) (Record, error) { return Load(path) }

// Save writes a license.json record atomically and creates its parent
// directory when necessary.
func Save(path string, machineID, licenseKey string) error {
	record := Record{
		MachineID:  normalize(machineID),
		LicenseKey: normalize(licenseKey),
		VerifiedAt: time.Now().UnixMilli(),
	}
	if record.MachineID == "" || record.LicenseKey == "" {
		return errors.New("machine ID and license key are required")
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create license directory: %w", err)
	}
	data, err := json.MarshalIndent(record, "", "  ")
	if err != nil {
		return fmt.Errorf("encode license: %w", err)
	}
	data = append(data, '\n')
	temporary, err := os.CreateTemp(filepath.Dir(path), ".license-*.tmp")
	if err != nil {
		return fmt.Errorf("create temporary license file: %w", err)
	}
	temporaryName := temporary.Name()
	defer os.Remove(temporaryName)
	if err := temporary.Chmod(0o600); err != nil {
		_ = temporary.Close()
		return fmt.Errorf("protect temporary license file: %w", err)
	}
	if _, err := temporary.Write(data); err != nil {
		_ = temporary.Close()
		return fmt.Errorf("write license: %w", err)
	}
	if err := temporary.Sync(); err != nil {
		_ = temporary.Close()
		return fmt.Errorf("sync license: %w", err)
	}
	if err := temporary.Close(); err != nil {
		return fmt.Errorf("close temporary license file: %w", err)
	}
	if err := os.Rename(temporaryName, path); err != nil {
		return fmt.Errorf("install license file: %w", err)
	}
	return nil
}

// SaveLicense is an explicit alias for Save.
func SaveLicense(path string, machineID, licenseKey string) error {
	return Save(path, machineID, licenseKey)
}

// LicensePath resolves the legacy license.json location for a data directory.
func LicensePath(dataDir string) string {
	if strings.TrimSpace(dataDir) == "" {
		dataDir = config.ResolvePaths().DataDir
	}
	return filepath.Join(dataDir, "license.json")
}

// Check validates the current machine against the installed record. A
// missing or malformed file maps to the legacy no_license reason.
func (v *Validator) Check() Status {
	currentMachineID := v.machineID()
	record, err := Load(v.path())
	if err != nil {
		return Status{Reason: "no_license", MachineID: currentMachineID}
	}
	if normalize(record.MachineID) != normalize(currentMachineID) {
		return Status{Reason: "machine_id_mismatch", MachineID: currentMachineID}
	}
	if !VerifyLicenseKey(record.MachineID, record.LicenseKey, v.options.Secret) {
		return Status{Reason: "invalid_license", MachineID: currentMachineID}
	}
	return Status{Valid: true, MachineID: normalize(currentMachineID)}
}

// VerifyAndRun applies the default-off gate and optionally prompts for a key
// when enforcement is enabled. It returns false after the configured number
// of invalid attempts or when input is unavailable.
func (v *Validator) VerifyAndRun(ctx context.Context) bool {
	if ctx == nil {
		ctx = context.Background()
	}
	if !v.options.Enabled {
		return true
	}
	status := v.Check()
	if status.Valid {
		return true
	}
	if v.options.Input == nil {
		return false
	}

	_, _ = fmt.Fprintf(v.options.Output, "\n[license] status: %s\nmachine ID: %s\n", status.Reason, machineid.Display(status.MachineID))
	scanner := bufio.NewScanner(v.options.Input)
	for attempt := 0; attempt < v.options.MaxAttempts; attempt++ {
		select {
		case <-ctx.Done():
			return false
		default:
		}
		_, _ = fmt.Fprint(v.options.Output, "license key: ")
		if !scanner.Scan() {
			return false
		}
		key := strings.TrimSpace(scanner.Text())
		if VerifyLicenseKey(status.MachineID, key, v.options.Secret) {
			if err := Save(v.path(), status.MachineID, key); err != nil {
				_, _ = fmt.Fprintf(v.options.Output, "license save failed: %v\n", err)
				return false
			}
			_, _ = fmt.Fprintln(v.options.Output, "license verified")
			return true
		}
		remaining := v.options.MaxAttempts - attempt - 1
		if remaining > 0 {
			_, _ = fmt.Fprintf(v.options.Output, "invalid license key (%d attempts remaining)\n", remaining)
		}
	}
	return false
}

// CheckLicense checks using environment defaults.
func CheckLicense() Status { return New(DefaultOptions()).Check() }

// Check is a concise package-level alias for CheckLicense.
func Check() Status { return CheckLicense() }

// VerifyAndRun checks using environment defaults. It is intended for the
// process entry point and is disabled unless FARM_LICENSE_ENABLED=true.
func VerifyAndRun() bool { return New(DefaultOptions()).VerifyAndRun(context.Background()) }

func (v *Validator) path() string {
	if strings.TrimSpace(v.options.LicenseFile) != "" {
		return v.options.LicenseFile
	}
	return LicensePath(v.options.DataDir)
}

func (v *Validator) machineID() string {
	if v.options.MachineID == nil {
		return machineid.GenerateMachineID()
	}
	return normalize(v.options.MachineID())
}

func deriveAESKey(secret string) ([]byte, error) {
	return scryptKey([]byte(secret), []byte(licenseSalt), 16384, 8, 1, 32)
}

func pkcs7Pad(value []byte, blockSize int) []byte {
	padding := blockSize - len(value)%blockSize
	result := make([]byte, len(value)+padding)
	copy(result, value)
	for i := len(value); i < len(result); i++ {
		result[i] = byte(padding)
	}
	return result
}

func normalize(value string) string {
	return strings.ToUpper(strings.ReplaceAll(strings.TrimSpace(value), "-", ""))
}

func groupKey(value string) string {
	groups := make([]string, 0, 4)
	for len(value) > 0 {
		n := 4
		if len(value) < n {
			n = len(value)
		}
		groups = append(groups, value[:n])
		value = value[n:]
	}
	return strings.Join(groups, "-")
}
