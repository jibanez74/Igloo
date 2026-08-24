package tmdb

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"igloo/cmd/internal/helpers"

	cache "github.com/patrickmn/go-cache"
)

func newTestClient(baseURL string) *tmdbClient {
	return &tmdbClient{
		key:            "test-api-key",
		baseURL:        baseURL,
		httpClient:     &http.Client{Timeout: time.Second},
		maxRetries:     3,
		retryBaseDelay: time.Millisecond,
		movieCache:     cache.New(tmdbMovieCacheTTL, tmdbMovieCacheCleanup),
	}
}

func TestNew(t *testing.T) {
	_, err := New("")
	if err == nil {
		t.Fatal("expected error when API key is empty")
	}

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
	if concrete.maxRetries != tmdbHTTPMaxRetries {
		t.Fatalf("expected max retries %d, got %d", tmdbHTTPMaxRetries, concrete.maxRetries)
	}
}

func TestGetTmdbMovieByID_RequiresTmdbID(t *testing.T) {
	var attempts atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts.Add(1)
	}))
	defer server.Close()

	err := newTestClient(server.URL).GetTmdbMovieByID(context.Background(), &TmdbMovie{})
	if err == nil {
		t.Fatal("expected missing tmdb id to return error")
	}
	if attempts.Load() != 0 {
		t.Fatalf("expected no HTTP requests, got %d", attempts.Load())
	}
}

func TestGetTmdbMovieByID_RetriesRateLimitAndSucceeds(t *testing.T) {
	var attempts atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if attempts.Add(1) < 3 {
			w.Header().Set("Retry-After", "0")
			w.WriteHeader(http.StatusTooManyRequests)
			_, _ = w.Write([]byte(`{"status_message":"rate limit"}`))
			return
		}

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":603,"title":"The Matrix","release_date":"1999-03-30"}`))
	}))
	defer server.Close()

	movie := &TmdbMovie{TmdbID: 603}
	err := newTestClient(server.URL).GetTmdbMovieByID(context.Background(), movie)
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

func TestGetTmdbMovieByID_RateLimitExhaustsRetries(t *testing.T) {
	var attempts atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts.Add(1)
		w.Header().Set("Retry-After", "0")
		w.WriteHeader(http.StatusTooManyRequests)
		_, _ = w.Write([]byte(`{"status_message":"rate limit"}`))
	}))
	defer server.Close()

	err := newTestClient(server.URL).GetTmdbMovieByID(context.Background(), &TmdbMovie{TmdbID: 603})
	if err == nil {
		t.Fatal("expected exhausted retries to return error")
	}
	if err.Error() != "rate limit exceeded for tmdb" {
		t.Fatalf("error = %q, want rate limit exceeded for tmdb", err.Error())
	}
	if attempts.Load() != 3 {
		t.Fatalf("expected 3 attempts, got %d", attempts.Load())
	}
}

func TestGetTmdbMovieByID_RetriesServerErrorAndSucceeds(t *testing.T) {
	var attempts atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if attempts.Add(1) == 1 {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":603,"title":"The Matrix"}`))
	}))
	defer server.Close()

	movie := &TmdbMovie{TmdbID: 603}
	err := newTestClient(server.URL).GetTmdbMovieByID(context.Background(), movie)
	if err != nil {
		t.Fatalf("GetTmdbMovieByID returned error: %v", err)
	}
	if movie.Title != "The Matrix" {
		t.Fatalf("expected title after 500 retry, got %q", movie.Title)
	}
	if attempts.Load() != 2 {
		t.Fatalf("expected 2 attempts, got %d", attempts.Load())
	}
}

func TestGetTmdbMovieByID_RetriesNetworkErrorAndSucceeds(t *testing.T) {
	var attempts atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if attempts.Add(1) == 1 {
			hj, ok := w.(http.Hijacker)
			if !ok {
				panic("test server does not support hijacking")
			}
			conn, _, err := hj.Hijack()
			if err == nil {
				conn.Close()
			}
			return
		}

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":603,"title":"The Matrix"}`))
	}))
	defer server.Close()

	movie := &TmdbMovie{TmdbID: 603}
	err := newTestClient(server.URL).GetTmdbMovieByID(context.Background(), movie)
	if err != nil {
		t.Fatalf("GetTmdbMovieByID returned error: %v", err)
	}
	if movie.Title != "The Matrix" {
		t.Fatalf("expected title after network-error retry, got %q", movie.Title)
	}
	if attempts.Load() != 2 {
		t.Fatalf("expected 2 attempts, got %d", attempts.Load())
	}
}

func TestGetTmdbMovieByID_RetriesTruncatedBodyAndSucceeds(t *testing.T) {
	var attempts atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if attempts.Add(1) == 1 {
			w.Header().Set("Content-Length", "100")
			_, _ = w.Write([]byte(`{"id":603`))
			return
		}

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":603,"title":"The Matrix"}`))
	}))
	defer server.Close()

	movie := &TmdbMovie{TmdbID: 603}
	err := newTestClient(server.URL).GetTmdbMovieByID(context.Background(), movie)
	if err != nil {
		t.Fatalf("GetTmdbMovieByID returned error: %v", err)
	}
	if movie.Title != "The Matrix" {
		t.Fatalf("expected title after truncated-body retry, got %q", movie.Title)
	}
	if attempts.Load() != 2 {
		t.Fatalf("expected 2 attempts, got %d", attempts.Load())
	}
}

