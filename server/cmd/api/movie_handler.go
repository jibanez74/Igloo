package main

import (
	"context"
	"database/sql"
	"errors"
	"igloo/cmd/internal/database"
	"igloo/cmd/internal/helpers"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
)

// The page size every paginated library listing answers with — movies, shows,
// albums, musicians, playlist movies and search alike. Track listings page
// differently; see track_handler.go.
const (
	libraryDefaultPerPage = 24
	libraryMaxPerPage     = 48
)

// Said by every handler that resolves a movie by id. The first two are built
// from the mediaRef helpers in playback_source.go rather than spelled again:
// the playback routes answer the same two conditions through those, and the
// REST routes must not say it differently.
var (
	movieNotFoundMessage  = mediaKindMovie.notFoundMessage()
	invalidMovieIDMessage = invalidMediaIDMessage(mediaKindMovie)
)

const (
	getMovieLogMessage = "failed to get movie"
	fetchMovieMessage  = "failed to fetch movie from server"
)

// moviesLibraryData is the JSON shape of helpers.JSONResponse.Data for GET /api/movies/library.
type moviesLibraryData struct {
	Movies     []database.GetMoviesLibraryAscRow `json:"movies"`
	Total      int64                             `json:"total"`
	Page       int64                             `json:"page"`
	PerPage    int64                             `json:"per_page"`
	TotalPages int64                             `json:"total_pages"`
	Sort       string                            `json:"sort"`
}

// moviesStatsData is the JSON shape of helpers.JSONResponse.Data for GET /api/movies/stats.
type moviesStatsData struct {
	TotalMovies int64 `json:"total_movies"`
}

// movieLibraryRow is every generated movie listing row. Each has exactly
// GetMoviesLibraryAscRow's fields, so it converts to that type directly.
type movieLibraryRow interface {
	database.GetMoviesLibraryDescRow |
		database.GetMoviesByGenreAscRow | database.GetMoviesByGenreDescRow |
		database.GetLikedMoviesForUserAscRow | database.GetLikedMoviesForUserDescRow |
		database.GetPlaylistMoviesPaginatedAscRow | database.GetPlaylistMoviesPaginatedDescRow
}

func movieLibraryRows[T movieLibraryRow](rows []T) []database.GetMoviesLibraryAscRow {
	out := make([]database.GetMoviesLibraryAscRow, len(rows))
	for i, row := range rows {
		out[i] = database.GetMoviesLibraryAscRow(row)
	}
	return out
}

func writeMoviesLibrary(w http.ResponseWriter, movies []database.GetMoviesLibraryAscRow, total, page, perPage int64, sort string) {
	helpers.WriteJSON(w, http.StatusOK, helpers.JSONResponse{
		Data: moviesLibraryData{
			Movies:     movies,
			Total:      total,
			Page:       page,
			PerPage:    perPage,
			TotalPages: totalPages(total, perPage),
			Sort:       sort,
		},
	})
}

func (app *Application) logNormalizedChapterStartTimes(
	movie database.Movie,
	original []database.Chapter,
	normalized []database.Chapter,
) {
	count := min(len(original), len(normalized))
	for i := 0; i < count; i++ {
		if original[i].StartTime == normalized[i].StartTime {
			continue
		}

		app.Logger.Debug(
			"normalized chapter start time",
			"movie_id", movie.ID,
			"movie_title", movie.Title,
			"chapter_id", normalized[i].ID,
			"chapter_index", i,
			"original_start_time", original[i].StartTime,
			"normalized_start_time", normalized[i].StartTime,
		)
	}
}

func (app *Application) GetMoviesLibrary(w http.ResponseWriter, r *http.Request) {
	page, perPage, sortParam := parseLibraryQuery(r)
	ctx := r.Context()

	total, err := app.Queries.GetMoviesCount(ctx)
	if err != nil {
		app.serverError(w, err, "failed to get movies count", "failed to fetch movies count")
		return
	}

	params := database.GetMoviesLibraryAscParams{Limit: perPage, Offset: (page - 1) * perPage}

	var movies []database.GetMoviesLibraryAscRow
	if sortParam == "desc" {
		var rows []database.GetMoviesLibraryDescRow
		rows, err = app.Queries.GetMoviesLibraryDesc(ctx, database.GetMoviesLibraryDescParams(params))
		movies = movieLibraryRows(rows)
	} else {
		movies, err = app.Queries.GetMoviesLibraryAsc(ctx, params)
	}
	if err != nil {
		app.serverError(w, err, "failed to get movies library", "failed to fetch movies")
		return
	}

	writeMoviesLibrary(w, movies, total, page, perPage, sortParam)
}

