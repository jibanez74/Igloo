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
	"time"

	"igloo/cmd/internal/database"
	"igloo/cmd/internal/ffprobe"
	spotifyapi "igloo/cmd/internal/spotify"

	spotifylib "github.com/zmb3/spotify/v2"
)

func testMusicMetadata() *ffprobe.FfprobeResult {
	return &ffprobe.FfprobeResult{
		Format: ffprobe.Format{
			Duration:   "180",
			Size:       "5",
			BitRate:    "256000",
			FormatName: "mov,mp4,m4a,3gp,3g2,mj2",
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
			Duration:   "180.250",
			Size:       "5",
			BitRate:    "256000",
			FormatName: "mov,mp4,m4a,3gp,3g2,mj2",
			Tags:       tags,
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
	result *ffprobe.FfprobeResult
	calls  int
}

func (s *countingMusicScannerFfprobe) GetMetadata(filePath string) (*ffprobe.FfprobeResult, error) {
	s.calls++
	return s.result, nil
}

func (s *countingMusicScannerFfprobe) GetAudioMetadata(filePath string) (*ffprobe.FfprobeResult, error) {
	s.calls++
	return s.result, nil
}

type failingPathMusicScannerFfprobe struct {
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

func (s *failingPathMusicScannerFfprobe) GetAudioMetadata(filePath string) (*ffprobe.FfprobeResult, error) {
	s.calls++
	if filePath == s.failingPath {
		return nil, errors.New("ffprobe failed")
	}

	return s.result, nil
}

type musicScannerFfprobeByPath struct {
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

func (s *musicScannerFfprobeByPath) GetAudioMetadata(filePath string) (*ffprobe.FfprobeResult, error) {
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

type musicScannerSpotifyStub struct {
	artist      *spotifylib.FullArtist
	artistErr   error
	artistCalls int
	album       *spotifylib.FullAlbum
	albumErr    error
	albumCalls  int
	clearCalls  int
}

func (s *musicScannerSpotifyStub) SearchArtistByName(_ context.Context, _ string) (*spotifylib.FullArtist, error) {
	s.artistCalls++
	if s.artistErr != nil {
		return nil, s.artistErr
	}

	return s.artist, nil
}

func (s *musicScannerSpotifyStub) SearchAndGetAlbumDetails(_ context.Context, _, _ string) (*spotifylib.FullAlbum, error) {
	s.albumCalls++
	if s.albumErr != nil {
		return nil, s.albumErr
	}

	return s.album, nil
}

func (s *musicScannerSpotifyStub) ClearAllCaches() {
	s.clearCalls++
}

func runMusicScanForTest(t *testing.T, app *Application) {
	t.Helper()

	finishMusicScan()
	if !tryBeginMusicScan() {
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

	file := trackFile{
		path: filepath.Join(t.TempDir(), "Test Track.m4a"),
		ext:  "m4a",
		size: 5,
	}

	scanned, skipped, errCount := app.processMusicBatch(context.Background(), []trackFile{file})
	if scanned != 1 || skipped != 0 || errCount != 0 {
		t.Fatalf("first scan result scanned=%d skipped=%d errors=%d, want 1/0/0", scanned, skipped, errCount)
	}

	var trackCount int
	err := app.DB.QueryRow("SELECT COUNT(*) FROM tracks WHERE file_path = ? AND size = ?", file.path, file.size).Scan(&trackCount)
	if err != nil {
		t.Fatalf("count tracks: %v", err)
	}
	if trackCount != 1 {
		t.Fatalf("track count = %d, want 1", trackCount)
	}
	if ffprobeStub.calls != 1 {
		t.Fatalf("ffprobe calls = %d, want 1", ffprobeStub.calls)
	}

	scanned, skipped, errCount = app.processMusicBatch(context.Background(), []trackFile{file})
	if scanned != 0 || skipped != 1 || errCount != 0 {
		t.Fatalf("second scan result scanned=%d skipped=%d errors=%d, want 0/1/0", scanned, skipped, errCount)
	}
	if ffprobeStub.calls != 1 {
		t.Fatalf("ffprobe calls after skip = %d, want 1", ffprobeStub.calls)
	}

	changedFile := file
	changedFile.size = 6
	scanned, skipped, errCount = app.processMusicBatch(context.Background(), []trackFile{changedFile})
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
	files := []trackFile{
		{
			path: filepath.Join(dir, "Track One.m4a"),
			ext:  "m4a",
			size: 5,
		},
		{
			path: filepath.Join(dir, "Track Two.m4a"),
			ext:  "m4a",
			size: 6,
		},
	}

	scanned, skipped, errCount := app.processMusicBatch(context.Background(), files)
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

	files := []trackFile{
		{
			path: badPath,
			ext:  "m4a",
			size: 5,
		},
		{
			path: goodPath,
			ext:  "m4a",
			size: 6,
		},
	}

	scanned, skipped, errCount := app.processMusicBatch(context.Background(), files)
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

	files := []trackFile{
		{
			path: badPath,
			ext:  "m4a",
			size: 5,
		},
		{
			path: goodPath,
			ext:  "m4a",
			size: 6,
		},
	}

	scanned, skipped, errCount := app.processMusicBatch(context.Background(), files)
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
	finishMusicScan()

	if !tryBeginMusicScan() {
		t.Fatal("first music scan guard acquisition failed")
	}
	defer finishMusicScan()

	if tryBeginMusicScan() {
		t.Fatal("second music scan guard acquisition succeeded, want blocked")
	}

	finishMusicScan()
	if !tryBeginMusicScan() {
		t.Fatal("music scan guard did not reset after finish")
	}
}

func TestProcessMusicBatchAssignsFirstSpotifyImages(t *testing.T) {
	app := setupTestApp(t)
	defer app.DB.Close()

	app.Ffprobe = &countingMusicScannerFfprobe{result: testMusicMetadata()}
	app.Spotify = &musicScannerSpotifyStub{
		artist: &spotifylib.FullArtist{
			SimpleArtist: spotifylib.SimpleArtist{
				ID:   spotifylib.ID("artist123"),
				Name: "Test Artist",
			},
			Images: []spotifylib.Image{
				{URL: "https://i.scdn.co/artist-first.jpg"},
				{URL: "https://i.scdn.co/artist-second.jpg"},
			},
		},
		album: &spotifylib.FullAlbum{
			SimpleAlbum: spotifylib.SimpleAlbum{
				ID:          spotifylib.ID("album123"),
				Name:        "Test Album",
				TotalTracks: 10,
				Images: []spotifylib.Image{
					{URL: "https://i.scdn.co/album-first.jpg"},
					{URL: "https://i.scdn.co/album-second.jpg"},
				},
			},
		},
	}

	file := trackFile{
		path: filepath.Join(t.TempDir(), "Test Track.m4a"),
		ext:  "m4a",
		size: 5,
	}

	scanned, skipped, errCount := app.processMusicBatch(context.Background(), []trackFile{file})
	if scanned != 1 || skipped != 0 || errCount != 0 {
		t.Fatalf("scan result scanned=%d skipped=%d errors=%d, want 1/0/0", scanned, skipped, errCount)
	}

	var albumCover sql.NullString
	err := app.DB.QueryRow("SELECT cover FROM albums WHERE spotify_id = ?", "album123").Scan(&albumCover)
	if err != nil {
		t.Fatalf("get album cover: %v", err)
	}
	if !albumCover.Valid || albumCover.String != "https://i.scdn.co/album-first.jpg" {
		t.Fatalf("album cover = %#v, want first Spotify album image", albumCover)
	}

	var musicianThumb sql.NullString
	err = app.DB.QueryRow("SELECT thumb FROM musicians WHERE spotify_id = ?", "artist123").Scan(&musicianThumb)
	if err != nil {
		t.Fatalf("get musician thumb: %v", err)
	}
	if !musicianThumb.Valid || musicianThumb.String != "https://i.scdn.co/artist-first.jpg" {
		t.Fatalf("musician thumb = %#v, want first Spotify artist image", musicianThumb)
	}
}

func TestProcessMusicBatchRefreshesExistingSpotifyImages(t *testing.T) {
	app := setupTestApp(t)
	defer app.DB.Close()

	seededMusician, err := app.Queries.UpsertMusician(context.Background(), database.UpsertMusicianParams{
		Name:      "Existing Artist",
		SortName:  "Existing Artist",
		SpotifyID: sql.NullString{String: "artist123", Valid: true},
		Thumb:     sql.NullString{String: "file:///music/artist.jpg", Valid: true},
	})
	if err != nil {
		t.Fatalf("seed musician: %v", err)
	}

	seededAlbum, err := app.Queries.UpsertAlbum(context.Background(), database.UpsertAlbumParams{
		Title:     "Existing Album",
		SortTitle: "Existing Album",
		Musician:  sql.NullString{String: "Existing Artist", Valid: true},
		SpotifyID: sql.NullString{String: "album123", Valid: true},
		Cover:     sql.NullString{String: "file:///music/cover.jpg", Valid: true},
	})
	if err != nil {
		t.Fatalf("seed album: %v", err)
	}

	app.Ffprobe = &countingMusicScannerFfprobe{result: testMusicMetadata()}
	app.Spotify = &musicScannerSpotifyStub{
		artist: &spotifylib.FullArtist{
			SimpleArtist: spotifylib.SimpleArtist{
				ID:   spotifylib.ID("artist123"),
				Name: "Test Artist",
			},
			Images: []spotifylib.Image{{URL: "https://i.scdn.co/refreshed-artist.jpg"}},
		},
		album: &spotifylib.FullAlbum{
			SimpleAlbum: spotifylib.SimpleAlbum{
				ID:     spotifylib.ID("album123"),
				Name:   "Test Album",
				Images: []spotifylib.Image{{URL: "https://i.scdn.co/refreshed-album.jpg"}},
			},
		},
	}

	file := trackFile{
		path: filepath.Join(t.TempDir(), "Test Track.m4a"),
		ext:  "m4a",
		size: 5,
	}

	scanned, skipped, errCount := app.processMusicBatch(context.Background(), []trackFile{file})
	if scanned != 1 || skipped != 0 || errCount != 0 {
		t.Fatalf("scan result scanned=%d skipped=%d errors=%d, want 1/0/0", scanned, skipped, errCount)
	}

	var albumCover sql.NullString
	err = app.DB.QueryRow("SELECT cover FROM albums WHERE id = ?", seededAlbum.ID).Scan(&albumCover)
	if err != nil {
		t.Fatalf("get album cover: %v", err)
	}
	if !albumCover.Valid || albumCover.String != "https://i.scdn.co/refreshed-album.jpg" {
		t.Fatalf("album cover = %#v, want refreshed Spotify album image", albumCover)
	}

	var musicianThumb sql.NullString
	err = app.DB.QueryRow("SELECT thumb FROM musicians WHERE id = ?", seededMusician.ID).Scan(&musicianThumb)
	if err != nil {
		t.Fatalf("get musician thumb: %v", err)
	}
	if !musicianThumb.Valid || musicianThumb.String != "https://i.scdn.co/refreshed-artist.jpg" {
		t.Fatalf("musician thumb = %#v, want refreshed Spotify artist image", musicianThumb)
	}
}

func TestProcessMusicBatchPreservesExistingImagesWithoutSpotifyMatch(t *testing.T) {
	app := setupTestApp(t)
	defer app.DB.Close()

	_, err := app.Queries.UpsertMusician(context.Background(), database.UpsertMusicianParams{
		Name:     "Test Artist",
		SortName: "Test Artist",
		Thumb:    sql.NullString{String: "file:///music/artist.jpg", Valid: true},
	})
	if err != nil {
		t.Fatalf("seed musician: %v", err)
	}

	_, err = app.Queries.UpsertAlbum(context.Background(), database.UpsertAlbumParams{
		Title:     "Test Album",
		SortTitle: "Test Album",
		Musician:  sql.NullString{String: "Test Artist", Valid: true},
		Cover:     sql.NullString{String: "file:///music/cover.jpg", Valid: true},
	})
	if err != nil {
		t.Fatalf("seed album: %v", err)
	}

	noSpotifyMatch := errors.New("no spotify match")
	app.Ffprobe = &countingMusicScannerFfprobe{result: testMusicMetadata()}
	app.Spotify = &musicScannerSpotifyStub{
		artistErr: noSpotifyMatch,
		albumErr:  noSpotifyMatch,
	}

	file := trackFile{
		path: filepath.Join(t.TempDir(), "Test Track.m4a"),
		ext:  "m4a",
		size: 5,
	}

	scanned, skipped, errCount := app.processMusicBatch(context.Background(), []trackFile{file})
	if scanned != 1 || skipped != 0 || errCount != 0 {
		t.Fatalf("scan result scanned=%d skipped=%d errors=%d, want 1/0/0", scanned, skipped, errCount)
	}

	var albumCover sql.NullString
	err = app.DB.QueryRow("SELECT cover FROM albums WHERE title = ? AND musician = ?", "Test Album", "Test Artist").Scan(&albumCover)
	if err != nil {
		t.Fatalf("get album cover: %v", err)
	}
	if !albumCover.Valid || albumCover.String != "file:///music/cover.jpg" {
		t.Fatalf("album cover = %#v, want preserved existing cover", albumCover)
	}

	var musicianThumb sql.NullString
	err = app.DB.QueryRow("SELECT thumb FROM musicians WHERE name = ?", "Test Artist").Scan(&musicianThumb)
	if err != nil {
		t.Fatalf("get musician thumb: %v", err)
	}
	if !musicianThumb.Valid || musicianThumb.String != "file:///music/artist.jpg" {
		t.Fatalf("musician thumb = %#v, want preserved existing thumb", musicianThumb)
	}
}

func TestProcessMusicBatchPreservesExistingImagesWhenSpotifyMatchHasNoImages(t *testing.T) {
	app := setupTestApp(t)
	defer app.DB.Close()

	seededMusician, err := app.Queries.UpsertMusician(context.Background(), database.UpsertMusicianParams{
		Name:      "Existing Artist",
		SortName:  "Existing Artist",
		SpotifyID: sql.NullString{String: "artist123", Valid: true},
		Thumb:     sql.NullString{String: "file:///music/artist.jpg", Valid: true},
	})
	if err != nil {
		t.Fatalf("seed musician: %v", err)
	}

	seededAlbum, err := app.Queries.UpsertAlbum(context.Background(), database.UpsertAlbumParams{
		Title:     "Existing Album",
		SortTitle: "Existing Album",
		Musician:  sql.NullString{String: "Existing Artist", Valid: true},
		SpotifyID: sql.NullString{String: "album123", Valid: true},
		Cover:     sql.NullString{String: "file:///music/cover.jpg", Valid: true},
	})
	if err != nil {
		t.Fatalf("seed album: %v", err)
	}

	app.Ffprobe = &countingMusicScannerFfprobe{result: testMusicMetadata()}
	app.Spotify = &musicScannerSpotifyStub{
		artist: &spotifylib.FullArtist{
			SimpleArtist: spotifylib.SimpleArtist{
				ID:   spotifylib.ID("artist123"),
				Name: "Test Artist",
			},
		},
		album: &spotifylib.FullAlbum{
			SimpleAlbum: spotifylib.SimpleAlbum{
				ID:   spotifylib.ID("album123"),
				Name: "Test Album",
			},
		},
	}

	file := trackFile{
		path: filepath.Join(t.TempDir(), "Test Track.m4a"),
		ext:  "m4a",
		size: 5,
	}

	scanned, skipped, errCount := app.processMusicBatch(context.Background(), []trackFile{file})
	if scanned != 1 || skipped != 0 || errCount != 0 {
		t.Fatalf("scan result scanned=%d skipped=%d errors=%d, want 1/0/0", scanned, skipped, errCount)
	}

	var albumCover sql.NullString
	err = app.DB.QueryRow("SELECT cover FROM albums WHERE id = ?", seededAlbum.ID).Scan(&albumCover)
	if err != nil {
		t.Fatalf("get album cover: %v", err)
	}
	if !albumCover.Valid || albumCover.String != "file:///music/cover.jpg" {
		t.Fatalf("album cover = %#v, want preserved existing cover", albumCover)
	}

	var musicianThumb sql.NullString
	err = app.DB.QueryRow("SELECT thumb FROM musicians WHERE id = ?", seededMusician.ID).Scan(&musicianThumb)
	if err != nil {
		t.Fatalf("get musician thumb: %v", err)
	}
	if !musicianThumb.Valid || musicianThumb.String != "file:///music/artist.jpg" {
		t.Fatalf("musician thumb = %#v, want preserved existing thumb", musicianThumb)
	}
}

func TestProcessMusicBatchIgnoresEmbeddedArtworkWithoutSpotifyMatch(t *testing.T) {
	app := setupTestApp(t)
	defer app.DB.Close()

	metadata := testMusicMetadata()
	metadata.Streams = append(metadata.Streams, ffprobe.Stream{
		Index:     1,
		CodecName: "mjpeg",
		CodecType: "video",
		Disposition: ffprobe.StreamDisposition{
			AttachedPic: 1,
		},
	})
	app.Ffprobe = &countingMusicScannerFfprobe{result: metadata}

	file := trackFile{
		path: filepath.Join(t.TempDir(), "Test Track.m4a"),
		ext:  "m4a",
		size: 5,
	}

	scanned, skipped, errCount := app.processMusicBatch(context.Background(), []trackFile{file})
	if scanned != 1 || skipped != 0 || errCount != 0 {
		t.Fatalf("scan result scanned=%d skipped=%d errors=%d, want 1/0/0", scanned, skipped, errCount)
	}

	var albumCover sql.NullString
	err := app.DB.QueryRow("SELECT cover FROM albums WHERE title = ?", "Test Album").Scan(&albumCover)
	if err != nil {
		t.Fatalf("get album cover: %v", err)
	}
	if albumCover.Valid {
		t.Fatalf("album cover = %#v, want no embedded artwork assigned", albumCover)
	}

	var musicianThumb sql.NullString
	err = app.DB.QueryRow("SELECT thumb FROM musicians WHERE name = ?", "Test Artist").Scan(&musicianThumb)
	if err != nil {
		t.Fatalf("get musician thumb: %v", err)
	}
	if musicianThumb.Valid {
		t.Fatalf("musician thumb = %#v, want no embedded artwork assigned", musicianThumb)
	}
}

func TestProcessMusicBatchUsesScanLocalEntityCaches(t *testing.T) {
	app := setupTestApp(t)
	defer app.DB.Close()

	spotifyStub := &musicScannerSpotifyStub{
		artistErr: errors.New("artist unavailable"),
		albumErr:  errors.New("album unavailable"),
	}
	app.Ffprobe = &countingMusicScannerFfprobe{result: testMusicMetadata()}
	app.Spotify = spotifyStub

	dir := t.TempDir()
	files := []trackFile{
		{
			path: filepath.Join(dir, "Track One.m4a"),
			ext:  "m4a",
			size: 5,
		},
		{
			path: filepath.Join(dir, "Track Two.m4a"),
			ext:  "m4a",
			size: 6,
		},
	}

	scanned, skipped, errCount := app.processMusicBatch(context.Background(), files)
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

func TestProcessMusicBatchRespectsPersistedSpotifyUnmatchedRows(t *testing.T) {
	app := setupTestApp(t)
	defer app.DB.Close()

	musician, err := app.Queries.UpsertMusician(context.Background(), database.UpsertMusicianParams{
		Name:     "Test Artist",
		SortName: "Test Artist",
	})
	if err != nil {
		t.Fatalf("seed musician: %v", err)
	}

	album, err := app.Queries.UpsertAlbum(context.Background(), database.UpsertAlbumParams{
		Title:     "Test Album",
		SortTitle: "Test Album",
		Musician:  sql.NullString{String: "Test Artist", Valid: true},
	})
	if err != nil {
		t.Fatalf("seed album: %v", err)
	}

	err = app.Queries.UpsertMusicSpotifyMatch(context.Background(), database.UpsertMusicSpotifyMatchParams{
		EntityType: musicSpotifyEntityMusician,
		EntityID:   musician.ID,
		Status:     musicSpotifyStatusUnmatched,
		Reason:     sql.NullString{String: "no_results", Valid: true},
	})
	if err != nil {
		t.Fatalf("seed musician spotify match: %v", err)
	}

	err = app.Queries.UpsertMusicSpotifyMatch(context.Background(), database.UpsertMusicSpotifyMatchParams{
		EntityType: musicSpotifyEntityAlbum,
		EntityID:   album.ID,
		Status:     musicSpotifyStatusUnmatched,
		Reason:     sql.NullString{String: "score_below_threshold", Valid: true},
	})
	if err != nil {
		t.Fatalf("seed album spotify match: %v", err)
	}

	spotifyStub := &musicScannerSpotifyStub{
		artistErr: errors.New("should not search artist"),
		albumErr:  errors.New("should not search album"),
	}
	app.Ffprobe = &countingMusicScannerFfprobe{result: testMusicMetadata()}
	app.Spotify = spotifyStub

	file := trackFile{
		path: filepath.Join(t.TempDir(), "Test Track.m4a"),
		ext:  "m4a",
		size: 5,
	}

	scanned, skipped, errCount := app.processMusicBatch(context.Background(), []trackFile{file})
	if scanned != 1 || skipped != 0 || errCount != 0 {
		t.Fatalf("scan result scanned=%d skipped=%d errors=%d, want 1/0/0", scanned, skipped, errCount)
	}
	if spotifyStub.artistCalls != 0 {
		t.Fatalf("artist calls = %d, want 0", spotifyStub.artistCalls)
	}
	if spotifyStub.albumCalls != 0 {
		t.Fatalf("album calls = %d, want 0", spotifyStub.albumCalls)
	}
}

func TestProcessMusicBatchRetriesPersistedSpotifyFailedRows(t *testing.T) {
	app := setupTestApp(t)
	defer app.DB.Close()

	musician, err := app.Queries.UpsertMusician(context.Background(), database.UpsertMusicianParams{
		Name:     "Test Artist",
		SortName: "Test Artist",
	})
	if err != nil {
		t.Fatalf("seed musician: %v", err)
	}

	album, err := app.Queries.UpsertAlbum(context.Background(), database.UpsertAlbumParams{
		Title:     "Test Album",
		SortTitle: "Test Album",
		Musician:  sql.NullString{String: "Test Artist", Valid: true},
	})
	if err != nil {
		t.Fatalf("seed album: %v", err)
	}

	err = app.Queries.UpsertMusicSpotifyMatch(context.Background(), database.UpsertMusicSpotifyMatchParams{
		EntityType: musicSpotifyEntityMusician,
		EntityID:   musician.ID,
		Status:     musicSpotifyStatusFailed,
		Error:      sql.NullString{String: "temporary artist error", Valid: true},
	})
	if err != nil {
		t.Fatalf("seed musician spotify match: %v", err)
	}

	err = app.Queries.UpsertMusicSpotifyMatch(context.Background(), database.UpsertMusicSpotifyMatchParams{
		EntityType: musicSpotifyEntityAlbum,
		EntityID:   album.ID,
		Status:     musicSpotifyStatusFailed,
		Error:      sql.NullString{String: "temporary album error", Valid: true},
	})
	if err != nil {
		t.Fatalf("seed album spotify match: %v", err)
	}

	spotifyStub := &musicScannerSpotifyStub{
		artistErr: errors.New("artist still unavailable"),
		albumErr:  errors.New("album still unavailable"),
	}
	app.Ffprobe = &countingMusicScannerFfprobe{result: testMusicMetadata()}
	app.Spotify = spotifyStub

	file := trackFile{
		path: filepath.Join(t.TempDir(), "Test Track.m4a"),
		ext:  "m4a",
		size: 5,
	}

	scanned, skipped, errCount := app.processMusicBatch(context.Background(), []trackFile{file})
	if scanned != 1 || skipped != 0 || errCount != 0 {
		t.Fatalf("scan result scanned=%d skipped=%d errors=%d, want 1/0/0", scanned, skipped, errCount)
	}
	if spotifyStub.artistCalls != 1 {
		t.Fatalf("artist calls = %d, want 1", spotifyStub.artistCalls)
	}
	if spotifyStub.albumCalls != 1 {
		t.Fatalf("album calls = %d, want 1", spotifyStub.albumCalls)
	}
}

func TestProcessMusicBatchDoesNotUpdateUnchangedSpotifyImages(t *testing.T) {
	app := setupTestApp(t)
	defer app.DB.Close()

	seededMusician, err := app.Queries.UpsertMusician(context.Background(), database.UpsertMusicianParams{
		Name:      "Existing Artist",
		SortName:  "Existing Artist",
		SpotifyID: sql.NullString{String: "artist123", Valid: true},
		Thumb:     sql.NullString{String: "https://i.scdn.co/artist.jpg", Valid: true},
	})
	if err != nil {
		t.Fatalf("seed musician: %v", err)
	}

	seededAlbum, err := app.Queries.UpsertAlbum(context.Background(), database.UpsertAlbumParams{
		Title:     "Existing Album",
		SortTitle: "Existing Album",
		Musician:  sql.NullString{String: "Existing Artist", Valid: true},
		SpotifyID: sql.NullString{String: "album123", Valid: true},
		Cover:     sql.NullString{String: "https://i.scdn.co/album.jpg", Valid: true},
	})
	if err != nil {
		t.Fatalf("seed album: %v", err)
	}

	time.Sleep(1100 * time.Millisecond)

	app.Ffprobe = &countingMusicScannerFfprobe{result: testMusicMetadata()}
	app.Spotify = &musicScannerSpotifyStub{
		artist: &spotifylib.FullArtist{
			SimpleArtist: spotifylib.SimpleArtist{
				ID:   spotifylib.ID("artist123"),
				Name: "Test Artist",
			},
			Images: []spotifylib.Image{{URL: "https://i.scdn.co/artist.jpg"}},
		},
		album: &spotifylib.FullAlbum{
			SimpleAlbum: spotifylib.SimpleAlbum{
				ID:     spotifylib.ID("album123"),
				Name:   "Test Album",
				Images: []spotifylib.Image{{URL: "https://i.scdn.co/album.jpg"}},
			},
		},
	}

	file := trackFile{
		path: filepath.Join(t.TempDir(), "Test Track.m4a"),
		ext:  "m4a",
		size: 5,
	}

	scanned, skipped, errCount := app.processMusicBatch(context.Background(), []trackFile{file})
	if scanned != 1 || skipped != 0 || errCount != 0 {
		t.Fatalf("scan result scanned=%d skipped=%d errors=%d, want 1/0/0", scanned, skipped, errCount)
	}

	var musicianUpdatedAt string
	err = app.DB.QueryRow("SELECT updated_at FROM musicians WHERE id = ?", seededMusician.ID).Scan(&musicianUpdatedAt)
	if err != nil {
		t.Fatalf("get musician updated_at: %v", err)
	}
	if musicianUpdatedAt != seededMusician.UpdatedAt {
		t.Fatalf("musician updated_at = %q, want unchanged %q", musicianUpdatedAt, seededMusician.UpdatedAt)
	}

	var albumUpdatedAt string
	err = app.DB.QueryRow("SELECT updated_at FROM albums WHERE id = ?", seededAlbum.ID).Scan(&albumUpdatedAt)
	if err != nil {
		t.Fatalf("get album updated_at: %v", err)
	}
	if albumUpdatedAt != seededAlbum.UpdatedAt {
		t.Fatalf("album updated_at = %q, want unchanged %q", albumUpdatedAt, seededAlbum.UpdatedAt)
	}
}

func TestRunMusicScanDoesNotClearSpotifyRuntimeCache(t *testing.T) {
	app := setupTestApp(t)
	defer app.DB.Close()

	musicDir := t.TempDir()
	trackPath := filepath.Join(musicDir, "Test Track.m4a")
	err := os.WriteFile(trackPath, []byte("test"), 0644)
	if err != nil {
		t.Fatalf("write track file: %v", err)
	}

	spotifyStub := &musicScannerSpotifyStub{}
	app.Ffprobe = &countingMusicScannerFfprobe{result: testMusicMetadata()}
	app.Spotify = spotifyStub
	app.Settings = &database.Setting{
		MusicDir: sql.NullString{String: musicDir, Valid: true},
	}

	finishMusicScan()
	if !tryBeginMusicScan() {
		t.Fatal("failed to acquire music scan guard")
	}

	app.runMusicScan()

	if spotifyStub.clearCalls != 0 {
		t.Fatalf("spotify cache clear calls = %d, want 0", spotifyStub.clearCalls)
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
			Duration:   "245.125",
			BitRate:    "1411200",
			FormatName: "flac",
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
				Tags: ffprobe.StreamTags{
					Language: "jpn",
				},
			},
		},
	}
	app.Ffprobe = newMusicScannerFfprobeByPath(map[string]*ffprobe.FfprobeResult{
		trackPath: metadata,
	})

	resolved, err := app.resolveTrackFile(context.Background(), newMusicScanContext(map[string]int64{}), trackFile{
		path: trackPath,
		ext:  "flac",
		size: 42,
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
	if params.Codec != "flac" || params.Profile != "Lossless" || params.Channels != "5.1" || params.ChannelLayout != "5.1" {
		t.Fatalf("audio fields = codec %q profile %q channels %q layout %q, want flac/Lossless/5.1/5.1",
			params.Codec, params.Profile, params.Channels, params.ChannelLayout)
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
	if resolved.album == nil {
		t.Fatal("expected resolved album")
	}
	if resolved.album.title != "Mapped Album" || resolved.album.sortTitle != "Mapped Sort Album" || resolved.album.albumArtist != "Mapped Album Artist" {
		t.Fatalf("resolved album = %#v, want mapped album tags", resolved.album)
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

	resolved, err := app.resolveTrackFile(context.Background(), newMusicScanContext(map[string]int64{}), trackFile{
		path: trackPath,
		ext:  "mp3",
		size: 7,
	})
	if err != nil {
		t.Fatalf("resolve track file: %v", err)
	}

	params := resolved.params
	if params.Title != "No Tags.mp3" || params.SortTitle != "No Tags.mp3" {
		t.Fatalf("title/sort_title = %q/%q, want filename fallback", params.Title, params.SortTitle)
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

	files := []trackFile{
		{path: firstPath, ext: "m4a", size: 5},
		{path: secondPath, ext: "m4a", size: 6},
	}
	scanned, skipped, errCount := app.processMusicBatch(context.Background(), files)
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

	file := trackFile{path: trackPath, ext: "m4a", size: 5}
	scanned, skipped, errCount := app.processMusicBatch(context.Background(), []trackFile{file})
	if scanned != 1 || skipped != 0 || errCount != 0 {
		t.Fatalf("first scan result scanned=%d skipped=%d errors=%d, want 1/0/0", scanned, skipped, errCount)
	}

	ffprobeStub.results[trackPath] = testMusicMetadataWithTags(ffprobe.FormatTags{
		Title:  "Updated Title",
		Artist: "Genre Artist",
		Album:  "Genre Album",
		Genre:  "Jazz",
	})
	file.size = 8

	scanned, skipped, errCount = app.processMusicBatch(context.Background(), []trackFile{file})
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

func TestProcessMusicBatchPersistsSpotifyMatchedRows(t *testing.T) {
	app := setupTestApp(t)
	defer app.DB.Close()

	app.Ffprobe = &countingMusicScannerFfprobe{result: testMusicMetadata()}
	app.Spotify = &musicScannerSpotifyStub{
		artist: &spotifylib.FullArtist{
			SimpleArtist: spotifylib.SimpleArtist{
				ID:   spotifylib.ID("artist123"),
				Name: "Test Artist",
			},
		},
		album: &spotifylib.FullAlbum{
			SimpleAlbum: spotifylib.SimpleAlbum{
				ID:   spotifylib.ID("album123"),
				Name: "Test Album",
			},
		},
	}

	file := trackFile{
		path: filepath.Join(t.TempDir(), "Test Track.m4a"),
		ext:  "m4a",
		size: 5,
	}
	scanned, skipped, errCount := app.processMusicBatch(context.Background(), []trackFile{file})
	if scanned != 1 || skipped != 0 || errCount != 0 {
		t.Fatalf("scan result scanned=%d skipped=%d errors=%d, want 1/0/0", scanned, skipped, errCount)
	}

	var musicianStatus string
	var musicianSpotifyID sql.NullString
	err := app.DB.QueryRow(`
		SELECT msm.status, msm.spotify_id
		FROM music_spotify_matches AS msm
		INNER JOIN musicians AS m ON m.id = msm.entity_id
		WHERE msm.entity_type = ? AND m.name = ?
	`, musicSpotifyEntityMusician, "Test Artist").Scan(&musicianStatus, &musicianSpotifyID)
	if err != nil {
		t.Fatalf("get musician spotify match: %v", err)
	}
	if musicianStatus != musicSpotifyStatusMatched || !musicianSpotifyID.Valid || musicianSpotifyID.String != "artist123" {
		t.Fatalf("musician match = %s/%#v, want matched/artist123", musicianStatus, musicianSpotifyID)
	}

	var albumStatus string
	var albumSpotifyID sql.NullString
	err = app.DB.QueryRow(`
		SELECT msm.status, msm.spotify_id
		FROM music_spotify_matches AS msm
		INNER JOIN albums AS a ON a.id = msm.entity_id
		WHERE msm.entity_type = ? AND a.title = ?
	`, musicSpotifyEntityAlbum, "Test Album").Scan(&albumStatus, &albumSpotifyID)
	if err != nil {
		t.Fatalf("get album spotify match: %v", err)
	}
	if albumStatus != musicSpotifyStatusMatched || !albumSpotifyID.Valid || albumSpotifyID.String != "album123" {
		t.Fatalf("album match = %s/%#v, want matched/album123", albumStatus, albumSpotifyID)
	}
}

func TestProcessMusicBatchPersistsSpotifyUnmatchedDetails(t *testing.T) {
	app := setupTestApp(t)
	defer app.DB.Close()

	app.Ffprobe = &countingMusicScannerFfprobe{result: testMusicMetadata()}
	app.Spotify = &musicScannerSpotifyStub{
		artistErr: &spotifyapi.MatchError{
			Info: spotifyapi.MatchDebugInfo{
				Lookup:        "artist",
				Input:         "Test Artist",
				SearchQuery:   "test artist",
				Strategy:      "normalized",
				CandidateName: "Best Guess",
				Score:         52,
				Threshold:     78,
				Reason:        "score_below_threshold",
			},
		},
		albumErr: &spotifyapi.MatchError{
			Info: spotifyapi.MatchDebugInfo{
				Lookup:          "album",
				Input:           "Test Album",
				SearchQuery:     "album:test album artist:test artist",
				Strategy:        "album_artist",
				CandidateName:   "Wrong Album",
				CandidateArtist: "Wrong Artist",
				Reason:          "no_results",
			},
		},
	}

	file := trackFile{
		path: filepath.Join(t.TempDir(), "Test Track.m4a"),
		ext:  "m4a",
		size: 5,
	}
	scanned, skipped, errCount := app.processMusicBatch(context.Background(), []trackFile{file})
	if scanned != 1 || skipped != 0 || errCount != 0 {
		t.Fatalf("scan result scanned=%d skipped=%d errors=%d, want 1/0/0", scanned, skipped, errCount)
	}

	var status string
	var reason sql.NullString
	var score sql.NullInt64
	var threshold sql.NullInt64
	var candidateName sql.NullString
	var searchQuery sql.NullString
	var strategy sql.NullString
	var errorText sql.NullString
	err := app.DB.QueryRow(`
		SELECT msm.status, msm.reason, msm.score, msm.threshold_value, msm.candidate_name, msm.search_query, msm.strategy, msm.error
		FROM music_spotify_matches AS msm
		INNER JOIN musicians AS m ON m.id = msm.entity_id
		WHERE msm.entity_type = ? AND m.name = ?
	`, musicSpotifyEntityMusician, "Test Artist").Scan(&status, &reason, &score, &threshold, &candidateName, &searchQuery, &strategy, &errorText)
	if err != nil {
		t.Fatalf("get musician unmatched row: %v", err)
	}
	if status != musicSpotifyStatusUnmatched || !reason.Valid || reason.String != "score_below_threshold" {
		t.Fatalf("musician status/reason = %s/%#v, want unmatched/score_below_threshold", status, reason)
	}
	if !score.Valid || score.Int64 != 52 || !threshold.Valid || threshold.Int64 != 78 {
		t.Fatalf("musician score/threshold = %#v/%#v, want 52/78", score, threshold)
	}
	if !candidateName.Valid || candidateName.String != "Best Guess" {
		t.Fatalf("candidate name = %#v, want Best Guess", candidateName)
	}
	if !searchQuery.Valid || searchQuery.String != "test artist" || !strategy.Valid || strategy.String != "normalized" {
		t.Fatalf("search/strategy = %#v/%#v, want test artist/normalized", searchQuery, strategy)
	}
	if errorText.Valid {
		t.Fatalf("error text = %#v, want null for unmatched row", errorText)
	}

	var albumReason sql.NullString
	var candidateArtist sql.NullString
	err = app.DB.QueryRow(`
		SELECT msm.reason, msm.candidate_artist
		FROM music_spotify_matches AS msm
		INNER JOIN albums AS a ON a.id = msm.entity_id
		WHERE msm.entity_type = ? AND a.title = ?
	`, musicSpotifyEntityAlbum, "Test Album").Scan(&albumReason, &candidateArtist)
	if err != nil {
		t.Fatalf("get album unmatched row: %v", err)
	}
	if !albumReason.Valid || albumReason.String != "no_results" {
		t.Fatalf("album reason = %#v, want no_results", albumReason)
	}
	if !candidateArtist.Valid || candidateArtist.String != "Wrong Artist" {
		t.Fatalf("album candidate artist = %#v, want Wrong Artist", candidateArtist)
	}
}

func TestProcessMusicBatchPersistsSpotifyFailedRows(t *testing.T) {
	app := setupTestApp(t)
	defer app.DB.Close()

	app.Ffprobe = &countingMusicScannerFfprobe{result: testMusicMetadata()}
	app.Spotify = &musicScannerSpotifyStub{
		artistErr: errors.New("artist temporary failure"),
		albumErr:  errors.New("album temporary failure"),
	}

	file := trackFile{
		path: filepath.Join(t.TempDir(), "Test Track.m4a"),
		ext:  "m4a",
		size: 5,
	}
	scanned, skipped, errCount := app.processMusicBatch(context.Background(), []trackFile{file})
	if scanned != 1 || skipped != 0 || errCount != 0 {
		t.Fatalf("scan result scanned=%d skipped=%d errors=%d, want 1/0/0", scanned, skipped, errCount)
	}

	var status string
	var errorText sql.NullString
	err := app.DB.QueryRow(`
		SELECT msm.status, msm.error
		FROM music_spotify_matches AS msm
		INNER JOIN musicians AS m ON m.id = msm.entity_id
		WHERE msm.entity_type = ? AND m.name = ?
	`, musicSpotifyEntityMusician, "Test Artist").Scan(&status, &errorText)
	if err != nil {
		t.Fatalf("get musician failed row: %v", err)
	}
	if status != musicSpotifyStatusFailed || !errorText.Valid || errorText.String != "artist temporary failure" {
		t.Fatalf("musician failed row = %s/%#v, want failed/artist temporary failure", status, errorText)
	}

	err = app.DB.QueryRow(`
		SELECT msm.status, msm.error
		FROM music_spotify_matches AS msm
		INNER JOIN albums AS a ON a.id = msm.entity_id
		WHERE msm.entity_type = ? AND a.title = ?
	`, musicSpotifyEntityAlbum, "Test Album").Scan(&status, &errorText)
	if err != nil {
		t.Fatalf("get album failed row: %v", err)
	}
	if status != musicSpotifyStatusFailed || !errorText.Valid || errorText.String != "album temporary failure" {
		t.Fatalf("album failed row = %s/%#v, want failed/album temporary failure", status, errorText)
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

	files := []trackFile{
		{path: badPath, ext: "m4a", size: 5},
		{path: goodPath, ext: "m4a", size: 6},
	}
	scanned, skipped, errCount := app.processMusicBatchWithContext(context.Background(), scan, files)
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
			Artist: "Artist One & Artist Two, Artist One",
			Album:  "Compound Album",
			Genre:  "Indie",
		}),
	})

	scanned, skipped, errCount := app.processMusicBatch(context.Background(), []trackFile{
		{path: trackPath, ext: "m4a", size: 5},
	})
	if scanned != 1 || skipped != 0 || errCount != 0 {
		t.Fatalf("scan result scanned=%d skipped=%d errors=%d, want 1/0/0", scanned, skipped, errCount)
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
	app := setupTestApp(t)
	defer app.DB.Close()

	trackPath := filepath.Join(t.TempDir(), "Ampersand Artist.m4a")
	app.Ffprobe = newMusicScannerFfprobeByPath(map[string]*ffprobe.FfprobeResult{
		trackPath: testMusicMetadataWithTags(ffprobe.FormatTags{
			Title:  "Ampersand Artist",
			Artist: "Brooks & Dunn",
		}),
	})

	scanned, skipped, errCount := app.processMusicBatch(context.Background(), []trackFile{
		{path: trackPath, ext: "m4a", size: 5},
	})
	if scanned != 1 || skipped != 0 || errCount != 0 {
		t.Fatalf("scan result scanned=%d skipped=%d errors=%d, want 1/0/0", scanned, skipped, errCount)
	}

	if got := countMusicScannerRows(t, app.DB, "SELECT COUNT(*) FROM musicians WHERE name = ?", "Brooks & Dunn"); got != 1 {
		t.Fatalf("combined musician count = %d, want 1", got)
	}
	if got := countMusicScannerRows(t, app.DB, "SELECT COUNT(*) FROM musicians WHERE name IN (?, ?)", "Brooks", "Dunn"); got != 0 {
		t.Fatalf("split musician count = %d, want 0", got)
	}
	if got := countMusicScannerRows(t, app.DB, `
		SELECT COUNT(*)
		FROM track_musicians AS tm
		INNER JOIN tracks AS t ON t.id = tm.track_id
		INNER JOIN musicians AS m ON m.id = tm.musician_id
		WHERE t.file_path = ? AND m.name = ?
	`, trackPath, "Brooks & Dunn"); got != 1 {
		t.Fatalf("combined track_musicians count = %d, want 1", got)
	}
}

func TestProcessMusicBatchRemovesStaleTrackMusiciansOnRescan(t *testing.T) {
	app := setupTestApp(t)
	defer app.DB.Close()

	trackPath := filepath.Join(t.TempDir(), "Changed Artist.m4a")
	ffprobeStub := newMusicScannerFfprobeByPath(map[string]*ffprobe.FfprobeResult{
		trackPath: testMusicMetadataWithTags(ffprobe.FormatTags{
			Title:  "Changed Artist",
			Artist: "Artist One & Artist Two, Artist One",
		}),
	})
	app.Ffprobe = ffprobeStub

	scanned, skipped, errCount := app.processMusicBatch(context.Background(), []trackFile{
		{path: trackPath, ext: "m4a", size: 5},
	})
	if scanned != 1 || skipped != 0 || errCount != 0 {
		t.Fatalf("first scan result scanned=%d skipped=%d errors=%d, want 1/0/0", scanned, skipped, errCount)
	}

	ffprobeStub.results[trackPath] = testMusicMetadataWithTags(ffprobe.FormatTags{
		Title:  "Changed Artist",
		Artist: "Solo Artist",
	})

	scanned, skipped, errCount = app.processMusicBatch(context.Background(), []trackFile{
		{path: trackPath, ext: "m4a", size: 8},
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
