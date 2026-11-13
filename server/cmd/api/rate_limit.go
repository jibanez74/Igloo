package main

import (
	"net"
	"net/http"
	"net/netip"
	"sync"
	"time"

	"github.com/go-chi/chi/v5/middleware"
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

// clientIPMiddleware records each request's client IP for clientIP. With no
// trusted proxies the client is the TCP peer and forwarded headers are
// ignored, since any client can send them. With trusted proxies, a request
// whose peer is one of them names its client in X-Forwarded-For: the
// right-most entry that is not itself a trusted proxy. RemoteAddr is never
// rewritten, so the request log keeps the real peer.
func clientIPMiddleware(trustedProxies []netip.Prefix) func(http.Handler) http.Handler {
	if len(trustedProxies) == 0 {
		return middleware.ClientIPFromRemoteAddr
	}

	prefixes := make([]string, len(trustedProxies))
	for i, prefix := range trustedProxies {
		prefixes[i] = prefix.String()
	}
	return middleware.ClientIPFromXFF(prefixes...)
}

// clientIP returns the IP that rate limits key on, as clientIPMiddleware
// resolved it. When it resolved none (a trusted proxy sent no usable
// X-Forwarded-For), the peer stands in: the proxy's shared bucket is the
// safe answer, never a header value.
func clientIP(r *http.Request) string {
	if ip := middleware.GetClientIP(r.Context()); ip != "" {
		return ip
	}

	return remoteAddrIP(r.RemoteAddr)
}

// remoteAddrIP returns the host of remoteAddr, with an IPv4-mapped address
// folded to IPv4 as clientIPMiddleware stores it, so the fallback keys a
// client the same way.
func remoteAddrIP(remoteAddr string) string {
	host, _, err := net.SplitHostPort(remoteAddr)
	if err != nil {
		host = remoteAddr
	}
	if addr, err := netip.ParseAddr(host); err == nil {
		return addr.Unmap().String()
	}
	return host
}
