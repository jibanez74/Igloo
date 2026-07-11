package main

import (
	"fmt"
	"os"
	"runtime"
	"strconv"
	"strings"
	"sync"
)

const (
	envHLSMaxCPUTranscodes        = "HLS_MAX_CPU_TRANSCODES"
	hlsCPUTranscodeDefaultDivisor = 4
)

type hlsTranscodeCapacityError struct {
	MaxActive int
}

func (e *hlsTranscodeCapacityError) Error() string {
	return fmt.Sprintf("server is already running the maximum number of CPU HLS transcodes (%d)", e.MaxActive)
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
