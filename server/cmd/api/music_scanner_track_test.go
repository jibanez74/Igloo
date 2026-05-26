package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"

	"igloo/cmd/internal/database"
	"igloo/cmd/internal/ffprobe"
	spotifyapi "igloo/cmd/internal/spotify"

	_ "github.com/mattn/go-sqlite3"
	spotifylib "github.com/zmb3/spotify/v2"
)

type stubMusicScannerFfprobe struct {
	result *ffprobe.FfprobeResult
	err    error
}

func (s *stubMusicScannerFfprobe) GetMetadata(_ string) (*ffprobe.FfprobeResult, error) {
	return s.result, s.err
}

type sequenceMusicScannerFfprobe struct {
	results []*ffprobe.FfprobeResult
	calls   int
}

func (s *sequenceMusicScannerFfprobe) GetMetadata(_ string) (*ffprobe.FfprobeResult, error) {
	if len(s.results) == 0 {
		return nil, errors.New("no ffprobe results configured")
	}

	index := s.calls
	if index >= len(s.results) {
		index = len(s.results) - 1
	}
	s.calls++

	return s.results[index], nil
}

type stubSpotifyLookup struct {
	artist *spotifylib.FullArtist
	err    error
}

type stubSpotifyAlbumLookup struct {
	album *spotifylib.FullAlbum
	err   error
}

type stubMusicScannerSpotify struct {
	artistLookups map[string]stubSpotifyLookup
	albumLookups  map[string]stubSpotifyAlbumLookup
	album         *spotifylib.FullAlbum
	albumErr      error
	albumInputs   []spotifyapi.AlbumSearchInput
}

func (s *stubMusicScannerSpotify) SearchAndGetAlbumDetails(_ context.Context, input spotifyapi.AlbumSearchInput) (*spotifylib.FullAlbum, error) {
	s.albumInputs = append(s.albumInputs, input)
	if lookup, ok := s.albumLookups[input.Title]; ok {
		return lookup.album, lookup.err
	}

	return s.album, s.albumErr
}

func (s *stubMusicScannerSpotify) SearchArtistByName(_ context.Context, artistName string) (*spotifylib.FullArtist, error) {
	lookup, ok := s.artistLookups[artistName]
	if ok {
		return lookup.artist, lookup.err
	}

	return nil, &spotifyapi.MatchError{
		Info: spotifyapi.MatchDebugInfo{
			Lookup:      "artist",
			Input:       artistName,
			SearchQuery: artistName,
			Strategy:    "artist_search",
			Threshold:   78,
			Reason:      "no_results",
		},
	}
}

func (s *stubMusicScannerSpotify) ClearAllCaches() {}

func newStubArtist(id, name string) *spotifylib.FullArtist {
	return &spotifylib.FullArtist{
		SimpleArtist: spotifylib.SimpleArtist{
			ID:   spotifylib.ID(id),
			Name: name,
		},
	}
}

func newMusicScannerTestApp(t *testing.T, probe ffprobe.FfprobeInterface, spotifyClient spotifyapi.SpotifyInterface) *Application {
	t.Helper()

	db, err := sql.Open("sqlite3", ":memory:?_foreign_keys=on")
	if err != nil {
		t.Fatalf("open in-memory db: %v", err)
	}
	t.Cleanup(func() {
		_ = db.Close()
	})

	app := &Application{
		DB:      db,
		Ffprobe: probe,
		Spotify: spotifyClient,
	}
	setupTestLogger(t, app)

	err = app.InitTables()
	if err != nil {
		t.Fatalf("InitTables failed: %v", err)
	}

	queries, err := database.Prepare(context.Background(), db)
	if err != nil {
		t.Fatalf("prepare queries: %v", err)
	}
	t.Cleanup(func() {
		_ = queries.Close()
	})

	app.Queries = queries

	return app
}

func newTestTrackMetadata(artist string) *ffprobe.FfprobeResult {
	return &ffprobe.FfprobeResult{
		Streams: []ffprobe.Stream{
			{
				CodecType:     "audio",
				CodecName:     "mp3",
				Profile:       "Layer 3",
				Channels:      2,
				ChannelLayout: "stereo",
			},
		},
		Format: ffprobe.Format{
			Duration: "180.0",
			Size:     "123456",
			BitRate:  "320000",
			Tags: ffprobe.FormatTags{
				Title:  "Test Song",
				Artist: artist,
				Track:  "1/1",
				Disc:   "1/1",
			},
		},
	}
}

func newTestAlbumTrackMetadata(artist, album string) *ffprobe.FfprobeResult {
	metadata := newTestTrackMetadata(artist)
	metadata.Format.Tags.Album = album
	metadata.Format.Tags.AlbumArtist = artist
	metadata.Format.Tags.Date = "2020"
	return metadata
}

func newStubAlbum(id, name, coverURL string) *spotifylib.FullAlbum {
	return &spotifylib.FullAlbum{
		SimpleAlbum: spotifylib.SimpleAlbum{
			ID:                   spotifylib.ID(id),
			Name:                 name,
			ReleaseDate:          "2020-01-01",
			ReleaseDatePrecision: "day",
			Images:               []spotifylib.Image{{URL: coverURL, Width: 640, Height: 640}},
			TotalTracks:          1,
		},
		Popularity: 75,
	}
}

