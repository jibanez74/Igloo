package music

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"

	"igloo/cmd/internal/database"
	"igloo/cmd/internal/ffprobe"
	"igloo/cmd/internal/scanner"

	spotifylib "github.com/zmb3/spotify/v2"
)

func TestProcessMusicBatchInsertsTrackAndSkipsExistingPathSize(t *testing.T) {
	app := setupMusicScanner(t)
	defer app.db.Close()

	ffprobeStub := &countingMusicScannerFfprobe{result: testMusicMetadata()}
	app.ffprobe = ffprobeStub

	file := scanner.ScanFile{
		Path: filepath.Join(t.TempDir(), "Test Track.m4a"),
		Ext:  "m4a",
		Size: 5,
	}

	scanned, skipped, errCount := app.processMusicBatchForTest(context.Background(), []scanner.ScanFile{file})
	if scanned != 1 || skipped != 0 || errCount != 0 {
		t.Fatalf("first scan result scanned=%d skipped=%d errors=%d, want 1/0/0", scanned, skipped, errCount)
	}

	var trackCount int
	err := app.db.QueryRow("SELECT COUNT(*) FROM tracks WHERE file_path = ? AND size = ?", file.Path, file.Size).Scan(&trackCount)
	if err != nil {
		t.Fatalf("count tracks: %v", err)
	}
	if trackCount != 1 {
		t.Fatalf("track count = %d, want 1", trackCount)
	}
	if ffprobeStub.calls != 1 {
		t.Fatalf("ffprobe calls = %d, want 1", ffprobeStub.calls)
	}

	scanned, skipped, errCount = app.processMusicBatchForTest(context.Background(), []scanner.ScanFile{file})
	if scanned != 0 || skipped != 1 || errCount != 0 {
		t.Fatalf("second scan result scanned=%d skipped=%d errors=%d, want 0/1/0", scanned, skipped, errCount)
	}
	if ffprobeStub.calls != 1 {
		t.Fatalf("ffprobe calls after skip = %d, want 1", ffprobeStub.calls)
	}

	changedFile := file
	changedFile.Size = 6
	scanned, skipped, errCount = app.processMusicBatchForTest(context.Background(), []scanner.ScanFile{changedFile})
	if scanned != 1 || skipped != 0 || errCount != 0 {
		t.Fatalf("changed size scan result scanned=%d skipped=%d errors=%d, want 1/0/0", scanned, skipped, errCount)
	}
	if ffprobeStub.calls != 2 {
		t.Fatalf("ffprobe calls after changed size = %d, want 2", ffprobeStub.calls)
	}
}

func TestProcessMusicBatchSkipsBadTrackAndCommitsGoodTrack(t *testing.T) {
	app := setupMusicScanner(t)
	defer app.db.Close()

	dir := t.TempDir()
	badPath := filepath.Join(dir, "Bad Track.m4a")
	goodPath := filepath.Join(dir, "Good Track.m4a")
	ffprobeStub := &failingPathMusicScannerFfprobe{
		result:      testMusicMetadata(),
		failingPath: badPath,
	}
	app.ffprobe = ffprobeStub

	files := []scanner.ScanFile{
		{
			Path: badPath,
			Ext:  "m4a",
			Size: 5,
		},
		{
			Path: goodPath,
			Ext:  "m4a",
			Size: 6,
		},
	}

	scanned, skipped, errCount := app.processMusicBatchForTest(context.Background(), files)
	if scanned != 1 || skipped != 0 || errCount != 1 {
		t.Fatalf("scan result scanned=%d skipped=%d errors=%d, want 1/0/1", scanned, skipped, errCount)
	}

	var goodCount int
	err := app.db.QueryRow("SELECT COUNT(*) FROM tracks WHERE file_path = ?", goodPath).Scan(&goodCount)
	if err != nil {
		t.Fatalf("count good track: %v", err)
	}
	if goodCount != 1 {
		t.Fatalf("good track count = %d, want 1", goodCount)
	}

	var badCount int
	err = app.db.QueryRow("SELECT COUNT(*) FROM tracks WHERE file_path = ?", badPath).Scan(&badCount)
	if err != nil {
		t.Fatalf("count bad track: %v", err)
	}
	if badCount != 0 {
		t.Fatalf("bad track count = %d, want 0", badCount)
	}
	if ffprobeStub.calls != 2 {
		t.Fatalf("ffprobe calls = %d, want 2", ffprobeStub.calls)
	}
}

