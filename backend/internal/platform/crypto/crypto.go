// Package crypto provides the two cryptographic primitives the platform
// needs: password hashing (Argon2id) and authenticated symmetric encryption
// (AES-256-GCM) for secrets-at-rest such as connection credentials.
//
// Design choices, and why:
//   - Argon2id for passwords: memory-hard, resistant to GPU/ASIC cracking,
//     and the password-hashing competition winner. Parameters are tunable so
//     they can be raised as hardware gets faster without changing the format.
//   - AES-256-GCM for secrets: authenticated encryption (confidentiality +
//     integrity) with a widely-audited, hardware-accelerated cipher. Each
//     encryption uses a fresh random nonce, stored alongside the ciphertext.
package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"

	"golang.org/x/crypto/argon2"
)

// ---- Password hashing (Argon2id) ----

type argon2Params struct {
	memory      uint32
	iterations  uint32
	parallelism uint8
	saltLength  uint32
	keyLength   uint32
}

var defaultParams = argon2Params{
	memory:      64 * 1024, // 64 MB
	iterations:  3,
	parallelism: 2,
	saltLength:  16,
	keyLength:   32,
}

// HashPassword returns an encoded Argon2id hash in the standard
// $argon2id$v=19$m=...,t=...,p=...$salt$hash form, safe to store directly.
func HashPassword(password string) (string, error) {
	if len(password) == 0 {
		return "", errors.New("password must not be empty")
	}
	salt := make([]byte, defaultParams.saltLength)
	if _, err := rand.Read(salt); err != nil {
		return "", fmt.Errorf("generate salt: %w", err)
	}

	hash := argon2.IDKey([]byte(password), salt, defaultParams.iterations, defaultParams.memory, defaultParams.parallelism, defaultParams.keyLength)

	b64Salt := base64.RawStdEncoding.EncodeToString(salt)
	b64Hash := base64.RawStdEncoding.EncodeToString(hash)

	encoded := fmt.Sprintf("$argon2id$v=%d$m=%d,t=%d,p=%d$%s$%s",
		argon2.Version, defaultParams.memory, defaultParams.iterations, defaultParams.parallelism, b64Salt, b64Hash)
	return encoded, nil
}

// VerifyPassword compares a plaintext password against a previously encoded hash
// using a constant-time comparison to avoid timing side-channels.
func VerifyPassword(password, encoded string) (bool, error) {
	parts := strings.Split(encoded, "$")
	if len(parts) != 6 || parts[1] != "argon2id" {
		return false, errors.New("invalid hash format")
	}

	var version int
	if _, err := fmt.Sscanf(parts[2], "v=%d", &version); err != nil {
		return false, fmt.Errorf("parse version: %w", err)
	}

	var p argon2Params
	if _, err := fmt.Sscanf(parts[3], "m=%d,t=%d,p=%d", &p.memory, &p.iterations, &p.parallelism); err != nil {
		return false, fmt.Errorf("parse params: %w", err)
	}

	salt, err := base64.RawStdEncoding.DecodeString(parts[4])
	if err != nil {
		return false, fmt.Errorf("decode salt: %w", err)
	}
	wantHash, err := base64.RawStdEncoding.DecodeString(parts[5])
	if err != nil {
		return false, fmt.Errorf("decode hash: %w", err)
	}

	gotHash := argon2.IDKey([]byte(password), salt, p.iterations, p.memory, p.parallelism, uint32(len(wantHash)))

	if subtle.ConstantTimeCompare(gotHash, wantHash) == 1 {
		return true, nil
	}
	return false, nil
}

// ---- Symmetric encryption (AES-256-GCM) for secrets-at-rest ----

type Encryptor struct {
	gcm cipher.AEAD
}

// NewEncryptor builds an Encryptor from a base64-encoded 32-byte key.
func NewEncryptor(keyBase64 string) (*Encryptor, error) {
	key, err := decodeKey(keyBase64)
	if err != nil {
		return nil, err
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("create cipher: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("create gcm: %w", err)
	}
	return &Encryptor{gcm: gcm}, nil
}

func decodeKey(keyBase64 string) ([]byte, error) {
	key, err := base64.StdEncoding.DecodeString(keyBase64)
	if err != nil {
		return nil, fmt.Errorf("decode key: %w", err)
	}
	if len(key) != 32 {
		return nil, fmt.Errorf("encryption key must be 32 bytes (got %d) - generate one with `openssl rand -base64 32`", len(key))
	}
	return key, nil
}

// Encrypt returns nonce||ciphertext||tag as a hex string.
func (e *Encryptor) Encrypt(plaintext []byte) (string, error) {
	nonce := make([]byte, e.gcm.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return "", fmt.Errorf("generate nonce: %w", err)
	}
	sealed := e.gcm.Seal(nonce, nonce, plaintext, nil)
	return hex.EncodeToString(sealed), nil
}

// Decrypt reverses Encrypt.
func (e *Encryptor) Decrypt(encoded string) ([]byte, error) {
	raw, err := hex.DecodeString(encoded)
	if err != nil {
		return nil, fmt.Errorf("decode ciphertext: %w", err)
	}
	nonceSize := e.gcm.NonceSize()
	if len(raw) < nonceSize {
		return nil, errors.New("ciphertext too short")
	}
	nonce, ciphertext := raw[:nonceSize], raw[nonceSize:]
	plaintext, err := e.gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return nil, fmt.Errorf("decrypt: %w", err)
	}
	return plaintext, nil
}

// GenerateKey returns a fresh base64-encoded 32-byte key, for `de keygen`-style tooling.
func GenerateKey() (string, error) {
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(key), nil
}
