package spotify

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"

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

func writeJSON(t *testing.T, w http.ResponseWriter, v interface{}) {
	t.Helper()
	w.Header().Set("Content-Type", "application/json")
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("marshal test JSON: %v", err)
	}
	_, err = w.Write(b)
	if err != nil {
		t.Fatalf("write test JSON: %v", err)
	}
}

func writeSpotifyAPIError(t *testing.T, w http.ResponseWriter, status int, message string) {
	t.Helper()
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	b, err := json.Marshal(map[string]interface{}{
		"error": map[string]interface{}{
			"status":  status,
			"message": message,
		},
	})
	if err != nil {
		t.Fatalf("marshal Spotify API error: %v", err)
	}
	_, err = w.Write(b)
	if err != nil {
		t.Fatalf("write Spotify API error: %v", err)
	}
}

func writeSpotifyTokenResponse(t *testing.T, w http.ResponseWriter, accessToken string) {
	t.Helper()
	w.Header().Set("Content-Type", "application/json")
	b, err := json.Marshal(map[string]interface{}{
		"access_token": accessToken,
		"token_type":   "Bearer",
		"expires_in":   3600,
	})
	if err != nil {
		t.Fatalf("marshal Spotify token response: %v", err)
	}
	_, err = w.Write(b)
	if err != nil {
		t.Fatalf("write Spotify token response: %v", err)
	}
}

func requireMatchErrorInfo(t *testing.T, err error, want MatchDebugInfo) *MatchError {
	t.Helper()
	matchErr, ok := AsMatchError(err)
	if !ok {
		t.Fatalf("AsMatchError(%T) ok = false, want true; err = %v", err, err)
	}
	if matchErr.Info != want {
		t.Fatalf("MatchError.Info = %+v, want %+v", matchErr.Info, want)
	}
	return matchErr
}

func requireMatchErrorFields(t *testing.T, err error, want MatchDebugInfo) *MatchError {
	t.Helper()
	matchErr, ok := AsMatchError(err)
	if !ok {
		t.Fatalf("AsMatchError(%T) ok = false, want true; err = %v", err, err)
	}
	got := matchErr.Info
	if want.Lookup != "" && got.Lookup != want.Lookup {
		t.Fatalf("MatchError.Info.Lookup = %q, want %q", got.Lookup, want.Lookup)
	}
	if want.Input != "" && got.Input != want.Input {
		t.Fatalf("MatchError.Info.Input = %q, want %q", got.Input, want.Input)
	}
	if want.SearchQuery != "" && got.SearchQuery != want.SearchQuery {
		t.Fatalf("MatchError.Info.SearchQuery = %q, want %q", got.SearchQuery, want.SearchQuery)
	}
	if want.Strategy != "" && got.Strategy != want.Strategy {
		t.Fatalf("MatchError.Info.Strategy = %q, want %q", got.Strategy, want.Strategy)
	}
	if want.CandidateName != "" && got.CandidateName != want.CandidateName {
		t.Fatalf("MatchError.Info.CandidateName = %q, want %q", got.CandidateName, want.CandidateName)
	}
	if want.CandidateArtist != "" && got.CandidateArtist != want.CandidateArtist {
		t.Fatalf("MatchError.Info.CandidateArtist = %q, want %q", got.CandidateArtist, want.CandidateArtist)
	}
	if want.Score != 0 && got.Score != want.Score {
		t.Fatalf("MatchError.Info.Score = %d, want %d", got.Score, want.Score)
	}
	if want.Threshold != 0 && got.Threshold != want.Threshold {
		t.Fatalf("MatchError.Info.Threshold = %d, want %d", got.Threshold, want.Threshold)
	}
	if want.Reason != "" && got.Reason != want.Reason {
		t.Fatalf("MatchError.Info.Reason = %q, want %q", got.Reason, want.Reason)
	}
	return matchErr
}

func requireStringSlice(t *testing.T, got []string, want []string) {
	t.Helper()
	if !slices.Equal(got, want) {
		t.Fatalf("strings = %#v, want %#v", got, want)
	}
}

func albumSearchInput(title, artist string) AlbumSearchInput {
	return AlbumSearchInput{
		Title:  title,
		Artist: artist,
	}
}

func artistSearchJSON(id, name string) interface{} {
	return artistSearchItemsJSON(artistItemJSON(id, name))
}

func emptyArtistSearchJSON() interface{} {
	return map[string]interface{}{
		"artists": map[string]interface{}{
			"items": []interface{}{},
			"total": 0,
			"limit": 1,
		},
	}
}

func albumSearchJSON(id, name string) interface{} {
	return albumSearchItemsJSON(albumItemJSON(id, name))
}