func (app *Application) GetMovieGenresList(w http.ResponseWriter, r *http.Request) {
	rows, err := app.Queries.GetMovieGenresWithCounts(r.Context())
	if err != nil {
		app.serverError(w, err, "failed to get movie genres with counts", "failed to fetch genres")
		return
	}

	helpers.WriteJSON(w, http.StatusOK, helpers.JSONResponse{
		Data: map[string]any{
			"genres": rows,
		},
	})
}

func (app *Application) GetMoviesByGenreLibrary(w http.ResponseWriter, r *http.Request) {
	genreIDStr := chi.URLParam(r, "genreId")
	genreID, err := strconv.ParseInt(genreIDStr, 10, 64)
	if err != nil || genreID <= 0 {
		helpers.ErrorJSON(w, errors.New("invalid genre id"), http.StatusBadRequest)
		return
	}

	page, perPage, sortParam := parseLibraryQuery(r)
	ctx := r.Context()

	total, err := app.Queries.CountMoviesForGenre(ctx, genreID)
	if err != nil {
		app.serverError(w, err, "failed to count movies for genre", "failed to fetch movies count for genre", "genre_id", genreID)
		return
	}

	params := database.GetMoviesByGenreAscParams{GenreID: genreID, Limit: perPage, Offset: (page - 1) * perPage}

	var movies []database.GetMoviesLibraryAscRow
	if sortParam == "desc" {
		var rows []database.GetMoviesByGenreDescRow
		rows, err = app.Queries.GetMoviesByGenreDesc(ctx, database.GetMoviesByGenreDescParams(params))
		movies = movieLibraryRows(rows)
	} else {
		var rows []database.GetMoviesByGenreAscRow
		rows, err = app.Queries.GetMoviesByGenreAsc(ctx, params)
		movies = movieLibraryRows(rows)
	}
	if err != nil {
		app.serverError(w, err, "failed to get movies by genre", "failed to fetch movies", "genre_id", genreID)
		return
	}

	writeMoviesLibrary(w, movies, total, page, perPage, sortParam)
}

func (app *Application) GetLikedMovies(w http.ResponseWriter, r *http.Request) {
	userID := app.userIDFromRequest(r)

	page, perPage, sortParam := parseLibraryQuery(r)
	ctx := r.Context()

	total, err := app.Queries.CountUserLikedMovies(ctx, userID)
	if err != nil {
		app.serverError(w, err, "failed to count liked movies", "failed to fetch liked movies count", "userID", userID)
		return
	}

	params := database.GetLikedMoviesForUserAscParams{UserID: userID, Limit: perPage, Offset: (page - 1) * perPage}

	var movies []database.GetMoviesLibraryAscRow
	if sortParam == "desc" {
		var rows []database.GetLikedMoviesForUserDescRow
		rows, err = app.Queries.GetLikedMoviesForUserDesc(ctx, database.GetLikedMoviesForUserDescParams(params))
		movies = movieLibraryRows(rows)
	} else {
		var rows []database.GetLikedMoviesForUserAscRow
		rows, err = app.Queries.GetLikedMoviesForUserAsc(ctx, params)
		movies = movieLibraryRows(rows)
	}
	if err != nil {
		app.serverError(w, err, "failed to get liked movies", "failed to fetch liked movies", "userID", userID)
		return
	}

	writeMoviesLibrary(w, movies, total, page, perPage, sortParam)
}

