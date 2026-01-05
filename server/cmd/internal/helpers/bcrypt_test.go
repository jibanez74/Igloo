package helpers

import (
	"testing"

	"golang.org/x/crypto/bcrypt"
)

func TestHashPassword(t *testing.T) {
	password := "testPassword123"

	hashedPassword, err := HashPassword(password)
	if err != nil {
		t.Fatalf("HashPassword failed: %v", err)
	}

	// Verify hash is not empty
	if hashedPassword == "" {
		t.Error("HashPassword returned empty string")
	}

	// Verify hash is different from original password
	if hashedPassword == password {
		t.Error("HashPassword returned the same string as input")
	}

	// Verify hash can be validated by bcrypt
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

	// Each hash should be unique due to random salt
	if hash1 == hash2 {
		t.Error("HashPassword generated identical hashes for same password (should have different salts)")
	}
}

func TestHashPassword_EmptyPassword(t *testing.T) {
	hashedPassword, err := HashPassword("")
	if err != nil {
		t.Fatalf("HashPassword failed with empty password: %v", err)
	}

	// Empty password should still produce a valid hash
	if hashedPassword == "" {
		t.Error("HashPassword returned empty string for empty password")
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

func TestPasswordMatches_EmptyPassword(t *testing.T) {
	hashedPassword, err := HashPassword("somePassword")
	if err != nil {
		t.Fatalf("HashPassword failed: %v", err)
	}

	matches, err := PasswordMatches("", hashedPassword)
	if err != nil {
		t.Fatalf("PasswordMatches failed: %v", err)
	}

	if matches {
		t.Error("PasswordMatches returned true for empty password")
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

