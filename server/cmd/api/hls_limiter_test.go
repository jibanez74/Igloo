package main

import (
	"errors"
	"testing"
)

func TestConfiguredHLSMaxCPUTranscodes(t *testing.T) {
	tests := []struct {
		name string
		raw  string
		want int
	}{
		{name: "unset falls back to the CPU-derived default", raw: "", want: defaultHLSMaxCPUTranscodes()},
		{name: "explicit limit is honoured", raw: "2", want: 2},
		{name: "surrounding whitespace is trimmed", raw: " 3 ", want: 3},
		{name: "non-numeric value falls back", raw: "abc", want: defaultHLSMaxCPUTranscodes()},
		// Zero would disable transcoding entirely rather than mean "unlimited".
		{name: "zero falls back", raw: "0", want: defaultHLSMaxCPUTranscodes()},
		{name: "negative falls back", raw: "-4", want: defaultHLSMaxCPUTranscodes()},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv(envHLSMaxCPUTranscodes, tt.raw)

			got := configuredHLSMaxCPUTranscodes()
			if got != tt.want {
				t.Fatalf("configuredHLSMaxCPUTranscodes() = %d, want %d", got, tt.want)
			}
		})
	}
}

func TestConfiguredHLSMaxPersonalSessionsPerUser(t *testing.T) {
	tests := []struct {
		name string
		raw  string
		want int
	}{
		{name: "unset falls back to the default", raw: "", want: hlsMaxPersonalSessionsPerUserDefault},
		{name: "explicit limit is honoured", raw: "5", want: 5},
		{name: "surrounding whitespace is trimmed", raw: " 1 ", want: 1},
		{name: "non-numeric value falls back", raw: "many", want: hlsMaxPersonalSessionsPerUserDefault},
		{name: "zero falls back", raw: "0", want: hlsMaxPersonalSessionsPerUserDefault},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv(envHLSMaxSessionsPerUser, tt.raw)

			got := configuredHLSMaxPersonalSessionsPerUser()
			if got != tt.want {
				t.Fatalf("configuredHLSMaxPersonalSessionsPerUser() = %d, want %d", got, tt.want)
			}
		})
	}
}

func TestHLSTranscodeLimiter(t *testing.T) {
	t.Run("refuses a permit once the pool is full", func(t *testing.T) {
		limiter := newHLSTranscodeLimiter(2)

		first, err := limiter.tryAcquire()
		if err != nil {
			t.Fatalf("first tryAcquire: %v", err)
		}
		_, err = limiter.tryAcquire()
		if err != nil {
			t.Fatalf("second tryAcquire: %v", err)
		}

		_, err = limiter.tryAcquire()
		var capacityErr *hlsTranscodeCapacityError
		if !errors.As(err, &capacityErr) {
			t.Fatalf("third tryAcquire error = %v, want hlsTranscodeCapacityError", err)
		}
		if capacityErr.MaxActive != 2 {
			t.Fatalf("MaxActive = %d, want 2", capacityErr.MaxActive)
		}

		first()
		_, err = limiter.tryAcquire()
		if err != nil {
			t.Fatalf("tryAcquire after release: %v", err)
		}
	})

	// A release func can be called from both a defer and an explicit teardown
	// path. Crediting the pool twice would let the server run more concurrent
	// transcodes than the machine was sized for.
	t.Run("a repeated release returns only one permit", func(t *testing.T) {
		limiter := newHLSTranscodeLimiter(1)

		release, err := limiter.tryAcquire()
		if err != nil {
			t.Fatalf("tryAcquire: %v", err)
		}
		release()
		release()

		_, err = limiter.tryAcquire()
		if err != nil {
			t.Fatalf("tryAcquire after double release: %v", err)
		}

		_, err = limiter.tryAcquire()
		var capacityErr *hlsTranscodeCapacityError
		if !errors.As(err, &capacityErr) {
			t.Fatalf("second tryAcquire error = %v, want the pool to be full again", err)
		}
	})

	// A zero-capacity channel would refuse every transcode forever, so the
	// constructor clamps rather than trusting its caller.
	t.Run("a non-positive size still yields one permit", func(t *testing.T) {
		limiter := newHLSTranscodeLimiter(0)

		release, err := limiter.tryAcquire()
		if err != nil {
			t.Fatalf("tryAcquire: %v", err)
		}
		defer release()

		_, err = limiter.tryAcquire()
		var capacityErr *hlsTranscodeCapacityError
		if !errors.As(err, &capacityErr) {
			t.Fatalf("second tryAcquire error = %v, want capacity error", err)
		}
		if capacityErr.MaxActive != 1 {
			t.Fatalf("MaxActive = %d, want the clamped 1", capacityErr.MaxActive)
		}
	})
}

func TestAcquireHLSTranscodeSlot_InstallsMissingLimiter(t *testing.T) {
	app := setupTestApp(t)
	defer app.DB.Close()
	app.HLSTranscodeLimiter = nil

	release, err := app.acquireHLSTranscodeSlot()
	if err != nil {
		t.Fatalf("acquireHLSTranscodeSlot: %v", err)
	}
	defer release()

	if app.HLSTranscodeLimiter == nil {
		t.Fatal("acquireHLSTranscodeSlot did not install a limiter")
	}
}