func (app *Application) GetMovieLikeStatus(w http.ResponseWriter, r *http.Request) {
	userID := app.userIDFromRequest(r)
	idParam := chi.URLParam(r, "id")
	movieID, err := strconv.ParseInt(idParam, 10, 64)
	if err != nil || movieID <= 0 {
		helpers.ErrorJSON(w, errors.New(invalidMovieIDMessage), http.StatusBadRequest)
		return
	}

	isLiked, err := app.Queries.IsMovieLiked(r.Context(), database.IsMovieLikedParams{
		UserID:  userID,
		MovieID: movieID,
	})
	if err != nil {
		app.serverError(w, err, "failed to check movie like status", "failed to check like status", "movie_id", movieID)
		return
	}

	helpers.WriteJSON(w, http.StatusOK, helpers.JSONResponse{
		Data: map[string]any{
			"is_liked": isLiked,
		},
	})
}

// toggleLike flips a like in one transaction without a read-then-write race:
// unlike deletes the row, and only when nothing was removed does like insert it
// (the like queries are idempotent via ON CONFLICT DO NOTHING).
func (app *Application) toggleLike(
	ctx context.Context,
	unlike func(*database.Queries) (int64, error),
	like func(*database.Queries) error,
) (isLiked bool, err error) {
	tx, err := app.DB.BeginTx(ctx, nil)
	if err != nil {
		return false, err
	}
	defer func() { _ = tx.Rollback() }()

	qtx := app.Queries.WithTx(tx)
	removed, err := unlike(qtx)
	if err != nil {
		return false, err
	}
	if removed == 0 {
		err = like(qtx)
		if err != nil {
			return false, err
		}
	}

	err = tx.Commit()
	if err != nil {
		return false, err
	}
	return removed == 0, nil
}

func (app *Application) ToggleLikeMovie(w http.ResponseWriter, r *http.Request) {
	userID := app.userIDFromRequest(r)

	movieID, ok := parseIDParam(w, r, "id", invalidMovieIDMessage)
	if !ok {
		return
	}

	ctx := r.Context()

	movieOK, err := app.Queries.MovieExists(ctx, movieID)
	if err != nil {
		app.serverError(w, err, "failed to get movie for like toggle", "failed to verify movie exists", "id", movieID)
		return
	}
	if !movieOK {
		helpers.ErrorJSON(w, errors.New(movieNotFoundMessage), http.StatusNotFound)
		return
	}

	isLiked, err := app.toggleLike(ctx,
		func(q *database.Queries) (int64, error) {
			return q.UnlikeMovie(ctx, database.UnlikeMovieParams{UserID: userID, MovieID: movieID})
		},
		func(q *database.Queries) error {
			return q.LikeMovie(ctx, database.LikeMovieParams{UserID: userID, MovieID: movieID})
		},
	)
	if err != nil {
		app.serverError(w, err, "failed to toggle movie like", "failed to update like", "movieID", movieID, "userID", userID)
		return
	}

	helpers.WriteJSON(w, http.StatusOK, helpers.JSONResponse{
		Data: map[string]any{
			"movie_id": movieID,
			"is_liked": isLiked,
		},
	})
}

func (app *Application) GetMoviesStats(w http.ResponseWriter, r *http.Request) {
	total, err := app.Queries.GetMoviesCount(r.Context())
	if err != nil {
		app.serverError(w, err, "failed to get movies count", "failed to fetch movies stats")
		return
	}

	helpers.WriteJSON(w, http.StatusOK, helpers.JSONResponse{
		Data: moviesStatsData{
			TotalMovies: total,
		},
	})
}

func (app *Application) GetLatestMovies(w http.ResponseWriter, r *http.Request) {
	movies, err := app.Queries.GetLatestMovies(r.Context())
	if err != nil {
		app.serverError(w, err, "failed to get latest movies", internalServerErrorMessage)
		return
	}

	helpers.WriteJSON(w, http.StatusOK, helpers.JSONResponse{Data: map[string]any{"movies": movies}})
}

