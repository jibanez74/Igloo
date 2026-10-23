//go:build integration

package spotify

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"igloo/cmd/internal/helpers"
)

// newIntegrationClient builds a client from SPOTIFY_CLIENT_ID and
// SPOTIFY_CLIENT_SECRET, seeded from the repo root .env when one exists. Missing
// credentials fail rather than skip: the live suite is opt-in through make
// test-spotify-integration, and a silent skip would report a green run that
// tested nothing.
func newIntegrationClient(t *testing.T) *Client {
	t.Helper()

	_, currentFile, _, _ := runtime.Caller(0)
	repoRoot := filepath.Join(filepath.Dir(currentFile), "..", "..", "..", "..")
	err := helpers.LoadEnvFile(filepath.Join(repoRoot, ".env"))
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("load repo .env: %v", err)
	}

	clientID := os.Getenv("SPOTIFY_CLIENT_ID")
	clientSecret := os.Getenv("SPOTIFY_CLIENT_SECRET")
	if clientID == "" || clientSecret == "" {
		t.Fatal("SPOTIFY_CLIENT_ID and SPOTIFY_CLIENT_SECRET must be set")
	}

	client, err := New(context.Background(), clientID, clientSecret)
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	return client
}

// Popularity, followers and genres are not decoded: Spotify stopped sending
// them to Development Mode apps.
func TestSpotifyIntegration(t *testing.T) {
	client := newIntegrationClient(t)
	ctx := context.Background()

	t.Run("matches an artist by name", func(t *testing.T) {
		artist, err := client.SearchArtistByName(ctx, "Radiohead")
		if err != nil {
			t.Fatalf("SearchArtistByName: %v", err)
		}
		if artist.ID == "" || artist.Name != "Radiohead" || len(artist.Images) == 0 || artist.Images[0].URL == "" {
			t.Fatalf("artist = %+v, want Radiohead with an id and an image", artist)
		}
	})

	t.Run("matches an album and loads its details with every track", func(t *testing.T) {
		album, err := client.MatchAlbum(ctx, AlbumQuery{Title: "OK Computer", Artist: "Radiohead", Year: 1997, TotalTracks: 12})
		if err != nil {
			t.Fatalf("MatchAlbum: %v", err)
		}
		if album.ID == "" || !strings.HasPrefix(album.Name, "OK Computer") || album.TotalTracks == 0 || len(album.Images) == 0 || album.AlbumType != "album" {
			t.Fatalf("album = %+v, want the OK Computer album with tracks and a cover", album)
		}
		releaseYear := album.ReleaseDateTime().Year()
		if releaseYear != 1997 {
			t.Fatalf("release date %q (%s) = year %d, want 1997", album.ReleaseDate, album.ReleaseDatePrecision, releaseYear)
		}
		if len(album.Tracks) != int(album.TotalTracks) {
			t.Fatalf("tracks = %d, want all %d", len(album.Tracks), album.TotalTracks)
		}
		first := album.Tracks[0]
		if first.ID == "" || first.Name == "" || first.TrackNumber != 1 || first.DiscNumber != 1 || first.Duration == 0 {
			t.Fatalf("first track = %+v, want id, name, position and length", first)
		}
		if len(album.Artists) == 0 || album.Artists[0].ID == "" {
			t.Fatalf("artists = %+v, want the credited artist with an id", album.Artists)
		}

		artist, err := client.GetArtist(ctx, album.Artists[0].ID)
		if err != nil {
			t.Fatalf("GetArtist: %v", err)
		}
		if artist.Name != "Radiohead" || len(artist.Images) == 0 {
			t.Fatalf("artist = %+v, want Radiohead with an image", artist)
		}
	})

	t.Run("pages a long album's track list", func(t *testing.T) {
		// The Beatles ("the White Album", super deluxe edition) lists 107 tracks.
		album, err := client.GetAlbum(ctx, "1WMVvswNzB9i2UMh9svso5")
		if err != nil {
			t.Fatalf("GetAlbum: %v", err)
		}
		if album.TotalTracks <= spotifyAlbumTracksPageLimit || len(album.Tracks) != int(album.TotalTracks) {
			t.Fatalf("tracks = %d of %d, want every page of a >%d-track album", len(album.Tracks), album.TotalTracks, spotifyAlbumTracksPageLimit)
		}
	})

	t.Run("reports an unmatched album as a match error", func(t *testing.T) {
		_, err := client.MatchAlbum(ctx, AlbumQuery{Title: "Qzxv Wplk Nonexistent Album 4417", Artist: "Nobody Qzxv"})
		matchErr, ok := errors.AsType[*MatchError](err)
		if !ok {
			t.Fatalf("err = %v, want a MatchError", err)
		}
		unmatched := matchErr.Info.Reason == MatchReasonNoResults || matchErr.Info.Reason == MatchReasonScoreBelowThreshold
		if !unmatched {
			t.Fatalf("reason = %q, want no_results or score_below_threshold", matchErr.Info.Reason)
		}
	})

	t.Run("searches albums", func(t *testing.T) {
		albums, err := client.SearchAlbums(ctx, "Abbey Road")
		if err != nil {
			t.Fatalf("SearchAlbums: %v", err)
		}
		if len(albums) == 0 {
			t.Fatal("SearchAlbums returned no albums")
		}
		first := albums[0]
		if first.ID == "" || first.Name == "" || len(first.Artists) == 0 || first.ReleaseDate == "" ||
			!strings.HasPrefix(first.ExternalURLs["spotify"], "https://open.spotify.com/album/") {
			t.Fatalf("first album = %+v, want id, name, artists, release date and a Spotify URL", first)
		}
	})

	t.Run("searches tracks", func(t *testing.T) {
		tracks, err := client.SearchTracks(ctx, "Paranoid Android")
		if err != nil {
			t.Fatalf("SearchTracks: %v", err)
		}
		if len(tracks) == 0 {
			t.Fatal("SearchTracks returned no tracks")
		}
		first := tracks[0]
		if first.ID == "" || first.Name == "" || first.Duration == 0 || first.Album.Name == "" ||
			!strings.HasPrefix(first.ExternalURLs["spotify"], "https://open.spotify.com/track/") {
			t.Fatalf("first track = %+v, want id, name, duration, album and a Spotify URL", first)
		}
	})
}
