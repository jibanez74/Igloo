package music

import (
	"context"
	"fmt"
	"path/filepath"
	"slices"
	"testing"

	"igloo/cmd/internal/database"
	"igloo/cmd/internal/ffprobe"
	"igloo/cmd/internal/scanner"
	spotifyapi "igloo/cmd/internal/spotify"
)

func TestProcessMusicBatchSplitsCompoundArtistsIntoTrackMusicians(t *testing.T) {
	app := setupMusicScanner(t)
	defer app.db.Close()

	trackPath := filepath.Join(t.TempDir(), "Compound Artists.m4a")
	app.ffprobe = newMusicScannerFfprobeByPath(map[string]*ffprobe.FfprobeResult{
		trackPath: testMusicMetadataWithTags(ffprobe.FormatTags{
			Title:  "Compound Artists",
			Artist: "Artist One & Artist Two, Artist One",
			Album:  "Compound Album",
			Genre:  "Indie",
		}),
	})

	scanned, skipped, errCount := app.processMusicBatchForTest(context.Background(), []scanner.ScanFile{
		{Path: trackPath, Ext: "m4a", Size: 5},
	})
	if scanned != 1 || skipped != 0 || errCount != 0 {
		t.Fatalf("scan result scanned=%d skipped=%d errors=%d, want 1/0/0", scanned, skipped, errCount)
	}

	if got := countScannerRows(t, app.db, "SELECT COUNT(*) FROM musicians WHERE name IN (?, ?)", "Artist One", "Artist Two"); got != 2 {
		t.Fatalf("split musician count = %d, want 2", got)
	}

	var primaryArtist string
	err := app.db.QueryRow(`
		SELECT m.name
		FROM tracks AS t
		INNER JOIN musicians AS m ON m.id = t.musician_id
		WHERE t.file_path = ?
	`, trackPath).Scan(&primaryArtist)
	if err != nil {
		t.Fatalf("get primary artist: %v", err)
	}
	if primaryArtist != "Artist One" {
		t.Fatalf("primary artist = %q, want Artist One", primaryArtist)
	}

	if got := countScannerRows(t, app.db, `
		SELECT COUNT(*)
		FROM track_musicians AS tm
		INNER JOIN tracks AS t ON t.id = tm.track_id
		INNER JOIN musicians AS m ON m.id = tm.musician_id
		WHERE t.file_path = ? AND m.name IN (?, ?)
	`, trackPath, "Artist One", "Artist Two"); got != 2 {
		t.Fatalf("track_musicians split artist count = %d, want 2", got)
	}

	if got := countScannerRows(t, app.db, `
		SELECT COUNT(*)
		FROM musician_albums AS ma
		INNER JOIN musicians AS m ON m.id = ma.musician_id
		INNER JOIN albums AS a ON a.id = ma.album_id
		WHERE a.title = ? AND m.name IN (?, ?)
	`, "Compound Album", "Artist One", "Artist Two"); got != 2 {
		t.Fatalf("musician_albums split artist count = %d, want 2", got)
	}
}

func TestSplitCompoundArtistCreditsPreservesSuffixes(t *testing.T) {
	credits := parseCompoundArtistCredits("Anthony Ramos, Okieriete Onaodowan, Daveed Diggs, Lin-Manuel Miranda & Leslie Odom, Jr.")
	want := []string{
		"Anthony Ramos",
		"Okieriete Onaodowan",
		"Daveed Diggs",
		"Lin-Manuel Miranda",
		"Leslie Odom, Jr.",
	}
	if !slices.Equal(credits.parts, want) {
		t.Fatalf("parts = %#v, want %#v", credits.parts, want)
	}
	if !shouldSplitCompoundArtistCreditsLocally(credits) {
		t.Fatal("expected Hamilton-style credits to split locally")
	}

	credits = parseCompoundArtistCredits("Earth, Wind & Fire")
	if shouldSplitCompoundArtistCreditsLocally(credits) {
		t.Fatal("expected single-word comma/ampersand band name to stay combined locally")
	}
}