func emptyAlbumSearchJSON() interface{} {
	return map[string]interface{}{
		"albums": map[string]interface{}{
			"items": []interface{}{},
			"total": 0,
			"limit": 1,
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

func fullAlbumJSONWithTracks(id, name string, tracks ...string) interface{} {
	trackItems := make([]map[string]interface{}, 0, len(tracks))
	for index, track := range tracks {
		trackItems = append(trackItems, map[string]interface{}{
			"id":                fmt.Sprintf("%s-track-%d", id, index+1),
			"name":              track,
			"type":              "track",
			"duration_ms":       180000,
			"disc_number":       1,
			"track_number":      index + 1,
			"artists":           []interface{}{},
			"external_urls":     map[string]interface{}{},
			"external_ids":      map[string]interface{}{},
			"href":              "",
			"uri":               "",
			"available_markets": []interface{}{},
		})
	}

	album := fullAlbumJSON(id, name).(map[string]interface{})
	album["tracks"] = map[string]interface{}{
		"items":  trackItems,
		"total":  len(trackItems),
		"limit":  50,
		"offset": 0,
	}
	return album
}

func TestMatchError(t *testing.T) {
	wrappedErr := errors.New("spotify service unavailable")
	matchErr := &MatchError{
		Info: MatchDebugInfo{
			Lookup:          "artist",
			Input:           "Hall & Oates",
			SearchQuery:     "Hall Oates",
			Strategy:        "artist_search",
			CandidateName:   "Hall Tribute",
			CandidateArtist: "Daryl Hall",
			Score:           42,
			Threshold:       spotifyArtistThreshold,
			Reason:          "score_below_threshold",
		},
		Err: wrappedErr,
	}

	wantMessage := `spotify artist match failed input="Hall & Oates" search="Hall Oates" candidate="Hall Tribute" candidate_artist="Daryl Hall" score=42 threshold=78 strategy=artist_search reason=score_below_threshold error=spotify service unavailable`
	if got := matchErr.Error(); got != wantMessage {
		t.Fatalf("MatchError.Error() = %q, want %q", got, wantMessage)
	}

	if !errors.Is(matchErr, wrappedErr) {
		t.Fatal("errors.Is did not find wrapped error")
	}

	discovered, ok := AsMatchError(fmt.Errorf("scanner skipped artist: %w", matchErr))
	if !ok {
		t.Fatal("AsMatchError did not unwrap a wrapped MatchError")
	}
	if discovered != matchErr {
		t.Fatalf("AsMatchError returned %#v, want original MatchError %#v", discovered, matchErr)
	}

	if _, ok := AsMatchError(errors.New("plain error")); ok {
		t.Fatal("AsMatchError returned ok for a non-MatchError")
	}
}

func TestSpotifyMatchScoring(t *testing.T) {
	t.Run("scores artist names exactly", func(t *testing.T) {
		tests := []struct {
			name      string
			query     string
			candidate string
			want      int
		}{
			{name: "diacritic variant", query: "Beyonce", candidate: "Beyoncé", want: 100},
			{name: "punctuation variant", query: "Guns N Roses", candidate: "Guns N' Roses", want: 100},
			{name: "canonical duo variant", query: "Hall & Oates", candidate: "Daryl Hall & John Oates", want: 80},
			{name: "compound credit primary artist only", query: "Charlie Puth & Coco Jones", candidate: "Charlie Puth", want: 30},
			{name: "single token prefix only", query: "Pink", candidate: "Pink Floyd", want: 68},
			{name: "unrelated artists", query: "Unknown Artist", candidate: "Charlie Puth", want: 0},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				got := scoreArtistName(tt.query, tt.candidate)
				if got != tt.want {
					t.Fatalf("scoreArtistName(%q, %q) = %d, want %d", tt.query, tt.candidate, got, tt.want)
				}
			})
		}
	})

	t.Run("scores album titles exactly", func(t *testing.T) {
		tests := []struct {
			name      string
			query     string
			candidate string
			want      int
		}{
			{name: "remaster noise token", query: "Abbey Road (Remastered)", candidate: "Abbey Road", want: 98},
			{name: "apostrophe variant", query: "Whatever\u2019s Clever!", candidate: "Whatever's Clever!", want: 100},
			{name: "unicode title token", query: "Love Yourself Answer", candidate: "Love Yourself 結 'Answer'", want: 86},
			{name: "edition noise tokens", query: "Meteora Deluxe Edition", candidate: "Meteora", want: 98},
			{name: "different album with shared noise token", query: "My Album", candidate: "Completely Different Album", want: 35},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				got := scoreAlbumTitle(tt.query, tt.candidate)
				if got != tt.want {
					t.Fatalf("scoreAlbumTitle(%q, %q) = %d, want %d", tt.query, tt.candidate, got, tt.want)
				}
			})
		}
	})
}

