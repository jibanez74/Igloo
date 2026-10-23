package spotify

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"testing"
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

	t.Run("returns search_failed when the spotify request fails", func(t *testing.T) {
		sc := newMockClient(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			writeSpotifyAPIError(w, http.StatusInternalServerError, "search failed")
		}))
		_, err := sc.SearchArtistByName(context.Background(), "John Mayer")
		assertMatchReason(t, err, MatchReasonSearchFailed)
	})
}

func TestGetArtist(t *testing.T) {
	t.Run("returns the artist by id", func(t *testing.T) {
		var path string
		sc := newMockClient(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			path = r.URL.Path
			writeJSON(w, artistItemJSON("abc123", "Charlie Puth"))
		}))

		artist, err := sc.GetArtist(context.Background(), "abc123")
		if err != nil {
			t.Fatalf("GetArtist failed: %v", err)
		}
		if path != "/v1/artists/abc123" {
			t.Fatalf("path = %q, want the artist endpoint", path)
		}
		if artist.ID != "abc123" || artist.Name != "Charlie Puth" || len(artist.Images) != 1 {
			t.Fatalf("artist = %+v, want Charlie Puth with an image", artist)
		}
	})

	t.Run("surfaces an unknown id as a 404 status error", func(t *testing.T) {
		sc := newMockClient(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			writeSpotifyAPIError(w, http.StatusNotFound, "non existing id")
		}))

		_, err := sc.GetArtist(context.Background(), "gone123")
		statusErr, ok := errors.AsType[*StatusError](err)
		if !ok || statusErr.StatusCode != http.StatusNotFound || !strings.Contains(err.Error(), "spotify artist lookup failed") {
			t.Fatalf("err = %v, want a wrapped 404 StatusError", err)
		}
	})
}
