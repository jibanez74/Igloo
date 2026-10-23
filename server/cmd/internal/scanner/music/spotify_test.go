package music

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	"igloo/cmd/internal/database"
	"igloo/cmd/internal/ffprobe"
	"igloo/cmd/internal/scanner"
	"igloo/cmd/internal/scanner/scannertest"
	spotifyapi "igloo/cmd/internal/spotify"
)

func TestProcessMusicBatchAssignsFirstSpotifyImages(t *testing.T) {
	app := setupMusicScanner(t)

	app.ffprobe = &scannertest.CountingProbe{Default: testMusicMetadata()}
	app.spotify = &musicScannerSpotifyStub{
		artist: &spotifyapi.FullArtist{
			SimpleArtist: spotifyapi.SimpleArtist{
				ID:   spotifyapi.ID("artist123"),
				Name: "Test Artist",
			},
			Images: []spotifyapi.Image{
				{URL: "https://i.scdn.co/artist-first.jpg"},
				{URL: "https://i.scdn.co/artist-second.jpg"},
			},
		},
		album: &spotifyapi.FullAlbum{
			SimpleAlbum: spotifyapi.SimpleAlbum{
				ID:          spotifyapi.ID("album123"),
				Name:        "Test Album",
				TotalTracks: 10,
				Images: []spotifyapi.Image{
					{URL: "https://i.scdn.co/album-first.jpg"},
					{URL: "https://i.scdn.co/album-second.jpg"},
				},
			},
		},
	}

	file := testTrackFile(t)

	scanned, skipped, errCount := app.processMusicBatchForTest(t, context.Background(), []scanner.ScanFile{file})
	if scanned != 1 || skipped != 0 || errCount != 0 {
		t.Fatalf("scan result scanned=%d skipped=%d errors=%d, want 1/0/0", scanned, skipped, errCount)
	}

	var albumCover sql.NullString
	err := app.tx.DB.QueryRow("SELECT cover FROM albums WHERE spotify_id = ?", "album123").Scan(&albumCover)
	if err != nil {
		t.Fatalf("get album cover: %v", err)
	}
	if !albumCover.Valid || albumCover.String != "https://i.scdn.co/album-first.jpg" {
		t.Fatalf("album cover = %#v, want first Spotify album image", albumCover)
	}

	var musicianThumb sql.NullString
	err = app.tx.DB.QueryRow("SELECT thumb FROM musicians WHERE spotify_id = ?", "artist123").Scan(&musicianThumb)
	if err != nil {
		t.Fatalf("get musician thumb: %v", err)
	}
	if !musicianThumb.Valid || musicianThumb.String != "https://i.scdn.co/artist-first.jpg" {
		t.Fatalf("musician thumb = %#v, want first Spotify artist image", musicianThumb)
	}
}

func TestProcessMusicBatchRefreshesExistingSpotifyImages(t *testing.T) {
	app := setupMusicScanner(t)

	seededMusician, seededAlbum := seedMatchedSpotifyEntities(t, app)

	app.ffprobe = &scannertest.CountingProbe{Default: testMusicMetadata()}
	app.spotify = &musicScannerSpotifyStub{
		artist: &spotifyapi.FullArtist{
			SimpleArtist: spotifyapi.SimpleArtist{
				ID:   spotifyapi.ID("artist123"),
				Name: "Test Artist",
			},
			Images: []spotifyapi.Image{{URL: "https://i.scdn.co/refreshed-artist.jpg"}},
		},
		album: &spotifyapi.FullAlbum{
			SimpleAlbum: spotifyapi.SimpleAlbum{
				ID:     spotifyapi.ID("album123"),
				Name:   "Test Album",
				Images: []spotifyapi.Image{{URL: "https://i.scdn.co/refreshed-album.jpg"}},
			},
		},
	}

	file := testTrackFile(t)

	scanned, skipped, errCount := app.processMusicBatchForTest(t, context.Background(), []scanner.ScanFile{file})
	if scanned != 1 || skipped != 0 || errCount != 0 {
		t.Fatalf("scan result scanned=%d skipped=%d errors=%d, want 1/0/0", scanned, skipped, errCount)
	}

	var albumCover sql.NullString
	err := app.tx.DB.QueryRow("SELECT cover FROM albums WHERE id = ?", seededAlbum.ID).Scan(&albumCover)
	if err != nil {
		t.Fatalf("get album cover: %v", err)
	}
	if !albumCover.Valid || albumCover.String != "https://i.scdn.co/refreshed-album.jpg" {
		t.Fatalf("album cover = %#v, want refreshed Spotify album image", albumCover)
	}

	var musicianThumb sql.NullString
	err = app.tx.DB.QueryRow("SELECT thumb FROM musicians WHERE id = ?", seededMusician.ID).Scan(&musicianThumb)
	if err != nil {
		t.Fatalf("get musician thumb: %v", err)
	}
	if !musicianThumb.Valid || musicianThumb.String != "https://i.scdn.co/refreshed-artist.jpg" {
		t.Fatalf("musician thumb = %#v, want refreshed Spotify artist image", musicianThumb)
	}
}

