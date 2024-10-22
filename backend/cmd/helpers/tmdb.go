package helpers

import (
	"encoding/json"
	"errors"
	"fmt"
	"igloo/cmd/models"
	"io"
	"log"
	"net/http"
	"os"
)

const tmdbUrl = "https://api.themoviedb.org/3"

func GetTmdbMovie(movie *models.Movie, download bool) error {
	if movie.TmdbID == "" {
		return errors.New("tmdb id is required")
	}

	tmdbKey := os.Getenv("TMDB_API_KEY")
	if tmdbKey == "" {
		return errors.New("tmdb api key is required")
	}

	url := fmt.Sprintf("%s/movie/%s?api_key=%s&append_to_response=credits", tmdbUrl, movie.TmdbID, tmdbKey)

	client := &http.Client{}

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return err
	}

	req.Header.Add("Accept", "application/json")
	req.Header.Add("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	var tmdbObject interface{}

	err = json.Unmarshal(bodyBytes, &tmdbObject)
	if err != nil {
		return err
	}

	log.Print(tmdbObject)

	return nil
}
