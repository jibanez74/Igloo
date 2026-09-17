package main

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"time"
)

// A device token is the TV and native client's credential: a bearer token
// minted once at pairing, persisted only as a hash, and revoked once the device
// goes quiet. Quick connect issues them; DeviceTokenAuth, the devices list and
// the daily expiry sweep all read the values below.
const (
	deviceTokenPrefix   = "igd_"
	maxDeviceNameLength = 100

	deviceLastSeenTTL = 5 * time.Minute

	// Bearer tokens are resolved on every request, including each HLS segment
	// a TV client fetches, so the lookup is cached. Revocation evicts
	// explicitly; the TTL only bounds paths that delete a device without going
	// through a handler, such as the stale-device sweep.
	deviceAuthCacheTTL = 30 * time.Second

	// Devices whose last_used_at is older than this are revoked automatically,
	// both lazily at auth time and by the daily sweep.
	deviceInactivityTTL = 90 * 24 * time.Hour

	// Format produced by SQLite's CURRENT_TIMESTAMP (UTC, zero-padded).
	sqliteTimeLayout = "2006-01-02 15:04:05"
)

// deviceInactivityCutoff returns the oldest last_used_at still considered
// active, in SQLite CURRENT_TIMESTAMP format. Both sides are zero-padded
// "YYYY-MM-DD HH:MM:SS" UTC strings, so plain string comparison orders
// chronologically.
func deviceInactivityCutoff(now time.Time) string {
	return now.UTC().Add(-deviceInactivityTTL).Format(sqliteTimeLayout)
}

// generateDeviceToken returns a new bearer token and the hex-encoded SHA-256
// hash that is stored in the database. The plaintext token is never persisted.
func generateDeviceToken() (string, string, error) {
	buf := make([]byte, 32)
	_, err := rand.Read(buf)
	if err != nil {
		return "", "", err
	}

	token := deviceTokenPrefix + base64.RawURLEncoding.EncodeToString(buf)
	return token, hashDeviceToken(token), nil
}

func hashDeviceToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}
