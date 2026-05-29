package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"igloo/cmd/internal/database"
	"igloo/cmd/internal/ffprobe"

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

type countingMusicScannerFfprobe struct {
	result *ffprobe.FfprobeResult
	calls  int
}

func (s *countingMusicScannerFfprobe) GetMetadata(filePath string) (*ffprobe.FfprobeResult, error) {
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
