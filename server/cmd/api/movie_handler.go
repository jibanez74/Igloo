package main

import (
  "database/sql"
  "errors"
  "igloo/cmd/internal/database"
  "igloo/cmd/internal/helpers"
  "mime"
  "net/http"
  "os"
  "path/filepath"
  "strconv"
  "strings"

  "github.com/go-chi/chi/v5"
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

// parseMoviesLibraryQuery reads page, per_page, and sort for library- and liked-list endpoints.
func parseMoviesLibraryQuery(r *http.Request) (page, perPage int64, sort string) {
	page = 1
	if p := r.URL.Query().Get("page"); p != "" {
		parsed, err := strconv.ParseInt(p, 10, 64)
		if err == nil && parsed > 0 {
			page = parsed
		}
	}

	perPage = int64(helpers.MOVIES_LIBRARY_DEFAULT_PER_PAGE)
	if pp := r.URL.Query().Get("per_page"); pp != "" {
		parsed, err := strconv.ParseInt(pp, 10, 64)
		if err == nil && parsed > 0 {
			perPage = parsed
		}
	}
	if perPage > helpers.MOVIES_LIBRARY_MAX_PER_PAGE {
		perPage = helpers.MOVIES_LIBRARY_MAX_PER_PAGE
	}

	sort = strings.ToLower(strings.TrimSpace(r.URL.Query().Get("sort")))
	if sort != "desc" {
		sort = "asc"
	}
	return page, perPage, sort
}

// GetMoviesLibrary returns a paginated list of movies for the library grid (A–Z or Z–A).
// Query: page (default 1), per_page (default MOVIES_LIBRARY_DEFAULT_PER_PAGE, max MOVIES_LIBRARY_MAX_PER_PAGE), sort=asc|desc (default asc).
func (app *Application) GetMoviesLibrary(w http.ResponseWriter, r *http.Request) {
	page, perPage, sortParam := parseMoviesLibraryQuery(r)

	offset := (page - 1) * perPage
	ctx := r.Context()

	total, err := app.Queries.GetMoviesCount(ctx)
	if err != nil {
		app.Logger.Error("failed to get movies count", "error", err)
		helpers.ErrorJSON(w, errors.New("failed to fetch movies count"))
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
			app.Logger.Error("failed to get movies library", "error", err)
			helpers.ErrorJSON(w, errors.New("failed to fetch movies"))
			return
		}
		movies = libraryRowsFromDesc(descRows)
	} else {
		movies, err = app.Queries.GetMoviesLibraryAsc(ctx, pag)
		if err != nil {
			app.Logger.Error("failed to get movies library", "error", err)
			helpers.ErrorJSON(w, errors.New("failed to fetch movies"))
			return
		}
	}

	totalPages := total / perPage
	if total%perPage > 0 {
		totalPages++
	}

	res := helpers.JSONResponse{
		Error: false,
		Data: moviesLibraryData{
			Movies:     movies,
			Total:      total,
			Page:       page,
			PerPage:    perPage,
			TotalPages: totalPages,
			Sort:       sortParam,
		},
	}

	helpers.WriteJSON(w, http.StatusOK, res)
}

// GetLikedMovies returns the current user's liked movies as paginated rows (same shape as GET /api/movies/library).
func (app *Application) GetLikedMovies(w http.ResponseWriter, r *http.Request) {
	userID := app.SessionManager.GetInt64(r.Context(), helpers.COOKIE_USER_ID)
	if userID == 0 {
		helpers.ErrorJSON(w, errors.New(helpers.NOT_AUTHORIZED_MESSAGE), http.StatusUnauthorized)
		return
	}

	page, perPage, sortParam := parseMoviesLibraryQuery(r)
	offset := (page - 1) * perPage
	ctx := r.Context()

	total, err := app.Queries.CountUserLikedMovies(ctx, userID)
	if err != nil {
		app.Logger.Error("failed to count liked movies", "error", err, "userID", userID)
		helpers.ErrorJSON(w, errors.New("failed to fetch liked movies count"))
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
			app.Logger.Error("failed to get liked movies", "error", err, "userID", userID)
			helpers.ErrorJSON(w, errors.New("failed to fetch liked movies"))
			return
		}
		movies = libraryRowsFromLikedDesc(descRows)
	} else {
		ascRows, err := app.Queries.GetLikedMoviesForUserAsc(ctx, paramsAsc)
		if err != nil {
			app.Logger.Error("failed to get liked movies", "error", err, "userID", userID)
			helpers.ErrorJSON(w, errors.New("failed to fetch liked movies"))
			return
		}
		movies = libraryRowsFromLikedAsc(ascRows)
	}

	totalPages := total / perPage
	if total%perPage > 0 {
		totalPages++
	}

	res := helpers.JSONResponse{
		Error: false,
		Data: moviesLibraryData{
			Movies:     movies,
			Total:      total,
			Page:       page,
			PerPage:    perPage,
			TotalPages: totalPages,
			Sort:       sortParam,
		},
	}

	helpers.WriteJSON(w, http.StatusOK, res)
}