func TestSpotifyMatchSelectors(t *testing.T) {
	t.Run("selectBestArtistMatch chooses highest score after index penalty", func(t *testing.T) {
		artists := []spotifylib.FullArtist{
			{SimpleArtist: spotifylib.SimpleArtist{ID: "tribute", Name: "Beyonce Smith"}},
			{SimpleArtist: spotifylib.SimpleArtist{ID: "exact", Name: "Beyoncé"}},
		}

		artist, info := selectBestArtistMatch("Beyonce", artists, "artist_search")
		if artist == nil {
			t.Fatal("artist = nil, want match")
		}
		if artist.ID != "exact" {
			t.Fatalf("artist.ID = %q, want %q", artist.ID, "exact")
		}
		if info != (MatchDebugInfo{
			Lookup:        "artist",
			Input:         "Beyonce",
			SearchQuery:   "Beyonce",
			Strategy:      "artist_search",
			CandidateName: "Beyoncé",
			Score:         99,
			Threshold:     spotifyArtistThreshold,
			Reason:        "accepted",
		}) {
			t.Fatalf("MatchDebugInfo = %+v", info)
		}
	})

	t.Run("selectBestArtistMatch rejects below-threshold candidate", func(t *testing.T) {
		artists := []spotifylib.FullArtist{
			{SimpleArtist: spotifylib.SimpleArtist{ID: "primary", Name: "Charlie Puth"}},
		}

		artist, info := selectBestArtistMatch("Charlie Puth & Coco Jones", artists, "artist_search")
		if artist != nil {
			t.Fatalf("artist = %#v, want nil", artist)
		}
		if info.Score != 30 || info.Reason != "score_below_threshold" {
			t.Fatalf("info = %+v, want score 30 below-threshold", info)
		}
	})

	t.Run("selectBestAlbumMatch chooses later artist match despite index penalty", func(t *testing.T) {
		albums := []spotifylib.SimpleAlbum{
			{
				ID:      "wrong123",
				Name:    "Greatest Hits",
				Artists: []spotifylib.SimpleArtist{{Name: "Artist A"}},
			},
			{
				ID:      "right123",
				Name:    "Greatest Hits",
				Artists: []spotifylib.SimpleArtist{{Name: "Artist B"}},
			},
		}

		album, info := selectBestAlbumMatch("Greatest Hits", "Artist B", 0, albums, "query", "album_field_search")
		if album == nil {
			t.Fatal("album = nil, want match")
		}
		if album.ID != "right123" {
			t.Fatalf("album.ID = %q, want %q", album.ID, "right123")
		}
		if info != (MatchDebugInfo{
			Lookup:          "album",
			Input:           "Greatest Hits",
			SearchQuery:     "query",
			Strategy:        "album_field_search",
			CandidateName:   "Greatest Hits",
			CandidateArtist: "Artist B",
			Score:           99,
			Threshold:       spotifyAlbumThreshold,
			Reason:          "accepted",
		}) {
			t.Fatalf("MatchDebugInfo = %+v", info)
		}
	})

	t.Run("selectBestAlbumMatch uses release year as supporting evidence", func(t *testing.T) {
		albums := []spotifylib.SimpleAlbum{
			{
				ID:          "old123",
				Name:        "Greatest Hits",
				ReleaseDate: "1999-01-01",
				Artists:     []spotifylib.SimpleArtist{{Name: "Artist B"}},
			},
			{
				ID:          "new123",
				Name:        "Greatest Hits",
				ReleaseDate: "2020-01-01",
				Artists:     []spotifylib.SimpleArtist{{Name: "Artist B"}},
			},
		}

		album, info := selectBestAlbumMatch("Greatest Hits", "Artist B", 2020, albums, "query", "album_field_search")
		if album == nil {
			t.Fatal("album = nil, want match")
		}
		if album.ID != "new123" {
			t.Fatalf("album.ID = %q, want %q", album.ID, "new123")
		}
		if info.CandidateName != "Greatest Hits" || info.Reason != "accepted" {
			t.Fatalf("MatchDebugInfo = %+v, want accepted Greatest Hits", info)
		}
	})
}

func TestSearchArtistByName(t *testing.T) {
	t.Run("returns error for empty artist name", func(t *testing.T) {
		sc := newMockClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
		_, err := sc.SearchArtistByName(context.Background(), "")
		if err == nil {
			t.Fatal("expected error for empty artist name, got nil")
		}
		requireMatchErrorInfo(t, err, MatchDebugInfo{
			Lookup:    "artist",
			Input:     "",
			Strategy:  "artist_search",
			Reason:    "empty_query",
			Threshold: spotifyArtistThreshold,
		})
	})

	t.Run("returns artist when name matches", func(t *testing.T) {
		sc := newMockClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			writeJSON(t, w, artistSearchJSON("abc123", "Charlie Puth"))
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
		// "Hall & Oates" is a real artist on Spotify. The returned name equals the query,
		// so the one-directional validation passes and the band is not treated as a compound credit.
		sc := newMockClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			writeJSON(t, w, artistSearchJSON("hallOatesID", "Hall & Oates"))
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
		// The one-directional check: "charlie puth" does not contain "charlie puth & coco jones",
		// so validation fails. This is the discriminator that allows safe compound-credit splitting.
		sc := newMockClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			writeJSON(t, w, artistSearchJSON("charliePuthID", "Charlie Puth"))
		}))
		_, err := sc.SearchArtistByName(context.Background(), "Charlie Puth & Coco Jones")
		if err == nil {
			t.Fatal("expected validation error for compound credit, got nil")
		}
		requireMatchErrorFields(t, err, MatchDebugInfo{
			Lookup:        "artist",
			Input:         "Charlie Puth & Coco Jones",
			SearchQuery:   "Charlie Puth & Coco Jones",
			Strategy:      "artist_search",
			CandidateName: "Charlie Puth",
			Threshold:     spotifyArtistThreshold,
			Reason:        "score_below_threshold",
		})
	})

	t.Run("returns error when no results", func(t *testing.T) {
		sc := newMockClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			writeJSON(t, w, emptyArtistSearchJSON())
		}))
		_, err := sc.SearchArtistByName(context.Background(), "Unknown Artist XYZ999")
		if err == nil {
			t.Fatal("expected error for empty results, got nil")
		}
		requireMatchErrorInfo(t, err, MatchDebugInfo{
			Lookup:      "artist",
			Input:       "Unknown Artist XYZ999",
			SearchQuery: "Unknown Artist XYZ999",
			Strategy:    "artist_search",
			Reason:      "no_results",
			Threshold:   spotifyArtistThreshold,
		})
	})

	t.Run("returns no-results match error when artists payload is missing", func(t *testing.T) {
		sc := newMockClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			writeJSON(t, w, map[string]interface{}{})
		}))
		_, err := sc.SearchArtistByName(context.Background(), "John Mayer")
		if err == nil {
			t.Fatal("expected error for missing artists payload, got nil")
		}
		requireMatchErrorInfo(t, err, MatchDebugInfo{
			Lookup:      "artist",
			Input:       "John Mayer",
			SearchQuery: "John Mayer",
			Strategy:    "artist_search",
			Reason:      "no_results",
			Threshold:   spotifyArtistThreshold,
		})
	})

	t.Run("caches result and avoids redundant API calls", func(t *testing.T) {
		callCount := 0
		sc := newMockClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			callCount++
			writeJSON(t, w, artistSearchJSON("edID", "Ed Sheeran"))
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

	t.Run("cache key is case-insensitive", func(t *testing.T) {
		callCount := 0
		sc := newMockClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			callCount++
			writeJSON(t, w, artistSearchJSON("johnID", "John Mayer"))
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
			writeJSON(t, w, artistSearchJSON("trimmedID", "John Mayer"))
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
			writeSpotifyAPIError(t, w, http.StatusInternalServerError, "search failed")
		}))
		_, err := sc.SearchArtistByName(context.Background(), "John Mayer")
		if err == nil {
			t.Fatal("expected error when spotify search request fails, got nil")
		}
		matchErr := requireMatchErrorInfo(t, err, MatchDebugInfo{
			Lookup:      "artist",
			Input:       "John Mayer",
			SearchQuery: "John Mayer",
			Strategy:    "artist_search",
			Reason:      "search_failed",
			Threshold:   spotifyArtistThreshold,
		})
		if matchErr.Err == nil {
			t.Fatal("expected wrapped Spotify search error, got nil")
		}
	})
}