func TestProcessMusicBatchPreservesExistingImagesWithoutSpotifyMatch(t *testing.T) {
	app := setupMusicScanner(t)

	_, err := app.queries.UpsertMusician(context.Background(), database.UpsertMusicianParams{
		Name:     "Test Artist",
		SortName: "Test Artist",
		Thumb:    sql.NullString{String: "file:///music/artist.jpg", Valid: true},
	})
	if err != nil {
		t.Fatalf("seed musician: %v", err)
	}

	_, err = app.queries.UpsertAlbum(context.Background(), database.UpsertAlbumParams{
		Title:     "Test Album",
		SortTitle: "Test Album",
		Musician:  sql.NullString{String: "Test Artist", Valid: true},
		Cover:     sql.NullString{String: "file:///music/cover.jpg", Valid: true},
	})
	if err != nil {
		t.Fatalf("seed album: %v", err)
	}

	noSpotifyMatch := errors.New("no spotify match")
	app.ffprobe = &scannertest.CountingProbe{Default: testMusicMetadata()}
	app.spotify = &musicScannerSpotifyStub{
		artistErr: noSpotifyMatch,
		albumErr:  noSpotifyMatch,
	}

	file := testTrackFile(t)

	scanned, skipped, errCount := app.processMusicBatchForTest(t, context.Background(), []scanner.ScanFile{file})
	if scanned != 1 || skipped != 0 || errCount != 0 {
		t.Fatalf("scan result scanned=%d skipped=%d errors=%d, want 1/0/0", scanned, skipped, errCount)
	}

	var albumCover sql.NullString
	err = app.tx.DB.QueryRow("SELECT cover FROM albums WHERE title = ? AND musician = ?", "Test Album", "Test Artist").Scan(&albumCover)
	if err != nil {
		t.Fatalf("get album cover: %v", err)
	}
	if !albumCover.Valid || albumCover.String != "file:///music/cover.jpg" {
		t.Fatalf("album cover = %#v, want preserved existing cover", albumCover)
	}

	var musicianThumb sql.NullString
	err = app.tx.DB.QueryRow("SELECT thumb FROM musicians WHERE name = ?", "Test Artist").Scan(&musicianThumb)
	if err != nil {
		t.Fatalf("get musician thumb: %v", err)
	}
	if !musicianThumb.Valid || musicianThumb.String != "file:///music/artist.jpg" {
		t.Fatalf("musician thumb = %#v, want preserved existing thumb", musicianThumb)
	}
}

func TestProcessMusicBatchPreservesExistingImagesWhenSpotifyMatchHasNoImages(t *testing.T) {
	app := setupMusicScanner(t)

	seededMusician, seededAlbum := seedMatchedSpotifyEntities(t, app)

	app.ffprobe = &scannertest.CountingProbe{Default: testMusicMetadata()}
	app.spotify = &musicScannerSpotifyStub{
		artist: &spotifyapi.FullArtist{
			SimpleArtist: spotifyapi.SimpleArtist{
				ID:   spotifyapi.ID("artist123"),
				Name: "Test Artist",
			},
		},
		album: &spotifyapi.FullAlbum{
			SimpleAlbum: spotifyapi.SimpleAlbum{
				ID:   spotifyapi.ID("album123"),
				Name: "Test Album",
			},
		},
	}

	file := testTrackFile(t)

	scanned, skipped, errCount := app.processMusicBatchForTest(t, context.Background(), []scanner.ScanFile{file})
	if scanned != 1 || skipped != 0 || errCount != 0 {
		t.Fatalf("scan result scanned=%d skipped=%d errors=%d, want 1/0/0", scanned, skipped, errCount)
	}

	var albumCover sql.NullString
	err := app.tx.DB.QueryRow("SELECT cover FROM albums WHERE id = ?", seededAlbum.ID).Scan(&albumCover)
	if err != nil {
		t.Fatalf("get album cover: %v", err)
	}
	if !albumCover.Valid || albumCover.String != "file:///music/cover.jpg" {
		t.Fatalf("album cover = %#v, want preserved existing cover", albumCover)
	}

	var musicianThumb sql.NullString
	err = app.tx.DB.QueryRow("SELECT thumb FROM musicians WHERE id = ?", seededMusician.ID).Scan(&musicianThumb)
	if err != nil {
		t.Fatalf("get musician thumb: %v", err)
	}
	if !musicianThumb.Valid || musicianThumb.String != "file:///music/artist.jpg" {
		t.Fatalf("musician thumb = %#v, want preserved existing thumb", musicianThumb)
	}
}

func TestProcessMusicBatchIgnoresEmbeddedArtworkWithoutSpotifyMatch(t *testing.T) {
	app := setupMusicScanner(t)

	metadata := testMusicMetadata()
	metadata.Streams = append(metadata.Streams, ffprobe.Stream{
		Index:     1,
		CodecName: "mjpeg",
		CodecType: "video",
		Disposition: ffprobe.StreamDisposition{
			AttachedPic: 1,
		},
	})
	app.ffprobe = &scannertest.CountingProbe{Default: metadata}

	file := testTrackFile(t)

	scanned, skipped, errCount := app.processMusicBatchForTest(t, context.Background(), []scanner.ScanFile{file})
	if scanned != 1 || skipped != 0 || errCount != 0 {
		t.Fatalf("scan result scanned=%d skipped=%d errors=%d, want 1/0/0", scanned, skipped, errCount)
	}

	var albumCover sql.NullString
	err := app.tx.DB.QueryRow("SELECT cover FROM albums WHERE title = ?", "Test Album").Scan(&albumCover)
	if err != nil {
		t.Fatalf("get album cover: %v", err)
	}
	if albumCover.Valid {
		t.Fatalf("album cover = %#v, want no embedded artwork assigned", albumCover)
	}

	var musicianThumb sql.NullString
	err = app.tx.DB.QueryRow("SELECT thumb FROM musicians WHERE name = ?", "Test Artist").Scan(&musicianThumb)
	if err != nil {
		t.Fatalf("get musician thumb: %v", err)
	}
	if musicianThumb.Valid {
		t.Fatalf("musician thumb = %#v, want no embedded artwork assigned", musicianThumb)
	}
}

