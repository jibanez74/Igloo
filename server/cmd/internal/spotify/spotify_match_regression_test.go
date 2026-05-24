package spotify

import (
	"context"
	"net/http"
	"strings"
	"testing"

	spotifylib "github.com/zmb3/spotify/v2"
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
			writeJSON(t, w, artistSearchItemsJSON(
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
			writeJSON(t, w, artistSearchItemsJSON(
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
			writeJSON(t, w, artistSearchItemsJSON(
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
				writeJSON(t, w, albumSearchItemsJSON(
					albumItemJSON("wrong123", "Greatest Hits", "Artist A"),
					albumItemJSON("right123", "Greatest Hits", "Artist B"),
				))
				return
			}

			switch {
			case strings.HasSuffix(r.URL.Path, "/albums/right123"):
				writeJSON(t, w, fullAlbumJSON("right123", "Greatest Hits"))
			case strings.HasSuffix(r.URL.Path, "/albums/wrong123"):
				writeJSON(t, w, fullAlbumJSON("wrong123", "Greatest Hits"))
			default:
				http.NotFound(w, r)
			}
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
				writeJSON(t, w, albumSearchItemsJSON(
					albumItemJSON("answer123", "Love Yourself 結 'Answer'", "BTS"),
				))
				return
			}

			writeJSON(t, w, fullAlbumJSON("answer123", "Love Yourself 結 'Answer'"))
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
					writeJSON(t, w, emptyAlbumSearchJSON())
					return
				}

				writeJSON(t, w, albumSearchItemsJSON(
					albumItemJSON("meteora123", "Meteora", "Linkin Park"),
				))
				return
			}

			writeJSON(t, w, fullAlbumJSON("meteora123", "Meteora"))
		}))

		album, err := sc.SearchAndGetAlbumDetails(context.Background(), "Meteora (Deluxe Edition)", "LINKIN PARK")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if string(album.ID) != "meteora123" {
			t.Fatalf("album.ID = %q, want %q", album.ID, "meteora123")
		}
	})

	t.Run("selectBestAlbumMatch handles all-negative candidate scores without panicking", func(t *testing.T) {
		albums := []spotifylib.SimpleAlbum{
			{
				ID:   "bad123",
				Name: "Completely Different",
				Artists: []spotifylib.SimpleArtist{
					{Name: "Wrong Artist"},
				},
			},
			{
				ID:   "worse123",
				Name: "Nothing Related",
				Artists: []spotifylib.SimpleArtist{
					{Name: "Another Artist"},
				},
			},
		}

		album, info := selectBestAlbumMatch("Target Title", "Expected Artist", albums, "query", "album_field_search")
		if album != nil {
			t.Fatalf("album = %#v, want nil", album)
		}
		if info.Reason != "score_below_threshold" {
			t.Fatalf("reason = %q, want %q", info.Reason, "score_below_threshold")
		}
		if info.CandidateName != "Completely Different" {
			t.Fatalf("candidate = %q, want %q", info.CandidateName, "Completely Different")
		}
		if info.Score >= 0 {
			t.Fatalf("score = %d, want negative score", info.Score)
		}
	})
}