func TestProcessMusicBatchUsesScanLocalEntityCaches(t *testing.T) {
	app := setupMusicScanner(t)
	defer app.db.Close()

	spotifyStub := &musicScannerSpotifyStub{
		artistErr: errors.New("artist unavailable"),
		albumErr:  errors.New("album unavailable"),
	}
	app.ffprobe = &countingMusicScannerFfprobe{result: testMusicMetadata()}
	app.spotify = spotifyStub

	dir := t.TempDir()
	files := []scanner.ScanFile{
		{
			Path: filepath.Join(dir, "Track One.m4a"),
			Ext:  "m4a",
			Size: 5,
		},
		{
			Path: filepath.Join(dir, "Track Two.m4a"),
			Ext:  "m4a",
			Size: 6,
		},
	}

	scanned, skipped, errCount := app.processMusicBatchForTest(context.Background(), files)
	if scanned != 2 || skipped != 0 || errCount != 0 {
		t.Fatalf("scan result scanned=%d skipped=%d errors=%d, want 2/0/0", scanned, skipped, errCount)
	}
	if spotifyStub.artistCalls != 1 {
		t.Fatalf("artist calls = %d, want 1", spotifyStub.artistCalls)
	}
	if spotifyStub.albumCalls != 1 {
		t.Fatalf("album calls = %d, want 1", spotifyStub.albumCalls)
	}
}

func TestProcessMusicBatchPersistsGenresAndRelationships(t *testing.T) {
	app := setupMusicScanner(t)
	defer app.db.Close()

	dir := t.TempDir()
	firstPath := filepath.Join(dir, "First.m4a")
	secondPath := filepath.Join(dir, "Second.m4a")
	ffprobeStub := newMusicScannerFfprobeByPath(map[string]*ffprobe.FfprobeResult{
		firstPath: testMusicMetadataWithTags(ffprobe.FormatTags{
			Title:       "First",
			Artist:      "Track Artist",
			AlbumArtist: "Album Artist",
			Album:       "Shared Album",
			Genre:       "Synth Pop",
			Track:       "1/2",
		}),
		secondPath: testMusicMetadataWithTags(ffprobe.FormatTags{
			Title:       "Second",
			Artist:      "Track Artist",
			AlbumArtist: "Album Artist",
			Album:       "Shared Album",
			Genre:       "Synth Pop",
			Track:       "2/2",
		}),
	})
	app.ffprobe = ffprobeStub

	files := []scanner.ScanFile{
		{Path: firstPath, Ext: "m4a", Size: 5},
		{Path: secondPath, Ext: "m4a", Size: 6},
	}
	scanned, skipped, errCount := app.processMusicBatchForTest(context.Background(), files)
	if scanned != 2 || skipped != 0 || errCount != 0 {
		t.Fatalf("scan result scanned=%d skipped=%d errors=%d, want 2/0/0", scanned, skipped, errCount)
	}

	if got := countScannerRows(t, app.db, "SELECT COUNT(*) FROM tracks"); got != 2 {
		t.Fatalf("track count = %d, want 2", got)
	}
	if got := countScannerRows(t, app.db, "SELECT COUNT(*) FROM musicians WHERE name = ?", "Track Artist"); got != 1 {
		t.Fatalf("musician count = %d, want 1", got)
	}
	if got := countScannerRows(t, app.db, "SELECT COUNT(*) FROM albums WHERE title = ? AND musician = ?", "Shared Album", "Album Artist"); got != 1 {
		t.Fatalf("album count = %d, want 1", got)
	}
	if got := countScannerRows(t, app.db, "SELECT COUNT(*) FROM genres WHERE tag = ? AND genre_type = ?", "Synth Pop", "music"); got != 1 {
		t.Fatalf("genre count = %d, want 1", got)
	}
	if got := countScannerRows(t, app.db, `
		SELECT COUNT(*)
		FROM musician_albums AS ma
		INNER JOIN musicians AS m ON m.id = ma.musician_id
		INNER JOIN albums AS a ON a.id = ma.album_id
		WHERE m.name = ? AND a.title = ?
	`, "Track Artist", "Shared Album"); got != 1 {
		t.Fatalf("musician_albums count = %d, want 1", got)
	}
	if got := countScannerRows(t, app.db, "SELECT COUNT(*) FROM track_genres"); got != 2 {
		t.Fatalf("track_genres count = %d, want 2", got)
	}
	if got := countScannerRows(t, app.db, "SELECT COUNT(*) FROM musician_genres"); got != 1 {
		t.Fatalf("musician_genres count = %d, want 1", got)
	}
	if got := countScannerRows(t, app.db, "SELECT COUNT(*) FROM album_genres"); got != 1 {
		t.Fatalf("album_genres count = %d, want 1", got)
	}
}

