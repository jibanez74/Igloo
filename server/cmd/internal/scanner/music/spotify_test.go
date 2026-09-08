package music

import (
	"context"
	"database/sql"
	"errors"
	"path/filepath"
	"testing"

	"igloo/cmd/internal/database"
	"igloo/cmd/internal/ffprobe"
	"igloo/cmd/internal/scanner"
	spotifyapi "igloo/cmd/internal/spotify"

	spotifylib "github.com/zmb3/spotify/v2"
)

func TestProcessMusicBatchAssignsFirstSpotifyImages(t *testing.T) {
	app := setupMusicScanner(t)
	defer app.db.Close()

	app.ffprobe = &countingMusicScannerFfprobe{result: testMusicMetadata()}
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

	file := scanner.ScanFile{
		Path: filepath.Join(t.TempDir(), "Test Track.m4a"),
		Ext:  "m4a",
		Size: 5,
	}

	scanned, skipped, errCount := app.processMusicBatchForTest(t, context.Background(), []scanner.ScanFile{file})
	if scanned != 1 || skipped != 0 || errCount != 0 {
		t.Fatalf("scan result scanned=%d skipped=%d errors=%d, want 1/0/0", scanned, skipped, errCount)
	}

	var albumCover sql.NullString
	err := app.db.QueryRow("SELECT cover FROM albums WHERE spotify_id = ?", "album123").Scan(&albumCover)
	if err != nil {
		t.Fatalf("get album cover: %v", err)
	}
	if !albumCover.Valid || albumCover.String != "https://i.scdn.co/album-first.jpg" {
		t.Fatalf("album cover = %#v, want first Spotify album image", albumCover)
	}

	var musicianThumb sql.NullString
	err = app.db.QueryRow("SELECT thumb FROM musicians WHERE spotify_id = ?", "artist123").Scan(&musicianThumb)
	if err != nil {
		t.Fatalf("get musician thumb: %v", err)
	}
	if !musicianThumb.Valid || musicianThumb.String != "https://i.scdn.co/artist-first.jpg" {
		t.Fatalf("musician thumb = %#v, want first Spotify artist image", musicianThumb)
	}
}

func TestProcessMusicBatchRefreshesExistingSpotifyImages(t *testing.T) {
	app := setupMusicScanner(t)
	defer app.db.Close()

	seededMusician, err := app.queries.UpsertMusician(context.Background(), database.UpsertMusicianParams{
		Name:      "Existing Artist",
		SortName:  "Existing Artist",
		SpotifyID: sql.NullString{String: "artist123", Valid: true},
		Thumb:     sql.NullString{String: "file:///music/artist.jpg", Valid: true},
	})
	if err != nil {
		t.Fatalf("seed musician: %v", err)
	}

	seededAlbum, err := app.queries.UpsertAlbum(context.Background(), database.UpsertAlbumParams{
		Title:     "Existing Album",
		SortTitle: "Existing Album",
		Musician:  sql.NullString{String: "Existing Artist", Valid: true},
		SpotifyID: sql.NullString{String: "album123", Valid: true},
		Cover:     sql.NullString{String: "file:///music/cover.jpg", Valid: true},
	})
	if err != nil {
		t.Fatalf("seed album: %v", err)
	}

	app.ffprobe = &countingMusicScannerFfprobe{result: testMusicMetadata()}
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

	file := scanner.ScanFile{
		Path: filepath.Join(t.TempDir(), "Test Track.m4a"),
		Ext:  "m4a",
		Size: 5,
	}

	scanned, skipped, errCount := app.processMusicBatchForTest(t, context.Background(), []scanner.ScanFile{file})
	if scanned != 1 || skipped != 0 || errCount != 0 {
		t.Fatalf("scan result scanned=%d skipped=%d errors=%d, want 1/0/0", scanned, skipped, errCount)
	}

	var albumCover sql.NullString
	err = app.db.QueryRow("SELECT cover FROM albums WHERE id = ?", seededAlbum.ID).Scan(&albumCover)
	if err != nil {
		t.Fatalf("get album cover: %v", err)
	}
	if !albumCover.Valid || albumCover.String != "https://i.scdn.co/refreshed-album.jpg" {
		t.Fatalf("album cover = %#v, want refreshed Spotify album image", albumCover)
	}

	var musicianThumb sql.NullString
	err = app.db.QueryRow("SELECT thumb FROM musicians WHERE id = ?", seededMusician.ID).Scan(&musicianThumb)
	if err != nil {
		t.Fatalf("get musician thumb: %v", err)
	}
	if !musicianThumb.Valid || musicianThumb.String != "https://i.scdn.co/refreshed-artist.jpg" {
		t.Fatalf("musician thumb = %#v, want refreshed Spotify artist image", musicianThumb)
	}
}

