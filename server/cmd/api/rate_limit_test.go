package main

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"slices"
	"sync"
	"testing"
	"time"
)

// resolveClientIP runs one request through clientIPMiddleware and returns
// what clientIP saw, along with RemoteAddr as the handler received it.
func resolveClientIP(t *testing.T, trustedProxies string, remoteAddr string, headers http.Header) (ip string, seenRemoteAddr string) {
	t.Helper()

	prefixes, err := parseTrustedProxies(trustedProxies)
	if err != nil {
		t.Fatalf("parseTrustedProxies(%q): %v", trustedProxies, err)
	}

	handler := clientIPMiddleware(prefixes)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ip = clientIP(r)
		seenRemoteAddr = r.RemoteAddr
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = remoteAddr
	for name, values := range headers {
		for _, value := range values {
			req.Header.Add(name, value)
		}
	}
	handler.ServeHTTP(httptest.NewRecorder(), req)
	return ip, seenRemoteAddr
}

func TestClientIPMiddleware(t *testing.T) {
	spoofed := http.Header{
		"X-Forwarded-For": {"203.0.113.44"},
		"X-Real-Ip":       {"203.0.113.45"},
		"True-Client-Ip":  {"203.0.113.46"},
	}

	tests := []struct {
		name           string
		trustedProxies string
		remoteAddr     string
		headers        http.Header
		want           string
	}{
		{
			name:       "no trusted proxies ignores forwarded headers",
			remoteAddr: "198.51.100.7:54321",
			headers:    spoofed,
			want:       "198.51.100.7",
		},
		{
			name:       "no trusted proxies reads an IPv6 peer",
			remoteAddr: "[2001:db8::1]:12345",
			want:       "2001:db8::1",
		},
		{
			name:       "IPv4-mapped peer folds to IPv4",
			remoteAddr: "[::ffff:198.51.100.7]:54321",
			want:       "198.51.100.7",
		},
		{
			name:           "untrusted peer ignores forwarded headers",
			trustedProxies: "10.0.0.2",
			remoteAddr:     "198.51.100.7:54321",
			headers:        spoofed,
			want:           "198.51.100.7",
		},
		{
			name:           "trusted peer names its client",
			trustedProxies: "10.0.0.2",
			remoteAddr:     "10.0.0.2:40000",
			headers:        http.Header{"X-Forwarded-For": {"203.0.113.10"}},
			want:           "203.0.113.10",
		},
		{
			name:           "trusted peer behind an IPv4-mapped listener",
			trustedProxies: "127.0.0.1",
			remoteAddr:     "[::ffff:127.0.0.1]:40000",
			headers:        http.Header{"X-Forwarded-For": {"203.0.113.10"}},
			want:           "203.0.113.10",
		},
		{
			name:           "client-sent entries left of the client are ignored",
			trustedProxies: "10.0.0.2",
			remoteAddr:     "10.0.0.2:40000",
			headers:        http.Header{"X-Forwarded-For": {"192.0.2.99, 203.0.113.10"}},
			want:           "203.0.113.10",
		},
		{
			name:           "trusted inner proxy is skipped",
			trustedProxies: "10.0.0.0/24",
			remoteAddr:     "10.0.0.2:40000",
			headers:        http.Header{"X-Forwarded-For": {"192.0.2.99, 203.0.113.10, 10.0.0.3"}},
			want:           "203.0.113.10",
		},
		{
			name:           "duplicate headers merge in order",
			trustedProxies: "10.0.0.2",
			remoteAddr:     "10.0.0.2:40000",
			headers:        http.Header{"X-Forwarded-For": {"192.0.2.99", "203.0.113.10"}},
			want:           "203.0.113.10",
		},
		{
			name:           "trusted peer ignores other forwarded headers",
			trustedProxies: "10.0.0.2",
			remoteAddr:     "10.0.0.2:40000",
			headers:        http.Header{"X-Real-Ip": {"203.0.113.45"}, "True-Client-Ip": {"203.0.113.46"}},
			want:           "10.0.0.2",
		},
		{
			name:           "trusted peer without X-Forwarded-For stays the client",
			trustedProxies: "10.0.0.2",
			remoteAddr:     "10.0.0.2:40000",
			want:           "10.0.0.2",
		},
		{
			name:           "fallback to an IPv4-mapped trusted peer folds to IPv4",
			trustedProxies: "10.0.0.2",
			remoteAddr:     "[::ffff:10.0.0.2]:40000",
			want:           "10.0.0.2",
		},
		{
			name:           "chain of only trusted hops stays the peer",
			trustedProxies: "10.0.0.0/24",
			remoteAddr:     "10.0.0.2:40000",
			headers:        http.Header{"X-Forwarded-For": {"10.0.0.3"}},
			want:           "10.0.0.2",
		},
		{
			name:           "malformed entry fails closed to the peer",
			trustedProxies: "10.0.0.2",
			remoteAddr:     "10.0.0.2:40000",
			headers:        http.Header{"X-Forwarded-For": {"203.0.113.10, not-an-ip"}},
			want:           "10.0.0.2",
		},
		{
			name:           "IPv6 trusted proxy",
			trustedProxies: "::1",
			remoteAddr:     "[::1]:40000",
			headers:        http.Header{"X-Forwarded-For": {"2001:db8::44"}},
			want:           "2001:db8::44",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, seenRemoteAddr := resolveClientIP(t, tt.trustedProxies, tt.remoteAddr, tt.headers)
			if got != tt.want {
				t.Fatalf("clientIP() = %q, want %q", got, tt.want)
			}
			if seenRemoteAddr != tt.remoteAddr {
				t.Fatalf("RemoteAddr rewritten to %q, want %q", seenRemoteAddr, tt.remoteAddr)
			}
		})
	}
}

