package spotify

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	gocache "github.com/patrickmn/go-cache"
	spotifylib "github.com/zmb3/spotify/v2"
	"golang.org/x/oauth2"
)

// mockTransport intercepts HTTP calls and routes them to an in-process handler,
// allowing tests to run without a real Spotify API connection.
type mockTransport struct {
	handler http.Handler
}

func (m *mockTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	rr := httptest.NewRecorder()
	m.handler.ServeHTTP(rr, req)
	return rr.Result(), nil
}

func newMockClient(t *testing.T, handler http.Handler) *spotifyClient {
	t.Helper()
	client := spotifylib.New(&http.Client{Transport: &mockTransport{handler: handler}})
	return &spotifyClient{
		client:      client,
		artistCache: gocache.New(spotifyArtistCacheTTL, spotifyCacheCleanup),
		albumCache:  gocache.New(spotifyAlbumCacheTTL, spotifyCacheCleanup),
	}
}

func writeJSON(w http.ResponseWriter, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	b, _ := json.Marshal(v)
	w.Write(b)
}

func writeSpotifyAPIError(w http.ResponseWriter, status int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	b, _ := json.Marshal(map[string]interface{}{
		"error": map[string]interface{}{
			"status":  status,
			"message": message,
		},
	})
	w.Write(b)
}

func writeSpotifyTokenResponse(w http.ResponseWriter, accessToken string) {
	w.Header().Set("Content-Type", "application/json")
	b, _ := json.Marshal(map[string]interface{}{
		"access_token": accessToken,
		"token_type":   "Bearer",
		"expires_in":   3600,
	})
	w.Write(b)
}

func artistSearchJSON(id, name string) interface{} {
	return artistSearchItemsJSON(artistItemJSON(id, name))
}

func emptyArtistSearchJSON() interface{} {
	return artistSearchItemsJSON()
}

func albumSearchJSON(id, name string) interface{} {
	return albumSearchItemsJSON(albumItemJSON(id, name))
}

func emptyAlbumSearchJSON() interface{} {
	return albumSearchItemsJSON()
}

func trackItemJSON(id, name string) map[string]interface{} {
	return map[string]interface{}{
		"id":            id,
		"name":          name,
		"type":          "track",
		"duration_ms":   200000,
		"explicit":      false,
		"popularity":    70,
		"disc_number":   1,
		"track_number":  1,
		"artists":       []interface{}{},
		"album":         albumItemJSON(id+"-album", name),
		"external_urls": map[string]interface{}{},
		"href":          "",
		"uri":           "",
	}
}

func trackSearchItemsJSON(items ...map[string]interface{}) interface{} {
	return map[string]interface{}{
		"tracks": map[string]interface{}{
			"items": items,
			"total": len(items),
			"limit": len(items),
		},
	}
}

func fullAlbumJSON(id, name string) interface{} {
	return map[string]interface{}{
		"id":                     id,
		"name":                   name,
		"type":                   "album",
		"album_type":             "album",
		"release_date":           "2023-01-01",
		"release_date_precision": "day",
		"total_tracks":           12,
		"popularity":             75,
		"genres":                 []string{"pop"},
		"images":                 []map[string]interface{}{{"url": "https://example.com/cover.jpg", "height": 640, "width": 640}},
		"artists":                []interface{}{},
		"tracks":                 map[string]interface{}{"items": []interface{}{}, "total": 12, "limit": 50, "offset": 0},
		"copyrights":             []interface{}{},
		"external_ids":           map[string]interface{}{},
		"external_urls":          map[string]interface{}{},
		"href":                   "",
		"uri":                    "",
		"available_markets":      []interface{}{},
	}
}

