package helpers

import (
	"strings"
	"testing"
)

func TestPasswordMatches_IncorrectPassword(t *testing.T) {
	hashedPassword, err := HashPassword("correctPassword123")
	if err != nil {
		t.Fatalf("HashPassword failed: %v", err)
	}

	matches, err := PasswordMatches("wrongPassword456", hashedPassword)
	if err != nil {
		t.Fatalf("PasswordMatches failed: %v", err)
	}
	if matches {
		t.Error("PasswordMatches returned true for incorrect password")
	}
}

// A hash that bcrypt cannot read is an error, not a mismatch: the login
// handlers turn it into a 500 rather than a 401.
func TestPasswordMatches_InvalidHash(t *testing.T) {
	matches, err := PasswordMatches("password", "not-a-valid-hash")
	if err == nil {
		t.Fatal("PasswordMatches should return error for invalid hash")
	}
	if matches {
		t.Error("PasswordMatches returned true alongside an error")
	}
}

// bcrypt ignores bytes past its own password limit, so two distinct passwords
// sharing a 72-byte prefix would otherwise verify against the same hash.
func TestPasswordMatches_AtTheByteLimit(t *testing.T) {
	atLimit := strings.Repeat("a", USER_PASSWORD_MAX_BYTES)
	hash, err := HashPassword(atLimit)
	if err != nil {
		t.Fatalf("HashPassword returned error: %v", err)
	}

	matches, err := PasswordMatches(atLimit, hash)
	if err != nil {
		t.Fatalf("PasswordMatches returned error: %v", err)
	}
	if !matches {
		t.Errorf("a %d-byte password did not verify against its own hash", USER_PASSWORD_MAX_BYTES)
	}

	overLimit := atLimit + "b"
	matches, err = PasswordMatches(overLimit, hash)
	if err != nil {
		t.Fatalf("PasswordMatches returned error: %v", err)
	}
	if matches {
		t.Errorf("a %d-byte password verified against the hash of its %d-byte prefix",
			len(overLimit), USER_PASSWORD_MAX_BYTES)
	}
}
