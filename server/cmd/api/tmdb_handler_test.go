package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"igloo/cmd/internal/database"
	"igloo/cmd/internal/helpers"
	"igloo/cmd/internal/tmdb"

	"github.com/go-chi/chi/v5"
)

func TestTmdbSearchMovies_HTTPSearchRanksResults(t *testing.T) {
	app := setupTestApp(t)
	defer app.DB.Close()

	app.Tmdb = &stubMovieScannerTmdb{
		searchResults: []tmdb.TmdbMovie{
			{TmdbID: 1, Title: "Casino Royale", ReleaseDate: "1967-04-13", Popularity: 40, VoteAverage: 6.1},
			{TmdbID: 2, Title: "Casino Royale", ReleaseDate: "2006-11-14", Popularity: 35, VoteAverage: 7.6},
			{TmdbID: 3, Title: "Quantum of Solace", ReleaseDate: "2008-10-29", Popularity: 50, VoteAverage: 6.3},
		},
	}

	router := chi.NewRouter()
	router.Post("/api/movies/{id}/tmdb-search", app.TmdbSearchMovies)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/movies/10/tmdb-search", strings.NewReader(`{
		"title": "Casino Royale",
		"year": 2006
	}`))
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", w.Code, w.Body.String())
	}

	var resp struct {
		Error bool `json:"error"`
		Data  struct {
			Results []tmdb.TmdbMovieSearchResult `json:"results"`
		} `json:"data"`
	}
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp.Error {
		t.Fatalf("expected non-error response: %+v", resp)
	}
	if len(resp.Data.Results) != 3 {
		t.Fatalf("result count = %d, want 3", len(resp.Data.Results))
	}
	if resp.Data.Results[0].TmdbID != 2 {
		t.Fatalf("first tmdb id = %d, want 2 for 2006 match", resp.Data.Results[0].TmdbID)
	}
}

func TestTmdbSearchMovies_HTTPByID(t *testing.T) {
	app := setupTestApp(t)
	defer app.DB.Close()

	app.Tmdb = &stubMovieScannerTmdb{
		detailMovies: map[int]tmdb.TmdbMovie{
			603: {
				TmdbID:      603,
				Title:       "The Matrix",
				ReleaseDate: "1999-03-31",
				PosterPath:  "/poster.jpg",
			},
		},
	}

	router := chi.NewRouter()
	router.Post("/api/movies/{id}/tmdb-search", app.TmdbSearchMovies)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/movies/10/tmdb-search", strings.NewReader(`{"tmdb_id":603}`))
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", w.Code, w.Body.String())
	}

	var resp struct {
		Error bool `json:"error"`
		Data  struct {
			Results []tmdb.TmdbMovieSearchResult `json:"results"`
		} `json:"data"`
	}
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(resp.Data.Results) != 1 || resp.Data.Results[0].TmdbID != 603 || resp.Data.Results[0].Title != "The Matrix" {
		t.Fatalf("results = %+v, want single Matrix result", resp.Data.Results)
	}
}

func TestGetMovieByTmdbID_HTTP(t *testing.T) {
	app := setupTestApp(t)
	defer app.DB.Close()

	app.Tmdb = &stubMovieScannerTmdb{
		detailMovies: map[int]tmdb.TmdbMovie{
			603: {TmdbID: 603, Title: "The Matrix"},
		},
	}

	router := chi.NewRouter()
	router.Get("/api/tmdb/movies/{id}", app.GetMovieByTmdbID)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/tmdb/movies/603", nil)
	router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", w.Code, w.Body.String())
	}

	var resp struct {
		Error bool `json:"error"`
		Data  struct {
			Movie tmdb.TmdbMovie `json:"movie"`
		} `json:"data"`
	}
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp.Data.Movie.TmdbID != 603 || resp.Data.Movie.Title != "The Matrix" {
		t.Fatalf("movie = %+v, want The Matrix", resp.Data.Movie)
	}

	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/api/tmdb/movies/not-an-id", nil)
	router.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("invalid id status = %d, want 400", w.Code)
	}
}

func TestGetMoviesInTheaters_HTTPLimitsResults(t *testing.T) {
	app := setupTestApp(t)
	defer app.DB.Close()

	movies := make([]*tmdb.TmdbMovie, helpers.TMDB_MAX_ITEMS+5)
	for i := range movies {
		movies[i] = &tmdb.TmdbMovie{TmdbID: i + 1, Title: "Movie " + strconv.Itoa(i+1)}
	}
	app.Tmdb = &stubMovieScannerTmdb{theaterMovies: movies}

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/tmdb/movies/in-theaters", nil)
	app.GetMoviesInTheaters(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", w.Code, w.Body.String())
	}

	var resp struct {
		Error bool `json:"error"`
		Data  struct {
			Movies []tmdb.TmdbMovie `json:"movies"`
		} `json:"data"`
	}
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(resp.Data.Movies) != helpers.TMDB_MAX_ITEMS {
		t.Fatalf("movie count = %d, want capped count %d", len(resp.Data.Movies), helpers.TMDB_MAX_ITEMS)
	}
}

func TestGetMoviesInTheaters_HTTPError(t *testing.T) {
	app := setupTestApp(t)
	defer app.DB.Close()

	app.Tmdb = &stubMovieScannerTmdb{theatersErr: errors.New("tmdb unavailable")}

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/tmdb/movies/in-theaters", nil)
	app.GetMoviesInTheaters(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500", w.Code)
	}
}

func TestIdentifyMovie_HTTPPersistsTmdbMetadataAndRelationships(t *testing.T) {
	app := setupTestApp(t)
	defer app.DB.Close()

	ctx := context.Background()
	movie, err := app.Queries.UpsertMovie(ctx, database.UpsertMovieParams{
		Title:     "Unknown",
		FilePath:  "/movies/unknown.mkv",
		FileName:  "unknown.mkv",
		Size:      1,
		Container: "mkv",
		MimeType:  "video/x-matroska",
		Adult:     false,
	})
	if err != nil {
		t.Fatalf("insert movie: %v", err)
	}

	tmdbDetails := tmdbMovieFromJSON(t, `{
		"id": 603,
		"title": "The Matrix",
		"release_date": "1999-03-31",
		"poster_path": "/poster.jpg",
		"original_language": "en",
		"runtime": 136,
		"genres": [{"id": 28, "name": "Action"}],
		"credits": {
			"cast": [{"id": 6384, "name": "Keanu Reeves", "character": "Neo", "order": 0}],
			"crew": [{"id": 9339, "name": "Lana Wachowski", "job": "Director", "department": "Directing"}]
		},
		"videos": {"results": [{"id": "trailer", "key": "abc", "name": "Trailer", "site": "YouTube", "type": "Trailer"}]}
	}`)
	app.Tmdb = &stubMovieScannerTmdb{
		detailMovies: map[int]tmdb.TmdbMovie{603: tmdbDetails},
	}

	router := chi.NewRouter()
	router.Put("/api/movies/{id}/identify", app.IdentifyMovie)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPut, "/api/movies/"+strconv.FormatInt(movie.ID, 10)+"/identify", strings.NewReader(`{"tmdb_id":603}`))
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", w.Code, w.Body.String())
	}

	updated, err := app.Queries.GetMovieByID(ctx, movie.ID)
	if err != nil {
		t.Fatalf("get updated movie: %v", err)
	}
	if updated.Title != "The Matrix" || !updated.TmdbID.Valid || updated.TmdbID.Int64 != 603 ||
		!updated.Year.Valid || updated.Year.Int64 != 1999 || !updated.RunTime.Valid || updated.RunTime.Int64 != 136 {
		t.Fatalf("updated movie = %+v, want TMDB metadata", updated)
	}

	genres, err := app.Queries.GetGenresByMovieID(ctx, movie.ID)
	if err != nil {
		t.Fatalf("get genres: %v", err)
	}
	if movieGenreTags(genres) != "Action" {
		t.Fatalf("genres = %+v, want Action", genres)
	}

	cast, err := app.Queries.GetCastByMovieID(ctx, movie.ID)
	if err != nil {
		t.Fatalf("get cast: %v", err)
	}
	if len(cast) != 1 || cast[0].ArtistName != "Keanu Reeves" {
		t.Fatalf("cast = %+v, want Keanu Reeves", cast)
	}

	crew, err := app.Queries.GetCrewByMovieID(ctx, movie.ID)
	if err != nil {
		t.Fatalf("get crew: %v", err)
	}
	if len(crew) != 1 || crew[0].ArtistName != "Lana Wachowski" {
		t.Fatalf("crew = %+v, want Lana Wachowski", crew)
	}

	extras, err := app.Queries.GetMovieExtraVideos(ctx, movie.ID)
	if err != nil {
		t.Fatalf("get extras: %v", err)
	}
	if len(extras) != 1 || extras[0].Title != "Trailer" {
		t.Fatalf("extras = %+v, want Trailer", extras)
	}
}

func TestIdentifyMovie_HTTPErrorPaths(t *testing.T) {
	app := setupTestApp(t)
	defer app.DB.Close()

	router := chi.NewRouter()
	router.Put("/api/movies/{id}/identify", app.IdentifyMovie)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPut, "/api/movies/1/identify", strings.NewReader(`{"tmdb_id":603}`))
	router.ServeHTTP(w, req)
	if w.Code != http.StatusInternalServerError {
		t.Fatalf("unconfigured TMDB status = %d, want 500", w.Code)
	}

	app.Tmdb = &stubMovieScannerTmdb{detailMovies: map[int]tmdb.TmdbMovie{603: {TmdbID: 603, Title: "The Matrix"}}}

	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPut, "/api/movies/bad/identify", strings.NewReader(`{"tmdb_id":603}`))
	router.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("invalid movie id status = %d, want 400", w.Code)
	}

	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPut, "/api/movies/1/identify", strings.NewReader(`{"tmdb_id":0}`))
	router.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("invalid tmdb id status = %d, want 400", w.Code)
	}

	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPut, "/api/movies/999/identify", strings.NewReader(`{"tmdb_id":603}`))
	router.ServeHTTP(w, req)
	if w.Code != http.StatusNotFound {
		t.Fatalf("missing movie status = %d, want 404", w.Code)
	}
}

func TestUpdateMovieMetadata_DoesNotRequireRemovedLockFields(t *testing.T) {
	app := setupTestApp(t)
	defer app.DB.Close()

	ctx := context.Background()
	movie, err := app.Queries.UpsertMovie(ctx, database.UpsertMovieParams{
		Title:     "Original",
		FilePath:  "/movies/original.mkv",
		FileName:  "original.mkv",
		Size:      1,
		Container: "mkv",
		MimeType:  "video/x-matroska",
		Adult:     false,
	})
	if err != nil {
		t.Fatalf("insert movie: %v", err)
	}

	router := chi.NewRouter()
	router.Patch("/api/movies/{id}", app.UpdateMovieMetadata)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPatch, "/api/movies/"+strconv.FormatInt(movie.ID, 10), strings.NewReader(`{"title":"Updated"}`))
	router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", w.Code, w.Body.String())
	}

	updated, err := app.Queries.GetMovieByID(ctx, movie.ID)
	if err != nil {
		t.Fatalf("get updated movie: %v", err)
	}
	if updated.Title != "Updated" {
		t.Fatalf("title = %q, want Updated", updated.Title)
	}
}

func TestBuildUpdateParamsFromTmdbMapsNullableFields(t *testing.T) {
	movie := tmdbMovieFromJSON(t, `{
		"id": 603,
		"title": "The Matrix",
		"release_date": "1999-03-31",
		"imdb_id": "tt0133093",
		"poster_path": "/poster.jpg",
		"backdrop_path": "/backdrop.jpg",
		"adult": false,
		"original_language": "en",
		"overview": "Overview",
		"tagline": "Tagline",
		"vote_average": 8.2,
		"revenue": 463517383,
		"budget": 63000000,
		"runtime": 136,
		"release_dates": {
			"results": [{"iso_3166_1": "US", "release_dates": [{"certification": "R"}]}]
		}
	}`)
	params := buildUpdateParamsFromTmdb(10, &movie)
	if params.ID != 10 || params.Title != "The Matrix" || !params.TmdbID.Valid || params.TmdbID.Int64 != 603 ||
		!params.Year.Valid || params.Year.Int64 != 1999 || !params.Certification.Valid || params.Certification.String != "R" {
		t.Fatalf("params = %+v, want mapped TMDB fields", params)
	}
}

func TestUpdateMovieMetadata_PreservesOmittedNullableFields(t *testing.T) {
	app := setupTestApp(t)
	defer app.DB.Close()

	ctx := context.Background()
	movie, err := app.Queries.UpsertMovie(ctx, database.UpsertMovieParams{
		Title:         "Original",
		FilePath:      "/movies/original-preserve.mkv",
		FileName:      "original-preserve.mkv",
		Size:          1,
		Container:     "mkv",
		MimeType:      "video/x-matroska",
		Adult:         false,
		Overview:      helpers.NullString("Keep overview"),
		ReleaseDate:   helpers.NullString("2001-01-01"),
		Year:          helpers.NullInt64(2001),
		Certification: sql.NullString{String: "PG", Valid: true},
	})
	if err != nil {
		t.Fatalf("insert movie: %v", err)
	}

	router := chi.NewRouter()
	router.Patch("/api/movies/{id}", app.UpdateMovieMetadata)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPatch, "/api/movies/"+strconv.FormatInt(movie.ID, 10), strings.NewReader(`{"title":"Updated"}`))
	router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", w.Code, w.Body.String())
	}

	updated, err := app.Queries.GetMovieByID(ctx, movie.ID)
	if err != nil {
		t.Fatalf("get updated movie: %v", err)
	}
	if updated.Title != "Updated" ||
		!updated.Overview.Valid || updated.Overview.String != "Keep overview" ||
		!updated.Year.Valid || updated.Year.Int64 != 2001 ||
		!updated.Certification.Valid || updated.Certification.String != "PG" {
		t.Fatalf("updated movie = %+v, want omitted nullable fields preserved", updated)
	}
}
