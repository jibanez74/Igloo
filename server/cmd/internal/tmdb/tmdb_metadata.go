package tmdb

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"igloo/cmd/internal/helpers"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
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

// TmdbMovieSearchResult is the JSON shape for Igloo TMDB search/identify list responses.
// It uses tmdb_id to match the frontend contract; TmdbMovie uses json:"id" when
// unmarshaling the TMDB API, so it is not suitable to embed directly in these responses.
type TmdbMovieSearchResult struct {
	TmdbID      int    `json:"tmdb_id"`
	Title       string `json:"title"`
	ReleaseDate string `json:"release_date"`
	Overview    string `json:"overview"`
	PosterPath  string `json:"poster_path"`
}

// NewTmdbMovieSearchResult maps a TmdbMovie to the slim search/identify API shape.
func NewTmdbMovieSearchResult(m *TmdbMovie) TmdbMovieSearchResult {
	if m == nil {
		return TmdbMovieSearchResult{}
	}
	return TmdbMovieSearchResult{
		TmdbID:      m.TmdbID,
		Title:       m.Title,
		ReleaseDate: m.ReleaseDate,
		Overview:    m.Overview,
		PosterPath:  m.PosterPath,
	}
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

func (t *tmdbClient) GetTmdbMovieByID(ctx context.Context, movie *TmdbMovie) error {
	if movie.TmdbID == 0 {
		return errors.New("tmdb id is required")
	}

	if cached, ok := t.getMovie(movie.TmdbID); ok {
		*movie = *cached
		return nil
	}

	params := url.Values{}
	params.Add("append_to_response", "credits,videos,release_dates")

	requestURL := t.requestURL(fmt.Sprintf("/movie/%d", movie.TmdbID), params)

	bodyBytes, statusCode, err := t.getJSON(ctx, requestURL)
	if err != nil {
		return err
	}
	if statusCode != http.StatusOK {
		return tmdbStatusError(statusCode, "unable to get movie from tmdb")
	}

	err = json.Unmarshal(bodyBytes, movie)
	if err != nil {
		return err
	}

	movieCopy := *movie
	t.setMovie(movie.TmdbID, &movieCopy)

	return nil
}

func (t *tmdbClient) SearchMoviesByTitleAndYear(ctx context.Context, title string, year ...int) ([]TmdbMovie, error) {
	if title == "" {
		return nil, errors.New("movie title is required")
	}

	params := url.Values{}
	params.Add("query", title)
	params.Add("include_adult", "false")

	hasYear := len(year) > 0 && year[0] > 0
	if hasYear {
		params.Add("year", fmt.Sprintf("%d", year[0]))
	}

	results, err := t.getMovieList(ctx, "/search/movie", params, "unable to search movies from tmdb")
	if err != nil {
		return nil, err
	}

	// If the year-filtered search returned nothing, retry without the year constraint.
	if len(results) == 0 && hasYear {
		params.Del("year")
		results, err = t.getMovieList(ctx, "/search/movie", params, "unable to search movies from tmdb")
		if err != nil {
			return nil, err
		}
	}

	if len(results) == 0 {
		return nil, errors.New("no movies found with the given query")
	}

	return results, nil
}

func (t *tmdbClient) GetMoviesInTheaters(ctx context.Context) ([]*TmdbMovie, error) {
	params := url.Values{}
	params.Add("language", "en-US")
	params.Add("page", "1")
	params.Add("region", "US")

	results, err := t.getMovieList(ctx, "/movie/now_playing", params, "unable to get movies in theaters from tmdb")
	if err != nil {
		return nil, err
	}

	if len(results) == 0 {
		return nil, errors.New("no movies found in theaters")
	}

	return tmdbMoviePointers(results), nil
}

func (t *tmdbClient) requestURL(path string, params url.Values) string {
	if params == nil {
		params = url.Values{}
	}
	params.Set("api_key", t.key)
	return fmt.Sprintf("%s%s?%s", t.baseURL, path, params.Encode())
}

func (t *tmdbClient) getMovieList(ctx context.Context, path string, params url.Values, statusFallback string) ([]TmdbMovie, error) {
	requestURL := t.requestURL(path, params)
	bodyBytes, statusCode, err := t.getJSON(ctx, requestURL)
	if err != nil {
		return nil, err
	}
	if statusCode != http.StatusOK {
		return nil, tmdbStatusError(statusCode, statusFallback)
	}

	var response struct {
		Results []TmdbMovie `json:"results"`
	}

	err = json.Unmarshal(bodyBytes, &response)
	if err != nil {
		return nil, err
	}

	return response.Results, nil
}

func tmdbMoviePointers(movies []TmdbMovie) []*TmdbMovie {
	ptrs := make([]*TmdbMovie, len(movies))
	for i := range movies {
		ptrs[i] = &movies[i]
	}

	return ptrs
}

func (t *tmdbClient) getJSON(ctx context.Context, requestURL string) ([]byte, int, error) {
	maxAttempts := t.maxRetries
	if maxAttempts < 1 {
		maxAttempts = 1
	}

	for attempt := 0; attempt < maxAttempts; attempt++ {
		if ctx != nil {
			err := ctx.Err()
			if err != nil {
				return nil, 0, err
			}
		}

		reqCtx := ctx
		if reqCtx == nil {
			reqCtx = context.Background()
		}

		req, err := http.NewRequestWithContext(reqCtx, http.MethodGet, requestURL, nil)
		if err != nil {
			return nil, 0, err
		}

		req.Header.Add("Accept", "application/json")
		req.Header.Add("Content-Type", "application/json")

		resp, err := t.httpClient.Do(req)
		if err != nil {
			if attempt == maxAttempts-1 {
				return nil, 0, err
			}
			err = sleepWithContext(ctx, t.retryDelay(nil, attempt))
			if err != nil {
				return nil, 0, err
			}
			continue
		}

		bodyBytes, readErr := io.ReadAll(resp.Body)
		resp.Body.Close()
		if readErr != nil {
			if attempt == maxAttempts-1 {
				return nil, 0, readErr
			}
			err = sleepWithContext(ctx, t.retryDelay(resp.Header, attempt))
			if err != nil {
				return nil, 0, err
			}
			continue
		}

		if retryableTmdbStatus(resp.StatusCode) && attempt < maxAttempts-1 {
			err = sleepWithContext(ctx, t.retryDelay(resp.Header, attempt))
			if err != nil {
				return nil, 0, err
			}
			continue
		}

		return bodyBytes, resp.StatusCode, nil
	}

	return nil, 0, errors.New("tmdb request failed")
}

func sleepWithContext(ctx context.Context, delay time.Duration) error {
	if delay <= 0 {
		if ctx != nil {
			return ctx.Err()
		}
		return nil
	}
	if ctx == nil {
		time.Sleep(delay)
		return nil
	}

	timer := time.NewTimer(delay)
	defer timer.Stop()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

func retryableTmdbStatus(statusCode int) bool {
	return statusCode == http.StatusTooManyRequests || statusCode >= http.StatusInternalServerError
}

func tmdbStatusError(statusCode int, fallback string) error {
	if statusCode == http.StatusTooManyRequests {
		return errors.New("rate limit exceeded for tmdb")
	}
	return errors.New(fallback)
}

func (t *tmdbClient) retryDelay(headers http.Header, attempt int) time.Duration {
	if headers != nil {
		retryAfter := strings.TrimSpace(headers.Get("Retry-After"))
		if retryAfter != "" {
			if seconds, err := strconv.Atoi(retryAfter); err == nil && seconds > 0 {
				delay := time.Duration(seconds) * time.Second
				if delay > helpers.TMDB_HTTP_RETRY_MAX_DELAY {
					return helpers.TMDB_HTTP_RETRY_MAX_DELAY
				}
				return delay
			}

			if retryTime, err := http.ParseTime(retryAfter); err == nil {
				delay := time.Until(retryTime)
				if delay < 0 {
					delay = 0
				}
				if delay > helpers.TMDB_HTTP_RETRY_MAX_DELAY {
					return helpers.TMDB_HTTP_RETRY_MAX_DELAY
				}
				return delay
			}
		}
	}

	delay := t.retryBaseDelay
	if delay <= 0 {
		delay = helpers.TMDB_HTTP_RETRY_BASE_DELAY
	}

	for i := 0; i < attempt; i++ {
		delay *= 2
		if delay >= helpers.TMDB_HTTP_RETRY_MAX_DELAY {
			return helpers.TMDB_HTTP_RETRY_MAX_DELAY
		}
	}

	return delay
}
