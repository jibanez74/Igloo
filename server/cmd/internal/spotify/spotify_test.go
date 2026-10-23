package spotify

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"testing"
	"time"
)

func TestSpotifyRequestsUseDeadlineWhenCallerHasNone(t *testing.T) {
	sc := newMockClient(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, ok := r.Context().Deadline()
		if !ok {
			t.Error("expected spotify request context to have a deadline")
		}
		writeJSON(w, artistSearchJSON("jm123", "John Mayer"))
	}))

	_, err := sc.SearchArtistByName(context.Background(), "John Mayer")
	if err != nil {
		t.Fatalf("SearchArtistByName failed: %v", err)
	}
}

func TestSpotifyRequestsPreserveCallerDeadline(t *testing.T) {
	callerDeadline := time.Now().Add(time.Minute)
	var requestDeadline time.Time
	sc := newMockClient(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestDeadline, _ = r.Context().Deadline()
		writeJSON(w, artistSearchJSON("jm123", "John Mayer"))
	}))

	ctx, cancel := context.WithDeadline(context.Background(), callerDeadline)
	defer cancel()

	_, err := sc.SearchArtistByName(ctx, "John Mayer")
	if err != nil {
		t.Fatalf("SearchArtistByName failed: %v", err)
	}
	if !requestDeadline.Equal(callerDeadline) {
		t.Fatalf("request deadline = %s, want caller deadline %s", requestDeadline, callerDeadline)
	}
}

func TestNew(t *testing.T) {
	t.Run("reuses constructor token for first API request", func(t *testing.T) {
		tokenExchangeCount := 0
		searchCount := 0
		httpClient := &http.Client{
			Transport: &mockTransport{handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				switch {
				case r.URL.String() == "https://accounts.spotify.com/api/token":
					tokenExchangeCount++
					writeSpotifyTokenResponse(w, "validated-token")
				case strings.HasSuffix(r.URL.Path, "/search"):
					searchCount++
					got := r.Header.Get("Authorization")
					if got != "Bearer validated-token" {
						t.Errorf("Authorization = %q, want bearer token from constructor", got)
					}
					writeJSON(w, albumSearchJSON("reuse123", "Blue Record"))
				default:
					t.Errorf("unexpected request URL: %s", r.URL.String())
					http.NotFound(w, r)
				}
			})},
		}

		client, err := newClient(context.Background(), httpClient, "client-id", "client-secret")
		if err != nil {
			t.Fatalf("expected constructor to succeed, got error: %v", err)
		}
		if tokenExchangeCount != 1 {
			t.Fatalf("tokenExchangeCount after New = %d, want 1", tokenExchangeCount)
		}

		albums, err := client.SearchAlbums(context.Background(), "Blue Record")
		if err != nil {
			t.Fatalf("SearchAlbums failed: %v", err)
		}
		if len(albums) != 1 {
			t.Fatalf("albums length = %d, want 1", len(albums))
		}
		if searchCount != 1 {
			t.Fatalf("searchCount = %d, want 1", searchCount)
		}
		if tokenExchangeCount != 1 {
			t.Fatalf("tokenExchangeCount after first request = %d, want reused constructor token", tokenExchangeCount)
		}
	})

	t.Run("requests the token with the client-credentials grant", func(t *testing.T) {
		httpClient := &http.Client{
			Transport: &mockTransport{handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method != http.MethodPost {
					t.Errorf("method = %s, want POST", r.Method)
				}
				contentType := r.Header.Get("Content-Type")
				if contentType != "application/x-www-form-urlencoded" {
					t.Errorf("Content-Type = %q, want a form body", contentType)
				}
				id, secret, ok := r.BasicAuth()
				if !ok || id != "client-id" || secret != "client-secret" {
					t.Errorf("basic auth = %q, %q, %v, want the client credentials", id, secret, ok)
				}
				err := r.ParseForm()
				if err != nil {
					t.Errorf("ParseForm: %v", err)
				}
				grantType := r.PostForm.Get("grant_type")
				if grantType != "client_credentials" {
					t.Errorf("grant_type = %q, want client_credentials", grantType)
				}
				writeSpotifyTokenResponse(w, "granted-token")
			})},
		}

		client, err := newClient(context.Background(), httpClient, "client-id", "client-secret")
		if err != nil {
			t.Fatalf("expected constructor to succeed, got error: %v", err)
		}
		if client.tokens.accessToken != "granted-token" {
			t.Fatalf("accessToken = %q, want granted-token", client.tokens.accessToken)
		}
	})

	t.Run("returns error when token exchange fails", func(t *testing.T) {
		httpClient := &http.Client{
			Transport: &mockTransport{handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusUnauthorized)
				b, _ := json.Marshal(map[string]interface{}{
					"error":             "invalid_client",
					"error_description": "bad credentials",
				})
				w.Write(b)
			})},
		}

		client, err := newClient(context.Background(), httpClient, "client-id", "client-secret")
		if err == nil {
			t.Fatal("expected constructor error, got nil")
		}
		if client != nil {
			t.Fatal("expected nil client on constructor failure")
		}
		if !strings.Contains(err.Error(), "status 401: invalid_client: bad credentials") {
			t.Fatalf("error = %q, want the token endpoint's status and reason", err)
		}
		tokenErr, ok := errors.AsType[*TokenError](err)
		if !ok || tokenErr.StatusCode != http.StatusUnauthorized {
			t.Fatalf("err = %v, want a 401 TokenError", err)
		}
		if strings.Contains(err.Error(), "client-secret") {
			t.Fatalf("error = %q leaks the client secret", err)
		}
	})

	t.Run("returns error when the token response has no access token", func(t *testing.T) {
		httpClient := &http.Client{
			Transport: &mockTransport{handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				writeJSON(w, map[string]interface{}{"token_type": "Bearer", "expires_in": 3600})
			})},
		}

		_, err := newClient(context.Background(), httpClient, "client-id", "client-secret")
		if err == nil || !strings.Contains(err.Error(), "missing access_token") {
			t.Fatalf("err = %v, want a missing access_token error", err)
		}
	})

	t.Run("New returns a nil client when the token cannot be fetched", func(t *testing.T) {
		// A canceled context fails the token request before it leaves the
		// process, so this reaches no network.
		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		client, err := New(ctx, "client-id", "client-secret")
		if err == nil {
			t.Fatal("expected constructor error, got nil")
		}
		if client != nil {
			t.Fatalf("client = %#v, want nil", client)
		}
	})
}

