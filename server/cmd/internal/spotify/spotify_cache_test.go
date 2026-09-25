package spotify

import (
	"context"
	"net/http"
	"strings"
	"testing"
)

func TestClearAllCachesForcesFreshLookups(t *testing.T) {
	artistSearches := 0
	albumSearches := 0
	sc := newMockClient(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case !strings.HasSuffix(r.URL.Path, "/search"):
			writeJSON(w, fullAlbumJSON("albumClear123", "Abbey Road"))
		case r.URL.Query().Get("type") == "artist":
			artistSearches++
			writeJSON(w, artistSearchJSON("jm123", "John Mayer"))
		default:
			albumSearches++
			writeJSON(w, albumSearchJSON("albumClear123", "Abbey Road"))
		}
	}))
	ctx := context.Background()

	lookupBoth := func() {
		t.Helper()
		_, err := sc.SearchArtistByName(ctx, "John Mayer")
		if err != nil {
			t.Fatalf("artist lookup failed: %v", err)
		}
		_, err = sc.SearchAndGetAlbumDetails(ctx, "Abbey Road", "The Beatles")
		if err != nil {
			t.Fatalf("album lookup failed: %v", err)
		}
	}

	lookupBoth()
	lookupBoth()
	if artistSearches != 1 || albumSearches != 1 {
		t.Fatalf("searches before clear = artist %d, album %d, want one each", artistSearches, albumSearches)
	}

	sc.ClearAllCaches()

	lookupBoth()
	if artistSearches != 2 || albumSearches != 2 {
		t.Fatalf("searches after clear = artist %d, album %d, want two each", artistSearches, albumSearches)
	}
}