// loginAttempts sends one attempt per X-Forwarded-For value from remoteAddr
// through clientIPMiddleware into a limiter that admits one attempt per IP,
// and returns the status codes.
func loginAttempts(t *testing.T, trustedProxies string, remoteAddr string, forwardedFor ...string) []int {
	t.Helper()

	prefixes, err := parseTrustedProxies(trustedProxies)
	if err != nil {
		t.Fatalf("parseTrustedProxies(%q): %v", trustedProxies, err)
	}

	limiter := newRateLimiter()
	handler := clientIPMiddleware(prefixes)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !limiter.Allow("login:"+clientIP(r), 1, time.Minute) {
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))

	codes := make([]int, 0, len(forwardedFor))
	for _, value := range forwardedFor {
		req := httptest.NewRequest(http.MethodPost, "/api/auth/login", nil)
		req.RemoteAddr = remoteAddr
		req.Header.Set("X-Forwarded-For", value)

		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)
		codes = append(codes, w.Code)
	}
	return codes
}

func TestRateLimiterCannotBeBypassedByForwardedForRotation(t *testing.T) {
	codes := loginAttempts(t, "", "198.51.100.7:54321", "203.0.113.10", "203.0.113.11")
	if !slices.Equal(codes, []int{http.StatusOK, http.StatusTooManyRequests}) {
		t.Fatalf("statuses = %v, want [200 429]", codes)
	}
}

func TestRateLimiterUntrustedPeerCannotPickItsBucket(t *testing.T) {
	codes := loginAttempts(t, "10.0.0.2", "198.51.100.7:54321", "203.0.113.10", "203.0.113.11")
	if !slices.Equal(codes, []int{http.StatusOK, http.StatusTooManyRequests}) {
		t.Fatalf("statuses = %v, want [200 429]", codes)
	}
}

func TestRateLimiterSeparatesClientsBehindTrustedProxy(t *testing.T) {
	codes := loginAttempts(t, "10.0.0.2", "10.0.0.2:40000", "203.0.113.10", "203.0.113.11", "203.0.113.10")
	want := []int{http.StatusOK, http.StatusOK, http.StatusTooManyRequests}
	if !slices.Equal(codes, want) {
		t.Fatalf("statuses = %v, want %v", codes, want)
	}
}

func TestRateLimiterBoundsBucketMap(t *testing.T) {
	limiter := newRateLimiter()

	// Flood with distinct keys whose windows are all still active; the map
	// must stay bounded even though pruning alone would keep every bucket.
	total := rateLimiterPruneThreshold * 3
	for i := 0; i < total; i++ {
		limiter.Allow(fmt.Sprintf("auth:198.51.%d.%d", i/256, i%256), 5, time.Minute)
	}

	limiter.mu.Lock()
	size := len(limiter.buckets)
	limiter.mu.Unlock()

	// Allow evicts down to the threshold before inserting the new key.
	if size > rateLimiterPruneThreshold+1 {
		t.Fatalf("buckets = %d, want at most %d", size, rateLimiterPruneThreshold+1)
	}
}

func TestRateLimiterAllowsConcurrentCallsSafely(t *testing.T) {
	limiter := newRateLimiter()

	const limit = 7
	const attempts = 64

	var ready sync.WaitGroup
	ready.Add(attempts)

	start := make(chan struct{})
	results := make(chan bool, attempts)

	for i := 0; i < attempts; i++ {
		go func() {
			ready.Done()
			<-start
			results <- limiter.Allow("auth:198.51.100.7", limit, time.Minute)
		}()
	}

	ready.Wait()
	close(start)

	allowed := 0
	for i := 0; i < attempts; i++ {
		if <-results {
			allowed++
		}
	}

	if allowed != limit {
		t.Fatalf("allowed attempts = %d, want %d", allowed, limit)
	}
}

func TestClientIPFallsBackToRemoteAddr(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "[2001:db8::1]:12345"

	if got := clientIP(req); got != "2001:db8::1" {
		t.Fatalf("clientIP() = %q, want parsed RemoteAddr host", got)
	}
}

func TestRateLimiter_WindowRollsOver(t *testing.T) {
	limiter := newRateLimiter()

	current := time.Now()
	limiter.now = func() time.Time { return current }

	for i := 0; i < 3; i++ {
		if !limiter.Allow("key", 3, time.Minute) {
			t.Fatalf("attempt %d unexpectedly limited", i+1)
		}
	}
	if limiter.Allow("key", 3, time.Minute) {
		t.Fatal("4th attempt in window should be limited")
	}

	// A new window admits attempts again.
	current = current.Add(time.Minute + time.Second)
	if !limiter.Allow("key", 3, time.Minute) {
		t.Fatal("attempt after window rollover should be allowed")
	}
}

func TestRateLimiter_KeysAreIndependent(t *testing.T) {
	limiter := newRateLimiter()

	if !limiter.Allow("a", 1, time.Minute) {
		t.Fatal("first attempt for key a should be allowed")
	}
	if limiter.Allow("a", 1, time.Minute) {
		t.Fatal("second attempt for key a should be limited")
	}
	if !limiter.Allow("b", 1, time.Minute) {
		t.Fatal("key b should be unaffected by key a")
	}
}
