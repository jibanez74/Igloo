//go:build integration

package itunes

import (
	"context"
	"testing"
)

// These tests call the live iTunes Lookup API, which takes no credentials;
// run them with make test-itunes-integration.

func TestIntegrationLookupAlbumsResolvesListedAndDelistedIDs(t *testing.T) {
	const cryPretty, lostHighway = 1432044881, 927220868
	albums, err := New().LookupAlbums(context.Background(), []int64{cryPretty, lostHighway})
	if err != nil {
		t.Fatalf("LookupAlbums: %v", err)
	}
	album, listed := albums[cryPretty]
	if !listed {
		t.Fatalf("Cry Pretty (%d) is not listed: %+v", cryPretty, albums)
	}
	if album.Name != "Cry Pretty" || album.Type() != "album" || album.TrackCount != 13 {
		t.Fatalf("Cry Pretty = %+v, type %q", album, album.Type())
	}
	if len(album.Tracks) != 13 {
		t.Fatalf("Cry Pretty lists %d songs, want 13", len(album.Tracks))
	}
	_, title := album.Tracks[1432044890]
	if !title {
		t.Fatalf("Cry Pretty lacks its title track: %+v", album.Tracks)
	}
	_, delisted := albums[lostHighway]
	if delisted {
		t.Fatalf("the delisted Lost Highway reissue (%d) came back", lostHighway)
	}
}

// The API returns 50 songs per collection unless asked for more; a long album
// shows whether the request still raises that.
func TestIntegrationLookupAlbumsReturnsMoreThanFiftySongs(t *testing.T) {
	const columbiaYears = 404463399
	albums, err := New().LookupAlbums(context.Background(), []int64{columbiaYears})
	if err != nil {
		t.Fatalf("LookupAlbums: %v", err)
	}
	album := albums[columbiaYears]
	if len(album.Tracks) != albumTrackLimit {
		t.Fatalf("The Columbia Years lists %d of %d songs, want the %d-song cap", len(album.Tracks), album.TrackCount, albumTrackLimit)
	}
}
