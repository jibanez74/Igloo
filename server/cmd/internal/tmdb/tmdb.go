package tmdb

import (
	"context"
	"errors"
	"igloo/cmd/internal/helpers"
	"net/http"
	"time"

	cache "github.com/patrickmn/go-cache"
)

// TMDB request locale. The catalog is English-language and certifications are
// taken from the US rating board; the region only scopes the now-playing list.
const (
	tmdbRequestLanguage      = "en-US"
	tmdbRequestRegion        = "US"
	tmdbCertificationCountry = "US"
)

const (
	tmdbBaseAPIURL         = "https://api.themoviedb.org/3"
	tmdbHTTPMaxRetries     = 3
	tmdbHTTPRetryBaseDelay = 250 * time.Millisecond
	tmdbHTTPRetryMaxDelay  = 2 * time.Second
)

type TmdbInterface interface {
	SearchShowsByTitleAndYear(ctx context.Context, title string, year ...int) ([]TVShow, error)
	GetShowDetails(ctx context.Context, id int) (*TVShow, error)
	GetSeasonDetails(ctx context.Context, id, season int) (*TVSeason, error)
	GetTmdbMovieByID(ctx context.Context, movie *TmdbMovie) error
	SearchMoviesByTitleAndYear(ctx context.Context, title string, year ...int) ([]TmdbMovie, error)
	GetMoviesInTheaters(ctx context.Context) ([]*TmdbMovie, error)
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
		baseURL:        tmdbBaseAPIURL,
		httpClient:     &http.Client{Timeout: helpers.PROVIDER_HTTP_TIMEOUT},
		maxRetries:     tmdbHTTPMaxRetries,
		retryBaseDelay: tmdbHTTPRetryBaseDelay,
		movieCache:     cache.New(tmdbMovieCacheTTL, tmdbMovieCacheCleanup),
	}

	return &client, nil
}
