package main

import (
	"database/sql"
	"errors"
	"igloo/cmd/internal/database"
	"igloo/cmd/internal/helpers"
	"igloo/cmd/internal/scanner"
	moviescanner "igloo/cmd/internal/scanner/movie"
	"igloo/cmd/internal/tmdb"
	"net/http"
	"os"
)

func (app *Application) IdentifyMovie(w http.ResponseWriter, r *http.Request) {
	id, ok := parseIDParam(w, r, "id", invalidMovieIDMessage)
	if !ok {
		return
	}

	var payload struct {
		TmdbID int `json:"tmdb_id"`
	}

	if err := helpers.ReadJSON(w, r, &payload); err != nil || payload.TmdbID <= 0 {
		helpers.ErrorJSON(w, errors.New("valid tmdb_id is required"), http.StatusBadRequest)
		return
	}

	if !app.ensureTmdbAvailable(w) {
		return
	}

	ctx := r.Context()

	// Answer an unknown movie before spending a TMDB request on it. The
	// transaction below looks the movie up again for a concurrent delete.
	exists, err := app.Queries.MovieExists(ctx, id)
	if err != nil {
		app.serverError(w, err, getMovieLogMessage, "failed to fetch movie", "id", id)
		return
	}
	if !exists {
		helpers.ErrorJSON(w, errors.New(movieNotFoundMessage), http.StatusNotFound)
		return
	}

	tmdbMovie := &tmdb.TmdbMovie{TmdbID: payload.TmdbID}
	if err := app.Tmdb.GetTmdbMovieByID(ctx, tmdbMovie); err != nil {
		// 404 already answers an unknown local movie here, so an id TMDB does
		// not know is a bad request field rather than a missing resource.
		if tmdb.IsNotFound(err) {
			helpers.ErrorJSON(w, errors.New("tmdb_id not found on TMDB"), http.StatusBadRequest)
			return
		}
		app.serverError(w, err, "tmdb get by id failed", "failed to fetch movie from TMDB", "tmdb_id", payload.TmdbID)
		return
	}

	app.ScannerDBMu.Lock()
	defer app.ScannerDBMu.Unlock()

	tx, err := app.DB.BeginTx(ctx, nil)
	if err != nil {
		app.serverError(w, err, beginTransactionLogMessage, "failed to process request")
		return
	}
	defer tx.Rollback()

	qtx := app.Queries.WithTx(tx)

	movie, err := qtx.GetMovieByID(ctx, id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			helpers.ErrorJSON(w, errors.New(movieNotFoundMessage), http.StatusNotFound)
			return
		}
		app.serverError(w, err, getMovieLogMessage, "failed to fetch movie", "id", id)
		return
	}

	// The scanner owns the definition of which relationships a TMDB match
	// replaces, so identifying a movie by hand reuses it. The wrapped error
	// names the step that failed.
	err = moviescanner.ApplyTmdbMetadata(ctx, qtx, movie.ID, tmdbMovie)
	if err != nil {
		app.serverError(w, err, "failed to apply tmdb metadata", "failed to update movie metadata", "movie_id", movie.ID)
		return
	}

	if err = tx.Commit(); err != nil {
		app.serverError(w, err, "failed to commit identify transaction", "failed to save changes", "movie_id", movie.ID)
		return
	}

	helpers.WriteJSON(w, http.StatusOK, helpers.JSONResponse{Message: "movie identified successfully"})
}

