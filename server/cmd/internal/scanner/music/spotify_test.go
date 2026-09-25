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

	spotifylib "github.com/zmb3/spotify/v2"
)

func TestProcessMusicBatchAssignsFirstSpotifyImages(t *testing.T) {
	app := setupMusicScanner(t)

	app.ffprobe = &scannertest.CountingProbe{Default: testMusicMetadata()}
	app.spotify = &musicScannerSpotifyStub{
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

	err = app.queries.UpsertMusicSpotifyMatch(context.Background(), database.UpsertMusicSpotifyMatchParams{
		EntityType: musicSpotifyEntityMusician,
		EntityID:   musician.ID,
		Status:     musicSpotifyStatusUnmatched,
		Reason:     sql.NullString{String: "no_results", Valid: true},
	})
	if err != nil {
		t.Fatalf("seed musician spotify match: %v", err)
	}

	err = app.queries.UpsertMusicSpotifyMatch(context.Background(), database.UpsertMusicSpotifyMatchParams{
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

	err := app.queries.UpsertMusicSpotifyMatch(context.Background(), database.UpsertMusicSpotifyMatchParams{
		EntityType: musicSpotifyEntityMusician,
		EntityID:   musician.ID,
		Status:     musicSpotifyStatusFailed,
	})
	if err != nil {
		t.Fatalf("seed musician spotify match: %v", err)
	}

	err = app.queries.UpsertMusicSpotifyMatch(context.Background(), database.UpsertMusicSpotifyMatchParams{
		EntityType: musicSpotifyEntityAlbum,
		EntityID:   album.ID,
		Status:     musicSpotifyStatusFailed,
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

	file := testTrackFile(t)
	scanned, skipped, errCount := app.processMusicBatchForTest(t, context.Background(), []scanner.ScanFile{file})
	if scanned != 1 || skipped != 0 || errCount != 0 {
		t.Fatalf("scan result scanned=%d skipped=%d errors=%d, want 1/0/0", scanned, skipped, errCount)
	}

	var musicianStatus string
	var musicianSpotifyID sql.NullString
	err := app.tx.DB.QueryRow(`
		SELECT msm.status, m.spotify_id
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
	err = app.tx.DB.QueryRow(`
		SELECT msm.status, a.spotify_id
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

func TestProcessMusicBatchPersistsSpotifyMetadataAndGenres(t *testing.T) {
	app := setupMusicScanner(t)

	app.ffprobe = &scannertest.CountingProbe{Default: testMusicMetadata()}
	app.spotify = &musicScannerSpotifyStub{
		artist: &spotifylib.FullArtist{
			SimpleArtist: spotifylib.SimpleArtist{
				ID:   spotifylib.ID("artist-meta-123"),
				Name: "Test Artist",
			},
			Popularity: 76,
			Genres:     []string{"dream pop", "indie rock"},
			Followers: spotifylib.Followers{
				Count: 1500000,
			},
			Images: []spotifylib.Image{
				{URL: "https://i.scdn.co/artist-meta.jpg"},
			},
		},
		album: &spotifylib.FullAlbum{
			SimpleAlbum: spotifylib.SimpleAlbum{
				ID:                   spotifylib.ID("album-meta-123"),
				Name:                 "Test Album",
				ReleaseDate:          "2024-04-12",
				ReleaseDatePrecision: "day",
				TotalTracks:          10,
				Images: []spotifylib.Image{
					{URL: "https://i.scdn.co/album-meta.jpg"},
				},
			},
			Popularity: 64,
			Genres:     []string{"dream pop", "shoegaze"},
		},
	}

	file := testTrackFile(t)
	scanned, skipped, errCount := app.processMusicBatchForTest(t, context.Background(), []scanner.ScanFile{file})
	if scanned != 1 || skipped != 0 || errCount != 0 {
		t.Fatalf("scan result scanned=%d skipped=%d errors=%d, want 1/0/0", scanned, skipped, errCount)
	}

	var musicianSpotifyID sql.NullString
	var musicianPopularity sql.NullFloat64
	var musicianFollowers sql.NullInt64
	var musicianSummary sql.NullString
	var musicianThumb sql.NullString
	err := app.tx.DB.QueryRow(`
		SELECT spotify_id, spotify_popularity, spotify_followers, summary, thumb
		FROM musicians
		WHERE name = ?
	`, "Test Artist").Scan(&musicianSpotifyID, &musicianPopularity, &musicianFollowers, &musicianSummary, &musicianThumb)
	if err != nil {
		t.Fatalf("get musician metadata: %v", err)
	}
	if !musicianSpotifyID.Valid || musicianSpotifyID.String != "artist-meta-123" {
		t.Fatalf("musician spotify_id = %#v, want artist-meta-123", musicianSpotifyID)
	}
	if !musicianPopularity.Valid || musicianPopularity.Float64 != 76 {
		t.Fatalf("musician popularity = %#v, want 76", musicianPopularity)
	}
	if !musicianFollowers.Valid || musicianFollowers.Int64 != 1500000 {
		t.Fatalf("musician followers = %#v, want 1500000", musicianFollowers)
	}
	wantSummary := "Test Artist known for dream pop, indie rock is a popular artist with 1.5M followers on Spotify."
	if !musicianSummary.Valid || musicianSummary.String != wantSummary {
		t.Fatalf("musician summary = %#v, want %q", musicianSummary, wantSummary)
	}
	if !musicianThumb.Valid || musicianThumb.String != "https://i.scdn.co/artist-meta.jpg" {
		t.Fatalf("musician thumb = %#v, want Spotify artist image", musicianThumb)
	}

	var albumSpotifyID sql.NullString
	var albumPopularity sql.NullFloat64
	var totalTracks sql.NullInt64
	var releaseDate sql.NullString
	var year sql.NullInt64
	var cover sql.NullString
	err = app.tx.DB.QueryRow(`
		SELECT spotify_id, spotify_popularity, total_tracks, release_date, year, cover
		FROM albums
		WHERE title = ? AND musician = ?
	`, "Test Album", "Test Artist").Scan(&albumSpotifyID, &albumPopularity, &totalTracks, &releaseDate, &year, &cover)
	if err != nil {
		t.Fatalf("get album metadata: %v", err)
	}
	if !albumSpotifyID.Valid || albumSpotifyID.String != "album-meta-123" {
		t.Fatalf("album spotify_id = %#v, want album-meta-123", albumSpotifyID)
	}
	if !albumPopularity.Valid || albumPopularity.Float64 != 64 {
		t.Fatalf("album popularity = %#v, want 64", albumPopularity)
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

	got := scannertest.CountRows(t, app.tx.DB, "SELECT COUNT(*) FROM genres WHERE genre_type = ? AND tag IN (?, ?, ?)", "music", "dream pop", "indie rock", "shoegaze")
	if got != 3 {
		t.Fatalf("Spotify genre count = %d, want 3", got)
	}
	got = scannertest.CountRows(t, app.tx.DB, "SELECT COUNT(*) FROM genres WHERE genre_type = ? AND tag = ?", "music", "dream pop")
	if got != 1 {
		t.Fatalf("shared dream pop genre rows = %d, want 1", got)
	}
	got = scannertest.CountRows(t, app.tx.DB, `
		SELECT COUNT(*)
		FROM musician_genres AS mg
		INNER JOIN musicians AS m ON m.id = mg.musician_id
		INNER JOIN genres AS g ON g.id = mg.genre_id
		WHERE m.name = ? AND g.tag IN (?, ?)
	`, "Test Artist", "dream pop", "indie rock")
	if got != 2 {
		t.Fatalf("musician_genres count = %d, want 2", got)
	}
	got = scannertest.CountRows(t, app.tx.DB, `
		SELECT COUNT(*)
		FROM album_genres AS ag
		INNER JOIN albums AS a ON a.id = ag.album_id
		INNER JOIN genres AS g ON g.id = ag.genre_id
		WHERE a.title = ? AND a.musician = ? AND g.tag IN (?, ?)
	`, "Test Album", "Test Artist", "dream pop", "shoegaze")
	if got != 2 {
		t.Fatalf("album_genres count = %d, want 2", got)
	}
	got = scannertest.CountRows(t, app.tx.DB, `
		SELECT COUNT(*)
		FROM musician_genres AS mg
		INNER JOIN musicians AS m ON m.id = mg.musician_id
		INNER JOIN album_genres AS ag ON ag.genre_id = mg.genre_id
		INNER JOIN albums AS a ON a.id = ag.album_id
		INNER JOIN genres AS g ON g.id = mg.genre_id
		WHERE m.name = ? AND a.title = ? AND a.musician = ? AND g.tag = ?
	`, "Test Artist", "Test Album", "Test Artist", "dream pop")
	if got != 1 {
		t.Fatalf("shared dream pop relationship count = %d, want 1", got)
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
	var reason sql.NullString
	err := app.tx.DB.QueryRow(`
		SELECT msm.status, msm.reason
		FROM music_spotify_matches AS msm
		INNER JOIN musicians AS m ON m.id = msm.entity_id
		WHERE msm.entity_type = ? AND m.name = ?
	`, musicSpotifyEntityMusician, "Test Artist").Scan(&status, &reason)
	if err != nil {
		t.Fatalf("get musician unmatched row: %v", err)
	}
	if status != musicSpotifyStatusUnmatched || !reason.Valid || reason.String != "score_below_threshold" {
		t.Fatalf("musician status/reason = %s/%#v, want unmatched/score_below_threshold", status, reason)
	}

	var albumReason sql.NullString
	err = app.tx.DB.QueryRow(`
		SELECT msm.reason
		FROM music_spotify_matches AS msm
		INNER JOIN albums AS a ON a.id = msm.entity_id
		WHERE msm.entity_type = ? AND a.title = ?
	`, musicSpotifyEntityAlbum, "Test Album").Scan(&albumReason)
	if err != nil {
		t.Fatalf("get album unmatched row: %v", err)
	}
	if !albumReason.Valid || albumReason.String != "no_results" {
		t.Fatalf("album reason = %#v, want no_results", albumReason)
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
		FROM music_spotify_matches AS msm
		INNER JOIN musicians AS m ON m.id = msm.entity_id
		WHERE msm.entity_type = ? AND m.name = ?
	`, musicSpotifyEntityMusician, "Test Artist").Scan(&status, &reason)
	if err != nil {
		t.Fatalf("get musician failed row: %v", err)
	}
	if status != musicSpotifyStatusFailed || reason.Valid {
		t.Fatalf("musician failed row = %s/%#v, want failed/null", status, reason)
	}

	err = app.tx.DB.QueryRow(`
		SELECT msm.status, msm.reason
		FROM music_spotify_matches AS msm
		INNER JOIN albums AS a ON a.id = msm.entity_id
		WHERE msm.entity_type = ? AND a.title = ?
	`, musicSpotifyEntityAlbum, "Test Album").Scan(&status, &reason)
	if err != nil {
		t.Fatalf("get album failed row: %v", err)
	}
	if status != musicSpotifyStatusFailed || reason.Valid {
		t.Fatalf("album failed row = %s/%#v, want failed/null", status, reason)
	}
}

// Spotify reports 0 popularity and 0 followers for obscure artists, and the
// scanner maps a zero to NULL. Without a COALESCE guard the enrichment UPDATE
// erased values a previous, richer match had stored.
func TestProcessMusicBatchPreservesEnrichmentWhenSpotifyReportsZeroes(t *testing.T) {
	app := setupMusicScanner(t)
	ctx := context.Background()

	musician, err := app.queries.UpsertMusician(ctx, database.UpsertMusicianParams{
		Name:              "Test Artist",
		SortName:          "Test Artist",
		SpotifyID:         sql.NullString{String: "artist123", Valid: true},
		Summary:           sql.NullString{String: "A stored summary", Valid: true},
		SpotifyPopularity: sql.NullFloat64{Float64: 61, Valid: true},
		SpotifyFollowers:  sql.NullInt64{Int64: 4200, Valid: true},
	})
	if err != nil {
		t.Fatalf("seed musician: %v", err)
	}
	album, err := app.queries.UpsertAlbum(ctx, database.UpsertAlbumParams{
		Title:             "Test Album",
		SortTitle:         "Test Album",
		Musician:          sql.NullString{String: "Test Artist", Valid: true},
		SpotifyID:         sql.NullString{String: "album123", Valid: true},
		SpotifyPopularity: sql.NullFloat64{Float64: 55, Valid: true},
		TotalTracks:       sql.NullInt64{Int64: 12, Valid: true},
	})
	if err != nil {
		t.Fatalf("seed album: %v", err)
	}

	app.ffprobe = &scannertest.CountingProbe{Default: testMusicMetadata()}
	app.spotify = &musicScannerSpotifyStub{
		artist: &spotifylib.FullArtist{
			SimpleArtist: spotifylib.SimpleArtist{ID: spotifylib.ID("artist123"), Name: "Test Artist"},
		},
		album: &spotifylib.FullAlbum{
			SimpleAlbum: spotifylib.SimpleAlbum{ID: spotifylib.ID("album123"), Name: "Test Album"},
		},
	}

	file := testTrackFile(t)
	scanned, skipped, errCount := app.processMusicBatchForTest(t, ctx, []scanner.ScanFile{file})
	if scanned != 1 || skipped != 0 || errCount != 0 {
		t.Fatalf("scan result scanned=%d skipped=%d errors=%d, want 1/0/0", scanned, skipped, errCount)
	}

	var summary sql.NullString
	var popularity sql.NullFloat64
	var followers sql.NullInt64
	err = app.tx.DB.QueryRow("SELECT summary, spotify_popularity, spotify_followers FROM musicians WHERE id = ?", musician.ID).
		Scan(&summary, &popularity, &followers)
	if err != nil {
		t.Fatalf("get musician enrichment: %v", err)
	}
	if popularity.Float64 != 61 || followers.Int64 != 4200 || !summary.Valid {
		t.Fatalf("musician enrichment = %#v %#v %#v, want the stored values preserved", summary, popularity, followers)
	}

	var albumPopularity sql.NullFloat64
	var totalTracks sql.NullInt64
	err = app.tx.DB.QueryRow("SELECT spotify_popularity, total_tracks FROM albums WHERE id = ?", album.ID).
		Scan(&albumPopularity, &totalTracks)
	if err != nil {
		t.Fatalf("get album enrichment: %v", err)
	}
	if albumPopularity.Float64 != 55 || totalTracks.Int64 != 12 {
		t.Fatalf("album enrichment = %#v %#v, want the stored values preserved", albumPopularity, totalTracks)
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
		{"artist enrichment", "musicians", "summary='Summary' AND spotify_popularity=42 AND spotify_followers=100", func(q *database.Queries) error {
			return q.UpdateMusicArtistEnrichment(ctx, database.UpdateMusicArtistEnrichmentParams{ID: 1, Summary: sql.NullString{String: "Summary", Valid: true}, SpotifyPopularity: sql.NullFloat64{Float64: 42, Valid: true}, SpotifyFollowers: sql.NullInt64{Int64: 100, Valid: true}})
		}},
		{"album enrichment", "albums", "spotify_popularity=42 AND total_tracks=10", func(q *database.Queries) error {
			return q.UpdateMusicAlbumEnrichment(ctx, database.UpdateMusicAlbumEnrichmentParams{ID: 1, SpotifyPopularity: sql.NullFloat64{Float64: 42, Valid: true}, TotalTracks: sql.NullInt64{Int64: 10, Valid: true}})
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
	s.spotify = &musicScannerSpotifyStub{artist: &spotifylib.FullArtist{SimpleArtist: spotifylib.SimpleArtist{ID: "shared"}}}
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
