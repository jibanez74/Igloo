package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"io"
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

type failingRoundTripper struct{}

func (failingRoundTripper) RoundTrip(*http.Request) (*http.Response, error) {
	return nil, errors.New("upstream unavailable")
}

type stubTmdbClient struct {
	searchErr     error
	detailErr     error
	theatersErr   error
	searchResults []tmdb.TmdbMovie
	detailMovies  map[int]tmdb.TmdbMovie
	theaterMovies []*tmdb.TmdbMovie
	searchCalls   []stubTmdbSearchCall
	detailCalls   []int
}

type stubTmdbSearchCall struct {
	title string
	year  []int
}

func (s *stubTmdbClient) GetTmdbMovieByID(_ context.Context, movie *tmdb.TmdbMovie) error {
	s.detailCalls = append(s.detailCalls, movie.TmdbID)
	if s.detailErr != nil {
		return s.detailErr
	}
	if s.detailMovies == nil {
		return errors.New("tmdb details unavailable")
	}
	details, ok := s.detailMovies[movie.TmdbID]
	if !ok {
		return errors.New("tmdb details unavailable")
	}
	*movie = details
	return nil
}

func (s *stubTmdbClient) SearchMoviesByTitleAndYear(_ context.Context, title string, year ...int) ([]tmdb.TmdbMovie, error) {
	yearCopy := append([]int(nil), year...)
	s.searchCalls = append(s.searchCalls, stubTmdbSearchCall{title: title, year: yearCopy})
	if s.searchErr != nil {
		return nil, s.searchErr
	}
	results := make([]tmdb.TmdbMovie, len(s.searchResults))
	copy(results, s.searchResults)
	return results, nil
}

func (s *stubTmdbClient) GetMoviesInTheaters(_ context.Context) ([]*tmdb.TmdbMovie, error) {
	if s.theatersErr != nil {
		return nil, s.theatersErr
	}
	return s.theaterMovies, nil
}

func (*stubTmdbClient) ClearCache() {}

func tmdbMovieFromJSON(t *testing.T, payload string) tmdb.TmdbMovie {
	t.Helper()

	var movie tmdb.TmdbMovie
	err := json.Unmarshal([]byte(payload), &movie)
	if err != nil {
		t.Fatalf("unmarshal TMDB movie fixture: %v", err)
	}
	return movie
}

func movieGenreTags(genres []database.GetGenresByMovieIDRow) string {
	tags := make([]string, 0, len(genres))
	for _, genre := range genres {
		tags = append(tags, genre.Tag)
	}
	return strings.Join(tags, ",")
}