func TestSearchArtistByName(t *testing.T) {
	t.Run("returns error for empty artist name", func(t *testing.T) {
		sc := newMockClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
		_, err := sc.SearchArtistByName(context.Background(), "")
		if err == nil {
			t.Fatal("expected error for empty artist name, got nil")
		}
	})

	t.Run("returns error for whitespace-only artist name without calling the API", func(t *testing.T) {
		callCount := 0
		sc := newMockClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			callCount++
		}))
		_, err := sc.SearchArtistByName(context.Background(), "   ")
		if err == nil {
			t.Fatal("expected error for whitespace-only artist name, got nil")
		}
		if callCount != 0 {
			t.Fatalf("callCount = %d, want no API calls", callCount)
		}
	})

	t.Run("returns error when response has no artists object", func(t *testing.T) {
		sc := newMockClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			writeJSON(w, map[string]interface{}{})
		}))
		_, err := sc.SearchArtistByName(context.Background(), "John Mayer")
		if err == nil {
			t.Fatal("expected error for missing artists object, got nil")
		}
		matchErr, ok := AsMatchError(err)
		if !ok {
			t.Fatalf("expected MatchError, got %T", err)
		}
		if matchErr.Info.Reason != "no_results" {
			t.Fatalf("reason = %q, want no_results", matchErr.Info.Reason)
		}
	})

	t.Run("returns artist when name matches", func(t *testing.T) {
		sc := newMockClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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
		sc := newMockClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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

	t.Run("rejects compound credit when Spotify returns only the primary artist", func(t *testing.T) {
		// Spotify returns "Charlie Puth" for the query "Charlie Puth & Coco Jones".
		// The candidate covers only part of the query's tokens, so it scores below the
		// artist threshold. This is the discriminator that allows safe compound-credit splitting.
		sc := newMockClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			writeJSON(w, artistSearchJSON("charliePuthID", "Charlie Puth"))
		}))
		_, err := sc.SearchArtistByName(context.Background(), "Charlie Puth & Coco Jones")
		if err == nil {
			t.Fatal("expected validation error for compound credit, got nil")
		}
	})

	t.Run("returns error when no results", func(t *testing.T) {
		sc := newMockClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			writeJSON(w, emptyArtistSearchJSON())
		}))
		_, err := sc.SearchArtistByName(context.Background(), "Unknown Artist XYZ999")
		if err == nil {
			t.Fatal("expected error for empty results, got nil")
		}
	})

	t.Run("caches result and avoids redundant API calls", func(t *testing.T) {
		callCount := 0
		sc := newMockClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			callCount++
			writeJSON(w, artistSearchJSON("edID", "Ed Sheeran"))
		}))
		ctx := context.Background()

		first, err := sc.SearchArtistByName(ctx, "Ed Sheeran")
		if err != nil {
			t.Fatalf("first call failed: %v", err)
		}
		second, err := sc.SearchArtistByName(ctx, "Ed Sheeran")
		if err != nil {
			t.Fatalf("second call failed: %v", err)
		}
		if callCount != 1 {
			t.Errorf("expected 1 HTTP call, got %d (cache not working)", callCount)
		}
		if first.ID != second.ID {
			t.Error("cached result does not match original")
		}
	})

	t.Run("treats unexpected cached artist value as cache miss", func(t *testing.T) {
		callCount := 0
		sc := newMockClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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

	t.Run("cache key is case-insensitive", func(t *testing.T) {
		callCount := 0
		sc := newMockClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			callCount++
			writeJSON(w, artistSearchJSON("johnID", "John Mayer"))
		}))
		ctx := context.Background()

		_, err := sc.SearchArtistByName(ctx, "John Mayer")
		if err != nil {
			t.Fatalf("first call failed: %v", err)
		}
		_, err = sc.SearchArtistByName(ctx, "john mayer")
		if err != nil {
			t.Fatalf("second call with different casing failed: %v", err)
		}
		if callCount != 1 {
			t.Errorf("expected 1 HTTP call for case-insensitive cache key, got %d", callCount)
		}
	})

	t.Run("cache key trims surrounding whitespace", func(t *testing.T) {
		callCount := 0
		sc := newMockClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			callCount++
			writeJSON(w, artistSearchJSON("trimmedID", "John Mayer"))
		}))
		ctx := context.Background()

		_, err := sc.SearchArtistByName(ctx, "  John Mayer  ")
		if err != nil {
			t.Fatalf("first call failed: %v", err)
		}
		_, err = sc.SearchArtistByName(ctx, "john mayer")
		if err != nil {
			t.Fatalf("second call failed: %v", err)
		}
		if callCount != 1 {
			t.Errorf("expected 1 HTTP call for trimmed cache key, got %d", callCount)
		}
	})

	t.Run("returns error when spotify search request fails", func(t *testing.T) {
		sc := newMockClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			writeSpotifyAPIError(w, http.StatusInternalServerError, "search failed")
		}))
		_, err := sc.SearchArtistByName(context.Background(), "John Mayer")
		if err == nil {
			t.Fatal("expected error when spotify search request fails, got nil")
		}
	})
}

