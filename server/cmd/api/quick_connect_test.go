package main

import (
	"errors"
	"strings"
	"testing"
	"time"
)

func TestQuickConnectBroker_PurgesExpiredEntries(t *testing.T) {
	broker := NewQuickConnectBroker()

	code, _, err := broker.Initiate("TV", "android_tv", "")
	if err != nil {
		t.Fatalf("initiate: %v", err)
	}

	broker.now = func() time.Time { return time.Now().Add(quickConnectCodeTTL + time.Second) }

	// Any operation triggers the lazy purge.
	if broker.Approve(code, 1) {
		t.Fatal("expected expired code to be unapprovable")
	}

	broker.mu.Lock()
	remaining := len(broker.entries)
	broker.mu.Unlock()
	if remaining != 0 {
		t.Fatalf("entries = %d, want 0 after purge", remaining)
	}
}

func TestQuickConnectBroker_RedeemLeavesEntryUntilConsumed(t *testing.T) {
	broker := NewQuickConnectBroker()

	code, secret, err := broker.Initiate("TV", "android_tv", "")
	if err != nil {
		t.Fatalf("initiate: %v", err)
	}
	if !broker.Approve(code, 1) {
		t.Fatal("approve failed")
	}

	result := broker.Redeem(code, secret)
	if result.status != redeemApproved {
		t.Fatalf("redeem status = %d, want redeemApproved", result.status)
	}

	// The first redeem holds the claim while it issues the token, so a
	// concurrent redeem must not issue a second one.
	result = broker.Redeem(code, secret)
	if result.status != redeemPending {
		t.Fatalf("redeem while claimed status = %d, want redeemPending", result.status)
	}

	// A failed token issuance must leave the code redeemable on the next poll.
	broker.Release(code)
	result = broker.Redeem(code, secret)
	if result.status != redeemApproved {
		t.Fatalf("redeem after release status = %d, want redeemApproved", result.status)
	}

	broker.Consume(code)

	result = broker.Redeem(code, secret)
	if result.status != redeemNotFound {
		t.Fatalf("redeem after consume status = %d, want redeemNotFound", result.status)
	}
}

func TestQuickConnectBroker_RejectsWhenAtCapacity(t *testing.T) {
	broker := NewQuickConnectBroker()

	for i := 0; i < quickConnectMaxPendingCodes; i++ {
		_, _, err := broker.Initiate("TV", "android_tv", "")
		if err != nil {
			t.Fatalf("initiate #%d: %v", i+1, err)
		}
	}

	_, _, err := broker.Initiate("TV", "android_tv", "")
	if !errors.Is(err, errQuickConnectCapacityReached) {
		t.Fatalf("initiate over capacity err = %v, want errQuickConnectCapacityReached", err)
	}

	// Once entries expire, capacity frees up again.
	broker.now = func() time.Time { return time.Now().Add(quickConnectCodeTTL + time.Second) }
	_, _, err = broker.Initiate("TV", "android_tv", "")
	if err != nil {
		t.Fatalf("initiate after expiry: %v", err)
	}
}

func TestGenerateQuickConnectCode_UsesAlphabetAndLength(t *testing.T) {
	const samples = 2000

	seen := make(map[rune]bool, len(quickConnectCodeAlphabet))

	for i := 0; i < samples; i++ {
		code := generateQuickConnectCode()

		if len(code) != quickConnectCodeLength {
			t.Fatalf("code %q length = %d, want %d", code, len(code), quickConnectCodeLength)
		}

		for _, symbol := range code {
			if !strings.ContainsRune(quickConnectCodeAlphabet, symbol) {
				t.Fatalf("code %q contains %q, which is outside the alphabet", code, symbol)
			}

			seen[symbol] = true
		}
	}

	// Rejection sampling must still be able to reach every symbol; a sampling
	// bug that drops the tail of the alphabet shows up here.
	for _, symbol := range quickConnectCodeAlphabet {
		if !seen[symbol] {
			t.Fatalf("symbol %q never generated across %d codes", symbol, samples)
		}
	}
}