// Read-only transaction keeps related movie data in one snapshot.
func (app *Application) GetMovieDetails(w http.ResponseWriter, r *http.Request) {
	id, ok := parseIDParam(w, r, "id", invalidMovieIDMessage)
	if !ok {
		return
	}

	ctx := r.Context()

	tx, err := app.DB.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		app.serverError(w, err, beginTransactionLogMessage, fetchMovieMessage)
		return
	}
	defer tx.Rollback()

	qtx := app.Queries.WithTx(tx)

	movie, err := qtx.GetMovieDetails(ctx, id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			helpers.ErrorJSON(w, errors.New(movieNotFoundMessage), http.StatusNotFound)
			return
		}

		app.serverError(w, err, getMovieLogMessage, fetchMovieMessage, "id", id)
		return
	}

	cast, err := qtx.GetCastByMovieID(ctx, id)
	if err != nil {
		app.serverError(w, err, "failed to get cast for movie", "failed to fetch movie cast from server", "movie_id", id)
		return
	}

	crew, err := qtx.GetCrewByMovieID(ctx, id)
	if err != nil {
		app.serverError(w, err, "failed to get crew for movie", "failed to fetch movie crew from server", "movie_id", id)
		return
	}

	genres, err := qtx.GetGenresByMovieID(ctx, id)
	if err != nil {
		app.serverError(w, err, "failed to get genres for movie", "failed to fetch movie genres from server", "movie_id", id)
		return
	}

	productionCompanies, err := qtx.GetProductionCompaniesByMovieID(ctx, id)
	if err != nil {
		app.serverError(w, err, "failed to get production companies for movie", "failed to fetch movie production companies from server",
			"movie_id", id)
		return
	}

	extraVideos, err := qtx.GetMovieExtraVideos(ctx, id)
	if err != nil {
		app.serverError(w, err, "failed to get extra videos for movie", "failed to fetch movie extra videos from server",
			"movie_id", id)
		return
	}

	helpers.WriteJSON(w, http.StatusOK, helpers.JSONResponse{
		Data: map[string]any{
			"movie":                movie,
			"cast":                 cast,
			"crew":                 crew,
			"genres":               genres,
			"production_companies": productionCompanies,
			"extra_videos":         extraVideos,
		},
	})
}

func (app *Application) GetMovieTechnicalDetails(w http.ResponseWriter, r *http.Request) {
	id, ok := parseIDParam(w, r, "id", invalidMovieIDMessage)
	if !ok {
		return
	}

	ctx := r.Context()

	tx, err := app.DB.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		app.serverError(w, err, beginTransactionLogMessage, fetchTechnicalDetailsMessage)
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
		app.serverError(w, err, getMovieLogMessage, fetchTechnicalDetailsMessage, "id", id)
		return
	}

	videoStreams, err := qtx.GetVideoStreamsByMovieID(ctx, id)
	if err != nil {
		app.serverError(w, err, "failed to get video streams", "failed to fetch video streams", "movie_id", id)
		return
	}

	audioStreams, err := qtx.GetAudioStreamsByMovieID(ctx, id)
	if err != nil {
		app.serverError(w, err, "failed to get audio streams", "failed to fetch audio streams", "movie_id", id)
		return
	}

	subtitles, err := qtx.GetSubtitlesByMovieID(ctx, id)
	if err != nil {
		app.serverError(w, err, getSubtitlesLogMessage, fetchSubtitlesMessage, "movie_id", id)
		return
	}

	chapters, err := qtx.GetChaptersByMovieID(ctx, id)
	if err != nil {
		app.serverError(w, err, "failed to get chapters", "failed to fetch chapters", "movie_id", id)
		return
	}

	normalizedChapters := normalizeChaptersStartTimes(chapters, movie.Duration.Float64)
	app.logNormalizedChapterStartTimes(movie, chapters, normalizedChapters)
	chapters = normalizedChapters

	helpers.WriteJSON(w, http.StatusOK, helpers.JSONResponse{
		Data: map[string]any{
			"movie": map[string]any{
				"file_name": movie.FileName,
				"size":      movie.Size,
				"container": movie.Container,
				// The value the client's direct-play container gate reads, so
				// it must be the one the watch-room handler validates against.
				"mime_type": videoContentType(movie.Container, movie.MimeType),
				"run_time":  movie.RunTime,
				"duration":  movie.Duration,
			},
			"video_streams": videoStreams,
			"audio_streams": audioStreams,
			"subtitles":     subtitles,
			"chapters":      chapters,
		},
	})
}

func (app *Application) StreamMovie(w http.ResponseWriter, r *http.Request) {
	app.serveStreamFile(w, r, mediaKindMovie)
}
