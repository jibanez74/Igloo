package spotify

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"
)

func TestSpotifyAPIErrors(t *testing.T) {
	respondBody := func(status int, body string) func(http.ResponseWriter) {
		return func(w http.ResponseWriter) {
			w.WriteHeader(status)
			w.Write([]byte(body))
		}
	}
	tests := []struct {
		name        string
		respond     func(http.ResponseWriter)
		wantStatus  int
		wantMessage string
		wantReason  string
	}{
		{"uses the message from the error envelope", respondSpotifyAPIError(http.StatusBadGateway, "search failed"), http.StatusBadGateway, "search failed", ""},
		{"falls back to the status text for an empty body", respondBody(http.StatusServiceUnavailable, ""), http.StatusServiceUnavailable, "Service Unavailable", ""},
		{"falls back to the status text for a body that is not an envelope", respondBody(http.StatusTooManyRequests, "slow down"), http.StatusTooManyRequests, "Too Many Requests", ""},
		{"reads a quota reason inside the envelope", respondBody(http.StatusTooManyRequests, `{"error":{"status":429,"message":"quota exceeded","reason":"QUOTA_EXCEEDED"}}`), http.StatusTooManyRequests, "quota exceeded", quotaExceededReason},
		{"reads a quota reason at the top level", respondBody(http.StatusTooManyRequests, `{"reason":"QUOTA_EXCEEDED","error":{"status":429,"message":"quota exceeded"}}`), http.StatusTooManyRequests, "quota exceeded", quotaExceededReason},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			sc := newMockClient(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				tc.respond(w)
			}))

			_, err := sc.SearchAlbums(context.Background(), "Blue Record")
			statusErr, ok := errors.AsType[*StatusError](err)
			if !ok {
				t.Fatalf("err = %v, want a StatusError", err)
			}
			if statusErr.StatusCode != tc.wantStatus || statusErr.Message != tc.wantMessage || statusErr.Reason != tc.wantReason {
				t.Fatalf("StatusError = %+v, want %d %q reason %q", statusErr, tc.wantStatus, tc.wantMessage, tc.wantReason)
			}
			if tc.wantReason != "" && !strings.Contains(statusErr.Error(), tc.wantReason) {
				t.Fatalf("Error() = %q, want the reason", statusErr.Error())
			}
		})
	}
}

// The music scanner stops dispatch on a fatal failure or a run of transient
// ones, so the classification decides whether a bad secret costs one request
// or one per entity, and whether a deleted album trips the breaker.
func TestProviderFailure(t *testing.T) {
	tests := []struct {
		name          string
		err           error
		wantFatal     bool
		wantTransient bool
	}{
		{"nil", nil, false, false},
		{"api 401", &StatusError{StatusCode: http.StatusUnauthorized}, true, false},
		{"api 403", &StatusError{StatusCode: http.StatusForbidden}, true, false},
		{"api 404 concerns one entry", &StatusError{StatusCode: http.StatusNotFound}, false, false},
		{"api 400", &StatusError{StatusCode: http.StatusBadRequest}, false, false},
		{"api 429 rate limit", &StatusError{StatusCode: http.StatusTooManyRequests}, false, true},
		{"api 429 quota exhausted", &StatusError{StatusCode: http.StatusTooManyRequests, Reason: quotaExceededReason}, true, false},
		{"api 502", &StatusError{StatusCode: http.StatusBadGateway}, false, true},
		{"token 400 invalid_client", &TokenError{StatusCode: http.StatusBadRequest, Reason: "invalid_client"}, true, false},
		{"token 401", &TokenError{StatusCode: http.StatusUnauthorized}, true, false},
		{"token 503", &TokenError{StatusCode: http.StatusServiceUnavailable}, false, true},
		{"status wrapped in a match error", newMatchError(MatchDebugInfo{Reason: MatchReasonSearchFailed}, &StatusError{StatusCode: http.StatusUnauthorized}), true, false},
		{"transport wrapped in a match error", newMatchError(MatchDebugInfo{Reason: MatchReasonSearchFailed}, errors.New("connection reset")), false, true},
		{"no results is not a failure", newMatchError(MatchDebugInfo{Reason: MatchReasonNoResults}, nil), false, false},
		{"plain transport error", errors.New("connection reset"), false, true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			fatal, transient := ProviderFailure(tc.err)
			if fatal != tc.wantFatal || transient != tc.wantTransient {
				t.Fatalf("ProviderFailure = (%v, %v), want (%v, %v)", fatal, transient, tc.wantFatal, tc.wantTransient)
			}
		})
	}
}

// A rate limit that names a short Retry-After is waited out inside the
// request; the quota reason, a missing header, a long wait, a wait past the
// deadline and a third rate limit in a row are returned to the caller.
func TestGetJSONRateLimitRetry(t *testing.T) {
	rateLimited := func(w http.ResponseWriter, retryAfter, reason string) {
		if retryAfter != "" {
			w.Header().Set("Retry-After", retryAfter)
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusTooManyRequests)
		fmt.Fprintf(w, `{"error":{"status":429,"message":"slow down","reason":%q}}`, reason)
	}

	tests := []struct {
		name         string
		retryAfter   string
		reason       string
		limitedTimes int
		deadline     time.Duration
		wantCalls    int
		wantErr      bool
	}{
		{name: "retries a short wait and succeeds", retryAfter: "0", limitedTimes: 2, wantCalls: 3},
		{name: "gives up after the retry budget", retryAfter: "0", limitedTimes: 5, wantCalls: spotifyRateLimitRetries + 1, wantErr: true},
		{name: "does not retry an exhausted quota", retryAfter: "0", reason: quotaExceededReason, limitedTimes: 1, wantCalls: 1, wantErr: true},
		{name: "does not retry without Retry-After", limitedTimes: 1, wantCalls: 1, wantErr: true},
		{name: "does not wait out a long Retry-After", retryAfter: "60", limitedTimes: 1, wantCalls: 1, wantErr: true},
		{name: "does not wait past the deadline", retryAfter: "2", limitedTimes: 1, deadline: time.Second, wantCalls: 1, wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var calls int
			client := newMockClient(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls++
				if calls <= tt.limitedTimes {
					rateLimited(w, tt.retryAfter, tt.reason)
					return
				}
				writeJSON(w, map[string]string{"id": "ok"})
			}))

			ctx := context.Background()
			if tt.deadline > 0 {
				var cancel context.CancelFunc
				ctx, cancel = context.WithTimeout(ctx, tt.deadline)
				defer cancel()
			}

			var result struct {
				ID string `json:"id"`
			}
			start := time.Now()
			err := client.getJSON(ctx, spotifyAPIBaseURL+"/albums/x", &result)
			if time.Since(start) > 500*time.Millisecond {
				t.Fatalf("getJSON waited %s, want no real wait", time.Since(start))
			}
			if calls != tt.wantCalls {
				t.Fatalf("requests = %d, want %d", calls, tt.wantCalls)
			}
			if !tt.wantErr {
				if err != nil || result.ID != "ok" {
					t.Fatalf("getJSON = %v, id %q; want the retried response", err, result.ID)
				}
				return
			}
			status, ok := errors.AsType[*StatusError](err)
			if !ok || status.StatusCode != http.StatusTooManyRequests {
				t.Fatalf("error = %v, want the 429 StatusError", err)
			}
		})
	}
}