func TestProcessMusicBatchRespectsPersistedSpotifyUnmatchedRows(t *testing.T) {
	app := setupMusicScanner(t)

	musician, album := seedLocalMusicianAndAlbum(t, app)

	err := app.queries.SaveMusicArtistIdentity(context.Background(), database.SaveMusicArtistIdentityParams{IdentityKey: scanner.NormalizedScanCacheKey(musician.Name), MusicianID: musician.ID})
	if err != nil {
		t.Fatal(err)
	}
	err = app.queries.SaveMusicAlbumIdentity(context.Background(), database.SaveMusicAlbumIdentityParams{TitleKey: scanner.NormalizedScanCacheKey(album.Title), ArtistKey: scanner.NormalizedScanCacheKey(album.Musician.String), AlbumID: album.ID})
	if err != nil {
		t.Fatal(err)
	}

	err = app.queries.UpsertMusicProviderMatch(context.Background(), database.UpsertMusicProviderMatchParams{
		EntityType: musicEntityMusician,
		EntityID:   musician.ID,
		Provider:   musicProviderSpotify,
		Status:     musicStatusUnmatched,
		Reason:     sql.NullString{String: "no_results", Valid: true},
	})
	if err != nil {
		t.Fatalf("seed musician spotify match: %v", err)
	}

	err = app.queries.UpsertMusicProviderMatch(context.Background(), database.UpsertMusicProviderMatchParams{
		EntityType: musicEntityAlbum,
		EntityID:   album.ID,
		Provider:   musicProviderSpotify,
		Status:     musicStatusUnmatched,
		Reason:     sql.NullString{String: "score_below_threshold", Valid: true},
	})
	if err != nil {
		t.Fatalf("seed album spotify match: %v", err)
	}

	spotifyStub := &musicScannerSpotifyStub{
		artistErr: errors.New("should not search artist"),
		albumErr:  errors.New("should not search album"),
	}
	app.ffprobe = &scannertest.CountingProbe{Default: testMusicMetadata()}
	app.spotify = spotifyStub

	file := testTrackFile(t)

	scanned, skipped, errCount := app.processMusicBatchForTest(t, context.Background(), []scanner.ScanFile{file})
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
	app := setupMusicScanner(t)

	musician, album := seedLocalMusicianAndAlbum(t, app)

	err := app.queries.UpsertMusicProviderMatch(context.Background(), database.UpsertMusicProviderMatchParams{
		EntityType: musicEntityMusician,
		EntityID:   musician.ID,
		Provider:   musicProviderSpotify,
		Status:     musicStatusFailed,
	})
	if err != nil {
		t.Fatalf("seed musician spotify match: %v", err)
	}

	err = app.queries.UpsertMusicProviderMatch(context.Background(), database.UpsertMusicProviderMatchParams{
		EntityType: musicEntityAlbum,
		EntityID:   album.ID,
		Provider:   musicProviderSpotify,
		Status:     musicStatusFailed,
	})
	if err != nil {
		t.Fatalf("seed album spotify match: %v", err)
	}

	spotifyStub := &musicScannerSpotifyStub{
		artistErr: errors.New("artist still unavailable"),
		albumErr:  errors.New("album still unavailable"),
	}
	app.ffprobe = &scannertest.CountingProbe{Default: testMusicMetadata()}
	app.spotify = spotifyStub

	file := testTrackFile(t)

	scanned, skipped, errCount := app.processMusicBatchForTest(t, context.Background(), []scanner.ScanFile{file})
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
	app := setupMusicScanner(t)

	_, err := app.queries.UpsertMusician(context.Background(), database.UpsertMusicianParams{
		Name:      "Existing Artist",
		SortName:  "Existing Artist",
		SpotifyID: sql.NullString{String: "artist123", Valid: true},
		Thumb:     sql.NullString{String: "https://i.scdn.co/artist.jpg", Valid: true},
	})
	if err != nil {
		t.Fatalf("seed musician: %v", err)
	}

	_, err = app.queries.UpsertAlbum(context.Background(), database.UpsertAlbumParams{
		Title:     "Existing Album",
		SortTitle: "Existing Album",
		Musician:  sql.NullString{String: "Existing Artist", Valid: true},
		SpotifyID: sql.NullString{String: "album123", Valid: true},
		Cover:     sql.NullString{String: "https://i.scdn.co/album.jpg", Valid: true},
	})
	if err != nil {
		t.Fatalf("seed album: %v", err)
	}

	_, err = app.tx.DB.Exec(`
 CREATE TABLE image_writes(entity TEXT);
 CREATE TRIGGER count_artist_image AFTER UPDATE OF thumb ON musicians BEGIN INSERT INTO image_writes VALUES('artist'); END;
 CREATE TRIGGER count_album_image AFTER UPDATE OF cover ON albums BEGIN INSERT INTO image_writes VALUES('album'); END;
 `)
	if err != nil {
		t.Fatal(err)
	}

	app.ffprobe = &scannertest.CountingProbe{Default: testMusicMetadata()}
	app.spotify = &musicScannerSpotifyStub{
		artist: &spotifyapi.FullArtist{
			SimpleArtist: spotifyapi.SimpleArtist{
				ID:   spotifyapi.ID("artist123"),
				Name: "Test Artist",
			},
			Images: []spotifyapi.Image{{URL: "https://i.scdn.co/artist.jpg"}},
		},
		album: &spotifyapi.FullAlbum{
			SimpleAlbum: spotifyapi.SimpleAlbum{
				ID:     spotifyapi.ID("album123"),
				Name:   "Test Album",
				Images: []spotifyapi.Image{{URL: "https://i.scdn.co/album.jpg"}},
			},
		},
	}

	file := testTrackFile(t)

	scanned, skipped, errCount := app.processMusicBatchForTest(t, context.Background(), []scanner.ScanFile{file})
	if scanned != 1 || skipped != 0 || errCount != 0 {
		t.Fatalf("scan result scanned=%d skipped=%d errors=%d, want 1/0/0", scanned, skipped, errCount)
	}

	count := scannertest.CountRows(t, app.tx.DB, "SELECT count(*) FROM image_writes")
	if count != 0 {
		t.Fatalf("unchanged images caused %d writes", count)
	}
}

