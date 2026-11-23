package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"igloo/cmd/internal/database"
	"igloo/cmd/internal/helpers"
	"igloo/cmd/internal/tmdb"
)

type failingRoundTripper struct{}

func (failingRoundTripper) RoundTrip(*http.Request) (*http.Response, error) {
	return nil, errors.New("upstream unavailable")
}

type stubTmdbClient struct {
	person        *tmdb.Person
	personErr     error
	searchErr     error
	theatersErr   error
	searchResults []tmdb.TmdbMovie
	detailMovies  map[int]tmdb.TmdbMovie
	theaterMovies []*tmdb.TmdbMovie
	searchCalls   []stubTmdbSearchCall
}

type stubTmdbSearchCall struct {
	title string
	year  []int
}

func (s *stubTmdbClient) GetTmdbMovieByID(_ context.Context, movie *tmdb.TmdbMovie) error {
	if s.detailMovies == nil {
		return errors.New("tmdb details unavailable")
	}
	details, ok := s.detailMovies[movie.TmdbID]
	if !ok {
		return &tmdb.StatusError{StatusCode: http.StatusNotFound, Message: "unable to get movie from tmdb"}
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

func TestSearchTmdbMovies_HTTPSearchRanksResults(t *testing.T) {
	app := setupTestApp(t)

	app.Tmdb = &stubTmdbClient{
		searchResults: []tmdb.TmdbMovie{
			{TmdbID: 1, Title: "Casino Royale", ReleaseDate: "1967-04-13", Popularity: 40, VoteAverage: 6.1},
			{TmdbID: 2, Title: "Casino Royale", ReleaseDate: "2006-11-14", Popularity: 35, VoteAverage: 7.6},
			{TmdbID: 3, Title: "Quantum of Solace", ReleaseDate: "2008-10-29", Popularity: 50, VoteAverage: 6.3},
		},
	}

	actor := createTestUser(t, app, "Actor", "actor@example.com", true)
	router := authenticatedRouter(t, app, actor.ID)

	w := httptest.NewRecorder()
	req := newOpenAPIJSONRequest(http.MethodPost, "/api/tmdb/movies/search", `{
		"title": "Casino Royale",
		"year": 2006
	}`)
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", w.Code, w.Body.String())
	}
	assertOpenAPIExchange(t, "searchTmdbMovies", req, w)

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

	ctx := context.Background()
	existingMovieID, err := app.Queries.UpsertMovie(ctx, database.UpsertMovieParams{
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
	existingMovie, err := app.Queries.GetMovieByID(ctx, existingMovieID)
	if err != nil {
		t.Fatal(err)
	}
	// A second file of the same film: the result links the first one, as the
	// person page does.
	_, err = app.Queries.UpsertMovie(ctx, database.UpsertMovieParams{
		Title:     "The Matrix",
		FilePath:  "/movies/the-matrix-remaster.mkv",
		FileName:  "the-matrix-remaster.mkv",
		Size:      1,
		Container: "mkv",
		MimeType:  "video/x-matroska",
		Adult:     false,
		TmdbID:    helpers.NullInt64(603),
	})
	if err != nil {
		t.Fatalf("insert second file of the movie: %v", err)
	}

	stub := &stubTmdbClient{
		searchResults: []tmdb.TmdbMovie{
			{TmdbID: 603, Title: "The Matrix", ReleaseDate: "1999-03-31", PosterPath: "/matrix.jpg"},
			{TmdbID: 604, Title: "The Matrix Reloaded", ReleaseDate: "2003-05-15", PosterPath: "/reloaded.jpg"},
		},
	}
	app.Tmdb = stub

	actor := createTestUser(t, app, "Actor", "actor@example.com", false)
	router := authenticatedRouter(t, app, actor.ID)

	w := httptest.NewRecorder()
	req := newOpenAPIJSONRequest(http.MethodPost, "/api/tmdb/movies/search", `{
		"title": "The Matrix",
		"year": 1999
	}`)
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

func TestSearchTmdbMovies_HTTPByID(t *testing.T) {
	app := setupTestApp(t)

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

	actor := createTestUser(t, app, "Actor", "actor@example.com", true)
	router := authenticatedRouter(t, app, actor.ID)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/tmdb/movies/search", strings.NewReader(`{"tmdb_id":603}`))
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

	// An id TMDB does not know matches nothing, like a title that finds
	// nothing, rather than failing the search.
	unknown := newOpenAPIJSONRequest(http.MethodPost, "/api/tmdb/movies/search", `{"tmdb_id":604}`)
	w = serveOpenAPIExchange(t, router, "searchTmdbMovies", unknown, http.StatusOK)
	if !strings.Contains(w.Body.String(), `"results":[]`) {
		t.Fatalf("unknown tmdb_id body = %s, want empty results", w.Body.String())
	}
}

func TestTmdbHandlers_HTTPUnavailable(t *testing.T) {
	app := setupSessionTestApp(t)
	admin := createTestUser(t, app, "Admin", "unavailable@example.com", true)
	handler := authenticatedRouter(t, app, admin.ID)
	for _, tc := range []struct{ method, path, operation string }{
		{http.MethodPost, "/api/tmdb/movies/search", "searchTmdbMovies"},
		{http.MethodGet, "/api/tmdb/movies/in-theaters", "getMoviesInTheaters"},
		{http.MethodGet, "/api/tmdb/movies/603", "getMovieByTmdbID"},
	} {
		t.Run(tc.operation, func(t *testing.T) {
			request := httptest.NewRequest(tc.method, tc.path, nil)
			if tc.method == http.MethodPost {
				request = newOpenAPIJSONRequest(tc.method, tc.path, `{"title":"Arrival"}`)
			}
			response := httptest.NewRecorder()
			handler.ServeHTTP(response, request)
			if response.Code != http.StatusServiceUnavailable {
				t.Fatalf("status = %d, want 503: %s", response.Code, response.Body.String())
			}
			assertOpenAPIExchange(t, tc.operation, request, response)
		})
	}
}

func TestGetMovieByTmdbID_HTTP(t *testing.T) {
	app := setupTestApp(t)

	app.Tmdb = &stubTmdbClient{
		detailMovies: map[int]tmdb.TmdbMovie{
			603: {TmdbID: 603, Title: "The Matrix"},
		},
	}

	actor := createTestUser(t, app, "Actor", "actor@example.com", false)
	router := authenticatedRouter(t, app, actor.ID)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/tmdb/movies/603", nil)
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

	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/api/tmdb/movies/604", nil)
	router.ServeHTTP(w, req)
	if w.Code != http.StatusNotFound {
		t.Fatalf("unknown id status = %d, want 404; body = %s", w.Code, w.Body.String())
	}
	assertOpenAPIExchange(t, "getMovieByTmdbID", req, w)
}

func TestGetTmdbStatus_HTTP(t *testing.T) {
	app := setupTestApp(t)

	actor := createTestUser(t, app, "Actor", "actor@example.com", false)
	router := authenticatedRouter(t, app, actor.ID)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/tmdb/status", nil)
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
			// The handler runs on the server's goroutine, where t.Fatalf
			// cannot stop the test.
			t.Errorf("upstream path = %q, want /w500/poster.jpg", r.URL.Path)
			http.NotFound(w, r)
			return
		}

		w.Header().Set("Content-Type", "image/jpeg")
		w.Header().Set("Cache-Control", "public, max-age=86400")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte{1, 2, 3, 4})
	}))
	defer upstream.Close()

	app := setupTestApp(t)
	app.TmdbImageBaseURL = upstream.URL
	app.TmdbImageHTTPClient = upstream.Client()

	actor := createTestUser(t, app, "Actor", "actor@example.com", false)
	router := authenticatedRouter(t, app, actor.ID)

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

	actor := createTestUser(t, app, "Actor", "actor@example.com", false)
	router := authenticatedRouter(t, app, actor.ID)

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
		app.TmdbImageBaseURL = upstream.URL
		app.TmdbImageHTTPClient = upstream.Client()

		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/api/tmdb/images/w500/missing.jpg", nil)
		actor := createTestUser(t, app, "Actor", "actor@example.com", false)
		router := authenticatedRouter(t, app, actor.ID)
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
		app.TmdbImageBaseURL = "https://image.tmdb.org/t/p"
		app.TmdbImageHTTPClient = &http.Client{Transport: failingRoundTripper{}}

		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/api/tmdb/images/w500/poster.jpg", nil)
		actor := createTestUser(t, app, "Actor", "actor@example.com", false)
		router := authenticatedRouter(t, app, actor.ID)
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
	actor := createTestUser(t, app, "Actor", "actor@example.com", false)
	authenticatedRouter(t, app, actor.ID).ServeHTTP(w, req)

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
}