func TestSearchAndGetAlbumDetails(t *testing.T) {
	t.Run("returns error for empty album title", func(t *testing.T) {
		sc := newMockClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
		_, err := sc.SearchAndGetAlbumDetails(context.Background(), "", "Some Artist")
		if err == nil {
			t.Fatal("expected error for empty title, got nil")
		}
	})

	t.Run("builds structured field query when artist is provided", func(t *testing.T) {
		var capturedQuery string
		sc := newMockClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if strings.HasSuffix(r.URL.Path, "/search") {
				capturedQuery = r.URL.Query().Get("q")
				writeJSON(w, albumSearchJSON("t123", "Thriller"))
			} else {
				writeJSON(w, fullAlbumJSON("t123", "Thriller"))
			}
		}))
		_, err := sc.SearchAndGetAlbumDetails(context.Background(), "Thriller", "Michael Jackson")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !strings.Contains(capturedQuery, `album:"Thriller"`) {
			t.Errorf("expected album field filter in query, got: %s", capturedQuery)
		}
		if !strings.Contains(capturedQuery, `artist:"Michael Jackson"`) {
			t.Errorf("expected artist field filter in query, got: %s", capturedQuery)
		}
	})

	t.Run("sanitizes double quotes in structured field query values", func(t *testing.T) {
		var capturedQuery string
		sc := newMockClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if strings.HasSuffix(r.URL.Path, "/search") {
				capturedQuery = r.URL.Query().Get("q")
				writeJSON(w, albumSearchJSON("q123", `Live "At Home"`))
			} else {
				writeJSON(w, fullAlbumJSON("q123", `Live "At Home"`))
			}
		}))
		_, err := sc.SearchAndGetAlbumDetails(context.Background(), `Live "At Home"`, `The "Band"`)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if capturedQuery != `album:"Live 'At Home'" artist:"The 'Band'"` {
			t.Fatalf("query = %q, want sanitized field query", capturedQuery)
		}
	})

	t.Run("omits artist field filter when artist is empty", func(t *testing.T) {
		var capturedQuery string
		sc := newMockClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if strings.HasSuffix(r.URL.Path, "/search") {
				capturedQuery = r.URL.Query().Get("q")
				writeJSON(w, albumSearchJSON("g123", "Greatest Hits"))
			} else {
				writeJSON(w, fullAlbumJSON("g123", "Greatest Hits"))
			}
		}))
		_, err := sc.SearchAndGetAlbumDetails(context.Background(), "Greatest Hits", "")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !strings.Contains(capturedQuery, `album:"Greatest Hits"`) {
			t.Errorf("expected album field filter in query, got: %s", capturedQuery)
		}
		if strings.Contains(capturedQuery, "artist:") {
			t.Errorf("query should not contain artist field filter when artist is empty, got: %s", capturedQuery)
		}
	})

	t.Run("strips '- Single' suffix before sending search query", func(t *testing.T) {
		var capturedQuery string
		sc := newMockClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if strings.HasSuffix(r.URL.Path, "/search") {
				capturedQuery = r.URL.Query().Get("q")
				writeJSON(w, albumSearchJSON("s123", "The Joker and the Queen"))
			} else {
				writeJSON(w, fullAlbumJSON("s123", "The Joker and the Queen"))
			}
		}))
		_, err := sc.SearchAndGetAlbumDetails(context.Background(), "The Joker And The Queen - Single", "Ed Sheeran")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if strings.Contains(strings.ToLower(capturedQuery), "single") {
			t.Errorf("query must not contain '- Single' suffix, got: %s", capturedQuery)
		}
		if !strings.Contains(capturedQuery, "The Joker And The Queen") {
			t.Errorf("query must contain stripped title, got: %s", capturedQuery)
		}
	})

	t.Run("strips '- EP' suffix before sending search query", func(t *testing.T) {
		var capturedQuery string
		sc := newMockClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if strings.HasSuffix(r.URL.Path, "/search") {
				capturedQuery = r.URL.Query().Get("q")
				writeJSON(w, albumSearchJSON("e123", "Some Album"))
			} else {
				writeJSON(w, fullAlbumJSON("e123", "Some Album"))
			}
		}))
		_, err := sc.SearchAndGetAlbumDetails(context.Background(), "Some Album - EP", "Some Artist")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if strings.Contains(strings.ToLower(capturedQuery), " - ep") {
			t.Errorf("query must not contain '- EP' suffix, got: %s", capturedQuery)
		}
	})

	t.Run("preserves internal '- EP' text when it is not a suffix", func(t *testing.T) {
		var capturedQuery string
		sc := newMockClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if strings.HasSuffix(r.URL.Path, "/search") {
				capturedQuery = r.URL.Query().Get("q")
				writeJSON(w, albumSearchJSON("eps123", "Foo - EP Sessions"))
			} else {
				writeJSON(w, fullAlbumJSON("eps123", "Foo - EP Sessions"))
			}
		}))
		_, err := sc.SearchAndGetAlbumDetails(context.Background(), "Foo - EP Sessions", "Some Artist")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !strings.Contains(capturedQuery, "Foo - EP Sessions") {
			t.Errorf("query must preserve internal '- EP' text, got: %s", capturedQuery)
		}
		if strings.Contains(capturedQuery, `album:"Foo"`) {
			t.Errorf("query must not truncate title to prefix, got: %s", capturedQuery)
		}
	})

	t.Run("validation accepts when result name is contained in query title", func(t *testing.T) {
		// Spotify returns "Abbey Road" but the file tag says "Abbey Road (Remastered)".
		// "remastered" is an album noise token, so both base titles reduce to "abbey road"
		// and the candidate scores above the album threshold.
		sc := newMockClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if strings.HasSuffix(r.URL.Path, "/search") {
				writeJSON(w, albumSearchJSON("ar123", "Abbey Road"))
			} else {
				writeJSON(w, fullAlbumJSON("ar123", "Abbey Road"))
			}
		}))
		album, err := sc.SearchAndGetAlbumDetails(context.Background(), "Abbey Road (Remastered)", "The Beatles")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if string(album.ID) != "ar123" {
			t.Errorf("expected album ID 'ar123', got '%s'", album.ID)
		}
	})

	t.Run("validation accepts album titles with apostrophe character variants", func(t *testing.T) {
		// File tag uses a curly/smart apostrophe (U+2019), Spotify uses a straight apostrophe (U+0027).
		// The normalize function strips all non-alphanumeric characters before comparison,
		// so both variants produce "whatevers clever" and match correctly.
		sc := newMockClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if strings.HasSuffix(r.URL.Path, "/search") {
				writeJSON(w, albumSearchJSON("wc123", "Whatever's Clever!"))
			} else {
				writeJSON(w, fullAlbumJSON("wc123", "Whatever's Clever!"))
			}
		}))
		album, err := sc.SearchAndGetAlbumDetails(context.Background(), "Whatever\u2019s Clever!", "Charlie Puth")
		if err != nil {
			t.Fatalf("unexpected error for apostrophe variant: %v", err)
		}
		if string(album.ID) != "wc123" {
			t.Errorf("expected album ID 'wc123', got '%s'", album.ID)
		}
	})

	t.Run("validation rejects album with completely different name", func(t *testing.T) {
		sc := newMockClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if strings.HasSuffix(r.URL.Path, "/search") {
				writeJSON(w, albumSearchJSON("wrong123", "Completely Different Album"))
			} else {
				writeJSON(w, fullAlbumJSON("wrong123", "Completely Different Album"))
			}
		}))
		_, err := sc.SearchAndGetAlbumDetails(context.Background(), "My Album", "My Artist")
		if err == nil {
			t.Fatal("expected validation error for mismatched album name, got nil")
		}
	})

	t.Run("falls back to plain text search when field filter returns no results", func(t *testing.T) {
		searchCallCount := 0
		var queriesUsed []string
		sc := newMockClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if strings.HasSuffix(r.URL.Path, "/search") {
				searchCallCount++
				queriesUsed = append(queriesUsed, r.URL.Query().Get("q"))
				if searchCallCount == 1 {
					writeJSON(w, emptyAlbumSearchJSON())
				} else {
					writeJSON(w, albumSearchJSON("fb123", "Whatever's Clever!"))
				}
			} else {
				writeJSON(w, fullAlbumJSON("fb123", "Whatever's Clever!"))
			}
		}))
		album, err := sc.SearchAndGetAlbumDetails(context.Background(), "Whatever's Clever!", "Charlie Puth")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if searchCallCount != 2 {
			t.Errorf("expected 2 search calls (field filter + plain fallback), got %d", searchCallCount)
		}
		if !strings.Contains(queriesUsed[0], "album:") {
			t.Errorf("first query should be a field filter, got: %s", queriesUsed[0])
		}
		if strings.Contains(queriesUsed[1], "album:") {
			t.Errorf("fallback query should not contain field filter syntax, got: %s", queriesUsed[1])
		}
		if string(album.ID) != "fb123" {
			t.Errorf("expected album ID from fallback 'fb123', got '%s'", album.ID)
		}
	})

	t.Run("sanitizes double quotes in fallback query", func(t *testing.T) {
		searchCallCount := 0
		var queriesUsed []string
		sc := newMockClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if strings.HasSuffix(r.URL.Path, "/search") {
				searchCallCount++
				queriesUsed = append(queriesUsed, r.URL.Query().Get("q"))
				if searchCallCount == 1 {
					writeJSON(w, emptyAlbumSearchJSON())
				} else {
					writeJSON(w, albumSearchJSON("quoteFallback123", `Live "At Home"`))
				}
			} else {
				writeJSON(w, fullAlbumJSON("quoteFallback123", `Live "At Home"`))
			}
		}))
		_, err := sc.SearchAndGetAlbumDetails(context.Background(), `Live "At Home"`, `The "Band"`)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(queriesUsed) != 2 {
			t.Fatalf("queriesUsed length = %d, want 2", len(queriesUsed))
		}
		if queriesUsed[1] != `Live 'At Home' The 'Band'` {
			t.Fatalf("fallback query = %q, want sanitized plain query", queriesUsed[1])
		}
	})

	t.Run("returns error when both field filter and fallback return no results", func(t *testing.T) {
		sc := newMockClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if strings.HasSuffix(r.URL.Path, "/search") {
				writeJSON(w, emptyAlbumSearchJSON())
			}
		}))
		_, err := sc.SearchAndGetAlbumDetails(context.Background(), "Nonexistent Album XYZ999", "Nobody")
		if err == nil {
			t.Fatal("expected error when all searches return no results, got nil")
		}
	})

	t.Run("caches result and avoids redundant API calls", func(t *testing.T) {
		searchCallCount := 0
		sc := newMockClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if strings.HasSuffix(r.URL.Path, "/search") {
				searchCallCount++
				writeJSON(w, albumSearchJSON("abbey123", "Abbey Road"))
			} else {
				writeJSON(w, fullAlbumJSON("abbey123", "Abbey Road"))
			}
		}))
		ctx := context.Background()

		first, err := sc.SearchAndGetAlbumDetails(ctx, "Abbey Road", "The Beatles")
		if err != nil {
			t.Fatalf("first call failed: %v", err)
		}
		second, err := sc.SearchAndGetAlbumDetails(ctx, "Abbey Road", "The Beatles")
		if err != nil {
			t.Fatalf("second call failed: %v", err)
		}
		if searchCallCount != 1 {
			t.Errorf("expected 1 search call due to cache, got %d", searchCallCount)
		}
		if first.ID != second.ID {
			t.Error("cached result does not match original")
		}
	})

	t.Run("treats unexpected cached album value as cache miss", func(t *testing.T) {
		searchCallCount := 0
		sc := newMockClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if strings.HasSuffix(r.URL.Path, "/search") {
				searchCallCount++
				writeJSON(w, albumSearchJSON("safeAlbum123", "Abbey Road"))
			} else {
				writeJSON(w, fullAlbumJSON("safeAlbum123", "Abbey Road"))
			}
		}))
		sc.albumCache.Set("abbey road|the beatles", "bad-cache-value", gocache.DefaultExpiration)

		album, err := sc.SearchAndGetAlbumDetails(context.Background(), "Abbey Road", "The Beatles")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if searchCallCount != 1 {
			t.Fatalf("searchCallCount = %d, want cache miss and one search", searchCallCount)
		}
		if string(album.ID) != "safeAlbum123" {
			t.Fatalf("album.ID = %q, want safeAlbum123", album.ID)
		}
	})

	t.Run("cache key includes artist so same title with different artist is a separate entry", func(t *testing.T) {
		searchCallCount := 0
		sc := newMockClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if strings.HasSuffix(r.URL.Path, "/search") {
				searchCallCount++
				writeJSON(w, albumSearchJSON("hits123", "Greatest Hits"))
			} else {
				writeJSON(w, fullAlbumJSON("hits123", "Greatest Hits"))
			}
		}))
		ctx := context.Background()

		_, err := sc.SearchAndGetAlbumDetails(ctx, "Greatest Hits", "Artist A")
		if err != nil {
			t.Fatalf("first call failed: %v", err)
		}
		_, err = sc.SearchAndGetAlbumDetails(ctx, "Greatest Hits", "Artist B")
		if err != nil {
			t.Fatalf("second call failed: %v", err)
		}
		if searchCallCount != 2 {
			t.Errorf("expected 2 search calls for same title with different artists, got %d", searchCallCount)
		}
	})

	t.Run("cache key trims surrounding whitespace in title and artist", func(t *testing.T) {
		searchCallCount := 0
		sc := newMockClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if strings.HasSuffix(r.URL.Path, "/search") {
				searchCallCount++
				writeJSON(w, albumSearchJSON("trimAlbum123", "Abbey Road"))
			} else {
				writeJSON(w, fullAlbumJSON("trimAlbum123", "Abbey Road"))
			}
		}))
		ctx := context.Background()

		_, err := sc.SearchAndGetAlbumDetails(ctx, "  Abbey Road  ", "  The Beatles  ")
		if err != nil {
			t.Fatalf("first call failed: %v", err)
		}
		_, err = sc.SearchAndGetAlbumDetails(ctx, "abbey road", "the beatles")
		if err != nil {
			t.Fatalf("second call failed: %v", err)
		}
		if searchCallCount != 1 {
			t.Errorf("expected 1 search call for trimmed cache key, got %d", searchCallCount)
		}
	})

	t.Run("returns error when field filter search request fails", func(t *testing.T) {
		sc := newMockClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			writeSpotifyAPIError(w, http.StatusBadGateway, "field search failed")
		}))

		_, err := sc.SearchAndGetAlbumDetails(context.Background(), "Abbey Road", "The Beatles")
		if err == nil {
			t.Fatal("expected error when field filter search request fails, got nil")
		}
		matchErr, ok := AsMatchError(err)
		if !ok {
			t.Fatalf("expected MatchError, got %T", err)
		}
		if matchErr.Info.Reason != "search_failed" || matchErr.Info.Strategy != "album_field_search" {
			t.Fatalf("info = %+v, want search_failed on album_field_search", matchErr.Info)
		}
	})

	t.Run("returns error when fallback response has no albums object", func(t *testing.T) {
		searchCallCount := 0
		sc := newMockClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			searchCallCount++
			if searchCallCount == 1 {
				writeJSON(w, emptyAlbumSearchJSON())
				return
			}
			writeJSON(w, map[string]interface{}{})
		}))

		_, err := sc.SearchAndGetAlbumDetails(context.Background(), "Abbey Road", "The Beatles")
		if err == nil {
			t.Fatal("expected error when fallback response has no albums object, got nil")
		}
		if searchCallCount != 2 {
			t.Fatalf("searchCallCount = %d, want 2", searchCallCount)
		}
	})

	t.Run("returns error when fallback search request fails", func(t *testing.T) {
		searchCallCount := 0
		sc := newMockClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if !strings.HasSuffix(r.URL.Path, "/search") {
				t.Fatalf("unexpected non-search request: %s", r.URL.Path)
			}
			searchCallCount++
			if searchCallCount == 1 {
				writeJSON(w, emptyAlbumSearchJSON())
				return
			}
			writeSpotifyAPIError(w, http.StatusBadGateway, "fallback failed")
		}))

		_, err := sc.SearchAndGetAlbumDetails(context.Background(), "Abbey Road", "The Beatles")
		if err == nil {
			t.Fatal("expected error when fallback search request fails, got nil")
		}
	})

	t.Run("returns error when album details request fails", func(t *testing.T) {
		sc := newMockClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if strings.HasSuffix(r.URL.Path, "/search") {
				writeJSON(w, albumSearchJSON("badAlbum123", "Abbey Road"))
				return
			}
			writeSpotifyAPIError(w, http.StatusBadGateway, "album details failed")
		}))

		_, err := sc.SearchAndGetAlbumDetails(context.Background(), "Abbey Road", "The Beatles")
		if err == nil {
			t.Fatal("expected error when album details request fails, got nil")
		}
	})
}