func TestProcessMusicBatchUpdatesChangedTrackAndReplacesGenre(t *testing.T) {
	app := setupMusicScanner(t)
	defer app.db.Close()

	trackPath := filepath.Join(t.TempDir(), "Changing Genre.m4a")
	ffprobeStub := newMusicScannerFfprobeByPath(map[string]*ffprobe.FfprobeResult{
		trackPath: testMusicMetadataWithTags(ffprobe.FormatTags{
			Title:  "Original Title",
			Artist: "Genre Artist",
			Album:  "Genre Album",
			Genre:  "Rock",
		}),
	})
	app.ffprobe = ffprobeStub

	file := scanner.ScanFile{Path: trackPath, Ext: "m4a", Size: 5}
	scanned, skipped, errCount := app.processMusicBatchForTest(context.Background(), []scanner.ScanFile{file})
	if scanned != 1 || skipped != 0 || errCount != 0 {
		t.Fatalf("first scan result scanned=%d skipped=%d errors=%d, want 1/0/0", scanned, skipped, errCount)
	}

	ffprobeStub.results[trackPath] = testMusicMetadataWithTags(ffprobe.FormatTags{
		Title:  "Updated Title",
		Artist: "Genre Artist",
		Album:  "Genre Album",
		Genre:  "Jazz",
	})
	file.Size = 8

	scanned, skipped, errCount = app.processMusicBatchForTest(context.Background(), []scanner.ScanFile{file})
	if scanned != 1 || skipped != 0 || errCount != 0 {
		t.Fatalf("second scan result scanned=%d skipped=%d errors=%d, want 1/0/0", scanned, skipped, errCount)
	}

	var title string
	var size int64
	err := app.db.QueryRow("SELECT title, size FROM tracks WHERE file_path = ?", trackPath).Scan(&title, &size)
	if err != nil {
		t.Fatalf("get updated track: %v", err)
	}
	if title != "Updated Title" || size != 8 {
		t.Fatalf("updated track title/size = %q/%d, want Updated Title/8", title, size)
	}

	if got := countScannerRows(t, app.db, `
		SELECT COUNT(*)
		FROM track_genres AS tg
		INNER JOIN tracks AS t ON t.id = tg.track_id
		INNER JOIN genres AS g ON g.id = tg.genre_id
		WHERE t.file_path = ? AND g.tag = ?
	`, trackPath, "Jazz"); got != 1 {
		t.Fatalf("Jazz track genre count = %d, want 1", got)
	}
	if got := countScannerRows(t, app.db, `
		SELECT COUNT(*)
		FROM track_genres AS tg
		INNER JOIN tracks AS t ON t.id = tg.track_id
		INNER JOIN genres AS g ON g.id = tg.genre_id
		WHERE t.file_path = ? AND g.tag = ?
	`, trackPath, "Rock"); got != 0 {
		t.Fatalf("Rock track genre count = %d, want 0", got)
	}
}

