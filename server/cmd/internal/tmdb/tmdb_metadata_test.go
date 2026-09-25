package tmdb

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestGetTmdbMovieByID_RequiresTmdbID(t *testing.T) {
	var attempts atomic.Int32
	client := newServerClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts.Add(1)
	}))

	err := client.GetTmdbMovieByID(context.Background(), &TmdbMovie{})
	if err == nil {
		t.Fatal("expected missing tmdb id to return error")
	}
	if attempts.Load() != 0 {
		t.Fatalf("expected no HTTP requests, got %d", attempts.Load())
	}
}

func TestGetTmdbMovieByID_Retries(t *testing.T) {
	const success = `{"id":603,"title":"The Matrix"}`

	rateLimited := func(w http.ResponseWriter) {
		w.Header().Set("Retry-After", "0")
		w.WriteHeader(http.StatusTooManyRequests)
		_, _ = w.Write([]byte(`{"status_message":"rate limit"}`))
	}

	tests := []struct {
		name         string
		failures     int32
		fail         func(w http.ResponseWriter)
		wantAttempts int32
		wantErr      string
	}{
		{
			name:         "rate limit twice then success",
			failures:     2,
			fail:         rateLimited,
			wantAttempts: 3,
		},
		{
			name:     "server error then success",
			failures: 1,
			fail: func(w http.ResponseWriter) {
				w.WriteHeader(http.StatusInternalServerError)
			},
			wantAttempts: 2,
		},
		{
			name:     "closed connection then success",
			failures: 1,
			fail: func(w http.ResponseWriter) {
				hijacker, ok := w.(http.Hijacker)
				if !ok {
					panic("test server does not support hijacking")
				}
				conn, _, err := hijacker.Hijack()
				if err == nil {
					conn.Close()
				}
			},
			wantAttempts: 2,
		},
		{
			name:     "truncated body then success",
			failures: 1,
			fail: func(w http.ResponseWriter) {
				w.Header().Set("Content-Length", "100")
				_, _ = w.Write([]byte(`{"id":603`))
			},
			wantAttempts: 2,
		},
		{
			name:         "rate limit exhausts retries",
			failures:     3,
			fail:         rateLimited,
			wantAttempts: 3,
			wantErr:      "tmdb status 429: rate limit exceeded for tmdb",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var attempts atomic.Int32
			client := newServerClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				attempt := attempts.Add(1)
				if attempt <= tc.failures {
					tc.fail(w)
					return
				}
				_, _ = w.Write([]byte(success))
			}))

			movie := &TmdbMovie{TmdbID: 603}
			err := client.GetTmdbMovieByID(context.Background(), movie)
			if attempts.Load() != tc.wantAttempts {
				t.Fatalf("attempts = %d, want %d", attempts.Load(), tc.wantAttempts)
			}
			if tc.wantErr != "" {
				if err == nil || err.Error() != tc.wantErr {
					t.Fatalf("error = %v, want %q", err, tc.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("GetTmdbMovieByID returned error: %v", err)
			}
			if movie.Title != "The Matrix" {
				t.Fatalf("title = %q, want The Matrix after retry", movie.Title)
			}
		})
	}
}

func TestGetTmdbMovieByID_ContextExpiresDuringBackoff(t *testing.T) {
	client := newServerClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Retry-After", "2")
		w.WriteHeader(http.StatusTooManyRequests)
		_, _ = w.Write([]byte(`{"status_message":"rate limit"}`))
	}))

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()

	err := client.GetTmdbMovieByID(ctx, &TmdbMovie{TmdbID: 603})
	if err == nil {
		t.Fatal("expected context deadline error, got nil")
	}
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("expected context.DeadlineExceeded, got %v", err)
	}
}

func TestGetTmdbMovieByID_TimeoutReturnsError(t *testing.T) {
	client := newServerClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(50 * time.Millisecond)
		_, _ = w.Write([]byte(`{"id":603,"title":"The Matrix"}`))
	}))

	client.httpClient = &http.Client{Timeout: 10 * time.Millisecond}
	client.maxRetries = 1

	err := client.GetTmdbMovieByID(context.Background(), &TmdbMovie{TmdbID: 603})
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("expected context.DeadlineExceeded from the client timeout, got %v", err)
	}
}

func TestGetTmdbMovieByID_RespectsCanceledContext(t *testing.T) {
	var attempts atomic.Int32
	client := newServerClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts.Add(1)
	}))

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := client.GetTmdbMovieByID(ctx, &TmdbMovie{TmdbID: 603})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context.Canceled, got %v", err)
	}
	if attempts.Load() != 0 {
		t.Fatalf("expected no HTTP requests for a canceled context, got %d", attempts.Load())
	}
}

