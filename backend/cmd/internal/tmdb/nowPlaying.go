package tmdb

import (
	"encoding/json"
	"fmt"
	"net/http"
)

type nowPlayingResponse struct {
	Results []struct {
		ID               int     `json:"id"`
		Title            string  `json:"title"`
		Overview         string  `json:"overview"`
		PosterPath       string  `json:"poster_path"`
		BackdropPath     string  `json:"backdrop_path"`
		ReleaseDate      string  `json:"release_date"`
		VoteAverage      float64 `json:"vote_average"`
		VoteCount        int     `json:"vote_count"`
		Popularity       float64 `json:"popularity"`
		OriginalLanguage string  `json:"original_language"`
		GenreIDs         []int   `json:"genre_ids"`
		Adult            bool    `json:"adult"`
	} `json:"results"`
}

func (t *tmdb) GetNowPlayingMovies() (*nowPlayingResponse, error) {
	url := fmt.Sprintf("%s/movie/now_playing?api_key=%s&language=en-US", t.baseUrl, *t.key)

	resp, err := http.Get(url)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch now playing movies: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("TMDB API returned status code %d", resp.StatusCode)
	}

	var response nowPlayingResponse
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return nil, fmt.Errorf("failed to decode TMDB response: %w", err)
	}

	// Limit to 12 movies
	if len(response.Results) > 12 {
		response.Results = response.Results[:12]
	}

	return &response, nil
}
