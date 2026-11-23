package music

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"igloo/cmd/internal/database"
)

func TestUnknownAlbumTypeIsStoredAsUnknown(t *testing.T) {
	fixtureDir := t.TempDir()
	s := setupMusicScanner(t)
	album := testAlbum()
	album.AlbumType = "ep"
	s.spotify = &musicScannerSpotifyStub{artist: testArtist(), album: album}
	scanTaggedTrack(t, s, newMusicScanContext(nil), fixtureDir+"/01.m4a", 1, albumTags("Opening Song", "1"))

	var albumType sql.NullString
	err := s.tx.DB.QueryRow("SELECT album_type FROM albums WHERE spotify_id = 'album'").Scan(&albumType)
	if err != nil {
		t.Fatal(err)
	}
	if albumType.Valid {
		t.Fatalf("album_type = %#v, want NULL for a type the schema does not know", albumType)
	}
}

// One scan counts a failure once, however often the entity's cached failure is
// saved again (a later file, a merge clearing the id caches, an adoption per
// album); the next scan counts again and starts the wait.
func TestSaveMusicMatchCountsOneFailurePerScan(t *testing.T) {
	s := setupMusicScanner(t)
	ctx := context.Background()
	musician, err := s.queries.UpsertMusician(ctx, database.UpsertMusicianParams{Name: "Flaky", SortName: "Flaky"})
	if err != nil {
		t.Fatal(err)
	}
	failed := &resolvedMatch{status: musicStatusFailed}
	read := func() (attempts int64, retryAfter sql.NullInt64) {
		t.Helper()
		err := s.tx.DB.QueryRow(`SELECT attempts, retry_after FROM music_provider_matches WHERE entity_type='musician' AND entity_id=?`, musician.ID).
			Scan(&attempts, &retryAfter)
		if err != nil {
			t.Fatal(err)
		}
		return attempts, retryAfter
	}

	first := newMusicScanContext(nil)
	for range 3 {
		err = saveMusicMatch(ctx, s.queries, first, musicProviderSpotify, musicEntityMusician, musician.ID, failed)
		if err != nil {
			t.Fatal(err)
		}
	}
	if attempts, retryAfter := read(); attempts != 1 || retryAfter.Valid {
		t.Fatalf("one scan's repeated saves: attempts=%d retry_after=%v, want 1 and none", attempts, retryAfter)
	}

	second := newMusicScanContext(nil)
	second.startedAt = time.Now().Add(2 * time.Second)
	for range 2 {
		err = saveMusicMatch(ctx, s.queries, second, musicProviderSpotify, musicEntityMusician, musician.ID, failed)
		if err != nil {
			t.Fatal(err)
		}
	}
	attempts, retryAfter := read()
	if attempts != 2 || !retryAfter.Valid {
		t.Fatalf("the next scan's failure: attempts=%d retry_after=%v, want 2 and a wait", attempts, retryAfter)
	}

	err = saveMusicMatch(ctx, s.queries, second, musicProviderSpotify, musicEntityMusician, musician.ID, &resolvedMatch{status: musicStatusMatched})
	if err != nil {
		t.Fatal(err)
	}
	if attempts, retryAfter := read(); attempts != 0 || retryAfter.Valid {
		t.Fatalf("a match: attempts=%d retry_after=%v, want the count cleared", attempts, retryAfter)
	}
}
