package license

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestGenerateLicenseKeyMatchesNodeContract(t *testing.T) {
	const machine = "0123-4567-89ab-cdef"
	if got, want := GenerateLicenseKey(machine), "6C74-8A3B-CF89-215D"; got != want {
		t.Fatalf("GenerateLicenseKey() = %q, want %q", got, want)
	}
	if !VerifyLicenseKey(machine, "6c748a3b-cf89215d") {
		t.Fatal("VerifyLicenseKey rejected a normalized compatible key")
	}
	if VerifyLicenseKey(machine, "0000-0000-0000-0000") {
		t.Fatal("VerifyLicenseKey accepted an invalid key")
	}
}

func TestValidatorDefaultOff(t *testing.T) {
	validator := New(Options{Enabled: false, DataDir: t.TempDir(), Input: strings.NewReader("invalid\n")})
	if !validator.VerifyAndRun(context.Background()) {
		t.Fatal("disabled license gate rejected startup")
	}
}

func TestValidatorEnrollsAndChecksLicense(t *testing.T) {
	dir := t.TempDir()
	input := strings.NewReader(GenerateLicenseKey("AABBCCDDEEFF0011") + "\n")
	output := new(bytes.Buffer)
	validator := New(Options{
		Enabled:   true,
		DataDir:   dir,
		MachineID: func() string { return "AABB-CCDD-EEFF-0011" },
		Input:     input,
		Output:    output,
	})
	if !validator.VerifyAndRun(context.Background()) {
		t.Fatalf("VerifyAndRun failed enrollment: %s", output.String())
	}
	status := validator.Check()
	if !status.Valid || status.MachineID != "AABBCCDDEEFF0011" {
		t.Fatalf("unexpected enrolled status: %+v", status)
	}
	data, err := os.ReadFile(filepath.Join(dir, "license.json"))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(data, []byte(`"machineId": "AABBCCDDEEFF0011"`)) {
		t.Fatalf("license file did not preserve normalized machine ID: %s", data)
	}
}

func TestMalformedLicenseIsNoLicense(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "license.json"), []byte("not-json"), 0o600); err != nil {
		t.Fatal(err)
	}
	validator := New(Options{Enabled: true, DataDir: dir, MachineID: func() string { return "AABBCCDDEEFF0011" }})
	if status := validator.Check(); status.Valid || status.Reason != "no_license" {
		t.Fatalf("unexpected malformed license status: %+v", status)
	}
}
