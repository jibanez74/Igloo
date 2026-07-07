package main

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"sync"
	"time"
)

const (
	quickConnectCodeTTL     = 5 * time.Minute
	quickConnectPollSeconds = 2
	quickConnectCodeLength  = 6

	// No I, L, O, 0 or 1 so codes are unambiguous on a TV screen.
	quickConnectCodeAlphabet = "ABCDEFGHJKMNPQRSTUVWXYZ23456789"

	deviceTokenPrefix = "igd_"

	deviceLastSeenTTL = 5 * time.Minute
)

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

// Redeem checks a code+secret pair. Approved entries are consumed so a token
// can only be issued once per code.
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

	delete(b.entries, code)
	return redeemResult{
		status:     redeemApproved,
		userID:     entry.approvedUserID,
		deviceName: entry.deviceName,
		platform:   entry.platform,
		appVersion: entry.appVersion,
	}
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
	buf := make([]byte, quickConnectCodeLength)
	_, err := rand.Read(buf)
	if err != nil {
		return "", err
	}

	code := make([]byte, quickConnectCodeLength)
	for i, v := range buf {
		code[i] = quickConnectCodeAlphabet[int(v)%len(quickConnectCodeAlphabet)]
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