func TestProcessMusicBatchPersistsSpotifyMatchedRows(t *testing.T) {
	app := setupMusicScanner(t)

	app.ffprobe = &scannertest.CountingProbe{Default: testMusicMetadata()}
	app.spotify = &musicScannerSpotifyStub{
		artist: &spotifyapi.FullArtist{
			SimpleArtist: spotifyapi.SimpleArtist{
				ID:   spotifyapi.ID("artist123"),
				Name: "Test Artist",
			},
		},
		album: &spotifyapi.FullAlbum{
			SimpleAlbum: spotifyapi.SimpleAlbum{
				ID:   spotifyapi.ID("album123"),
				Name: "Test Album",
			},
		},
	}

	file := testTrackFile(t)
	scanned, skipped, errCount := app.processMusicBatchForTest(t, context.Background(), []scanner.ScanFile{file})
	if scanned != 1 || skipped != 0 || errCount != 0 {
		t.Fatalf("scan result scanned=%d skipped=%d errors=%d, want 1/0/0", scanned, skipped, errCount)
	}

	var musicianStatus string
	var musicianSpotifyID sql.NullString
	err := app.tx.DB.QueryRow(`
		SELECT msm.status, m.spotify_id
		FROM music_provider_matches AS msm
		INNER JOIN musicians AS m ON m.id = msm.entity_id
		WHERE msm.entity_type = ? AND m.name = ?
	`, musicEntityMusician, "Test Artist").Scan(&musicianStatus, &musicianSpotifyID)
	if err != nil {
		t.Fatalf("get musician spotify match: %v", err)
	}
	if musicianStatus != musicStatusMatched || !musicianSpotifyID.Valid || musicianSpotifyID.String != "artist123" {
		t.Fatalf("musician match = %s/%#v, want matched/artist123", musicianStatus, musicianSpotifyID)
	}

	var albumStatus string
	var albumSpotifyID sql.NullString
	err = app.tx.DB.QueryRow(`
		SELECT msm.status, a.spotify_id
		FROM music_provider_matches AS msm
		INNER JOIN albums AS a ON a.id = msm.entity_id
		WHERE msm.entity_type = ? AND a.title = ?
	`, musicEntityAlbum, "Test Album").Scan(&albumStatus, &albumSpotifyID)
	if err != nil {
		t.Fatalf("get album spotify match: %v", err)
	}
	if albumStatus != musicStatusMatched || !albumSpotifyID.Valid || albumSpotifyID.String != "album123" {
		t.Fatalf("album match = %s/%#v, want matched/album123", albumStatus, albumSpotifyID)
	}
}

