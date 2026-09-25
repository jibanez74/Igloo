package spotify

import (
	"context"
	"net/http"
	"strings"
	"testing"
)

func TestSearchTracks(t *testing.T) {
	t.Run("returns error for empty track title", func(t *testing.T) {
		callCount := 0
		sc := newMockClient(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			callCount++
		}))
		_, err := sc.SearchTracks(context.Background(), "   ")
		if err == nil {
			t.Fatal("expected error for empty title, got nil")
		}
		if callCount != 0 {
			t.Fatalf("callCount = %d, want no API calls", callCount)
		}
	})

	t.Run("returns candidate tracks from Spotify search", func(t *testing.T) {
		var capturedQuery string
		var capturedLimit string
		sc := newMockClient(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if !strings.HasSuffix(r.URL.Path, "/search") {
				t.Fatalf("unexpected request path: %s", r.URL.Path)
			}
			capturedQuery = r.URL.Query().Get("q")
			capturedLimit = r.URL.Query().Get("limit")
			track := map[string]interface{}{
				"id":            "track123",
				"name":          "Attention",
				"type":          "track",
				"duration_ms":   200000,
				"explicit":      false,
				"popularity":    70,
				"disc_number":   1,
				"track_number":  1,
				"artists":       []interface{}{},
				"album":         albumItemJSON("track123-album", "Attention"),
				"external_urls": map[string]interface{}{},
				"href":          "",
				"uri":           "",
			}
			writeJSON(w, map[string]interface{}{
				"tracks": map[string]interface{}{
					"items": []interface{}{track},
					"total": 1,
					"limit": 1,
				},
			})
		}))

		tracks, err := sc.SearchTracks(context.Background(), "  Attention  ")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if capturedQuery != "Attention" {
			t.Fatalf("query = %q, want trimmed track title", capturedQuery)
		}
		if capturedLimit != "10" {
			t.Fatalf("limit = %q, want 10", capturedLimit)
		}
		if len(tracks) != 1 || tracks[0].ID.String() != "track123" || tracks[0].Name != "Attention" {
			t.Fatalf("tracks = %+v, want Attention candidate", tracks)
		}
	})

	t.Run("returns empty slice when response has no tracks object", func(t *testing.T) {
		sc := newMockClient(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			writeJSON(w, map[string]interface{}{})
		}))

		tracks, err := sc.SearchTracks(context.Background(), "Unknown Track")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(tracks) != 0 {
			t.Fatalf("tracks length = %d, want 0", len(tracks))
		}
	})

	t.Run("returns error when spotify search request fails", func(t *testing.T) {
		sc := newMockClient(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			writeSpotifyAPIError(w, http.StatusInternalServerError, "search failed")
		}))

		_, err := sc.SearchTracks(context.Background(), "Attention")
		if err == nil {
			t.Fatal("expected error when spotify search request fails, got nil")
		}
		if !strings.Contains(err.Error(), "spotify track search failed") {
			t.Fatalf("error = %q, want wrapped track search failure", err.Error())
		}
	})
}
