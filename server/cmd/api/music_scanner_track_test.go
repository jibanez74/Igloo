package main

import (
	"context"
	"database/sql"
	"errors"
	"sort"
	"testing"

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

type stubSpotifyLookup struct {
	artist *spotifylib.FullArtist
	err    error
}

type stubMusicScannerSpotify struct {
	artistLookups map[string]stubSpotifyLookup
}

func (s *stubMusicScannerSpotify) SearchAndGetAlbumDetails(_ context.Context, _, _ string) (*spotifylib.FullAlbum, error) {
	return nil, nil
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
