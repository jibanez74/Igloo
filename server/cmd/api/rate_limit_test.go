package main

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/go-chi/chi/v5/middleware"
)

func TestClientIPUsesSocketIPCapturedBeforeRealIP(t *testing.T) {
	handler := preserveClientSocketIP(middleware.RealIP(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.RemoteAddr != "203.0.113.44" {
			t.Fatalf("RealIP did not rewrite RemoteAddr from forwarded header, got %q", r.RemoteAddr)
		}

		if got := clientIP(r); got != "198.51.100.7" {
			t.Fatalf("clientIP() = %q, want socket IP", got)
		}
	})))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "198.51.100.7:54321"
	req.Header.Set("X-Forwarded-For", "203.0.113.44")

	handler.ServeHTTP(httptest.NewRecorder(), req)
}

func TestRateLimiterCannotBeBypassedByForwardedForRotation(t *testing.T) {
	limiter := newRateLimiter()
	handler := preserveClientSocketIP(middleware.RealIP(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !limiter.Allow("auth:"+clientIP(r), 1, time.Minute) {
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}

		w.WriteHeader(http.StatusOK)
	})))

	for i, forwardedFor := range []string{"203.0.113.10", "203.0.113.11"} {
		req := httptest.NewRequest(http.MethodPost, "/api/auth/device-login", nil)
		req.RemoteAddr = "198.51.100.7:54321"
		req.Header.Set("X-Forwarded-For", forwardedFor)

		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)

		if i == 0 && w.Code != http.StatusOK {
			t.Fatalf("first request status = %d, want 200", w.Code)
		}
		if i == 1 && w.Code != http.StatusTooManyRequests {
			t.Fatalf("second request status = %d, want 429", w.Code)
		}
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
