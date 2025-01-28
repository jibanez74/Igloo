package tmdb

import (
	"errors"
	"igloo/cmd/internal/database/models"
	"net/http"
)

type Tmdb interface {
	GetTmdbMovieByID(movieByID *models.Movie) error
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

type tmdbVideos struct {
	Results []struct {
		Key   string `json:"key"`
		Kind  string `json:"type"`
		Title string `json:"name"`
		Site  string `json:"site"`
	} `json:"results"`
}

type tmdbReleaseDate struct {
	Country       string `json:"iso_3166_1"`
	Certification string `json:"certification"`
	ReleaseDate   string `json:"release_date"`
}

type tmdbReleaseDates struct {
	Results []struct {
		Country      string            `json:"iso_3166_1"`
		ReleaseDates []tmdbReleaseDate `json:"release_dates"`
	} `json:"results"`
}

type tmdbMovie struct {
	Title           string           `json:"title"`
	Adult           bool             `json:"adult"`
	TagLine         string           `json:"tagline"`
	Summary         string           `json:"overview"`
	Budget          uint             `json:"budget"`
	Revenue         uint             `json:"revenue"`
	RunTime         uint             `json:"runtime"`
	AudienceRating  float32          `json:"vote_average"`
	ImdbID          string           `json:"imdb_id"`
	ReleaseDate     string           `json:"release_date"`
	Thumb           string           `json:"poster_path"`
	Art             string           `json:"backdrop_path"`
	SpokenLanguages []tmdbLanguage   `json:"spoken_languages"`
	Genres          []tmdbGenre      `json:"genres"`
	Studios         []tmdbStudio     `json:"production_companies"`
	Credits         tmdbCredits      `json:"credits"`
	Videos          tmdbVideos       `json:"videos"`
	ReleaseDates    tmdbReleaseDates `json:"release_dates"` // New field for content ratings and release dates
}

type tmdbMovies struct {
	Results []tmdbMovie `json:"results"`
}

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
