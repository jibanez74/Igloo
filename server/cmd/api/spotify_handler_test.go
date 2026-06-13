package main

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"igloo/cmd/internal/database"
	"igloo/cmd/internal/helpers"
	spotifyapi "igloo/cmd/internal/spotify"

	"github.com/go-chi/chi/v5"
	spotifylib "github.com/zmb3/spotify/v2"
)

type spotifyHandlerStub struct {
	albums       []spotifylib.SimpleAlbum
	searchErr    error
	searchTitles []string
}

func (s *spotifyHandlerStub) SearchAndGetAlbumDetails(_ context.Context, _, _ string) (*spotifylib.FullAlbum, error) {
	return nil, nil
}

func (s *spotifyHandlerStub) SearchAlbums(_ context.Context, title string) ([]spotifylib.SimpleAlbum, error) {
	s.searchTitles = append(s.searchTitles, title)
	if s.searchErr != nil {
		return nil, s.searchErr
	}

	return s.albums, nil
}

func (s *spotifyHandlerStub) SearchArtistByName(_ context.Context, _ string) (*spotifylib.FullArtist, error) {
	return nil, nil
}

func (s *spotifyHandlerStub) ClearAllCaches() {}

var _ spotifyapi.SpotifyInterface = (*spotifyHandlerStub)(nil)

func TestGetSpotifyStatus_HTTP(t *testing.T) {
	app := setupTestApp(t)
	defer app.DB.Close()

	router := chi.NewRouter()
	router.Get("/api/spotify/status", app.GetSpotifyStatus)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/spotify/status", nil)
	router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", w.Code, w.Body.String())
	}

	var unavailableResp struct {
		Error bool `json:"error"`
		Data  struct {
			Available bool `json:"available"`
		} `json:"data"`
	}
	err := json.NewDecoder(w.Body).Decode(&unavailableResp)
	if err != nil {
		t.Fatalf("decode unavailable response: %v", err)
	}
	if unavailableResp.Data.Available {
		t.Fatal("expected Spotify status to be unavailable when app.Spotify is nil")
	}

	app.Spotify = &spotifyHandlerStub{}
	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/api/spotify/status", nil)
	router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", w.Code, w.Body.String())
	}

	var availableResp struct {
		Error bool `json:"error"`
		Data  struct {
			Available bool `json:"available"`
		} `json:"data"`
	}
	err = json.NewDecoder(w.Body).Decode(&availableResp)
	if err != nil {
		t.Fatalf("decode available response: %v", err)
	}
	if !availableResp.Data.Available {
		t.Fatal("expected Spotify status to be available when app.Spotify is configured")
	}
}

