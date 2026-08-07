package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"igloo/cmd/internal/database"
	"igloo/cmd/internal/ffprobe"
	"igloo/cmd/internal/helpers"
)

func (app *Application) processMusicBatchForTest(ctx context.Context, files []helpers.ScanFile) (scanned, skipped, errCount int) {
	scanIndex, err := app.loadMusicScanIndex(ctx)
	if err != nil {
		app.Logger.Error(fmt.Sprintf("failed to load music scan index: %s", err.Error()))
		return 0, 0, len(files)
	}

	return app.processMusicBatch(ctx, newMusicScanContext(scanIndex), files)
}

func testMusicMetadata() *ffprobe.FfprobeResult {
	return &ffprobe.FfprobeResult{
		Format: ffprobe.Format{
			Duration: "180",
			BitRate:  "256000",
			Tags: ffprobe.FormatTags{
				Title:  "Test Track",
				Artist: "Test Artist",
				Album:  "Test Album",
				Track:  "1/10",
			},
		},
		Streams: []ffprobe.Stream{
			{
				Index:         0,
				CodecName:     "aac",
				CodecType:     "audio",
				Channels:      2,
				ChannelLayout: "stereo",
			},
		},
	}
}

func testMusicMetadataWithTags(tags ffprobe.FormatTags) *ffprobe.FfprobeResult {
	return &ffprobe.FfprobeResult{
		Format: ffprobe.Format{
			Duration: "180.250",
			BitRate:  "256000",
			Tags:     tags,
		},
		Streams: []ffprobe.Stream{
			{
				Index:         0,
				CodecName:     "aac",
				CodecType:     "audio",
				Profile:       "LC",
				Channels:      2,
				ChannelLayout: "stereo",
				Tags: ffprobe.StreamTags{
					Language: "eng",
				},
			},
		},
	}
}

type countingMusicScannerFfprobe struct {
	noKeyframeProbe
	result *ffprobe.FfprobeResult
	calls  int
}

func (s *countingMusicScannerFfprobe) GetMetadata(filePath string) (*ffprobe.FfprobeResult, error) {
	s.calls++
	return s.result, nil
}

func (s *countingMusicScannerFfprobe) GetAudioMetadata(_ context.Context, filePath string) (*ffprobe.FfprobeResult, error) {
	s.calls++
	return s.result, nil
}

type failingPathMusicScannerFfprobe struct {
	noKeyframeProbe
	result      *ffprobe.FfprobeResult
	failingPath string
	calls       int
}

func (s *failingPathMusicScannerFfprobe) GetMetadata(filePath string) (*ffprobe.FfprobeResult, error) {
	s.calls++
	if filePath == s.failingPath {
		return nil, errors.New("ffprobe failed")
	}

	return s.result, nil
}

func (s *failingPathMusicScannerFfprobe) GetAudioMetadata(_ context.Context, filePath string) (*ffprobe.FfprobeResult, error) {
	s.calls++
	if filePath == s.failingPath {
		return nil, errors.New("ffprobe failed")
	}

	return s.result, nil
}

type musicScannerFfprobeByPath struct {
	noKeyframeProbe
	results       map[string]*ffprobe.FfprobeResult
	errors        map[string]error
	metadataCalls map[string]int
	audioCalls    map[string]int
}

func newMusicScannerFfprobeByPath(results map[string]*ffprobe.FfprobeResult) *musicScannerFfprobeByPath {
	return &musicScannerFfprobeByPath{
		results:       results,
		errors:        make(map[string]error),
		metadataCalls: make(map[string]int),
		audioCalls:    make(map[string]int),
	}
}

func (s *musicScannerFfprobeByPath) GetMetadata(filePath string) (*ffprobe.FfprobeResult, error) {
	s.metadataCalls[filePath]++
	return s.resultForPath(filePath)
}

func (s *musicScannerFfprobeByPath) GetAudioMetadata(_ context.Context, filePath string) (*ffprobe.FfprobeResult, error) {
	s.audioCalls[filePath]++
	return s.resultForPath(filePath)
}

func (s *musicScannerFfprobeByPath) resultForPath(filePath string) (*ffprobe.FfprobeResult, error) {
	if err, ok := s.errors[filePath]; ok {
		return nil, err
	}

	result, ok := s.results[filePath]
	if !ok {
		return nil, fmt.Errorf("unexpected ffprobe path: %s", filePath)
	}

	return result, nil
}

func (s *musicScannerFfprobeByPath) totalAudioCalls() int {
	total := 0
	for _, calls := range s.audioCalls {
		total += calls
	}
	return total
}

func (s *musicScannerFfprobeByPath) totalMetadataCalls() int {
	total := 0
	for _, calls := range s.metadataCalls {
		total += calls
	}
	return total
}

func runMusicScanForTest(t *testing.T, app *Application) {
	t.Helper()

	musicScanGuard.Finish()
	if !musicScanGuard.TryBegin() {
		t.Fatal("failed to acquire music scan guard")
	}

	app.runMusicScan()
}

func writeMusicScannerTestFile(t *testing.T, path, contents string) int64 {
	t.Helper()

	err := os.MkdirAll(filepath.Dir(path), 0755)
	if err != nil {
		t.Fatalf("create test music directory: %v", err)
	}

	err = os.WriteFile(path, []byte(contents), 0644)
	if err != nil {
		t.Fatalf("write test music file: %v", err)
	}

	return int64(len(contents))
}

func countMusicScannerRows(t *testing.T, db *sql.DB, query string, args ...any) int {
	t.Helper()

	var count int
	err := db.QueryRow(query, args...).Scan(&count)
	if err != nil {
		t.Fatalf("count rows: %v", err)
	}

	return count
}

func TestProcessMusicBatchInsertsTrackAndSkipsExistingPathSize(t *testing.T) {
	app := setupTestApp(t)
	defer app.DB.Close()

	ffprobeStub := &countingMusicScannerFfprobe{result: testMusicMetadata()}
	app.Ffprobe = ffprobeStub

	file := helpers.ScanFile{
		Path: filepath.Join(t.TempDir(), "Test Track.m4a"),
		Ext:  "m4a",
		Size: 5,
	}

	scanned, skipped, errCount := app.processMusicBatchForTest(context.Background(), []helpers.ScanFile{file})
	if scanned != 1 || skipped != 0 || errCount != 0 {
		t.Fatalf("first scan result scanned=%d skipped=%d errors=%d, want 1/0/0", scanned, skipped, errCount)
	}

	var trackCount int
	err := app.DB.QueryRow("SELECT COUNT(*) FROM tracks WHERE file_path = ? AND size = ?", file.Path, file.Size).Scan(&trackCount)
	if err != nil {
		t.Fatalf("count tracks: %v", err)
	}
	if trackCount != 1 {
		t.Fatalf("track count = %d, want 1", trackCount)
	}
	if ffprobeStub.calls != 1 {
		t.Fatalf("ffprobe calls = %d, want 1", ffprobeStub.calls)
	}

	scanned, skipped, errCount = app.processMusicBatchForTest(context.Background(), []helpers.ScanFile{file})
	if scanned != 0 || skipped != 1 || errCount != 0 {
		t.Fatalf("second scan result scanned=%d skipped=%d errors=%d, want 0/1/0", scanned, skipped, errCount)
	}
	if ffprobeStub.calls != 1 {
		t.Fatalf("ffprobe calls after skip = %d, want 1", ffprobeStub.calls)
	}

	changedFile := file
	changedFile.Size = 6
	scanned, skipped, errCount = app.processMusicBatchForTest(context.Background(), []helpers.ScanFile{changedFile})
	if scanned != 1 || skipped != 0 || errCount != 0 {
		t.Fatalf("changed size scan result scanned=%d skipped=%d errors=%d, want 1/0/0", scanned, skipped, errCount)
	}
	if ffprobeStub.calls != 2 {
		t.Fatalf("ffprobe calls after changed size = %d, want 2", ffprobeStub.calls)
	}
}

