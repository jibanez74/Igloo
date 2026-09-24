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

	"igloo/cmd/internal/helpers"
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
	hlsCPUTranscodeDefaultDivisor = 4

	// hlsHWTranscodeDefault caps concurrent hardware video encodes. Three is
	// the historical consumer NVENC session limit and a safe floor for QSV
	// and VideoToolbox; HLS_MAX_HW_TRANSCODES raises it on hosts that allow
	// more.
	hlsHWTranscodeDefault = 3

	// hlsMaxPersonalSessionsPerUserDefault caps concurrent personal sessions per
	// user so abandoned clients cannot pile up ffmpeg processes and temp dirs.
	hlsMaxPersonalSessionsPerUserDefault = 3
)

// hlsTranscodeAcquireWait bounds how long a session start parks for a transcode
// permit before falling back to the 503 + Retry-After path. A non-blocking
// acquire cannot guarantee progress: once segment serving is fast, running
// sessions never go idle long enough for same-owner LRU reclaim to free a
// permit, so a queued stream is rejected forever instead of waiting its turn.
// A var so tests can shrink the wait instead of sitting through it.
var hlsTranscodeAcquireWait = 15 * time.Second

// hlsTranscodePool names the limiter a session's video encode draws from. A
// session that copies video holds no permit at all, whatever it does with
// audio: an audio-only encode is cheap and is already bounded by the per-user
// session cap.
type hlsTranscodePool int

const (
	hlsTranscodePoolNone hlsTranscodePool = iota
	hlsTranscodePoolCPU
	hlsTranscodePoolHardware
)

func (p hlsTranscodePool) String() string {
	switch p {
	case hlsTranscodePoolCPU:
		return "cpu"
	case hlsTranscodePoolHardware:
		return "hardware"
	default:
		return "none"
	}
}

// hlsTranscodePoolFor picks the pool from the effective encoder alone. A
// hardware encode that runs a software filter chain (tone mapping without
// CUDA filters, yadif) still draws from the hardware pool; see docs/ffmpeg.md.
func hlsTranscodePoolFor(copyVideo bool, effectiveDevice string) hlsTranscodePool {
	if copyVideo {
		return hlsTranscodePoolNone
	}
	if effectiveDevice == helpers.HARDWARE_ACCELERATION_DEVICE_CPU {
		return hlsTranscodePoolCPU
	}
	return hlsTranscodePoolHardware
}

type hlsTranscodeCapacityError struct {
	Pool      hlsTranscodePool
	MaxActive int
}

func (e *hlsTranscodeCapacityError) Error() string {
	return fmt.Sprintf("server is already running the maximum number of %s HLS transcodes (%d)", e.Pool, e.MaxActive)
}

type hlsPersonalSessionCapacityError struct {
	MaxActive int
}

func (e *hlsPersonalSessionCapacityError) Error() string {
	return fmt.Sprintf("user is already running the maximum number of personal HLS sessions (%d)", e.MaxActive)
}

type hlsTranscodeLimiter struct {
	pool    hlsTranscodePool
	permits chan struct{}
}

func newHLSTranscodeLimiter(pool hlsTranscodePool, maxActive int) *hlsTranscodeLimiter {
	if maxActive < 1 {
		maxActive = 1
	}
	return &hlsTranscodeLimiter{pool: pool, permits: make(chan struct{}, maxActive)}
}

func defaultHLSMaxCPUTranscodes() int {
	return max(1, runtime.NumCPU()/hlsCPUTranscodeDefaultDivisor)
}

// configuredPositiveInt reads a startup cap from the environment. Blank,
// non-numeric, zero, and negative values all mean the default: zero would
// disable the feature rather than mean "unlimited".
func configuredPositiveInt(env string, fallback int) int {
	raw := strings.TrimSpace(os.Getenv(env))
	if raw == "" {
		return fallback
	}
	value, err := strconv.Atoi(raw)
	if err != nil || value < 1 {
		return fallback
	}
	return value
}

func configuredHLSMaxCPUTranscodes() int {
	return configuredPositiveInt(envHLSMaxCPUTranscodes, defaultHLSMaxCPUTranscodes())
}

func configuredHLSMaxHWTranscodes() int {
	return configuredPositiveInt(envHLSMaxHWTranscodes, hlsHWTranscodeDefault)
}

func configuredHLSMaxPersonalSessionsPerUser() int {
	return configuredPositiveInt(envHLSMaxSessionsPerUser, hlsMaxPersonalSessionsPerUserDefault)
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

func (l *hlsTranscodeLimiter) capacityError() *hlsTranscodeCapacityError {
	return &hlsTranscodeCapacityError{Pool: l.pool, MaxActive: cap(l.permits)}
}

func (l *hlsTranscodeLimiter) tryAcquire() (func(), error) {
	select {
	case l.permits <- struct{}{}:
		return l.releaser(), nil
	default:
		return nil, l.capacityError()
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
		return nil, l.capacityError()
	}
}

// occupancy reports the held permits and the pool size, for logging.
func (l *hlsTranscodeLimiter) occupancy() (active, capacity int) {
	return len(l.permits), cap(l.permits)
}

// hlsTranscodeLimiterOrInstall returns the process limiter for a pool,
// installing a default one if startup did not. The lock matters because the
// install races two concurrent session starts, which is exactly the traffic
// this limiter governs.
func (app *Application) hlsTranscodeLimiterOrInstall(pool hlsTranscodePool) *hlsTranscodeLimiter {
	app.HLSTranscodeLimiterMu.Lock()
	defer app.HLSTranscodeLimiterMu.Unlock()

	if pool == hlsTranscodePoolHardware {
		if app.HLSHWTranscodeLimiter == nil {
			app.HLSHWTranscodeLimiter = newHLSTranscodeLimiter(pool, hlsHWTranscodeDefault)
		}
		return app.HLSHWTranscodeLimiter
	}

	if app.HLSCPUTranscodeLimiter == nil {
		app.HLSCPUTranscodeLimiter = newHLSTranscodeLimiter(hlsTranscodePoolCPU, defaultHLSMaxCPUTranscodes())
	}
	return app.HLSCPUTranscodeLimiter
}

func (app *Application) acquireHLSTranscodeSlot(ctx context.Context, pool hlsTranscodePool, wait time.Duration) (func(), error) {
	limiter := app.hlsTranscodeLimiterOrInstall(pool)

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
		"pool", pool.String(),
		"active", active,
		"max", capacity,
		"waited_ms", wait.Milliseconds(),
	)
	return nil, err
}