func albumCover(t *testing.T, db *sql.DB, title string) sql.NullString {
	t.Helper()

	var cover sql.NullString
	err := db.QueryRow("SELECT cover FROM albums WHERE title = ?", title).Scan(&cover)
	if err != nil {
		t.Fatalf("query album cover: %v", err)
	}

	return cover
}

type scannerCapturedLogEntry struct {
	msg  string
	args []any
}

type scannerCapturedLogger struct {
	warnEntries []scannerCapturedLogEntry
}

func (l *scannerCapturedLogger) Debug(_ string, _ ...any) {}

func (l *scannerCapturedLogger) Info(_ string, _ ...any) {}

func (l *scannerCapturedLogger) Warn(msg string, args ...any) {
	entry := scannerCapturedLogEntry{
		msg:  msg,
		args: make([]any, len(args)),
	}
	copy(entry.args, args)
	l.warnEntries = append(l.warnEntries, entry)
}

func (l *scannerCapturedLogger) Error(_ string, _ ...any) {}

func logArgValue(args []any, key string) any {
	for index := 0; index+1 < len(args); index += 2 {
		if args[index] == key {
			return args[index+1]
		}
	}
	return nil
}

func insertAlbumWithTracks(t *testing.T, db *sql.DB, title, musician string, tracks []string, year int) int64 {
	t.Helper()

	result, err := db.Exec(
		`INSERT INTO albums (title, sort_title, musician) VALUES (?, ?, ?)`,
		title,
		title,
		musician,
	)
	if err != nil {
		t.Fatalf("insert album: %v", err)
	}

	albumID, err := result.LastInsertId()
	if err != nil {
		t.Fatalf("album LastInsertId: %v", err)
	}

	for index, title := range tracks {
		var trackYear sql.NullInt64
		if year > 0 {
			trackYear = sql.NullInt64{Int64: int64(year), Valid: true}
		}

		_, err = db.Exec(
			`INSERT INTO tracks (
				title, sort_title, file_path, file_name, container, mime_type, codec,
				size, track_index, duration, disc, channels, channel_layout, bit_rate,
				profile, year, album_id
			) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			title,
			title,
			fmt.Sprintf("/music/%d/%02d.mp3", albumID, index+1),
			fmt.Sprintf("%02d.mp3", index+1),
			"mp3",
			"audio/mpeg",
			"mp3",
			int64(123456+index),
			int64(index+1),
			int64(180000),
			int64(1),
			"stereo",
			"stereo",
			int64(320000),
			"Layer 3",
			trackYear,
			albumID,
		)
		if err != nil {
			t.Fatalf("insert track %q: %v", title, err)
		}
	}

	return albumID
}

func musicianNames(t *testing.T, db *sql.DB) []string {
	t.Helper()

	rows, err := db.Query("SELECT name FROM musicians ORDER BY name ASC")
	if err != nil {
		t.Fatalf("query musicians: %v", err)
	}
	defer rows.Close()

	var names []string
	for rows.Next() {
		var name string
		err = rows.Scan(&name)
		if err != nil {
			t.Fatalf("scan musician name: %v", err)
		}
		names = append(names, name)
	}

	err = rows.Err()
	if err != nil {
		t.Fatalf("iterate musicians: %v", err)
	}

	sort.Strings(names)

	return names
}

func musicianIDByName(t *testing.T, db *sql.DB, name string) int64 {
	t.Helper()

	var id int64
	err := db.QueryRow("SELECT id FROM musicians WHERE name = ?", name).Scan(&id)
	if err != nil {
		t.Fatalf("query musician %q: %v", name, err)
	}

	return id
}

func TestProcessTrackFile_PreservesFullArtistTagOnSpotifyProbeFailure(t *testing.T) {
	probe := &stubMusicScannerFfprobe{
		result: newTestTrackMetadata("Earth, Wind & Fire"),
	}
	spotifyClient := &stubMusicScannerSpotify{
		artistLookups: map[string]stubSpotifyLookup{
			"Earth, Wind & Fire": {
				err: errors.New("spotify unavailable"),
			},
		},
	}
	app := newMusicScannerTestApp(t, probe, spotifyClient)

	tx, err := app.DB.BeginTx(context.Background(), nil)
	if err != nil {
		t.Fatalf("begin tx: %v", err)
	}

	qtx := app.Queries.WithTx(tx)
	err = app.processTrackFile(context.Background(), qtx, "/music/earth-wind-fire.mp3", "mp3")
	if err != nil {
		_ = tx.Rollback()
		t.Fatalf("processTrackFile failed: %v", err)
	}

	err = tx.Commit()
	if err != nil {
		t.Fatalf("commit tx: %v", err)
	}

	names := musicianNames(t, app.DB)
	if len(names) != 1 {
		t.Fatalf("len(names) = %d, want 1; names=%v", len(names), names)
	}
	if names[0] != "Earth, Wind & Fire" {
		t.Fatalf("names[0] = %q, want %q", names[0], "Earth, Wind & Fire")
	}
}

func TestProcessTrackFile_SplitsCompoundCreditsOnMatchRejection(t *testing.T) {
	probe := &stubMusicScannerFfprobe{
		result: newTestTrackMetadata("Charlie Puth & Coco Jones"),
	}
	spotifyClient := &stubMusicScannerSpotify{
		artistLookups: map[string]stubSpotifyLookup{
			"Charlie Puth & Coco Jones": {
				err: &spotifyapi.MatchError{
					Info: spotifyapi.MatchDebugInfo{
						Lookup:        "artist",
						Input:         "Charlie Puth & Coco Jones",
						SearchQuery:   "Charlie Puth & Coco Jones",
						Strategy:      "artist_search",
						CandidateName: "Charlie Puth",
						Score:         34,
						Threshold:     78,
						Reason:        "score_below_threshold",
					},
				},
			},
			"Charlie Puth": {
				artist: newStubArtist("charlie-puth", "Charlie Puth"),
			},
			"Coco Jones": {
				artist: newStubArtist("coco-jones", "Coco Jones"),
			},
		},
	}
	app := newMusicScannerTestApp(t, probe, spotifyClient)

	tx, err := app.DB.BeginTx(context.Background(), nil)
	if err != nil {
		t.Fatalf("begin tx: %v", err)
	}

	qtx := app.Queries.WithTx(tx)
	err = app.processTrackFile(context.Background(), qtx, "/music/charlie-coco.mp3", "mp3")
	if err != nil {
		_ = tx.Rollback()
		t.Fatalf("processTrackFile failed: %v", err)
	}

	err = tx.Commit()
	if err != nil {
		t.Fatalf("commit tx: %v", err)
	}

	names := musicianNames(t, app.DB)
	if len(names) != 2 {
		t.Fatalf("len(names) = %d, want 2; names=%v", len(names), names)
	}

	wantNames := []string{"Charlie Puth", "Coco Jones"}
	for index := range wantNames {
		if names[index] != wantNames[index] {
			t.Fatalf("names[%d] = %q, want %q", index, names[index], wantNames[index])
		}
	}
}

func TestProcessTrackFile_SplitsFeatureCreditsIntoTrackMusicians(t *testing.T) {
	probe := &stubMusicScannerFfprobe{
		result: newTestAlbumTrackMetadata("Lead Artist feat. Guest Artist", "Feature Album"),
	}
	spotifyClient := &stubMusicScannerSpotify{
		artistLookups: map[string]stubSpotifyLookup{
			"Lead Artist feat. Guest Artist": {
				err: &spotifyapi.MatchError{
					Info: spotifyapi.MatchDebugInfo{
						Lookup:      "artist",
						Input:       "Lead Artist feat. Guest Artist",
						SearchQuery: "Lead Artist feat. Guest Artist",
						Strategy:    "artist_search",
						Threshold:   78,
						Reason:      "no_results",
					},
				},
			},
			"Lead Artist": {
				artist: newStubArtist("lead-artist", "Lead Artist"),
			},
			"Guest Artist": {
				artist: newStubArtist("guest-artist", "Guest Artist"),
			},
		},
	}
	app := newMusicScannerTestApp(t, probe, spotifyClient)

	tx, err := app.DB.BeginTx(context.Background(), nil)
	if err != nil {
		t.Fatalf("begin tx: %v", err)
	}

	qtx := app.Queries.WithTx(tx)
	err = app.processTrackFile(context.Background(), qtx, "/music/feature.mp3", "mp3")
	if err != nil {
		_ = tx.Rollback()
		t.Fatalf("processTrackFile failed: %v", err)
	}

	err = tx.Commit()
	if err != nil {
		t.Fatalf("commit tx: %v", err)
	}

	names := musicianNames(t, app.DB)
	wantNames := []string{"Guest Artist", "Lead Artist"}
	if len(names) != len(wantNames) {
		t.Fatalf("names = %v, want %v", names, wantNames)
	}
	for index := range wantNames {
		if names[index] != wantNames[index] {
			t.Fatalf("names = %v, want %v", names, wantNames)
		}
	}

	guestID := musicianIDByName(t, app.DB, "Guest Artist")
	tracks, err := app.Queries.GetTracksByMusicianID(context.Background(), sqlNullInt64(guestID))
	if err != nil {
		t.Fatalf("GetTracksByMusicianID: %v", err)
	}
	if len(tracks) != 1 || tracks[0].Title != "Test Song" {
		t.Fatalf("guest tracks = %#v, want Test Song", tracks)
	}
}

func TestProcessTrackFile_ReturnsErrorWhenSplitArtistPersistenceFails(t *testing.T) {
	probe := &stubMusicScannerFfprobe{
		result: newTestTrackMetadata("Charlie Puth & Coco Jones"),
	}
	spotifyClient := &stubMusicScannerSpotify{
		artistLookups: map[string]stubSpotifyLookup{
			"Charlie Puth & Coco Jones": {
				err: &spotifyapi.MatchError{
					Info: spotifyapi.MatchDebugInfo{
						Lookup:        "artist",
						Input:         "Charlie Puth & Coco Jones",
						SearchQuery:   "Charlie Puth & Coco Jones",
						Strategy:      "artist_search",
						CandidateName: "Charlie Puth",
						Score:         34,
						Threshold:     78,
						Reason:        "score_below_threshold",
					},
				},
			},
		},
	}
	app := newMusicScannerTestApp(t, probe, spotifyClient)

	tx, err := app.DB.BeginTx(context.Background(), nil)
	if err != nil {
		t.Fatalf("begin tx: %v", err)
	}

	qtx := app.Queries.WithTx(tx)
	err = tx.Rollback()
	if err != nil {
		t.Fatalf("rollback tx: %v", err)
	}

	err = app.processTrackFile(context.Background(), qtx, "/music/charlie-coco.mp3", "mp3")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), `compound musician failed for "Charlie Puth"`) {
		t.Fatalf("error = %q, want compound musician failure", err)
	}
}

func TestProcessTrackFile_StoresSpotifyAlbumCover(t *testing.T) {
	probe := &stubMusicScannerFfprobe{
		result: newTestAlbumTrackMetadata("The Example", "Example Album"),
	}
	spotifyClient := &stubMusicScannerSpotify{
		album: newStubAlbum("example-album", "Example Album", "https://example.com/cover.jpg"),
	}
	app := newMusicScannerTestApp(t, probe, spotifyClient)
	app.Settings = &database.Setting{
		StaticDir: filepath.Join(t.TempDir(), "static"),
	}

	tx, err := app.DB.BeginTx(context.Background(), nil)
	if err != nil {
		t.Fatalf("begin tx: %v", err)
	}

	qtx := app.Queries.WithTx(tx)
	err = app.processTrackFile(context.Background(), qtx, "/music/example.mp3", "mp3")
	if err != nil {
		_ = tx.Rollback()
		t.Fatalf("processTrackFile failed: %v", err)
	}

	err = tx.Commit()
	if err != nil {
		t.Fatalf("commit tx: %v", err)
	}

	cover := albumCover(t, app.DB, "Example Album")
	if !cover.Valid || cover.String != "https://example.com/cover.jpg" {
		t.Fatalf("cover = %#v, want Spotify cover URL", cover)
	}
	if len(spotifyClient.albumInputs) != 1 {
		t.Fatalf("albumInputs len = %d, want 1", len(spotifyClient.albumInputs))
	}
	input := spotifyClient.albumInputs[0]
	if input.Title != "Example Album" || input.Artist != "The Example" || input.Year != 2020 {
		t.Fatalf("album input = %+v, want title/artist/year from tags", input)
	}
	if len(input.TrackTitles) != 1 || input.TrackTitles[0] != "Test Song" {
		t.Fatalf("TrackTitles = %#v, want %#v", input.TrackTitles, []string{"Test Song"})
	}
}

func TestProcessTrackFile_RecordsSpotifyAlbumMatchDiagnostics(t *testing.T) {
	matchErr := &spotifyapi.MatchError{
		Info: spotifyapi.MatchDebugInfo{
			Lookup:        "album",
			Input:         "Missing Album",
			SearchQuery:   `album:"Missing Album" artist:"The Example"`,
			Strategy:      "album_field_search",
			CandidateName: "Wrong Album",
			Score:         42,
			Threshold:     76,
			Reason:        "score_below_threshold",
		},
	}
	probe := &stubMusicScannerFfprobe{
		result: newTestAlbumTrackMetadata("The Example", "Missing Album"),
	}
	spotifyClient := &stubMusicScannerSpotify{
		artistLookups: map[string]stubSpotifyLookup{
			"The Example": {
				artist: newStubArtist("the-example", "The Example"),
			},
		},
		albumLookups: map[string]stubSpotifyAlbumLookup{
			"Missing Album": {
				err: matchErr,
			},
		},
	}
	app := newMusicScannerTestApp(t, probe, spotifyClient)
	app.Settings = &database.Setting{
		StaticDir: filepath.Join(t.TempDir(), "static"),
	}

	tx, err := app.DB.BeginTx(context.Background(), nil)
	if err != nil {
		t.Fatalf("begin tx: %v", err)
	}

	qtx := app.Queries.WithTx(tx)
	err = app.processTrackFile(context.Background(), qtx, "/music/missing.mp3", "mp3")
	if err != nil {
		_ = tx.Rollback()
		t.Fatalf("processTrackFile failed: %v", err)
	}

	err = tx.Commit()
	if err != nil {
		t.Fatalf("commit tx: %v", err)
	}

	var albumID int64
	err = app.DB.QueryRow("SELECT id FROM albums WHERE title = ?", "Missing Album").Scan(&albumID)
	if err != nil {
		t.Fatalf("query album id: %v", err)
	}

	match, err := app.Queries.GetMusicSpotifyMatch(context.Background(), database.GetMusicSpotifyMatchParams{
		EntityType: spotifyMatchEntityAlbum,
		EntityID:   albumID,
	})
	if err != nil {
		t.Fatalf("GetMusicSpotifyMatch: %v", err)
	}
	if match.Status != spotifyMatchStatusUnmatched {
		t.Fatalf("match status = %q, want %q", match.Status, spotifyMatchStatusUnmatched)
	}
	if !match.Reason.Valid || match.Reason.String != "score_below_threshold" {
		t.Fatalf("match reason = %#v, want score_below_threshold", match.Reason)
	}
	if !match.CandidateName.Valid || match.CandidateName.String != "Wrong Album" {
		t.Fatalf("candidate = %#v, want Wrong Album", match.CandidateName)
	}
	if !match.Score.Valid || match.Score.Int64 != 42 {
		t.Fatalf("score = %#v, want 42", match.Score)
	}
}

func TestProcessTrackFile_UsesFolderAlbumArtworkWhenSpotifyDoesNotMatch(t *testing.T) {
	musicDir := t.TempDir()
	trackPath := filepath.Join(musicDir, "track.mp3")
	err := os.WriteFile(trackPath, []byte("audio"), 0o644)
	if err != nil {
		t.Fatalf("write track: %v", err)
	}
	err = os.WriteFile(filepath.Join(musicDir, "cover.jpg"), []byte("cover"), 0o644)
	if err != nil {
		t.Fatalf("write cover: %v", err)
	}

	probe := &stubMusicScannerFfprobe{
		result: newTestAlbumTrackMetadata("The Example", "Local Cover Album"),
	}
	app := newMusicScannerTestApp(t, probe, nil)
	app.Settings = &database.Setting{
		StaticDir: filepath.Join(t.TempDir(), "static"),
	}

	tx, err := app.DB.BeginTx(context.Background(), nil)
	if err != nil {
		t.Fatalf("begin tx: %v", err)
	}

	qtx := app.Queries.WithTx(tx)
	err = app.processTrackFile(context.Background(), qtx, trackPath, "mp3")
	if err != nil {
		_ = tx.Rollback()
		t.Fatalf("processTrackFile failed: %v", err)
	}

	err = tx.Commit()
	if err != nil {
		t.Fatalf("commit tx: %v", err)
	}

	cover := albumCover(t, app.DB, "Local Cover Album")
	if !cover.Valid || !strings.HasPrefix(cover.String, "/api/static/albums/album-") {
		t.Fatalf("cover = %#v, want local static album cover", cover)
	}

	coverPath := filepath.Join(app.Settings.StaticDir, strings.TrimPrefix(cover.String, "/api/static/"))
	got, err := os.ReadFile(coverPath)
	if err != nil {
		t.Fatalf("read copied cover: %v", err)
	}
	if string(got) != "cover" {
		t.Fatalf("copied cover = %q, want %q", got, "cover")
	}
}

func TestProcessTrackFile_ExtractsEmbeddedAlbumArtwork(t *testing.T) {
	musicDir := t.TempDir()
	trackPath := filepath.Join(musicDir, "track.mp3")
	err := os.WriteFile(trackPath, []byte("audio"), 0o644)
	if err != nil {
		t.Fatalf("write track: %v", err)
	}

	metadata := newTestAlbumTrackMetadata("The Example", "Embedded Cover Album")
	metadata.Streams = append(metadata.Streams, ffprobe.Stream{
		Index:     1,
		CodecName: "mjpeg",
		CodecType: "video",
		Disposition: ffprobe.StreamDisposition{
			AttachedPic: 1,
		},
	})
	probe := &stubMusicScannerFfprobe{
		result: metadata,
	}
	app := newMusicScannerTestApp(t, probe, nil)
	app.FFmpeg = &fakeFFmpeg{}
	app.Settings = &database.Setting{
		StaticDir: filepath.Join(t.TempDir(), "static"),
	}

	tx, err := app.DB.BeginTx(context.Background(), nil)
	if err != nil {
		t.Fatalf("begin tx: %v", err)
	}

	qtx := app.Queries.WithTx(tx)
	err = app.processTrackFile(context.Background(), qtx, trackPath, "mp3")
	if err != nil {
		_ = tx.Rollback()
		t.Fatalf("processTrackFile failed: %v", err)
	}

	err = tx.Commit()
	if err != nil {
		t.Fatalf("commit tx: %v", err)
	}

	cover := albumCover(t, app.DB, "Embedded Cover Album")
	if !cover.Valid || !strings.HasSuffix(cover.String, ".jpg") {
		t.Fatalf("cover = %#v, want extracted jpg cover", cover)
	}

	coverPath := filepath.Join(app.Settings.StaticDir, strings.TrimPrefix(cover.String, "/api/static/"))
	got, err := os.ReadFile(coverPath)
	if err != nil {
		t.Fatalf("read extracted cover: %v", err)
	}
	if string(got) != "image" {
		t.Fatalf("extracted cover = %q, want %q", got, "image")
	}
}

func TestScanMusicLibrary_RescansSameSizeMtimeChangesAndClearsStaleMetadata(t *testing.T) {
	musicDir := t.TempDir()
	trackPath := filepath.Join(musicDir, "TRACK.MP3")
	err := os.WriteFile(trackPath, []byte("audio"), 0o644)
	if err != nil {
		t.Fatalf("write track: %v", err)
	}

	first := newTestAlbumTrackMetadata("The Example", "Example Album")
	first.Format.Tags.Genre = "Rock"

	second := newTestTrackMetadata("")
	second.Format.Tags.Title = "Retagged Song"
	second.Format.Tags.Album = ""
	second.Format.Tags.AlbumArtist = ""
	second.Format.Tags.Genre = ""

	probe := &sequenceMusicScannerFfprobe{
		results: []*ffprobe.FfprobeResult{first, second},
	}
	app := newMusicScannerTestApp(t, probe, nil)
	app.Settings = &database.Setting{
		MusicDir:  sql.NullString{String: musicDir, Valid: true},
		StaticDir: filepath.Join(t.TempDir(), "static"),
	}

	app.ScanMusicLibrary()

	track, err := app.Queries.GetTrackByPath(context.Background(), trackPath)
	if err != nil {
		t.Fatalf("get scanned track: %v", err)
	}
	if track.Title != "Test Song" || !track.AlbumID.Valid || !track.MusicianID.Valid {
		t.Fatalf("first scan track = %+v, want tagged album and musician", track)
	}

	newTime := time.Now().Add(2 * time.Hour)
	err = os.Chtimes(trackPath, newTime, newTime)
	if err != nil {
		t.Fatalf("chtimes track: %v", err)
	}

	app.ScanMusicLibrary()

	track, err = app.Queries.GetTrackByPath(context.Background(), trackPath)
	if err != nil {
		t.Fatalf("get rescanned track: %v", err)
	}
	if track.Title != "Retagged Song" {
		t.Fatalf("track title = %q, want Retagged Song", track.Title)
	}
	if track.AlbumID.Valid {
		t.Fatalf("album_id = %#v, want cleared", track.AlbumID)
	}
	if track.MusicianID.Valid {
		t.Fatalf("musician_id = %#v, want cleared", track.MusicianID)
	}
	if probe.calls != 2 {
		t.Fatalf("ffprobe calls = %d, want 2", probe.calls)
	}

	var trackGenreCount int
	err = app.DB.QueryRow("SELECT COUNT(*) FROM track_genres WHERE track_id = ?", track.ID).Scan(&trackGenreCount)
	if err != nil {
		t.Fatalf("count track genres: %v", err)
	}
	if trackGenreCount != 0 {
		t.Fatalf("track genre count = %d, want 0", trackGenreCount)
	}

	var albumCount int
	err = app.DB.QueryRow("SELECT COUNT(*) FROM albums").Scan(&albumCount)
	if err != nil {
		t.Fatalf("count albums: %v", err)
	}
	if albumCount != 0 {
		t.Fatalf("album count = %d, want 0", albumCount)
	}

	var musicianCount int
	err = app.DB.QueryRow("SELECT COUNT(*) FROM musicians").Scan(&musicianCount)
	if err != nil {
		t.Fatalf("count musicians: %v", err)
	}
	if musicianCount != 0 {
		t.Fatalf("musician count = %d, want 0", musicianCount)
	}
}

func TestScanMusicLibrary_IndexesM4AWhenSpotifyMatchFails(t *testing.T) {
	musicDir := t.TempDir()
	trackPath := filepath.Join(musicDir, "03 I Miss You.m4a")
	err := os.WriteFile(trackPath, []byte("audio"), 0o644)
	if err != nil {
		t.Fatalf("write track: %v", err)
	}

	metadata := newTestAlbumTrackMetadata("Adele", "25")
	metadata.Streams[0].CodecName = "aac"
	metadata.Streams[0].Profile = "LC"
	metadata.Format.Tags.Title = "I Miss You"

	matchErr := &spotifyapi.MatchError{
		Info: spotifyapi.MatchDebugInfo{
			Lookup:      "album",
			Input:       "25",
			SearchQuery: `album:"25" artist:"Adele"`,
			Strategy:    "album_field_search",
			Reason:      "search_failed",
			Threshold:   76,
		},
		Err: errors.New("spotify unavailable"),
	}

	probe := &stubMusicScannerFfprobe{result: metadata}
	spotifyClient := &stubMusicScannerSpotify{
		artistLookups: map[string]stubSpotifyLookup{
			"Adele": {
				err: &spotifyapi.MatchError{
					Info: spotifyapi.MatchDebugInfo{
						Lookup:      "artist",
						Input:       "Adele",
						SearchQuery: "Adele",
						Strategy:    "artist_search",
						Reason:      "search_failed",
						Threshold:   78,
					},
					Err: errors.New("spotify unavailable"),
				},
			},
		},
		albumErr: matchErr,
	}
	app := newMusicScannerTestApp(t, probe, spotifyClient)
	app.Settings = &database.Setting{
		MusicDir:  sql.NullString{String: musicDir, Valid: true},
		StaticDir: filepath.Join(t.TempDir(), "static"),
	}

	app.ScanMusicLibrary()

	track, err := app.Queries.GetTrackByPath(context.Background(), trackPath)
	if err != nil {
		t.Fatalf("get scanned track: %v", err)
	}
	if track.Title != "I Miss You" {
		t.Fatalf("track title = %q, want I Miss You", track.Title)
	}
	if track.Container != "m4a" || track.MimeType != "audio/mp4" {
		t.Fatalf("track container/mime = %s/%s, want m4a/audio/mp4", track.Container, track.MimeType)
	}
	if !track.AlbumID.Valid || !track.MusicianID.Valid {
		t.Fatalf("track = %+v, want local album and musician despite Spotify failure", track)
	}
}

func TestScanMusicLibrary_DuplicateScanDoesNotStart(t *testing.T) {
	finishMusicScan()
	if !tryBeginMusicScan() {
		t.Fatal("expected test to acquire music scan guard")
	}
	defer finishMusicScan()

	probe := &sequenceMusicScannerFfprobe{
		results: []*ffprobe.FfprobeResult{newTestTrackMetadata("Adele")},
	}
	app := newMusicScannerTestApp(t, probe, nil)
	app.Settings = &database.Setting{
		MusicDir:  sql.NullString{String: t.TempDir(), Valid: true},
		StaticDir: filepath.Join(t.TempDir(), "static"),
	}

	app.ScanMusicLibrary()

	if probe.calls != 0 {
		t.Fatalf("ffprobe calls = %d, want duplicate scan to skip before probing", probe.calls)
	}
}

func TestScanMusicLibrary_ReconcileMissingTrackDeletesSecondaryMusicians(t *testing.T) {
	musicDir := t.TempDir()
	trackPath := filepath.Join(musicDir, "feature.mp3")
	err := os.WriteFile(trackPath, []byte("audio"), 0o644)
	if err != nil {
		t.Fatalf("write track: %v", err)
	}

	probe := &stubMusicScannerFfprobe{
		result: newTestTrackMetadata("Lead Artist feat. Guest Artist"),
	}
	spotifyClient := &stubMusicScannerSpotify{
		artistLookups: map[string]stubSpotifyLookup{
			"Lead Artist feat. Guest Artist": {
				err: &spotifyapi.MatchError{
					Info: spotifyapi.MatchDebugInfo{
						Lookup:      "artist",
						Input:       "Lead Artist feat. Guest Artist",
						SearchQuery: "Lead Artist feat. Guest Artist",
						Strategy:    "artist_search",
						Threshold:   78,
						Reason:      "no_results",
					},
				},
			},
			"Lead Artist": {
				artist: newStubArtist("lead-artist", "Lead Artist"),
			},
			"Guest Artist": {
				artist: newStubArtist("guest-artist", "Guest Artist"),
			},
		},
	}
	app := newMusicScannerTestApp(t, probe, spotifyClient)
	app.Settings = &database.Setting{
		MusicDir:  sql.NullString{String: musicDir, Valid: true},
		StaticDir: filepath.Join(t.TempDir(), "static"),
	}

	app.ScanMusicLibrary()

	names := musicianNames(t, app.DB)
	wantNames := []string{"Guest Artist", "Lead Artist"}
	if len(names) != len(wantNames) {
		t.Fatalf("names = %v, want %v", names, wantNames)
	}
	for index := range wantNames {
		if names[index] != wantNames[index] {
			t.Fatalf("names = %v, want %v", names, wantNames)
		}
	}

	err = os.Remove(trackPath)
	if err != nil {
		t.Fatalf("remove track: %v", err)
	}

	app.ScanMusicLibrary()

	names = musicianNames(t, app.DB)
	if len(names) != 0 {
		t.Fatalf("names after reconcile = %v, want none", names)
	}
}

func TestBackfillMissingAlbumSpotifyIDs_UpdatesExistingAlbumWithTrackEvidence(t *testing.T) {
	spotifyClient := &stubMusicScannerSpotify{
		album: newStubAlbum("example-album", "Example Album", "https://example.com/cover.jpg"),
	}
	app := newMusicScannerTestApp(t, nil, spotifyClient)
	albumID := insertAlbumWithTracks(t, app.DB, "Example Album", "The Example", []string{
		"First Song",
		"Second Song",
	}, 2020)

	backfilled, err := app.backfillMissingAlbumSpotifyIDs(context.Background())
	if err != nil {
		t.Fatalf("backfillMissingAlbumSpotifyIDs failed: %v", err)
	}
	if backfilled != 1 {
		t.Fatalf("backfilled = %d, want 1", backfilled)
	}

	updatedAlbum, err := app.Queries.GetAlbumByID(context.Background(), albumID)
	if err != nil {
		t.Fatalf("get updated album: %v", err)
	}
	if !updatedAlbum.SpotifyID.Valid || updatedAlbum.SpotifyID.String != "example-album" {
		t.Fatalf("SpotifyID = %#v, want example-album", updatedAlbum.SpotifyID)
	}
	if !updatedAlbum.SpotifyPopularity.Valid || updatedAlbum.SpotifyPopularity.Float64 != 75 {
		t.Fatalf("SpotifyPopularity = %#v, want 75", updatedAlbum.SpotifyPopularity)
	}
	if !updatedAlbum.ReleaseDate.Valid || updatedAlbum.ReleaseDate.String != "2020-01-01" {
		t.Fatalf("ReleaseDate = %#v, want 2020-01-01", updatedAlbum.ReleaseDate)
	}
	if !updatedAlbum.Year.Valid || updatedAlbum.Year.Int64 != 2020 {
		t.Fatalf("Year = %#v, want 2020", updatedAlbum.Year)
	}
	if !updatedAlbum.TotalTracks.Valid || updatedAlbum.TotalTracks.Int64 != 1 {
		t.Fatalf("TotalTracks = %#v, want 1", updatedAlbum.TotalTracks)
	}
	if !updatedAlbum.Cover.Valid || updatedAlbum.Cover.String != "https://example.com/cover.jpg" {
		t.Fatalf("Cover = %#v, want Spotify cover URL", updatedAlbum.Cover)
	}

	if len(spotifyClient.albumInputs) != 1 {
		t.Fatalf("albumInputs len = %d, want 1", len(spotifyClient.albumInputs))
	}
	input := spotifyClient.albumInputs[0]
	if input.Title != "Example Album" || input.Artist != "The Example" || input.Year != 2020 {
		t.Fatalf("album input = %+v, want title/artist/year from existing album", input)
	}
	wantTrackTitles := []string{"First Song", "Second Song"}
	if len(input.TrackTitles) != len(wantTrackTitles) {
		t.Fatalf("TrackTitles = %#v, want %#v", input.TrackTitles, wantTrackTitles)
	}
	for index := range wantTrackTitles {
		if input.TrackTitles[index] != wantTrackTitles[index] {
			t.Fatalf("TrackTitles = %#v, want %#v", input.TrackTitles, wantTrackTitles)
		}
	}
}

func TestBackfillMissingAlbumSpotifyIDs_LogsFailuresAndContinues(t *testing.T) {
	matchErr := &spotifyapi.MatchError{
		Info: spotifyapi.MatchDebugInfo{
			Lookup:        "album",
			Input:         "Missing Album",
			SearchQuery:   `album:"Missing Album" artist:"The Example"`,
			Strategy:      "album_field_search",
			CandidateName: "Wrong Album",
			Score:         42,
			Threshold:     76,
			Reason:        "score_below_threshold",
		},
	}
	spotifyClient := &stubMusicScannerSpotify{
		albumLookups: map[string]stubSpotifyAlbumLookup{
			"Missing Album": {
				err: matchErr,
			},
			"Matched Album": {
				album: newStubAlbum("matched-album", "Matched Album", "https://example.com/matched.jpg"),
			},
		},
	}
	app := newMusicScannerTestApp(t, nil, spotifyClient)
	logger := &scannerCapturedLogger{}
	app.Logger = logger

	missingAlbumID := insertAlbumWithTracks(t, app.DB, "Missing Album", "The Example", []string{"Missing Song"}, 2020)
	matchedAlbumID := insertAlbumWithTracks(t, app.DB, "Matched Album", "The Example", []string{"Matched Song"}, 2020)

	backfilled, err := app.backfillMissingAlbumSpotifyIDs(context.Background())
	if err != nil {
		t.Fatalf("backfillMissingAlbumSpotifyIDs failed: %v", err)
	}
	if backfilled != 1 {
		t.Fatalf("backfilled = %d, want 1", backfilled)
	}

	missingAlbum, err := app.Queries.GetAlbumByID(context.Background(), missingAlbumID)
	if err != nil {
		t.Fatalf("get missing album: %v", err)
	}
	if missingAlbum.SpotifyID.Valid {
		t.Fatalf("missing album SpotifyID = %#v, want null", missingAlbum.SpotifyID)
	}

	matchedAlbum, err := app.Queries.GetAlbumByID(context.Background(), matchedAlbumID)
	if err != nil {
		t.Fatalf("get matched album: %v", err)
	}
	if !matchedAlbum.SpotifyID.Valid || matchedAlbum.SpotifyID.String != "matched-album" {
		t.Fatalf("matched album SpotifyID = %#v, want matched-album", matchedAlbum.SpotifyID)
	}

	if len(logger.warnEntries) != 1 {
		t.Fatalf("warnEntries len = %d, want 1", len(logger.warnEntries))
	}
	entry := logger.warnEntries[0]
	if entry.msg != "failed to match album on Spotify" {
		t.Fatalf("warn message = %q, want Spotify match failure", entry.msg)
	}
	if got := logArgValue(entry.args, "reason"); got != "score_below_threshold" {
		t.Fatalf("logged reason = %#v, want score_below_threshold", got)
	}
	if got := logArgValue(entry.args, "candidate"); got != "Wrong Album" {
		t.Fatalf("logged candidate = %#v, want Wrong Album", got)
	}
	if got := logArgValue(entry.args, "score"); got != 42 {
		t.Fatalf("logged score = %#v, want 42", got)
	}
}

func TestScanMusicLibrary_BackfillsMissingAlbumCoverForUnchangedTrack(t *testing.T) {
	musicDir := t.TempDir()
	trackPath := filepath.Join(musicDir, "track.mp3")
	err := os.WriteFile(trackPath, []byte("audio"), 0o644)
	if err != nil {
		t.Fatalf("write track: %v", err)
	}
	err = os.WriteFile(filepath.Join(musicDir, "folder.jpg"), []byte("folder-cover"), 0o644)
	if err != nil {
		t.Fatalf("write cover: %v", err)
	}

	probe := &stubMusicScannerFfprobe{
		err: errors.New("ffprobe should not run for unchanged track with folder art"),
	}
	app := newMusicScannerTestApp(t, probe, nil)
	app.Settings = &database.Setting{
		MusicDir:  sql.NullString{String: musicDir, Valid: true},
		StaticDir: filepath.Join(t.TempDir(), "static"),
	}

	album, err := app.Queries.UpsertAlbum(context.Background(), database.UpsertAlbumParams{
		Title:     "Backfill Album",
		SortTitle: "Backfill Album",
		Musician:  sql.NullString{String: "The Example", Valid: true},
	})
	if err != nil {
		t.Fatalf("upsert album: %v", err)
	}

	info, err := os.Stat(trackPath)
	if err != nil {
		t.Fatalf("stat track: %v", err)
	}
	_, err = app.Queries.UpsertTrack(context.Background(), database.UpsertTrackParams{
		Title:         "Backfill Song",
		SortTitle:     "Backfill Song",
		FilePath:      trackPath,
		FileName:      filepath.Base(trackPath),
		Container:     "mp3",
		MimeType:      "audio/mpeg",
		Codec:         "mp3",
		Size:          info.Size(),
		TrackIndex:    1,
		Duration:      180000,
		Disc:          1,
		Channels:      "stereo",
		ChannelLayout: "stereo",
		BitRate:       320000,
		Profile:       "Layer 3",
		AlbumID:       sql.NullInt64{Int64: album.ID, Valid: true},
	})
	if err != nil {
		t.Fatalf("upsert track: %v", err)
	}

	app.ScanMusicLibrary()

	updatedAlbum, err := app.Queries.GetAlbumByID(context.Background(), album.ID)
	if err != nil {
		t.Fatalf("get album: %v", err)
	}
	if albumCoverMissing(updatedAlbum) {
		t.Fatalf("album cover is still missing after scan")
	}

	coverPath := filepath.Join(app.Settings.StaticDir, strings.TrimPrefix(updatedAlbum.Cover.String, "/api/static/"))
	got, err := os.ReadFile(coverPath)
	if err != nil {
		t.Fatalf("read backfilled cover: %v", err)
	}
	if string(got) != "folder-cover" {
		t.Fatalf("backfilled cover = %q, want %q", got, "folder-cover")
	}
}
