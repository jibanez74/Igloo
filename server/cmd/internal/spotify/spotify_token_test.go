package spotify

import (
	"context"
	"net/http"
	"testing"
	"time"
)

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