func TestProcessMusicBatchPreservesExistingImagesWithoutSpotifyMatch(t *testing.T) {
	app := setupMusicScanner(t)
	defer app.db.Close()

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
	app.ffprobe = &countingMusicScannerFfprobe{result: testMusicMetadata()}
	app.spotify = &musicScannerSpotifyStub{
		artistErr: noSpotifyMatch,
		albumErr:  noSpotifyMatch,
	}

	file := scanner.ScanFile{
		Path: filepath.Join(t.TempDir(), "Test Track.m4a"),
		Ext:  "m4a",
		Size: 5,
	}

	scanned, skipped, errCount := app.processMusicBatchForTest(t, context.Background(), []scanner.ScanFile{file})
	if scanned != 1 || skipped != 0 || errCount != 0 {
		t.Fatalf("scan result scanned=%d skipped=%d errors=%d, want 1/0/0", scanned, skipped, errCount)
	}

	var albumCover sql.NullString
	err = app.db.QueryRow("SELECT cover FROM albums WHERE title = ? AND musician = ?", "Test Album", "Test Artist").Scan(&albumCover)
	if err != nil {
		t.Fatalf("get album cover: %v", err)
	}
	if !albumCover.Valid || albumCover.String != "file:///music/cover.jpg" {
		t.Fatalf("album cover = %#v, want preserved existing cover", albumCover)
	}

	var musicianThumb sql.NullString
	err = app.db.QueryRow("SELECT thumb FROM musicians WHERE name = ?", "Test Artist").Scan(&musicianThumb)
	if err != nil {
		t.Fatalf("get musician thumb: %v", err)
	}
	if !musicianThumb.Valid || musicianThumb.String != "file:///music/artist.jpg" {
		t.Fatalf("musician thumb = %#v, want preserved existing thumb", musicianThumb)
	}
}

func TestProcessMusicBatchPreservesExistingImagesWhenSpotifyMatchHasNoImages(t *testing.T) {
	app := setupMusicScanner(t)
	defer app.db.Close()

	seededMusician, err := app.queries.UpsertMusician(context.Background(), database.UpsertMusicianParams{
		Name:      "Existing Artist",
		SortName:  "Existing Artist",
		SpotifyID: sql.NullString{String: "artist123", Valid: true},
		Thumb:     sql.NullString{String: "file:///music/artist.jpg", Valid: true},
	})
	if err != nil {
		t.Fatalf("seed musician: %v", err)
	}

	seededAlbum, err := app.queries.UpsertAlbum(context.Background(), database.UpsertAlbumParams{
		Title:     "Existing Album",
		SortTitle: "Existing Album",
		Musician:  sql.NullString{String: "Existing Artist", Valid: true},
		SpotifyID: sql.NullString{String: "album123", Valid: true},
		Cover:     sql.NullString{String: "file:///music/cover.jpg", Valid: true},
	})
	if err != nil {
		t.Fatalf("seed album: %v", err)
	}

	app.ffprobe = &countingMusicScannerFfprobe{result: testMusicMetadata()}
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

	file := scanner.ScanFile{
		Path: filepath.Join(t.TempDir(), "Test Track.m4a"),
		Ext:  "m4a",
		Size: 5,
	}

	scanned, skipped, errCount := app.processMusicBatchForTest(t, context.Background(), []scanner.ScanFile{file})
	if scanned != 1 || skipped != 0 || errCount != 0 {
		t.Fatalf("scan result scanned=%d skipped=%d errors=%d, want 1/0/0", scanned, skipped, errCount)
	}

	var albumCover sql.NullString
	err = app.db.QueryRow("SELECT cover FROM albums WHERE id = ?", seededAlbum.ID).Scan(&albumCover)
	if err != nil {
		t.Fatalf("get album cover: %v", err)
	}
	if !albumCover.Valid || albumCover.String != "file:///music/cover.jpg" {
		t.Fatalf("album cover = %#v, want preserved existing cover", albumCover)
	}

	var musicianThumb sql.NullString
	err = app.db.QueryRow("SELECT thumb FROM musicians WHERE id = ?", seededMusician.ID).Scan(&musicianThumb)
	if err != nil {
		t.Fatalf("get musician thumb: %v", err)
	}
	if !musicianThumb.Valid || musicianThumb.String != "file:///music/artist.jpg" {
		t.Fatalf("musician thumb = %#v, want preserved existing thumb", musicianThumb)
	}
}