func TestGetTmdbMovieByID_UsesCacheAndClearCache(t *testing.T) {
	var attempts atomic.Int32
	var capture queryCapture
	client := newServerClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempt := attempts.Add(1)
		capture.record(r)
		fmt.Fprintf(w, `{"id":603,"title":"The Matrix %d"}`, attempt)
	}))

	first := &TmdbMovie{TmdbID: 603}
	err := client.GetTmdbMovieByID(context.Background(), first)
	if err != nil {
		t.Fatalf("first GetTmdbMovieByID: %v", err)
	}
	second := &TmdbMovie{TmdbID: 603}
	err = client.GetTmdbMovieByID(context.Background(), second)
	if err != nil {
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
	err = client.GetTmdbMovieByID(context.Background(), third)
	if err != nil {
		t.Fatalf("post-clear GetTmdbMovieByID: %v", err)
	}
	if attempts.Load() != 2 {
		t.Fatalf("expected cache clear to force HTTP, got %d attempts", attempts.Load())
	}

	for i, query := range capture.all() {
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
	client := newServerClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts.Add(1)
		_, _ = w.Write([]byte(`{"id":603,"title":"The Matrix"}`))
	}))

	first := &TmdbMovie{TmdbID: 603}
	err := client.GetTmdbMovieByID(context.Background(), first)
	if err != nil {
		t.Fatalf("first GetTmdbMovieByID: %v", err)
	}
	first.Title = "mutated by caller"

	second := &TmdbMovie{TmdbID: 603}
	err = client.GetTmdbMovieByID(context.Background(), second)
	if err != nil {
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
	client := newServerClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"status_message":"not found"}`))
	}))

	err := client.GetTmdbMovieByID(context.Background(), &TmdbMovie{TmdbID: 603})
	if err == nil {
		t.Fatal("expected non-OK response to return error")
	}
	if err.Error() != "tmdb status 404: unable to get movie from tmdb" {
		t.Fatalf("error = %q, want tmdb status 404: unable to get movie from tmdb", err.Error())
	}
}

func TestGetTmdbMovieByID_MalformedJSONReturnsError(t *testing.T) {
	client := newServerClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"id":`))
	}))

	err := client.GetTmdbMovieByID(context.Background(), &TmdbMovie{TmdbID: 603})
	if err == nil {
		t.Fatal("expected malformed JSON to return error")
	}
}

func TestSearchMoviesByTitleAndYear_RetriesWithoutYearWhenEmpty(t *testing.T) {
	var capture queryCapture
	client := newServerClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capture.record(r)
		if r.URL.Query().Get("year") == strconv.Itoa(yearWithNoReleases) {
			_, _ = w.Write([]byte(`{"results":[]}`))
			return
		}
		_, _ = w.Write([]byte(`{"results":[{"id":603,"title":"The Matrix","release_date":"1999-03-31"}]}`))
	}))

	results, err := client.SearchMoviesByTitleAndYear(context.Background(), "The Matrix", yearWithNoReleases)
	if err != nil {
		t.Fatalf("SearchMoviesByTitleAndYear returned error: %v", err)
	}
	if len(results) != 1 || results[0].TmdbID != 603 {
		t.Fatalf("results = %+v, want The Matrix fallback result", results)
	}

	queries := capture.all()
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
	if queries[0].Get("year") != strconv.Itoa(yearWithNoReleases) {
		t.Fatalf("first request year = %q, want %d", queries[0].Get("year"), yearWithNoReleases)
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
	client := newServerClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"results":[]}`))
	}))

	_, err := client.SearchMoviesByTitleAndYear(context.Background(), "No Results")
	if !errors.Is(err, ErrNoMoviesFound) {
		t.Fatalf("expected ErrNoMoviesFound, got %v", err)
	}
}

func TestSearchMoviesByTitleAndYear_MalformedJSONReturnsError(t *testing.T) {
	client := newServerClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"results":`))
	}))

	_, err := client.SearchMoviesByTitleAndYear(context.Background(), "The Matrix")
	if err == nil {
		t.Fatal("expected malformed JSON to return error")
	}
}

