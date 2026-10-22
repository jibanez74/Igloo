package main

import (
	"database/sql"
	"errors"
	"igloo/cmd/internal/database"
	"igloo/cmd/internal/helpers"
	"igloo/cmd/internal/spotify"
	"net/http"
)

func (app *Application) GetMusiciansAlphabetical(w http.ResponseWriter, r *http.Request) {
	page, perPage := parsePageAndPerPage(r, libraryDefaultPerPage, libraryMaxPerPage)

	offset := (page - 1) * perPage

	total, err := app.Queries.GetMusiciansCount(r.Context())
	if err != nil {
		app.serverError(w, err, "failed to get musicians count", "failed to fetch musicians count")
		return
	}

	musicians, err := app.Queries.GetMusiciansAlphabetical(r.Context(), database.GetMusiciansAlphabeticalParams{
		Limit:  perPage,
		Offset: offset,
	})

	if err != nil {
		app.serverError(w, err, "failed to get musicians", "failed to fetch musicians")
		return
	}

	helpers.WriteJSON(w, http.StatusOK, helpers.JSONResponse{
		Data: map[string]any{
			"musicians":   musicians,
			"total":       total,
			"page":        page,
			"per_page":    perPage,
			"total_pages": totalPages(total, perPage),
		},
	})
}

// musicianWithSpotify is the musician row plus its Spotify page, like
// albumWithSpotify.
type musicianWithSpotify struct {
	database.Musician
	SpotifyURL *string `json:"spotify_url"`
}

func (app *Application) GetMusicianDetails(w http.ResponseWriter, r *http.Request) {
	id, ok := parseIDParam(w, r, "id", "invalid musician id")
	if !ok {
		return
	}

	ctx := r.Context()

	musician, err := app.Queries.GetMusicianByID(ctx, id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			helpers.ErrorJSON(w, errors.New("musician not found"), http.StatusNotFound)
			return
		}

		app.serverError(w, err, "failed to get musician", "failed to fetch musician from server", "id", id)
		return
	}

	albums, err := app.Queries.GetAlbumsByMusicianID(ctx, id)
	if err != nil {
		app.serverError(w, err, "failed to get albums for musician", "failed to fetch musician albums from server",
			"musician_id", id)
		return
	}

	tracks, err := app.Queries.GetTracksByMusicianID(ctx, sql.NullInt64{Int64: id, Valid: true})
	if err != nil {
		app.serverError(w, err, "failed to get tracks for musician", "failed to fetch musician tracks from server",
			"musician_id", id)
		return
	}

	genres, err := app.Queries.GetGenresByMusicianID(ctx, id)
	if err != nil {
		app.serverError(w, err, "failed to get genres for musician", "failed to fetch musician genres from server",
			"musician_id", id)
		return
	}

	var totalDuration int64

	for _, track := range tracks {
		totalDuration += track.Duration
	}

	helpers.WriteJSON(w, http.StatusOK, helpers.JSONResponse{
		Data: map[string]any{
			"musician":       musicianWithSpotify{Musician: musician, SpotifyURL: spotifyURLField(musician.SpotifyID, spotify.ArtistURL)},
			"albums":         albums,
			"tracks":         tracks,
			"genres":         genres,
			"total_duration": totalDuration,
		},
	})
}
