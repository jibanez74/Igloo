package spotify

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	gocache "github.com/patrickmn/go-cache"
	spotifylib "github.com/zmb3/spotify/v2"
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

func newMockClient(handler http.Handler) *spotifyClient {
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

// writeSpotifyTokenResponse keeps its Content-Type: oauth2 parses a token
// response by that header, and the recorder would otherwise guess text/plain.
func writeSpotifyTokenResponse(w http.ResponseWriter, accessToken string) {
	w.Header().Set("Content-Type", "application/json")
	b, _ := json.Marshal(map[string]interface{}{
		"access_token": accessToken,
		"token_type":   "Bearer",
		"expires_in":   3600,
	})
	w.Write(b)
}

// assertMatchReason fails unless err is a MatchError carrying reason; the
// music scanner branches on these values, so tests pin them explicitly.
func assertMatchReason(t *testing.T, err error, reason string) *MatchError {
	t.Helper()
	if err == nil {
		t.Fatalf("expected a %s match error, got nil", reason)
	}
	matchErr, ok := AsMatchError(err)
	if !ok {
		t.Fatalf("expected MatchError, got %T: %v", err, err)
	}
	if matchErr.Info.Reason != reason {
		t.Fatalf("reason = %q, want %q (%v)", matchErr.Info.Reason, reason, err)
	}
	return matchErr
}

func artistItemJSON(id, name string) map[string]interface{} {
	return map[string]interface{}{
		"id":            id,
		"name":          name,
		"type":          "artist",
		"popularity":    82,
		"followers":     map[string]interface{}{"total": 5000000, "href": nil},
		"genres":        []string{"pop"},
		"images":        []map[string]interface{}{{"url": "https://example.com/artist.jpg", "height": 640, "width": 640}},
		"external_urls": map[string]interface{}{},
		"href":          "",
		"uri":           "",
	}
}

func artistSearchItemsJSON(items ...map[string]interface{}) interface{} {
	return map[string]interface{}{
		"artists": map[string]interface{}{
			"items": items,
			"total": len(items),
			"limit": len(items),
		},
	}
}

func artistSearchJSON(id, name string) interface{} {
	return artistSearchItemsJSON(artistItemJSON(id, name))
}

func albumItemJSON(id, name string) map[string]interface{} {
	return map[string]interface{}{
		"id":                     id,
		"name":                   name,
		"type":                   "album",
		"album_type":             "album",
		"release_date":           "2023-01-01",
		"release_date_precision": "day",
		"total_tracks":           12,
		"images":                 []interface{}{},
		"artists":                []interface{}{},
		"external_urls":          map[string]interface{}{},
		"href":                   "",
		"uri":                    "",
		"available_markets":      []interface{}{},
	}
}

func albumSearchItemsJSON(items ...map[string]interface{}) interface{} {
	return map[string]interface{}{
		"albums": map[string]interface{}{
			"items": items,
			"total": len(items),
			"limit": len(items),
		},
	}
}

func albumSearchJSON(id, name string) interface{} {
	return albumSearchItemsJSON(albumItemJSON(id, name))
}

func emptyAlbumSearchJSON() interface{} {
	return albumSearchItemsJSON()
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

// albumSearchThenDetails answers every album search with one candidate and
// the album details request with its full record. When queries is non-nil,
// each search's q parameter is appended to it in request order.
func albumSearchThenDetails(id, name string, queries *[]string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		isSearch := strings.HasSuffix(r.URL.Path, "/search")
		if !isSearch {
			writeJSON(w, fullAlbumJSON(id, name))
			return
		}
		if queries != nil {
			*queries = append(*queries, r.URL.Query().Get("q"))
		}
		writeJSON(w, albumSearchJSON(id, name))
	})
}

// albumFallbackThenDetails answers the first album search with no results,
// every later search with one candidate, and the details request with its
// full record, so the client takes the plain-text fallback path.
func albumFallbackThenDetails(id, name string, queries *[]string) http.Handler {
	return albumSearchSequence(respondJSON(emptyAlbumSearchJSON()), respondJSON(albumSearchJSON(id, name)), respondJSON(fullAlbumJSON(id, name)), queries)
}

// albumSearchSequence answers the first album search (the field-filter
// strategy) with first, every later search (the fallback) with later, and
// any other request with details, or 404 when details is nil. When queries
// is non-nil, each search's q parameter is appended to it in request order.
func albumSearchSequence(first, later, details func(http.ResponseWriter), queries *[]string) http.Handler {
	searches := 0
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		isSearch := strings.HasSuffix(r.URL.Path, "/search")
		if !isSearch {
			if details == nil {
				http.NotFound(w, r)
				return
			}
			details(w)
			return
		}
		searches++
		if queries != nil {
			*queries = append(*queries, r.URL.Query().Get("q"))
		}
		if searches == 1 {
			first(w)
			return
		}
		later(w)
	})
}

func respondJSON(v interface{}) func(http.ResponseWriter) {
	return func(w http.ResponseWriter) { writeJSON(w, v) }
}

func respondSpotifyAPIError(status int, message string) func(http.ResponseWriter) {
	return func(w http.ResponseWriter) { writeSpotifyAPIError(w, status, message) }
}