func TestProcessMusicBatchKeepsAmpersandOnlyArtistCombinedOffline(t *testing.T) {
	app := setupMusicScanner(t)
	defer app.db.Close()

	trackPath := filepath.Join(t.TempDir(), "Ampersand Artist.m4a")
	app.ffprobe = newMusicScannerFfprobeByPath(map[string]*ffprobe.FfprobeResult{
		trackPath: testMusicMetadataWithTags(ffprobe.FormatTags{
			Title:  "Ampersand Artist",
			Artist: "Brooks & Dunn",
		}),
	})

	scanned, skipped, errCount := app.processMusicBatchForTest(context.Background(), []scanner.ScanFile{
		{Path: trackPath, Ext: "m4a", Size: 5},
	})
	if scanned != 1 || skipped != 0 || errCount != 0 {
		t.Fatalf("scan result scanned=%d skipped=%d errors=%d, want 1/0/0", scanned, skipped, errCount)
	}

	if got := countScannerRows(t, app.db, "SELECT COUNT(*) FROM musicians WHERE name = ?", "Brooks & Dunn"); got != 1 {
		t.Fatalf("combined musician count = %d, want 1", got)
	}
	if got := countScannerRows(t, app.db, "SELECT COUNT(*) FROM musicians WHERE name IN (?, ?)", "Brooks", "Dunn"); got != 0 {
		t.Fatalf("split musician count = %d, want 0", got)
	}
	if got := countScannerRows(t, app.db, `
		SELECT COUNT(*)
		FROM track_musicians AS tm
		INNER JOIN tracks AS t ON t.id = tm.track_id
		INNER JOIN musicians AS m ON m.id = tm.musician_id
		WHERE t.file_path = ? AND m.name = ?
	`, trackPath, "Brooks & Dunn"); got != 1 {
		t.Fatalf("combined track_musicians count = %d, want 1", got)
	}
}

func TestProcessMusicBatchSplitsAmpersandArtistAfterSpotifyNoMatch(t *testing.T) {
	app := setupMusicScanner(t)
	defer app.db.Close()

	trackPath := filepath.Join(t.TempDir(), "Spotify Split Artist.m4a")
	app.ffprobe = newMusicScannerFfprobeByPath(map[string]*ffprobe.FfprobeResult{
		trackPath: testMusicMetadataWithTags(ffprobe.FormatTags{
			Title:  "Spotify Split Artist",
			Artist: "Artist One & Artist Two",
		}),
	})
	app.spotify = &musicScannerSpotifyStub{
		artistErr: &spotifyapi.MatchError{
			Info: spotifyapi.MatchDebugInfo{
				Lookup: "artist",
				Input:  "Artist One & Artist Two",
				Reason: "no_results",
			},
		},
	}

	scanned, skipped, errCount := app.processMusicBatchForTest(context.Background(), []scanner.ScanFile{
		{Path: trackPath, Ext: "m4a", Size: 5},
	})
	if scanned != 1 || skipped != 0 || errCount != 0 {
		t.Fatalf("scan result scanned=%d skipped=%d errors=%d, want 1/0/0", scanned, skipped, errCount)
	}

	if got := countScannerRows(t, app.db, "SELECT COUNT(*) FROM musicians WHERE name = ?", "Artist One & Artist Two"); got != 0 {
		t.Fatalf("combined musician count = %d, want 0", got)
	}
	if got := countScannerRows(t, app.db, "SELECT COUNT(*) FROM musicians WHERE name IN (?, ?)", "Artist One", "Artist Two"); got != 2 {
		t.Fatalf("split musician count = %d, want 2", got)
	}

	var primaryArtist string
	err := app.db.QueryRow(`
		SELECT m.name
		FROM tracks AS t
		INNER JOIN musicians AS m ON m.id = t.musician_id
		WHERE t.file_path = ?
	`, trackPath).Scan(&primaryArtist)
	if err != nil {
		t.Fatalf("get primary artist: %v", err)
	}
	if primaryArtist != "Artist One" {
		t.Fatalf("primary artist = %q, want Artist One", primaryArtist)
	}

	if got := countScannerRows(t, app.db, `
		SELECT COUNT(*)
		FROM track_musicians AS tm
		INNER JOIN tracks AS t ON t.id = tm.track_id
		INNER JOIN musicians AS m ON m.id = tm.musician_id
		WHERE t.file_path = ? AND m.name IN (?, ?)
	`, trackPath, "Artist One", "Artist Two"); got != 2 {
		t.Fatalf("track_musicians split artist count = %d, want 2", got)
	}
}

