package spotify

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"testing"
)

func TestMatchAlbum(t *testing.T) {
	t.Run("rejects a blank album title without calling the API", func(t *testing.T) {
		callCount := 0
		sc := newMockClient(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			callCount++
		}))
		_, err := sc.MatchAlbum(context.Background(), AlbumQuery{Title: "  ", Artist: "Some Artist"})
		assertMatchReason(t, err, MatchReasonEmptyQuery)
		if callCount != 0 {
			t.Fatalf("callCount = %d, want no API calls", callCount)
		}
	})

	t.Run("builds structured field query when artist is provided", func(t *testing.T) {
		var queries []string
		sc := newMockClient(albumSearchThenDetails("t123", "Thriller", &queries))
		_, err := sc.MatchAlbum(context.Background(), AlbumQuery{Title: "Thriller", Artist: "Michael Jackson"})
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
		_, err := sc.MatchAlbum(context.Background(), AlbumQuery{Title: `Live "At Home"`, Artist: `The "Band"`})
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
		_, err := sc.MatchAlbum(context.Background(), AlbumQuery{Title: "Greatest Hits", Artist: ""})
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
		_, err := sc.MatchAlbum(context.Background(), AlbumQuery{Title: "The Joker And The Queen - Single", Artist: "Ed Sheeran"})
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
		_, err := sc.MatchAlbum(context.Background(), AlbumQuery{Title: "Some Album - EP", Artist: "Some Artist"})
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
		_, err := sc.MatchAlbum(context.Background(), AlbumQuery{Title: "Foo - EP Sessions", Artist: "Some Artist"})
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
		album, err := sc.MatchAlbum(context.Background(), AlbumQuery{Title: "Whatever’s Clever!", Artist: "Charlie Puth"})
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
		album, err := sc.MatchAlbum(context.Background(), AlbumQuery{Title: "Whatever's Clever!", Artist: "Charlie Puth"})
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
		var queries []string
		sc := newMockClient(albumSearchSequence(respondJSON(map[string]interface{}{}), respondJSON(albumSearchJSON("noobj123", "Abbey Road")), respondJSON(fullAlbumJSON("noobj123", "Abbey Road")), &queries))
		album, err := sc.MatchAlbum(context.Background(), AlbumQuery{Title: "Abbey Road", Artist: "The Beatles"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(queries) != 2 || string(album.ID) != "noobj123" {
			t.Fatalf("searches = %d, album = %v, want the fallback result", len(queries), album.ID)
		}
	})

	t.Run("sanitizes double quotes in fallback query", func(t *testing.T) {
		var queries []string
		sc := newMockClient(albumFallbackThenDetails("quoteFallback123", `Live "At Home"`, &queries))
		_, err := sc.MatchAlbum(context.Background(), AlbumQuery{Title: `Live "At Home"`, Artist: `The "Band"`})
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
		_, err := sc.MatchAlbum(context.Background(), AlbumQuery{Title: "Nonexistent Album XYZ999", Artist: "Nobody"})
		assertMatchReason(t, err, MatchReasonNoResults)
	})

	t.Run("returns search_failed when the field filter search request fails", func(t *testing.T) {
		sc := newMockClient(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			writeSpotifyAPIError(w, http.StatusBadGateway, "field search failed")
		}))

		_, err := sc.MatchAlbum(context.Background(), AlbumQuery{Title: "Abbey Road", Artist: "The Beatles"})
		matchErr := assertMatchReason(t, err, MatchReasonSearchFailed)
		if matchErr.Info.Strategy != strategyAlbumFieldSearch {
			t.Fatalf("strategy = %q, want %s", matchErr.Info.Strategy, strategyAlbumFieldSearch)
		}
	})

	t.Run("returns no_results when fallback response has no albums object", func(t *testing.T) {
		var queries []string
		sc := newMockClient(albumSearchSequence(respondJSON(emptyAlbumSearchJSON()), respondJSON(map[string]interface{}{}), nil, &queries))

		_, err := sc.MatchAlbum(context.Background(), AlbumQuery{Title: "Abbey Road", Artist: "The Beatles"})
		assertMatchReason(t, err, MatchReasonNoResults)
		if len(queries) != 2 {
			t.Fatalf("searches = %d, want 2", len(queries))
		}
	})

	t.Run("returns search_failed on the fallback strategy when the fallback request fails", func(t *testing.T) {
		sc := newMockClient(albumSearchSequence(respondJSON(emptyAlbumSearchJSON()), respondSpotifyAPIError(http.StatusBadGateway, "fallback failed"), nil, nil))

		_, err := sc.MatchAlbum(context.Background(), AlbumQuery{Title: "Abbey Road", Artist: "The Beatles"})
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

		_, err := sc.MatchAlbum(context.Background(), AlbumQuery{Title: "Abbey Road", Artist: "The Beatles"})
		matchErr := assertMatchReason(t, err, MatchReasonDetailsFailed)
		if matchErr.Info.Strategy != strategyAlbumFieldSearch || matchErr.Info.CandidateName != "Abbey Road" {
			t.Fatalf("info = %+v, want the field-search candidate", matchErr.Info)
		}
	})

	t.Run("returns details_failed on the fallback strategy when its details request fails", func(t *testing.T) {
		sc := newMockClient(albumSearchSequence(respondJSON(emptyAlbumSearchJSON()), respondJSON(albumSearchJSON("fallbackBad123", "Abbey Road")), respondSpotifyAPIError(http.StatusBadGateway, "album details failed"), nil))

		_, err := sc.MatchAlbum(context.Background(), AlbumQuery{Title: "Abbey Road", Artist: "The Beatles"})
		matchErr := assertMatchReason(t, err, MatchReasonDetailsFailed)
		if matchErr.Info.Strategy != strategyAlbumFallback || matchErr.Info.CandidateName != "Abbey Road" {
			t.Fatalf("info = %+v, want the fallback candidate", matchErr.Info)
		}
	})

	t.Run("records the rejected candidate's score", func(t *testing.T) {
		sc := newMockClient(albumSearchThenDetails("wrong123", "Completely Different Album", nil))
		_, err := sc.MatchAlbum(context.Background(), AlbumQuery{Title: "My Album", Artist: "My Artist"})
		matchErr := assertMatchReason(t, err, MatchReasonScoreBelowThreshold)
		if matchErr.Info.CandidateName != "Completely Different Album" || matchErr.Info.Score >= spotifyAlbumThreshold {
			t.Fatalf("info = %+v, want the rejected album with its score", matchErr.Info)
		}
	})

	t.Run("a candidate crediting the known artist id wins over a same-named one", func(t *testing.T) {
		sc := newMockClient(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if strings.HasSuffix(r.URL.Path, "/search") {
				other := albumItemJSON("other123", "Greatest Hits")
				other["artists"] = []interface{}{map[string]interface{}{"id": "artistOther", "name": "Nirvana"}}
				wanted := albumItemJSON("wanted123", "Greatest Hits")
				wanted["artists"] = []interface{}{map[string]interface{}{"id": "artistWanted", "name": "Nirvana Tribute Band"}}
				writeJSON(w, albumSearchItemsJSON(other, wanted))
				return
			}
			writeJSON(w, fullAlbumJSON(strings.TrimPrefix(r.URL.Path, "/v1/albums/"), "Greatest Hits"))
		}))

		album, err := sc.MatchAlbum(context.Background(), AlbumQuery{Title: "Greatest Hits", Artist: "Nirvana", ArtistID: "artistWanted"})
		if err != nil {
			t.Fatalf("MatchAlbum failed: %v", err)
		}
		if album.ID != "wanted123" {
			t.Fatalf("album.ID = %q, want the candidate credited to the known artist id", album.ID)
		}
	})

	t.Run("the files' year and track total break a tie between editions", func(t *testing.T) {
		sc := newMockClient(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if strings.HasSuffix(r.URL.Path, "/search") {
				remaster := albumItemJSON("remaster123", "Meteora")
				remaster["release_date"] = "2023-04-07"
				remaster["total_tracks"] = 34
				original := albumItemJSON("original123", "Meteora")
				original["release_date"] = "2003-03-25"
				original["total_tracks"] = 13
				writeJSON(w, albumSearchItemsJSON(remaster, original))
				return
			}
			writeJSON(w, fullAlbumJSON(strings.TrimPrefix(r.URL.Path, "/v1/albums/"), "Meteora"))
		}))

		album, err := sc.MatchAlbum(context.Background(), AlbumQuery{Title: "Meteora", Artist: "", Year: 2003, TotalTracks: 13})
		if err != nil {
			t.Fatalf("MatchAlbum failed: %v", err)
		}
		if album.ID != "original123" {
			t.Fatalf("album.ID = %q, want the edition agreeing with the tags", album.ID)
		}
	})

	t.Run("loads every page of the matched album's tracks", func(t *testing.T) {
		var requests []string
		sc := newMockClient(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			requests = append(requests, r.URL.RequestURI())
			switch {
			case strings.HasSuffix(r.URL.Path, "/search"):
				writeJSON(w, albumSearchJSON("paged123", "White Album"))
			case strings.HasSuffix(r.URL.Path, "/tracks"):
				writeJSON(w, albumTracksPageJSON(3, trackItemJSON("t3", "Glass Onion", 1, 3, 137000, false)))
			default:
				details := fullAlbumJSON("paged123", "White Album")
				details["tracks"] = albumTracksPageJSON(3, trackItemJSON("t1", "Back in the U.S.S.R.", 1, 1, 163000, false), trackItemJSON("t2", "Dear Prudence", 1, 2, 236000, true))
				writeJSON(w, details)
			}
		}))

		album, err := sc.MatchAlbum(context.Background(), AlbumQuery{Title: "White Album", Artist: "The Beatles"})
		if err != nil {
			t.Fatalf("MatchAlbum failed: %v", err)
		}
		if len(album.Tracks) != 3 || album.Tracks[2].ID != "t3" || album.Tracks[1].Explicit != true || album.Tracks[2].TrackNumber != 3 {
			t.Fatalf("tracks = %+v, want three tracks across two pages", album.Tracks)
		}
		wantPage := "/v1/albums/paged123/tracks?limit=50&offset=2"
		if len(requests) != 3 || requests[2] != wantPage {
			t.Fatalf("requests = %q, want the second page at %s", requests, wantPage)
		}
	})
}

