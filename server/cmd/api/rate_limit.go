package main

import (
	"context"
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

type clientSocketIPContextKey struct{}

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
		l.evictOverflowLocked()
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

// evictOverflowLocked drops arbitrary buckets until the map is back at the
// threshold. Under a flood from many distinct IPs, bounded memory matters
// more than perfect per-key limiting; an evicted key merely restarts its
// window on the next attempt.
func (l *rateLimiter) evictOverflowLocked() {
	for key := range l.buckets {
		if len(l.buckets) <= rateLimiterPruneThreshold {
			return
		}
		delete(l.buckets, key)
	}
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

func preserveClientSocketIP(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := context.WithValue(r.Context(), clientSocketIPContextKey{}, remoteAddrIP(r.RemoteAddr))
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// clientIP returns the TCP peer IP captured before chi's RealIP middleware can
// rewrite RemoteAddr from spoofable forwarded headers.
func clientIP(r *http.Request) string {
	if ip, ok := r.Context().Value(clientSocketIPContextKey{}).(string); ok && ip != "" {
		return ip
	}

	return remoteAddrIP(r.RemoteAddr)
}

func remoteAddrIP(remoteAddr string) string {
	host, _, err := net.SplitHostPort(remoteAddr)
	if err != nil {
		return remoteAddr
	}
	return host
}
