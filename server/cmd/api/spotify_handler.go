package main

import (
	"database/sql"
	"errors"
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

	helpers.WriteJSON(w, http.StatusOK, helpers.JSONResponse{
		Error: false,
		Data: map[string]any{
			"results": app.mapSpotifyAlbumSearchResults(r, albums),
		},
	})
}

func (app *Application) ensureSpotifyAvailable(w http.ResponseWriter) bool {
	if app.Spotify != nil {
		return true
	}

	helpers.ErrorJSON(w, errors.New("Spotify search is unavailable"), http.StatusServiceUnavailable)
	return false
}

func (app *Application) mapSpotifyAlbumSearchResults(r *http.Request, albums []spotifylib.SimpleAlbum) []spotifyAlbumSearchResult {
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
			app.Logger.Error("failed to look up existing album by spotify id", "error", err, "spotify_id", spotifyID)
		}

		mapped = append(mapped, result)
	}

	return mapped
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