func TestProcessMusicBatchRemovesStaleTrackMusiciansOnRescan(t *testing.T) {
	app := setupMusicScanner(t)
	defer app.db.Close()

	trackPath := filepath.Join(t.TempDir(), "Changed Artist.m4a")
	ffprobeStub := newMusicScannerFfprobeByPath(map[string]*ffprobe.FfprobeResult{
		trackPath: testMusicMetadataWithTags(ffprobe.FormatTags{
			Title:  "Changed Artist",
			Artist: "Artist One & Artist Two, Artist One",
		}),
	})
	app.ffprobe = ffprobeStub

	scanned, skipped, errCount := app.processMusicBatchForTest(context.Background(), []scanner.ScanFile{
		{Path: trackPath, Ext: "m4a", Size: 5},
	})
	if scanned != 1 || skipped != 0 || errCount != 0 {
		t.Fatalf("first scan result scanned=%d skipped=%d errors=%d, want 1/0/0", scanned, skipped, errCount)
	}

	ffprobeStub.results[trackPath] = testMusicMetadataWithTags(ffprobe.FormatTags{
		Title:  "Changed Artist",
		Artist: "Solo Artist",
	})

	scanned, skipped, errCount = app.processMusicBatchForTest(context.Background(), []scanner.ScanFile{
		{Path: trackPath, Ext: "m4a", Size: 8},
	})
	if scanned != 1 || skipped != 0 || errCount != 0 {
		t.Fatalf("second scan result scanned=%d skipped=%d errors=%d, want 1/0/0", scanned, skipped, errCount)
	}

	if got := countScannerRows(t, app.db, `
		SELECT COUNT(*)
		FROM track_musicians AS tm
		INNER JOIN tracks AS t ON t.id = tm.track_id
		INNER JOIN musicians AS m ON m.id = tm.musician_id
		WHERE t.file_path = ? AND m.name = ?
	`, trackPath, "Solo Artist"); got != 1 {
		t.Fatalf("solo track_musicians count = %d, want 1", got)
	}
	if got := countScannerRows(t, app.db, `
		SELECT COUNT(*)
		FROM track_musicians AS tm
		INNER JOIN tracks AS t ON t.id = tm.track_id
		INNER JOIN musicians AS m ON m.id = tm.musician_id
		WHERE t.file_path = ? AND m.name IN (?, ?)
	`, trackPath, "Artist One", "Artist Two"); got != 0 {
		t.Fatalf("stale split track_musicians count = %d, want 0", got)
	}
}

func TestRepeatedCompoundCreditsFromPersistedMiss(t *testing.T) {
	s := setupMusicScanner(t)
	defer s.db.Close()
	ctx := context.Background()
	combined := "One & Two"
	musician, err := s.queries.UpsertMusician(ctx, database.UpsertMusicianParams{Name: combined, SortName: combined})
	if err != nil {
		t.Fatal(err)
	}
	err = s.queries.SaveMusicArtistIdentity(ctx, database.SaveMusicArtistIdentityParams{IdentityKey: scanner.NormalizedScanCacheKey(musician.Name), MusicianID: musician.ID})
	if err != nil {
		t.Fatal(err)
	}

	_, err = s.db.Exec("INSERT INTO music_spotify_matches(entity_type, entity_id, status, reason) VALUES ('musician', ?, 'unmatched', 'no_results')", musician.ID)
	if err != nil {
		t.Fatal(err)
	}
	s.ffprobe = &countingMusicScannerFfprobe{result: testMusicMetadataWithTags(ffprobe.FormatTags{Title: "Track", Artist: combined})}
	scan := newMusicScanContext(nil)
	for i := 0; i < 3; i++ {
		file := scanner.ScanFile{Path: fmt.Sprintf("/music/%d.m4a", i), Ext: "m4a", Size: 1}
		scanned, _, failures := s.processMusicBatch(ctx, scan, []scanner.ScanFile{file})
		if scanned != 1 || failures != 0 {
			t.Fatalf("track %d: scanned=%d errors=%d", i, scanned, failures)
		}
		count := countScannerRows(t, s.db, "SELECT count(*) FROM track_musicians tm JOIN musicians m ON m.id = tm.musician_id JOIN tracks t ON t.id = tm.track_id WHERE t.file_path = ? AND m.name IN ('One', 'Two')", file.Path)
		if count != 2 {
			t.Fatalf("track %d has %d split credits", i, count)
		}
	}
}
