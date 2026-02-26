package main

import (
	"database/sql"
	"errors"
	"igloo/cmd/internal/helpers"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"strconv"

	"github.com/go-chi/chi/v5"
)

// GetLatestMovies returns the 12 most recently added movies from the database.
// Response includes id, title, poster_path (path only; frontend builds full URL), and year.
func (app *Application) GetLatestMovies(w http.ResponseWriter, r *http.Request) {
	movies, err := app.Queries.GetLatestMovies(r.Context())
	if err != nil {
		app.Logger.Error("failed to get latest movies", "error", err)
		helpers.ErrorJSON(w, err)
		return
	}

	res := helpers.JSONResponse{
		Error: false,
		Data:  map[string]any{"movies": movies},
	}

	helpers.WriteJSON(w, http.StatusOK, res)
}

// GetMovieDetails returns a movie with all related data (cast, crew, genres, production companies, extra videos).
// Uses a read-only transaction so all data is from a single consistent snapshot.
func (app *Application) GetMovieDetails(w http.ResponseWriter, r *http.Request) {
	idParam := chi.URLParam(r, "id")
	id, err := strconv.ParseInt(idParam, 10, 64)
	if err != nil {
		helpers.ErrorJSON(w, errors.New("invalid movie id"), http.StatusBadRequest)
		return
	}

	ctx := r.Context()

	tx, err := app.DB.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		app.Logger.Error("failed to begin transaction", "error", err)
		helpers.ErrorJSON(w, errors.New("failed to fetch movie from server"))
		return
	}
	defer tx.Rollback()

	qtx := app.Queries.WithTx(tx)

	movie, err := qtx.GetMovieByID(ctx, id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			helpers.ErrorJSON(w, errors.New("movie not found"), http.StatusNotFound)
			return
		}

		app.Logger.Error("failed to get movie", "error", err, "id", id)
		helpers.ErrorJSON(w, errors.New("failed to fetch movie from server"))
		return
	}

	cast, err := qtx.GetCastByMovieID(ctx, id)
	if err != nil {
		app.Logger.Error("failed to get cast for movie", "error", err, "movie_id", id)
		helpers.ErrorJSON(w, errors.New("failed to fetch movie cast from server"))
		return
	}

	crew, err := qtx.GetCrewByMovieID(ctx, id)
	if err != nil {
		app.Logger.Error("failed to get crew for movie", "error", err, "movie_id", id)
		helpers.ErrorJSON(w, errors.New("failed to fetch movie crew from server"))
		return
	}

	genres, err := qtx.GetGenresByMovieID(ctx, id)
	if err != nil {
		app.Logger.Error("failed to get genres for movie", "error", err, "movie_id", id)
		helpers.ErrorJSON(w, errors.New("failed to fetch movie genres from server"))
		return
	}

	productionCompanies, err := qtx.GetProductionCompaniesByMovieID(ctx, id)
	if err != nil {
		app.Logger.Error("failed to get production companies for movie", "error", err, "movie_id", id)
		helpers.ErrorJSON(w, errors.New("failed to fetch movie production companies from server"))
		return
	}

	extraVideos, err := qtx.GetMovieExtraVideos(ctx, id)
	if err != nil {
		app.Logger.Error("failed to get extra videos for movie", "error", err, "movie_id", id)
		helpers.ErrorJSON(w, errors.New("failed to fetch movie extra videos from server"))
		return
	}

	res := helpers.JSONResponse{
		Error: false,
		Data: map[string]any{
			"movie":                movie,
			"cast":                 cast,
			"crew":                 crew,
			"genres":               genres,
			"production_companies": productionCompanies,
			"extra_videos":         extraVideos,
		},
	}

	helpers.WriteJSON(w, http.StatusOK, res)
}

// StreamMovie streams the movie file for playback (direct stream, no transcoding).
func (app *Application) StreamMovie(w http.ResponseWriter, r *http.Request) {
	idParam := chi.URLParam(r, "id")
	id, err := strconv.ParseInt(idParam, 10, 64)
	if err != nil {
		helpers.ErrorJSON(w, errors.New("invalid movie id"), http.StatusBadRequest)
		return
	}

	movie, err := app.Queries.GetMovieForDirectStream(r.Context(), id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			helpers.ErrorJSON(w, errors.New("movie not found"), http.StatusNotFound)
			return
		}

		app.Logger.Error("failed to get movie for streaming", "error", err, "id", id)
		helpers.ErrorJSON(w, errors.New("failed to fetch movie from server"))
		return
	}

	file, err := os.Open(movie.FilePath)
	if err != nil {
		if os.IsNotExist(err) {
			app.Logger.Error("movie file not found on disk", "path", movie.FilePath, "id", id)
			helpers.ErrorJSON(w, errors.New("movie file not found"), http.StatusNotFound)
			return
		}

		app.Logger.Error("failed to open movie file", "error", err, "path", movie.FilePath)
		helpers.ErrorJSON(w, errors.New("failed to open movie file"))
		return
	}
	defer file.Close()

	stat, err := file.Stat()
	if err != nil {
		app.Logger.Error("failed to stat movie file", "error", err, "path", movie.FilePath)
		helpers.ErrorJSON(w, errors.New("failed to read movie file"))
		return
	}

	contentType := movie.MimeType
	if contentType == "" {
		ext := filepath.Ext(movie.FileName)
		contentType = mime.TypeByExtension(ext)
		if contentType == "" {
			contentType = "application/octet-stream"
		}
	}

	w.Header().Set("Content-Type", contentType)

	http.ServeContent(w, r, movie.FileName, stat.ModTime(), file)
}
