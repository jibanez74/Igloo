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
	"igloo/cmd/internal/scanner/scannertest"
	spotifyapi "igloo/cmd/internal/spotify"
)

func TestProcessMusicBatchSkipsBadTrackAndCommitsGoodTrack(t *testing.T) {
	app := setupMusicScanner(t)

	dir := t.TempDir()
	badPath := filepath.Join(dir, "Bad Track.m4a")
	goodPath := filepath.Join(dir, "Good Track.m4a")
	metadata := testMusicMetadata()
	ffprobeStub := &scannertest.CountingProbe{Hook: func(_ context.Context, path string) (*ffprobe.FfprobeResult, error) {
		if path == badPath {
			return nil, errors.New("ffprobe failed")
		}
		result := *metadata
		return &result, nil
	}}
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

	scanned, skipped, errCount := app.processMusicBatchForTest(t, context.Background(), files)
	if scanned != 1 || skipped != 0 || errCount != 1 {
		t.Fatalf("scan result scanned=%d skipped=%d errors=%d, want 1/0/1", scanned, skipped, errCount)
	}

	var goodCount int
	err := app.tx.DB.QueryRow("SELECT COUNT(*) FROM tracks WHERE file_path = ?", goodPath).Scan(&goodCount)
	if err != nil {
		t.Fatalf("count good track: %v", err)
	}
	if goodCount != 1 {
		t.Fatalf("good track count = %d, want 1", goodCount)
	}

	var badCount int
	err = app.tx.DB.QueryRow("SELECT COUNT(*) FROM tracks WHERE file_path = ?", badPath).Scan(&badCount)
	if err != nil {
		t.Fatalf("count bad track: %v", err)
	}
	if badCount != 0 {
		t.Fatalf("bad track count = %d, want 0", badCount)
	}
	if ffprobeStub.Calls() != 2 {
		t.Fatalf("ffprobe calls = %d, want 2", ffprobeStub.Calls())
	}
}

func TestProcessMusicBatchUsesScanLocalEntityCaches(t *testing.T) {
	app := setupMusicScanner(t)

	spotifyStub := &musicScannerSpotifyStub{
		artistErr: errors.New("artist unavailable"),
		albumErr:  errors.New("album unavailable"),
	}
	app.ffprobe = &scannertest.CountingProbe{Default: testMusicMetadata()}
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

	scanned, skipped, errCount := app.processMusicBatchForTest(t, context.Background(), files)
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
	scanned, skipped, errCount := app.processMusicBatchForTest(t, context.Background(), files)
	if scanned != 2 || skipped != 0 || errCount != 0 {
		t.Fatalf("scan result scanned=%d skipped=%d errors=%d, want 2/0/0", scanned, skipped, errCount)
	}

	got := scannertest.CountRows(t, app.tx.DB, "SELECT COUNT(*) FROM tracks")
	if got != 2 {
		t.Fatalf("track count = %d, want 2", got)
	}
	got = scannertest.CountRows(t, app.tx.DB, "SELECT COUNT(*) FROM musicians WHERE name = ?", "Track Artist")
	if got != 1 {
		t.Fatalf("musician count = %d, want 1", got)
	}
	got = scannertest.CountRows(t, app.tx.DB, "SELECT COUNT(*) FROM albums WHERE title = ? AND musician = ?", "Shared Album", "Album Artist")
	if got != 1 {
		t.Fatalf("album count = %d, want 1", got)
	}
	got = scannertest.CountRows(t, app.tx.DB, "SELECT COUNT(*) FROM genres WHERE tag = ? AND genre_type = ?", "Synth Pop", "music")
	if got != 1 {
		t.Fatalf("genre count = %d, want 1", got)
	}
	got = scannertest.CountRows(t, app.tx.DB, `
		SELECT COUNT(*)
		FROM musician_albums AS ma
		INNER JOIN musicians AS m ON m.id = ma.musician_id
		INNER JOIN albums AS a ON a.id = ma.album_id
		WHERE m.name = ? AND a.title = ?
	`, "Track Artist", "Shared Album")
	if got != 1 {
		t.Fatalf("musician_albums count = %d, want 1", got)
	}
	got = scannertest.CountRows(t, app.tx.DB, "SELECT COUNT(*) FROM track_genres")
	if got != 2 {
		t.Fatalf("track_genres count = %d, want 2", got)
	}
	got = scannertest.CountRows(t, app.tx.DB, "SELECT COUNT(*) FROM musician_genres")
	if got != 1 {
		t.Fatalf("musician_genres count = %d, want 1", got)
	}
	got = scannertest.CountRows(t, app.tx.DB, "SELECT COUNT(*) FROM album_genres")
	if got != 1 {
		t.Fatalf("album_genres count = %d, want 1", got)
	}
}