func TestTmdbSearchMovies_HTTPSearchRanksResults(t *testing.T) {
	app := setupTestApp(t)
	defer app.DB.Close()

	app.Tmdb = &stubTmdbClient{
		searchResults: []tmdb.TmdbMovie{
			{TmdbID: 1, Title: "Casino Royale", ReleaseDate: "1967-04-13", Popularity: 40, VoteAverage: 6.1},
			{TmdbID: 2, Title: "Casino Royale", ReleaseDate: "2006-11-14", Popularity: 35, VoteAverage: 7.6},
			{TmdbID: 3, Title: "Quantum of Solace", ReleaseDate: "2008-10-29", Popularity: 50, VoteAverage: 6.3},
		},
	}

	router := chi.NewRouter()
	router.Post("/api/movies/{id}/tmdb-search", app.TmdbSearchMovies)

	w := httptest.NewRecorder()
	req := newOpenAPIJSONRequest(http.MethodPost, "/api/movies/10/tmdb-search", `{
		"title": "Casino Royale",
		"year": 2006
	}`)
	req.AddCookie(&http.Cookie{Name: "session", Value: "openapi-contract"})
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", w.Code, w.Body.String())
	}
	assertOpenAPIExchange(t, "tmdbSearchMovies", req, w)

	var resp struct {
		Error bool `json:"error"`
		Data  struct {
			Results []tmdbSearchResult `json:"results"`
		} `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
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

func TestSearchTmdbMovies_HTTPMarksExistingLibraryMatches(t *testing.T) {
	app := setupTestApp(t)
	defer app.DB.Close()

	ctx := context.Background()
	existingMovie, err := app.Queries.UpsertMovie(ctx, database.UpsertMovieParams{
		Title:     "The Matrix",
		FilePath:  "/movies/the-matrix.mkv",
		FileName:  "the-matrix.mkv",
		Size:      1,
		Container: "mkv",
		MimeType:  "video/x-matroska",
		Adult:     false,
		TmdbID:    helpers.NullInt64(603),
	})
	if err != nil {
		t.Fatalf("insert existing movie: %v", err)
	}

	stub := &stubTmdbClient{
		searchResults: []tmdb.TmdbMovie{
			{TmdbID: 603, Title: "The Matrix", ReleaseDate: "1999-03-31", PosterPath: "/matrix.jpg"},
			{TmdbID: 604, Title: "The Matrix Reloaded", ReleaseDate: "2003-05-15", PosterPath: "/reloaded.jpg"},
		},
	}
	app.Tmdb = stub

	router := chi.NewRouter()
	router.Post("/api/tmdb/movies/search", app.SearchTmdbMovies)

	w := httptest.NewRecorder()
	req := newOpenAPIJSONRequest(http.MethodPost, "/api/tmdb/movies/search", `{
		"title": "The Matrix",
		"year": 1999
	}`)
	req.AddCookie(&http.Cookie{Name: "session", Value: "openapi-contract"})
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", w.Code, w.Body.String())
	}
	assertOpenAPIExchange(t, "searchTmdbMovies", req, w)

	var resp struct {
		Error bool `json:"error"`
		Data  struct {
			Results []struct {
				TmdbID           int    `json:"tmdb_id"`
				Title            string `json:"title"`
				AlreadyInLibrary bool   `json:"already_in_library"`
				LibraryMovieID   *int64 `json:"library_movie_id"`
			} `json:"results"`
		} `json:"data"`
	}
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp.Error {
		t.Fatalf("expected success response: %+v", resp)
	}
	if len(resp.Data.Results) != 2 {
		t.Fatalf("result count = %d, want 2", len(resp.Data.Results))
	}
	if !resp.Data.Results[0].AlreadyInLibrary {
		t.Fatalf("expected first result to be flagged as already in library: %+v", resp.Data.Results[0])
	}
	if resp.Data.Results[0].LibraryMovieID == nil || *resp.Data.Results[0].LibraryMovieID != existingMovie.ID {
		t.Fatalf("library_movie_id = %v, want %d", resp.Data.Results[0].LibraryMovieID, existingMovie.ID)
	}
	if len(stub.searchCalls) != 1 || len(stub.searchCalls[0].year) != 1 || stub.searchCalls[0].year[0] != 1999 {
		t.Fatalf("expected TMDB search to receive year 1999, got %+v", stub.searchCalls)
	}
}

func TestTmdbSearchMovies_HTTPByID(t *testing.T) {
	app := setupTestApp(t)
	defer app.DB.Close()

	app.Tmdb = &stubTmdbClient{
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
			Results []tmdbSearchResult `json:"results"`
		} `json:"data"`
	}
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(resp.Data.Results) != 1 || resp.Data.Results[0].TmdbID != 603 || resp.Data.Results[0].Title != "The Matrix" {
		t.Fatalf("results = %+v, want single Matrix result", resp.Data.Results)
	}
}

func TestSearchTmdbMovies_HTTPUnavailable(t *testing.T) {
	app := setupTestApp(t)
	defer app.DB.Close()

	router := chi.NewRouter()
	router.Post("/api/tmdb/movies/search", app.SearchTmdbMovies)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/tmdb/movies/search", strings.NewReader(`{"title":"Arrival"}`))
	router.ServeHTTP(w, req)

	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want 503, body = %s", w.Code, w.Body.String())
	}
}

func TestGetMovieByTmdbID_HTTP(t *testing.T) {
	app := setupTestApp(t)
	defer app.DB.Close()

	app.Tmdb = &stubTmdbClient{
		detailMovies: map[int]tmdb.TmdbMovie{
			603: {TmdbID: 603, Title: "The Matrix"},
		},
	}

	router := chi.NewRouter()
	router.Get("/api/tmdb/movies/{id}", app.GetMovieByTmdbID)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/tmdb/movies/603", nil)
	req.AddCookie(&http.Cookie{Name: "session", Value: "openapi-contract"})
	router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", w.Code, w.Body.String())
	}
	assertOpenAPIExchange(t, "getMovieByTmdbID", req, w)

	var rawResp struct {
		Data struct {
			Movie map[string]json.RawMessage `json:"movie"`
		} `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &rawResp); err != nil {
		t.Fatalf("decode sparse response: %v", err)
	}
	for _, field := range []string{"genre_ids", "production_companies", "genres"} {
		if string(rawResp.Data.Movie[field]) != "null" {
			t.Fatalf("sparse movie %s = %s, want null", field, rawResp.Data.Movie[field])
		}
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

func TestGetTmdbStatus_HTTP(t *testing.T) {
	app := setupTestApp(t)
	defer app.DB.Close()

	router := chi.NewRouter()
	router.Get("/api/tmdb/status", app.GetTmdbStatus)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/tmdb/status", nil)
	req.AddCookie(&http.Cookie{Name: "session", Value: "openapi-contract"})
	router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", w.Code, w.Body.String())
	}
	assertOpenAPIExchange(t, "getTmdbStatus", req, w)

	var unavailableResp struct {
		Error bool `json:"error"`
		Data  struct {
			Available bool `json:"available"`
		} `json:"data"`
	}
	if err := json.NewDecoder(w.Body).Decode(&unavailableResp); err != nil {
		t.Fatalf("decode unavailable response: %v", err)
	}
	if unavailableResp.Data.Available {
		t.Fatal("expected TMDB status to be unavailable when app.Tmdb is nil")
	}

	app.Tmdb = &stubTmdbClient{}
	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/api/tmdb/status", nil)
	router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", w.Code, w.Body.String())
	}

	var availableResp struct {
		Error bool `json:"error"`
		Data  struct {
			Available bool `json:"available"`
		} `json:"data"`
	}
	if err := json.NewDecoder(w.Body).Decode(&availableResp); err != nil {
		t.Fatalf("decode available response: %v", err)
	}
	if !availableResp.Data.Available {
		t.Fatal("expected TMDB status to be available when app.Tmdb is configured")
	}
}

