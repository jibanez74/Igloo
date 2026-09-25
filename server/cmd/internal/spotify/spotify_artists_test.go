package spotify

import (
	"context"
	"net/http"
	"testing"

	gocache "github.com/patrickmn/go-cache"
)

func TestSearchArtistByName(t *testing.T) {
	t.Run("rejects a blank artist name without calling the API", func(t *testing.T) {
		callCount := 0
		sc := newMockClient(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			callCount++
		}))
		_, err := sc.SearchArtistByName(context.Background(), "   ")
		assertMatchReason(t, err, MatchReasonEmptyQuery)
		if callCount != 0 {
			t.Fatalf("callCount = %d, want no API calls", callCount)
		}
	})

	t.Run("returns no_results when response has no artists object", func(t *testing.T) {
		sc := newMockClient(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			writeJSON(w, map[string]interface{}{})
		}))
		_, err := sc.SearchArtistByName(context.Background(), "John Mayer")
		assertMatchReason(t, err, MatchReasonNoResults)
	})

	t.Run("returns no_results when the artist list is empty", func(t *testing.T) {
		sc := newMockClient(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			writeJSON(w, artistSearchItemsJSON())
		}))
		_, err := sc.SearchArtistByName(context.Background(), "Unknown Artist XYZ999")
		assertMatchReason(t, err, MatchReasonNoResults)
	})

	t.Run("returns artist when name matches", func(t *testing.T) {
		sc := newMockClient(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			writeJSON(w, artistSearchJSON("abc123", "Charlie Puth"))
		}))
		artist, err := sc.SearchArtistByName(context.Background(), "Charlie Puth")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if string(artist.ID) != "abc123" {
			t.Errorf("expected ID 'abc123', got '%s'", artist.ID)
		}
		if artist.Name != "Charlie Puth" {
			t.Errorf("expected name 'Charlie Puth', got '%s'", artist.Name)
		}
	})

	t.Run("accepts real band name containing ampersand", func(t *testing.T) {
		// "Hall & Oates" is a real artist on Spotify. The returned name normalizes to the
		// same text as the query, so it scores as an exact match above the artist threshold.
		sc := newMockClient(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			writeJSON(w, artistSearchJSON("hallOatesID", "Hall & Oates"))
		}))
		artist, err := sc.SearchArtistByName(context.Background(), "Hall & Oates")
		if err != nil {
			t.Fatalf("unexpected error for real band name with ampersand: %v", err)
		}
		if artist.Name != "Hall & Oates" {
			t.Errorf("expected 'Hall & Oates', got '%s'", artist.Name)
		}
	})

	t.Run("reports score_below_threshold with the best candidate", func(t *testing.T) {
		sc := newMockClient(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			writeJSON(w, artistSearchJSON("redVinesID", "Red Vines"))
		}))
		_, err := sc.SearchArtistByName(context.Background(), "Red Hot Chili Peppers")
		matchErr := assertMatchReason(t, err, MatchReasonScoreBelowThreshold)
		if matchErr.Info.CandidateName != "Red Vines" || matchErr.Info.Score >= spotifyArtistThreshold {
			t.Fatalf("info = %+v, want the rejected candidate below %d", matchErr.Info, spotifyArtistThreshold)
		}
	})

	t.Run("treats unexpected cached artist value as cache miss", func(t *testing.T) {
		callCount := 0
		sc := newMockClient(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			callCount++
			writeJSON(w, artistSearchJSON("safeArtist123", "John Mayer"))
		}))
		sc.artistCache.Set("john mayer", "bad-cache-value", gocache.DefaultExpiration)

		artist, err := sc.SearchArtistByName(context.Background(), "John Mayer")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if callCount != 1 {
			t.Fatalf("callCount = %d, want cache miss and one search", callCount)
		}
		if string(artist.ID) != "safeArtist123" {
			t.Fatalf("artist.ID = %q, want safeArtist123", artist.ID)
		}
	})

	t.Run("cache key ignores case and surrounding whitespace", func(t *testing.T) {
		callCount := 0
		sc := newMockClient(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			callCount++
			writeJSON(w, artistSearchJSON("trimmedID", "John Mayer"))
		}))
		ctx := context.Background()

		first, err := sc.SearchArtistByName(ctx, "  John Mayer  ")
		if err != nil {
			t.Fatalf("first call failed: %v", err)
		}
		second, err := sc.SearchArtistByName(ctx, "john mayer")
		if err != nil {
			t.Fatalf("second call failed: %v", err)
		}
		if callCount != 1 {
			t.Errorf("expected 1 HTTP call for the normalized cache key, got %d", callCount)
		}
		if first != second {
			t.Error("expected the cached artist pointer to be returned")
		}
	})

	t.Run("returns search_failed when the spotify request fails", func(t *testing.T) {
		sc := newMockClient(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			writeSpotifyAPIError(w, http.StatusInternalServerError, "search failed")
		}))
		_, err := sc.SearchArtistByName(context.Background(), "John Mayer")
		assertMatchReason(t, err, MatchReasonSearchFailed)
	})
}
