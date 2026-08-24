package main

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"
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

func TestHLSTranscodeLimiterAcquire(t *testing.T) {
	// The whole point of the wait: a full pool must admit the next request when a
	// permit frees, not reject it. Reclaim cannot cover this — every permit here
	// belongs to a session that is genuinely running.
	t.Run("a parked waiter is admitted when a permit frees", func(t *testing.T) {
		limiter := newHLSTranscodeLimiter(1)

		held, err := limiter.tryAcquire()
		if err != nil {
			t.Fatalf("tryAcquire: %v", err)
		}

		var (
			wg      sync.WaitGroup
			release func()
			waitErr error
		)
		wg.Add(1)
		go func() {
			defer wg.Done()
			release, waitErr = limiter.acquire(context.Background(), 5*time.Second)
		}()

		held()
		wg.Wait()

		if waitErr != nil {
			t.Fatalf("acquire after release: %v", waitErr)
		}
		if release == nil {
			t.Fatal("acquire returned a nil release func")
		}

		// The handed-off permit must still be a real one: releasing it frees the
		// pool exactly once.
		release()
		final, err := limiter.tryAcquire()
		if err != nil {
			t.Fatalf("tryAcquire after the waiter released: %v", err)
		}
		final()
	})

	// The starvation this replaces: a queued stream was refused forever while
	// others held the pool. Every waiter must get in as permits recycle.
	t.Run("every waiter is admitted as permits recycle", func(t *testing.T) {
		limiter := newHLSTranscodeLimiter(1)

		held, err := limiter.tryAcquire()
		if err != nil {
			t.Fatalf("tryAcquire: %v", err)
		}

		const waiters = 5
		admitted := make(chan struct{}, waiters)
		var wg sync.WaitGroup

		for range waiters {
			wg.Add(1)
			go func() {
				defer wg.Done()
				release, acquireErr := limiter.acquire(context.Background(), 10*time.Second)
				if acquireErr != nil {
					return
				}
				admitted <- struct{}{}
				release()
			}()
		}

		held()
		wg.Wait()

		if len(admitted) != waiters {
			t.Fatalf("admitted %d of %d waiters", len(admitted), waiters)
		}
	})

	t.Run("a cancelled context stops the wait and leaks no permit", func(t *testing.T) {
		limiter := newHLSTranscodeLimiter(1)

		held, err := limiter.tryAcquire()
		if err != nil {
			t.Fatalf("tryAcquire: %v", err)
		}

		ctx, cancel := context.WithCancel(context.Background())
		var (
			wg      sync.WaitGroup
			waitErr error
		)
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, waitErr = limiter.acquire(ctx, time.Minute)
		}()

		cancel()
		wg.Wait()

		if !errors.Is(waitErr, context.Canceled) {
			t.Fatalf("acquire error = %v, want context.Canceled", waitErr)
		}

		// The abandoned send must not have credited the pool.
		held()
		release, err := limiter.tryAcquire()
		if err != nil {
			t.Fatalf("tryAcquire after the cancelled wait: %v", err)
		}
		defer release()
		active, capacity := limiter.occupancy()
		if active != 1 || capacity != 1 {
			t.Fatalf("occupancy = %d/%d, want 1/1", active, capacity)
		}
	})

	t.Run("an exhausted wait returns the capacity error", func(t *testing.T) {
		limiter := newHLSTranscodeLimiter(2)

		for i := range 2 {
			release, err := limiter.tryAcquire()
			if err != nil {
				t.Fatalf("tryAcquire %d: %v", i, err)
			}
			defer release()
		}

		_, err := limiter.acquire(context.Background(), 20*time.Millisecond)
		var capacityErr *hlsTranscodeCapacityError
		if !errors.As(err, &capacityErr) {
			t.Fatalf("acquire error = %v, want hlsTranscodeCapacityError", err)
		}
		if capacityErr.MaxActive != 2 {
			t.Fatalf("MaxActive = %d, want 2", capacityErr.MaxActive)
		}
	})

	// Callers that must not park (room warm-up, the first admission attempt)
	// pass a zero budget and need the old instant refusal.
	t.Run("a non-positive wait refuses immediately", func(t *testing.T) {
		limiter := newHLSTranscodeLimiter(1)

		release, err := limiter.tryAcquire()
		if err != nil {
			t.Fatalf("tryAcquire: %v", err)
		}
		defer release()

		started := time.Now()
		_, err = limiter.acquire(context.Background(), 0)
		var capacityErr *hlsTranscodeCapacityError
		if !errors.As(err, &capacityErr) {
			t.Fatalf("acquire error = %v, want hlsTranscodeCapacityError", err)
		}
		if elapsed := time.Since(started); elapsed > time.Second {
			t.Fatalf("a zero-wait acquire took %s, want an immediate refusal", elapsed)
		}
	})
}

func TestAcquireHLSTranscodeSlot_InstallsMissingLimiter(t *testing.T) {
	app := setupTestApp(t)
	defer app.DB.Close()
	app.HLSTranscodeLimiter = nil

	release, err := app.acquireHLSTranscodeSlot(context.Background(), 0)
	if err != nil {
		t.Fatalf("acquireHLSTranscodeSlot: %v", err)
	}
	defer release()

	if app.HLSTranscodeLimiter == nil {
		t.Fatal("acquireHLSTranscodeSlot did not install a limiter")
	}
}

// withTestHLSTranscodeAcquireWait shrinks the permit wait for tests that drive a
// full pool through GetOrCreateHLSSession, so they assert the refusal without
// sitting out the production budget.
func withTestHLSTranscodeAcquireWait(t *testing.T, wait time.Duration) {
	t.Helper()

	original := hlsTranscodeAcquireWait
	hlsTranscodeAcquireWait = wait
	t.Cleanup(func() {
		hlsTranscodeAcquireWait = original
	})
}
