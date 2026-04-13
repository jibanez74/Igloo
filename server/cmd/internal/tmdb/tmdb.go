package tmdb

import (
	"errors"
	"igloo/cmd/internal/helpers"
	"net/http"
	"time"

	cache "github.com/patrickmn/go-cache"
)

type TmdbInterface interface {
	GetTmdbMovieByID(movie *TmdbMovie) error
	GetTmdbMovieByTitle(movie *TmdbMovie) error
	SearchMoviesByTitleAndYear(title string, year ...int) ([]TmdbMovie, error)
	GetMoviesInTheaters() ([]*TmdbMovie, error)
	GetTmdbPopularMovies(region ...string) ([]*TmdbMovie, error)
	ClearCache()
}

type tmdbClient struct {
	key            string
	baseURL        string
	httpClient     *http.Client
	maxRetries     int
	retryBaseDelay time.Duration
	movieCache     *cache.Cache
}

var _ TmdbInterface = (*tmdbClient)(nil)

func New(apiKey string) (TmdbInterface, error) {
	if apiKey == "" {
		return nil, errors.New("TMDB_API_KEY environment variable is not set")
	}

	client := tmdbClient{
		key:            apiKey,
		baseURL:        helpers.TMDB_BASE_API_URL,
		httpClient:     &http.Client{Timeout: helpers.TMDB_HTTP_TIMEOUT},
		maxRetries:     helpers.TMDB_HTTP_MAX_RETRIES,
		retryBaseDelay: helpers.TMDB_HTTP_RETRY_BASE_DELAY,
		movieCache:     cache.New(tmdbMovieCacheTTL, tmdbMovieCacheCleanup),
	}

	return &client, nil
}
