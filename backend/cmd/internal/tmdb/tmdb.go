package tmdb

import (
	"net/http"
)

type Tmdb interface {
	GetTmdbMovieByID(*string) (*tmdbMovie, error)
}

type tmdb struct {
	baseUrl string
	key     *string
	http    *http.Client
}

type tmdbLanguage struct {
	ISO639_1 string `json:"iso_639_1"`
	Name     string `json:"name"`
}

type tmdbGenre struct {
	ID  int32  `json:"id"`
	Tag string `json:"name"`
}

type tmdbStudio struct {
	ID      int32  `json:"id"`
	Name    string `json:"name"`
	Logo    string `json:"logo_path"`
	Country string `json:"origin_country"`
}