func TestProxyTmdbImage_HTTPSuccessStreamsImage(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/w500/poster.jpg" {
			t.Fatalf("upstream path = %q, want /w500/poster.jpg", r.URL.Path)
		}

		w.Header().Set("Content-Type", "image/jpeg")
		w.Header().Set("Cache-Control", "public, max-age=86400")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte{1, 2, 3, 4})
	}))
	defer upstream.Close()

	app := setupTestApp(t)
	defer app.DB.Close()
	app.TmdbImageBaseURL = upstream.URL
	app.TmdbImageHTTPClient = upstream.Client()

	router := chi.NewRouter()
	router.Get("/api/tmdb/images/{size}/{file}", app.ProxyTmdbImage)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/tmdb/images/w500/poster.jpg", nil)
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", w.Code, w.Body.String())
	}
	if got := w.Header().Get("Content-Type"); got != "image/jpeg" {
		t.Fatalf("content type = %q, want image/jpeg", got)
	}
	if got := w.Header().Get("Cache-Control"); got != "public, max-age=86400" {
		t.Fatalf("cache control = %q, want upstream cache header", got)
	}
	if got := w.Body.Bytes(); string(got) != string([]byte{1, 2, 3, 4}) {
		t.Fatalf("body = %#v, want image bytes", got)
	}
}

