package main

import (
	"database/sql"
	"errors"
	"igloo/cmd/internal/database"
	"igloo/cmd/internal/helpers"
	"igloo/cmd/internal/tmdb"
	"net/http"
	"os"
	"strconv"

	"github.com/go-chi/chi/v5"
)

type movieMetadataLocks struct {
	Title          bool
	TmdbID         bool
	ImdbID         bool
	PosterPath     bool
	BackdropPath   bool
	Adult          bool
	Language       bool
	Year           bool
	ReleaseDate    bool
	Overview       bool
	TagLine        bool
	Certification  bool
	CriticRating   bool
	AudienceRating bool
	Revenue        bool
	Budget         bool
	RunTime        bool
}

func (l movieMetadataLocks) any() bool {
	return l.Title || l.TmdbID || l.ImdbID || l.PosterPath || l.BackdropPath ||
		l.Adult || l.Language || l.Year || l.ReleaseDate || l.Overview ||
		l.TagLine || l.Certification || l.CriticRating || l.AudienceRating ||
		l.Revenue || l.Budget || l.RunTime
}

func (l movieMetadataLocks) toParams(movieID int64) database.LockMovieMetadataFieldsParams {
	return database.LockMovieMetadataFieldsParams{
		UserLockedTitle:          l.Title,
		UserLockedTmdbID:         l.TmdbID,
		UserLockedImdbID:         l.ImdbID,
		UserLockedPosterPath:     l.PosterPath,
		UserLockedBackdropPath:   l.BackdropPath,
		UserLockedAdult:          l.Adult,
		UserLockedLanguage:       l.Language,
		UserLockedYear:           l.Year,
		UserLockedReleaseDate:    l.ReleaseDate,
		UserLockedOverview:       l.Overview,
		UserLockedTagLine:        l.TagLine,
		UserLockedCertification:  l.Certification,
		UserLockedCriticRating:   l.CriticRating,
		UserLockedAudienceRating: l.AudienceRating,
		UserLockedRevenue:        l.Revenue,
		UserLockedBudget:         l.Budget,
		UserLockedRunTime:        l.RunTime,
		ID:                       movieID,
	}
}

func tmdbMovieLocks() movieMetadataLocks {
	return movieMetadataLocks{
		Title:         true,
		TmdbID:        true,
		ImdbID:        true,
		PosterPath:    true,
		BackdropPath:  true,
		Adult:         true,
		Language:      true,
		Year:          true,
		ReleaseDate:   true,
		Overview:      true,
		TagLine:       true,
		Certification: true,
		CriticRating:  true,
		Revenue:       true,
		Budget:        true,
		RunTime:       true,
	}
}

