package main

import (
	"fmt"
	"os"
	"runtime"
	"strconv"
	"strings"
	"sync"
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

func (l *hlsTranscodeLimiter) tryAcquire() (func(), error) {
	if l == nil {
		return func() {}, nil
	}
	select {
	case l.permits <- struct{}{}:
		var releaseOnce sync.Once
		return func() {
			releaseOnce.Do(func() {
				<-l.permits
			})
		}, nil
	default:
		return nil, &hlsTranscodeCapacityError{MaxActive: cap(l.permits)}
	}
}

func (app *Application) acquireHLSTranscodeSlot() (func(), error) {
	if app.HLSTranscodeLimiter == nil {
		app.HLSTranscodeLimiter = newHLSTranscodeLimiter(defaultHLSMaxCPUTranscodes())
	}
	return app.HLSTranscodeLimiter.tryAcquire()
}
