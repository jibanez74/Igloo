package tmdb

import (
	"errors"

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
	key        string
	movieCache *cache.Cache
}

var _ TmdbInterface = (*tmdbClient)(nil)

func New(apiKey string) (TmdbInterface, error) {
	if apiKey == "" {
		return nil, errors.New("TMDB_API_KEY environment variable is not set")
	}

	client := tmdbClient{
		key:        apiKey,
		movieCache: cache.New(tmdbMovieCacheTTL, tmdbMovieCacheCleanup),
	}

	return &client, nil
}
