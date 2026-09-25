package spotify

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"

	"golang.org/x/oauth2"
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
						t.Fatalf("Authorization = %q, want bearer token from constructor", got)
					}
					writeJSON(w, albumSearchJSON("reuse123", "Blue Record"))
				default:
					t.Fatalf("unexpected request URL: %s", r.URL.String())
				}
			})},
		}
		ctx := context.WithValue(context.Background(), oauth2.HTTPClient, httpClient)

		client, err := New(ctx, "client-id", "client-secret")
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
		ctx := context.WithValue(context.Background(), oauth2.HTTPClient, httpClient)

		client, err := New(ctx, "client-id", "client-secret")
		if err == nil {
			t.Fatal("expected constructor error, got nil")
		}
		if client != nil {
			t.Fatal("expected nil client on constructor failure")
		}
	})
}
