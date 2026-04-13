package main

import (
	"errors"
	"igloo/cmd/internal/helpers"
	"igloo/cmd/internal/tmdb"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
)

// TmdbSearchMovies searches TMDB by title/year or fetches a single movie by TMDB ID.
// POST /api/movies/:id/tmdb-search   body: { title, year?, tmdb_id? }
func (app *Application) TmdbSearchMovies(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	var payload struct {
		Title  string `json:"title"`
		Year   int    `json:"year"`
		TmdbID int    `json:"tmdb_id"`
	}

	err := helpers.ReadJSON(w, r, &payload, 0)
	if err != nil {
		helpers.ErrorJSON(w, errors.New("invalid request body"), http.StatusBadRequest)
		return
	}

	if payload.TmdbID > 0 {
		movie := &tmdb.TmdbMovie{TmdbID: payload.TmdbID}

		err = app.Tmdb.GetTmdbMovieByID(ctx, movie)
		if err != nil {
			app.Logger.Error("tmdb get by id failed", "error", err, "tmdb_id", payload.TmdbID)
			helpers.ErrorJSON(w, errors.New("failed to fetch movie from TMDB"))
			return
		}

		helpers.WriteJSON(w, http.StatusOK, helpers.JSONResponse{
			Error: false,
			Data: map[string]any{
				"results": []tmdb.TmdbMovieSearchResult{tmdb.NewTmdbMovieSearchResult(movie)},
			},
		})

		return
	}

	if payload.Title == "" {
		helpers.ErrorJSON(w, errors.New("title or tmdb_id is required"), http.StatusBadRequest)
		return
	}

	searchTitle := normalizeMovieTitleForSearch(payload.Title)
	if searchTitle == "" {
		searchTitle = payload.Title
	}

	results, err := app.Tmdb.SearchMoviesByTitleAndYear(ctx, searchTitle)
	if err != nil {
		app.Logger.Error("tmdb search failed", "error", err, "title", payload.Title, "normalized_title", searchTitle)
		helpers.ErrorJSON(w, errors.New("TMDB search failed"))
		return
	}

	ranked := rankTmdbMatches(results, searchTitle, payload.Year)
	mapped := make([]tmdb.TmdbMovieSearchResult, 0, len(results))
	for _, candidate := range ranked {
		mapped = append(mapped, tmdb.NewTmdbMovieSearchResult(candidate.Movie))
	}

	helpers.WriteJSON(w, http.StatusOK, helpers.JSONResponse{
		Error: false,
		Data:  map[string]any{"results": mapped},
	})
}

// GetMoviesInTheaters returns the latest movies currently playing in theaters.
// The response is limited to a maximum of 12 movies.
func (app *Application) GetMoviesInTheaters(w http.ResponseWriter, r *http.Request) {
	movies, err := app.Tmdb.GetMoviesInTheaters(r.Context())
	if err != nil {
		app.Logger.Error("failed to get movies in theaters", "error", err)
		helpers.ErrorJSON(w, errors.New("failed to fetch movies in theaters"))
		return
	}

	if len(movies) > helpers.TMDB_MAX_ITEMS {
		movies = movies[:helpers.TMDB_MAX_ITEMS]
	}

	res := helpers.JSONResponse{
		Error: false,
		Data: map[string]any{
			"movies": movies,
		},
	}

	helpers.WriteJSON(w, http.StatusOK, res)
}

// GetMovieByTmdbID returns a single movie from TMDB by its TMDB ID.
func (app *Application) GetMovieByTmdbID(w http.ResponseWriter, r *http.Request) {
	idParam := chi.URLParam(r, "id")
	id, err := strconv.Atoi(idParam)
	if err != nil || id <= 0 {
		helpers.ErrorJSON(w, errors.New("invalid tmdb movie id"), http.StatusBadRequest)
		return
	}

	movie := &tmdb.TmdbMovie{TmdbID: id}
	err = app.Tmdb.GetTmdbMovieByID(r.Context(), movie)
	if err != nil {
		app.Logger.Error("failed to get movie from tmdb", "error", err, "tmdb_id", id)
		helpers.ErrorJSON(w, errors.New("failed to fetch movie from tmdb"))
		return
	}

	res := helpers.JSONResponse{
		Error: false,
		Data: map[string]any{
			"movie": movie,
		},
	}

	helpers.WriteJSON(w, http.StatusOK, res)
}