func TestProcessMusicBatchUpdatesChangedTrackAndReplacesGenre(t *testing.T) {
	app := setupMusicScanner(t)

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
	scanned, skipped, errCount := app.processMusicBatchForTest(t, context.Background(), []scanner.ScanFile{file})
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

	scanned, skipped, errCount = app.processMusicBatchForTest(t, context.Background(), []scanner.ScanFile{file})
	if scanned != 1 || skipped != 0 || errCount != 0 {
		t.Fatalf("second scan result scanned=%d skipped=%d errors=%d, want 1/0/0", scanned, skipped, errCount)
	}

	var title string
	var size int64
	err := app.tx.DB.QueryRow("SELECT title, size FROM tracks WHERE file_path = ?", trackPath).Scan(&title, &size)
	if err != nil {
		t.Fatalf("get updated track: %v", err)
	}
	if title != "Updated Title" || size != 8 {
		t.Fatalf("updated track title/size = %q/%d, want Updated Title/8", title, size)
	}

	got := scannertest.CountRows(t, app.tx.DB, `
		SELECT COUNT(*)
		FROM track_genres AS tg
		INNER JOIN tracks AS t ON t.id = tg.track_id
		INNER JOIN genres AS g ON g.id = tg.genre_id
		WHERE t.file_path = ? AND g.tag = ?
	`, trackPath, "Jazz")
	if got != 1 {
		t.Fatalf("Jazz track genre count = %d, want 1", got)
	}
	got = scannertest.CountRows(t, app.tx.DB, `
		SELECT COUNT(*)
		FROM track_genres AS tg
		INNER JOIN tracks AS t ON t.id = tg.track_id
		INNER JOIN genres AS g ON g.id = tg.genre_id
		WHERE t.file_path = ? AND g.tag = ?
	`, trackPath, "Rock")
	if got != 0 {
		t.Fatalf("Rock track genre count = %d, want 0", got)
	}
}