func TestProcessMusicBatchCommitsMultipleTracks(t *testing.T) {
	app := setupTestApp(t)
	defer app.DB.Close()

	ffprobeStub := &countingMusicScannerFfprobe{result: testMusicMetadata()}
	app.Ffprobe = ffprobeStub

	dir := t.TempDir()
	files := []helpers.ScanFile{
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

	var trackCount int
	err := app.DB.QueryRow("SELECT COUNT(*) FROM tracks").Scan(&trackCount)
	if err != nil {
		t.Fatalf("count tracks: %v", err)
	}
	if trackCount != 2 {
		t.Fatalf("track count = %d, want 2", trackCount)
	}
	if ffprobeStub.calls != 2 {
		t.Fatalf("ffprobe calls = %d, want 2", ffprobeStub.calls)
	}
}

func TestProcessMusicBatchSkipsBadTrackAndCommitsGoodTrack(t *testing.T) {
	app := setupTestApp(t)
	defer app.DB.Close()

	dir := t.TempDir()
	badPath := filepath.Join(dir, "Bad Track.m4a")
	goodPath := filepath.Join(dir, "Good Track.m4a")
	ffprobeStub := &failingPathMusicScannerFfprobe{
		result:      testMusicMetadata(),
		failingPath: badPath,
	}
	app.Ffprobe = ffprobeStub

	files := []helpers.ScanFile{
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
	err := app.DB.QueryRow("SELECT COUNT(*) FROM tracks WHERE file_path = ?", goodPath).Scan(&goodCount)
	if err != nil {
		t.Fatalf("count good track: %v", err)
	}
	if goodCount != 1 {
		t.Fatalf("good track count = %d, want 1", goodCount)
	}

	var badCount int
	err = app.DB.QueryRow("SELECT COUNT(*) FROM tracks WHERE file_path = ?", badPath).Scan(&badCount)
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

func TestProcessMusicBatchRollsBackFailedPersistAndCommitsLaterTrack(t *testing.T) {
	app := setupTestApp(t)
	defer app.DB.Close()

	dir := t.TempDir()
	badPath := filepath.Join(dir, "Bad Track.m4a")
	goodPath := filepath.Join(dir, "Good Track.m4a")
	escapedBadPath := strings.ReplaceAll(badPath, "'", "''")
	_, err := app.DB.Exec(fmt.Sprintf(`CREATE TRIGGER fail_bad_track BEFORE INSERT ON tracks
		WHEN new.file_path = '%s'
		BEGIN
			SELECT RAISE(ABORT, 'forced track failure');
		END;`, escapedBadPath))
	if err != nil {
		t.Fatalf("create failing trigger: %v", err)
	}

	ffprobeStub := &countingMusicScannerFfprobe{result: testMusicMetadata()}
	app.Ffprobe = ffprobeStub

	files := []helpers.ScanFile{
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

	var badCount int
	err = app.DB.QueryRow("SELECT COUNT(*) FROM tracks WHERE file_path = ?", badPath).Scan(&badCount)
	if err != nil {
		t.Fatalf("count bad track: %v", err)
	}
	if badCount != 0 {
		t.Fatalf("bad track count = %d, want 0", badCount)
	}

	var goodCount int
	err = app.DB.QueryRow("SELECT COUNT(*) FROM tracks WHERE file_path = ?", goodPath).Scan(&goodCount)
	if err != nil {
		t.Fatalf("count good track: %v", err)
	}
	if goodCount != 1 {
		t.Fatalf("good track count = %d, want 1", goodCount)
	}
}

func TestMusicScanGuardPreventsConcurrentScans(t *testing.T) {
	musicScanGuard.Finish()

	if !musicScanGuard.TryBegin() {
		t.Fatal("first music scan guard acquisition failed")
	}
	defer musicScanGuard.Finish()

	if musicScanGuard.TryBegin() {
		t.Fatal("second music scan guard acquisition succeeded, want blocked")
	}

	musicScanGuard.Finish()
	if !musicScanGuard.TryBegin() {
		t.Fatal("music scan guard did not reset after finish")
	}
}

func TestProcessMusicBatchUsesScanLocalEntityCaches(t *testing.T) {
	app := setupTestApp(t)
	defer app.DB.Close()

	// Every UpsertMusician/UpsertAlbum that hits an existing row fires the
	// conflict UPDATE. The per-scan caches must short-circuit the second track's
	// upserts entirely, so no UPDATE may fire during the batch.
	_, err := app.DB.Exec(`CREATE TABLE music_upsert_probe (event TEXT NOT NULL)`)
	if err != nil {
		t.Fatalf("create probe table: %v", err)
	}
	_, err = app.DB.Exec(`CREATE TRIGGER probe_musician_update AFTER UPDATE ON musicians
		BEGIN
			INSERT INTO music_upsert_probe (event) VALUES ('musician_update');
		END;`)
	if err != nil {
		t.Fatalf("create musician trigger: %v", err)
	}
	_, err = app.DB.Exec(`CREATE TRIGGER probe_album_update AFTER UPDATE ON albums
		BEGIN
			INSERT INTO music_upsert_probe (event) VALUES ('album_update');
		END;`)
	if err != nil {
		t.Fatalf("create album trigger: %v", err)
	}

	app.Ffprobe = &countingMusicScannerFfprobe{result: testMusicMetadata()}

	dir := t.TempDir()
	files := []helpers.ScanFile{
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

	if got := countMusicScannerRows(t, app.DB, "SELECT COUNT(*) FROM musicians"); got != 1 {
		t.Fatalf("musician count = %d, want 1", got)
	}
	if got := countMusicScannerRows(t, app.DB, "SELECT COUNT(*) FROM albums"); got != 1 {
		t.Fatalf("album count = %d, want 1", got)
	}
	if got := countMusicScannerRows(t, app.DB, "SELECT COUNT(*) FROM music_upsert_probe"); got != 0 {
		t.Fatalf("entity upsert updates during one batch = %d, want 0 (scan cache not used)", got)
	}
}

func TestRunMusicScanWalksAudioFilesAndSkipsUnchangedFiles(t *testing.T) {
	app := setupTestApp(t)
	defer app.DB.Close()

	musicDir := t.TempDir()
	m4aPath := filepath.Join(musicDir, "Album", "Track One.m4a")
	mp3Path := filepath.Join(musicDir, "Album", "Track Two.MP3")
	flacPath := filepath.Join(musicDir, "Nested", "Track Three.flac")
	ignoredPath := filepath.Join(musicDir, "Album", "cover.jpg")

	writeMusicScannerTestFile(t, m4aPath, "m4a")
	writeMusicScannerTestFile(t, mp3Path, "mp3")
	writeMusicScannerTestFile(t, flacPath, "flac")
	writeMusicScannerTestFile(t, ignoredPath, "jpg")

	ffprobeStub := newMusicScannerFfprobeByPath(map[string]*ffprobe.FfprobeResult{
		m4aPath: testMusicMetadataWithTags(ffprobe.FormatTags{
			Title:  "Track One",
			Artist: "Walk Artist",
			Album:  "Walk Album",
			Track:  "1/3",
		}),
		mp3Path: testMusicMetadataWithTags(ffprobe.FormatTags{
			Title:  "Track Two",
			Artist: "Walk Artist",
			Album:  "Walk Album",
			Track:  "2/3",
		}),
		flacPath: testMusicMetadataWithTags(ffprobe.FormatTags{
			Title:  "Track Three",
			Artist: "Walk Artist",
			Album:  "Walk Album",
			Track:  "3/3",
		}),
	})
	app.Ffprobe = ffprobeStub
	app.Settings = &database.Setting{
		MusicDir: sql.NullString{String: musicDir, Valid: true},
	}

	runMusicScanForTest(t, app)

	if got := countMusicScannerRows(t, app.DB, "SELECT COUNT(*) FROM tracks"); got != 3 {
		t.Fatalf("track count after first scan = %d, want 3", got)
	}
	if got := countMusicScannerRows(t, app.DB, "SELECT COUNT(*) FROM tracks WHERE file_path = ?", ignoredPath); got != 0 {
		t.Fatalf("ignored file track count = %d, want 0", got)
	}
	if ffprobeStub.totalAudioCalls() != 3 {
		t.Fatalf("audio metadata calls = %d, want 3", ffprobeStub.totalAudioCalls())
	}
	if ffprobeStub.totalMetadataCalls() != 0 {
		t.Fatalf("generic metadata calls = %d, want 0", ffprobeStub.totalMetadataCalls())
	}

	runMusicScanForTest(t, app)

	if ffprobeStub.totalAudioCalls() != 3 {
		t.Fatalf("audio metadata calls after unchanged rescan = %d, want 3", ffprobeStub.totalAudioCalls())
	}

	newSize := writeMusicScannerTestFile(t, mp3Path, "mp3 changed")
	ffprobeStub.results[mp3Path] = testMusicMetadataWithTags(ffprobe.FormatTags{
		Title:  "Track Two Updated",
		Artist: "Walk Artist",
		Album:  "Walk Album",
		Track:  "2/3",
	})

	runMusicScanForTest(t, app)

	if ffprobeStub.totalAudioCalls() != 4 {
		t.Fatalf("audio metadata calls after changed rescan = %d, want 4", ffprobeStub.totalAudioCalls())
	}
	if ffprobeStub.audioCalls[mp3Path] != 2 {
		t.Fatalf("changed file audio calls = %d, want 2", ffprobeStub.audioCalls[mp3Path])
	}

	var title string
	var size int64
	err := app.DB.QueryRow("SELECT title, size FROM tracks WHERE file_path = ?", mp3Path).Scan(&title, &size)
	if err != nil {
		t.Fatalf("get updated track: %v", err)
	}
	if title != "Track Two Updated" || size != newSize {
		t.Fatalf("updated track title/size = %q/%d, want %q/%d", title, size, "Track Two Updated", newSize)
	}
}

func TestResolveTrackFileMapsAudioMetadata(t *testing.T) {
	app := setupTestApp(t)
	defer app.DB.Close()

	trackPath := filepath.Join(t.TempDir(), "Mapped Track.flac")
	metadata := &ffprobe.FfprobeResult{
		Format: ffprobe.Format{
			Duration: "245.125",
			BitRate:  "1411200",
			Tags: ffprobe.FormatTags{
				Title:       "Mapped Title",
				Artist:      "Mapped Artist",
				AlbumArtist: "Mapped Album Artist",
				Composer:    "Mapped Composer",
				Album:       "Mapped Album",
				Genre:       "Mapped Genre",
				Track:       "7/12",
				Disc:        "2/3",
				Date:        "2024-02-03",
				Copyright:   "Mapped Copyright",
				SortName:    "Mapped Sort Title",
				SortAlbum:   "Mapped Sort Album",
				SortArtist:  "Mapped Sort Artist",
			},
		},
		Streams: []ffprobe.Stream{
			{
				Index:     0,
				CodecName: "mjpeg",
				CodecType: "video",
				Disposition: ffprobe.StreamDisposition{
					AttachedPic: 1,
				},
			},
			{
				Index:         1,
				CodecName:     "flac",
				CodecType:     "audio",
				Profile:       "Lossless",
				Channels:      6,
				ChannelLayout: "5.1",
				SampleRate:    "44100",
				Tags: ffprobe.StreamTags{
					Language: "jpn",
				},
			},
		},
	}
	app.Ffprobe = newMusicScannerFfprobeByPath(map[string]*ffprobe.FfprobeResult{
		trackPath: metadata,
	})

	resolved, err := app.resolveTrackFile(context.Background(), helpers.ScanFile{
		Path: trackPath,
		Ext:  "flac",
		Size: 42,
	})
	if err != nil {
		t.Fatalf("resolve track file: %v", err)
	}

	params := resolved.params
	if params.Title != "Mapped Title" || params.SortTitle != "Mapped Sort Title" {
		t.Fatalf("title/sort_title = %q/%q, want mapped tags", params.Title, params.SortTitle)
	}
	if params.Container != "flac" || params.MimeType != "audio/flac" {
		t.Fatalf("container/mime = %q/%q, want flac/audio/flac", params.Container, params.MimeType)
	}
	if params.Duration != 245125 || params.TrackIndex != 7 || params.Disc != 2 {
		t.Fatalf("duration/track/disc = %d/%d/%d, want 245125/7/2", params.Duration, params.TrackIndex, params.Disc)
	}
	if params.BitRate != 1411200 {
		t.Fatalf("bit rate = %d, want 1411200", params.BitRate)
	}
	if params.Codec != "flac" || params.Profile != "Lossless" || params.Channels != "6" || params.ChannelLayout != "5.1" {
		t.Fatalf("audio fields = codec %q profile %q channels %q layout %q, want flac/Lossless/6/5.1",
			params.Codec, params.Profile, params.Channels, params.ChannelLayout)
	}
	if !params.SampleRate.Valid || params.SampleRate.Int64 != 44100 {
		t.Fatalf("sample rate = %#v, want 44100", params.SampleRate)
	}
	if !params.Language.Valid || params.Language.String != "jpn" {
		t.Fatalf("language = %#v, want jpn", params.Language)
	}
	if !params.ReleaseDate.Valid || params.ReleaseDate.String != "2024-02-03" {
		t.Fatalf("release date = %#v, want 2024-02-03", params.ReleaseDate)
	}
	if !params.Year.Valid || params.Year.Int64 != 2024 {
		t.Fatalf("year = %#v, want 2024", params.Year)
	}
	if !params.Composer.Valid || params.Composer.String != "Mapped Composer" {
		t.Fatalf("composer = %#v, want mapped composer", params.Composer)
	}
	if !params.Copyright.Valid || params.Copyright.String != "Mapped Copyright" {
		t.Fatalf("copyright = %#v, want mapped copyright", params.Copyright)
	}
	if resolved.genreTag != "Mapped Genre" {
		t.Fatalf("genre tag = %q, want Mapped Genre", resolved.genreTag)
	}
	if len(resolved.musicians) != 1 || resolved.musicians[0].name != "Mapped Artist" || resolved.musicians[0].sortName != "Mapped Sort Artist" {
		t.Fatalf("resolved musicians = %#v, want mapped artist and sort artist", resolved.musicians)
	}
	if resolved.musicians[0].nameKey != "mapped artist" {
		t.Fatalf("musician name key = %q, want %q", resolved.musicians[0].nameKey, "mapped artist")
	}
	if resolved.album == nil {
		t.Fatal("expected resolved album")
	}
	if resolved.album.title != "Mapped Album" || resolved.album.sortTitle != "Mapped Sort Album" || resolved.album.albumArtist != "Mapped Album Artist" {
		t.Fatalf("resolved album = %#v, want mapped album tags", resolved.album)
	}
	wantAlbumKey := albumIdentityKey("Mapped Album", "Mapped Album Artist", false)
	if resolved.album.albumKey != wantAlbumKey {
		t.Fatalf("album key = %q, want %q", resolved.album.albumKey, wantAlbumKey)
	}
	if resolved.album.isCompilation {
		t.Fatal("album resolved as compilation, want regular album")
	}
	if resolved.album.totalTracks != 12 {
		t.Fatalf("album total tracks = %d, want 12 from track tag", resolved.album.totalTracks)
	}
}

func TestResolveTrackFileFallsBackToFilenameAndNumericDefaults(t *testing.T) {
	app := setupTestApp(t)
	defer app.DB.Close()

	trackPath := filepath.Join(t.TempDir(), "No Tags.mp3")
	app.Ffprobe = newMusicScannerFfprobeByPath(map[string]*ffprobe.FfprobeResult{
		trackPath: {
			Format: ffprobe.Format{
				Duration: "not-a-duration",
				BitRate:  "not-a-bitrate",
			},
			Streams: []ffprobe.Stream{
				{
					CodecName: "mp3",
					CodecType: "audio",
					Channels:  2,
				},
			},
		},
	})

	resolved, err := app.resolveTrackFile(context.Background(), helpers.ScanFile{
		Path: trackPath,
		Ext:  "mp3",
		Size: 7,
	})
	if err != nil {
		t.Fatalf("resolve track file: %v", err)
	}

	params := resolved.params
	if params.Title != "No Tags" || params.SortTitle != "No Tags" {
		t.Fatalf("title/sort_title = %q/%q, want filename fallback without extension", params.Title, params.SortTitle)
	}
	if params.MimeType != "audio/mpeg" {
		t.Fatalf("mime type = %q, want audio/mpeg", params.MimeType)
	}
	if params.Duration != 0 || params.BitRate != 0 {
		t.Fatalf("duration/bitrate = %d/%d, want zero defaults", params.Duration, params.BitRate)
	}
	if params.Channels != "2" || params.ChannelLayout != "2" {
		t.Fatalf("channels/layout = %q/%q, want numeric fallback", params.Channels, params.ChannelLayout)
	}
	if params.SampleRate.Valid {
		t.Fatalf("sample rate = %#v, want null without stream sample_rate", params.SampleRate)
	}
	if len(resolved.musicians) != 0 {
		t.Fatalf("musicians = %#v, want none without artist tag", resolved.musicians)
	}
	if resolved.album != nil {
		t.Fatalf("album = %#v, want none without album tag", resolved.album)
	}
}

func TestProcessMusicBatchPersistsGenresAndRelationships(t *testing.T) {
	app := setupTestApp(t)
	defer app.DB.Close()

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
	app.Ffprobe = ffprobeStub

	files := []helpers.ScanFile{
		{Path: firstPath, Ext: "m4a", Size: 5},
		{Path: secondPath, Ext: "m4a", Size: 6},
	}
	scanned, skipped, errCount := app.processMusicBatchForTest(context.Background(), files)
	if scanned != 2 || skipped != 0 || errCount != 0 {
		t.Fatalf("scan result scanned=%d skipped=%d errors=%d, want 2/0/0", scanned, skipped, errCount)
	}

	if got := countMusicScannerRows(t, app.DB, "SELECT COUNT(*) FROM tracks"); got != 2 {
		t.Fatalf("track count = %d, want 2", got)
	}
	if got := countMusicScannerRows(t, app.DB, "SELECT COUNT(*) FROM musicians WHERE name = ?", "Track Artist"); got != 1 {
		t.Fatalf("musician count = %d, want 1", got)
	}
	if got := countMusicScannerRows(t, app.DB, "SELECT COUNT(*) FROM albums WHERE title = ? AND musician = ?", "Shared Album", "Album Artist"); got != 1 {
		t.Fatalf("album count = %d, want 1", got)
	}
	if got := countMusicScannerRows(t, app.DB, "SELECT COUNT(*) FROM genres WHERE tag = ? AND genre_type = ?", "Synth Pop", "music"); got != 1 {
		t.Fatalf("genre count = %d, want 1", got)
	}
	if got := countMusicScannerRows(t, app.DB, `
		SELECT COUNT(*)
		FROM musician_albums AS ma
		INNER JOIN musicians AS m ON m.id = ma.musician_id
		INNER JOIN albums AS a ON a.id = ma.album_id
		WHERE m.name = ? AND a.title = ?
	`, "Track Artist", "Shared Album"); got != 1 {
		t.Fatalf("musician_albums count = %d, want 1", got)
	}
	if got := countMusicScannerRows(t, app.DB, "SELECT COUNT(*) FROM track_genres"); got != 2 {
		t.Fatalf("track_genres count = %d, want 2", got)
	}
	if got := countMusicScannerRows(t, app.DB, "SELECT COUNT(*) FROM musician_genres"); got != 1 {
		t.Fatalf("musician_genres count = %d, want 1", got)
	}
	if got := countMusicScannerRows(t, app.DB, "SELECT COUNT(*) FROM album_genres"); got != 1 {
		t.Fatalf("album_genres count = %d, want 1", got)
	}
}

func TestProcessMusicBatchUpdatesChangedTrackAndReplacesGenre(t *testing.T) {
	app := setupTestApp(t)
	defer app.DB.Close()

	trackPath := filepath.Join(t.TempDir(), "Changing Genre.m4a")
	ffprobeStub := newMusicScannerFfprobeByPath(map[string]*ffprobe.FfprobeResult{
		trackPath: testMusicMetadataWithTags(ffprobe.FormatTags{
			Title:  "Original Title",
			Artist: "Genre Artist",
			Album:  "Genre Album",
			Genre:  "Rock",
		}),
	})
	app.Ffprobe = ffprobeStub

	file := helpers.ScanFile{Path: trackPath, Ext: "m4a", Size: 5}
	scanned, skipped, errCount := app.processMusicBatchForTest(context.Background(), []helpers.ScanFile{file})
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

	scanned, skipped, errCount = app.processMusicBatchForTest(context.Background(), []helpers.ScanFile{file})
	if scanned != 1 || skipped != 0 || errCount != 0 {
		t.Fatalf("second scan result scanned=%d skipped=%d errors=%d, want 1/0/0", scanned, skipped, errCount)
	}

	var title string
	var size int64
	err := app.DB.QueryRow("SELECT title, size FROM tracks WHERE file_path = ?", trackPath).Scan(&title, &size)
	if err != nil {
		t.Fatalf("get updated track: %v", err)
	}
	if title != "Updated Title" || size != 8 {
		t.Fatalf("updated track title/size = %q/%d, want Updated Title/8", title, size)
	}

	if got := countMusicScannerRows(t, app.DB, `
		SELECT COUNT(*)
		FROM track_genres AS tg
		INNER JOIN tracks AS t ON t.id = tg.track_id
		INNER JOIN genres AS g ON g.id = tg.genre_id
		WHERE t.file_path = ? AND g.tag = ?
	`, trackPath, "Jazz"); got != 1 {
		t.Fatalf("Jazz track genre count = %d, want 1", got)
	}
	if got := countMusicScannerRows(t, app.DB, `
		SELECT COUNT(*)
		FROM track_genres AS tg
		INNER JOIN tracks AS t ON t.id = tg.track_id
		INNER JOIN genres AS g ON g.id = tg.genre_id
		WHERE t.file_path = ? AND g.tag = ?
	`, trackPath, "Rock"); got != 0 {
		t.Fatalf("Rock track genre count = %d, want 0", got)
	}
}

func TestProcessMusicBatchClearsTrackGenresWhenGenreRemoved(t *testing.T) {
	app := setupTestApp(t)
	defer app.DB.Close()

	trackPath := filepath.Join(t.TempDir(), "Removed Genre.m4a")
	ffprobeStub := newMusicScannerFfprobeByPath(map[string]*ffprobe.FfprobeResult{
		trackPath: testMusicMetadataWithTags(ffprobe.FormatTags{
			Title:  "Genre Removed",
			Artist: "Genre Artist",
			Album:  "Genre Album",
			Genre:  "Rock",
		}),
	})
	app.Ffprobe = ffprobeStub

	file := helpers.ScanFile{Path: trackPath, Ext: "m4a", Size: 5}
	scanned, skipped, errCount := app.processMusicBatchForTest(context.Background(), []helpers.ScanFile{file})
	if scanned != 1 || skipped != 0 || errCount != 0 {
		t.Fatalf("first scan result scanned=%d skipped=%d errors=%d, want 1/0/0", scanned, skipped, errCount)
	}

	if got := countMusicScannerRows(t, app.DB, `
		SELECT COUNT(*)
		FROM track_genres AS tg
		INNER JOIN tracks AS t ON t.id = tg.track_id
		WHERE t.file_path = ?
	`, trackPath); got != 1 {
		t.Fatalf("initial track genre count = %d, want 1", got)
	}

	ffprobeStub.results[trackPath] = testMusicMetadataWithTags(ffprobe.FormatTags{
		Title:  "Genre Removed",
		Artist: "Genre Artist",
		Album:  "Genre Album",
	})
	file.Size = 8

	scanned, skipped, errCount = app.processMusicBatchForTest(context.Background(), []helpers.ScanFile{file})
	if scanned != 1 || skipped != 0 || errCount != 0 {
		t.Fatalf("second scan result scanned=%d skipped=%d errors=%d, want 1/0/0", scanned, skipped, errCount)
	}

	if got := countMusicScannerRows(t, app.DB, `
		SELECT COUNT(*)
		FROM track_genres AS tg
		INNER JOIN tracks AS t ON t.id = tg.track_id
		WHERE t.file_path = ?
	`, trackPath); got != 0 {
		t.Fatalf("track genre count after genre removal = %d, want 0", got)
	}
}

func TestProcessMusicBatchClearsArtistAlbumAndJoinRowsWhenTagsRemoved(t *testing.T) {
	app := setupTestApp(t)
	defer app.DB.Close()

	trackPath := filepath.Join(t.TempDir(), "Removed Tags.m4a")
	ffprobeStub := newMusicScannerFfprobeByPath(map[string]*ffprobe.FfprobeResult{
		trackPath: testMusicMetadataWithTags(ffprobe.FormatTags{
			Title:  "Removed Tags",
			Artist: "Tagged Artist",
			Album:  "Tagged Album",
			Genre:  "Tagged Genre",
		}),
	})
	app.Ffprobe = ffprobeStub

	file := helpers.ScanFile{Path: trackPath, Ext: "m4a", Size: 5}
	scanned, skipped, errCount := app.processMusicBatchForTest(context.Background(), []helpers.ScanFile{file})
	if scanned != 1 || skipped != 0 || errCount != 0 {
		t.Fatalf("first scan result scanned=%d skipped=%d errors=%d, want 1/0/0", scanned, skipped, errCount)
	}

	if got := countMusicScannerRows(t, app.DB, `
		SELECT COUNT(*)
		FROM track_musicians AS tm
		INNER JOIN tracks AS t ON t.id = tm.track_id
		WHERE t.file_path = ?
	`, trackPath); got != 1 {
		t.Fatalf("initial track_musicians count = %d, want 1", got)
	}
	if got := countMusicScannerRows(t, app.DB, `
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

	scanned, skipped, errCount = app.processMusicBatchForTest(context.Background(), []helpers.ScanFile{file})
	if scanned != 1 || skipped != 0 || errCount != 0 {
		t.Fatalf("second scan result scanned=%d skipped=%d errors=%d, want 1/0/0", scanned, skipped, errCount)
	}

	var musicianID sql.NullInt64
	var albumID sql.NullInt64
	err := app.DB.QueryRow("SELECT musician_id, album_id FROM tracks WHERE file_path = ?", trackPath).Scan(&musicianID, &albumID)
	if err != nil {
		t.Fatalf("get rescanned track relationships: %v", err)
	}
	if musicianID.Valid {
		t.Fatalf("track musician_id = %#v, want null after artist tag removal", musicianID)
	}
	if albumID.Valid {
		t.Fatalf("track album_id = %#v, want null after album tag removal", albumID)
	}

	if got := countMusicScannerRows(t, app.DB, `
		SELECT COUNT(*)
		FROM track_musicians AS tm
		INNER JOIN tracks AS t ON t.id = tm.track_id
		WHERE t.file_path = ?
	`, trackPath); got != 0 {
		t.Fatalf("track_musicians count after tag removal = %d, want 0", got)
	}
	if got := countMusicScannerRows(t, app.DB, `
		SELECT COUNT(*)
		FROM track_genres AS tg
		INNER JOIN tracks AS t ON t.id = tg.track_id
		WHERE t.file_path = ?
	`, trackPath); got != 0 {
		t.Fatalf("track_genres count after tag removal = %d, want 0", got)
	}
}

func TestProcessMusicBatchDoesNotMergeFailedPersistIntoScanContext(t *testing.T) {
	app := setupTestApp(t)
	defer app.DB.Close()

	dir := t.TempDir()
	badPath := filepath.Join(dir, "Bad Cache Track.m4a")
	goodPath := filepath.Join(dir, "Good Cache Track.m4a")
	escapedBadPath := strings.ReplaceAll(badPath, "'", "''")
	_, err := app.DB.Exec(fmt.Sprintf(`CREATE TRIGGER fail_bad_cache_track BEFORE INSERT ON tracks
		WHEN new.file_path = '%s'
		BEGIN
			SELECT RAISE(ABORT, 'forced track failure');
		END;`, escapedBadPath))
	if err != nil {
		t.Fatalf("create failing trigger: %v", err)
	}

	app.Ffprobe = newMusicScannerFfprobeByPath(map[string]*ffprobe.FfprobeResult{
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

	files := []helpers.ScanFile{
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
	if got := countMusicScannerRows(t, app.DB, "SELECT COUNT(*) FROM tracks WHERE file_path = ?", badPath); got != 0 {
		t.Fatalf("bad track count = %d, want 0", got)
	}
	if got := countMusicScannerRows(t, app.DB, "SELECT COUNT(*) FROM tracks WHERE file_path = ?", goodPath); got != 1 {
		t.Fatalf("good track count = %d, want 1", got)
	}
}

func TestProcessMusicBatchSplitsCompoundArtistsIntoTrackMusicians(t *testing.T) {
	app := setupTestApp(t)
	defer app.DB.Close()

	trackPath := filepath.Join(t.TempDir(), "Compound Artists.m4a")
	app.Ffprobe = newMusicScannerFfprobeByPath(map[string]*ffprobe.FfprobeResult{
		trackPath: testMusicMetadataWithTags(ffprobe.FormatTags{
			Title:  "Compound Artists",
			Artist: "Artist One feat. Artist Two; Artist One",
			Album:  "Compound Album",
			Genre:  "Indie",
		}),
	})

	scanned, skipped, errCount := app.processMusicBatchForTest(context.Background(), []helpers.ScanFile{
		{Path: trackPath, Ext: "m4a", Size: 5},
	})
	if scanned != 1 || skipped != 0 || errCount != 0 {
		t.Fatalf("scan result scanned=%d skipped=%d errors=%d, want 1/0/0", scanned, skipped, errCount)
	}

	if got := countMusicScannerRows(t, app.DB, "SELECT COUNT(*) FROM musicians"); got != 2 {
		t.Fatalf("musician count = %d, want 2 deduplicated split credits", got)
	}
	if got := countMusicScannerRows(t, app.DB, "SELECT COUNT(*) FROM musicians WHERE name IN (?, ?)", "Artist One", "Artist Two"); got != 2 {
		t.Fatalf("split musician count = %d, want 2", got)
	}

	var primaryArtist string
	err := app.DB.QueryRow(`
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

	if got := countMusicScannerRows(t, app.DB, `
		SELECT COUNT(*)
		FROM track_musicians AS tm
		INNER JOIN tracks AS t ON t.id = tm.track_id
		INNER JOIN musicians AS m ON m.id = tm.musician_id
		WHERE t.file_path = ? AND m.name IN (?, ?)
	`, trackPath, "Artist One", "Artist Two"); got != 2 {
		t.Fatalf("track_musicians split artist count = %d, want 2", got)
	}

	if got := countMusicScannerRows(t, app.DB, `
		SELECT COUNT(*)
		FROM musician_albums AS ma
		INNER JOIN musicians AS m ON m.id = ma.musician_id
		INNER JOIN albums AS a ON a.id = ma.album_id
		WHERE a.title = ? AND m.name IN (?, ?)
	`, "Compound Album", "Artist One", "Artist Two"); got != 2 {
		t.Fatalf("musician_albums split artist count = %d, want 2", got)
	}
}

func TestSplitArtistCredits(t *testing.T) {
	tests := []struct {
		name      string
		artistTag string
		want      []string
	}{
		{"feat with dot", "Artist One feat. Artist Two", []string{"Artist One", "Artist Two"}},
		{"feat case-insensitive", "Artist One FEAT. Artist Two", []string{"Artist One", "Artist Two"}},
		{"ft without dot", "Artist One ft Artist Two", []string{"Artist One", "Artist Two"}},
		{"featuring", "Jay-Z featuring Alicia Keys", []string{"Jay-Z", "Alicia Keys"}},
		{"with", "Santana with Rob Thomas", []string{"Santana", "Rob Thomas"}},
		{"vs with dot", "Daft Punk vs. Queen", []string{"Daft Punk", "Queen"}},
		{"semicolon", "David Guetta; Sia", []string{"David Guetta", "Sia"}},
		{"spaced slash", "Nujabes / Fat Jon", []string{"Nujabes", "Fat Jon"}},
		{"parenthesized feat splits", "Beyoncé (feat. JAY-Z)", []string{"Beyoncé", "JAY-Z"}},
		{"paren cut leftover is cleaned", "Beyoncé ( feat. JAY-Z)", []string{"Beyoncé", "JAY-Z"}},
		{"duplicate credits deduplicate", "Drake feat. Drake", []string{"Drake"}},
		{"unspaced slash stays combined", "AC/DC", []string{"AC/DC"}},
		{"ampersand stays combined", "Tom Petty & The Heartbreakers", []string{"Tom Petty & The Heartbreakers"}},
		{"comma and ampersand stay combined", "Earth, Wind & Fire", []string{"Earth, Wind & Fire"}},
		{
			"hamilton cast credit stays combined",
			"Anthony Ramos, Okieriete Onaodowan, Daveed Diggs, Lin-Manuel Miranda & Leslie Odom, Jr.",
			[]string{"Anthony Ramos, Okieriete Onaodowan, Daveed Diggs, Lin-Manuel Miranda & Leslie Odom, Jr."},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := splitArtistCredits(tt.artistTag)
			if !slices.Equal(got, tt.want) {
				t.Fatalf("splitArtistCredits(%q) = %#v, want %#v", tt.artistTag, got, tt.want)
			}
		})
	}
}

func TestProcessMusicBatchKeepsCommaAndAmpersandArtistsCombined(t *testing.T) {
	app := setupTestApp(t)
	defer app.DB.Close()

	dir := t.TempDir()
	combinedArtists := []string{
		"Brooks & Dunn",
		"Earth, Wind & Fire",
		"Tom Petty & The Heartbreakers",
	}
	results := make(map[string]*ffprobe.FfprobeResult, len(combinedArtists))
	files := make([]helpers.ScanFile, 0, len(combinedArtists))
	for i, artist := range combinedArtists {
		trackPath := filepath.Join(dir, fmt.Sprintf("Combined %d.m4a", i+1))
		results[trackPath] = testMusicMetadataWithTags(ffprobe.FormatTags{
			Title:  fmt.Sprintf("Combined %d", i+1),
			Artist: artist,
		})
		files = append(files, helpers.ScanFile{Path: trackPath, Ext: "m4a", Size: int64(5 + i)})
	}
	app.Ffprobe = newMusicScannerFfprobeByPath(results)

	scanned, skipped, errCount := app.processMusicBatchForTest(context.Background(), files)
	if scanned != 3 || skipped != 0 || errCount != 0 {
		t.Fatalf("scan result scanned=%d skipped=%d errors=%d, want 3/0/0", scanned, skipped, errCount)
	}

	if got := countMusicScannerRows(t, app.DB, "SELECT COUNT(*) FROM musicians"); got != 3 {
		t.Fatalf("musician count = %d, want 3 combined acts", got)
	}
	for _, artist := range combinedArtists {
		if got := countMusicScannerRows(t, app.DB, "SELECT COUNT(*) FROM musicians WHERE name = ?", artist); got != 1 {
			t.Fatalf("musician count for %q = %d, want 1 combined row", artist, got)
		}
	}
	if got := countMusicScannerRows(t, app.DB, "SELECT COUNT(*) FROM musicians WHERE name IN (?, ?, ?, ?)",
		"Brooks", "Dunn", "Tom Petty", "The Heartbreakers"); got != 0 {
		t.Fatalf("split fragment musician count = %d, want 0", got)
	}
}

func TestProcessMusicBatchRemovesStaleTrackMusiciansOnRescan(t *testing.T) {
	app := setupTestApp(t)
	defer app.DB.Close()

	trackPath := filepath.Join(t.TempDir(), "Changed Artist.m4a")
	ffprobeStub := newMusicScannerFfprobeByPath(map[string]*ffprobe.FfprobeResult{
		trackPath: testMusicMetadataWithTags(ffprobe.FormatTags{
			Title:  "Changed Artist",
			Artist: "Artist One feat. Artist Two",
		}),
	})
	app.Ffprobe = ffprobeStub

	scanned, skipped, errCount := app.processMusicBatchForTest(context.Background(), []helpers.ScanFile{
		{Path: trackPath, Ext: "m4a", Size: 5},
	})
	if scanned != 1 || skipped != 0 || errCount != 0 {
		t.Fatalf("first scan result scanned=%d skipped=%d errors=%d, want 1/0/0", scanned, skipped, errCount)
	}

	ffprobeStub.results[trackPath] = testMusicMetadataWithTags(ffprobe.FormatTags{
		Title:  "Changed Artist",
		Artist: "Solo Artist",
	})

	scanned, skipped, errCount = app.processMusicBatchForTest(context.Background(), []helpers.ScanFile{
		{Path: trackPath, Ext: "m4a", Size: 8},
	})
	if scanned != 1 || skipped != 0 || errCount != 0 {
		t.Fatalf("second scan result scanned=%d skipped=%d errors=%d, want 1/0/0", scanned, skipped, errCount)
	}

	if got := countMusicScannerRows(t, app.DB, `
		SELECT COUNT(*)
		FROM track_musicians AS tm
		INNER JOIN tracks AS t ON t.id = tm.track_id
		INNER JOIN musicians AS m ON m.id = tm.musician_id
		WHERE t.file_path = ? AND m.name = ?
	`, trackPath, "Solo Artist"); got != 1 {
		t.Fatalf("solo track_musicians count = %d, want 1", got)
	}
	if got := countMusicScannerRows(t, app.DB, `
		SELECT COUNT(*)
		FROM track_musicians AS tm
		INNER JOIN tracks AS t ON t.id = tm.track_id
		INNER JOIN musicians AS m ON m.id = tm.musician_id
		WHERE t.file_path = ? AND m.name IN (?, ?)
	`, trackPath, "Artist One", "Artist Two"); got != 0 {
		t.Fatalf("stale split track_musicians count = %d, want 0", got)
	}
}

func TestAlbumIdentityKey(t *testing.T) {
	tests := []struct {
		name          string
		title         string
		albumArtist   string
		isCompilation bool
		want          string
	}{
		{"plain title and artist", "Renaissance", "Beyoncé", false, "renaissance\x1fbeyonce"},
		{"case and diacritic variant collapses", "RENAISSANCE", "Beyonce", false, "renaissance\x1fbeyonce"},
		{"ampersand spelled as and", "Rock & Roll", "AC/DC", false, "rock and roll\x1fac dc"},
		{"punctuation collapses", "Sgt. Pepper's!!", "The Beatles", false, "sgt pepper s\x1fthe beatles"},
		{"punctuation variant collapses", "Sgt Pepper's", "The Beatles", false, "sgt pepper s\x1fthe beatles"},
		{"compilation uses various artists sentinel", "Now 100", "ignored", true, "now 100\x1fvarious artists"},
		{"lead credit only", "Hamilton", "Lin-Manuel Miranda", false, "hamilton\x1flin manuel miranda"},
		{
			"full cast credit cuts to lead credit",
			"Hamilton",
			"Lin-Manuel Miranda, Leslie Odom, Jr. & Christopher Jackson",
			false,
			"hamilton\x1flin manuel miranda",
		},
		{"punctuation-only names keep raw fallback", "!!!", "!!!", false, "!!!\x1f!!!"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := albumIdentityKey(tt.title, tt.albumArtist, tt.isCompilation)
			if got != tt.want {
				t.Fatalf("albumIdentityKey(%q, %q, %t) = %q, want %q", tt.title, tt.albumArtist, tt.isCompilation, got, tt.want)
			}
		})
	}
}

func TestProcessMusicBatchGroupsCompoundAlbumArtistSpellingsIntoOneAlbum(t *testing.T) {
	app := setupTestApp(t)
	defer app.DB.Close()

	dir := t.TempDir()
	firstPath := filepath.Join(dir, "Alexander Hamilton.m4a")
	secondPath := filepath.Join(dir, "Aaron Burr, Sir.m4a")
	albumTitle := "Hamilton (Original Broadway Cast Recording)"
	app.Ffprobe = newMusicScannerFfprobeByPath(map[string]*ffprobe.FfprobeResult{
		firstPath: testMusicMetadataWithTags(ffprobe.FormatTags{
			Title:       "Alexander Hamilton",
			Artist:      "Lin-Manuel Miranda",
			AlbumArtist: "Lin-Manuel Miranda",
			Album:       albumTitle,
			Track:       "1/46",
		}),
		secondPath: testMusicMetadataWithTags(ffprobe.FormatTags{
			Title:       "Aaron Burr, Sir",
			Artist:      "Leslie Odom, Jr.",
			AlbumArtist: "Lin-Manuel Miranda, Leslie Odom, Jr. & Christopher Jackson",
			Album:       albumTitle,
			Track:       "2/46",
		}),
	})

	scanned, skipped, errCount := app.processMusicBatchForTest(context.Background(), []helpers.ScanFile{
		{Path: firstPath, Ext: "m4a", Size: 5},
		{Path: secondPath, Ext: "m4a", Size: 6},
	})
	if scanned != 2 || skipped != 0 || errCount != 0 {
		t.Fatalf("scan result scanned=%d skipped=%d errors=%d, want 2/0/0", scanned, skipped, errCount)
	}

	if got := countMusicScannerRows(t, app.DB, "SELECT COUNT(*) FROM albums"); got != 1 {
		t.Fatalf("album count = %d, want 1 shared cast recording", got)
	}

	var musician sql.NullString
	var isCompilation bool
	err := app.DB.QueryRow("SELECT musician, is_compilation FROM albums WHERE title = ?", albumTitle).Scan(&musician, &isCompilation)
	if err != nil {
		t.Fatalf("get shared album: %v", err)
	}
	if !musician.Valid || musician.String != "Lin-Manuel Miranda" {
		t.Fatalf("album musician = %#v, want first-writer Lin-Manuel Miranda", musician)
	}
	if isCompilation {
		t.Fatal("cast recording flagged as compilation, want regular album")
	}

	if got := countMusicScannerRows(t, app.DB, "SELECT COUNT(*) FROM tracks WHERE album_id = (SELECT id FROM albums WHERE title = ?)", albumTitle); got != 2 {
		t.Fatalf("tracks linked to shared album = %d, want 2", got)
	}
}

func TestProcessMusicBatchGroupsVariousArtistsCompilationIntoOneAlbum(t *testing.T) {
	app := setupTestApp(t)
	defer app.DB.Close()

	dir := t.TempDir()
	albumTitle := "Now That's What I Call Music"
	artists := []string{"Artist A", "Artist B", "Artist C"}
	results := make(map[string]*ffprobe.FfprobeResult, len(artists))
	files := make([]helpers.ScanFile, 0, len(artists))
	for i, artist := range artists {
		trackPath := filepath.Join(dir, fmt.Sprintf("Compilation %d.m4a", i+1))
		results[trackPath] = testMusicMetadataWithTags(ffprobe.FormatTags{
			Title:       fmt.Sprintf("Compilation Track %d", i+1),
			Artist:      artist,
			Album:       albumTitle,
			Compilation: "1",
			Track:       fmt.Sprintf("%d/3", i+1),
		})
		files = append(files, helpers.ScanFile{Path: trackPath, Ext: "m4a", Size: int64(5 + i)})
	}
	app.Ffprobe = newMusicScannerFfprobeByPath(results)

	scanned, skipped, errCount := app.processMusicBatchForTest(context.Background(), files)
	if scanned != 3 || skipped != 0 || errCount != 0 {
		t.Fatalf("scan result scanned=%d skipped=%d errors=%d, want 3/0/0", scanned, skipped, errCount)
	}

	if got := countMusicScannerRows(t, app.DB, "SELECT COUNT(*) FROM albums"); got != 1 {
		t.Fatalf("album count = %d, want 1 shared compilation", got)
	}

	var musician sql.NullString
	var isCompilation bool
	var albumArtistID sql.NullInt64
	err := app.DB.QueryRow("SELECT musician, is_compilation, album_artist_id FROM albums WHERE title = ?", albumTitle).
		Scan(&musician, &isCompilation, &albumArtistID)
	if err != nil {
		t.Fatalf("get compilation album: %v", err)
	}
	if !isCompilation {
		t.Fatal("compilation flag not set on Various Artists album")
	}
	if !musician.Valid || musician.String != "Various Artists" {
		t.Fatalf("album musician = %#v, want Various Artists", musician)
	}
	if albumArtistID.Valid {
		t.Fatalf("album_artist_id = %#v, want null for Various Artists compilation", albumArtistID)
	}

	if got := countMusicScannerRows(t, app.DB, "SELECT COUNT(*) FROM musicians"); got != 3 {
		t.Fatalf("musician count = %d, want 3 individual artists", got)
	}
	if got := countMusicScannerRows(t, app.DB, "SELECT COUNT(*) FROM tracks WHERE album_id = (SELECT id FROM albums WHERE title = ?)", albumTitle); got != 3 {
		t.Fatalf("tracks linked to compilation = %d, want 3", got)
	}
}

func TestProcessMusicBatchKeepsSingleArtistCompilationUnderArtist(t *testing.T) {
	app := setupTestApp(t)
	defer app.DB.Close()

	trackPath := filepath.Join(t.TempDir(), "Greatest Hits Track.m4a")
	app.Ffprobe = newMusicScannerFfprobeByPath(map[string]*ffprobe.FfprobeResult{
		trackPath: testMusicMetadataWithTags(ffprobe.FormatTags{
			Title:       "Every Light in the House",
			Artist:      "Trace Adkins",
			AlbumArtist: "Trace Adkins",
			Album:       "Greatest Hits Collection",
			Compilation: "1",
		}),
	})

	scanned, skipped, errCount := app.processMusicBatchForTest(context.Background(), []helpers.ScanFile{
		{Path: trackPath, Ext: "m4a", Size: 5},
	})
	if scanned != 1 || skipped != 0 || errCount != 0 {
		t.Fatalf("scan result scanned=%d skipped=%d errors=%d, want 1/0/0", scanned, skipped, errCount)
	}

	var albumKey string
	var musician sql.NullString
	var isCompilation bool
	var albumArtistID sql.NullInt64
	err := app.DB.QueryRow("SELECT album_key, musician, is_compilation, album_artist_id FROM albums WHERE title = ?", "Greatest Hits Collection").
		Scan(&albumKey, &musician, &isCompilation, &albumArtistID)
	if err != nil {
		t.Fatalf("get greatest hits album: %v", err)
	}
	if isCompilation {
		t.Fatal("single-artist greatest hits flagged as Various Artists compilation")
	}
	if !musician.Valid || musician.String != "Trace Adkins" {
		t.Fatalf("album musician = %#v, want Trace Adkins", musician)
	}
	wantKey := albumIdentityKey("Greatest Hits Collection", "Trace Adkins", false)
	if albumKey != wantKey {
		t.Fatalf("album key = %q, want artist-scoped %q", albumKey, wantKey)
	}
	if !albumArtistID.Valid {
		t.Fatal("album_artist_id is null, want link to Trace Adkins")
	}

	var linkedArtist string
	err = app.DB.QueryRow("SELECT name FROM musicians WHERE id = ?", albumArtistID.Int64).Scan(&linkedArtist)
	if err != nil {
		t.Fatalf("get linked album artist: %v", err)
	}
	if linkedArtist != "Trace Adkins" {
		t.Fatalf("linked album artist = %q, want Trace Adkins", linkedArtist)
	}
}

func TestProcessMusicBatchDedupesMusiciansByNameKey(t *testing.T) {
	app := setupTestApp(t)
	defer app.DB.Close()

	dir := t.TempDir()
	firstPath := filepath.Join(dir, "Halo.m4a")
	secondPath := filepath.Join(dir, "Formation.m4a")
	app.Ffprobe = newMusicScannerFfprobeByPath(map[string]*ffprobe.FfprobeResult{
		firstPath: testMusicMetadataWithTags(ffprobe.FormatTags{
			Title:  "Halo",
			Artist: "Beyoncé",
		}),
		secondPath: testMusicMetadataWithTags(ffprobe.FormatTags{
			Title:  "Formation",
			Artist: "Beyonce",
		}),
	})

	// Two separate batches, so the second spelling reaches the DB upsert instead
	// of the per-scan cache: dedupe must hold at the name_key constraint itself.
	scanned, skipped, errCount := app.processMusicBatchForTest(context.Background(), []helpers.ScanFile{
		{Path: firstPath, Ext: "m4a", Size: 5},
	})
	if scanned != 1 || skipped != 0 || errCount != 0 {
		t.Fatalf("first scan result scanned=%d skipped=%d errors=%d, want 1/0/0", scanned, skipped, errCount)
	}

	scanned, skipped, errCount = app.processMusicBatchForTest(context.Background(), []helpers.ScanFile{
		{Path: secondPath, Ext: "m4a", Size: 6},
	})
	if scanned != 1 || skipped != 0 || errCount != 0 {
		t.Fatalf("second scan result scanned=%d skipped=%d errors=%d, want 1/0/0", scanned, skipped, errCount)
	}

	if got := countMusicScannerRows(t, app.DB, "SELECT COUNT(*) FROM musicians"); got != 1 {
		t.Fatalf("musician count = %d, want 1 deduplicated row", got)
	}

	var name string
	err := app.DB.QueryRow("SELECT name FROM musicians WHERE name_key = ?", "beyonce").Scan(&name)
	if err != nil {
		t.Fatalf("get deduplicated musician: %v", err)
	}
	if name != "Beyoncé" {
		t.Fatalf("musician name = %q, want first scanned spelling Beyoncé", name)
	}

	if got := countMusicScannerRows(t, app.DB, `
		SELECT COUNT(*)
		FROM track_musicians AS tm
		INNER JOIN musicians AS m ON m.id = tm.musician_id
		WHERE m.name_key = ?
	`, "beyonce"); got != 2 {
		t.Fatalf("tracks credited to deduplicated musician = %d, want 2", got)
	}
}

func TestProcessMusicBatchRefreshesSortTagsOnRescan(t *testing.T) {
	app := setupTestApp(t)
	defer app.DB.Close()

	trackPath := filepath.Join(t.TempDir(), "Refresh Track.m4a")
	ffprobeStub := newMusicScannerFfprobeByPath(map[string]*ffprobe.FfprobeResult{
		trackPath: testMusicMetadataWithTags(ffprobe.FormatTags{
			Title:      "Refresh Track",
			Artist:     "Refresh Artist",
			SortArtist: "Artist, Refresh",
			Album:      "Refresh Album",
			SortAlbum:  "Refresh Album, The",
		}),
	})
	app.Ffprobe = ffprobeStub

	scanned, skipped, errCount := app.processMusicBatchForTest(context.Background(), []helpers.ScanFile{
		{Path: trackPath, Ext: "m4a", Size: 5},
	})
	if scanned != 1 || skipped != 0 || errCount != 0 {
		t.Fatalf("first scan result scanned=%d skipped=%d errors=%d, want 1/0/0", scanned, skipped, errCount)
	}

	// Same identity keys, retagged sort names and a different artist spelling.
	ffprobeStub.results[trackPath] = testMusicMetadataWithTags(ffprobe.FormatTags{
		Title:      "Refresh Track",
		Artist:     "REFRESH ARTIST",
		SortArtist: "Artist, Refreshed",
		Album:      "Refresh Album",
		SortAlbum:  "Refresh Album, A",
	})

	scanned, skipped, errCount = app.processMusicBatchForTest(context.Background(), []helpers.ScanFile{
		{Path: trackPath, Ext: "m4a", Size: 8},
	})
	if scanned != 1 || skipped != 0 || errCount != 0 {
		t.Fatalf("second scan result scanned=%d skipped=%d errors=%d, want 1/0/0", scanned, skipped, errCount)
	}

	if got := countMusicScannerRows(t, app.DB, "SELECT COUNT(*) FROM musicians"); got != 1 {
		t.Fatalf("musician count = %d, want 1", got)
	}

	var musicianName string
	var sortName string
	err := app.DB.QueryRow("SELECT name, sort_name FROM musicians WHERE name_key = ?", "refresh artist").Scan(&musicianName, &sortName)
	if err != nil {
		t.Fatalf("get rescanned musician: %v", err)
	}
	if sortName != "Artist, Refreshed" {
		t.Fatalf("musician sort_name = %q, want refreshed Artist, Refreshed", sortName)
	}
	if musicianName != "Refresh Artist" {
		t.Fatalf("musician name = %q, want unchanged first spelling Refresh Artist", musicianName)
	}

	var sortTitle string
	err = app.DB.QueryRow("SELECT sort_title FROM albums WHERE title = ?", "Refresh Album").Scan(&sortTitle)
	if err != nil {
		t.Fatalf("get rescanned album: %v", err)
	}
	if sortTitle != "Refresh Album, A" {
		t.Fatalf("album sort_title = %q, want refreshed Refresh Album, A", sortTitle)
	}
}

func TestResolveTrackMusicians(t *testing.T) {
	tests := []struct {
		name       string
		artistTag  string
		sortArtist string
		artistMBID string
		want       []resolvedMusician
	}{
		{
			name:       "single credit claims valid mbid",
			artistTag:  "Beyoncé",
			sortArtist: "Knowles, Beyoncé",
			artistMBID: "859D0860-D480-4EFD-970C-C05D5F1882B9",
			want: []resolvedMusician{
				{name: "Beyoncé", sortName: "Knowles, Beyoncé", nameKey: "beyonce", mbArtistID: "859d0860-d480-4efd-970c-c05d5f1882b9"},
			},
		},
		{
			name:      "single credit without sort artist falls back to name",
			artistTag: "Adele",
			want: []resolvedMusician{
				{name: "Adele", sortName: "Adele", nameKey: "adele"},
			},
		},
		{
			name:       "compound credit never claims the mbid",
			artistTag:  "Artist One feat. Artist Two",
			sortArtist: "One, Artist",
			artistMBID: "859d0860-d480-4efd-970c-c05d5f1882b9",
			want: []resolvedMusician{
				{name: "Artist One", sortName: "Artist One", nameKey: "artist one"},
				{name: "Artist Two", sortName: "Artist Two", nameKey: "artist two"},
			},
		},
		{
			name:       "invalid mbid is dropped",
			artistTag:  "Adele",
			artistMBID: "not-a-musicbrainz-id",
			want: []resolvedMusician{
				{name: "Adele", sortName: "Adele", nameKey: "adele"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := resolveTrackMusicians(tt.artistTag, tt.sortArtist, tt.artistMBID)
			if !slices.Equal(got, tt.want) {
				t.Fatalf("resolveTrackMusicians(%q, %q, %q) = %#v, want %#v",
					tt.artistTag, tt.sortArtist, tt.artistMBID, got, tt.want)
			}
		})
	}
}

func TestParseTrackTotal(t *testing.T) {
	tests := []struct {
		name     string
		totalTag string
		trackTag string
		want     int64
	}{
		{"total tag wins", "10", "", 10},
		{"total tag beats track fraction", "12", "3/10", 12},
		{"track fraction fallback", "", "3/12", 12},
		{"plain track number has no total", "", "5", 0},
		{"garbage yields zero", "garbage", "junk", 0},
		{"empty tags yield zero", "", "", 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseTrackTotal(tt.totalTag, tt.trackTag)
			if got != tt.want {
				t.Fatalf("parseTrackTotal(%q, %q) = %d, want %d", tt.totalTag, tt.trackTag, got, tt.want)
			}
		})
	}
}
