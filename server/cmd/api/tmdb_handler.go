package main

import (
	"context"
	"database/sql"
	"errors"
	"igloo/cmd/internal/helpers"
	"igloo/cmd/internal/tmdb"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"
)

const (
	tmdbImageBaseURL = "https://image.tmdb.org/t/p"
	tmdbImageSize    = "original"
	tmdbBackdropSize = "w1280"
	tmdbPosterSize   = "w500"
	tmdbProfileSize  = "w185"
	tmdbLogoSize     = "w92"
	tmdbMaxItems     = 12
)

type tmdbSearchPayload struct {
	Title  string `json:"title"`
	Year   int    `json:"year"`
	TmdbID int    `json:"tmdb_id"`
}

type tmdbSearchResult struct {
	TmdbID           int    `json:"tmdb_id"`
	Title            string `json:"title"`
	ReleaseDate      string `json:"release_date"`
	Overview         string `json:"overview"`
	PosterPath       string `json:"poster_path"`
	AlreadyInLibrary bool   `json:"already_in_library"`
	LibraryMovieID   *int64 `json:"library_movie_id,omitempty"`
}

type theaterMovieResponse struct {
	TmdbID        int     `json:"id"`
	Title         string  `json:"title"`
	OriginalTitle string  `json:"original_title"`
	Overview      string  `json:"overview"`
	ReleaseDate   string  `json:"release_date"`
	PosterPath    string  `json:"poster_path"`
	BackdropPath  string  `json:"backdrop_path"`
	Popularity    float64 `json:"popularity"`
	VoteAverage   float64 `json:"vote_average"`
	VoteCount     int     `json:"vote_count"`
	Adult         bool    `json:"adult"`
	OriginalLang  string  `json:"original_language"`
	GenreIDs      []int   `json:"genre_ids"`
	Video         bool    `json:"video"`
}

var supportedTmdbImageSizes = map[string]bool{
	tmdbImageSize:    true,
	tmdbBackdropSize: true,
	tmdbPosterSize:   true,
	tmdbProfileSize:  true,
	tmdbLogoSize:     true,
}

func (app *Application) SearchTmdbMovies(w http.ResponseWriter, r *http.Request) {
	app.handleTmdbMovieSearch(w, r)
}

func (app *Application) TmdbSearchMovies(w http.ResponseWriter, r *http.Request) {
	app.handleTmdbMovieSearch(w, r)
}

func (app *Application) handleTmdbMovieSearch(w http.ResponseWriter, r *http.Request) {
	if !app.ensureTmdbAvailable(w) {
		return
	}

	ctx := r.Context()

	var payload tmdbSearchPayload

	err := helpers.ReadJSON(w, r, &payload, 0)
	if err != nil {
		helpers.ErrorJSON(w, errors.New(invalidRequestBodyMessage), http.StatusBadRequest)
		return
	}

	payload.Title = strings.TrimSpace(payload.Title)
	if payload.TmdbID <= 0 && payload.Title == "" {
		helpers.ErrorJSON(w, errors.New("title or tmdb_id is required"), http.StatusBadRequest)
		return
	}

	results, err := app.searchTmdbMovies(ctx, payload)
	if err != nil {
		app.Logger.Error("tmdb search failed", "error", err, "title", payload.Title, "tmdb_id", payload.TmdbID)
		helpers.ErrorJSON(w, err)
		return
	}

	helpers.WriteJSON(w, http.StatusOK, helpers.JSONResponse{
		Error: false,
		Data:  map[string]any{"results": results},
	})
}

func (app *Application) GetTmdbStatus(w http.ResponseWriter, r *http.Request) {
	helpers.WriteJSON(w, http.StatusOK, helpers.JSONResponse{
		Error: false,
		Data: map[string]any{
			"available": app.Tmdb != nil,
		},
	})
}

