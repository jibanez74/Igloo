package spotify

import (
	"context"
	"net/http"
	"strings"
	"testing"

	gocache "github.com/patrickmn/go-cache"
)

func TestSearchAndGetAlbumDetails(t *testing.T) {
	t.Run("rejects a blank album title without calling the API", func(t *testing.T) {
		callCount := 0
		sc := newMockClient(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			callCount++
		}))
		_, err := sc.SearchAndGetAlbumDetails(context.Background(), "  ", "Some Artist")
		assertMatchReason(t, err, MatchReasonEmptyQuery)
		if callCount != 0 {
			t.Fatalf("callCount = %d, want no API calls", callCount)
		}
	})

	t.Run("builds structured field query when artist is provided", func(t *testing.T) {
		var queries []string
		sc := newMockClient(albumSearchThenDetails("t123", "Thriller", &queries))
		_, err := sc.SearchAndGetAlbumDetails(context.Background(), "Thriller", "Michael Jackson")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(queries) != 1 || queries[0] != `album:"Thriller" artist:"Michael Jackson"` {
			t.Fatalf("queries = %q, want one field query with album and artist filters", queries)
		}
	})

	t.Run("sanitizes double quotes in structured field query values", func(t *testing.T) {
		var queries []string
		sc := newMockClient(albumSearchThenDetails("q123", `Live "At Home"`, &queries))
		_, err := sc.SearchAndGetAlbumDetails(context.Background(), `Live "At Home"`, `The "Band"`)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(queries) != 1 || queries[0] != `album:"Live 'At Home'" artist:"The 'Band'"` {
			t.Fatalf("queries = %q, want sanitized field query", queries)
		}
	})

	t.Run("omits artist field filter when artist is empty", func(t *testing.T) {
		var queries []string
		sc := newMockClient(albumSearchThenDetails("g123", "Greatest Hits", &queries))
		_, err := sc.SearchAndGetAlbumDetails(context.Background(), "Greatest Hits", "")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(queries) != 1 || queries[0] != `album:"Greatest Hits"` {
			t.Fatalf("queries = %q, want album filter only", queries)
		}
	})

	t.Run("strips '- Single' suffix before sending search query", func(t *testing.T) {
		var queries []string
		sc := newMockClient(albumSearchThenDetails("s123", "The Joker and the Queen", &queries))
		_, err := sc.SearchAndGetAlbumDetails(context.Background(), "The Joker And The Queen - Single", "Ed Sheeran")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(queries) != 1 || queries[0] != `album:"The Joker And The Queen" artist:"Ed Sheeran"` {
			t.Fatalf("queries = %q, want the title without its '- Single' suffix", queries)
		}
	})

	t.Run("strips '- EP' suffix before sending search query", func(t *testing.T) {
		var queries []string
		sc := newMockClient(albumSearchThenDetails("e123", "Some Album", &queries))
		_, err := sc.SearchAndGetAlbumDetails(context.Background(), "Some Album - EP", "Some Artist")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(queries) != 1 || queries[0] != `album:"Some Album" artist:"Some Artist"` {
			t.Fatalf("queries = %q, want the title without its '- EP' suffix", queries)
		}
	})

	t.Run("preserves internal '- EP' text when it is not a suffix", func(t *testing.T) {
		var queries []string
		sc := newMockClient(albumSearchThenDetails("eps123", "Foo - EP Sessions", &queries))
		_, err := sc.SearchAndGetAlbumDetails(context.Background(), "Foo - EP Sessions", "Some Artist")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(queries) != 1 || queries[0] != `album:"Foo - EP Sessions" artist:"Some Artist"` {
			t.Fatalf("queries = %q, want the full title", queries)
		}
	})

	t.Run("validation accepts album titles with apostrophe character variants", func(t *testing.T) {
		// File tag uses a curly/smart apostrophe (U+2019), Spotify uses a straight apostrophe (U+0027).
		// The normalize function strips all non-alphanumeric characters before comparison,
		// so both variants produce "whatevers clever" and match correctly.
		sc := newMockClient(albumSearchThenDetails("wc123", "Whatever's Clever!", nil))
		album, err := sc.SearchAndGetAlbumDetails(context.Background(), "Whatever’s Clever!", "Charlie Puth")
		if err != nil {
			t.Fatalf("unexpected error for apostrophe variant: %v", err)
		}
		if string(album.ID) != "wc123" {
			t.Errorf("expected album ID 'wc123', got '%s'", album.ID)
		}
	})

	t.Run("falls back to plain text search when field filter returns no results", func(t *testing.T) {
		var queries []string
		sc := newMockClient(albumFallbackThenDetails("fb123", "Whatever's Clever!", &queries))
		album, err := sc.SearchAndGetAlbumDetails(context.Background(), "Whatever's Clever!", "Charlie Puth")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(queries) != 2 {
			t.Fatalf("expected 2 search calls (field filter + plain fallback), got %d", len(queries))
		}
		if !strings.Contains(queries[0], "album:") {
			t.Errorf("first query should be a field filter, got: %s", queries[0])
		}
		if strings.Contains(queries[1], "album:") {
			t.Errorf("fallback query should not contain field filter syntax, got: %s", queries[1])
		}
		if string(album.ID) != "fb123" {
			t.Errorf("expected album ID from fallback 'fb123', got '%s'", album.ID)
		}
	})

	t.Run("falls back when the field search response has no albums object", func(t *testing.T) {
		searches := 0
		sc := newMockClient(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			isSearch := strings.HasSuffix(r.URL.Path, "/search")
			if !isSearch {
				writeJSON(w, fullAlbumJSON("noobj123", "Abbey Road"))
				return
			}
			searches++
			if searches == 1 {
				writeJSON(w, map[string]interface{}{})
				return
			}
			writeJSON(w, albumSearchJSON("noobj123", "Abbey Road"))
		}))
		album, err := sc.SearchAndGetAlbumDetails(context.Background(), "Abbey Road", "The Beatles")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if searches != 2 || string(album.ID) != "noobj123" {
			t.Fatalf("searches = %d, album = %v, want the fallback result", searches, album.ID)
		}
	})

	t.Run("sanitizes double quotes in fallback query", func(t *testing.T) {
		var queries []string
		sc := newMockClient(albumFallbackThenDetails("quoteFallback123", `Live "At Home"`, &queries))
		_, err := sc.SearchAndGetAlbumDetails(context.Background(), `Live "At Home"`, `The "Band"`)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(queries) != 2 {
			t.Fatalf("queries length = %d, want 2", len(queries))
		}
		if queries[1] != `Live 'At Home' The 'Band'` {
			t.Fatalf("fallback query = %q, want sanitized plain query", queries[1])
		}
	})

	t.Run("returns no_results when both field filter and fallback return no results", func(t *testing.T) {
		sc := newMockClient(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if strings.HasSuffix(r.URL.Path, "/search") {
				writeJSON(w, emptyAlbumSearchJSON())
			}
		}))
		_, err := sc.SearchAndGetAlbumDetails(context.Background(), "Nonexistent Album XYZ999", "Nobody")
		assertMatchReason(t, err, MatchReasonNoResults)
	})

	t.Run("reports score_below_threshold when no candidate is close enough", func(t *testing.T) {
		sc := newMockClient(albumSearchThenDetails("wrong123", "Completely Different Album", nil))
		_, err := sc.SearchAndGetAlbumDetails(context.Background(), "My Album", "My Artist")
		matchErr := assertMatchReason(t, err, MatchReasonScoreBelowThreshold)
		if matchErr.Info.CandidateName != "Completely Different Album" {
			t.Fatalf("candidate = %q, want the rejected album", matchErr.Info.CandidateName)
		}
	})

	t.Run("treats unexpected cached album value as cache miss", func(t *testing.T) {
		var queries []string
		sc := newMockClient(albumSearchThenDetails("safeAlbum123", "Abbey Road", &queries))
		sc.albumCache.Set("abbey road|the beatles", "bad-cache-value", gocache.DefaultExpiration)

		album, err := sc.SearchAndGetAlbumDetails(context.Background(), "Abbey Road", "The Beatles")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(queries) != 1 {
			t.Fatalf("searches = %d, want cache miss and one search", len(queries))
		}
		if string(album.ID) != "safeAlbum123" {
			t.Fatalf("album.ID = %q, want safeAlbum123", album.ID)
		}
		// The planted key must be the one the lookup used, or the miss above
		// proves nothing about the wrong-type branch.
		if _, ok := sc.getAlbum("abbey road|the beatles"); !ok {
			t.Fatal("the fetched album did not replace the wrong-type entry under the planted key")
		}
	})

	t.Run("cache key includes artist so same title with different artist is a separate entry", func(t *testing.T) {
		var queries []string
		sc := newMockClient(albumSearchThenDetails("hits123", "Greatest Hits", &queries))
		ctx := context.Background()

		_, err := sc.SearchAndGetAlbumDetails(ctx, "Greatest Hits", "Artist A")
		if err != nil {
			t.Fatalf("first call failed: %v", err)
		}
		_, err = sc.SearchAndGetAlbumDetails(ctx, "Greatest Hits", "Artist B")
		if err != nil {
			t.Fatalf("second call failed: %v", err)
		}
		if len(queries) != 2 {
			t.Errorf("expected 2 search calls for same title with different artists, got %d", len(queries))
		}
	})

	t.Run("cache key ignores case and surrounding whitespace in title and artist", func(t *testing.T) {
		var queries []string
		sc := newMockClient(albumSearchThenDetails("trimAlbum123", "Abbey Road", &queries))
		ctx := context.Background()

		first, err := sc.SearchAndGetAlbumDetails(ctx, "  Abbey Road  ", "  The Beatles  ")
		if err != nil {
			t.Fatalf("first call failed: %v", err)
		}
		second, err := sc.SearchAndGetAlbumDetails(ctx, "abbey road", "the beatles")
		if err != nil {
			t.Fatalf("second call failed: %v", err)
		}
		if len(queries) != 1 {
			t.Errorf("expected 1 search call for the normalized cache key, got %d", len(queries))
		}
		if first != second {
			t.Error("expected the cached album pointer to be returned")
		}
	})

	t.Run("returns search_failed when the field filter search request fails", func(t *testing.T) {
		sc := newMockClient(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			writeSpotifyAPIError(w, http.StatusBadGateway, "field search failed")
		}))

		_, err := sc.SearchAndGetAlbumDetails(context.Background(), "Abbey Road", "The Beatles")
		matchErr := assertMatchReason(t, err, MatchReasonSearchFailed)
		if matchErr.Info.Strategy != strategyAlbumFieldSearch {
			t.Fatalf("strategy = %q, want %s", matchErr.Info.Strategy, strategyAlbumFieldSearch)
		}
	})

	t.Run("returns no_results when fallback response has no albums object", func(t *testing.T) {
		searchCallCount := 0
		sc := newMockClient(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			searchCallCount++
			if searchCallCount == 1 {
				writeJSON(w, emptyAlbumSearchJSON())
				return
			}
			writeJSON(w, map[string]interface{}{})
		}))

		_, err := sc.SearchAndGetAlbumDetails(context.Background(), "Abbey Road", "The Beatles")
		assertMatchReason(t, err, MatchReasonNoResults)
		if searchCallCount != 2 {
			t.Fatalf("searchCallCount = %d, want 2", searchCallCount)
		}
	})

	t.Run("returns search_failed on the fallback strategy when the fallback request fails", func(t *testing.T) {
		searchCallCount := 0
		sc := newMockClient(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if !strings.HasSuffix(r.URL.Path, "/search") {
				t.Errorf("unexpected non-search request: %s", r.URL.Path)
				http.NotFound(w, r)
				return
			}
			searchCallCount++
			if searchCallCount == 1 {
				writeJSON(w, emptyAlbumSearchJSON())
				return
			}
			writeSpotifyAPIError(w, http.StatusBadGateway, "fallback failed")
		}))

		_, err := sc.SearchAndGetAlbumDetails(context.Background(), "Abbey Road", "The Beatles")
		matchErr := assertMatchReason(t, err, MatchReasonSearchFailed)
		if matchErr.Info.Strategy != strategyAlbumFallback {
			t.Fatalf("strategy = %q, want %s", matchErr.Info.Strategy, strategyAlbumFallback)
		}
	})

	t.Run("returns details_failed when the album details request fails", func(t *testing.T) {
		sc := newMockClient(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if strings.HasSuffix(r.URL.Path, "/search") {
				writeJSON(w, albumSearchJSON("badAlbum123", "Abbey Road"))
				return
			}
			writeSpotifyAPIError(w, http.StatusBadGateway, "album details failed")
		}))

		_, err := sc.SearchAndGetAlbumDetails(context.Background(), "Abbey Road", "The Beatles")
		matchErr := assertMatchReason(t, err, MatchReasonDetailsFailed)
		if matchErr.Info.Strategy != strategyAlbumFieldSearch || matchErr.Info.CandidateName != "Abbey Road" {
			t.Fatalf("info = %+v, want the field-search candidate", matchErr.Info)
		}
	})

	t.Run("returns details_failed on the fallback strategy when its details request fails", func(t *testing.T) {
		searchCallCount := 0
		sc := newMockClient(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			isSearch := strings.HasSuffix(r.URL.Path, "/search")
			if !isSearch {
				writeSpotifyAPIError(w, http.StatusBadGateway, "album details failed")
				return
			}
			searchCallCount++
			if searchCallCount == 1 {
				writeJSON(w, emptyAlbumSearchJSON())
				return
			}
			writeJSON(w, albumSearchJSON("fallbackBad123", "Abbey Road"))
		}))

		_, err := sc.SearchAndGetAlbumDetails(context.Background(), "Abbey Road", "The Beatles")
		matchErr := assertMatchReason(t, err, MatchReasonDetailsFailed)
		if matchErr.Info.Strategy != strategyAlbumFallback || matchErr.Info.CandidateName != "Abbey Road" {
			t.Fatalf("info = %+v, want the fallback candidate", matchErr.Info)
		}
	})
}

func TestSearchAlbums(t *testing.T) {
	t.Run("returns error for empty album title", func(t *testing.T) {
		sc := newMockClient(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
		_, err := sc.SearchAlbums(context.Background(), "")
		if err == nil {
			t.Fatal("expected error for empty title, got nil")
		}
	})

	t.Run("returns candidate albums from Spotify search", func(t *testing.T) {
		var capturedQuery string
		var capturedLimit string
		sc := newMockClient(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if !strings.HasSuffix(r.URL.Path, "/search") {
				t.Errorf("unexpected request path: %s", r.URL.Path)
				http.NotFound(w, r)
				return
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
		sc := newMockClient(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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

	t.Run("returns empty slice when response has no albums object", func(t *testing.T) {
		sc := newMockClient(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			writeJSON(w, map[string]interface{}{})
		}))

		albums, err := sc.SearchAlbums(context.Background(), "Unknown Album")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if albums == nil || len(albums) != 0 {
			t.Fatalf("albums = %#v, want an empty non-nil slice", albums)
		}
	})

	t.Run("returns error when spotify search request fails", func(t *testing.T) {
		sc := newMockClient(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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
