package tmdb

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"igloo/cmd/internal/helpers"

	cache "github.com/patrickmn/go-cache"
)

func TestNew(t *testing.T) {
	t.Run("returns error when API key is empty", func(t *testing.T) {
		_, err := New("")
		if err == nil {
			t.Error("Expected error when API key is empty")
		}
	})

	t.Run("returns client when API key is provided", func(t *testing.T) {
		client, err := New("test-api-key")
		if err != nil {
			t.Errorf("Unexpected error: %v", err)
		}
		if client == nil {
			t.Error("Expected client to be non-nil")
		}
	})
}

func TestGetTmdbMovieByID_RetriesRateLimitAndSucceeds(t *testing.T) {
	var attempts atomic.Int32

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		count := attempts.Add(1)
		if count < 3 {
			w.Header().Set("Retry-After", "0")
			w.WriteHeader(http.StatusTooManyRequests)
			_, _ = w.Write([]byte(`{"status_message":"rate limit"}`))
			return
		}

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":603,"title":"The Matrix","release_date":"1999-03-30"}`))
	}))
	defer server.Close()

	client := &tmdbClient{
		key:            "test-api-key",
		baseURL:        server.URL,
		httpClient:     &http.Client{Timeout: 200 * time.Millisecond},
		maxRetries:     3,
		retryBaseDelay: time.Millisecond,
		movieCache:     cache.New(tmdbMovieCacheTTL, tmdbMovieCacheCleanup),
	}

	movie := &TmdbMovie{TmdbID: 603}
	err := client.GetTmdbMovieByID(context.Background(), movie)
	if err != nil {
		t.Fatalf("GetTmdbMovieByID returned error: %v", err)
	}

	if movie.Title != "The Matrix" {
		t.Fatalf("expected title to be populated after retry, got %q", movie.Title)
	}
	if attempts.Load() != 3 {
		t.Fatalf("expected 3 attempts, got %d", attempts.Load())
	}
}

func TestGetTmdbMovieByID_TimeoutReturnsError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(50 * time.Millisecond)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":603,"title":"The Matrix"}`))
	}))
	defer server.Close()

	client := &tmdbClient{
		key:            "test-api-key",
		baseURL:        server.URL,
		httpClient:     &http.Client{Timeout: 10 * time.Millisecond},
		maxRetries:     1,
		retryBaseDelay: time.Millisecond,
		movieCache:     cache.New(tmdbMovieCacheTTL, tmdbMovieCacheCleanup),
	}

	movie := &TmdbMovie{TmdbID: 603}
	err := client.GetTmdbMovieByID(context.Background(), movie)
	if err == nil {
		t.Fatal("expected timeout error, got nil")
	}
}

func TestGetTmdbMovieByID_RespectsCanceledContext(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(100 * time.Millisecond)
		w.WriteHeader(http.StatusTooManyRequests)
		_, _ = w.Write([]byte(`{"status_message":"rate limit"}`))
	}))
	defer server.Close()

	client := &tmdbClient{
		key:            "test-api-key",
		baseURL:        server.URL,
		httpClient:     &http.Client{Timeout: 500 * time.Millisecond},
		maxRetries:     3,
		retryBaseDelay: 50 * time.Millisecond,
		movieCache:     cache.New(tmdbMovieCacheTTL, tmdbMovieCacheCleanup),
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	movie := &TmdbMovie{TmdbID: 603}
	err := client.GetTmdbMovieByID(ctx, movie)
	if err == nil {
		t.Fatal("expected canceled context error, got nil")
	}
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context.Canceled, got %v", err)
	}
}

func TestNew_ConfiguresSharedHTTPClient(t *testing.T) {
	client, err := New("test-api-key")
	if err != nil {
		t.Fatalf("New returned error: %v", err)
	}

	concrete, ok := client.(*tmdbClient)
	if !ok {
		t.Fatal("expected concrete tmdbClient")
	}
	if concrete.httpClient == nil {
		t.Fatal("expected shared http client to be configured")
	}
	if concrete.httpClient.Timeout != helpers.TMDB_HTTP_TIMEOUT {
		t.Fatalf("expected timeout %s, got %s", helpers.TMDB_HTTP_TIMEOUT, concrete.httpClient.Timeout)
	}
	if concrete.maxRetries != helpers.TMDB_HTTP_MAX_RETRIES {
		t.Fatalf("expected max retries %d, got %d", helpers.TMDB_HTTP_MAX_RETRIES, concrete.maxRetries)
	}
}

