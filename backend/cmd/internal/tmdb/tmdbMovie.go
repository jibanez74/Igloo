package tmdb

import (
  "encoding/json"
  "errors"
  "fmt"
  "io"
  "net/http"
  "net/url"

  "github.com/gofiber/fiber/v2"
)

func (t *tmdb) GetTmdbMovieByID(tmdbID *string) (*tmdbMovie, error) {
  if tmdbID == nil {
    return nil, errors.New("tmdb id is required")
  }

  baseURL := fmt.Sprintf("%s/movie/%s", t.baseUrl, *tmdbID)

  params := url.Values{}
  params.Add("api_key", *t.key)
  params.Add("append_to_response", "credits,videos,release_dates")

  requestURL := fmt.Sprintf("%s?%s", baseURL, params.Encode())

  req, err := http.NewRequest("GET", requestURL, nil)
  if err != nil {
    return nil, err
  }

  req.Header.Add("Accept", "application/json")
  req.Header.Add("Content-Type", "application/json")

  resp, err := t.http.Do(req)
  if err != nil {
    return nil, err
  }
  defer resp.Body.Close()

  if resp.StatusCode != fiber.StatusOK {
    if resp.StatusCode == fiber.StatusTooManyRequests {
      return nil, errors.New("rate limit exceeded for tmdb")
    }

    return nil, errors.New("unable to get movie from tmdb")
  }

  bodyBytes, err := io.ReadAll(resp.Body)
  if err != nil {
    return nil, err
  }

  var tmdbObject tmdbMovie

  err = json.Unmarshal(bodyBytes, &tmdbObject)
  if err != nil {
    return nil, err
  }

  return &tmdbObject, nil
}
