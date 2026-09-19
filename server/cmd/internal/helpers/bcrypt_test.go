package helpers

import (
	"strings"
	"testing"

	"golang.org/x/crypto/bcrypt"
)

func TestHashPassword(t *testing.T) {
	password := "testPassword123"

	hashedPassword, err := HashPassword(password)
	if err != nil {
		t.Fatalf("HashPassword failed: %v", err)
	}

	if hashedPassword == "" {
		t.Error("HashPassword returned empty string")
	}

	if hashedPassword == password {
		t.Error("HashPassword returned the same string as input")
	}

	err = bcrypt.CompareHashAndPassword([]byte(hashedPassword), []byte(password))
	if err != nil {
		t.Errorf("Generated hash is not valid bcrypt hash: %v", err)
	}
}

func TestHashPassword_DifferentHashes(t *testing.T) {
	password := "samePassword"

	hash1, err := HashPassword(password)
	if err != nil {
		t.Fatalf("First HashPassword call failed: %v", err)
	}

	hash2, err := HashPassword(password)
	if err != nil {
		t.Fatalf("Second HashPassword call failed: %v", err)
	}

	if hash1 == hash2 {
		t.Error("HashPassword generated identical hashes for same password (should have different salts)")
	}
}

func TestPasswordMatches_CorrectPassword(t *testing.T) {
	password := "correctPassword123"

	hashedPassword, err := HashPassword(password)
	if err != nil {
		t.Fatalf("HashPassword failed: %v", err)
	}

	matches, err := PasswordMatches(password, hashedPassword)
	if err != nil {
		t.Fatalf("PasswordMatches failed: %v", err)
	}

	if !matches {
		t.Error("PasswordMatches returned false for correct password")
	}
}

func TestPasswordMatches_IncorrectPassword(t *testing.T) {
	password := "correctPassword123"
	wrongPassword := "wrongPassword456"

	hashedPassword, err := HashPassword(password)
	if err != nil {
		t.Fatalf("HashPassword failed: %v", err)
	}

	matches, err := PasswordMatches(wrongPassword, hashedPassword)
	if err != nil {
		t.Fatalf("PasswordMatches failed: %v", err)
	}

	if matches {
		t.Error("PasswordMatches returned true for incorrect password")
	}
}

func TestPasswordMatches_InvalidHash(t *testing.T) {
	_, err := PasswordMatches("password", "not-a-valid-hash")
	if err == nil {
		t.Error("PasswordMatches should return error for invalid hash")
	}
}

func TestPasswordMatches_EmptyHash(t *testing.T) {
	_, err := PasswordMatches("password", "")
	if err == nil {
		t.Error("PasswordMatches should return error for empty hash")
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
