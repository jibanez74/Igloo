package helpers

import (
	"crypto/rand"
	"encoding/base32"
)

func GenerateRandomString(length int) string {
	randomBytes := make([]byte, length)
	rand.Read(randomBytes)
	return base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(randomBytes)[:length]
}