func TestGetTmdbMovieByID_ContextExpiresDuringBackoff(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Retry-After", "2")
		w.WriteHeader(http.StatusTooManyRequests)
		_, _ = w.Write([]byte(`{"status_message":"rate limit"}`))
	}))
	defer server.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()

	err := newTestClient(server.URL).GetTmdbMovieByID(ctx, &TmdbMovie{TmdbID: 603})
	if err == nil {
		t.Fatal("expected context deadline error, got nil")
	}
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("expected context.DeadlineExceeded, got %v", err)
	}
}

func TestGetTmdbMovieByID_TimeoutReturnsError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(50 * time.Millisecond)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":603,"title":"The Matrix"}`))
	}))
	defer server.Close()

	client := newTestClient(server.URL)
	client.httpClient = &http.Client{Timeout: 10 * time.Millisecond}
	client.maxRetries = 1

	err := client.GetTmdbMovieByID(context.Background(), &TmdbMovie{TmdbID: 603})
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

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := newTestClient(server.URL).GetTmdbMovieByID(ctx, &TmdbMovie{TmdbID: 603})
	if err == nil {
		t.Fatal("expected canceled context error, got nil")
	}
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context.Canceled, got %v", err)
	}
}

func TestGetTmdbMovieByID_UsesCacheAndClearCache(t *testing.T) {
	var (
		attempts atomic.Int32
		mu       sync.Mutex
		queries  []url.Values
	)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempt := attempts.Add(1)
		mu.Lock()
		query := r.URL.Query()
		query.Set("path", r.URL.Path)
		queries = append(queries, query)
		mu.Unlock()

		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"id":603,"title":"The Matrix %d"}`, attempt)
	}))
	defer server.Close()

	client := newTestClient(server.URL)

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

	for i, query := range queries {
		if query.Get("path") != "/movie/603" {
			t.Fatalf("request %d path = %q, want /movie/603", i, query.Get("path"))
		}
		if query.Get("append_to_response") != "credits,videos,release_dates" {
			t.Fatalf("request %d append_to_response = %q", i, query.Get("append_to_response"))
		}
	}
}

func TestGetTmdbMovieByID_CachedCopiesAreIsolated(t *testing.T) {
	var attempts atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts.Add(1)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":603,"title":"The Matrix"}`))
	}))
	defer server.Close()

	client := newTestClient(server.URL)

	first := &TmdbMovie{TmdbID: 603}
	if err := client.GetTmdbMovieByID(context.Background(), first); err != nil {
		t.Fatalf("first GetTmdbMovieByID: %v", err)
	}
	first.Title = "mutated by caller"

	second := &TmdbMovie{TmdbID: 603}
	if err := client.GetTmdbMovieByID(context.Background(), second); err != nil {
		t.Fatalf("cached GetTmdbMovieByID: %v", err)
	}
	if second.Title != "The Matrix" {
		t.Fatalf("cached title = %q, want The Matrix after caller mutation", second.Title)
	}
	if attempts.Load() != 1 {
		t.Fatalf("expected cached call to avoid HTTP, got %d attempts", attempts.Load())
	}
}

func TestGetTmdbMovieByID_NonOKReturnsError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"status_message":"not found"}`))
	}))
	defer server.Close()

	err := newTestClient(server.URL).GetTmdbMovieByID(context.Background(), &TmdbMovie{TmdbID: 603})
	if err == nil {
		t.Fatal("expected non-OK response to return error")
	}
	if err.Error() != "unable to get movie from tmdb" {
		t.Fatalf("error = %q, want unable to get movie from tmdb", err.Error())
	}
}

func TestGetTmdbMovieByID_MalformedJSONReturnsError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":`))
	}))
	defer server.Close()

	err := newTestClient(server.URL).GetTmdbMovieByID(context.Background(), &TmdbMovie{TmdbID: 603})
	if err == nil {
		t.Fatal("expected malformed JSON to return error")
	}
}

