package tmdb

import (
	"errors"
)

type TmdbInterface interface {
	GetTmdbMovieByID(movie *TmdbMovie) error
	GetTmdbMovieByTitle(movie *TmdbMovie) error
	SearchMoviesByTitleAndYear(title string, year ...int) ([]TmdbMovie, error)
	GetMoviesInTheaters() ([]*TmdbMovie, error)
	GetTmdbPopularMovies(region ...string) ([]*TmdbMovie, error)
}

type tmdbClient struct {
	key string
}

func New(apiKey string) (TmdbInterface, error) {
	if apiKey == "" {
		return nil, errors.New("TMDB_API_KEY environment variable is not set")
	}

	client := tmdbClient{
		key: apiKey,
	}

	return &client, nil
}
