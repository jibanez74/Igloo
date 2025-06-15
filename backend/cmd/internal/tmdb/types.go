package tmdb

type TmdbInterface interface {
	GetTmdbMovieByID(movie *TMDBMovie) error
	GetTmdbMovieByTitle(movie *TMDBMovie) error
	SearchMoviesByTitleAndYear(title string, year ...int) ([]TMDBMovie, error)
	GetMoviesInTheaters() ([]*TMDBMovie, error)
	GetTmdbPopularMovies(region ...string) ([]*TMDBMovie, error)
}

type TmdbClient struct {
	Key string `json:"api_key"`
}

type TMDBLanguage struct {
	ISO639_1 string `json:"iso_639_1"`
	Name     string `json:"name"`
}

type TMDBGenre struct {
	ID  int32  `json:"id"`
	Tag string `json:"name"`
}

type TMDBStudio struct {
	ID      int32  `json:"id"`
	Name    string `json:"name"`
	Logo    string `json:"logo_path"`
	Country string `json:"origin_country"`
}

type TMDBCredits struct {
	Cast []struct {
		ID           int32  `json:"id"`
		Name         string `json:"name"`
		OriginalName string `json:"original_name"`
		Thumb        string `json:"profile_path"`
		Character    string `json:"character"`
		SortOrder    int32  `json:"order"`
	} `json:"cast"`

	Crew []struct {
		ID           int32  `json:"id"`
		Name         string `json:"name"`
		OriginalName string `json:"original_name"`
		Thumb        string `json:"profile_path"`
		Job          string `json:"job"`
		Department   string `json:"department"`
	} `json:"crew"`
}

type TMDBVideo struct {
	ID          string `json:"id"`
	Site        string `json:"site"`
	Key         string `json:"key"`
	Name        string `json:"name"`
	Type        string `json:"type"`
	Official    bool   `json:"official"`
	Language    string `json:"iso_639_1"`
	Country     string `json:"iso_3166_1"`
	PublishedAt string `json:"published_at"`
}

type TMDBVideos struct {
	Results []TMDBVideo `json:"results"`
}

type TMDBReleaseDate struct {
	Country       string `json:"iso_3166_1"`
	Certification string `json:"certification"`
	ReleaseDate   string `json:"release_date"`
}

type TMDBReleaseDates struct {
	Results []struct {
		Country      string            `json:"iso_3166_1"`
		ReleaseDates []TMDBReleaseDate `json:"release_dates"`
	} `json:"results"`
}

type TMDBMovie struct {
	Title           string           `json:"title"`
	Adult           bool             `json:"adult"`
	TagLine         string           `json:"tagline"`
	Summary         string           `json:"overview"`
	Budget          int64            `json:"budget"`
	Revenue         int64            `json:"revenue"`
	RunTime         int32            `json:"runtime"`
	AudienceRating  float32          `json:"vote_average"`
	ImdbID          string           `json:"imdb_id"`
	TmdbID          int32            `json:"id"`
	ReleaseDate     string           `json:"release_date"`
	Thumb           string           `json:"poster_path"`
	Art             string           `json:"backdrop_path"`
	SpokenLanguages []TMDBLanguage   `json:"spoken_languages"`
	Genres          []TMDBGenre      `json:"genres"`
	Studios         []TMDBStudio     `json:"production_companies"`
	Credits         TMDBCredits      `json:"credits"`
	Videos          TMDBVideos       `json:"videos"`
	ReleaseDates    TMDBReleaseDates `json:"release_dates"`
}