func TestGetTmdbMovieByID_UsesCacheAndClearCache(t *testing.T) {
	var attempts atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempt := attempts.Add(1)
		if r.URL.Path != "/movie/603" {
			t.Fatalf("path = %q, want /movie/603", r.URL.Path)
		}
		if r.URL.Query().Get("append_to_response") != "credits,videos,release_dates" {
			t.Fatalf("append_to_response = %q", r.URL.Query().Get("append_to_response"))
		}

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":603,"title":"The Matrix ` + string(rune('0'+attempt)) + `"}`))
	}))
	defer server.Close()

	client := &tmdbClient{
		key:            "test-api-key",
		baseURL:        server.URL,
		httpClient:     server.Client(),
		maxRetries:     1,
		retryBaseDelay: time.Millisecond,
		movieCache:     cache.New(tmdbMovieCacheTTL, tmdbMovieCacheCleanup),
	}

	first := &TmdbMovie{TmdbID: 603}
	if err := client.GetTmdbMovieByID(context.Background(), first); err != nil {
		t.Fatalf("first GetTmdbMovieByID: %v", err)
	}
	second := &TmdbMovie{TmdbID: 603}
	if err := client.GetTmdbMovieByID(context.Background(), second); err != nil {
		t.Fatalf("cached GetTmdbMovieByID: %v", err)
	}
	if attempts.Load() != 1 {
		t.Fatalf("expected cached call to avoid HTTP, got %d attempts", attempts.Load())
	}
	if first.Title != second.Title {
		t.Fatalf("cached title = %q, want %q", second.Title, first.Title)
	}

	client.ClearCache()
	third := &TmdbMovie{TmdbID: 603}
	if err := client.GetTmdbMovieByID(context.Background(), third); err != nil {
		t.Fatalf("post-clear GetTmdbMovieByID: %v", err)
	}
	if attempts.Load() != 2 {
		t.Fatalf("expected cache clear to force HTTP, got %d attempts", attempts.Load())
	}
}

func TestGetTmdbMovieByID_NonOKReturnsError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"status_message":"not found"}`))
	}))
	defer server.Close()

	client := &tmdbClient{
		key:            "test-api-key",
		baseURL:        server.URL,
		httpClient:     server.Client(),
		maxRetries:     1,
		retryBaseDelay: time.Millisecond,
		movieCache:     cache.New(tmdbMovieCacheTTL, tmdbMovieCacheCleanup),
	}

	err := client.GetTmdbMovieByID(context.Background(), &TmdbMovie{TmdbID: 603})
	if err == nil {
		t.Fatal("expected non-OK response to return error")
	}
	if err.Error() != "unable to get movie from tmdb" {
		t.Fatalf("error = %q, want unable to get movie from tmdb", err.Error())
	}
}

func TestSearchMoviesByTitleAndYear_RetriesWithoutYearWhenEmpty(t *testing.T) {
	var requests []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests = append(requests, r.URL.RawQuery)
		if r.URL.Path != "/search/movie" {
			t.Fatalf("path = %q, want /search/movie", r.URL.Path)
		}
		if r.URL.Query().Get("query") != "The Matrix" {
			t.Fatalf("query = %q, want The Matrix", r.URL.Query().Get("query"))
		}
		if r.URL.Query().Get("include_adult") != "false" {
			t.Fatalf("include_adult = %q, want false", r.URL.Query().Get("include_adult"))
		}

		w.Header().Set("Content-Type", "application/json")
		if r.URL.Query().Get("year") == "1850" {
			_, _ = w.Write([]byte(`{"results":[]}`))
			return
		}
		_, _ = w.Write([]byte(`{"results":[{"id":603,"title":"The Matrix","release_date":"1999-03-31"}]}`))
	}))
	defer server.Close()

	client := &tmdbClient{
		key:            "test-api-key",
		baseURL:        server.URL,
		httpClient:     server.Client(),
		maxRetries:     1,
		retryBaseDelay: time.Millisecond,
		movieCache:     cache.New(tmdbMovieCacheTTL, tmdbMovieCacheCleanup),
	}

	results, err := client.SearchMoviesByTitleAndYear(context.Background(), "The Matrix", 1850)
	if err != nil {
		t.Fatalf("SearchMoviesByTitleAndYear returned error: %v", err)
	}
	if len(results) != 1 || results[0].TmdbID != 603 {
		t.Fatalf("results = %+v, want The Matrix fallback result", results)
	}
	if len(requests) != 2 {
		t.Fatalf("request count = %d, want 2", len(requests))
	}
}

func TestSearchMoviesByTitleAndYear_RejectsEmptyAndNoResults(t *testing.T) {
	client := &tmdbClient{
		key:            "test-api-key",
		baseURL:        "http://127.0.0.1",
		httpClient:     &http.Client{Timeout: time.Millisecond},
		maxRetries:     1,
		retryBaseDelay: time.Millisecond,
		movieCache:     cache.New(tmdbMovieCacheTTL, tmdbMovieCacheCleanup),
	}

	if _, err := client.SearchMoviesByTitleAndYear(context.Background(), ""); err == nil {
		t.Fatal("expected empty title to return error")
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"results":[]}`))
	}))
	defer server.Close()
	client.baseURL = server.URL
	client.httpClient = server.Client()

	if _, err := client.SearchMoviesByTitleAndYear(context.Background(), "No Results"); err == nil {
		t.Fatal("expected empty results to return error")
	}
}

