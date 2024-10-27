package tmdb

import (
	"igloo/cmd/database/models"
	"net/http"
	"time"
)

type Tmdb interface {
	formatReleaseDate(date string) (time.Time, error)
	GetTmdbMovieByID(movieByID *models.Movie) error
}

type tmdb struct {
	baseUrl string
	key     string
	http    *http.Client
}

type tmdbLanguage struct {
	ISO639_1 string `json:"iso_639_1"`
	Name     string `json:"name"`
}

type tmdbGenre struct {
	Tag string `json:"name"`
}

type tmdbStudio struct {
	Name    string `json:"name"`
	Logo    string `json:"logo_path"`
	Country string `json:"origin_country"`
}

type tmdbCredits struct {
	Cast []struct {
		Name         string `json:"name"`
		OriginalName string `json:"original_name"`
		Thumb        string `json:"profile_path"`
		Character    string `json:"character"`
		Order        int    `json:"order"`
	} `json:"cast"`

	Crew []struct {
		Name         string `json:"name"`
		OriginalName string `json:"original_name"`
		Thumb        string `json:"profile_path"`
		Job          string `json:"job"`
		Department   string `json:"department"`
	} `json:"crew"`
}

type tmdbMovie struct {
	Title           string         `json:"title"`
	Adult           bool           `json:"adult"`
	TagLine         string         `json:"tagline"`
	Summary         string         `json:"overview"`
	Budget          uint           `json:"budget"`
	Revenue         uint           `json:"revenue"`
	RunTime         uint           `json:"runtime"`
	AudienceRating  float32        `json:"vote_average"`
	ImdbID          string         `json:"imdb_id"`
	ReleaseDate     string         `json:"release_date"`
	Thumb           string         `json:"poster_path"`
	Art             string         `json:"backdrop_path"`
	SpokenLanguages []tmdbLanguage `json:"spoken_languages"`
	Genres          []tmdbGenre    `json:"genres"`
	Studios         []tmdbStudio   `json:"production_companies"`
	Credits         tmdbCredits    `json:"credits"`
}