// ToggleLikeMovie toggles like for the session user on the given movie (same behavior as music track likes).
func (app *Application) ToggleLikeMovie(w http.ResponseWriter, r *http.Request) {
	userID := app.SessionManager.GetInt64(r.Context(), helpers.COOKIE_USER_ID)
	if userID == 0 {
		helpers.ErrorJSON(w, errors.New(helpers.NOT_AUTHORIZED_MESSAGE), http.StatusUnauthorized)
		return
	}

	idParam := chi.URLParam(r, "id")
	movieID, err := strconv.ParseInt(idParam, 10, 64)
	if err != nil {
		helpers.ErrorJSON(w, errors.New("invalid movie id"), http.StatusBadRequest)
		return
	}

	ctx := r.Context()

	_, err = app.Queries.GetMovieByID(ctx, movieID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			helpers.ErrorJSON(w, errors.New("movie not found"), http.StatusNotFound)
			return
		}
		app.Logger.Error("failed to get movie for like toggle", "error", err, "id", movieID)
		helpers.ErrorJSON(w, errors.New("failed to verify movie exists"))
		return
	}

	isLiked, err := app.Queries.IsMovieLiked(ctx, database.IsMovieLikedParams{
		UserID:  userID,
		MovieID: movieID,
	})
	if err != nil {
		app.Logger.Error("failed to check if movie is liked", "error", err, "movieID", movieID, "userID", userID)
		helpers.ErrorJSON(w, errors.New("failed to check like status"))
		return
	}

	if isLiked {
		err = app.Queries.UnlikeMovie(ctx, database.UnlikeMovieParams{
			UserID:  userID,
			MovieID: movieID,
		})
		if err != nil {
			app.Logger.Error("failed to unlike movie", "error", err, "movieID", movieID, "userID", userID)
			helpers.ErrorJSON(w, errors.New("failed to unlike movie"))
			return
		}
	} else {
		err = app.Queries.LikeMovie(ctx, database.LikeMovieParams{
			UserID:  userID,
			MovieID: movieID,
		})
		if err != nil {
			app.Logger.Error("failed to like movie", "error", err, "movieID", movieID, "userID", userID)
			helpers.ErrorJSON(w, errors.New("failed to like movie"))
			return
		}
	}

	res := helpers.JSONResponse{
		Error: false,
		Data: map[string]any{
			"movie_id": movieID,
			"is_liked": !isLiked,
		},
	}

	helpers.WriteJSON(w, http.StatusOK, res)
}

// GetMoviesStats returns aggregate counts for the movies UI (stats row).
func (app *Application) GetMoviesStats(w http.ResponseWriter, r *http.Request) {
	total, err := app.Queries.GetMoviesCount(r.Context())
	if err != nil {
		app.Logger.Error("failed to get movies count", "error", err)
		helpers.ErrorJSON(w, errors.New("failed to fetch movies stats"))
		return
	}

	res := helpers.JSONResponse{
		Error: false,
		Data: moviesStatsData{
			TotalMovies: total,
		},
	}

	helpers.WriteJSON(w, http.StatusOK, res)
}

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

// GetMovieTechnicalDetails returns video/audio streams, subtitles, and chapters
// for a movie. All data comes from the database (populated by the scanner via ffprobe).
func (app *Application) GetMovieTechnicalDetails(w http.ResponseWriter, r *http.Request) {
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
    helpers.ErrorJSON(w, errors.New("failed to fetch technical details"))
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
    helpers.ErrorJSON(w, errors.New("failed to fetch technical details"))
    return
  }

  videoStreams, err := qtx.GetVideoStreamsByMovieID(ctx, id)
  if err != nil {
    app.Logger.Error("failed to get video streams", "error", err, "movie_id", id)
    helpers.ErrorJSON(w, errors.New("failed to fetch video streams"))
    return
  }

  audioStreams, err := qtx.GetAudioStreamsByMovieID(ctx, id)
  if err != nil {
    app.Logger.Error("failed to get audio streams", "error", err, "movie_id", id)
    helpers.ErrorJSON(w, errors.New("failed to fetch audio streams"))
    return
  }

  subtitles, err := qtx.GetSubtitlesByMovieID(ctx, id)
  if err != nil {
    app.Logger.Error("failed to get subtitles", "error", err, "movie_id", id)
    helpers.ErrorJSON(w, errors.New("failed to fetch subtitles"))
    return
  }

  chapters, err := qtx.GetChaptersByMovieID(ctx, sql.NullInt64{Int64: id, Valid: true})
  if err != nil {
    app.Logger.Error("failed to get chapters", "error", err, "movie_id", id)
    helpers.ErrorJSON(w, errors.New("failed to fetch chapters"))
    return
  }

  res := helpers.JSONResponse{
    Error: false,
    Data: map[string]any{
      "movie": map[string]any{
        "file_name": movie.FileName,
        "file_path": movie.FilePath,
        "size":      movie.Size,
        "container": movie.Container,
        "mime_type": movie.MimeType,
        "run_time":  movie.RunTime,
        "duration":  movie.Duration,
      },
      "video_streams": videoStreams,
      "audio_streams": audioStreams,
      "subtitles":     subtitles,
      "chapters":      chapters,
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
