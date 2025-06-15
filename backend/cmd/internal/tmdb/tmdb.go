package tmdb

import (
	"errors"
	"os"
)

func New() (TmdbInterface, error) {
	apiKey := os.Getenv("TMDB_API_KEY")
	if apiKey == "" {
		return nil, errors.New("TMDB_API_KEY environment variable is not set")
	}

	client := TmdbClient{
		Key: apiKey,
	}

	return &client, nil
}
