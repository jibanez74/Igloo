package spotify

import (
	"context"
	"net/http"
	"strings"
	"testing"
)

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

func albumItemJSON(id, name string, artists ...string) map[string]interface{} {
	artistItems := make([]map[string]interface{}, 0, len(artists))
	for _, artist := range artists {
		artistItems = append(artistItems, map[string]interface{}{
			"id":            strings.ToLower(strings.ReplaceAll(artist, " ", "")),
			"name":          artist,
			"type":          "artist",
			"external_urls": map[string]interface{}{},
			"href":          "",
			"uri":           "",
		})
	}

	return map[string]interface{}{
		"id":                     id,
		"name":                   name,
		"type":                   "album",
		"album_type":             "album",
		"release_date":           "2023-01-01",
		"release_date_precision": "day",
		"total_tracks":           12,
		"images":                 []interface{}{},
		"artists":                artistItems,
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

func TestSearchArtistByName_Regressions(t *testing.T) {
	t.Run("accepts diacritic variants", func(t *testing.T) {
		sc := newMockClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			writeJSON(w, artistSearchItemsJSON(
				artistItemJSON("beyonce-smith", "Beyonce Smith"),
				artistItemJSON("beyonce", "Beyoncé"),
			))
		}))

		artist, err := sc.SearchArtistByName(context.Background(), "Beyonce")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if artist.Name != "Beyoncé" {
			t.Fatalf("artist.Name = %q, want %q", artist.Name, "Beyoncé")
		}
	})

	t.Run("accepts punctuation variants", func(t *testing.T) {
		sc := newMockClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			writeJSON(w, artistSearchItemsJSON(
				artistItemJSON("guns-tribute", "Guns Tribute"),
				artistItemJSON("guns", "Guns N' Roses"),
			))
		}))

		artist, err := sc.SearchArtistByName(context.Background(), "Guns N Roses")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if artist.Name != "Guns N' Roses" {
			t.Fatalf("artist.Name = %q, want %q", artist.Name, "Guns N' Roses")
		}
	})

	t.Run("accepts canonical duo name variants", func(t *testing.T) {
		sc := newMockClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			writeJSON(w, artistSearchItemsJSON(
				artistItemJSON("hall-oates", "Daryl Hall & John Oates"),
			))
		}))

		artist, err := sc.SearchArtistByName(context.Background(), "Hall & Oates")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if artist.Name != "Daryl Hall & John Oates" {
			t.Fatalf("artist.Name = %q, want %q", artist.Name, "Daryl Hall & John Oates")
		}
	})
}

func TestSearchAndGetAlbumDetails_Regressions(t *testing.T) {
	t.Run("prefers later candidate with matching artist", func(t *testing.T) {
		sc := newMockClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if strings.HasSuffix(r.URL.Path, "/search") {
				writeJSON(w, albumSearchItemsJSON(
					albumItemJSON("wrong123", "Greatest Hits", "Artist A"),
					albumItemJSON("right123", "Greatest Hits", "Artist B"),
				))
				return
			}

			writeJSON(w, fullAlbumJSON("right123", "Greatest Hits"))
		}))

		album, err := sc.SearchAndGetAlbumDetails(context.Background(), "Greatest Hits", "Artist B")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if string(album.ID) != "right123" {
			t.Fatalf("album.ID = %q, want %q", album.ID, "right123")
		}
	})

	t.Run("accepts unicode-bearing titles", func(t *testing.T) {
		sc := newMockClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if strings.HasSuffix(r.URL.Path, "/search") {
				writeJSON(w, albumSearchItemsJSON(
					albumItemJSON("answer123", "Love Yourself 結 'Answer'", "BTS"),
				))
				return
			}

			writeJSON(w, fullAlbumJSON("answer123", "Love Yourself 結 'Answer'"))
		}))

		album, err := sc.SearchAndGetAlbumDetails(context.Background(), "Love Yourself Answer", "BTS")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if string(album.ID) != "answer123" {
			t.Fatalf("album.ID = %q, want %q", album.ID, "answer123")
		}
	})

	t.Run("accepts edition family fallback results", func(t *testing.T) {
		searchCallCount := 0
		sc := newMockClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if strings.HasSuffix(r.URL.Path, "/search") {
				searchCallCount++
				if searchCallCount == 1 {
					writeJSON(w, emptyAlbumSearchJSON())
					return
				}

				writeJSON(w, albumSearchItemsJSON(
					albumItemJSON("meteora123", "Meteora", "Linkin Park"),
				))
				return
			}

			writeJSON(w, fullAlbumJSON("meteora123", "Meteora"))
		}))

		album, err := sc.SearchAndGetAlbumDetails(context.Background(), "Meteora (Deluxe Edition)", "LINKIN PARK")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if string(album.ID) != "meteora123" {
			t.Fatalf("album.ID = %q, want %q", album.ID, "meteora123")
		}
	})
}
