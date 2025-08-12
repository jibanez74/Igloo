package tmdb

import (
  "encoding/json"
  "errors"
  "fmt"
  "io"
  "net/http"
  "net/url"
  "strconv"
)

func (t *TmdbClient) GetTmdbMovieByID(movie *TmdbMovie) error {
  if movie.TmdbID == 0 {
    return errors.New("tmdb id is required")
  }

  params := url.Values{}
  params.Add("api_key", t.Key)
  params.Add("append_to_response", "credits,videos,release_dates")

  requestURL := fmt.Sprintf("%s/movie/%d?%s", BASE_API_URL, movie.TmdbID, params.Encode())

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

func (t *TmdbClient) GetTmdbMovieByTitle(movie *TmdbMovie) error {
  if movie.Title == "" {
    return errors.New("movie title is required")
  }

  params := url.Values{}
  params.Add("api_key", t.Key)
  params.Add("query", movie.Title)
  params.Add("include_adult", "false")

  requestURL := fmt.Sprintf("%s/search/movie?%s", BASE_API_URL, params.Encode())

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

func (t *TmdbClient) SearchMoviesByTitleAndYear(title string, year ...int) ([]TmdbMovie, error) {
  if title == "" {
    return nil, errors.New("movie title is required")
  }

  params := url.Values{}
  params.Add("api_key", t.Key)
  params.Add("query", title)
  params.Add("include_adult", "false")

  if len(year) > 0 && year[0] > 0 {
    params.Add("year", fmt.Sprintf("%d", year[0]))
  }

  requestURL := fmt.Sprintf("%s/search/movie?%s", BASE_API_URL, params.Encode())

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

func (t *TmdbClient) GetMoviesInTheaters() ([]*TmdbMovie, error) {
  params := url.Values{}
  params.Add("api_key", t.Key)
  params.Add("language", "en-US")
  params.Add("page", "1")
  params.Add("region", "US")

  requestURL := fmt.Sprintf("%s/movie/now_playing?%s", BASE_API_URL, params.Encode())

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

func (t *TmdbClient) GetTmdbPopularMovies(region ...string) ([]*TmdbMovie, error) {
  params := url.Values{}
  params.Add("api_key", t.Key)
  params.Add("language", "en-US")
  params.Add("page", "1")

  regionCode := "US"
  if len(region) > 0 && region[0] != "" {
    regionCode = region[0]
  }
  params.Add("region", regionCode)

  requestURL := fmt.Sprintf("%s/movie/popular?%s", BASE_API_URL, params.Encode())

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