func TestProxyTmdbImage_HTTPRejectsInvalidPath(t *testing.T) {
	app := setupTestApp(t)
	defer app.DB.Close()

	router := chi.NewRouter()
	router.Get("/api/tmdb/images/{size}/{file}", app.ProxyTmdbImage)

	tests := []string{
		"/api/tmdb/images/w999/poster.jpg",
		"/api/tmdb/images/w500/bad..jpg",
		"/api/tmdb/images/w500/poster$.jpg",
	}

	for _, path := range tests {
		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, path, nil)
		router.ServeHTTP(w, req)

		if w.Code != http.StatusBadRequest {
			t.Fatalf("%s status = %d, want 400, body = %s", path, w.Code, w.Body.String())
		}

		var resp helpers.JSONResponse
		if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
			t.Fatalf("decode invalid response: %v", err)
		}
		if !resp.Error {
			t.Fatalf("%s response = %+v, want error", path, resp)
		}
	}
}

func TestProxyTmdbImage_HTTPReturnsErrorForUpstreamFailures(t *testing.T) {
	t.Run("non-200", func(t *testing.T) {
		upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			http.Error(w, "missing", http.StatusNotFound)
		}))
		defer upstream.Close()

		app := setupTestApp(t)
		defer app.DB.Close()
		app.TmdbImageBaseURL = upstream.URL
		app.TmdbImageHTTPClient = upstream.Client()

		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/api/tmdb/images/w500/missing.jpg", nil)
		router := chi.NewRouter()
		router.Get("/api/tmdb/images/{size}/{file}", app.ProxyTmdbImage)
		router.ServeHTTP(w, req)

		if w.Code != http.StatusBadGateway {
			t.Fatalf("status = %d, want 502, body = %s", w.Code, w.Body.String())
		}
		if got := w.Header().Get("Content-Type"); got != "application/json" {
			t.Fatalf("content type = %q, want application/json", got)
		}
	})

	t.Run("fetch error", func(t *testing.T) {
		app := setupTestApp(t)
		defer app.DB.Close()
		app.TmdbImageBaseURL = "https://image.tmdb.org/t/p"
		app.TmdbImageHTTPClient = &http.Client{Transport: failingRoundTripper{}}

		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/api/tmdb/images/w500/poster.jpg", nil)
		router := chi.NewRouter()
		router.Get("/api/tmdb/images/{size}/{file}", app.ProxyTmdbImage)
		router.ServeHTTP(w, req)

		if w.Code != http.StatusBadGateway {
			t.Fatalf("status = %d, want 502, body = %s", w.Code, w.Body.String())
		}

		body, err := io.ReadAll(w.Result().Body)
		if err != nil {
			t.Fatalf("read body: %v", err)
		}
		var resp helpers.JSONResponse
		if err := json.Unmarshal(body, &resp); err != nil {
			t.Fatalf("decode fetch error response: %v\nbody=%s", err, string(body))
		}
		if !resp.Error {
			t.Fatalf("response = %+v, want error", resp)
		}
	})
}

