package tmdb

import (
	"net/http"
	"time"
)

func New(key string) Tmdb {
	return &tmdb{
		baseUrl: "https://api.themoviedb.org/3",
		key:     key,
		http:    &http.Client{},
	}
}

func (t *tmdb) formatReleaseDate(date string) (time.Time, error) {
	layout := "2006-01-02"

	return time.Parse(layout, date)
}
