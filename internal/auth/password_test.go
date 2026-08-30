package auth

import (
	"strings"
	"testing"
)

func TestHashPassword_Format(t *testing.T) {
	hash, err := HashPassword("correct-horse-battery-staple")
	if err != nil {
		t.Fatalf("HashPassword() error = %v", err)
	}

	if !strings.HasPrefix(hash, "$argon2id$v=19$") {
		t.Errorf("HashPassword() hash = %q, want $argon2id$ prefix", hash)
	}

	parts := strings.Split(hash, "$")
	if len(parts) != 6 {
		t.Fatalf("HashPassword() parts = %d, want 6", len(parts))
	}

	if parts[1] != "argon2id" {
		t.Errorf("HashPassword() variant = %q, want argon2id", parts[1])
	}

	if parts[2] != "v=19" {
		t.Errorf("HashPassword() version = %q, want v=19", parts[2])
	}
}

func TestCheckPassword_Success(t *testing.T) {
	password := "p@ssw0rd!"
	hash, err := HashPassword(password)
	if err != nil {
		t.Fatalf("HashPassword() error = %v", err)
	}

	if !CheckPassword(password, hash) {
		t.Error("CheckPassword() = false, want true")
	}
}

func TestCheckPassword_WrongPassword(t *testing.T) {
	hash, err := HashPassword("correct-password")
	if err != nil {
		t.Fatalf("HashPassword() error = %v", err)
	}

	if CheckPassword("wrong-password", hash) {
		t.Error("CheckPassword() = true, want false")
	}
}

func TestCheckPassword_InvalidFormat(t *testing.T) {
	tests := []struct {
		name string
		hash string
	}{
		{name: "empty", hash: ""},
		{name: "too few parts", hash: "$argon2id$v=19$m=8,t=1,p=1$salt"},
		{name: "too many parts", hash: "$a$b$c$d$e$f$g"},
		{name: "wrong variant", hash: "$argon2i$v=19$m=8,t=1,p=1$salt$hash"},
		{name: "wrong version", hash: "$argon2id$v=18$m=8,t=1,p=1$salt$hash"},
		{name: "invalid params", hash: "$argon2id$v=19$x=8,t=1,p=1$salt$hash"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if CheckPassword("password", tt.hash) {
				t.Errorf("CheckPassword() = true, want false")
			}
		})
	}
}

func TestCheckPassword_WeakParamRejection(t *testing.T) {
	password := "test-password"
	hash, err := HashPassword(password)
	if err != nil {
		t.Fatalf("HashPassword() error = %v", err)
	}

	parts := strings.Split(hash, "$")
	weakHash := "$argon2id$v=19$m=1,t=1,p=1$" + parts[4] + "$" + parts[5]

	if CheckPassword(password, weakHash) {
		t.Error("CheckPassword() = true, want false (reject weak parameters)")
	}
}

func TestHashPassword_UniqueSalts(t *testing.T) {
	hash1, err := HashPassword("same-password")
	if err != nil {
		t.Fatalf("HashPassword() error = %v", err)
	}
	hash2, err := HashPassword("same-password")
	if err != nil {
		t.Fatalf("HashPassword() error = %v", err)
	}

	parts1 := strings.Split(hash1, "$")
	parts2 := strings.Split(hash2, "$")

	if parts1[4] == parts2[4] {
		t.Error("Salt should be unique for each hash")
	}
}

func TestParsePHCString_Invalid(t *testing.T) {
	tests := []struct {
		name string
		s    string
	}{
		{name: "empty", s: ""},
		{name: "wrong parts", s: "$a$b$c"},
		{name: "wrong variant", s: "$argon2i$v=19$m=8,t=1,p=1$salt$hash"},
		{name: "wrong version", s: "$argon2id$v=18$m=8,t=1,p=1$salt$hash"},
		{name: "invalid params", s: "$argon2id$v=19$x=8$salt$hash"},
		{name: "low memory", s: "$argon2id$v=19$m=512,t=1,p=1$salt$hash"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if result := parsePHCString(tt.s); result != nil {
				t.Errorf("parsePHCString() = %v, want nil", result)
			}
		})
	}
}

func TestParsePHCString_Valid(t *testing.T) {
	result := parsePHCString("$argon2id$v=19$m=65536,t=3,p=2$salt$hash")
	if result == nil {
		t.Fatal("parsePHCString() = nil, want non-nil")
	}
	if result.memory != 65536 {
		t.Errorf("memory = %d, want 65536", result.memory)
	}
	if result.time != 3 {
		t.Errorf("time = %d, want 3", result.time)
	}
	if result.threads != 2 {
		t.Errorf("threads = %d, want 2", result.threads)
	}
}

func TestNeedsRehash(t *testing.T) {
	// A hash produced with current params → false.
	currentHash, err := HashPassword("test")
	if err != nil {
		t.Fatalf("HashPassword() error = %v", err)
	}
	if NeedsRehash(currentHash) {
		t.Error("NeedsRehash(current-params hash) = true, want false")
	}

	// A hash with weaker memory → true.
	weakMemHash := "$argon2id$v=19$m=32768,t=3,p=2$saltsalt$hashhashhashhashhashhash"
	if !NeedsRehash(weakMemHash) {
		t.Error("NeedsRehash(weaker-memory hash) = false, want true")
	}

	// A hash with weaker time → true.
	weakTimeHash := "$argon2id$v=19$m=65536,t=1,p=2$saltsalt$hashhashhashhashhashhash"
	if !NeedsRehash(weakTimeHash) {
		t.Error("NeedsRehash(weaker-time hash) = false, want true")
	}

	// A hash with weaker threads → true.
	weakThreadsHash := "$argon2id$v=19$m=65536,t=3,p=1$saltsalt$hashhashhashhashhashhash"
	if !NeedsRehash(weakThreadsHash) {
		t.Error("NeedsRehash(weaker-threads hash) = false, want true")
	}

	// A hash with stronger memory (non-standard) → true (doesn't match current).
	strongMemHash := "$argon2id$v=19$m=131072,t=3,p=2$saltsalt$hashhashhashhashhashhash"
	if !NeedsRehash(strongMemHash) {
		t.Error("NeedsRehash(stronger-memory hash) = false, want true")
	}

	// An unparseable hash → true.
	if !NeedsRehash("invalid-hash") {
		t.Error("NeedsRehash(invalid hash) = false, want true")
	}

	// Empty string → true.
	if !NeedsRehash("") {
		t.Error("NeedsRehash(empty) = false, want true")
	}
}
