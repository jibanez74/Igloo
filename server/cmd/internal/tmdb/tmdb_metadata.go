package tmdb

import (
	"encoding/json"
	"errors"
	"fmt"
	"igloo/cmd/internal/helpers"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
)

// TmdbVideoResult is a single video (trailer, featurette, etc.) from TMDB videos.results.
type TmdbVideoResult struct {
	ID       string `json:"id"`
	Key      string `json:"key"`
	Name     string `json:"name"`
	Site     string `json:"site"`
	Type     string `json:"type"`
	Official bool   `json:"official"`
}

type TmdbMovie struct {
	TmdbID              int     `json:"id"`
	Title               string  `json:"title"`
	OriginalTitle       string  `json:"original_title"`
	Overview            string  `json:"overview"`
	ReleaseDate         string  `json:"release_date"`
	PosterPath          string  `json:"poster_path"`
	BackdropPath        string  `json:"backdrop_path"`
	Popularity          float64 `json:"popularity"`
	VoteAverage         float64 `json:"vote_average"`
	VoteCount           int     `json:"vote_count"`
	Adult               bool    `json:"adult"`
	OriginalLang        string  `json:"original_language"`
	GenreIDs            []int   `json:"genre_ids"`
	Video               bool    `json:"video"`
	Runtime             int     `json:"runtime"`
	Status              string  `json:"status"`
	Tagline             string  `json:"tagline"`
	Budget              int64   `json:"budget"`
	Revenue             int64   `json:"revenue"`
	Homepage            string  `json:"homepage"`
	ImdbID              string  `json:"imdb_id"`
	ProductionCompanies []struct {
		ID            int    `json:"id"`
		LogoPath      string `json:"logo_path"`
		Name          string `json:"name"`
		OriginCountry string `json:"origin_country"`
	} `json:"production_companies"`
	Genres []struct {
		ID   int    `json:"id"`
		Name string `json:"name"`
	} `json:"genres"`
	Credits struct {
		Cast []struct {
			ID          int    `json:"id"`
			Name        string `json:"name"`
			Character   string `json:"character"`
			ProfilePath string `json:"profile_path"`
			Order       int    `json:"order"`
		} `json:"cast"`
		Crew []struct {
			ID          int    `json:"id"`
			Name        string `json:"name"`
			Job         string `json:"job"`
			Department  string `json:"department"`
			ProfilePath string `json:"profile_path"`
		} `json:"crew"`
	} `json:"credits"`
	Videos struct {
		Results []TmdbVideoResult `json:"results"`
	} `json:"videos"`
	ReleaseDates struct {
		Results []struct {
			ISO3166_1    string `json:"iso_3166_1"`
			ReleaseDates []struct {
				Certification string `json:"certification"`
			} `json:"release_dates"`
		} `json:"results"`
	} `json:"release_dates"`
}

// Certification returns the parental rating (e.g. PG-13, R) from TMDB release_dates.
// Prefers US certification; otherwise returns the first non-empty certification from any country.
func (m *TmdbMovie) Certification() string {
	var usCert, firstCert string
	for _, r := range m.ReleaseDates.Results {
		for _, rd := range r.ReleaseDates {
			c := strings.TrimSpace(rd.Certification)
			if c == "" {
				continue
			}
			if firstCert == "" {
				firstCert = c
			}
			if r.ISO3166_1 == "US" {
				usCert = c
				break
			}
		}
		if usCert != "" {
			break
		}
	}
	if usCert != "" {
		return usCert
	}
	return firstCert
}

func (t *tmdbClient) GetTmdbMovieByID(movie *TmdbMovie) error {
	if movie.TmdbID == 0 {
		return errors.New("tmdb id is required")
	}

	params := url.Values{}
	params.Add("api_key", t.key)
	params.Add("append_to_response", "credits,videos,release_dates")

	requestURL := fmt.Sprintf("%s/movie/%d?%s", helpers.TMDB_BASE_API_URL, movie.TmdbID, params.Encode())

	req, err := http.NewRequest("GET", requestURL, nil)
	if err != nil {
		return err
	}

	req.Header.Add("Accept", "application/json")
	req.Header.Add("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		if resp.StatusCode == http.StatusTooManyRequests {
			return errors.New("rate limit exceeded for tmdb")
		}
		return errors.New("unable to get movie from tmdb")
	}

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	err = json.Unmarshal(bodyBytes, movie)
	if err != nil {
		return err
	}

	return nil
}