func TestSearchAndGetAlbumDetails(t *testing.T) {
	t.Run("returns error for empty album title", func(t *testing.T) {
		sc := newMockClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
		_, err := sc.SearchAndGetAlbumDetails(context.Background(), albumSearchInput("", "Some Artist"))
		if err == nil {
			t.Fatal("expected error for empty title, got nil")
		}
		requireMatchErrorInfo(t, err, MatchDebugInfo{
			Lookup:    "album",
			Input:     "",
			Strategy:  "album_field_search",
			Reason:    "empty_query",
			Threshold: spotifyAlbumThreshold,
		})
	})

	t.Run("builds structured field query when artist is provided", func(t *testing.T) {
		var capturedQuery string
		sc := newMockClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if strings.HasSuffix(r.URL.Path, "/search") {
				capturedQuery = r.URL.Query().Get("q")
				writeJSON(t, w, albumSearchJSON("t123", "Thriller"))
			} else {
				writeJSON(t, w, fullAlbumJSON("t123", "Thriller"))
			}
		}))
		_, err := sc.SearchAndGetAlbumDetails(context.Background(), albumSearchInput("Thriller", "Michael Jackson"))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		wantQuery := `album:"Thriller" artist:"Michael Jackson"`
		if capturedQuery != wantQuery {
			t.Errorf("query = %q, want %q", capturedQuery, wantQuery)
		}
	})

	t.Run("omits artist field filter when artist is empty", func(t *testing.T) {
		var capturedQuery string
		sc := newMockClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if strings.HasSuffix(r.URL.Path, "/search") {
				capturedQuery = r.URL.Query().Get("q")
				writeJSON(t, w, albumSearchJSON("g123", "Greatest Hits"))
			} else {
				writeJSON(t, w, fullAlbumJSON("g123", "Greatest Hits"))
			}
		}))
		_, err := sc.SearchAndGetAlbumDetails(context.Background(), albumSearchInput("Greatest Hits", ""))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		wantQuery := `album:"Greatest Hits"`
		if capturedQuery != wantQuery {
			t.Errorf("query = %q, want %q", capturedQuery, wantQuery)
		}
	})

	t.Run("normalizes album suffixes in field search query", func(t *testing.T) {
		tests := []struct {
			name      string
			title     string
			artist    string
			albumName string
			wantQuery string
		}{
			{
				name:      "strips single suffix",
				title:     "The Joker And The Queen - Single",
				artist:    "Ed Sheeran",
				albumName: "The Joker and the Queen",
				wantQuery: `album:"The Joker And The Queen" artist:"Ed Sheeran"`,
			},
			{
				name:      "strips EP suffix",
				title:     "Some Album - EP",
				artist:    "Some Artist",
				albumName: "Some Album",
				wantQuery: `album:"Some Album" artist:"Some Artist"`,
			},
			{
				name:      "strips LP suffix",
				title:     "Some Album - LP",
				artist:    "Some Artist",
				albumName: "Some Album",
				wantQuery: `album:"Some Album" artist:"Some Artist"`,
			},
			{
				name:      "strips album suffix",
				title:     "Some Album - Album",
				artist:    "Some Artist",
				albumName: "Some Album",
				wantQuery: `album:"Some Album" artist:"Some Artist"`,
			},
			{
				name:      "preserves internal EP text",
				title:     "Foo - EP Sessions",
				artist:    "Some Artist",
				albumName: "Foo - EP Sessions",
				wantQuery: `album:"Foo - EP Sessions" artist:"Some Artist"`,
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				var capturedQuery string
				sc := newMockClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					if strings.HasSuffix(r.URL.Path, "/search") {
						capturedQuery = r.URL.Query().Get("q")
						writeJSON(t, w, albumSearchJSON("suffix123", tt.albumName))
						return
					}

					writeJSON(t, w, fullAlbumJSON("suffix123", tt.albumName))
				}))
				_, err := sc.SearchAndGetAlbumDetails(context.Background(), albumSearchInput(tt.title, tt.artist))
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				if capturedQuery != tt.wantQuery {
					t.Fatalf("query = %q, want %q", capturedQuery, tt.wantQuery)
				}
			})
		}
	})

	t.Run("validation accepts when result name is contained in query title", func(t *testing.T) {
		// Spotify returns "Abbey Road" but the file tag says "Abbey Road (Remastered)".
		// Bidirectional check: "abbey road" is contained in "abbey road remastered" -> passes.
		sc := newMockClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if strings.HasSuffix(r.URL.Path, "/search") {
				writeJSON(t, w, albumSearchJSON("ar123", "Abbey Road"))
			} else {
				writeJSON(t, w, fullAlbumJSON("ar123", "Abbey Road"))
			}
		}))
		album, err := sc.SearchAndGetAlbumDetails(context.Background(), albumSearchInput("Abbey Road (Remastered)", "The Beatles"))
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
				writeJSON(t, w, albumSearchJSON("wc123", "Whatever's Clever!"))
			} else {
				writeJSON(t, w, fullAlbumJSON("wc123", "Whatever's Clever!"))
			}
		}))
		album, err := sc.SearchAndGetAlbumDetails(context.Background(), albumSearchInput("Whatever\u2019s Clever!", "Charlie Puth"))
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
				writeJSON(t, w, albumSearchJSON("wrong123", "Completely Different Album"))
			} else {
				writeJSON(t, w, fullAlbumJSON("wrong123", "Completely Different Album"))
			}
		}))
		_, err := sc.SearchAndGetAlbumDetails(context.Background(), albumSearchInput("My Album", "My Artist"))
		if err == nil {
			t.Fatal("expected validation error for mismatched album name, got nil")
		}
		requireMatchErrorFields(t, err, MatchDebugInfo{
			Lookup:        "album",
			Input:         "My Album",
			SearchQuery:   `album:"My Album" artist:"My Artist"`,
			Strategy:      "album_field_search",
			CandidateName: "Completely Different Album",
			Threshold:     spotifyAlbumThreshold,
			Reason:        "score_below_threshold",
		})
	})

	t.Run("returns error when field search request fails", func(t *testing.T) {
		sc := newMockClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if !strings.HasSuffix(r.URL.Path, "/search") {
				t.Fatalf("unexpected non-search request: %s", r.URL.Path)
			}
			writeSpotifyAPIError(t, w, http.StatusBadGateway, "field search failed")
		}))

		_, err := sc.SearchAndGetAlbumDetails(context.Background(), albumSearchInput("Abbey Road", "The Beatles"))
		if err == nil {
			t.Fatal("expected error when field search request fails, got nil")
		}
		matchErr := requireMatchErrorInfo(t, err, MatchDebugInfo{
			Lookup:      "album",
			Input:       "Abbey Road",
			SearchQuery: `album:"Abbey Road" artist:"The Beatles"`,
			Strategy:    "album_field_search",
			Reason:      "search_failed",
			Threshold:   spotifyAlbumThreshold,
		})
		if matchErr.Err == nil {
			t.Fatal("expected wrapped Spotify field search error, got nil")
		}
	})

	t.Run("falls back to plain text search when field filter returns no results", func(t *testing.T) {
		searchCallCount := 0
		var requests []string
		sc := newMockClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if strings.HasSuffix(r.URL.Path, "/search") {
				searchCallCount++
				requests = append(requests, "search:"+r.URL.Query().Get("q"))
				if searchCallCount == 1 {
					writeJSON(t, w, emptyAlbumSearchJSON())
				} else {
					writeJSON(t, w, albumSearchJSON("fb123", "Whatever's Clever!"))
				}
			} else {
				requests = append(requests, "details:"+r.URL.Path)
				writeJSON(t, w, fullAlbumJSON("fb123", "Whatever's Clever!"))
			}
		}))
		album, err := sc.SearchAndGetAlbumDetails(context.Background(), albumSearchInput("Whatever's Clever!", "Charlie Puth"))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if searchCallCount != 2 {
			t.Errorf("expected 2 search calls (field filter + plain fallback), got %d", searchCallCount)
		}
		requireStringSlice(t, requests, []string{
			`search:album:"Whatever's Clever!" artist:"Charlie Puth"`,
			`search:Whatever's Clever! Charlie Puth`,
			"details:/v1/albums/fb123",
		})
		if string(album.ID) != "fb123" {
			t.Errorf("expected album ID from fallback 'fb123', got '%s'", album.ID)
		}
	})

	t.Run("falls back when field search response is missing albums payload", func(t *testing.T) {
		searchCallCount := 0
		var requests []string
		sc := newMockClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if strings.HasSuffix(r.URL.Path, "/search") {
				searchCallCount++
				requests = append(requests, "search:"+r.URL.Query().Get("q"))
				if searchCallCount == 1 {
					writeJSON(t, w, map[string]interface{}{})
					return
				}

				writeJSON(t, w, albumSearchJSON("missingField123", "Abbey Road"))
				return
			}

			requests = append(requests, "details:"+r.URL.Path)
			writeJSON(t, w, fullAlbumJSON("missingField123", "Abbey Road"))
		}))

		album, err := sc.SearchAndGetAlbumDetails(context.Background(), albumSearchInput("Abbey Road", "The Beatles"))
		if err != nil {
			t.Fatalf("unexpected error after missing field-search albums payload fallback: %v", err)
		}
		if searchCallCount != 2 {
			t.Fatalf("searchCallCount = %d, want 2", searchCallCount)
		}
		requireStringSlice(t, requests, []string{
			`search:album:"Abbey Road" artist:"The Beatles"`,
			"search:Abbey Road The Beatles",
			"details:/v1/albums/missingField123",
		})
		if string(album.ID) != "missingField123" {
			t.Fatalf("album.ID = %q, want %q", album.ID, "missingField123")
		}
	})

	t.Run("accepts album-only fallback when track evidence matches", func(t *testing.T) {
		searchCallCount := 0
		var requests []string
		sc := newMockClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if strings.HasSuffix(r.URL.Path, "/search") {
				searchCallCount++
				requests = append(requests, "search:"+r.URL.Query().Get("q"))
				if searchCallCount < 3 {
					writeJSON(t, w, emptyAlbumSearchJSON())
					return
				}

				writeJSON(t, w, albumSearchItemsJSON(
					albumItemJSON("albumOnly123", "Deep Cut Album"),
				))
				return
			}

			requests = append(requests, "details:"+r.URL.Path)
			writeJSON(t, w, fullAlbumJSONWithTracks("albumOnly123", "Deep Cut Album", "Needle Song"))
		}))

		album, err := sc.SearchAndGetAlbumDetails(context.Background(), AlbumSearchInput{
			Title:       "Deep Cut Album",
			Artist:      "Library Artist",
			TrackTitles: []string{"Needle Song"},
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if string(album.ID) != "albumOnly123" {
			t.Fatalf("album.ID = %q, want %q", album.ID, "albumOnly123")
		}
		requireStringSlice(t, requests, []string{
			`search:album:"Deep Cut Album" artist:"Library Artist"`,
			"search:Deep Cut Album Library Artist",
			`search:album:"Deep Cut Album"`,
			"details:/v1/albums/albumOnly123",
		})
	})

	t.Run("rejects album-only fallback when track evidence does not match", func(t *testing.T) {
		searchCallCount := 0
		sc := newMockClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if strings.HasSuffix(r.URL.Path, "/search") {
				searchCallCount++
				if searchCallCount < 3 {
					writeJSON(t, w, emptyAlbumSearchJSON())
					return
				}

				writeJSON(t, w, albumSearchItemsJSON(
					albumItemJSON("albumOnly123", "Deep Cut Album"),
				))
				return
			}

			writeJSON(t, w, fullAlbumJSONWithTracks("albumOnly123", "Deep Cut Album", "Different Song"))
		}))

		_, err := sc.SearchAndGetAlbumDetails(context.Background(), AlbumSearchInput{
			Title:       "Deep Cut Album",
			Artist:      "Library Artist",
			TrackTitles: []string{"Needle Song"},
		})
		if err == nil {
			t.Fatal("expected track evidence rejection, got nil")
		}
		requireMatchErrorFields(t, err, MatchDebugInfo{
			Lookup:    "album",
			Input:     "Deep Cut Album",
			Strategy:  "album_title_field_search",
			Reason:    "track_mismatch",
			Threshold: spotifyAlbumThreshold,
		})
	})

	t.Run("returns error when all album search strategies return no results", func(t *testing.T) {
		sc := newMockClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if strings.HasSuffix(r.URL.Path, "/search") {
				writeJSON(t, w, emptyAlbumSearchJSON())
			}
		}))
		_, err := sc.SearchAndGetAlbumDetails(context.Background(), albumSearchInput("Nonexistent Album XYZ999", "Nobody"))
		if err == nil {
			t.Fatal("expected error when all searches return no results, got nil")
		}
		requireMatchErrorInfo(t, err, MatchDebugInfo{
			Lookup:      "album",
			Input:       "Nonexistent Album XYZ999",
			SearchQuery: "Nonexistent Album XYZ999",
			Strategy:    "album_title_fallback_search",
			Reason:      "no_results",
			Threshold:   spotifyAlbumThreshold,
		})
	})

	t.Run("returns no-results error when fallback response is missing albums payload", func(t *testing.T) {
		searchCallCount := 0
		sc := newMockClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if !strings.HasSuffix(r.URL.Path, "/search") {
				t.Fatalf("unexpected non-search request: %s", r.URL.Path)
			}
			searchCallCount++
			if searchCallCount == 1 {
				writeJSON(t, w, emptyAlbumSearchJSON())
				return
			}
			writeJSON(t, w, map[string]interface{}{})
		}))

		_, err := sc.SearchAndGetAlbumDetails(context.Background(), albumSearchInput("Nonexistent Album XYZ999", "Nobody"))
		if err == nil {
			t.Fatal("expected error when fallback albums payload is missing, got nil")
		}
		if searchCallCount != 4 {
			t.Fatalf("searchCallCount = %d, want 4", searchCallCount)
		}
		requireMatchErrorInfo(t, err, MatchDebugInfo{
			Lookup:      "album",
			Input:       "Nonexistent Album XYZ999",
			SearchQuery: "Nonexistent Album XYZ999",
			Strategy:    "album_title_fallback_search",
			Reason:      "no_results",
			Threshold:   spotifyAlbumThreshold,
		})
	})

	t.Run("caches result and avoids redundant API calls", func(t *testing.T) {
		searchCallCount := 0
		detailsCallCount := 0
		sc := newMockClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if strings.HasSuffix(r.URL.Path, "/search") {
				searchCallCount++
				writeJSON(t, w, albumSearchJSON("abbey123", "Abbey Road"))
			} else {
				detailsCallCount++
				writeJSON(t, w, fullAlbumJSON("abbey123", "Abbey Road"))
			}
		}))
		ctx := context.Background()

		first, err := sc.SearchAndGetAlbumDetails(ctx, albumSearchInput("Abbey Road", "The Beatles"))
		if err != nil {
			t.Fatalf("first call failed: %v", err)
		}
		second, err := sc.SearchAndGetAlbumDetails(ctx, albumSearchInput("Abbey Road", "The Beatles"))
		if err != nil {
			t.Fatalf("second call failed: %v", err)
		}
		if searchCallCount != 1 {
			t.Errorf("expected 1 search call due to cache, got %d", searchCallCount)
		}
		if detailsCallCount != 1 {
			t.Errorf("expected 1 album details call due to cache, got %d", detailsCallCount)
		}
		if first.ID != second.ID {
			t.Error("cached result does not match original")
		}
	})

	t.Run("cache key includes artist so same title with different artist is a separate entry", func(t *testing.T) {
		searchCallCount := 0
		sc := newMockClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if strings.HasSuffix(r.URL.Path, "/search") {
				searchCallCount++
				writeJSON(t, w, albumSearchJSON("hits123", "Greatest Hits"))
			} else {
				writeJSON(t, w, fullAlbumJSON("hits123", "Greatest Hits"))
			}
		}))
		ctx := context.Background()

		_, err := sc.SearchAndGetAlbumDetails(ctx, albumSearchInput("Greatest Hits", "Artist A"))
		if err != nil {
			t.Fatalf("first call failed: %v", err)
		}
		_, err = sc.SearchAndGetAlbumDetails(ctx, albumSearchInput("Greatest Hits", "Artist B"))
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
				writeJSON(t, w, albumSearchJSON("trimAlbum123", "Abbey Road"))
			} else {
				writeJSON(t, w, fullAlbumJSON("trimAlbum123", "Abbey Road"))
			}
		}))
		ctx := context.Background()

		_, err := sc.SearchAndGetAlbumDetails(ctx, albumSearchInput("  Abbey Road  ", "  The Beatles  "))
		if err != nil {
			t.Fatalf("first call failed: %v", err)
		}
		_, err = sc.SearchAndGetAlbumDetails(ctx, albumSearchInput("abbey road", "the beatles"))
		if err != nil {
			t.Fatalf("second call failed: %v", err)
		}
		if searchCallCount != 1 {
			t.Errorf("expected 1 search call for trimmed cache key, got %d", searchCallCount)
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
				writeJSON(t, w, emptyAlbumSearchJSON())
				return
			}
			writeSpotifyAPIError(t, w, http.StatusBadGateway, "fallback failed")
		}))

		_, err := sc.SearchAndGetAlbumDetails(context.Background(), albumSearchInput("Abbey Road", "The Beatles"))
		if err == nil {
			t.Fatal("expected error when fallback search request fails, got nil")
		}
		matchErr := requireMatchErrorInfo(t, err, MatchDebugInfo{
			Lookup:      "album",
			Input:       "Abbey Road",
			SearchQuery: "Abbey Road The Beatles",
			Strategy:    "album_fallback_search",
			Reason:      "search_failed",
			Threshold:   spotifyAlbumThreshold,
		})
		if matchErr.Err == nil {
			t.Fatal("expected wrapped Spotify fallback search error, got nil")
		}
	})

	t.Run("returns error when album details request fails", func(t *testing.T) {
		sc := newMockClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if strings.HasSuffix(r.URL.Path, "/search") {
				writeJSON(t, w, albumSearchJSON("badAlbum123", "Abbey Road"))
				return
			}
			writeSpotifyAPIError(t, w, http.StatusBadGateway, "album details failed")
		}))

		_, err := sc.SearchAndGetAlbumDetails(context.Background(), albumSearchInput("Abbey Road", "The Beatles"))
		if err == nil {
			t.Fatal("expected error when album details request fails, got nil")
		}
		matchErr := requireMatchErrorInfo(t, err, MatchDebugInfo{
			Lookup:        "album",
			Input:         "Abbey Road",
			SearchQuery:   `album:"Abbey Road" artist:"The Beatles"`,
			Strategy:      "album_field_search",
			CandidateName: "Abbey Road",
			Reason:        "details_failed",
			Threshold:     spotifyAlbumThreshold,
		})
		if matchErr.Err == nil {
			t.Fatal("expected wrapped Spotify album details error, got nil")
		}
	})
}

