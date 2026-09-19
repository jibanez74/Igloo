package helpers

import (
	"errors"

	"golang.org/x/crypto/bcrypt"
)

// USER_PASSWORD_MAX_BYTES is bcrypt's own password limit. PasswordMatches
// rejects anything longer rather than letting bcrypt ignore the excess bytes,
// and the user handlers reject it at validation time for the same reason.
const USER_PASSWORD_MAX_BYTES = 72

func HashPassword(password string) (string, error) {
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}

	return string(hashedPassword), nil
}

func PasswordMatches(plainText, password string) (bool, error) {
	// bcrypt comparison otherwise ignores bytes beyond its password limit.
	passwordBytes := len(plainText)
	if passwordBytes > USER_PASSWORD_MAX_BYTES {
		return false, nil
	}

	err := bcrypt.CompareHashAndPassword([]byte(password), []byte(plainText))
	if err != nil {
		if errors.Is(err, bcrypt.ErrMismatchedHashAndPassword) {
			return false, nil
		}
		return false, err
	}

	return true, nil
}
