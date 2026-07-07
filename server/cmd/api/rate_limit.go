package main

import (
	"net"
	"net/http"
	"sync"
	"time"
)

const rateLimiterPruneThreshold = 1024

type rateBucket struct {
	count       int
	windowStart time.Time
}

// rateLimiter is a minimal fixed-window limiter for auth endpoints. State is
// in-memory and per-process, which is sufficient for the single-binary home
// server deployment; it resets on restart.
type rateLimiter struct {
	mu      sync.Mutex
	buckets map[string]rateBucket
	now     func() time.Time
}

func newRateLimiter() *rateLimiter {
	return &rateLimiter{
		buckets: make(map[string]rateBucket),
		now:     time.Now,
	}
}

// Allow reports whether another attempt is permitted for key within the given
// fixed window, and records the attempt when it is.
func (l *rateLimiter) Allow(key string, limit int, window time.Duration) bool {
	l.mu.Lock()
	defer l.mu.Unlock()

	now := l.now()

	if len(l.buckets) > rateLimiterPruneThreshold {
		l.pruneLocked(now)
	}

	bucket, ok := l.buckets[key]
	if !ok || now.Sub(bucket.windowStart) >= window {
		l.buckets[key] = rateBucket{count: 1, windowStart: now}
		return true
	}

	if bucket.count >= limit {
		return false
	}

	bucket.count++
	l.buckets[key] = bucket
	return true
}

func (l *rateLimiter) pruneLocked(now time.Time) {
	// Buckets older than the longest window used anywhere (5 minutes) can no
	// longer influence a decision.
	const maxWindow = 5 * time.Minute
	for key, bucket := range l.buckets {
		if now.Sub(bucket.windowStart) >= maxWindow {
			delete(l.buckets, key)
		}
	}
}

// clientIP returns the request's remote IP. The chi RealIP middleware already
// rewrites RemoteAddr from proxy headers when present.
func clientIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}