func (t *tmdbClient) GetTmdbMovieByTitle(movie *TmdbMovie) error {
	if movie.Title == "" {
		return errors.New("movie title is required")
	}

	params := url.Values{}
	params.Add("api_key", t.key)
	params.Add("query", movie.Title)
	params.Add("include_adult", "false")

	requestURL := fmt.Sprintf("%s/search/movie?%s", helpers.TMDB_BASE_API_URL, params.Encode())

	req, err := http.NewRequest("GET", requestURL, nil)
	if err != nil {
		return err
	}

	req.Header.Add("Accept", "application/json")
	req.Header.Add("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		if resp.StatusCode == http.StatusTooManyRequests {
			return errors.New("rate limit exceeded for tmdb")
		}
		return errors.New("unable to search movie by title from tmdb")
	}

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	var searchResult struct {
		Results []TmdbMovie `json:"results"`
	}

	err = json.Unmarshal(bodyBytes, &searchResult)
	if err != nil {
		return err
	}

	if len(searchResult.Results) == 0 {
		return errors.New("no movie found with the given title")
	}

	*movie = searchResult.Results[0]
	return nil
}

func (t *tmdbClient) SearchMoviesByTitleAndYear(title string, year ...int) ([]TmdbMovie, error) {
	if title == "" {
		return nil, errors.New("movie title is required")
	}

	params := url.Values{}
	params.Add("api_key", t.key)
	params.Add("query", title)
	params.Add("include_adult", "false")

	if len(year) > 0 && year[0] > 0 {
		params.Add("year", fmt.Sprintf("%d", year[0]))
	}

	requestURL := fmt.Sprintf("%s/search/movie?%s", helpers.TMDB_BASE_API_URL, params.Encode())

	req, err := http.NewRequest("GET", requestURL, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Add("Accept", "application/json")
	req.Header.Add("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		if resp.StatusCode == http.StatusTooManyRequests {
			return nil, errors.New("rate limit exceeded for tmdb")
		}

		return nil, errors.New("unable to search movies from tmdb")
	}

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var searchResult struct {
		Results []TmdbMovie `json:"results"`
	}

	err = json.Unmarshal(bodyBytes, &searchResult)
	if err != nil {
		return nil, err
	}

	if len(searchResult.Results) == 0 {
		return nil, errors.New("no movies found with the given query")
	}

	if len(year) > 0 && year[0] > 0 {
		var filteredResults []TmdbMovie
		for _, movie := range searchResult.Results {
			if len(movie.ReleaseDate) >= 4 {
				movieYear, err := strconv.Atoi(movie.ReleaseDate[:4])
				if err == nil && movieYear == year[0] {
					filteredResults = append(filteredResults, movie)
				}
			}
		}

		if len(filteredResults) == 0 {
			return nil, fmt.Errorf("no movies found with title '%s' from year %d", title, year[0])
		}

		return filteredResults, nil
	}

	return searchResult.Results, nil
}

func (t *tmdbClient) GetMoviesInTheaters() ([]*TmdbMovie, error) {
	params := url.Values{}
	params.Add("api_key", t.key)
	params.Add("language", "en-US")
	params.Add("page", "1")
	params.Add("region", "US")

	requestURL := fmt.Sprintf("%s/movie/now_playing?%s", helpers.TMDB_BASE_API_URL, params.Encode())

	req, err := http.NewRequest("GET", requestURL, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Add("Accept", "application/json")
	req.Header.Add("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		if resp.StatusCode == http.StatusTooManyRequests {
			return nil, errors.New("rate limit exceeded for tmdb")
		}
		return nil, errors.New("unable to get movies in theaters from tmdb")
	}

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var response struct {
		Results []TmdbMovie `json:"results"`
	}

	err = json.Unmarshal(bodyBytes, &response)
	if err != nil {
		return nil, err
	}

	if len(response.Results) == 0 {
		return nil, errors.New("no movies found in theaters")
	}

	movies := make([]*TmdbMovie, len(response.Results))
	for i, movie := range response.Results {
		movies[i] = &movie
	}

	return movies, nil
}

func (t *tmdbClient) GetTmdbPopularMovies(region ...string) ([]*TmdbMovie, error) {
	params := url.Values{}
	params.Add("api_key", t.key)
	params.Add("language", "en-US")
	params.Add("page", "1")

	regionCode := "US"
	if len(region) > 0 && region[0] != "" {
		regionCode = region[0]
	}
	params.Add("region", regionCode)

	requestURL := fmt.Sprintf("%s/movie/popular?%s", helpers.TMDB_BASE_API_URL, params.Encode())

	req, err := http.NewRequest("GET", requestURL, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Add("Accept", "application/json")
	req.Header.Add("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		if resp.StatusCode == http.StatusTooManyRequests {
			return nil, errors.New("rate limit exceeded for tmdb")
		}
		return nil, errors.New("unable to get popular movies from tmdb")
	}

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var response struct {
		Results []TmdbMovie `json:"results"`
	}

	err = json.Unmarshal(bodyBytes, &response)
	if err != nil {
		return nil, err
	}

	if len(response.Results) == 0 {
		return nil, errors.New("no popular movies found")
	}

	movies := make([]*TmdbMovie, len(response.Results))
	for i, movie := range response.Results {
		movies[i] = &movie
	}

	return movies, nil
}
