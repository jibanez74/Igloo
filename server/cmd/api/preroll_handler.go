package main

import (
	"context"
	"database/sql"
	"errors"
	"math/rand/v2"
	"net/http"
	"time"

	"igloo/cmd/internal/database"
	"igloo/cmd/internal/helpers"
	"igloo/cmd/internal/preroll"

	"github.com/go-chi/chi/v5"
)

const (
	// How long a play waits for a cold in-theaters cache before starting with
	// library trailers only. The refresh keeps running and serves the next play.
	prerollTheatersWait = 5 * time.Second
	// TMDB's now-playing page holds at most this many movies; fetching this
	// many library rows beyond the count keeps TMDB-id dedupe from starving
	// the library share.
	prerollLibraryOverfetch   = 12
	prerollQueueFailedMessage = "failed to build the trailer queue"
)

type prerollTrailerResponse struct {
	Title      string `json:"title"`
	YouTubeKey string `json:"youtube_key"`
	Source     string `json:"source"`
	MovieID    *int64 `json:"movie_id"`
	TmdbID     *int64 `json:"tmdb_id"`
}

func prerollTrailerPayload(trailer preroll.Trailer) prerollTrailerResponse {
	payload := prerollTrailerResponse{
		Title:      trailer.Title,
		YouTubeKey: trailer.YouTubeKey,
		Source:     trailer.Source,
	}
	if trailer.MovieID != 0 {
		movieID := trailer.MovieID
		payload.MovieID = &movieID
	}
	if trailer.TmdbID != 0 {
		tmdbID := trailer.TmdbID
		payload.TmdbID = &tmdbID
	}
	return payload
}

// GetMoviePreroll returns the ordered trailer queue to play before a movie,
// built from the caller's saved preferences. Trailers stream from YouTube;
// the server only hands out keys it already holds.
func (app *Application) GetMoviePreroll(w http.ResponseWriter, r *http.Request) {
	userID, ok := app.currentUserID(w, r)
	if !ok {
		return
	}

	media, err := parseMediaID(chi.URLParam(r, "id"), mediaKindMovie)
	if err != nil {
		helpers.ErrorJSON(w, err, http.StatusBadRequest)
		return
	}

	ctx := r.Context()
	movie, err := app.Queries.GetMovieByID(ctx, media.ID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			helpers.ErrorJSON(w, errors.New(movieNotFoundMessage), http.StatusNotFound)
			return
		}
		app.Logger.Error("failed to load movie for preroll", "error", err, "movie_id", media.ID)
		helpers.ErrorJSON(w, errors.New(prerollQueueFailedMessage))
		return
	}

	prefs, err := app.loadTrailerPreferences(ctx, userID)
	if err != nil {
		app.Logger.Error("failed to load trailer preferences for preroll", "error", err, "user_id", userID)
		helpers.ErrorJSON(w, errors.New(prerollQueueFailedMessage))
		return
	}

	trailers := make([]prerollTrailerResponse, 0, prefs.Count)
	if prefs.Enabled {
		queue, err := app.buildPrerollQueue(ctx, prefs, movie)
		if err != nil {
			app.Logger.Error("failed to build preroll queue", "error", err, "movie_id", media.ID)
			helpers.ErrorJSON(w, errors.New(prerollQueueFailedMessage))
			return
		}
		for _, trailer := range queue {
			trailers = append(trailers, prerollTrailerPayload(trailer))
		}
	}

	helpers.WriteJSON(w, http.StatusOK, helpers.JSONResponse{Error: false, Data: map[string]any{"trailers": trailers}})
}

// buildPrerollQueue loads the library pool first, so a library preference the
// library can fill never waits on TMDB; theaters are resolved for the other
// sources, and for a library preference only to top up a short queue.
func (app *Application) buildPrerollQueue(ctx context.Context, prefs preroll.Preferences, movie database.Movie) ([]preroll.Trailer, error) {
	rows, err := app.Queries.GetRandomLibraryTrailers(ctx, database.GetRandomLibraryTrailersParams{
		ExcludeMovieID: movie.ID,
		RowLimit:       int64(prefs.Count + prerollLibraryOverfetch),
	})
	if err != nil {
		return nil, err
	}
	library := make([]preroll.Trailer, 0, len(rows))
	for _, row := range rows {
		library = append(library, preroll.Trailer{
			Title:      row.Title,
			YouTubeKey: row.YoutubeKey,
			Source:     preroll.SourceLibrary,
			MovieID:    row.MovieID,
			TmdbID:     row.TmdbID.Int64,
		})
	}

	rng := rand.New(rand.NewPCG(rand.Uint64(), rand.Uint64()))
	excludeTmdbID := movie.TmdbID.Int64
	if prefs.Source == preroll.SourceLibrary {
		queue := preroll.Select(prefs, library, nil, excludeTmdbID, rng)
		if len(queue) >= prefs.Count {
			return queue, nil
		}
	}

	waitCtx, cancel := context.WithTimeout(ctx, prerollTheatersWait)
	theaters := app.PrerollTheaters.Trailers(waitCtx)
	cancel()

	return preroll.Select(prefs, library, theaters, excludeTmdbID, rng), nil
}