func TestProcessMusicBatchPersistsSpotifyMetadata(t *testing.T) {
	app := setupMusicScanner(t)

	app.ffprobe = &scannertest.CountingProbe{Default: testMusicMetadata()}
	app.spotify = &musicScannerSpotifyStub{
		artist: &spotifyapi.FullArtist{
			SimpleArtist: spotifyapi.SimpleArtist{ID: "artist-meta-123", Name: "Test Artist"},
			Images:       []spotifyapi.Image{{URL: "https://i.scdn.co/artist-meta.jpg"}},
		},
		album: &spotifyapi.FullAlbum{
			SimpleAlbum: spotifyapi.SimpleAlbum{
				ID:                   "album-meta-123",
				Name:                 "Test Album",
				AlbumType:            "single",
				ReleaseDate:          "2024-04-12",
				ReleaseDatePrecision: "day",
				TotalTracks:          10,
				Images:               []spotifyapi.Image{{URL: "https://i.scdn.co/album-meta.jpg"}},
			},
			Tracks: []spotifyapi.SimpleTrack{{ID: "track-meta-1", Name: "Test Track", DiscNumber: 1, TrackNumber: 1, Duration: 180000, Explicit: true}},
		},
	}

	file := testTrackFile(t)
	scan := newMusicScanContext(nil)
	scanned, skipped, errCount := app.processMusicFixtureBatch(t, context.Background(), scan, []scanner.ScanFile{file})
	if scanned != 1 || skipped != 0 || errCount != 0 {
		t.Fatalf("scan result scanned=%d skipped=%d errors=%d, want 1/0/0", scanned, skipped, errCount)
	}
	linkScannedTracks(t, app, scan)

	var musicianSpotifyID sql.NullString
	var musicianThumb sql.NullString
	err := app.tx.DB.QueryRow(`SELECT spotify_id, thumb FROM musicians WHERE name = ?`, "Test Artist").Scan(&musicianSpotifyID, &musicianThumb)
	if err != nil {
		t.Fatalf("get musician metadata: %v", err)
	}
	if !musicianSpotifyID.Valid || musicianSpotifyID.String != "artist-meta-123" {
		t.Fatalf("musician spotify_id = %#v, want artist-meta-123", musicianSpotifyID)
	}
	if !musicianThumb.Valid || musicianThumb.String != "https://i.scdn.co/artist-meta.jpg" {
		t.Fatalf("musician thumb = %#v, want Spotify artist image", musicianThumb)
	}

	var albumSpotifyID sql.NullString
	var albumType sql.NullString
	var totalTracks sql.NullInt64
	var releaseDate sql.NullString
	var year sql.NullInt64
	var cover sql.NullString
	err = app.tx.DB.QueryRow(`
		SELECT spotify_id, album_type, total_tracks, release_date, year, cover
		FROM albums
		WHERE title = ? AND musician = ?
	`, "Test Album", "Test Artist").Scan(&albumSpotifyID, &albumType, &totalTracks, &releaseDate, &year, &cover)
	if err != nil {
		t.Fatalf("get album metadata: %v", err)
	}
	if !albumSpotifyID.Valid || albumSpotifyID.String != "album-meta-123" {
		t.Fatalf("album spotify_id = %#v, want album-meta-123", albumSpotifyID)
	}
	if !albumType.Valid || albumType.String != "single" {
		t.Fatalf("album album_type = %#v, want single", albumType)
	}
	if !totalTracks.Valid || totalTracks.Int64 != 10 {
		t.Fatalf("album total_tracks = %#v, want 10", totalTracks)
	}
	if !releaseDate.Valid || releaseDate.String != "2024-04-12" {
		t.Fatalf("album release_date = %#v, want 2024-04-12", releaseDate)
	}
	if !year.Valid || year.Int64 != 2024 {
		t.Fatalf("album year = %#v, want 2024", year)
	}
	if !cover.Valid || cover.String != "https://i.scdn.co/album-meta.jpg" {
		t.Fatalf("album cover = %#v, want Spotify album image", cover)
	}

	var trackSpotifyID sql.NullString
	var explicit bool
	var trackStatus string
	err = app.tx.DB.QueryRow(`
		SELECT t.spotify_id, t.explicit, m.status FROM tracks t
		JOIN music_provider_matches m ON m.entity_type = 'track' AND m.entity_id = t.id
		WHERE t.file_path = ?
	`, file.Path).Scan(&trackSpotifyID, &explicit, &trackStatus)
	if err != nil {
		t.Fatalf("get track link: %v", err)
	}
	if !trackSpotifyID.Valid || trackSpotifyID.String != "track-meta-1" || !explicit || trackStatus != musicStatusMatched {
		t.Fatalf("track link = %#v explicit=%v status=%s, want track-meta-1 explicit matched", trackSpotifyID, explicit, trackStatus)
	}

	// Spotify sends no genres; nothing but the files' own tags creates one.
	genres := scannertest.CountRows(t, app.tx.DB, "SELECT COUNT(*) FROM genres")
	if genres != 0 {
		t.Fatalf("genre count = %d, want none from Spotify", genres)
	}
}