func TestProcessMusicBatchClearsArtistAlbumAndJoinRowsWhenTagsRemoved(t *testing.T) {
	app := setupMusicScanner(t)
	defer app.db.Close()

	trackPath := filepath.Join(t.TempDir(), "Removed Tags.m4a")
	ffprobeStub := newMusicScannerFfprobeByPath(map[string]*ffprobe.FfprobeResult{
		trackPath: testMusicMetadataWithTags(ffprobe.FormatTags{
			Title:  "Removed Tags",
			Artist: "Tagged Artist",
			Album:  "Tagged Album",
			Genre:  "Tagged Genre",
		}),
	})
	app.ffprobe = ffprobeStub

	file := scanner.ScanFile{Path: trackPath, Ext: "m4a", Size: 5}
	scanned, skipped, errCount := app.processMusicBatchForTest(context.Background(), []scanner.ScanFile{file})
	if scanned != 1 || skipped != 0 || errCount != 0 {
		t.Fatalf("first scan result scanned=%d skipped=%d errors=%d, want 1/0/0", scanned, skipped, errCount)
	}

	if got := countScannerRows(t, app.db, `
		SELECT COUNT(*)
		FROM track_musicians AS tm
		INNER JOIN tracks AS t ON t.id = tm.track_id
		WHERE t.file_path = ?
	`, trackPath); got != 1 {
		t.Fatalf("initial track_musicians count = %d, want 1", got)
	}
	if got := countScannerRows(t, app.db, `
		SELECT COUNT(*)
		FROM track_genres AS tg
		INNER JOIN tracks AS t ON t.id = tg.track_id
		WHERE t.file_path = ?
	`, trackPath); got != 1 {
		t.Fatalf("initial track_genres count = %d, want 1", got)
	}

	ffprobeStub.results[trackPath] = testMusicMetadataWithTags(ffprobe.FormatTags{
		Title: "Removed Tags",
	})
	file.Size = 8

	scanned, skipped, errCount = app.processMusicBatchForTest(context.Background(), []scanner.ScanFile{file})
	if scanned != 1 || skipped != 0 || errCount != 0 {
		t.Fatalf("second scan result scanned=%d skipped=%d errors=%d, want 1/0/0", scanned, skipped, errCount)
	}

	var musicianID sql.NullInt64
	var albumID sql.NullInt64
	err := app.db.QueryRow("SELECT musician_id, album_id FROM tracks WHERE file_path = ?", trackPath).Scan(&musicianID, &albumID)
	if err != nil {
		t.Fatalf("get rescanned track relationships: %v", err)
	}
	if musicianID.Valid {
		t.Fatalf("track musician_id = %#v, want null after artist tag removal", musicianID)
	}
	if albumID.Valid {
		t.Fatalf("track album_id = %#v, want null after album tag removal", albumID)
	}

	if got := countScannerRows(t, app.db, `
		SELECT COUNT(*)
		FROM track_musicians AS tm
		INNER JOIN tracks AS t ON t.id = tm.track_id
		WHERE t.file_path = ?
	`, trackPath); got != 0 {
		t.Fatalf("track_musicians count after tag removal = %d, want 0", got)
	}
	if got := countScannerRows(t, app.db, `
		SELECT COUNT(*)
		FROM track_genres AS tg
		INNER JOIN tracks AS t ON t.id = tg.track_id
		WHERE t.file_path = ?
	`, trackPath); got != 0 {
		t.Fatalf("track_genres count after tag removal = %d, want 0", got)
	}
}

func TestProcessMusicBatchDoesNotMergeFailedPersistIntoScanContext(t *testing.T) {
	app := setupMusicScanner(t)
	defer app.db.Close()

	dir := t.TempDir()
	badPath := filepath.Join(dir, "Bad Cache Track.m4a")
	goodPath := filepath.Join(dir, "Good Cache Track.m4a")
	escapedBadPath := strings.ReplaceAll(badPath, "'", "''")
	_, err := app.db.Exec(fmt.Sprintf(`CREATE TRIGGER fail_bad_cache_track BEFORE INSERT ON tracks
		WHEN new.file_path = '%s'
		BEGIN
			SELECT RAISE(ABORT, 'forced track failure');
		END;`, escapedBadPath))
	if err != nil {
		t.Fatalf("create failing trigger: %v", err)
	}

	app.ffprobe = newMusicScannerFfprobeByPath(map[string]*ffprobe.FfprobeResult{
		badPath: testMusicMetadataWithTags(ffprobe.FormatTags{
			Title:  "Bad Cache Track",
			Artist: "Cache Artist",
			Album:  "Cache Album",
		}),
		goodPath: testMusicMetadataWithTags(ffprobe.FormatTags{
			Title:  "Good Cache Track",
			Artist: "Cache Artist",
			Album:  "Cache Album",
		}),
	})

	scanIndex, err := app.loadMusicScanIndex(context.Background())
	if err != nil {
		t.Fatalf("load scan index: %v", err)
	}
	scan := newMusicScanContext(scanIndex)

	files := []scanner.ScanFile{
		{Path: badPath, Ext: "m4a", Size: 5},
		{Path: goodPath, Ext: "m4a", Size: 6},
	}
	scanned, skipped, errCount := app.processMusicBatch(context.Background(), scan, files)
	if scanned != 1 || skipped != 0 || errCount != 1 {
		t.Fatalf("scan result scanned=%d skipped=%d errors=%d, want 1/0/1", scanned, skipped, errCount)
	}
	if _, ok := scan.trackIndex[filepath.Clean(badPath)]; ok {
		t.Fatal("bad track was merged into scan index after failed transaction")
	}
	if got := scan.trackIndex[filepath.Clean(goodPath)]; got != 6 {
		t.Fatalf("good track scan index size = %d, want 6", got)
	}
	if got := countScannerRows(t, app.db, "SELECT COUNT(*) FROM tracks WHERE file_path = ?", badPath); got != 0 {
		t.Fatalf("bad track count = %d, want 0", got)
	}
	if got := countScannerRows(t, app.db, "SELECT COUNT(*) FROM tracks WHERE file_path = ?", goodPath); got != 1 {
		t.Fatalf("good track count = %d, want 1", got)
	}
}