func TestGetAlbum(t *testing.T) {
	t.Run("returns the album with its tracks", func(t *testing.T) {
		sc := newMockClient(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			details := fullAlbumJSON("album123", "Blue Record")
			details["album_type"] = "single"
			details["tracks"] = albumTracksPageJSON(1, trackItemJSON("t1", "Blue", 1, 1, 200000, false))
			writeJSON(w, details)
		}))

		album, err := sc.GetAlbum(context.Background(), "album123")
		if err != nil {
			t.Fatalf("GetAlbum failed: %v", err)
		}
		if album.ID != "album123" || album.AlbumType != "single" || len(album.Tracks) != 1 || album.Tracks[0].Name != "Blue" {
			t.Fatalf("album = %+v, want the single with its one track", album)
		}
	})

	t.Run("stops paging when a page comes back empty", func(t *testing.T) {
		pages := 0
		sc := newMockClient(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if strings.HasSuffix(r.URL.Path, "/tracks") {
				pages++
				writeJSON(w, albumTracksPageJSON(5))
				return
			}
			details := fullAlbumJSON("short123", "Short")
			details["tracks"] = albumTracksPageJSON(5, trackItemJSON("t1", "One", 1, 1, 1000, false))
			writeJSON(w, details)
		}))

		album, err := sc.GetAlbum(context.Background(), "short123")
		if err != nil {
			t.Fatalf("GetAlbum failed: %v", err)
		}
		if len(album.Tracks) != 1 || pages != 1 {
			t.Fatalf("tracks = %d, pages = %d, want one track after a single empty page", len(album.Tracks), pages)
		}
	})

	t.Run("surfaces a missing album as a 404 status error", func(t *testing.T) {
		sc := newMockClient(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			writeSpotifyAPIError(w, http.StatusNotFound, "non existing id")
		}))

		_, err := sc.GetAlbum(context.Background(), "gone123")
		statusErr, ok := errors.AsType[*StatusError](err)
		if !ok || statusErr.StatusCode != http.StatusNotFound {
			t.Fatalf("err = %v, want a 404 StatusError", err)
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