func TestSearchAlbums(t *testing.T) {
	t.Run("returns error for empty album title", func(t *testing.T) {
		sc := newMockClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
		_, err := sc.SearchAlbums(context.Background(), "")
		if err == nil {
			t.Fatal("expected error for empty title, got nil")
		}
	})

	t.Run("returns candidate albums from Spotify search", func(t *testing.T) {
		var capturedQuery string
		var capturedLimit string
		sc := newMockClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if !strings.HasSuffix(r.URL.Path, "/search") {
				t.Fatalf("unexpected request path: %s", r.URL.Path)
			}
			capturedQuery = r.URL.Query().Get("q")
			capturedLimit = r.URL.Query().Get("limit")
			writeJSON(w, albumSearchJSON("album123", "Blue Record"))
		}))

		albums, err := sc.SearchAlbums(context.Background(), "  Blue Record  ")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if capturedQuery != "Blue Record" {
			t.Fatalf("query = %q, want trimmed album title", capturedQuery)
		}
		if capturedLimit != "10" {
			t.Fatalf("limit = %q, want 10", capturedLimit)
		}
		if len(albums) != 1 || albums[0].ID.String() != "album123" || albums[0].Name != "Blue Record" {
			t.Fatalf("albums = %+v, want Blue Record candidate", albums)
		}
	})

	t.Run("returns empty slice when Spotify has no album results", func(t *testing.T) {
		sc := newMockClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			writeJSON(w, emptyAlbumSearchJSON())
		}))

		albums, err := sc.SearchAlbums(context.Background(), "Unknown Album")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(albums) != 0 {
			t.Fatalf("albums length = %d, want 0", len(albums))
		}
	})

	t.Run("returns error when spotify search request fails", func(t *testing.T) {
		sc := newMockClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			writeSpotifyAPIError(w, http.StatusInternalServerError, "search failed")
		}))

		_, err := sc.SearchAlbums(context.Background(), "Blue Record")
		if err == nil {
			t.Fatal("expected error when spotify search request fails, got nil")
		}
		if !strings.Contains(err.Error(), "spotify album search failed") {
			t.Fatalf("error = %q, want wrapped album search failure", err.Error())
		}
	})
}

