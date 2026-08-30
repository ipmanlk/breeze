package auth

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"fmt"
	"strings"
	"sync"

	"golang.org/x/crypto/argon2"
)

const (
	argon2Time    = 3
	argon2Memory  = 64 * 1024
	argon2Threads = 2
	argon2KeyLen  = 32
	saltLen       = 32
)

const (
	minMemory = 1 * 1024
	minTime   = 1
	minKeyLen = 8
)

func HashPassword(password string) (string, error) {
	salt := make([]byte, saltLen)
	if _, err := rand.Read(salt); err != nil {
		return "", fmt.Errorf("generate salt: %w", err)
	}

	hash := argon2.IDKey([]byte(password), salt, argon2Time, argon2Memory, argon2Threads, argon2KeyLen)

	b64Salt := base64.RawStdEncoding.EncodeToString(salt)
	b64Hash := base64.RawStdEncoding.EncodeToString(hash)

	return fmt.Sprintf("$argon2id$v=19$m=%d,t=%d,p=%d$%s$%s",
		argon2Memory, argon2Time, argon2Threads, b64Salt, b64Hash), nil
}

func CheckPassword(password, encodedHash string) bool {
	parts := parsePHCString(encodedHash)
	if parts == nil {
		return false
	}

	salt, err := base64.RawStdEncoding.DecodeString(parts.raw[4])
	if err != nil {
		return false
	}

	hash, err := base64.RawStdEncoding.DecodeString(parts.raw[5])
	if err != nil {
		return false
	}

	if len(hash) < minKeyLen {
		return false
	}

	computed := argon2.IDKey([]byte(password), salt, parts.time, parts.memory, uint8(parts.threads), uint32(len(hash)))

	return subtle.ConstantTimeCompare(hash, computed) == 1
}

type phcParts struct {
	raw     []string
	memory  uint32
	time    uint32
	threads uint32
}

// NeedsRehash reports whether the stored hash uses weaker parameters than
// the current argon2* constants, indicating the password should be rehashed
// with stronger params on the next successful login.
// Only memory, time, and threads are compared (the tunable cost parameters).
// The key length is not tracked in the PHC string and is assumed fixed.
func NeedsRehash(encodedHash string) bool {
	parts := parsePHCString(encodedHash)
	if parts == nil {
		return true // unparseable / unknown format → rehash with current params
	}
	return parts.memory != argon2Memory || parts.time != argon2Time || parts.threads != argon2Threads
}

var (
	dummyHashOnce sync.Once
	dummyHash     string
)

// BurnPasswordCheck performs a full argon2id verification against a
// throwaway hash, costing roughly the same time as a real CheckPassword.
//
// Callers use this on "unknown account" paths so that response timing does
// not reveal whether an email exists: without it, an unknown email returns
// immediately while a known one pays the hashing cost.
func BurnPasswordCheck(password string) {
	dummyHashOnce.Do(func() {
		if h, err := HashPassword("timing-equalizer account does not exist"); err == nil {
			dummyHash = h
		}
	})
	if dummyHash != "" {
		CheckPassword(password, dummyHash)
	}
}

func parsePHCString(s string) *phcParts {
	parts := strings.Split(s, "$")
	if len(parts) != 6 {
		return nil
	}
	if parts[1] != "argon2id" || parts[2] != "v=19" {
		return nil
	}
	var memory, time, threads uint32
	if _, err := fmt.Sscanf(parts[3], "m=%d,t=%d,p=%d", &memory, &time, &threads); err != nil {
		return nil
	}
	if memory < minMemory || time < minTime || threads > 255 {
		return nil
	}
	return &phcParts{raw: parts, memory: memory, time: time, threads: threads}
}
