package main

import (
	"database/sql"
	"errors"
	"fmt"
	"igloo/cmd/internal/helpers"
	"net/http"
	"strings"

	spotifylib "github.com/zmb3/spotify/v2"
)

type spotifyAlbumSearchPayload struct {
	Title string `json:"title"`
}

type spotifyAlbumSearchResult struct {
	SpotifyID        string   `json:"spotify_id"`
	Title            string   `json:"title"`
	ArtistNames      []string `json:"artist_names"`
	ReleaseDate      string   `json:"release_date"`
	AlbumType        string   `json:"album_type"`
	TotalTracks      int      `json:"total_tracks"`
	CoverURL         string   `json:"cover_url"`
	SpotifyURL       string   `json:"spotify_url"`
	AlreadyInLibrary bool     `json:"already_in_library"`
	LibraryAlbumID   *int64   `json:"library_album_id,omitempty"`
}

func (app *Application) GetSpotifyStatus(w http.ResponseWriter, r *http.Request) {
	helpers.WriteJSON(w, http.StatusOK, helpers.JSONResponse{
		Error: false,
		Data: map[string]any{
			"available": app.Spotify != nil,
		},
	})
}

func (app *Application) SearchSpotifyAlbums(w http.ResponseWriter, r *http.Request) {
	if !app.ensureSpotifyAvailable(w) {
		return
	}

	var payload spotifyAlbumSearchPayload

	err := helpers.ReadJSON(w, r, &payload, 0)
	if err != nil {
		helpers.ErrorJSON(w, errors.New("invalid request body"), http.StatusBadRequest)
		return
	}

	payload.Title = strings.TrimSpace(payload.Title)
	if payload.Title == "" {
		helpers.ErrorJSON(w, errors.New("title is required"), http.StatusBadRequest)
		return
	}

	albums, err := app.Spotify.SearchAlbums(r.Context(), payload.Title)
	if err != nil {
		app.Logger.Error("spotify album search failed", "error", err, "title", payload.Title)
		helpers.ErrorJSON(w, errors.New("Spotify album search failed"), http.StatusBadGateway)
		return
	}

	results, err := app.mapSpotifyAlbumSearchResults(r, albums)
	if err != nil {
		app.Logger.Error("failed to map spotify album search results", "error", err)
		helpers.ErrorJSON(w, errors.New("failed to map Spotify album search results"), http.StatusInternalServerError)
		return
	}

	helpers.WriteJSON(w, http.StatusOK, helpers.JSONResponse{
		Error: false,
		Data: map[string]any{
			"results": results,
		},
	})
}

type spotifyTrackSearchPayload struct {
	Title string `json:"title"`
}

type spotifyTrackSearchResult struct {
	SpotifyID   string   `json:"spotify_id"`
	Title       string   `json:"title"`
	ArtistNames []string `json:"artist_names"`
	AlbumName   string   `json:"album_name"`
	ReleaseDate string   `json:"release_date"`
	DurationMs  int      `json:"duration_ms"`
	CoverURL    string   `json:"cover_url"`
	SpotifyURL  string   `json:"spotify_url"`
}

// SearchSpotifyTracks backs the "Request Track" picker. Unlike albums, tracks
// have no spotify_id in the library, so results omit an "already in library"
// flag.
func (app *Application) SearchSpotifyTracks(w http.ResponseWriter, r *http.Request) {
	if !app.ensureSpotifyAvailable(w) {
		return
	}

	var payload spotifyTrackSearchPayload

	err := helpers.ReadJSON(w, r, &payload, 0)
	if err != nil {
		helpers.ErrorJSON(w, errors.New("invalid request body"), http.StatusBadRequest)
		return
	}

	payload.Title = strings.TrimSpace(payload.Title)
	if payload.Title == "" {
		helpers.ErrorJSON(w, errors.New("title is required"), http.StatusBadRequest)
		return
	}

	tracks, err := app.Spotify.SearchTracks(r.Context(), payload.Title)
	if err != nil {
		app.Logger.Error("spotify track search failed", "error", err, "title", payload.Title)
		helpers.ErrorJSON(w, errors.New("Spotify track search failed"), http.StatusBadGateway)
		return
	}

	results := mapSpotifyTrackSearchResults(tracks)

	helpers.WriteJSON(w, http.StatusOK, helpers.JSONResponse{
		Error: false,
		Data: map[string]any{
			"results": results,
		},
	})
}

func mapSpotifyTrackSearchResults(tracks []spotifylib.FullTrack) []spotifyTrackSearchResult {
	mapped := make([]spotifyTrackSearchResult, 0, len(tracks))

	for _, track := range tracks {
		spotifyID := track.ID.String()
		if spotifyID == "" {
			continue
		}

		mapped = append(mapped, spotifyTrackSearchResult{
			SpotifyID:   spotifyID,
			Title:       track.Name,
			ArtistNames: spotifyAlbumArtistNames(track.Artists),
			AlbumName:   track.Album.Name,
			ReleaseDate: track.Album.ReleaseDate,
			DurationMs:  int(track.Duration),
			CoverURL:    firstSpotifyAlbumImageURL(track.Album.Images),
			SpotifyURL:  track.ExternalURLs["spotify"],
		})
	}

	return mapped
}

func (app *Application) ensureSpotifyAvailable(w http.ResponseWriter) bool {
	if app.Spotify != nil {
		return true
	}

	helpers.ErrorJSON(w, errors.New("Spotify search is unavailable"), http.StatusServiceUnavailable)
	return false
}

func (app *Application) mapSpotifyAlbumSearchResults(r *http.Request, albums []spotifylib.SimpleAlbum) ([]spotifyAlbumSearchResult, error) {
	mapped := make([]spotifyAlbumSearchResult, 0, len(albums))

	for _, album := range albums {
		spotifyID := album.ID.String()
		if spotifyID == "" {
			continue
		}

		result := spotifyAlbumSearchResult{
			SpotifyID:   spotifyID,
			Title:       album.Name,
			ArtistNames: spotifyAlbumArtistNames(album.Artists),
			ReleaseDate: album.ReleaseDate,
			AlbumType:   album.AlbumType,
			TotalTracks: int(album.TotalTracks),
			CoverURL:    firstSpotifyAlbumImageURL(album.Images),
			SpotifyURL:  album.ExternalURLs["spotify"],
		}

		existingAlbum, err := app.Queries.GetAlbumBySpotifyID(r.Context(), helpers.NullString(spotifyID))
		if err == nil {
			result.AlreadyInLibrary = true
			result.LibraryAlbumID = &existingAlbum.ID
		} else if !errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("failed to look up existing album by spotify id %q: %w", spotifyID, err)
		}

		mapped = append(mapped, result)
	}

	return mapped, nil
}

func spotifyAlbumArtistNames(artists []spotifylib.SimpleArtist) []string {
	names := make([]string, 0, len(artists))

	for _, artist := range artists {
		name := strings.TrimSpace(artist.Name)
		if name == "" {
			continue
		}
		names = append(names, name)
	}

	return names
}

func firstSpotifyAlbumImageURL(images []spotifylib.Image) string {
	if len(images) == 0 {
		return ""
	}

	return images[0].URL
}
