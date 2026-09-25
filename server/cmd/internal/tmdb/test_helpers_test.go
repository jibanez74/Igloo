package tmdb

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"sync"
	"testing"
	"time"

	cache "github.com/patrickmn/go-cache"
)

// yearWithNoReleases is a year no catalog entry can carry, so a search
// filtered by it must fall back to an unfiltered search. The live tests use
// the same value against the real API.
const yearWithNoReleases = 1850

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

// newServerClient serves handler for the rest of the test and returns a
// client pointed at it.
func newServerClient(t *testing.T, handler http.Handler) *tmdbClient {
	t.Helper()
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)
	return newTestClient(server.URL)
}

// queryCapture records every request's query string plus its path under the
// "path" key, so a test can assert both after the calls complete.
type queryCapture struct {
	mu      sync.Mutex
	queries []url.Values
}

func (c *queryCapture) record(r *http.Request) {
	query := r.URL.Query()
	query.Set("path", r.URL.Path)
	c.mu.Lock()
	c.queries = append(c.queries, query)
	c.mu.Unlock()
}

func (c *queryCapture) all() []url.Values {
	c.mu.Lock()
	defer c.mu.Unlock()
	return append([]url.Values(nil), c.queries...)
}