func TestSearchTracks(t *testing.T) {
	t.Run("returns error for empty track title", func(t *testing.T) {
		callCount := 0
		sc := newMockClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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
		sc := newMockClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if !strings.HasSuffix(r.URL.Path, "/search") {
				t.Fatalf("unexpected request path: %s", r.URL.Path)
			}
			capturedQuery = r.URL.Query().Get("q")
			capturedLimit = r.URL.Query().Get("limit")
			writeJSON(w, trackSearchItemsJSON(trackItemJSON("track123", "Attention")))
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
		sc := newMockClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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
		sc := newMockClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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

func TestSpotifyRequestsUseDeadlineWhenCallerHasNone(t *testing.T) {
	sc := newMockClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if _, ok := r.Context().Deadline(); !ok {
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
	sc := newMockClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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

func TestClearAllCaches(t *testing.T) {
	t.Run("forces fresh API calls after caches are cleared", func(t *testing.T) {
		callCount := 0
		sc := newMockClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			callCount++
			writeJSON(w, artistSearchJSON("jm123", "John Mayer"))
		}))
		ctx := context.Background()

		_, err := sc.SearchArtistByName(ctx, "John Mayer")
		if err != nil {
			t.Fatalf("first call failed: %v", err)
		}
		if callCount != 1 {
			t.Fatalf("expected 1 call before clear, got %d", callCount)
		}

		sc.ClearAllCaches()

		_, err = sc.SearchArtistByName(ctx, "John Mayer")
		if err != nil {
			t.Fatalf("call after cache clear failed: %v", err)
		}
		if callCount != 2 {
			t.Errorf("expected 2 calls after cache clear, got %d (cache not cleared)", callCount)
		}
	})

	t.Run("clears album cache too", func(t *testing.T) {
		searchCallCount := 0
		sc := newMockClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if strings.HasSuffix(r.URL.Path, "/search") {
				searchCallCount++
				writeJSON(w, albumSearchJSON("albumClear123", "Abbey Road"))
			} else {
				writeJSON(w, fullAlbumJSON("albumClear123", "Abbey Road"))
			}
		}))
		ctx := context.Background()

		_, err := sc.SearchAndGetAlbumDetails(ctx, "Abbey Road", "The Beatles")
		if err != nil {
			t.Fatalf("first call failed: %v", err)
		}
		if searchCallCount != 1 {
			t.Fatalf("expected 1 search call before clear, got %d", searchCallCount)
		}

		sc.ClearAllCaches()

		_, err = sc.SearchAndGetAlbumDetails(ctx, "Abbey Road", "The Beatles")
		if err != nil {
			t.Fatalf("call after cache clear failed: %v", err)
		}
		if searchCallCount != 2 {
			t.Errorf("expected 2 search calls after clearing album cache, got %d", searchCallCount)
		}
	})
}

func TestNew(t *testing.T) {
	t.Run("returns a client when token exchange succeeds", func(t *testing.T) {
		httpClient := &http.Client{
			Transport: &mockTransport{handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.String() != "https://accounts.spotify.com/api/token" {
					t.Fatalf("unexpected token URL: %s", r.URL.String())
				}
				writeSpotifyTokenResponse(w, "test-token")
			})},
		}
		ctx := context.WithValue(context.Background(), oauth2.HTTPClient, httpClient)

		client, err := New(ctx, "client-id", "client-secret")
		if err != nil {
			t.Fatalf("expected constructor to succeed, got error: %v", err)
		}
		if client == nil {
			t.Fatal("expected non-nil client")
		}
	})

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
					if got := r.Header.Get("Authorization"); got != "Bearer validated-token" {
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