func (app *Application) IdentifyMovie(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		helpers.ErrorJSON(w, errors.New("invalid movie id"), http.StatusBadRequest)
		return
	}

	var payload struct {
		TmdbID int `json:"tmdb_id"`
	}

	if err := helpers.ReadJSON(w, r, &payload, 0); err != nil || payload.TmdbID <= 0 {
		helpers.ErrorJSON(w, errors.New("valid tmdb_id is required"), http.StatusBadRequest)
		return
	}

	if app.Tmdb == nil {
		helpers.ErrorJSON(w, errors.New("TMDB is not configured"))
		return
	}

	ctx := r.Context()

	tmdbMovie := &tmdb.TmdbMovie{TmdbID: payload.TmdbID}
	if err := app.Tmdb.GetTmdbMovieByID(ctx, tmdbMovie); err != nil {
		app.Logger.Error("tmdb get by id failed", "error", err, "tmdb_id", payload.TmdbID)
		helpers.ErrorJSON(w, errors.New("failed to fetch movie from TMDB"))
		return
	}

	tx, err := app.DB.BeginTx(ctx, nil)
	if err != nil {
		app.Logger.Error("failed to begin transaction", "error", err)
		helpers.ErrorJSON(w, errors.New("failed to process request"))
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
		helpers.ErrorJSON(w, errors.New("failed to fetch movie"))
		return
	}

	params := buildUpdateParamsFromTmdb(movie.ID, tmdbMovie)

	if _, err = qtx.UpdateMovie(ctx, params); err != nil {
		app.Logger.Error("failed to update movie", "error", err, "id", id)
		helpers.ErrorJSON(w, errors.New("failed to update movie"))
		return
	}

	err = qtx.LockMovieMetadataFields(ctx, tmdbMovieLocks().toParams(movie.ID))
	if err != nil {
		app.Logger.Error("failed to lock movie metadata", "error", err, "id", id)
		helpers.ErrorJSON(w, errors.New("failed to update movie"))
		return
	}

	// Delete existing cast and crew before re-inserting (the other entity
	// processors already delete-then-insert so they handle this internally).
	if err = qtx.DeleteMovieCast(ctx, movie.ID); err != nil {
		app.Logger.Error("failed to delete cast", "error", err, "movie_id", movie.ID)
		helpers.ErrorJSON(w, errors.New("failed to update movie cast"))
		return
	}
	if err = qtx.DeleteMovieCrew(ctx, movie.ID); err != nil {
		app.Logger.Error("failed to delete crew", "error", err, "movie_id", movie.ID)
		helpers.ErrorJSON(w, errors.New("failed to update movie crew"))
		return
	}

	if err = app.processProductionCompanies(ctx, qtx, movie.ID, tmdbMovie.ProductionCompanies); err != nil {
		app.Logger.Error("failed to process production companies", "error", err, "movie_id", movie.ID)
		helpers.ErrorJSON(w, errors.New("failed to update production companies"))
		return
	}
	if err = app.processCast(ctx, qtx, movie.ID, tmdbMovie.Credits.Cast); err != nil {
		app.Logger.Error("failed to process cast", "error", err, "movie_id", movie.ID)
		helpers.ErrorJSON(w, errors.New("failed to update cast"))
		return
	}
	if err = app.processCrew(ctx, qtx, movie.ID, tmdbMovie.Credits.Crew); err != nil {
		app.Logger.Error("failed to process crew", "error", err, "movie_id", movie.ID)
		helpers.ErrorJSON(w, errors.New("failed to update crew"))
		return
	}
	if err = app.processMovieGenres(ctx, qtx, movie.ID, tmdbMovie.Genres); err != nil {
		app.Logger.Error("failed to process genres", "error", err, "movie_id", movie.ID)
		helpers.ErrorJSON(w, errors.New("failed to update genres"))
		return
	}
	if err = app.processExtraVideos(ctx, qtx, movie.ID, tmdbMovie.Videos.Results); err != nil {
		app.Logger.Error("failed to process extra videos", "error", err, "movie_id", movie.ID)
		helpers.ErrorJSON(w, errors.New("failed to update extra videos"))
		return
	}

	if err = tx.Commit(); err != nil {
		app.Logger.Error("failed to commit identify transaction", "error", err, "movie_id", movie.ID)
		helpers.ErrorJSON(w, errors.New("failed to save changes"))
		return
	}

	helpers.WriteJSON(w, http.StatusOK, helpers.JSONResponse{
		Error:   false,
		Message: "movie identified successfully",
	})
}