func (app *Application) ProxyTmdbImage(w http.ResponseWriter, r *http.Request) {
	size := chi.URLParam(r, "size")
	file := chi.URLParam(r, "file")
	if !supportedTmdbImageSizes[size] || !isSafeTmdbImageFile(file) {
		helpers.ErrorJSON(w, errors.New("invalid TMDB image path"), http.StatusBadRequest)
		return
	}

	baseURL := strings.TrimRight(app.TmdbImageBaseURL, "/")
	if baseURL == "" {
		baseURL = tmdbImageBaseURL
	}

	imageURL, err := url.JoinPath(baseURL, size, file)
	if err != nil {
		app.Logger.Error("failed to build TMDB image proxy URL", "error", err, "base_url", baseURL)
		helpers.ErrorJSON(w, errors.New("failed to fetch TMDB image"), http.StatusBadGateway)
		return
	}

	req, err := http.NewRequestWithContext(r.Context(), http.MethodGet, imageURL, nil)
	if err != nil {
		app.Logger.Error("failed to build TMDB image proxy request", "error", err, "url", imageURL)
		helpers.ErrorJSON(w, errors.New("failed to fetch TMDB image"), http.StatusBadGateway)
		return
	}

	client := app.TmdbImageHTTPClient
	if client == nil {
		client = &http.Client{Timeout: helpers.TMDB_HTTP_TIMEOUT}
	}

	resp, err := client.Do(req)
	if err != nil {
		app.Logger.Error("failed to fetch TMDB image", "error", err, "url", imageURL)
		helpers.ErrorJSON(w, errors.New("failed to fetch TMDB image"), http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		app.Logger.Error("TMDB image upstream returned an error", "status", resp.StatusCode, "url", imageURL)
		helpers.ErrorJSON(w, errors.New("failed to fetch TMDB image"), http.StatusBadGateway)
		return
	}

	for _, header := range []string{"Content-Type", "Cache-Control", "ETag", "Last-Modified"} {
		value := resp.Header.Get(header)
		if value != "" {
			w.Header().Set(header, value)
		}
	}
	if w.Header().Get("Content-Type") == "" {
		w.Header().Set("Content-Type", "application/octet-stream")
	}
	if resp.ContentLength >= 0 {
		w.Header().Set("Content-Length", strconv.FormatInt(resp.ContentLength, 10))
	}
	w.WriteHeader(http.StatusOK)

	_, err = io.Copy(w, resp.Body)
	if err != nil {
		app.Logger.Error("failed to stream TMDB image response", "error", err, "url", imageURL)
	}
}

func isSafeTmdbImageFile(file string) bool {
	if file == "" || strings.Contains(file, "..") || strings.ContainsAny(file, `/\`) {
		return false
	}

	for _, char := range file {
		if char >= 'a' && char <= 'z' {
			continue
		}
		if char >= 'A' && char <= 'Z' {
			continue
		}
		if char >= '0' && char <= '9' {
			continue
		}
		if char == '.' || char == '_' || char == '-' {
			continue
		}
		return false
	}

	return true
}

func (app *Application) searchTmdbMovies(ctx context.Context, payload tmdbSearchPayload) ([]tmdbSearchResult, error) {
	if payload.TmdbID > 0 {
		movie := &tmdb.TmdbMovie{TmdbID: payload.TmdbID}

		err := app.Tmdb.GetTmdbMovieByID(ctx, movie)
		if err != nil {
			return nil, errors.New("failed to fetch movie from TMDB")
		}

		return app.mapTmdbSearchResults(ctx, []*tmdb.TmdbMovie{movie}), nil
	}

	searchTitle := normalizeMovieTitleForSearch(payload.Title)
	if searchTitle == "" {
		searchTitle = payload.Title
	}

	var (
		results []tmdb.TmdbMovie
		err     error
	)
	if payload.Year > 0 {
		results, err = app.Tmdb.SearchMoviesByTitleAndYear(ctx, searchTitle, payload.Year)
	} else {
		results, err = app.Tmdb.SearchMoviesByTitleAndYear(ctx, searchTitle)
	}
	if err != nil {
		if err.Error() == "no movies found with the given query" {
			return []tmdbSearchResult{}, nil
		}
		return nil, errors.New("TMDB search failed")
	}

	ranked := rankTmdbMatches(results, searchTitle, payload.Year)
	mapped := make([]*tmdb.TmdbMovie, 0, len(results))
	for _, candidate := range ranked {
		mapped = append(mapped, candidate.Movie)
	}

	return app.mapTmdbSearchResults(ctx, mapped), nil
}

func (app *Application) GetMoviesInTheaters(w http.ResponseWriter, r *http.Request) {
	if !app.ensureTmdbAvailable(w) {
		return
	}

	movies, err := app.Tmdb.GetMoviesInTheaters(r.Context())
	if err != nil {
		app.Logger.Error("failed to get movies in theaters", "error", err)
		helpers.ErrorJSON(w, errors.New("failed to fetch movies in theaters"))
		return
	}

	mapped := make([]theaterMovieResponse, 0, min(len(movies), tmdbMaxItems))
	for _, movie := range movies {
		if movie == nil {
			continue
		}

		genreIDs := movie.GenreIDs
		if genreIDs == nil {
			genreIDs = []int{}
		}

		mapped = append(mapped, theaterMovieResponse{
			TmdbID:        movie.TmdbID,
			Title:         movie.Title,
			OriginalTitle: movie.OriginalTitle,
			Overview:      movie.Overview,
			ReleaseDate:   movie.ReleaseDate,
			PosterPath:    movie.PosterPath,
			BackdropPath:  movie.BackdropPath,
			Popularity:    movie.Popularity,
			VoteAverage:   movie.VoteAverage,
			VoteCount:     movie.VoteCount,
			Adult:         movie.Adult,
			OriginalLang:  movie.OriginalLang,
			GenreIDs:      genreIDs,
			Video:         movie.Video,
		})
		if len(mapped) == tmdbMaxItems {
			break
		}
	}

	res := helpers.JSONResponse{
		Error: false,
		Data: map[string]any{
			"movies": mapped,
		},
	}

	helpers.WriteJSON(w, http.StatusOK, res)
}

func (app *Application) GetMovieByTmdbID(w http.ResponseWriter, r *http.Request) {
	if !app.ensureTmdbAvailable(w) {
		return
	}

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

func (app *Application) ensureTmdbAvailable(w http.ResponseWriter) bool {
	if app.Tmdb != nil {
		return true
	}

	helpers.ErrorJSON(w, errors.New("TMDB search is unavailable"), http.StatusServiceUnavailable)
	return false
}

func (app *Application) mapTmdbSearchResults(ctx context.Context, movies []*tmdb.TmdbMovie) []tmdbSearchResult {
	mapped := make([]tmdbSearchResult, 0, len(movies))

	for _, movie := range movies {
		if movie == nil {
			continue
		}

		result := tmdbSearchResult{
			TmdbID:      movie.TmdbID,
			Title:       movie.Title,
			ReleaseDate: movie.ReleaseDate,
			Overview:    movie.Overview,
			PosterPath:  movie.PosterPath,
		}

		existingMovie, err := app.Queries.GetMovieByTmdbID(ctx, helpers.NullInt64(int64(movie.TmdbID)))
		if err == nil {
			result.AlreadyInLibrary = true
			result.LibraryMovieID = &existingMovie.ID
		} else if !errors.Is(err, sql.ErrNoRows) {
			app.Logger.Error("failed to look up existing movie by tmdb id", "error", err, "tmdb_id", movie.TmdbID)
		}

		mapped = append(mapped, result)
	}

	return mapped
}