func TestProcessMusicBatchIgnoresEmbeddedArtworkWithoutSpotifyMatch(t *testing.T) {
	app := setupMusicScanner(t)
	defer app.db.Close()

	metadata := testMusicMetadata()
	metadata.Streams = append(metadata.Streams, ffprobe.Stream{
		Index:     1,
		CodecName: "mjpeg",
		CodecType: "video",
		Disposition: ffprobe.StreamDisposition{
			AttachedPic: 1,
		},
	})
	app.ffprobe = &countingMusicScannerFfprobe{result: metadata}

	file := scanner.ScanFile{
		Path: filepath.Join(t.TempDir(), "Test Track.m4a"),
		Ext:  "m4a",
		Size: 5,
	}

	scanned, skipped, errCount := app.processMusicBatchForTest(t, context.Background(), []scanner.ScanFile{file})
	if scanned != 1 || skipped != 0 || errCount != 0 {
		t.Fatalf("scan result scanned=%d skipped=%d errors=%d, want 1/0/0", scanned, skipped, errCount)
	}

	var albumCover sql.NullString
	err := app.db.QueryRow("SELECT cover FROM albums WHERE title = ?", "Test Album").Scan(&albumCover)
	if err != nil {
		t.Fatalf("get album cover: %v", err)
	}
	if albumCover.Valid {
		t.Fatalf("album cover = %#v, want no embedded artwork assigned", albumCover)
	}

	var musicianThumb sql.NullString
	err = app.db.QueryRow("SELECT thumb FROM musicians WHERE name = ?", "Test Artist").Scan(&musicianThumb)
	if err != nil {
		t.Fatalf("get musician thumb: %v", err)
	}
	if musicianThumb.Valid {
		t.Fatalf("musician thumb = %#v, want no embedded artwork assigned", musicianThumb)
	}
}