func TestClearAllCaches(t *testing.T) {
	t.Run("forces fresh API calls after caches are cleared", func(t *testing.T) {
		callCount := 0
		sc := newMockClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			callCount++
			writeJSON(t, w, artistSearchJSON("jm123", "John Mayer"))
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
		detailsCallCount := 0
		sc := newMockClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if strings.HasSuffix(r.URL.Path, "/search") {
				searchCallCount++
				writeJSON(t, w, albumSearchJSON("albumClear123", "Abbey Road"))
			} else {
				detailsCallCount++
				writeJSON(t, w, fullAlbumJSON("albumClear123", "Abbey Road"))
			}
		}))
		ctx := context.Background()

		_, err := sc.SearchAndGetAlbumDetails(ctx, albumSearchInput("Abbey Road", "The Beatles"))
		if err != nil {
			t.Fatalf("first call failed: %v", err)
		}
		if searchCallCount != 1 {
			t.Fatalf("expected 1 search call before clear, got %d", searchCallCount)
		}
		if detailsCallCount != 1 {
			t.Fatalf("expected 1 details call before clear, got %d", detailsCallCount)
		}

		sc.ClearAllCaches()

		_, err = sc.SearchAndGetAlbumDetails(ctx, albumSearchInput("Abbey Road", "The Beatles"))
		if err != nil {
			t.Fatalf("call after cache clear failed: %v", err)
		}
		if searchCallCount != 2 {
			t.Errorf("expected 2 search calls after clearing album cache, got %d", searchCallCount)
		}
		if detailsCallCount != 2 {
			t.Errorf("expected 2 details calls after clearing album cache, got %d", detailsCallCount)
		}
	})
}

