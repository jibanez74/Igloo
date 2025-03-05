package helpers

import (
	"crypto/rand"
	"encoding/base32"
)

// GenerateRandomString generates a random string of specified length
func GenerateRandomString(length int) string {
	randomBytes := make([]byte, length)
	rand.Read(randomBytes)

	// Use base32 encoding (which is safe for URLs and user input)
	// and remove padding characters
	return base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(randomBytes)[:length]
}