func TestPersistResolvedTrackInvalidatesOnlyAfterCommitBeforeMergingCaches(t *testing.T) {
	s := setupMusicScanner(t)
	defer s.db.Close()
	ctx := context.Background()
	scan := newMusicScanContext(nil)
	resolved := &resolvedTrack{
		params:    database.UpsertTrackParams{FilePath: "/music/track.m4a", FileName: "track.m4a", Title: "Track", Size: 4, Container: "m4a", MimeType: "audio/mp4"},
		musicians: []resolvedMusician{{name: "Artist", sortName: "Artist"}},
	}
	var invalidatedIDs []int64
	s.invalidateCommittedTrack = func(trackID int64) {
		invalidatedIDs = append(invalidatedIDs, trackID)
		var title string
		err := s.db.QueryRow("SELECT title FROM tracks WHERE id = ?", trackID).Scan(&title)
		if err != nil || title != "Track" {
			t.Errorf("callback did not observe committed track: title=%q, err=%v", title, err)
		}
		if scan.trackUnchanged(resolved.params.FilePath, resolved.params.Size) {
			t.Error("track index updated before invalidation")
		}
		if scan.musicianIDs.Has(scanner.NormalizedScanCacheKey("Artist")) {
			t.Error("entity caches merged before invalidation")
		}
		unlocked := s.scannerDBMu.TryLock()
		if unlocked {
			s.scannerDBMu.Unlock()
			t.Error("database mutex released before invalidation")
		}
	}

	_, err := s.db.Exec("CREATE TRIGGER fail_track BEFORE INSERT ON tracks BEGIN SELECT RAISE(ABORT, 'failed track'); END")
	if err != nil {
		t.Fatal(err)
	}
	_, err = s.persistResolvedTrack(ctx, scan, resolved)
	if err == nil || len(invalidatedIDs) != 0 {
		t.Fatalf("failed persist: err=%v, invalidations=%v", err, invalidatedIDs)
	}
	_, err = s.db.Exec("DROP TRIGGER fail_track")
	if err != nil {
		t.Fatal(err)
	}
	trackID, err := s.persistResolvedTrack(ctx, scan, resolved)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(invalidatedIDs, []int64{trackID}) {
		t.Fatalf("invalidations = %v, want [%d]", invalidatedIDs, trackID)
	}
	if !scan.trackUnchanged(resolved.params.FilePath, resolved.params.Size) {
		t.Error("committed track missing from scan index")
	}
	if !scan.musicianIDs.Has(scanner.NormalizedScanCacheKey("Artist")) {
		t.Error("committed musician missing from scan cache")
	}
}

