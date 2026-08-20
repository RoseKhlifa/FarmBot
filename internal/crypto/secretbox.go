// Package crypto contains small, application-level cryptographic helpers.
package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
)

const (
	MasterKeyEnv     = "FARM_MASTER_KEY"
	CiphertextPrefix = "fbenc:v1:"
	AES256KeySize    = 32
)

var (
	ErrMasterKeyMissing   = errors.New("FARM_MASTER_KEY is required")
	ErrMasterKeyInvalid   = errors.New("FARM_MASTER_KEY must decode to 32 bytes")
	ErrCiphertextInvalid  = errors.New("invalid secretbox ciphertext")
	ErrCiphertextRequired = errors.New("value is not encrypted")
)

// SecretBox encrypts application secrets with AES-256-GCM. A fresh nonce is
// generated for every call and stored before the GCM payload.
type SecretBox struct {
	block cipher.AEAD
}

// NewSecretBox constructs a box from an exact 32-byte key. The key is copied
// so callers can safely reuse or clear their input buffer.
func NewSecretBox(key []byte) (*SecretBox, error) {
	if len(key) != AES256KeySize {
		return nil, ErrMasterKeyInvalid
	}
	keyCopy := append([]byte(nil), key...)
	block, err := aes.NewCipher(keyCopy)
	if err != nil {
		return nil, fmt.Errorf("create AES-256 cipher: %w", err)
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("create AES-GCM: %w", err)
	}
	return &SecretBox{block: aead}, nil
}

// NewSecretBoxFromEnv loads FARM_MASTER_KEY and fails closed when it is
// missing or cannot be decoded as an AES-256 key.
func NewSecretBoxFromEnv() (*SecretBox, error) {
	return NewSecretBoxFromValue(os.Getenv(MasterKeyEnv))
}

// NewFromEnv is a concise compatibility alias.
func NewFromEnv() (*SecretBox, error) { return NewSecretBoxFromEnv() }

// NewSecretBoxFromValue accepts raw 32-byte values and the conventional hex
// or base64 encodings used by deployment secret stores. Arbitrary passphrases
// are deliberately rejected instead of silently weakening the key.
func NewSecretBoxFromValue(value string) (*SecretBox, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil, ErrMasterKeyMissing
	}
	key, err := decodeMasterKey(value)
	if err != nil {
		return nil, err
	}
	return NewSecretBox(key)
}

func decodeMasterKey(value string) ([]byte, error) {
	if len(value) == AES256KeySize {
		return []byte(value), nil
	}
	if len(value) == AES256KeySize*2 {
		if key, err := hex.DecodeString(value); err == nil && len(key) == AES256KeySize {
			return key, nil
		}
	}
	if key, err := base64.RawStdEncoding.DecodeString(value); err == nil && len(key) == AES256KeySize {
		return key, nil
	}
	if key, err := base64.StdEncoding.DecodeString(value); err == nil && len(key) == AES256KeySize {
		return key, nil
	}
	return nil, ErrMasterKeyInvalid
}

// Encrypt returns nonce || ciphertext. It never returns plaintext on error.
func (b *SecretBox) Encrypt(plaintext []byte) ([]byte, error) {
	if b == nil || b.block == nil {
		return nil, ErrMasterKeyMissing
	}
	nonce := make([]byte, b.block.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, fmt.Errorf("generate secretbox nonce: %w", err)
	}
	return b.block.Seal(nonce, nonce, plaintext, nil), nil
}

// Decrypt reverses Encrypt and rejects malformed or tampered payloads.
func (b *SecretBox) Decrypt(ciphertext []byte) ([]byte, error) {
	if b == nil || b.block == nil {
		return nil, ErrMasterKeyMissing
	}
	if len(ciphertext) < b.block.NonceSize() {
		return nil, ErrCiphertextInvalid
	}
	nonce := ciphertext[:b.block.NonceSize()]
	plain, err := b.block.Open(nil, nonce, ciphertext[b.block.NonceSize():], nil)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrCiphertextInvalid, err)
	}
	return plain, nil
}

// EncryptString returns a database-safe, versioned base64 envelope.
func (b *SecretBox) EncryptString(plaintext string) (string, error) {
	payload, err := b.Encrypt([]byte(plaintext))
	if err != nil {
		return "", err
	}
	return CiphertextPrefix + base64.RawStdEncoding.EncodeToString(payload), nil
}

// DecryptString opens a versioned envelope. Plaintext values are rejected so
// callers cannot accidentally treat an unencrypted database value as safe.
func (b *SecretBox) DecryptString(value string) (string, error) {
	if !strings.HasPrefix(value, CiphertextPrefix) {
		return "", ErrCiphertextRequired
	}
	payload, err := base64.RawStdEncoding.DecodeString(strings.TrimPrefix(value, CiphertextPrefix))
	if err != nil {
		return "", ErrCiphertextInvalid
	}
	plain, err := b.Decrypt(payload)
	if err != nil {
		return "", fmt.Errorf("decrypt secret: %w", err)
	}
	return string(plain), nil
}

// DecryptStringOrPlain supports one-time migration of legacy values. The
// encrypted flag lets a repository rewrite a legacy value after a successful
// read without ever logging or returning the secret to the caller.
func (b *SecretBox) DecryptStringOrPlain(value string) (plaintext string, encrypted bool, err error) {
	if strings.HasPrefix(value, CiphertextPrefix) {
		plaintext, err = b.DecryptString(value)
		return plaintext, true, err
	}
	if b == nil || b.block == nil {
		return "", false, ErrMasterKeyMissing
	}
	return value, false, nil
}