func TestProcessMusicBatchRespectsPersistedSpotifyUnmatchedRows(t *testing.T) {
	app := setupMusicScanner(t)
	defer app.db.Close()

	musician, err := app.queries.UpsertMusician(context.Background(), database.UpsertMusicianParams{
		Name:     "Test Artist",
		SortName: "Test Artist",
	})
	if err != nil {
		t.Fatalf("seed musician: %v", err)
	}

	album, err := app.queries.UpsertAlbum(context.Background(), database.UpsertAlbumParams{
		Title:     "Test Album",
		SortTitle: "Test Album",
		Musician:  sql.NullString{String: "Test Artist", Valid: true},
	})
	if err != nil {
		t.Fatalf("seed album: %v", err)
	}

	err = app.queries.SaveMusicArtistIdentity(context.Background(), database.SaveMusicArtistIdentityParams{IdentityKey: scanner.NormalizedScanCacheKey(musician.Name), MusicianID: musician.ID})
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
	app.ffprobe = &countingMusicScannerFfprobe{result: testMusicMetadata()}
	app.spotify = spotifyStub

	file := scanner.ScanFile{
		Path: filepath.Join(t.TempDir(), "Test Track.m4a"),
		Ext:  "m4a",
		Size: 5,
	}

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
	defer app.db.Close()

	musician, err := app.queries.UpsertMusician(context.Background(), database.UpsertMusicianParams{
		Name:     "Test Artist",
		SortName: "Test Artist",
	})
	if err != nil {
		t.Fatalf("seed musician: %v", err)
	}

	album, err := app.queries.UpsertAlbum(context.Background(), database.UpsertAlbumParams{
		Title:     "Test Album",
		SortTitle: "Test Album",
		Musician:  sql.NullString{String: "Test Artist", Valid: true},
	})
	if err != nil {
		t.Fatalf("seed album: %v", err)
	}

	err = app.queries.UpsertMusicSpotifyMatch(context.Background(), database.UpsertMusicSpotifyMatchParams{
		EntityType: musicSpotifyEntityMusician,
		EntityID:   musician.ID,
		Status:     musicSpotifyStatusFailed,
		Error:      sql.NullString{String: "temporary artist error", Valid: true},
	})
	if err != nil {
		t.Fatalf("seed musician spotify match: %v", err)
	}

	err = app.queries.UpsertMusicSpotifyMatch(context.Background(), database.UpsertMusicSpotifyMatchParams{
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
	app.ffprobe = &countingMusicScannerFfprobe{result: testMusicMetadata()}
	app.spotify = spotifyStub

	file := scanner.ScanFile{
		Path: filepath.Join(t.TempDir(), "Test Track.m4a"),
		Ext:  "m4a",
		Size: 5,
	}

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
	defer app.db.Close()

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

	_, err = app.db.Exec(`
 CREATE TABLE image_writes(entity TEXT);
 CREATE TRIGGER count_artist_image AFTER UPDATE OF thumb ON musicians BEGIN INSERT INTO image_writes VALUES('artist'); END;
 CREATE TRIGGER count_album_image AFTER UPDATE OF cover ON albums BEGIN INSERT INTO image_writes VALUES('album'); END;
 `)
	if err != nil {
		t.Fatal(err)
	}

	app.ffprobe = &countingMusicScannerFfprobe{result: testMusicMetadata()}
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

	file := scanner.ScanFile{
		Path: filepath.Join(t.TempDir(), "Test Track.m4a"),
		Ext:  "m4a",
		Size: 5,
	}

	scanned, skipped, errCount := app.processMusicBatchForTest(t, context.Background(), []scanner.ScanFile{file})
	if scanned != 1 || skipped != 0 || errCount != 0 {
		t.Fatalf("scan result scanned=%d skipped=%d errors=%d, want 1/0/0", scanned, skipped, errCount)
	}

	count := countScannerRows(t, app.db, "SELECT count(*) FROM image_writes")
	if count != 0 {
		t.Fatalf("unchanged images caused %d writes", count)
	}
}

func TestProcessMusicBatchPersistsSpotifyMatchedRows(t *testing.T) {
	app := setupMusicScanner(t)
	defer app.db.Close()

	app.ffprobe = &countingMusicScannerFfprobe{result: testMusicMetadata()}
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

	file := scanner.ScanFile{
		Path: filepath.Join(t.TempDir(), "Test Track.m4a"),
		Ext:  "m4a",
		Size: 5,
	}
	scanned, skipped, errCount := app.processMusicBatchForTest(t, context.Background(), []scanner.ScanFile{file})
	if scanned != 1 || skipped != 0 || errCount != 0 {
		t.Fatalf("scan result scanned=%d skipped=%d errors=%d, want 1/0/0", scanned, skipped, errCount)
	}

	var musicianStatus string
	var musicianSpotifyID sql.NullString
	err := app.db.QueryRow(`
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
	err = app.db.QueryRow(`
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

func TestProcessMusicBatchPersistsSpotifyMetadataAndGenres(t *testing.T) {
	app := setupMusicScanner(t)
	defer app.db.Close()

	app.ffprobe = &countingMusicScannerFfprobe{result: testMusicMetadata()}
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

	file := scanner.ScanFile{
		Path: filepath.Join(t.TempDir(), "Test Track.m4a"),
		Ext:  "m4a",
		Size: 5,
	}
	scanned, skipped, errCount := app.processMusicBatchForTest(t, context.Background(), []scanner.ScanFile{file})
	if scanned != 1 || skipped != 0 || errCount != 0 {
		t.Fatalf("scan result scanned=%d skipped=%d errors=%d, want 1/0/0", scanned, skipped, errCount)
	}

	var musicianSpotifyID sql.NullString
	var musicianPopularity sql.NullFloat64
	var musicianFollowers sql.NullInt64
	var musicianSummary sql.NullString
	var musicianThumb sql.NullString
	err := app.db.QueryRow(`
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
	err = app.db.QueryRow(`
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

	if got := countScannerRows(t, app.db, "SELECT COUNT(*) FROM genres WHERE genre_type = ? AND tag IN (?, ?, ?)", "music", "dream pop", "indie rock", "shoegaze"); got != 3 {
		t.Fatalf("Spotify genre count = %d, want 3", got)
	}
	if got := countScannerRows(t, app.db, "SELECT COUNT(*) FROM genres WHERE genre_type = ? AND tag = ?", "music", "dream pop"); got != 1 {
		t.Fatalf("shared dream pop genre rows = %d, want 1", got)
	}
	if got := countScannerRows(t, app.db, `
		SELECT COUNT(*)
		FROM musician_genres AS mg
		INNER JOIN musicians AS m ON m.id = mg.musician_id
		INNER JOIN genres AS g ON g.id = mg.genre_id
		WHERE m.name = ? AND g.tag IN (?, ?)
	`, "Test Artist", "dream pop", "indie rock"); got != 2 {
		t.Fatalf("musician_genres count = %d, want 2", got)
	}
	if got := countScannerRows(t, app.db, `
		SELECT COUNT(*)
		FROM album_genres AS ag
		INNER JOIN albums AS a ON a.id = ag.album_id
		INNER JOIN genres AS g ON g.id = ag.genre_id
		WHERE a.title = ? AND a.musician = ? AND g.tag IN (?, ?)
	`, "Test Album", "Test Artist", "dream pop", "shoegaze"); got != 2 {
		t.Fatalf("album_genres count = %d, want 2", got)
	}
	if got := countScannerRows(t, app.db, `
		SELECT COUNT(*)
		FROM musician_genres AS mg
		INNER JOIN musicians AS m ON m.id = mg.musician_id
		INNER JOIN album_genres AS ag ON ag.genre_id = mg.genre_id
		INNER JOIN albums AS a ON a.id = ag.album_id
		INNER JOIN genres AS g ON g.id = mg.genre_id
		WHERE m.name = ? AND a.title = ? AND a.musician = ? AND g.tag = ?
	`, "Test Artist", "Test Album", "Test Artist", "dream pop"); got != 1 {
		t.Fatalf("shared dream pop relationship count = %d, want 1", got)
	}
}

func TestProcessMusicBatchPersistsSpotifyUnmatchedDetails(t *testing.T) {
	app := setupMusicScanner(t)
	defer app.db.Close()

	app.ffprobe = &countingMusicScannerFfprobe{result: testMusicMetadata()}
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

	file := scanner.ScanFile{
		Path: filepath.Join(t.TempDir(), "Test Track.m4a"),
		Ext:  "m4a",
		Size: 5,
	}
	scanned, skipped, errCount := app.processMusicBatchForTest(t, context.Background(), []scanner.ScanFile{file})
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
	err := app.db.QueryRow(`
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
	err = app.db.QueryRow(`
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
	app := setupMusicScanner(t)
	defer app.db.Close()

	app.ffprobe = &countingMusicScannerFfprobe{result: testMusicMetadata()}
	app.spotify = &musicScannerSpotifyStub{
		artistErr: errors.New("artist temporary failure"),
		albumErr:  errors.New("album temporary failure"),
	}

	file := scanner.ScanFile{
		Path: filepath.Join(t.TempDir(), "Test Track.m4a"),
		Ext:  "m4a",
		Size: 5,
	}
	scanned, skipped, errCount := app.processMusicBatchForTest(t, context.Background(), []scanner.ScanFile{file})
	if scanned != 1 || skipped != 0 || errCount != 0 {
		t.Fatalf("scan result scanned=%d skipped=%d errors=%d, want 1/0/0", scanned, skipped, errCount)
	}

	var status string
	var errorText sql.NullString
	err := app.db.QueryRow(`
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

	err = app.db.QueryRow(`
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