func TestGetMoviesInTheaters_HTTPLimitsResults(t *testing.T) {
	app := setupTestApp(t)
	defer app.DB.Close()

	movies := make([]*tmdb.TmdbMovie, tmdbMaxItems+6)
	for i := 1; i < len(movies); i++ {
		movies[i] = &tmdb.TmdbMovie{
			TmdbID:        i,
			Title:         "Movie " + strconv.Itoa(i),
			OriginalTitle: "Original " + strconv.Itoa(i),
			Overview:      "Overview",
			ReleaseDate:   "2026-01-01",
			PosterPath:    "/poster.jpg",
			BackdropPath:  "/backdrop.jpg",
			Popularity:    12.5,
			VoteAverage:   8.25,
			VoteCount:     100,
			OriginalLang:  "en",
		}
	}
	app.Tmdb = &stubTmdbClient{theaterMovies: movies}

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/tmdb/movies/in-theaters", nil)
	req.AddCookie(&http.Cookie{Name: "session", Value: "openapi-contract"})
	app.GetMoviesInTheaters(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", w.Code, w.Body.String())
	}
	assertOpenAPIExchange(t, "getMoviesInTheaters", req, w)

	var resp struct {
		Error bool `json:"error"`
		Data  struct {
			Movies []theaterMovieResponse `json:"movies"`
		} `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(resp.Data.Movies) != tmdbMaxItems {
		t.Fatalf("movie count = %d, want capped count %d", len(resp.Data.Movies), tmdbMaxItems)
	}
	if resp.Data.Movies[0].TmdbID != 1 || resp.Data.Movies[tmdbMaxItems-1].TmdbID != tmdbMaxItems {
		t.Fatalf("movie order = first %d, last %d; want 1 through %d", resp.Data.Movies[0].TmdbID, resp.Data.Movies[tmdbMaxItems-1].TmdbID, tmdbMaxItems)
	}

	var rawResp struct {
		Data struct {
			Movies []map[string]json.RawMessage `json:"movies"`
		} `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &rawResp); err != nil {
		t.Fatalf("decode raw theater response: %v", err)
	}
	wantFields := []string{
		"id", "title", "original_title", "overview", "release_date", "poster_path", "backdrop_path",
		"popularity", "vote_average", "vote_count", "adult", "original_language", "genre_ids", "video",
	}
	first := rawResp.Data.Movies[0]
	if len(first) != len(wantFields) {
		t.Fatalf("theater movie fields = %v, want exactly %v", first, wantFields)
	}
	for _, field := range wantFields {
		if _, ok := first[field]; !ok {
			t.Errorf("theater movie missing field %q", field)
		}
	}
	if string(first["genre_ids"]) != "[]" {
		t.Errorf("genre_ids = %s, want []", first["genre_ids"])
	}
	for _, field := range []string{"runtime", "status", "tagline", "budget", "revenue", "homepage", "imdb_id", "production_companies", "genres", "credits", "videos", "release_dates"} {
		if _, ok := first[field]; ok {
			t.Errorf("theater movie unexpectedly includes detail field %q", field)
		}
	}
}

func TestGetMoviesInTheaters_HTTPError(t *testing.T) {
	app := setupTestApp(t)
	defer app.DB.Close()

	app.Tmdb = &stubTmdbClient{theatersErr: errors.New("tmdb unavailable")}

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
	app.Tmdb = &stubTmdbClient{
		detailMovies: map[int]tmdb.TmdbMovie{603: tmdbDetails},
	}

	router := chi.NewRouter()
	router.Put("/api/movies/{id}/identify", app.IdentifyMovie)

	w := httptest.NewRecorder()
	req := newOpenAPIJSONRequest(http.MethodPut, "/api/movies/"+strconv.FormatInt(movie.ID, 10)+"/identify", `{"tmdb_id":603}`)
	addOpenAPITestCookie(req)
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", w.Code, w.Body.String())
	}
	assertOpenAPIExchange(t, "identifyMovie", req, w)

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

	app.Tmdb = &stubTmdbClient{detailMovies: map[int]tmdb.TmdbMovie{603: {TmdbID: 603, Title: "The Matrix"}}}

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
	req := newOpenAPIJSONRequest(http.MethodPatch, "/api/movies/"+strconv.FormatInt(movie.ID, 10), `{"title":"Updated"}`)
	addOpenAPITestCookie(req)
	router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", w.Code, w.Body.String())
	}
	assertOpenAPIExchange(t, "updateMovieMetadata", req, w)

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