func TestTokenRefresh(t *testing.T) {
	t.Run("fetches one new token when the cached one is about to expire", func(t *testing.T) {
		tokenExchangeCount := 0
		var authorizations []string
		sc := newMockClient(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.String() == spotifyTokenURL {
				tokenExchangeCount++
				writeSpotifyTokenResponse(w, "fresh-token")
				return
			}
			authorizations = append(authorizations, r.Header.Get("Authorization"))
			writeJSON(w, albumSearchJSON("album123", "Blue Record"))
		}))
		sc.tokens.accessToken = "stale-token"
		sc.tokens.expiry = time.Now().Add(spotifyTokenExpiryMargin / 2)

		for range 2 {
			_, err := sc.SearchAlbums(context.Background(), "Blue Record")
			if err != nil {
				t.Fatalf("SearchAlbums failed: %v", err)
			}
		}

		if tokenExchangeCount != 1 {
			t.Fatalf("tokenExchangeCount = %d, want one refresh", tokenExchangeCount)
		}
		if len(authorizations) != 2 || authorizations[0] != "Bearer fresh-token" || authorizations[1] != "Bearer fresh-token" {
			t.Fatalf("authorizations = %q, want the refreshed token on both requests", authorizations)
		}
	})

	t.Run("reports search_failed when the refresh fails", func(t *testing.T) {
		searchCount := 0
		sc := newMockClient(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.String() == spotifyTokenURL {
				w.WriteHeader(http.StatusInternalServerError)
				return
			}
			searchCount++
			writeJSON(w, artistSearchJSON("jm123", "John Mayer"))
		}))
		sc.tokens.expiry = time.Now()

		_, err := sc.SearchArtistByName(context.Background(), "John Mayer")
		assertMatchReason(t, err, MatchReasonSearchFailed)
		if searchCount != 0 {
			t.Fatalf("searchCount = %d, want no API request without a token", searchCount)
		}
	})
}

func TestSpotifyRequestShape(t *testing.T) {
	var requests []string
	sc := newMockClient(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests = append(requests, r.Method+" "+r.URL.String()+" "+r.Header.Get("Authorization"))
		if strings.HasSuffix(r.URL.Path, "/search") {
			writeJSON(w, albumSearchJSON("abc123", "Abbey Road"))
			return
		}
		writeJSON(w, fullAlbumJSON("abc123", "Abbey Road"))
	}))

	_, err := sc.MatchAlbum(context.Background(), AlbumQuery{Title: "Abbey Road"})
	if err != nil {
		t.Fatalf("MatchAlbum failed: %v", err)
	}

	want := []string{
		`GET https://api.spotify.com/v1/search?limit=5&q=album%3A%22Abbey+Road%22&type=album Bearer test-token`,
		`GET https://api.spotify.com/v1/albums/abc123 Bearer test-token`,
	}
	if strings.Join(requests, "\n") != strings.Join(want, "\n") {
		t.Fatalf("requests =\n%s\nwant\n%s", strings.Join(requests, "\n"), strings.Join(want, "\n"))
	}
}

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