func TestSearchSpotifyAlbums_HTTPMarksExistingLibraryMatches(t *testing.T) {
	app := setupTestApp(t)
	defer app.DB.Close()

	ctx := context.Background()
	existingAlbum, err := app.Queries.UpsertAlbum(ctx, database.UpsertAlbumParams{
		Title:     "Blue Record",
		SortTitle: "Blue Record",
		SpotifyID: helpers.NullString("album123"),
		Musician:  helpers.NullString("The Band"),
	})
	if err != nil {
		t.Fatalf("insert existing album: %v", err)
	}

	stub := &spotifyHandlerStub{
		albums: []spotifylib.SimpleAlbum{
			{
				ID:          spotifylib.ID("album123"),
				Name:        "Blue Record",
				AlbumType:   "album",
				ReleaseDate: "2026-01-02",
				TotalTracks: spotifylib.Numeric(10),
				Artists: []spotifylib.SimpleArtist{
					{Name: "The Band"},
				},
				Images: []spotifylib.Image{
					{URL: "https://i.scdn.co/image/cover.jpg", Height: 640, Width: 640},
				},
				ExternalURLs: map[string]string{
					"spotify": "https://open.spotify.com/album/album123",
				},
			},
			{
				ID:          spotifylib.ID("album456"),
				Name:        "Blue Record Live",
				AlbumType:   "album",
				ReleaseDate: "2026",
				TotalTracks: spotifylib.Numeric(8),
			},
		},
	}
	app.Spotify = stub

	router := chi.NewRouter()
	router.Post("/api/spotify/albums/search", app.SearchSpotifyAlbums)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/spotify/albums/search", strings.NewReader(`{"title":"  Blue Record  "}`))
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", w.Code, w.Body.String())
	}

	var resp struct {
		Error bool `json:"error"`
		Data  struct {
			Results []struct {
				SpotifyID        string   `json:"spotify_id"`
				Title            string   `json:"title"`
				ArtistNames      []string `json:"artist_names"`
				ReleaseDate      string   `json:"release_date"`
				AlbumType        string   `json:"album_type"`
				TotalTracks      int      `json:"total_tracks"`
				CoverURL         string   `json:"cover_url"`
				SpotifyURL       string   `json:"spotify_url"`
				AlreadyInLibrary bool     `json:"already_in_library"`
				LibraryAlbumID   *int64   `json:"library_album_id"`
			} `json:"results"`
		} `json:"data"`
	}
	err = json.NewDecoder(w.Body).Decode(&resp)
	if err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp.Error {
		t.Fatalf("expected success response: %+v", resp)
	}
	if len(resp.Data.Results) != 2 {
		t.Fatalf("result count = %d, want 2", len(resp.Data.Results))
	}
	first := resp.Data.Results[0]
	if first.SpotifyID != "album123" || first.Title != "Blue Record" {
		t.Fatalf("first result = %+v, want Blue Record album123", first)
	}
	if len(first.ArtistNames) != 1 || first.ArtistNames[0] != "The Band" {
		t.Fatalf("artist names = %+v, want The Band", first.ArtistNames)
	}
	if first.ReleaseDate != "2026-01-02" || first.AlbumType != "album" || first.TotalTracks != 10 {
		t.Fatalf("metadata = %+v, want release date/type/tracks", first)
	}
	if first.CoverURL != "https://i.scdn.co/image/cover.jpg" {
		t.Fatalf("cover_url = %q", first.CoverURL)
	}
	if first.SpotifyURL != "https://open.spotify.com/album/album123" {
		t.Fatalf("spotify_url = %q", first.SpotifyURL)
	}
	if !first.AlreadyInLibrary {
		t.Fatalf("expected first result to be flagged as already in library: %+v", first)
	}
	if first.LibraryAlbumID == nil || *first.LibraryAlbumID != existingAlbum.ID {
		t.Fatalf("library_album_id = %v, want %d", first.LibraryAlbumID, existingAlbum.ID)
	}
	if len(stub.searchTitles) != 1 || stub.searchTitles[0] != "Blue Record" {
		t.Fatalf("search titles = %+v, want trimmed Blue Record", stub.searchTitles)
	}
}

func TestSearchSpotifyAlbums_HTTPUnavailable(t *testing.T) {
	app := setupTestApp(t)
	defer app.DB.Close()

	router := chi.NewRouter()
	router.Post("/api/spotify/albums/search", app.SearchSpotifyAlbums)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/spotify/albums/search", strings.NewReader(`{"title":"Blue Record"}`))
	router.ServeHTTP(w, req)

	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want 503, body = %s", w.Code, w.Body.String())
	}
}

func TestSearchSpotifyAlbums_HTTPValidationAndUpstreamErrors(t *testing.T) {
	app := setupTestApp(t)
	defer app.DB.Close()

	router := chi.NewRouter()
	router.Post("/api/spotify/albums/search", app.SearchSpotifyAlbums)
	app.Spotify = &spotifyHandlerStub{}

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/spotify/albums/search", strings.NewReader(`{"title":"   "}`))
	router.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("empty title status = %d, want 400, body = %s", w.Code, w.Body.String())
	}

	app.Spotify = &spotifyHandlerStub{searchErr: errors.New("spotify down")}
	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/api/spotify/albums/search", strings.NewReader(`{"title":"Blue Record"}`))
	router.ServeHTTP(w, req)
	if w.Code != http.StatusBadGateway {
		t.Fatalf("upstream error status = %d, want 502, body = %s", w.Code, w.Body.String())
	}
}