func TestSearchMoviesByTitleAndYear_RetriesWithoutYearWhenEmpty(t *testing.T) {
	var (
		mu      sync.Mutex
		queries []url.Values
	)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		query := r.URL.Query()
		query.Set("path", r.URL.Path)
		queries = append(queries, query)
		mu.Unlock()

		w.Header().Set("Content-Type", "application/json")
		if r.URL.Query().Get("year") == "1850" {
			_, _ = w.Write([]byte(`{"results":[]}`))
			return
		}
		_, _ = w.Write([]byte(`{"results":[{"id":603,"title":"The Matrix","release_date":"1999-03-31"}]}`))
	}))
	defer server.Close()

	results, err := newTestClient(server.URL).SearchMoviesByTitleAndYear(context.Background(), "The Matrix", 1850)
	if err != nil {
		t.Fatalf("SearchMoviesByTitleAndYear returned error: %v", err)
	}
	if len(results) != 1 || results[0].TmdbID != 603 {
		t.Fatalf("results = %+v, want The Matrix fallback result", results)
	}

	if len(queries) != 2 {
		t.Fatalf("request count = %d, want 2", len(queries))
	}
	for i, query := range queries {
		if query.Get("path") != "/search/movie" {
			t.Fatalf("request %d path = %q, want /search/movie", i, query.Get("path"))
		}
		if query.Get("query") != "The Matrix" {
			t.Fatalf("request %d query = %q, want The Matrix", i, query.Get("query"))
		}
		if query.Get("include_adult") != "false" {
			t.Fatalf("request %d include_adult = %q, want false", i, query.Get("include_adult"))
		}
	}
	if queries[0].Get("year") != "1850" {
		t.Fatalf("first request year = %q, want 1850", queries[0].Get("year"))
	}
	if queries[1].Has("year") {
		t.Fatalf("fallback request should drop year, got %q", queries[1].Get("year"))
	}
}

func TestSearchMoviesByTitleAndYear_RejectsEmptyTitle(t *testing.T) {
	_, err := newTestClient("").SearchMoviesByTitleAndYear(context.Background(), "")
	if err == nil {
		t.Fatal("expected empty title to return error")
	}
}

func TestSearchMoviesByTitleAndYear_NoResultsReturnsError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"results":[]}`))
	}))
	defer server.Close()

	_, err := newTestClient(server.URL).SearchMoviesByTitleAndYear(context.Background(), "No Results")
	if err == nil {
		t.Fatal("expected empty results to return error")
	}
}

func TestSearchMoviesByTitleAndYear_MalformedJSONReturnsError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"results":`))
	}))
	defer server.Close()

	_, err := newTestClient(server.URL).SearchMoviesByTitleAndYear(context.Background(), "The Matrix")
	if err == nil {
		t.Fatal("expected malformed JSON to return error")
	}
}

func TestGetMoviesInTheaters(t *testing.T) {
	var (
		mu      sync.Mutex
		queries []url.Values
	)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		query := r.URL.Query()
		query.Set("path", r.URL.Path)
		queries = append(queries, query)
		mu.Unlock()

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"results":[{"id":1,"title":"One"},{"id":2,"title":"Two"}]}`))
	}))
	defer server.Close()

	movies, err := newTestClient(server.URL).GetMoviesInTheaters(context.Background())
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

	if len(queries) != 1 {
		t.Fatalf("request count = %d, want 1", len(queries))
	}
	if queries[0].Get("path") != "/movie/now_playing" {
		t.Fatalf("path = %q, want /movie/now_playing", queries[0].Get("path"))
	}
	if queries[0].Get("region") != "US" || queries[0].Get("language") != "en-US" || queries[0].Get("page") != "1" {
		t.Fatalf("unexpected query: %v", queries[0])
	}
}

func TestGetMoviesInTheaters_EmptyResultsReturnsError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"results":[]}`))
	}))
	defer server.Close()

	if _, err := newTestClient(server.URL).GetMoviesInTheaters(context.Background()); err == nil {
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

func TestRetryDelayHonorsRetryAfterAndCaps(t *testing.T) {
	client := &tmdbClient{retryBaseDelay: 100 * time.Millisecond}

	headers := http.Header{}
	headers.Set("Retry-After", "1")
	if got := client.retryDelay(headers, 0); got != time.Second {
		t.Fatalf("Retry-After seconds delay = %s, want 1s", got)
	}

	headers.Set("Retry-After", "60")
	if got := client.retryDelay(headers, 0); got != tmdbHTTPRetryMaxDelay {
		t.Fatalf("capped Retry-After delay = %s, want %s", got, tmdbHTTPRetryMaxDelay)
	}

	headers.Set("Retry-After", time.Now().Add(time.Minute).UTC().Format(http.TimeFormat))
	if got := client.retryDelay(headers, 0); got != tmdbHTTPRetryMaxDelay {
		t.Fatalf("capped Retry-After date delay = %s, want %s", got, tmdbHTTPRetryMaxDelay)
	}

	headers.Set("Retry-After", time.Now().Add(-time.Minute).UTC().Format(http.TimeFormat))
	if got := client.retryDelay(headers, 0); got != 0 {
		t.Fatalf("past Retry-After date delay = %s, want 0", got)
	}

	headers.Set("Retry-After", "not-a-date")
	if got := client.retryDelay(headers, 2); got != 400*time.Millisecond {
		t.Fatalf("exponential delay = %s, want 400ms", got)
	}
	if got := client.retryDelay(headers, 5); got != tmdbHTTPRetryMaxDelay {
		t.Fatalf("capped exponential delay = %s, want %s", got, tmdbHTTPRetryMaxDelay)
	}

	zero := &tmdbClient{}
	if got := zero.retryDelay(nil, 0); got != tmdbHTTPRetryBaseDelay {
		t.Fatalf("zero-value base delay = %s, want %s", got, tmdbHTTPRetryBaseDelay)
	}
}