func (app *Application) UpdateMovieMetadata(w http.ResponseWriter, r *http.Request) {
	id, ok := parseIDParam(w, r, "id", invalidMovieIDMessage)
	if !ok {
		return
	}

	var payload struct {
		Title         *string `json:"title"`
		Year          *int64  `json:"year"`
		ReleaseDate   *string `json:"release_date"`
		Overview      *string `json:"overview"`
		TagLine       *string `json:"tag_line"`
		Certification *string `json:"certification"`
		PosterPath    *string `json:"poster_path"`
		BackdropPath  *string `json:"backdrop_path"`
		Language      *string `json:"language"`
	}

	if err := helpers.ReadJSON(w, r, &payload); err != nil {
		helpers.ErrorJSON(w, errors.New(invalidRequestBodyMessage), http.StatusBadRequest)
		return
	}

	ctx := r.Context()

	tx, err := app.DB.BeginTx(ctx, nil)
	if err != nil {
		app.serverError(w, err, beginTransactionLogMessage, "failed to update movie", "id", id)
		return
	}
	defer tx.Rollback()

	qtx := app.Queries.WithTx(tx)

	movie, err := qtx.GetMovieByID(ctx, id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			helpers.ErrorJSON(w, errors.New(movieNotFoundMessage), http.StatusNotFound)
			return
		}
		app.serverError(w, err, getMovieLogMessage, "failed to fetch movie", "id", id)
		return
	}

	// Start from current values; only override fields present in the request.
	params := database.UpdateMovieParams{
		ID:             movie.ID,
		Title:          movie.Title,
		TmdbID:         movie.TmdbID,
		ImdbID:         movie.ImdbID,
		PosterPath:     movie.PosterPath,
		BackdropPath:   movie.BackdropPath,
		Adult:          movie.Adult,
		Language:       movie.Language,
		Year:           movie.Year,
		ReleaseDate:    movie.ReleaseDate,
		Overview:       movie.Overview,
		TagLine:        movie.TagLine,
		Certification:  movie.Certification,
		CriticRating:   movie.CriticRating,
		AudienceRating: movie.AudienceRating,
		Revenue:        movie.Revenue,
		Budget:         movie.Budget,
		RunTime:        movie.RunTime,
	}

	if payload.Title != nil {
		params.Title = *payload.Title
	}
	if payload.Year != nil {
		params.Year = helpers.NullInt64(*payload.Year)
	}
	if payload.ReleaseDate != nil {
		params.ReleaseDate = helpers.NullString(*payload.ReleaseDate)
	}
	if payload.Overview != nil {
		params.Overview = helpers.NullString(*payload.Overview)
	}
	if payload.TagLine != nil {
		params.TagLine = helpers.NullString(*payload.TagLine)
	}
	if payload.Certification != nil {
		params.Certification = helpers.NullString(*payload.Certification)
	}
	if payload.PosterPath != nil {
		params.PosterPath = helpers.NullString(*payload.PosterPath)
	}
	if payload.BackdropPath != nil {
		params.BackdropPath = helpers.NullString(*payload.BackdropPath)
	}
	if payload.Language != nil {
		params.Language = helpers.NullString(*payload.Language)
	}

	rows, err := qtx.UpdateMovie(ctx, params)
	if err == nil && rows == 0 {
		err = sql.ErrNoRows
	}
	if err != nil {
		app.serverError(w, err, "failed to update movie metadata", "failed to update movie", "id", id)
		return
	}

	err = tx.Commit()
	if err != nil {
		app.serverError(w, err, "failed to commit movie metadata update", "failed to update movie", "id", id)
		return
	}

	helpers.WriteJSON(w, http.StatusOK, helpers.JSONResponse{Message: "movie updated successfully"})
}

func (app *Application) DeleteMovie(w http.ResponseWriter, r *http.Request) {
	id, ok := parseIDParam(w, r, "id", invalidMovieIDMessage)
	if !ok {
		return
	}

	var payload struct {
		DeleteFile bool `json:"delete_file"`
	}
	// DELETE may omit the body; delete_file defaults to false.
	_ = helpers.ReadJSON(w, r, &payload)

	ctx := r.Context()

	movie, err := app.Queries.GetMovieByID(ctx, id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			helpers.ErrorJSON(w, errors.New(movieNotFoundMessage), http.StatusNotFound)
			return
		}
		app.serverError(w, err, "failed to get movie for deletion", "failed to fetch movie", "id", id)
		return
	}

	// The cascade removes the movie's watch rooms, so their ids are read in the
	// same transaction and invalidated after the commit, as the scanner does.
	// Caches are evicted after the delete, never before: a request that missed
	// the cache while the row still existed would otherwise republish it.
	var roomIDs []int64
	tx := scanner.TxRunner{DB: app.DB, Mu: &app.ScannerDBMu, Queries: app.Queries}
	err = tx.Run(ctx, func(qtx *database.Queries) error {
		var err error
		roomIDs, err = qtx.ListWatchRoomIDsByMovieID(ctx, id)
		if err != nil {
			return err
		}
		return qtx.DeleteMovie(ctx, id)
	}, func() {
		app.invalidateDeletedWatchRooms(roomIDs)
		app.invalidateCommittedMovie(id)
	})
	if err != nil {
		app.serverError(w, err, "failed to delete movie", "failed to delete movie", "id", id)
		return
	}

	if payload.DeleteFile {
		if err := os.Remove(movie.FilePath); err != nil && !os.IsNotExist(err) {
			app.Logger.Error("failed to delete movie file from disk", "error", err, "path", movie.FilePath)
		}
	}

	helpers.WriteJSON(w, http.StatusOK, helpers.JSONResponse{Message: "movie deleted successfully"})
}