// TMDB listing nothing in theaters is an empty list, not a failure.
func TestGetMoviesInTheaters_HTTPEmptyList(t *testing.T) {
	app := setupTestApp(t)
	app.Tmdb = &stubTmdbClient{theaterMovies: []*tmdb.TmdbMovie{}}
	actor := createTestUser(t, app, "Actor", "actor@example.com", false)

	req := httptest.NewRequest(http.MethodGet, "/api/tmdb/movies/in-theaters", nil)
	w := serveOpenAPIExchange(t, authenticatedRouter(t, app, actor.ID), "getMoviesInTheaters", req, http.StatusOK)
	if !strings.Contains(w.Body.String(), `"movies":[]`) {
		t.Fatalf("body = %s, want an empty movies list", w.Body.String())
	}
}

func TestGetMoviesInTheaters_HTTPError(t *testing.T) {
	app := setupTestApp(t)

	app.Tmdb = &stubTmdbClient{theatersErr: errors.New("tmdb unavailable")}

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/tmdb/movies/in-theaters", nil)
	actor := createTestUser(t, app, "Actor", "actor@example.com", false)
	authenticatedRouter(t, app, actor.ID).ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500", w.Code)
	}
}

// Ranking is asserted through the route in TestSearchTmdbMovies_HTTPSearchRanksResults;
// the client's no-results sentinel must surface as an empty list, not an error.
func TestSearchTmdbMovies_NoResultsSentinelIsEmpty(t *testing.T) {
	app := setupTestApp(t)
	app.Tmdb = &stubTmdbClient{searchErr: fmt.Errorf("wrapped: %w", tmdb.ErrNoMoviesFound)}
	results, err := app.searchTmdbMovies(context.Background(), tmdbSearchRequest{Title: "Arrival"})
	if err != nil || len(results) != 0 {
		t.Fatalf("no-result sentinel=%+v %v", results, err)
	}
}

func (*stubTmdbClient) SearchShowsByTitleAndYear(context.Context, string, ...int) ([]tmdb.TVShow, error) {
	return nil, tmdb.ErrNoShowsFound
}
func (*stubTmdbClient) GetShowDetails(context.Context, int) (*tmdb.TVShow, error) {
	return nil, tmdb.ErrNoShowsFound
}
func (*stubTmdbClient) GetSeasonDetails(context.Context, int, int) (*tmdb.TVSeason, error) {
	return nil, tmdb.ErrNoShowsFound
}

func (s *stubTmdbClient) GetPersonDetails(_ context.Context, _ int) (*tmdb.Person, error) {
	return s.person, s.personErr
}