func TestGetMoviesInTheaters(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/movie/now_playing" {
			t.Fatalf("path = %q, want /movie/now_playing", r.URL.Path)
		}
		if r.URL.Query().Get("region") != "US" || r.URL.Query().Get("language") != "en-US" || r.URL.Query().Get("page") != "1" {
			t.Fatalf("unexpected query: %s", r.URL.RawQuery)
		}

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"results":[{"id":1,"title":"One"},{"id":2,"title":"Two"}]}`))
	}))
	defer server.Close()

	client := &tmdbClient{
		key:            "test-api-key",
		baseURL:        server.URL,
		httpClient:     server.Client(),
		maxRetries:     1,
		retryBaseDelay: time.Millisecond,
		movieCache:     cache.New(tmdbMovieCacheTTL, tmdbMovieCacheCleanup),
	}

	movies, err := client.GetMoviesInTheaters(context.Background())
	if err != nil {
		t.Fatalf("GetMoviesInTheaters returned error: %v", err)
	}
	if len(movies) != 2 || movies[0].TmdbID != 1 || movies[1].TmdbID != 2 {
		t.Fatalf("movies = %+v, want two now-playing movies", movies)
	}
	movies[0].Title = "Changed"
	if movies[1].Title == "Changed" {
		t.Fatal("movie pointers should point to distinct slice elements")
	}
}

func TestGetMoviesInTheaters_EmptyResultsReturnsError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"results":[]}`))
	}))
	defer server.Close()

	client := &tmdbClient{
		key:            "test-api-key",
		baseURL:        server.URL,
		httpClient:     server.Client(),
		maxRetries:     1,
		retryBaseDelay: time.Millisecond,
		movieCache:     cache.New(tmdbMovieCacheTTL, tmdbMovieCacheCleanup),
	}

	if _, err := client.GetMoviesInTheaters(context.Background()); err == nil {
		t.Fatal("expected empty now-playing results to return error")
	}
}

func TestTmdbMovieCertificationPrefersUSAndFallsBack(t *testing.T) {
	var movie TmdbMovie
	err := json.Unmarshal([]byte(`{
		"release_dates": {
			"results": [
				{"iso_3166_1": "GB", "release_dates": [{"certification": "15"}]},
				{"iso_3166_1": "US", "release_dates": [{"certification": ""}, {"certification": "R"}]}
			]
		}
	}`), &movie)
	if err != nil {
		t.Fatalf("unmarshal certification fixture: %v", err)
	}
	if got := movie.Certification(); got != "R" {
		t.Fatalf("Certification() = %q, want R", got)
	}

	err = json.Unmarshal([]byte(`{
		"release_dates": {
			"results": [
				{"iso_3166_1": "GB", "release_dates": [{"certification": "15"}]}
			]
		}
	}`), &movie)
	if err != nil {
		t.Fatalf("unmarshal fallback certification fixture: %v", err)
	}
	if got := movie.Certification(); got != "15" {
		t.Fatalf("fallback Certification() = %q, want 15", got)
	}
}

func TestNewTmdbMovieSearchResult(t *testing.T) {
	if got := NewTmdbMovieSearchResult(nil); got != (TmdbMovieSearchResult{}) {
		t.Fatalf("nil mapping = %+v, want zero value", got)
	}

	got := NewTmdbMovieSearchResult(&TmdbMovie{
		TmdbID:      603,
		Title:       "The Matrix",
		ReleaseDate: "1999-03-31",
		Overview:    "Overview",
		PosterPath:  "/poster.jpg",
	})
	if got.TmdbID != 603 || got.Title != "The Matrix" || got.ReleaseDate != "1999-03-31" ||
		got.Overview != "Overview" || got.PosterPath != "/poster.jpg" {
		t.Fatalf("mapped result = %+v", got)
	}
}

func TestRetryDelayHonorsRetryAfterAndCaps(t *testing.T) {
	client := &tmdbClient{retryBaseDelay: 100 * time.Millisecond}

	headers := http.Header{}
	headers.Set("Retry-After", "1")
	if got := client.retryDelay(headers, 0); got != time.Second {
		t.Fatalf("Retry-After seconds delay = %s, want 1s", got)
	}

	headers.Set("Retry-After", "60")
	if got := client.retryDelay(headers, 0); got != helpers.TMDB_HTTP_RETRY_MAX_DELAY {
		t.Fatalf("capped Retry-After delay = %s, want %s", got, helpers.TMDB_HTTP_RETRY_MAX_DELAY)
	}

	headers.Set("Retry-After", "not-a-date")
	if got := client.retryDelay(headers, 2); got != 400*time.Millisecond {
		t.Fatalf("exponential delay = %s, want 400ms", got)
	}
}