func TestProcessMusicBatchClearsArtistAlbumAndJoinRowsWhenTagsRemoved(t *testing.T) {
	app := setupMusicScanner(t)

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
	scanned, skipped, errCount := app.processMusicBatchForTest(t, context.Background(), []scanner.ScanFile{file})
	if scanned != 1 || skipped != 0 || errCount != 0 {
		t.Fatalf("first scan result scanned=%d skipped=%d errors=%d, want 1/0/0", scanned, skipped, errCount)
	}

	got := scannertest.CountRows(t, app.tx.DB, `
		SELECT COUNT(*)
		FROM track_musicians AS tm
		INNER JOIN tracks AS t ON t.id = tm.track_id
		WHERE t.file_path = ?
	`, trackPath)
	if got != 1 {
		t.Fatalf("initial track_musicians count = %d, want 1", got)
	}
	got = scannertest.CountRows(t, app.tx.DB, `
		SELECT COUNT(*)
		FROM track_genres AS tg
		INNER JOIN tracks AS t ON t.id = tg.track_id
		WHERE t.file_path = ?
	`, trackPath)
	if got != 1 {
		t.Fatalf("initial track_genres count = %d, want 1", got)
	}

	ffprobeStub.results[trackPath] = testMusicMetadataWithTags(ffprobe.FormatTags{
		Title: "Removed Tags",
	})
	file.Size = 8

	scanned, skipped, errCount = app.processMusicBatchForTest(t, context.Background(), []scanner.ScanFile{file})
	if scanned != 1 || skipped != 0 || errCount != 0 {
		t.Fatalf("second scan result scanned=%d skipped=%d errors=%d, want 1/0/0", scanned, skipped, errCount)
	}

	var musicianID sql.NullInt64
	var albumID sql.NullInt64
	err := app.tx.DB.QueryRow("SELECT musician_id, album_id FROM tracks WHERE file_path = ?", trackPath).Scan(&musicianID, &albumID)
	if err != nil {
		t.Fatalf("get rescanned track relationships: %v", err)
	}
	if musicianID.Valid {
		t.Fatalf("track musician_id = %#v, want null after artist tag removal", musicianID)
	}
	if albumID.Valid {
		t.Fatalf("track album_id = %#v, want null after album tag removal", albumID)
	}

	got = scannertest.CountRows(t, app.tx.DB, `
		SELECT COUNT(*)
		FROM track_musicians AS tm
		INNER JOIN tracks AS t ON t.id = tm.track_id
		WHERE t.file_path = ?
	`, trackPath)
	if got != 0 {
		t.Fatalf("track_musicians count after tag removal = %d, want 0", got)
	}
	got = scannertest.CountRows(t, app.tx.DB, `
		SELECT COUNT(*)
		FROM track_genres AS tg
		INNER JOIN tracks AS t ON t.id = tg.track_id
		WHERE t.file_path = ?
	`, trackPath)
	if got != 0 {
		t.Fatalf("track_genres count after tag removal = %d, want 0", got)
	}
}