func TestProcessMusicBatchPersistsSpotifyUnmatchedReasons(t *testing.T) {
	app := setupMusicScanner(t)

	app.ffprobe = &scannertest.CountingProbe{Default: testMusicMetadata()}
	app.spotify = &musicScannerSpotifyStub{
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

	file := testTrackFile(t)
	scanned, skipped, errCount := app.processMusicBatchForTest(t, context.Background(), []scanner.ScanFile{file})
	if scanned != 1 || skipped != 0 || errCount != 0 {
		t.Fatalf("scan result scanned=%d skipped=%d errors=%d, want 1/0/0", scanned, skipped, errCount)
	}

	var status string
	var reason, candidate sql.NullString
	var score sql.NullInt64
	err := app.tx.DB.QueryRow(`
		SELECT msm.status, msm.reason, msm.candidate, msm.score
		FROM music_provider_matches AS msm
		INNER JOIN musicians AS m ON m.id = msm.entity_id
		WHERE msm.entity_type = ? AND m.name = ?
	`, musicEntityMusician, "Test Artist").Scan(&status, &reason, &candidate, &score)
	if err != nil {
		t.Fatalf("get musician unmatched row: %v", err)
	}
	if status != musicStatusUnmatched || !reason.Valid || reason.String != "score_below_threshold" {
		t.Fatalf("musician status/reason = %s/%#v, want unmatched/score_below_threshold", status, reason)
	}
	// A scored miss keeps the rejected candidate and its score, so the miss
	// can be explained and the threshold tuned.
	if candidate.String != "Best Guess" || !score.Valid || score.Int64 != 52 {
		t.Fatalf("musician candidate/score = %#v/%#v, want Best Guess/52", candidate, score)
	}

	var albumReason, albumCandidate sql.NullString
	var albumScore sql.NullInt64
	err = app.tx.DB.QueryRow(`
		SELECT msm.reason, msm.candidate, msm.score
		FROM music_provider_matches AS msm
		INNER JOIN albums AS a ON a.id = msm.entity_id
		WHERE msm.entity_type = ? AND a.title = ?
	`, musicEntityAlbum, "Test Album").Scan(&albumReason, &albumCandidate, &albumScore)
	if err != nil {
		t.Fatalf("get album unmatched row: %v", err)
	}
	if !albumReason.Valid || albumReason.String != "no_results" {
		t.Fatalf("album reason = %#v, want no_results", albumReason)
	}
	// An unscored miss keeps the candidate name alone.
	if albumCandidate.String != "Wrong Album" || albumScore.Valid {
		t.Fatalf("album candidate/score = %#v/%#v, want Wrong Album and no score", albumCandidate, albumScore)
	}
}

func TestProcessMusicBatchPersistsSpotifyFailedRows(t *testing.T) {
	app := setupMusicScanner(t)

	app.ffprobe = &scannertest.CountingProbe{Default: testMusicMetadata()}
	app.spotify = &musicScannerSpotifyStub{
		artistErr: errors.New("artist temporary failure"),
		albumErr:  errors.New("album temporary failure"),
	}

	file := testTrackFile(t)
	scanned, skipped, errCount := app.processMusicBatchForTest(t, context.Background(), []scanner.ScanFile{file})
	if scanned != 1 || skipped != 0 || errCount != 0 {
		t.Fatalf("scan result scanned=%d skipped=%d errors=%d, want 1/0/0", scanned, skipped, errCount)
	}

	var status string
	var reason sql.NullString
	err := app.tx.DB.QueryRow(`
		SELECT msm.status, msm.reason
		FROM music_provider_matches AS msm
		INNER JOIN musicians AS m ON m.id = msm.entity_id
		WHERE msm.entity_type = ? AND m.name = ?
	`, musicEntityMusician, "Test Artist").Scan(&status, &reason)
	if err != nil {
		t.Fatalf("get musician failed row: %v", err)
	}
	if status != musicStatusFailed || reason.Valid {
		t.Fatalf("musician failed row = %s/%#v, want failed/null", status, reason)
	}

	err = app.tx.DB.QueryRow(`
		SELECT msm.status, msm.reason
		FROM music_provider_matches AS msm
		INNER JOIN albums AS a ON a.id = msm.entity_id
		WHERE msm.entity_type = ? AND a.title = ?
	`, musicEntityAlbum, "Test Album").Scan(&status, &reason)
	if err != nil {
		t.Fatalf("get album failed row: %v", err)
	}
	if status != musicStatusFailed || reason.Valid {
		t.Fatalf("album failed row = %s/%#v, want failed/null", status, reason)
	}
}

// Spotify omits total_tracks for some entries and sends album types the schema
// does not know; the scanner maps both to NULL, and without a COALESCE guard
// the enrichment UPDATE erased values a previous, richer match had stored.
func TestProcessMusicBatchPreservesAlbumEnrichmentWhenSpotifyOmitsIt(t *testing.T) {
	app := setupMusicScanner(t)
	ctx := context.Background()

	_, err := app.queries.UpsertMusician(ctx, database.UpsertMusicianParams{
		Name:      "Test Artist",
		SortName:  "Test Artist",
		SpotifyID: sql.NullString{String: "artist123", Valid: true},
	})
	if err != nil {
		t.Fatalf("seed musician: %v", err)
	}
	album, err := app.queries.UpsertAlbum(ctx, database.UpsertAlbumParams{
		Title:       "Test Album",
		SortTitle:   "Test Album",
		Musician:    sql.NullString{String: "Test Artist", Valid: true},
		SpotifyID:   sql.NullString{String: "album123", Valid: true},
		TotalTracks: sql.NullInt64{Int64: 12, Valid: true},
	})
	if err != nil {
		t.Fatalf("seed album: %v", err)
	}
	err = app.queries.UpdateMusicAlbumEnrichment(ctx, database.UpdateMusicAlbumEnrichmentParams{ID: album.ID, AlbumType: sql.NullString{String: "album", Valid: true}})
	if err != nil {
		t.Fatalf("seed album type: %v", err)
	}

	app.ffprobe = &scannertest.CountingProbe{Default: testMusicMetadata()}
	app.spotify = &musicScannerSpotifyStub{
		artist: &spotifyapi.FullArtist{
			SimpleArtist: spotifyapi.SimpleArtist{ID: spotifyapi.ID("artist123"), Name: "Test Artist"},
		},
		album: &spotifyapi.FullAlbum{
			SimpleAlbum: spotifyapi.SimpleAlbum{ID: spotifyapi.ID("album123"), Name: "Test Album", AlbumType: "ep"},
		},
	}

	file := testTrackFile(t)
	scanned, skipped, errCount := app.processMusicBatchForTest(t, ctx, []scanner.ScanFile{file})
	if scanned != 1 || skipped != 0 || errCount != 0 {
		t.Fatalf("scan result scanned=%d skipped=%d errors=%d, want 1/0/0", scanned, skipped, errCount)
	}

	var albumType sql.NullString
	var totalTracks sql.NullInt64
	err = app.tx.DB.QueryRow("SELECT album_type, total_tracks FROM albums WHERE id = ?", album.ID).
		Scan(&albumType, &totalTracks)
	if err != nil {
		t.Fatalf("get album enrichment: %v", err)
	}
	if albumType.String != "album" || totalTracks.Int64 != 12 {
		t.Fatalf("album enrichment = %#v %#v, want the stored values preserved", albumType, totalTracks)
	}
}

// Enrichment writes stamp updated_at so clients can poll for changed
// artists and albums; derived-value and image writes are covered by the
// unchanged-metadata tests above.
func TestSpotifyEnrichmentWritesAdvanceUpdatedAt(t *testing.T) {
	ctx := context.Background()
	cases := []struct {
		name, table, predicate string
		update                 func(*database.Queries) error
	}{
		{"album enrichment", "albums", "total_tracks=10 AND album_type='single'", func(q *database.Queries) error {
			return q.UpdateMusicAlbumEnrichment(ctx, database.UpdateMusicAlbumEnrichmentParams{ID: 1, TotalTracks: sql.NullInt64{Int64: 10, Valid: true}, AlbumType: sql.NullString{String: "single", Valid: true}})
		}},
		{"artist Spotify ID", "musicians", "spotify_id='artist'", func(q *database.Queries) error {
			return q.SetMusicArtistSpotifyID(ctx, database.SetMusicArtistSpotifyIDParams{ID: 1, SpotifyID: sql.NullString{String: "artist", Valid: true}})
		}},
		{"album Spotify ID", "albums", "spotify_id='album'", func(q *database.Queries) error {
			return q.SetMusicAlbumSpotifyID(ctx, database.SetMusicAlbumSpotifyIDParams{ID: 1, SpotifyID: sql.NullString{String: "album", Valid: true}})
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			s := setupMusicScanner(t)
			_, err := s.tx.DB.Exec(`INSERT INTO musicians(id,name,sort_name,updated_at) VALUES(1,'Artist','Artist',datetime('now','-1 hour'));
 INSERT INTO albums(id,title,sort_title,updated_at) VALUES(1,'Album','Album',datetime('now','-1 hour'));`)
			if err != nil {
				t.Fatal(err)
			}
			var old string
			err = s.tx.DB.QueryRow("SELECT updated_at FROM " + tc.table + " WHERE id=1").Scan(&old)
			if err != nil {
				t.Fatal(err)
			}
			err = tc.update(s.queries)
			if err != nil {
				t.Fatal(err)
			}
			count := scannertest.CountRows(t, s.tx.DB, "SELECT count(*) FROM "+tc.table+" WHERE id=1 AND updated_at>? AND ("+tc.predicate+")", old)
			if count != 1 {
				t.Fatal("metadata or timestamp did not update")
			}
		})
	}
}

func TestArtistSortPersistenceAndSpotifyReconciliation(t *testing.T) {
	cases := []struct {
		artist, sorts string
		want          map[string]string
	}{
		{"Artist One, Artist Two, Artist One", "Z & Middle & A", map[string]string{"Artist One": "A", "Artist Two": "Middle"}},
		{"Artist One, Artist Two", "Same & Same", map[string]string{"Artist One": "Same", "Artist Two": "Same"}},
		{"Artist One, Artist Two, Artist Three", " & Middle & ", map[string]string{"Artist One": "Artist One", "Artist Two": "Middle", "Artist Three": "Artist Three"}},
		{"Artist One, Artist Two", "One, Artist & Two, Artist", map[string]string{"Artist One": "One, Artist", "Artist Two": "Two, Artist"}},
		{"Artist One, Artist Two", "One, Artist, Two, Artist", map[string]string{"Artist One": "Artist One", "Artist Two": "Artist Two"}},
	}
	for i, tc := range cases {
		for _, retry := range []bool{false, true} {
			t.Run(fmt.Sprintf("%d/retry=%v", i, retry), func(t *testing.T) {
				s := setupMusicScanner(t)
				artist := tc.artist
				if retry {
					// Ampersand-only credits remain combined offline.
					artist = strings.ReplaceAll(artist, ", ", " & ")
				}
				scanTaggedTrack(t, s, newMusicScanContext(nil), filepath.Join(t.TempDir(), "track"), 1, ffprobe.FormatTags{Title: "Track", Artist: artist, SortArtist: tc.sorts})
				if retry {
					s.spotify = &musicScannerSpotifyStub{artistErr: &spotifyapi.MatchError{Info: spotifyapi.MatchDebugInfo{Reason: spotifyapi.MatchReasonNoResults}}}
					err := s.retrySpotify(context.Background(), newMusicScanContext(nil), newScanReport(Status{}))
					if err != nil {
						t.Fatal(err)
					}
				}
				for name, want := range tc.want {
					var got string
					err := s.tx.DB.QueryRow("SELECT sort_name FROM musicians WHERE name=?", name).Scan(&got)
					if err != nil || got != want {
						t.Fatalf("%s: %q want %q: %v", name, got, want, err)
					}
				}
				var raw string
				err := s.tx.DB.QueryRow("SELECT artist_sort FROM music_track_metadata").Scan(&raw)
				if err != nil || raw != tc.sorts {
					t.Fatalf("raw sort lost: %q: %v", raw, err)
				}
			})
		}
	}
}

func TestMergedArtistSortVotesOncePerTrack(t *testing.T) {
	s := setupMusicScanner(t)
	s.spotify = &musicScannerSpotifyStub{artist: &spotifyapi.FullArtist{SimpleArtist: spotifyapi.SimpleArtist{ID: "shared"}}}
	dir := t.TempDir()
	scanTaggedTrack(t, s, newMusicScanContext(nil), filepath.Join(dir, "one"), 1,
		ffprobe.FormatTags{Title: "One", Artist: "Artist One, Artist Two, Artist One", SortArtist: "Z & A & Z"})
	scanTaggedTrack(t, s, newMusicScanContext(nil), filepath.Join(dir, "two"), 1,
		ffprobe.FormatTags{Title: "Two", Artist: "Artist Two", SortArtist: "Z"})
	var sort string
	err := s.tx.DB.QueryRow("SELECT sort_name FROM musicians").Scan(&sort)
	if err != nil || sort != "A" {
		t.Fatalf("one vote per track tie: %q: %v", sort, err)
	}
	count := scannertest.CountRows(t, s.tx.DB, "SELECT count(*) FROM musicians")
	if count != 1 {
		t.Fatal("artists did not merge")
	}
	scanTaggedTrack(t, s, newMusicScanContext(nil), filepath.Join(dir, "three"), 1,
		ffprobe.FormatTags{Title: "Three", Artist: "Artist One", SortArtist: "Z"})
	err = s.tx.DB.QueryRow("SELECT sort_name FROM musicians").Scan(&sort)
	if err != nil || sort != "Z" {
		t.Fatalf("majority: %q: %v", sort, err)
	}
}

// seedMatchedSpotifyEntities stores an artist and album that were already
// matched to Spotify and carry local images, and returns their rows.
func seedMatchedSpotifyEntities(t *testing.T, app *Scanner) (database.Musician, database.Album) {
	t.Helper()
	musicianIdentity, err := app.queries.UpsertMusician(context.Background(), database.UpsertMusicianParams{
		Name:      "Existing Artist",
		SortName:  "Existing Artist",
		SpotifyID: sql.NullString{String: "artist123", Valid: true},
		Thumb:     sql.NullString{String: "file:///music/artist.jpg", Valid: true},
	})
	if err != nil {
		t.Fatalf("seed musician: %v", err)
	}
	musician, err := app.queries.GetMusicianByID(context.Background(), musicianIdentity.ID)
	if err != nil {
		t.Fatal(err)
	}
	albumIdentity, err := app.queries.UpsertAlbum(context.Background(), database.UpsertAlbumParams{
		Title:     "Existing Album",
		SortTitle: "Existing Album",
		Musician:  sql.NullString{String: "Existing Artist", Valid: true},
		SpotifyID: sql.NullString{String: "album123", Valid: true},
		Cover:     sql.NullString{String: "file:///music/cover.jpg", Valid: true},
	})
	if err != nil {
		t.Fatalf("seed album: %v", err)
	}
	album, err := app.queries.GetAlbumByID(context.Background(), albumIdentity.ID)
	if err != nil {
		t.Fatal(err)
	}
	return musician, album
}

// testTrackFile is the one-track library every Spotify test scans.
func testTrackFile(t *testing.T) scanner.ScanFile {
	t.Helper()
	return scanner.ScanFile{Path: filepath.Join(t.TempDir(), "Test Track.m4a"), Ext: "m4a", Size: 5}
}

// seedLocalMusicianAndAlbum stores an artist and album that have never been
// matched to Spotify and returns their rows.
func seedLocalMusicianAndAlbum(t *testing.T, app *Scanner) (database.Musician, database.Album) {
	t.Helper()
	musicianIdentity, err := app.queries.UpsertMusician(context.Background(), database.UpsertMusicianParams{
		Name:     "Test Artist",
		SortName: "Test Artist",
	})
	if err != nil {
		t.Fatalf("seed musician: %v", err)
	}
	musician, err := app.queries.GetMusicianByID(context.Background(), musicianIdentity.ID)
	if err != nil {
		t.Fatal(err)
	}

	albumIdentity, err := app.queries.UpsertAlbum(context.Background(), database.UpsertAlbumParams{
		Title:     "Test Album",
		SortTitle: "Test Album",
		Musician:  sql.NullString{String: "Test Artist", Valid: true},
	})
	if err != nil {
		t.Fatalf("seed album: %v", err)
	}
	album, err := app.queries.GetAlbumByID(context.Background(), albumIdentity.ID)
	if err != nil {
		t.Fatal(err)
	}
	return musician, album
}
