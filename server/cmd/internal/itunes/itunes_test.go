package itunes

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func newTestClient(t *testing.T, handler http.HandlerFunc) *Client {
	t.Helper()
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)
	client := New()
	client.baseURL = server.URL
	client.spacing = 0
	return client
}

func TestLookupAlbumsReadsCollectionsAndSongs(t *testing.T) {
	var query string
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		query = r.URL.RawQuery
		fmt.Fprint(w, `{"resultCount":4,"results":[
			{"wrapperType":"collection","collectionType":"Album","collectionId":11,"collectionName":"Cry Pretty","artistName":"Carrie Underwood","trackCount":13,"artworkUrl100":"https://is1-ssl.mzstatic.com/image/thumb/x/100x100bb.jpg"},
			{"wrapperType":"track","kind":"song","collectionId":11,"trackId":101,"trackExplicitness":"notExplicit"},
			{"wrapperType":"track","kind":"song","collectionId":11,"trackId":102,"trackExplicitness":"explicit"},
			{"wrapperType":"track","kind":"song","collectionId":99,"trackId":901,"trackExplicitness":"explicit"}
		]}`)
	})

	albums, err := client.LookupAlbums(context.Background(), []int64{11, 12})
	if err != nil {
		t.Fatalf("LookupAlbums returned error: %v", err)
	}
	for _, part := range []string{"id=11%2C12", "entity=song", "limit=200", "country=us"} {
		if !strings.Contains(query, part) {
			t.Fatalf("query %q lacks %q", query, part)
		}
	}
	if len(albums) != 1 {
		t.Fatalf("albums = %+v, want only the listed collection", albums)
	}
	album := albums[11]
	if album.TrackCount != 13 || album.Type() != "album" || album.CoverURL() != "https://is1-ssl.mzstatic.com/image/thumb/x/640x640bb.jpg" {
		t.Fatalf("album = %+v, type %q, cover %q", album, album.Type(), album.CoverURL())
	}
	want := map[int64]Track{101: {Explicit: false}, 102: {Explicit: true}}
	if len(album.Tracks) != len(want) || album.Tracks[101] != want[101] || album.Tracks[102] != want[102] {
		t.Fatalf("tracks = %+v, want %+v", album.Tracks, want)
	}
}

func TestLookupAlbumsErrors(t *testing.T) {
	cases := map[string]http.HandlerFunc{
		"throttled": func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusForbidden) },
		"outage":    func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusServiceUnavailable) },
		"bad json":  func(w http.ResponseWriter, _ *http.Request) { fmt.Fprint(w, `{"results":`) },
	}
	for name, handler := range cases {
		t.Run(name, func(t *testing.T) {
			_, err := newTestClient(t, handler).LookupAlbums(context.Background(), []int64{1})
			if err == nil {
				t.Fatal("LookupAlbums did not error")
			}
			fatal, transient := ProviderFailure(err)
			if fatal || !transient {
				t.Fatalf("ProviderFailure = (%v, %v), want transient only", fatal, transient)
			}
		})
	}
}

func TestLookupAlbumsBoundsItsRequest(t *testing.T) {
	requests := 0
	client := newTestClient(t, func(w http.ResponseWriter, _ *http.Request) {
		requests++
		fmt.Fprint(w, `{"results":[]}`)
	})
	albums, err := client.LookupAlbums(context.Background(), nil)
	if err != nil || len(albums) != 0 {
		t.Fatalf("no ids = %+v, %v; want an empty result", albums, err)
	}
	_, err = client.LookupAlbums(context.Background(), make([]int64, MaxLookupIDs+1))
	if err == nil {
		t.Fatal("too many ids did not error")
	}
	if requests != 0 {
		t.Fatalf("requests = %d, want none", requests)
	}
}

func TestLookupAlbumsSpacesRequests(t *testing.T) {
	client := newTestClient(t, func(w http.ResponseWriter, _ *http.Request) { fmt.Fprint(w, `{"results":[]}`) })
	client.spacing = 50 * time.Millisecond
	start := time.Now()
	for range 2 {
		_, err := client.LookupAlbums(context.Background(), []int64{1})
		if err != nil {
			t.Fatalf("LookupAlbums returned error: %v", err)
		}
	}
	elapsed := time.Since(start)
	if elapsed < client.spacing {
		t.Fatalf("two lookups took %v, want at least %v apart", elapsed, client.spacing)
	}

	client.spacing = time.Hour
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := client.LookupAlbums(ctx, []int64{1})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("a cancelled wait returned %v, want context.Canceled", err)
	}
}

func TestAlbumTypeAndLinks(t *testing.T) {
	cases := []struct {
		album Album
		want  string
	}{
		{Album{Name: "Cry Pretty", ArtistName: "Carrie Underwood"}, "album"},
		{Album{Name: "Zombie - Single", ArtistName: "Bad Wolves"}, "single"},
		{Album{Name: "Acoustic - EP", ArtistName: "Various Artists"}, "single"},
		{Album{Name: "2011 Grammy Nominees", ArtistName: "Various Artists"}, "compilation"},
	}
	for _, tc := range cases {
		got := tc.album.Type()
		if got != tc.want {
			t.Errorf("%q by %q: type %q, want %q", tc.album.Name, tc.album.ArtistName, got, tc.want)
		}
	}
	got := AlbumURL(1432044881)
	if got != "https://music.apple.com/us/album/1432044881" {
		t.Fatalf("AlbumURL = %q", got)
	}
}