func TestGetMoviesInTheaters(t *testing.T) {
	var capture queryCapture
	client := newServerClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capture.record(r)
		_, _ = w.Write([]byte(`{"results":[{"id":1,"title":"One"},{"id":2,"title":"Two"}]}`))
	}))

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

	queries := capture.all()
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
	client := newServerClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"results":[]}`))
	}))

	_, err := client.GetMoviesInTheaters(context.Background())
	if err == nil {
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
	got := movie.Certification()
	if got != "R" {
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
	got = movie.Certification()
	if got != "15" {
		t.Fatalf("fallback Certification() = %q, want 15", got)
	}
}

func TestRetryDelayHonorsRetryAfterAndCaps(t *testing.T) {
	client := &tmdbClient{retryBaseDelay: 100 * time.Millisecond}

	headers := http.Header{}
	headers.Set("Retry-After", "1")
	got := client.retryDelay(headers, 0)
	if got != time.Second {
		t.Fatalf("Retry-After seconds delay = %s, want 1s", got)
	}

	headers.Set("Retry-After", "60")
	got = client.retryDelay(headers, 0)
	if got != tmdbHTTPRetryMaxDelay {
		t.Fatalf("capped Retry-After delay = %s, want %s", got, tmdbHTTPRetryMaxDelay)
	}

	headers.Set("Retry-After", time.Now().Add(time.Minute).UTC().Format(http.TimeFormat))
	got = client.retryDelay(headers, 0)
	if got != tmdbHTTPRetryMaxDelay {
		t.Fatalf("capped Retry-After date delay = %s, want %s", got, tmdbHTTPRetryMaxDelay)
	}

	headers.Set("Retry-After", time.Now().Add(-time.Minute).UTC().Format(http.TimeFormat))
	got = client.retryDelay(headers, 0)
	if got != 0 {
		t.Fatalf("past Retry-After date delay = %s, want 0", got)
	}

	headers.Set("Retry-After", "not-a-date")
	got = client.retryDelay(headers, 2)
	if got != 400*time.Millisecond {
		t.Fatalf("exponential delay = %s, want 400ms", got)
	}
	got = client.retryDelay(headers, 5)
	if got != tmdbHTTPRetryMaxDelay {
		t.Fatalf("capped exponential delay = %s, want %s", got, tmdbHTTPRetryMaxDelay)
	}

	zero := &tmdbClient{}
	got = zero.retryDelay(nil, 0)
	if got != tmdbHTTPRetryBaseDelay {
		t.Fatalf("zero-value base delay = %s, want %s", got, tmdbHTTPRetryBaseDelay)
	}
}

func TestProviderStatusClassificationThroughHTTP(t *testing.T) {
	for _, code := range []int{401, 403, 404, 429, 503} {
		t.Run(fmt.Sprint(code), func(t *testing.T) {
			client := newServerClient(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(code) }))
			_, err := client.SearchMoviesByTitleAndYear(context.Background(), "movie")
			authentication, transient := ProviderFailure(err)
			if authentication != (code == 401 || code == 403) || transient != (code == 429 || code == 503) {
				t.Fatalf("code=%d auth=%v transient=%v err=%v", code, authentication, transient, err)
			}
		})
	}
}

// A definitive no-match from either catalog is not a provider failure, so it
// never counts toward a scanner's circuit breaker.
func TestProviderFailureIgnoresNoMatchSentinels(t *testing.T) {
	for _, err := range []error{ErrNoMoviesFound, ErrNoShowsFound, nil} {
		authentication, transient := ProviderFailure(err)
		if authentication || transient {
			t.Fatalf("%v classified as a provider failure", err)
		}
	}
}

// A transport failure carries no status, so it is transient but never an
// authentication failure.
func TestProviderFailureTreatsTransportErrorsAsTransient(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	server.Close()

	_, err := newTestClient(server.URL).SearchMoviesByTitleAndYear(context.Background(), "movie")
	if err == nil {
		t.Fatal("expected a transport error from the closed server")
	}
	authentication, transient := ProviderFailure(err)
	if authentication || !transient {
		t.Fatalf("transport error classified auth=%v transient=%v: %v", authentication, transient, err)
	}
}

// The client retries after a transport failure, so the handler can run more
// than once; the sync.Once guards keep a retry from closing a closed channel.
func TestCancelBlockedMovieRequest(t *testing.T) {
	entered := make(chan struct{})
	stopped := make(chan struct{})
	var enteredOnce, stoppedOnce sync.Once
	client := newServerClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		enteredOnce.Do(func() { close(entered) })
		<-r.Context().Done()
		stoppedOnce.Do(func() { close(stopped) })
	}))
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	result := make(chan error, 1)
	go func() { _, err := client.SearchMoviesByTitleAndYear(ctx, "movie"); result <- err }()
	select {
	case <-entered:
	case <-time.After(time.Second):
		t.Fatal("request never reached the server")
	}
	cancel()
	select {
	case err := <-result:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("blocked request cancellation: %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("request did not cancel")
	}
	select {
	case <-stopped:
	case <-time.After(time.Second):
		t.Fatal("upstream request remained active")
	}
}
