package tmdb

import (
	"errors"
)

func New(apiKey string) (TmdbInterface, error) {
	if apiKey == "" {
		return nil, errors.New("TMDB_API_KEY environment variable is not set")
	}

	client := TmdbClient{
		Key: apiKey,
	}

	return &client, nil
}
