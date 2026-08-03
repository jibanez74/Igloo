package main

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"sync"
	"time"
)

var errQuickConnectCapacityReached = errors.New("quick connect pending-code capacity reached")

const (
	quickConnectCodeTTL     = 5 * time.Minute
	quickConnectPollSeconds = 2
	quickConnectCodeLength  = 6

	// Hard ceiling on pending codes so the public initiate endpoint cannot
	// grow the map unbounded from many source IPs. Legitimate use is a
	// handful of codes; the per-IP limiter caps one IP at ~50 live entries.
	quickConnectMaxPendingCodes = 1000

	// No I, L, O, 0 or 1 so codes are unambiguous on a TV screen.
	quickConnectCodeAlphabet = "ABCDEFGHJKMNPQRSTUVWXYZ23456789"

	deviceTokenPrefix = "igd_"

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

type quickConnectEntry struct {
	secretHash     [32]byte
	deviceName     string
	platform       string
	appVersion     string
	approvedUserID int64
	expiresAt      time.Time
}

// QuickConnectBroker holds pending quick-connect codes in memory. Codes are
// short-lived pairing ephemera, so they are not persisted: a restart simply
// forces the device to request a new code. Expired entries are purged lazily
// on every operation, so no background cleanup is needed.
type QuickConnectBroker struct {
	mu      sync.Mutex
	entries map[string]*quickConnectEntry
	now     func() time.Time
}

func NewQuickConnectBroker() *QuickConnectBroker {
	return &QuickConnectBroker{
		entries: make(map[string]*quickConnectEntry),
		now:     time.Now,
	}
}

// Initiate registers a new pending code and returns it together with the
// device-held secret required to redeem it once approved.
func (b *QuickConnectBroker) Initiate(deviceName, platform, appVersion string) (string, string, error) {
	secretBytes := make([]byte, 32)
	_, err := rand.Read(secretBytes)
	if err != nil {
		return "", "", err
	}
	secret := base64.RawURLEncoding.EncodeToString(secretBytes)

	b.mu.Lock()
	defer b.mu.Unlock()
	b.purgeExpiredLocked()

	if len(b.entries) >= quickConnectMaxPendingCodes {
		return "", "", errQuickConnectCapacityReached
	}

	var code string
	for {
		code, err = generateQuickConnectCode()
		if err != nil {
			return "", "", err
		}
		_, exists := b.entries[code]
		if !exists {
			break
		}
	}

	b.entries[code] = &quickConnectEntry{
		secretHash: sha256.Sum256([]byte(secret)),
		deviceName: deviceName,
		platform:   platform,
		appVersion: appVersion,
		expiresAt:  b.now().Add(quickConnectCodeTTL),
	}

	return code, secret, nil
}

// Approve binds a pending code to the approving user. It returns false when
// the code is unknown, expired, or already approved.
func (b *QuickConnectBroker) Approve(code string, userID int64) bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.purgeExpiredLocked()

	entry, ok := b.entries[code]
	if !ok {
		return false
	}
	if entry.approvedUserID != 0 {
		return false
	}

	entry.approvedUserID = userID
	return true
}

type quickConnectDeviceInfo struct {
	deviceName string
	platform   string
	appVersion string
}

// Lookup returns the pending device's metadata without binding or consuming
// the code, so the approving user can see what they are approving. ok is
// false for unknown, expired, or already-approved codes — exactly the
// conditions under which Approve fails, so lookup leaks nothing extra.
func (b *QuickConnectBroker) Lookup(code string) (quickConnectDeviceInfo, bool) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.purgeExpiredLocked()

	entry, ok := b.entries[code]
	if !ok {
		return quickConnectDeviceInfo{}, false
	}
	if entry.approvedUserID != 0 {
		return quickConnectDeviceInfo{}, false
	}

	return quickConnectDeviceInfo{
		deviceName: entry.deviceName,
		platform:   entry.platform,
		appVersion: entry.appVersion,
	}, true
}

type redeemStatus int

const (
	redeemNotFound redeemStatus = iota
	redeemPending
	redeemApproved
)

type redeemResult struct {
	status     redeemStatus
	userID     int64
	deviceName string
	platform   string
	appVersion string
}

// Redeem checks a code+secret pair. Approved entries are NOT consumed here:
// the caller must call Consume once the device token has been durably issued,
// so a failed issuance leaves the code redeemable on the device's next poll.
func (b *QuickConnectBroker) Redeem(code, secret string) redeemResult {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.purgeExpiredLocked()

	entry, ok := b.entries[code]
	if !ok {
		return redeemResult{status: redeemNotFound}
	}

	secretHash := sha256.Sum256([]byte(secret))
	if subtle.ConstantTimeCompare(secretHash[:], entry.secretHash[:]) != 1 {
		return redeemResult{status: redeemNotFound}
	}

	if entry.approvedUserID == 0 {
		return redeemResult{status: redeemPending}
	}

	return redeemResult{
		status:     redeemApproved,
		userID:     entry.approvedUserID,
		deviceName: entry.deviceName,
		platform:   entry.platform,
		appVersion: entry.appVersion,
	}
}

// Consume removes a code after its device token has been issued, so the code
// cannot mint another token. Concurrent redeems of the same code before
// Consume could each issue a token, but both need the device-held secret and
// the device polls sequentially; worst case is an extra device row the user
// can revoke.
func (b *QuickConnectBroker) Consume(code string) {
	b.mu.Lock()
	defer b.mu.Unlock()
	delete(b.entries, code)
}

func (b *QuickConnectBroker) purgeExpiredLocked() {
	now := b.now()
	for code, entry := range b.entries {
		if now.After(entry.expiresAt) {
			delete(b.entries, code)
		}
	}
}

func generateQuickConnectCode() (string, error) {
	// 256 is not a multiple of the alphabet length, so plain modulo would make
	// the first 256%len symbols more likely. Bytes at or above the largest
	// whole multiple are rejected instead, which keeps every symbol equally
	// likely at the cost of redrawing ~3% of the time.
	unbiasedLimit := 256 - (256 % len(quickConnectCodeAlphabet))

	code := make([]byte, 0, quickConnectCodeLength)
	buf := make([]byte, quickConnectCodeLength)

	for len(code) < quickConnectCodeLength {
		_, err := rand.Read(buf)
		if err != nil {
			return "", err
		}

		for _, v := range buf {
			if int(v) >= unbiasedLimit {
				continue
			}

			code = append(code, quickConnectCodeAlphabet[int(v)%len(quickConnectCodeAlphabet)])
			if len(code) == quickConnectCodeLength {
				break
			}
		}
	}

	return string(code), nil
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