func TestProcessMusicBatchDoesNotMergeFailedPersistIntoScanContext(t *testing.T) {
	app := setupMusicScanner(t)

	dir := t.TempDir()
	badPath := filepath.Join(dir, "Bad Cache Track.m4a")
	goodPath := filepath.Join(dir, "Good Cache Track.m4a")
	escapedBadPath := strings.ReplaceAll(badPath, "'", "''")
	_, err := app.tx.DB.Exec(fmt.Sprintf(`CREATE TRIGGER fail_bad_cache_track BEFORE INSERT ON tracks
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

	scanIndex, _, err := app.loadMusicScanIndex(context.Background(), nil, nil)
	if err != nil {
		t.Fatalf("load scan index: %v", err)
	}
	scan := newMusicScanContext(scanIndex)

	files := []scanner.ScanFile{
		{Path: badPath, Ext: "m4a", Size: 5},
		{Path: goodPath, Ext: "m4a", Size: 6},
	}
	scanned, skipped, errCount := app.processMusicFixtureBatch(t, context.Background(), scan, files)
	if scanned != 1 || skipped != 0 || errCount != 1 {
		t.Fatalf("scan result scanned=%d skipped=%d errors=%d, want 1/0/1", scanned, skipped, errCount)
	}
	_, ok := scan.trackIndex[filepath.Clean(badPath)]
	if ok {
		t.Fatal("bad track was merged into scan index after failed transaction")
	}
	got := scan.trackIndex[filepath.Clean(goodPath)].Size
	if got != 6 {
		t.Fatalf("good track scan index size = %d, want 6", got)
	}
	badTracks := scannertest.CountRows(t, app.tx.DB, "SELECT COUNT(*) FROM tracks WHERE file_path = ?", badPath)
	if badTracks != 0 {
		t.Fatalf("bad track count = %d, want 0", badTracks)
	}
	goodTracks := scannertest.CountRows(t, app.tx.DB, "SELECT COUNT(*) FROM tracks WHERE file_path = ?", goodPath)
	if goodTracks != 1 {
		t.Fatalf("good track count = %d, want 1", goodTracks)
	}
}

func TestPersistResolvedTrackInvalidatesOnlyAfterCommitBeforeMergingCaches(t *testing.T) {
	musicDir := t.TempDir()
	s := setupMusicScanner(t)
	ctx := context.Background()
	scan := newMusicScanContext(nil)
	resolved := &resolvedTrack{
		params:    database.UpsertTrackParams{FilePath: musicDir + "/track.m4a", FileName: "track.m4a", Title: "Track", Size: 4, Container: "m4a", MimeType: "audio/mp4"},
		musicians: []resolvedMusician{{name: "Artist", sortName: "Artist"}},
	}
	prepareMusicFixtures(t, []scanner.ScanFile{{Path: resolved.params.FilePath, Size: resolved.params.Size}})
	inspection, inspectErr := scanner.InspectFile(ctx, resolved.params.FilePath, nil, s.now, scanner.FileQuietPeriod)
	if inspectErr != nil {
		t.Fatal(inspectErr)
	}
	defer inspection.Close()
	resolved.inspection = inspection
	var invalidatedIDs []int64
	s.invalidateCommittedTrack = func(trackID int64) {
		invalidatedIDs = append(invalidatedIDs, trackID)
		var title string
		err := s.tx.DB.QueryRow("SELECT title FROM tracks WHERE id = ?", trackID).Scan(&title)
		if err != nil || title != "Track" {
			t.Errorf("callback did not observe committed track: title=%q, err=%v", title, err)
		}
		_, exists := scan.trackIndex[resolved.params.FilePath]
		if exists {
			t.Error("track index updated before invalidation")
		}
		_, cached := scan.musicianIDs.Get(scanner.NormalizedScanCacheKey("Artist"))
		if cached {
			t.Error("entity caches merged before invalidation")
		}
		unlocked := s.tx.Mu.TryLock()
		if unlocked {
			s.tx.Mu.Unlock()
			t.Error("database mutex released before invalidation")
		}
	}

	_, err := s.tx.DB.Exec("CREATE TRIGGER fail_track BEFORE INSERT ON tracks BEGIN SELECT RAISE(ABORT, 'failed track'); END")
	if err != nil {
		t.Fatal(err)
	}
	_, err = s.persistResolvedTrack(ctx, scan, resolved)
	if err == nil || len(invalidatedIDs) != 0 {
		t.Fatalf("failed persist: err=%v, invalidations=%v", err, invalidatedIDs)
	}
	_, err = s.tx.DB.Exec("DROP TRIGGER fail_track")
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
	_, exists := scan.trackIndex[resolved.params.FilePath]
	if !exists {
		t.Error("committed track missing from scan index")
	}
	_, cached := scan.musicianIDs.Get(scanner.NormalizedScanCacheKey("Artist"))
	if !cached {
		t.Error("committed musician missing from scan cache")
	}
}

func TestRelationshipFailuresRollBackAndRetry(t *testing.T) {
	musicDir := t.TempDir()
	cases := []struct {
		name, table, when string
		spotify           bool
	}{
		{name: "musician album", table: "musician_albums"},
		{name: "track musician", table: "track_musicians"},
		{name: "track genre", table: "track_genres"},
		{name: "musician genre", table: "musician_genres"},
		{name: "album genre", table: "album_genres"},
		{name: "Spotify bookkeeping", table: "music_provider_matches", spotify: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			s := setupMusicScanner(t)
			ctx := context.Background()
			file := scanner.ScanFile{Path: musicDir + "/retry.m4a", Ext: "m4a", Size: 2}
			_, err := s.queries.UpsertTrack(ctx, database.UpsertTrackParams{FilePath: file.Path, FileName: "retry.m4a", Title: "Original", Size: 1, Container: "m4a", MimeType: "audio/mp4"})
			if err != nil {
				t.Fatal(err)
			}
			metadata := testMusicMetadata()
			metadata.Format.Tags.Genre = "Local Genre"
			s.ffprobe = &scannertest.CountingProbe{Default: metadata}
			if tc.spotify {
				s.spotify = &musicScannerSpotifyStub{artist: &spotifyapi.FullArtist{SimpleArtist: spotifyapi.SimpleArtist{ID: "artist"}}, album: &spotifyapi.FullAlbum{SimpleAlbum: spotifyapi.SimpleAlbum{ID: "album"}}}
			}
			scan := newMusicScanContext(nil)
			invalidations := 0
			s.invalidateCommittedTrack = func(int64) { invalidations++ }
			_, err = s.tx.DB.Exec("CREATE TRIGGER fail_relationship BEFORE INSERT ON " + tc.table + tc.when + " BEGIN SELECT RAISE(ABORT, 'relationship failure'); END")
			if err != nil {
				t.Fatal(err)
			}
			scanned, _, failures := s.processMusicFixtureBatch(t, ctx, scan, []scanner.ScanFile{file})
			if scanned != 0 || failures != 1 || invalidations != 0 {
				t.Fatalf("scanned=%d errors=%d invalidations=%d", scanned, failures, invalidations)
			}
			for _, table := range []string{"musicians", "albums", "genres", "musician_albums", "track_musicians", "track_genres", "musician_genres", "album_genres", "music_provider_matches"} {
				count := scannertest.CountRows(t, s.tx.DB, "SELECT count(*) FROM "+table)
				if count != 0 {
					t.Fatalf("rollback left %d rows in %s", count, table)
				}
			}
			count := scannertest.CountRows(t, s.tx.DB, "SELECT count(*) FROM tracks WHERE title = 'Original' AND size = 1")
			if count != 1 {
				t.Fatal("original track changed on rollback")
			}
			// External lookup outcomes survive rollback; database IDs never do.
			expected := newMusicScanContext(nil)
			expected.startedAt = scan.startedAt
			expected.artistAttempts = scan.artistAttempts
			expected.albumAttempts = scan.albumAttempts
			expected.enrichmentCounts = scan.enrichmentCounts
			expected.enrichmentOutcome = scan.enrichmentOutcome
			expected.artistAttemptsByID = scan.artistAttemptsByID
			expected.albumAttemptsByID = scan.albumAttemptsByID
			expected.albumTracks = scan.albumTracks
			expected.breaker = scan.breaker
			expected.appleBreaker = scan.appleBreaker
			if !reflect.DeepEqual(scan, expected) {
				t.Fatalf("failed transaction published cache entries: %+v", scan)
			}
			logs := s.logger.(*scannertest.Logger)
			if len(logs.WarnEntries) != 1 || !strings.Contains(fmt.Sprint(logs.WarnEntries[0].Args), file.Path) || !strings.Contains(fmt.Sprint(logs.WarnEntries[0].Args), "relationship failure") {
				t.Fatalf("failure logs: %+v", logs.WarnEntries)
			}
			_, err = s.tx.DB.Exec("DROP TRIGGER fail_relationship")
			if err != nil {
				t.Fatal(err)
			}
			scanned, _, failures = s.processMusicFixtureBatch(t, ctx, scan, []scanner.ScanFile{file})
			if scanned != 1 || failures != 0 || invalidations != 1 {
				t.Fatalf("retry: scanned=%d errors=%d invalidations=%d", scanned, failures, invalidations)
			}
			count = scannertest.CountRows(t, s.tx.DB, "SELECT count(*) FROM tracks WHERE title = 'Test Track' AND size = 2")
			if count != 1 {
				t.Fatal("retry failed to update track")
			}
		})
	}
}

func TestTrackGenreRestoredWithinScan(t *testing.T) {
	musicDir := t.TempDir()
	s := setupMusicScanner(t)
	scan := newMusicScanContext(nil)
	metadata := testMusicMetadata()
	s.ffprobe = &scannertest.CountingProbe{Default: metadata}
	for i, genre := range []string{"A", "B", "A"} {
		metadata.Format.Tags.Genre = genre
		scanned, _, failures := s.processMusicFixtureBatch(t, context.Background(), scan, []scanner.ScanFile{{Path: musicDir + "/genre.m4a", Ext: "m4a", Size: int64(i + 1)}})
		if scanned != 1 || failures != 0 {
			t.Fatalf("genre %s: scanned=%d errors=%d", genre, scanned, failures)
		}
		var got string
		err := s.tx.DB.QueryRow("SELECT g.tag FROM track_genres tg JOIN genres g ON g.id = tg.genre_id").Scan(&got)
		if err != nil || got != genre {
			t.Fatalf("genre=%q err=%v, want %q", got, err, genre)
		}
	}
}

func TestUniqueIDsPreservesOrderAndDropsRepeats(t *testing.T) {
	got := uniqueIDs([]int64{3, 1, 3}, []int64{1, 2})
	want := []int64{3, 1, 2}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("uniqueIDs = %v, want %v", got, want)
	}

	// A rescan of an unchanged track sees the same artists in the old and the
	// new credit set; reconciling each one twice is the case this exists for.
	artists := []int64{7, 8}
	got = uniqueIDs(artists, []int64{7, 8})
	if !reflect.DeepEqual(got, []int64{7, 8}) {
		t.Fatalf("unchanged rescan = %v, want 7,8 once each", got)
	}
	if !reflect.DeepEqual(artists, []int64{7, 8}) {
		t.Fatalf("uniqueIDs mutated its input: %v", artists)
	}

	got = uniqueIDs(nil)
	if len(got) != 0 {
		t.Fatalf("uniqueIDs(nil) = %v, want empty", got)
	}

	present := validIDs(sql.NullInt64{}, sql.NullInt64{Int64: 5, Valid: true})
	if !reflect.DeepEqual(present, []int64{5}) {
		t.Fatalf("validIDs = %v, want only the valid id", present)
	}
}

func TestAlbumMusicBrainzIDsFollowTheMajorityOfTheirTracks(t *testing.T) {
	s := setupMusicScanner(t)
	dir := t.TempDir()
	const releaseA = "0a000000-0000-4000-8000-000000000000"
	const releaseB = "0b000000-0000-4000-8000-000000000000"
	const group = "0c000000-0000-4000-8000-000000000000"
	tagged := func(title, release string) ffprobe.FormatTags {
		return ffprobe.FormatTags{Title: title, Artist: "Coldplay", Album: "Parachutes", MusicBrainzReleaseID: release, MusicBrainzReleaseGroupID: group}
	}
	albumIDs := func() (string, string) {
		t.Helper()
		var release, releaseGroup sql.NullString
		err := s.tx.DB.QueryRow("SELECT musicbrainz_release_id, musicbrainz_release_group_id FROM albums").Scan(&release, &releaseGroup)
		if err != nil {
			t.Fatal(err)
		}
		return release.String, releaseGroup.String
	}

	scanTaggedTrack(t, s, newMusicScanContext(nil), filepath.Join(dir, "one"), 1, tagged("One", releaseB))
	scanTaggedTrack(t, s, newMusicScanContext(nil), filepath.Join(dir, "two"), 1, tagged("Two", releaseA))
	release, releaseGroup := albumIDs()
	if release != releaseA || releaseGroup != group {
		t.Fatalf("tied album ids = %q, %q, want the lowest release %q and %q", release, releaseGroup, releaseA, group)
	}

	scanTaggedTrack(t, s, newMusicScanContext(nil), filepath.Join(dir, "three"), 1, tagged("Three", releaseB))
	release, _ = albumIDs()
	if release != releaseB {
		t.Fatalf("album release = %q, want the majority %q", release, releaseB)
	}

	// Changing a track's tags takes its vote back.
	scanTaggedTrack(t, s, newMusicScanContext(nil), filepath.Join(dir, "three"), 2, tagged("Three", ""))
	release, _ = albumIDs()
	if release != releaseA {
		t.Fatalf("album release after a vote went away = %q, want %q", release, releaseA)
	}
}

func TestArtistMusicBrainzIDsPairWithCreditsOnlyOnePerCredit(t *testing.T) {
	s := setupMusicScanner(t)
	dir := t.TempDir()
	const lead = "1a000000-0000-4000-8000-000000000000"
	const guest = "1b000000-0000-4000-8000-000000000000"
	musicianIDs := func() map[string]string {
		t.Helper()
		rows, err := s.tx.DB.Query("SELECT name, COALESCE(musicbrainz_id, '') FROM musicians")
		if err != nil {
			t.Fatal(err)
		}
		defer rows.Close()
		ids := map[string]string{}
		for rows.Next() {
			var name, id string
			err = rows.Scan(&name, &id)
			if err != nil {
				t.Fatal(err)
			}
			ids[name] = id
		}
		return ids
	}

	// The ARTISTS tag names two credits and the id tag gives one id each.
	scanTaggedTrack(t, s, newMusicScanContext(nil), filepath.Join(dir, "one"), 1, ffprobe.FormatTags{
		Title: "One", Artist: "Lead feat. Guest", Artists: "Lead;Guest", MusicBrainzArtistID: lead + ";" + guest,
	})
	got := musicianIDs()
	if got["Lead"] != lead || got["Guest"] != guest || len(got) != 2 {
		t.Fatalf("musician ids = %v, want Lead %s and Guest %s", got, lead, guest)
	}

	// One credit and two ids: the positions cannot be trusted, so no vote.
	scanTaggedTrack(t, s, newMusicScanContext(nil), filepath.Join(dir, "two"), 1, ffprobe.FormatTags{
		Title: "Two", Artist: "Solo", MusicBrainzArtistID: lead + ";" + guest,
	})
	got = musicianIDs()
	if got["Solo"] != "" {
		t.Fatalf("Solo musicbrainz_id = %q, want none from an unpaired id list", got["Solo"])
	}

	// A lone credit that reads as two names, beside one id: ffprobe may have
	// cut an m4a's (MP4) list to its first id, which is one of the two artists'.
	scanTaggedTrack(t, s, newMusicScanContext(nil), filepath.Join(dir, "three"), 1, ffprobe.FormatTags{
		Title: "Three", Artist: "Simon & Garfunkel", MusicBrainzArtistID: lead,
	})
	got = musicianIDs()
	if got["Simon & Garfunkel"] != "" {
		t.Fatalf("Simon & Garfunkel musicbrainz_id = %q, want none for a lone compound credit in an m4a", got["Simon & Garfunkel"])
	}
	// A Vorbis comment list is never cut, so a FLAC's one id is the duo's.
	s.ffprobe = &scannertest.CountingProbe{Default: testMusicMetadataWithTags(ffprobe.FormatTags{
		Title: "Four", Artist: "Hall & Oates", MusicBrainzArtistID: guest,
	})}
	imported, _, failures := s.processMusicFixtureBatch(t, context.Background(), newMusicScanContext(nil), []scanner.ScanFile{{Path: filepath.Join(dir, "four.flac"), Ext: "flac", Size: 1}})
	if imported != 1 || failures != 0 {
		t.Fatalf("scan four.flac: %d imported, %d failed", imported, failures)
	}
	got = musicianIDs()
	if got["Hall & Oates"] != guest {
		t.Fatalf("Hall & Oates musicbrainz_id = %q, want %q from a whole Vorbis list", got["Hall & Oates"], guest)
	}

	// Removing the only voting track clears the artist's id.
	scanTaggedTrack(t, s, newMusicScanContext(nil), filepath.Join(dir, "one"), 2, ffprobe.FormatTags{
		Title: "One", Artist: "Lead feat. Guest", Artists: "Lead;Guest",
	})
	got = musicianIDs()
	if got["Lead"] != "" || got["Guest"] != "" {
		t.Fatalf("musician ids after the votes went away = %v, want none", got)
	}
	votes := scannertest.CountRows(t, s.tx.DB, "SELECT count(*) FROM music_credit_ids c JOIN tracks t ON t.id = c.track_id WHERE t.file_path = ?", filepath.Join(dir, "one"))
	if votes != 0 {
		t.Fatalf("music_credit_ids rows for the rescanned track = %d, want 0", votes)
	}
}

func TestTrackMetadataKeepsTheTagsLaterPassesRead(t *testing.T) {
	s := setupMusicScanner(t)
	const recording = "2a000000-0000-4000-8000-000000000000"
	const albumArtist = "2b000000-0000-4000-8000-000000000000"
	scanTaggedTrack(t, s, newMusicScanContext(nil), filepath.Join(t.TempDir(), "one"), 1, ffprobe.FormatTags{
		Title: "One", Artist: "A", Album: "Pure Dance", Track: "2", TrackTotal: "12", Disc: "1/2",
		Compilation: "1", ISRC: "usum71703861", Label: "Ministry of Sound", Artists: "A;B",
		MusicBrainzRecordingID: recording, MusicBrainzAlbumArtistID: albumArtist,
	})

	var row struct {
		artists, isrc, label, recording, albumArtists string
		compilation                                   bool
		trackTotal, discTotal                         int64
	}
	err := s.tx.DB.QueryRow(`SELECT artists_tag, isrc, label, mb_recording_id, mb_album_artist_ids, compilation, track_total, disc_total
		FROM music_track_metadata`).Scan(&row.artists, &row.isrc, &row.label, &row.recording, &row.albumArtists, &row.compilation, &row.trackTotal, &row.discTotal)
	if err != nil {
		t.Fatal(err)
	}
	want := struct {
		artists, isrc, label, recording, albumArtists string
		compilation                                   bool
		trackTotal, discTotal                         int64
	}{"A;B", "USUM71703861", "Ministry of Sound", recording, albumArtist, true, 12, 2}
	if row != want {
		t.Fatalf("music_track_metadata = %+v, want %+v", row, want)
	}
}

func TestArtistMusicBrainzIDGetsOneVotePerTrack(t *testing.T) {
	s := setupMusicScanner(t)
	const first = "6a000000-0000-4000-8000-000000000000"
	const repeat = "6b000000-0000-4000-8000-000000000000"
	const other = "6c000000-0000-4000-8000-000000000000"
	// The repeated credit's second id does not out-vote its first.
	scanTaggedTrack(t, s, newMusicScanContext(nil), filepath.Join(t.TempDir(), "one"), 1, ffprobe.FormatTags{
		Title: "One", Artist: "Artist One, Artist Two, Artist One", MusicBrainzArtistID: first + ";" + other + ";" + repeat,
	})
	votes := scannertest.CountRows(t, s.tx.DB, "SELECT count(*) FROM music_credit_ids")
	if votes != 2 {
		t.Fatalf("music_credit_ids rows = %d, want one per credited artist", votes)
	}
	var id string
	err := s.tx.DB.QueryRow("SELECT musicbrainz_id FROM musicians WHERE name = 'Artist One'").Scan(&id)
	if err != nil {
		t.Fatal(err)
	}
	if id != first {
		t.Fatalf("Artist One musicbrainz_id = %q, want the first credit's %q", id, first)
	}
}

func TestASingleArtistsValueIsKeptAsTheLeadNotAsCredits(t *testing.T) {
	s := setupMusicScanner(t)
	// The ID3 list was cut to its first value: the lead, not the credits.
	scanTaggedTrack(t, s, newMusicScanContext(nil), filepath.Join(t.TempDir(), "one"), 1, ffprobe.FormatTags{
		Title: "One", Artist: "Leslie Odom, Jr. & Lin-Manuel Miranda", Artists: "Leslie Odom, Jr.",
	})
	var stored string
	err := s.tx.DB.QueryRow("SELECT artists_tag FROM music_track_metadata").Scan(&stored)
	if err != nil {
		t.Fatal(err)
	}
	if stored != "Leslie Odom, Jr." {
		t.Fatalf("artists_tag = %q, want the lone lead kept", stored)
	}
	credits := scannertest.CountRows(t, s.tx.DB, "SELECT count(*) FROM track_musicians")
	if credits != 2 {
		t.Fatalf("credits = %d, want the ARTIST string's two", credits)
	}
}
