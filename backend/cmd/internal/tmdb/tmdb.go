package tmdb

import (
	"errors"
	"net/http"
	"time"
)

func New(key *string) (Tmdb, error) {
	if key == nil || *key == "" {
		return nil, errors.New("tmdb key is required")
	}

	return &tmdb{
		baseUrl: "https://api.themoviedb.org/3",
		key:     key,
		http:    &http.Client{},
	}, nil
}

func (t *tmdb) formatReleaseDate(date string) (time.Time, error) {
	layout := "2006-01-02"
	return time.Parse(layout, date)
}