func TestNew(t *testing.T) {
	t.Run("returns a client when token exchange succeeds", func(t *testing.T) {
		tokenCallCount := 0
		httpClient := &http.Client{
			Transport: &mockTransport{handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				tokenCallCount++
				if r.Method != http.MethodPost {
					t.Fatalf("token request method = %s, want %s", r.Method, http.MethodPost)
				}
				if r.URL.String() != "https://accounts.spotify.com/api/token" {
					t.Fatalf("unexpected token URL: %s", r.URL.String())
				}
				writeSpotifyTokenResponse(t, w, "test-token")
			})},
		}
		ctx := context.WithValue(context.Background(), interface{}(oauth2.HTTPClient), httpClient)

		client, err := New(ctx, "client-id", "client-secret")
		if err != nil {
			t.Fatalf("expected constructor to succeed, got error: %v", err)
		}
		if client == nil {
			t.Fatal("expected non-nil client")
		}
		if tokenCallCount != 1 {
			t.Fatalf("tokenCallCount = %d, want 1", tokenCallCount)
		}
	})

	t.Run("returns error when token exchange fails", func(t *testing.T) {
		tokenCallCount := 0
		httpClient := &http.Client{
			Transport: &mockTransport{handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				tokenCallCount++
				if r.Method != http.MethodPost {
					t.Fatalf("token request method = %s, want %s", r.Method, http.MethodPost)
				}
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusUnauthorized)
				writeJSON(t, w, map[string]interface{}{
					"error":             "invalid_client",
					"error_description": "bad credentials",
				})
			})},
		}
		ctx := context.WithValue(context.Background(), interface{}(oauth2.HTTPClient), httpClient)

		client, err := New(ctx, "client-id", "client-secret")
		if err == nil {
			t.Fatal("expected constructor error, got nil")
		}
		if client != nil {
			t.Fatal("expected nil client on constructor failure")
		}
		if tokenCallCount != 2 {
			t.Fatalf("tokenCallCount = %d, want 2", tokenCallCount)
		}
	})

	t.Run("retries rate limited API requests", func(t *testing.T) {
		searchCallCount := 0
		httpClient := &http.Client{
			Transport: &mockTransport{handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.String() == "https://accounts.spotify.com/api/token" {
					writeSpotifyTokenResponse(t, w, "test-token")
					return
				}

				if !strings.HasSuffix(r.URL.Path, "/search") {
					t.Fatalf("unexpected request URL: %s", r.URL.String())
				}

				searchCallCount++
				if searchCallCount == 1 {
					w.Header().Set("Retry-After", "0")
					writeSpotifyAPIError(t, w, http.StatusTooManyRequests, "rate limited")
					return
				}

				writeJSON(t, w, artistSearchJSON("jm123", "John Mayer"))
			})},
		}
		ctx := context.WithValue(context.Background(), interface{}(oauth2.HTTPClient), httpClient)

		client, err := New(ctx, "client-id", "client-secret")
		if err != nil {
			t.Fatalf("expected constructor to succeed, got error: %v", err)
		}

		artist, err := client.SearchArtistByName(ctx, "John Mayer")
		if err != nil {
			t.Fatalf("SearchArtistByName failed: %v", err)
		}
		if artist.Name != "John Mayer" {
			t.Fatalf("artist.Name = %q, want John Mayer", artist.Name)
		}
		if searchCallCount != 2 {
			t.Fatalf("searchCallCount = %d, want 2", searchCallCount)
		}
	})
}
