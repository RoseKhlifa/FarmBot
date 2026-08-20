package crypto

import (
	"bytes"
	"encoding/hex"
	"os"
	"testing"
)

func TestSecretBoxRoundTripAndNonceRandomness(t *testing.T) {
	box, err := NewSecretBox(bytes.Repeat([]byte{7}, AES256KeySize))
	if err != nil {
		t.Fatal(err)
	}
	one, err := box.Encrypt([]byte("credential"))
	if err != nil {
		t.Fatal(err)
	}
	two, err := box.Encrypt([]byte("credential"))
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Equal(one, two) {
		t.Fatal("encryption reused a nonce")
	}
	plain, err := box.Decrypt(one)
	if err != nil || string(plain) != "credential" {
		t.Fatalf("decrypt = %q, %v", plain, err)
	}
}

func TestSecretBoxEnvRequiresExactKey(t *testing.T) {
	old, present := os.LookupEnv(MasterKeyEnv)
	t.Cleanup(func() {
		if present {
			_ = os.Setenv(MasterKeyEnv, old)
		} else {
			_ = os.Unsetenv(MasterKeyEnv)
		}
	})
	_ = os.Unsetenv(MasterKeyEnv)
	if _, err := NewSecretBoxFromEnv(); err != ErrMasterKeyMissing {
		t.Fatalf("missing key error = %v", err)
	}
	_ = os.Setenv(MasterKeyEnv, hex.EncodeToString(bytes.Repeat([]byte{3}, AES256KeySize)))
	if _, err := NewSecretBoxFromEnv(); err != nil {
		t.Fatal(err)
	}
}