func TestRelationshipFailuresRollBackAndRetry(t *testing.T) {
	cases := []struct {
		name, table, when string
		spotify           bool
	}{
		{name: "musician album", table: "musician_albums"},
		{name: "track musician", table: "track_musicians"},
		{name: "track genre", table: "track_genres"},
		{name: "musician genre", table: "musician_genres"},
		{name: "album genre", table: "album_genres"},
		{name: "Spotify musician genre", table: "musician_genres", spotify: true},
		{name: "Spotify album genre", table: "album_genres", spotify: true},
		{name: "Spotify genre", table: "genres", when: " WHEN NEW.tag = 'Spotify Genre'", spotify: true},
		{name: "Spotify bookkeeping", table: "music_spotify_matches", spotify: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			s := setupMusicScanner(t)
			defer s.db.Close()
			ctx := context.Background()
			file := scanner.ScanFile{Path: "/music/retry.m4a", Ext: "m4a", Size: 2}
			_, err := s.queries.UpsertTrack(ctx, database.UpsertTrackParams{FilePath: file.Path, FileName: "retry.m4a", Title: "Original", Size: 1, Container: "m4a", MimeType: "audio/mp4"})
			if err != nil {
				t.Fatal(err)
			}
			metadata := testMusicMetadata()
			metadata.Format.Tags.Genre = "Local Genre"
			s.ffprobe = &countingMusicScannerFfprobe{result: metadata}
			if tc.spotify {
				s.spotify = &musicScannerSpotifyStub{artist: &spotifylib.FullArtist{SimpleArtist: spotifylib.SimpleArtist{ID: "artist"}, Genres: []string{"Spotify Genre"}}, album: &spotifylib.FullAlbum{SimpleAlbum: spotifylib.SimpleAlbum{ID: "album"}, Genres: []string{"Spotify Genre"}}}
			}
			scan := newMusicScanContext(nil)
			invalidations := 0
			s.invalidateCommittedTrack = func(int64) { invalidations++ }
			_, err = s.db.Exec("CREATE TRIGGER fail_relationship BEFORE INSERT ON " + tc.table + tc.when + " BEGIN SELECT RAISE(ABORT, 'relationship failure'); END")
			if err != nil {
				t.Fatal(err)
			}
			scanned, _, failures := s.processMusicBatch(ctx, scan, []scanner.ScanFile{file})
			if scanned != 0 || failures != 1 || invalidations != 0 {
				t.Fatalf("scanned=%d errors=%d invalidations=%d", scanned, failures, invalidations)
			}
			for _, table := range []string{"musicians", "albums", "genres", "musician_albums", "track_musicians", "track_genres", "musician_genres", "album_genres", "music_spotify_matches"} {
				count := countScannerRows(t, s.db, "SELECT count(*) FROM "+table)
				if count != 0 {
					t.Fatalf("rollback left %d rows in %s", count, table)
				}
			}
			count := countScannerRows(t, s.db, "SELECT count(*) FROM tracks WHERE title = 'Original' AND size = 1")
			if count != 1 {
				t.Fatal("original track changed on rollback")
			}
			// External lookup outcomes survive rollback; database IDs never do.
			expected := newMusicScanContext(nil)
			expected.artistAttempts = scan.artistAttempts
			expected.albumAttempts = scan.albumAttempts
			expected.enrichmentCounts = scan.enrichmentCounts
			expected.artistAttemptsByID = scan.artistAttemptsByID
			expected.albumAttemptsByID = scan.albumAttemptsByID
			if !reflect.DeepEqual(scan, expected) {
				t.Fatalf("failed transaction published cache entries: %+v", scan)
			}
			logs := s.logger.(*capturedLogger)
			if len(logs.warnEntries) != 1 || !strings.Contains(fmt.Sprint(logs.warnEntries[0].args), file.Path) || !strings.Contains(fmt.Sprint(logs.warnEntries[0].args), "relationship failure") {
				t.Fatalf("failure logs: %+v", logs.warnEntries)
			}
			_, err = s.db.Exec("DROP TRIGGER fail_relationship")
			if err != nil {
				t.Fatal(err)
			}
			scanned, _, failures = s.processMusicBatch(ctx, scan, []scanner.ScanFile{file})
			if scanned != 1 || failures != 0 || invalidations != 1 {
				t.Fatalf("retry: scanned=%d errors=%d invalidations=%d", scanned, failures, invalidations)
			}
			count = countScannerRows(t, s.db, "SELECT count(*) FROM tracks WHERE title = 'Test Track' AND size = 2")
			if count != 1 {
				t.Fatal("retry failed to update track")
			}
		})
	}
}

func TestTrackGenreRestoredWithinScan(t *testing.T) {
	s := setupMusicScanner(t)
	defer s.db.Close()
	scan := newMusicScanContext(nil)
	metadata := testMusicMetadata()
	s.ffprobe = &countingMusicScannerFfprobe{result: metadata}
	for i, genre := range []string{"A", "B", "A"} {
		metadata.Format.Tags.Genre = genre
		scanned, _, failures := s.processMusicBatch(context.Background(), scan, []scanner.ScanFile{{Path: "/music/genre.m4a", Ext: "m4a", Size: int64(i + 1)}})
		if scanned != 1 || failures != 0 {
			t.Fatalf("genre %s: scanned=%d errors=%d", genre, scanned, failures)
		}
		var got string
		err := s.db.QueryRow("SELECT g.tag FROM track_genres tg JOIN genres g ON g.id = tg.genre_id").Scan(&got)
		if err != nil || got != genre {
			t.Fatalf("genre=%q err=%v, want %q", got, err, genre)
		}
	}
}
