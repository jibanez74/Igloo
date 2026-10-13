package main

import (
	"context"
	"database/sql"
	"errors"
	"igloo/cmd/internal/database"
	"igloo/cmd/internal/helpers"
	"net/http"
	"strconv"
	"strings"

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

func libraryRowsFromDesc(rows []database.GetMoviesLibraryDescRow) []database.GetMoviesLibraryAscRow {
	out := make([]database.GetMoviesLibraryAscRow, len(rows))
	for i, r := range rows {
		out[i] = database.GetMoviesLibraryAscRow{
			ID:            r.ID,
			Title:         r.Title,
			PosterPath:    r.PosterPath,
			Year:          r.Year,
			Certification: r.Certification,
		}
	}
	return out
}

func libraryRowsFromLikedAsc(rows []database.GetLikedMoviesForUserAscRow) []database.GetMoviesLibraryAscRow {
	out := make([]database.GetMoviesLibraryAscRow, len(rows))
	for i, r := range rows {
		out[i] = database.GetMoviesLibraryAscRow{
			ID:            r.ID,
			Title:         r.Title,
			PosterPath:    r.PosterPath,
			Year:          r.Year,
			Certification: r.Certification,
		}
	}
	return out
}

func libraryRowsFromLikedDesc(rows []database.GetLikedMoviesForUserDescRow) []database.GetMoviesLibraryAscRow {
	out := make([]database.GetMoviesLibraryAscRow, len(rows))
	for i, r := range rows {
		out[i] = database.GetMoviesLibraryAscRow{
			ID:            r.ID,
			Title:         r.Title,
			PosterPath:    r.PosterPath,
			Year:          r.Year,
			Certification: r.Certification,
		}
	}
	return out
}

func (app *Application) logNormalizedChapterStartTimes(
	movie database.Movie,
	original []database.Chapter,
	normalized []database.Chapter,
) {
	if app.Logger == nil {
		return
	}

	count := len(original)
	if len(normalized) < count {
		count = len(normalized)
	}

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

// parsePageAndPerPage reads the page and per_page query parameters every
// paginated listing shares. Missing, unparseable and nonpositive values fall
// back to the defaults rather than failing the request, and per_page above the
// cap is clamped; docs/openapi.json documents that on PerPageQuery.
func parsePageAndPerPage(r *http.Request) (page, perPage int64) {
	page = 1
	if p := r.URL.Query().Get("page"); p != "" {
		parsed, err := strconv.ParseInt(p, 10, 64)
		if err == nil && parsed > 0 {
			page = parsed
		}
	}

	perPage = int64(libraryDefaultPerPage)
	if pp := r.URL.Query().Get("per_page"); pp != "" {
		parsed, err := strconv.ParseInt(pp, 10, 64)
		if err == nil && parsed > 0 {
			perPage = parsed
		}
	}
	if perPage > libraryMaxPerPage {
		perPage = libraryMaxPerPage
	}
	return page, perPage
}

func parseLibraryQuery(r *http.Request) (page, perPage int64, sort string) {
	page, perPage = parsePageAndPerPage(r)

	sort = strings.ToLower(strings.TrimSpace(r.URL.Query().Get("sort")))
	if sort != "desc" {
		sort = "asc"
	}
	return page, perPage, sort
}

func (app *Application) GetMoviesLibrary(w http.ResponseWriter, r *http.Request) {
	page, perPage, sortParam := parseLibraryQuery(r)

	offset := (page - 1) * perPage
	ctx := r.Context()

	total, err := app.Queries.GetMoviesCount(ctx)
	if err != nil {
		app.serverError(w, err, "failed to get movies count", "failed to fetch movies count")
		return
	}

	pag := database.GetMoviesLibraryAscParams{Limit: perPage, Offset: offset}

	var movies []database.GetMoviesLibraryAscRow
	if sortParam == "desc" {
		descRows, err := app.Queries.GetMoviesLibraryDesc(ctx, database.GetMoviesLibraryDescParams{
			Limit:  pag.Limit,
			Offset: pag.Offset,
		})
		if err != nil {
			app.serverError(w, err, "failed to get movies library", "failed to fetch movies")
			return
		}
		movies = libraryRowsFromDesc(descRows)
	} else {
		movies, err = app.Queries.GetMoviesLibraryAsc(ctx, pag)
		if err != nil {
			app.serverError(w, err, "failed to get movies library", "failed to fetch movies")
			return
		}
	}

	totalPages := total / perPage
	if total%perPage > 0 {
		totalPages++
	}

	helpers.WriteJSON(w, http.StatusOK, helpers.JSONResponse{
		Data: moviesLibraryData{
			Movies:     movies,
			Total:      total,
			Page:       page,
			PerPage:    perPage,
			TotalPages: totalPages,
			Sort:       sortParam,
		},
	})
}

func libraryRowsFromGenreAsc(rows []database.GetMoviesByGenreAscRow) []database.GetMoviesLibraryAscRow {
	out := make([]database.GetMoviesLibraryAscRow, len(rows))
	for i, r := range rows {
		out[i] = database.GetMoviesLibraryAscRow{
			ID:            r.ID,
			Title:         r.Title,
			PosterPath:    r.PosterPath,
			Year:          r.Year,
			Certification: r.Certification,
		}
	}
	return out
}

func libraryRowsFromGenreDesc(rows []database.GetMoviesByGenreDescRow) []database.GetMoviesLibraryAscRow {
	out := make([]database.GetMoviesLibraryAscRow, len(rows))
	for i, r := range rows {
		out[i] = database.GetMoviesLibraryAscRow{
			ID:            r.ID,
			Title:         r.Title,
			PosterPath:    r.PosterPath,
			Year:          r.Year,
			Certification: r.Certification,
		}
	}
	return out
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
	offset := (page - 1) * perPage
	ctx := r.Context()

	total, err := app.Queries.CountMoviesForGenre(ctx, genreID)
	if err != nil {
		app.serverError(w, err, "failed to count movies for genre", "failed to fetch movies count for genre", "genre_id", genreID)
		return
	}

	paramsAsc := database.GetMoviesByGenreAscParams{GenreID: genreID, Limit: perPage, Offset: offset}

	var movies []database.GetMoviesLibraryAscRow
	if sortParam == "desc" {
		descRows, err := app.Queries.GetMoviesByGenreDesc(ctx, database.GetMoviesByGenreDescParams{
			GenreID: paramsAsc.GenreID,
			Limit:   paramsAsc.Limit,
			Offset:  paramsAsc.Offset,
		})
		if err != nil {
			app.serverError(w, err, "failed to get movies by genre", "failed to fetch movies", "genre_id", genreID)
			return
		}
		movies = libraryRowsFromGenreDesc(descRows)
	} else {
		ascRows, err := app.Queries.GetMoviesByGenreAsc(ctx, paramsAsc)
		if err != nil {
			app.serverError(w, err, "failed to get movies by genre", "failed to fetch movies", "genre_id", genreID)
			return
		}
		movies = libraryRowsFromGenreAsc(ascRows)
	}

	totalPages := total / perPage
	if total%perPage > 0 {
		totalPages++
	}

	helpers.WriteJSON(w, http.StatusOK, helpers.JSONResponse{
		Data: moviesLibraryData{
			Movies:     movies,
			Total:      total,
			Page:       page,
			PerPage:    perPage,
			TotalPages: totalPages,
			Sort:       sortParam,
		},
	})
}

func (app *Application) GetLikedMovies(w http.ResponseWriter, r *http.Request) {
	userID := app.userIDFromRequest(r)

	page, perPage, sortParam := parseLibraryQuery(r)
	offset := (page - 1) * perPage
	ctx := r.Context()

	total, err := app.Queries.CountUserLikedMovies(ctx, userID)
	if err != nil {
		app.serverError(w, err, "failed to count liked movies", "failed to fetch liked movies count", "userID", userID)
		return
	}

	paramsAsc := database.GetLikedMoviesForUserAscParams{UserID: userID, Limit: perPage, Offset: offset}

	var movies []database.GetMoviesLibraryAscRow
	if sortParam == "desc" {
		descRows, err := app.Queries.GetLikedMoviesForUserDesc(ctx, database.GetLikedMoviesForUserDescParams{
			UserID: paramsAsc.UserID,
			Limit:  paramsAsc.Limit,
			Offset: paramsAsc.Offset,
		})
		if err != nil {
			app.serverError(w, err, "failed to get liked movies", "failed to fetch liked movies", "userID", userID)
			return
		}
		movies = libraryRowsFromLikedDesc(descRows)
	} else {
		ascRows, err := app.Queries.GetLikedMoviesForUserAsc(ctx, paramsAsc)
		if err != nil {
			app.serverError(w, err, "failed to get liked movies", "failed to fetch liked movies", "userID", userID)
			return
		}
		movies = libraryRowsFromLikedAsc(ascRows)
	}

	totalPages := total / perPage
	if total%perPage > 0 {
		totalPages++
	}

	helpers.WriteJSON(w, http.StatusOK, helpers.JSONResponse{
		Data: moviesLibraryData{
			Movies:     movies,
			Total:      total,
			Page:       page,
			PerPage:    perPage,
			TotalPages: totalPages,
			Sort:       sortParam,
		},
	})
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

// toggleMovieLike flips like state in a single DB transaction without IsMovieLiked (avoids read-then-write races).
// It DELETEs the row; if a row was removed, the new state is unliked; otherwise it INSERTs (LikeMovie is
// idempotent via ON CONFLICT DO NOTHING).
func (app *Application) toggleMovieLike(ctx context.Context, userID, movieID int64) (isLiked bool, err error) {
	tx, err := app.DB.BeginTx(ctx, nil)
	if err != nil {
		return false, err
	}
	defer func() { _ = tx.Rollback() }()

	qtx := app.Queries.WithTx(tx)

	removed, err := qtx.UnlikeMovie(ctx, database.UnlikeMovieParams{UserID: userID, MovieID: movieID})
	if err != nil {
		return false, err
	}
	if removed > 0 {
		if err := tx.Commit(); err != nil {
			return false, err
		}
		return false, nil
	}

	if err := qtx.LikeMovie(ctx, database.LikeMovieParams{UserID: userID, MovieID: movieID}); err != nil {
		return false, err
	}
	if err := tx.Commit(); err != nil {
		return false, err
	}
	return true, nil
}

func (app *Application) ToggleLikeMovie(w http.ResponseWriter, r *http.Request) {
	userID := app.userIDFromRequest(r)

	idParam := chi.URLParam(r, "id")
	movieID, err := strconv.ParseInt(idParam, 10, 64)
	if err != nil {
		helpers.ErrorJSON(w, errors.New(invalidMovieIDMessage), http.StatusBadRequest)
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

	isLiked, err := app.toggleMovieLike(ctx, userID, movieID)
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
	idParam := chi.URLParam(r, "id")
	id, err := strconv.ParseInt(idParam, 10, 64)
	if err != nil {
		helpers.ErrorJSON(w, errors.New(invalidMovieIDMessage), http.StatusBadRequest)
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
	idParam := chi.URLParam(r, "id")
	id, err := strconv.ParseInt(idParam, 10, 64)
	if err != nil {
		helpers.ErrorJSON(w, errors.New(invalidMovieIDMessage), http.StatusBadRequest)
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

	durationSec := 0.0
	if movie.Duration.Valid {
		durationSec = movie.Duration.Float64
	}
	normalizedChapters := normalizeChaptersStartTimes(chapters, durationSec)
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
