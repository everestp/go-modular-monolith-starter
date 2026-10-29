package security

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"fmt"
	"strings"

	"golang.org/x/crypto/argon2"
)

// Argon2Config configures the password hashing parameters.
type Argon2Config struct {
	Memory      uint32
	Iterations  uint32
	Parallelism uint8
	KeyLength   uint32
	Pepper      string
}

// HashPassword creates an argon2id hash from a password.
func HashPassword(password string, cfg Argon2Config) (string, error) {
	if password == "" {
		return "", fmt.Errorf("password is required")
	}
	if cfg.Memory == 0 {
		cfg.Memory = 64 * 1024
	}
	if cfg.Iterations == 0 {
		cfg.Iterations = 3
	}
	if cfg.Parallelism == 0 {
		cfg.Parallelism = 2
	}
	if cfg.KeyLength == 0 {
		cfg.KeyLength = 32
	}

	salt := make([]byte, 16)
	if _, err := rand.Read(salt); err != nil {
		return "", fmt.Errorf("generate salt: %w", err)
	}
	secret := cfg.Pepper + password
	key := argon2.IDKey([]byte(secret), salt, cfg.Iterations, cfg.Memory, cfg.Parallelism, cfg.KeyLength)
	encodedSalt := base64.RawStdEncoding.EncodeToString(salt)
	encodedHash := base64.RawStdEncoding.EncodeToString(key)
	return fmt.Sprintf("$argon2id$v=%d$m=%d,t=%d,p=%d$%s$%s", argon2.Version, cfg.Memory, cfg.Iterations, cfg.Parallelism, encodedSalt, encodedHash), nil
}

// VerifyPassword compares a password with an argon2id hash.
func VerifyPassword(password, encodedHash string, cfg Argon2Config) (bool, error) {
	if encodedHash == "" {
		return false, nil
	}
	parts := strings.Split(encodedHash, "$")
	if len(parts) != 6 || parts[1] != "argon2id" {
		return false, fmt.Errorf("unsupported hash format")
	}
	var version int
	if _, err := fmt.Sscanf(parts[2], "v=%d", &version); err != nil {
		return false, fmt.Errorf("parse version: %w", err)
	}
	if version != argon2.Version {
		return false, fmt.Errorf("unsupported argon2 version")
	}
	var memory, iterations, keyLength int
	var parallelism int
	_, err := fmt.Sscanf(parts[3], "m=%d,t=%d,p=%d", &memory, &iterations, &parallelism)
	if err != nil {
		return false, fmt.Errorf("parse params: %w", err)
	}
	if memory <= 0 || iterations <= 0 || parallelism <= 0 {
		return false, fmt.Errorf("invalid hash parameters")
	}
	keyLength = 32
	decodedSalt, err := base64.RawStdEncoding.DecodeString(parts[4])
	if err != nil {
		return false, fmt.Errorf("decode salt: %w", err)
	}
	decodedHash, err := base64.RawStdEncoding.DecodeString(parts[5])
	if err != nil {
		return false, fmt.Errorf("decode hash: %w", err)
	}
	secret := cfg.Pepper + password
	computed := argon2.IDKey([]byte(secret), decodedSalt, uint32(iterations), uint32(memory), uint8(parallelism), uint32(keyLength))
	return subtle.ConstantTimeCompare(decodedHash, computed) == 1, nil
}
