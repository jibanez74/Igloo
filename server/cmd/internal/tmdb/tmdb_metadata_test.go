package tmdb

import (
	"context"
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
