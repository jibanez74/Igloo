package main

import (
	"context"
	"fmt"
	"os"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"
)

// hlsStorageCapacityError reports that the transcode directory is too full to
// start another session.
type hlsStorageCapacityError struct {
	FreeBytes     uint64
	RequiredBytes uint64
}

func (e *hlsStorageCapacityError) Error() string {
	return fmt.Sprintf(
		"not enough free space in the transcode directory to start playback (free: %d bytes, required: %d bytes)",
		e.FreeBytes, e.RequiredBytes,
	)
}

const (
	envHLSMaxCPUTranscodes        = "HLS_MAX_CPU_TRANSCODES"
	envHLSMaxSessionsPerUser      = "HLS_MAX_SESSIONS_PER_USER"
	hlsCPUTranscodeDefaultDivisor = 4
)

// hlsTranscodeAcquireWait bounds how long a session start parks for a transcode
// permit before falling back to the 503 + Retry-After path. A non-blocking
// acquire cannot guarantee progress: once segment serving is fast, running
// sessions never go idle long enough for same-owner LRU reclaim to free a
// permit, so a queued stream is rejected forever instead of waiting its turn.
// A var so tests can shrink the wait instead of sitting through it.
var hlsTranscodeAcquireWait = 15 * time.Second

type hlsTranscodeCapacityError struct {
	MaxActive int
}

func (e *hlsTranscodeCapacityError) Error() string {
	return fmt.Sprintf("server is already running the maximum number of CPU HLS transcodes (%d)", e.MaxActive)
}

type hlsPersonalSessionCapacityError struct {
	MaxActive int
}

func (e *hlsPersonalSessionCapacityError) Error() string {
	return fmt.Sprintf("user is already running the maximum number of personal HLS sessions (%d)", e.MaxActive)
}

type hlsTranscodeLimiter struct {
	permits chan struct{}
}

func newHLSTranscodeLimiter(maxActive int) *hlsTranscodeLimiter {
	if maxActive < 1 {
		maxActive = 1
	}
	return &hlsTranscodeLimiter{permits: make(chan struct{}, maxActive)}
}

func defaultHLSMaxCPUTranscodes() int {
	return max(1, runtime.NumCPU()/hlsCPUTranscodeDefaultDivisor)
}

func configuredHLSMaxCPUTranscodes() int {
	raw := strings.TrimSpace(os.Getenv(envHLSMaxCPUTranscodes))
	if raw == "" {
		return defaultHLSMaxCPUTranscodes()
	}
	value, err := strconv.Atoi(raw)
	if err != nil || value < 1 {
		return defaultHLSMaxCPUTranscodes()
	}
	return value
}

func configuredHLSMaxPersonalSessionsPerUser() int {
	raw := strings.TrimSpace(os.Getenv(envHLSMaxSessionsPerUser))
	if raw == "" {
		return hlsMaxPersonalSessionsPerUserDefault
	}
	value, err := strconv.Atoi(raw)
	if err != nil || value < 1 {
		return hlsMaxPersonalSessionsPerUserDefault
	}
	return value
}

// releaser returns the release closure for a permit this limiter just handed
// out. It is once-guarded so a double release cannot over-credit the pool.
func (l *hlsTranscodeLimiter) releaser() func() {
	var releaseOnce sync.Once
	return func() {
		releaseOnce.Do(func() {
			<-l.permits
		})
	}
}

func (l *hlsTranscodeLimiter) tryAcquire() (func(), error) {
	select {
	case l.permits <- struct{}{}:
		return l.releaser(), nil
	default:
		return nil, &hlsTranscodeCapacityError{MaxActive: cap(l.permits)}
	}
}

// acquire takes a permit, parking for up to wait if the pool is full. A wait of
// zero or less is exactly tryAcquire.
//
// Parking is a send on the permit channel rather than a poll: the runtime queues
// blocked senders in FIFO order and a release (the receive in releaser) hands the
// slot straight to the head of that queue, so admission is first-come-first-served
// with no window where a freed slot sits idle.
func (l *hlsTranscodeLimiter) acquire(ctx context.Context, wait time.Duration) (func(), error) {
	release, err := l.tryAcquire()
	if err == nil || wait <= 0 {
		return release, err
	}

	timer := time.NewTimer(wait)
	defer timer.Stop()

	select {
	case l.permits <- struct{}{}:
		return l.releaser(), nil
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-timer.C:
		return nil, &hlsTranscodeCapacityError{MaxActive: cap(l.permits)}
	}
}

// occupancy reports the held permits and the pool size, for logging.
func (l *hlsTranscodeLimiter) occupancy() (active, capacity int) {
	return len(l.permits), cap(l.permits)
}

// hlsTranscodeLimiterOrInstall returns the process limiter, installing a default
// one if startup did not. The lock matters because the install races two
// concurrent session starts, which is exactly the traffic this limiter governs.
func (app *Application) hlsTranscodeLimiterOrInstall() *hlsTranscodeLimiter {
	app.HLSTranscodeLimiterMu.Lock()
	defer app.HLSTranscodeLimiterMu.Unlock()

	if app.HLSTranscodeLimiter == nil {
		app.HLSTranscodeLimiter = newHLSTranscodeLimiter(defaultHLSMaxCPUTranscodes())
	}
	return app.HLSTranscodeLimiter
}

func (app *Application) acquireHLSTranscodeSlot(ctx context.Context, wait time.Duration) (func(), error) {
	limiter := app.hlsTranscodeLimiterOrInstall()

	release, err := limiter.acquire(ctx, wait)
	if err == nil {
		return release, nil
	}

	// A cancelled context is a client that went away, not a capacity refusal.
	if ctx.Err() != nil {
		return nil, err
	}

	active, capacity := limiter.occupancy()
	app.Logger.Warn("hls transcode limiter rejected",
		"active", active,
		"max", capacity,
		"waited_ms", wait.Milliseconds(),
	)
	return nil, err
}
