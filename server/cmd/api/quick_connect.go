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