func (app *Application) UpdateMovieMetadata(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		helpers.ErrorJSON(w, errors.New("invalid movie id"), http.StatusBadRequest)
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

	if err := helpers.ReadJSON(w, r, &payload, 0); err != nil {
		helpers.ErrorJSON(w, errors.New("invalid request body"), http.StatusBadRequest)
		return
	}

	ctx := r.Context()

	tx, err := app.DB.BeginTx(ctx, nil)
	if err != nil {
		app.Logger.Error("failed to begin transaction", "error", err, "id", id)
		helpers.ErrorJSON(w, errors.New("failed to update movie"))
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
		helpers.ErrorJSON(w, errors.New("failed to fetch movie"))
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

	if _, err = qtx.UpdateMovie(ctx, params); err != nil {
		app.Logger.Error("failed to update movie metadata", "error", err, "id", id)
		helpers.ErrorJSON(w, errors.New("failed to update movie"))
		return
	}

	locks := movieMetadataLocks{
		Title:         payload.Title != nil,
		Year:          payload.Year != nil,
		ReleaseDate:   payload.ReleaseDate != nil,
		Overview:      payload.Overview != nil,
		TagLine:       payload.TagLine != nil,
		Certification: payload.Certification != nil,
		PosterPath:    payload.PosterPath != nil,
		BackdropPath:  payload.BackdropPath != nil,
		Language:      payload.Language != nil,
	}

	if locks.any() {
		err = qtx.LockMovieMetadataFields(ctx, locks.toParams(movie.ID))
		if err != nil {
			app.Logger.Error("failed to lock movie metadata", "error", err, "id", id)
			helpers.ErrorJSON(w, errors.New("failed to update movie"))
			return
		}
	}

	err = tx.Commit()
	if err != nil {
		app.Logger.Error("failed to commit movie metadata update", "error", err, "id", id)
		helpers.ErrorJSON(w, errors.New("failed to update movie"))
		return
	}

	helpers.WriteJSON(w, http.StatusOK, helpers.JSONResponse{
		Error:   false,
		Message: "movie updated successfully",
	})
}

func (app *Application) DeleteMovie(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		helpers.ErrorJSON(w, errors.New("invalid movie id"), http.StatusBadRequest)
		return
	}

	var payload struct {
		DeleteFile bool `json:"delete_file"`
	}
	// DELETE may omit the body; delete_file defaults to false.
	_ = helpers.ReadJSON(w, r, &payload, 0)

	ctx := r.Context()

	movie, err := app.Queries.GetMovieByID(ctx, id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			helpers.ErrorJSON(w, errors.New("movie not found"), http.StatusNotFound)
			return
		}
		app.Logger.Error("failed to get movie for deletion", "error", err, "id", id)
		helpers.ErrorJSON(w, errors.New("failed to fetch movie"))
		return
	}

	app.invalidateSubtitleVTTCache(id)

	if err = app.Queries.DeleteMovie(ctx, id); err != nil {
		app.Logger.Error("failed to delete movie", "error", err, "id", id)
		helpers.ErrorJSON(w, errors.New("failed to delete movie"))
		return
	}

	if payload.DeleteFile {
		if err := os.Remove(movie.FilePath); err != nil && !os.IsNotExist(err) {
			app.Logger.Error("failed to delete movie file from disk", "error", err, "path", movie.FilePath)
		}
	}

	helpers.WriteJSON(w, http.StatusOK, helpers.JSONResponse{
		Error:   false,
		Message: "movie deleted successfully",
	})
}

func buildUpdateParamsFromTmdb(movieID int64, m *tmdb.TmdbMovie) database.UpdateMovieParams {
	params := database.UpdateMovieParams{
		ID:            movieID,
		Title:         m.Title,
		TmdbID:        helpers.NullInt64(int64(m.TmdbID)),
		ImdbID:        helpers.NullString(m.ImdbID),
		PosterPath:    helpers.NullString(m.PosterPath),
		BackdropPath:  helpers.NullString(m.BackdropPath),
		Adult:         m.Adult,
		Language:      helpers.NullString(m.OriginalLang),
		Overview:      helpers.NullString(m.Overview),
		TagLine:       helpers.NullString(m.Tagline),
		Certification: helpers.NullString(m.Certification()),
		CriticRating:  helpers.NullFloat64(m.VoteAverage),
		Revenue:       helpers.NullFloat64(float64(m.Revenue)),
		Budget:        helpers.NullFloat64(float64(m.Budget)),
		RunTime:       helpers.NullInt64(int64(m.Runtime)),
	}

	if m.ReleaseDate != "" {
		params.ReleaseDate = helpers.NullString(m.ReleaseDate)
		if year := extractYearFromReleaseDate(m.ReleaseDate); year > 0 {
			params.Year = helpers.NullInt64(int64(year))
		}
	}

	return params
}
