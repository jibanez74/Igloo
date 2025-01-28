package tmdb

import (
	"encoding/json"
	"errors"
	"fmt"
	"igloo/cmd/internal/database/models"
	"igloo/cmd/internal/helpers"
	"io"
	"net/http"
	"net/url"
	"strings"
	"unicode/utf8"

	"github.com/gofiber/fiber/v2"
)

func (t *tmdb) GetTmdbMovieByID(movie *models.Movie) error {
	if movie.TmdbID == "" {
		return errors.New("tmdb id is required")
	}

	baseURL := fmt.Sprintf("%s/movie/%s", t.baseUrl, movie.TmdbID)

	params := url.Values{}
	params.Add("api_key", *t.key)
	params.Add("append_to_response", "credits,videos,release_dates")

	requestURL := fmt.Sprintf("%s?%s", baseURL, params.Encode())

	req, err := http.NewRequest("GET", requestURL, nil)
	if err != nil {
		return err
	}

	req.Header.Add("Accept", "application/json")
	req.Header.Add("Content-Type", "application/json")

	resp, err := t.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != fiber.StatusOK {
		if resp.StatusCode == fiber.StatusTooManyRequests {
			return errors.New("rate limit exceeded for tmdb")
		}

		return errors.New("unable to get movie from tmdb")
	}

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	var tmdbObject tmdbMovie

	err = json.Unmarshal(bodyBytes, &tmdbObject)
	if err != nil {
		return err
	}

	movie.Title = tmdbObject.Title
	movie.Adult = tmdbObject.Adult
	movie.TagLine = tmdbObject.TagLine
	movie.Summary = tmdbObject.Summary
	movie.Budget = tmdbObject.Budget
	movie.Revenue = tmdbObject.Revenue
	movie.RunTime = tmdbObject.RunTime
	movie.AudienceRating = tmdbObject.AudienceRating
	movie.ImdbUrl = fmt.Sprintf("https://www.imdb.com/title/%s", tmdbObject.ImdbID)

	if len(tmdbObject.SpokenLanguages) > 0 {
		var languages []string

		for _, l := range tmdbObject.SpokenLanguages {
			if utf8.ValidString(l.Name) {
				languages = append(languages, l.Name)
			} else {
				validLanguageName := helpers.SanitizeString(l.Name)
				languages = append(languages, validLanguageName)
			}
		}

		movie.SpokenLanguages = strings.Join(languages, ", ")
	}

	releaseDate, err := formatReleaseDate(tmdbObject.ReleaseDate)
	if err == nil {
		movie.ReleaseDate = releaseDate
		movie.Year = uint(releaseDate.Year())
	}

	if tmdbObject.Thumb != "" {
		tmdbURL := fmt.Sprintf("https://image.tmdb.org/t/p/w500%s", tmdbObject.Thumb)

		filename, err := helpers.SaveImage(
			tmdbURL,
			movie.Title,
			"static/movies/thumb",
		)

		if err != nil {
			return fmt.Errorf("failed to save poster: %w", err)
		}

		movie.Thumb = fmt.Sprintf("/static/movies/thumb/%s", filename)
	}

	if tmdbObject.Art != "" {
		tmdbURL := fmt.Sprintf("https://image.tmdb.org/t/p/w1280%s", tmdbObject.Art)

		filename, err := helpers.SaveImage(
			tmdbURL,
			movie.Title,
			"static/movies/art",
		)

		if err != nil {
			return fmt.Errorf("failed to save backdrop: %w", err)
		}

		movie.Art = fmt.Sprintf("/static/movies/art/%s", filename)
	}

	for _, g := range tmdbObject.Genres {
		movie.Genres = append(movie.Genres, &models.Genre{
			Tag:       g.Tag,
			GenreType: "movie",
		})
	}

	for _, s := range tmdbObject.Studios {
		var studioLogo string

		if s.Logo != "" {
			tmdbURL := fmt.Sprintf("https://image.tmdb.org/t/p/w500%s", s.Logo)

			filename, err := helpers.SaveImage(
				tmdbURL,
				s.Name,
				"static/studios",
			)

			if err != nil {
				studioLogo = tmdbURL
			} else {

				studioLogo = fmt.Sprintf("/static/studios/%s", filename)
			}
		}

		movie.Studios = append(movie.Studios, &models.Studio{
			Name:    s.Name,
			Logo:    studioLogo,
			Country: s.Country,
		})
	}

	for _, c := range tmdbObject.Credits.Cast {
		tmdbURL := fmt.Sprintf("https://image.tmdb.org/t/p/w500%s", c.Thumb)

		var artistThumb string

		if c.Thumb != "" {
			filename, err := helpers.SaveImage(
				tmdbURL,
				c.Name,
				"static/artists",
			)

			if err != nil {
				artistThumb = tmdbURL
			} else {
				artistThumb = fmt.Sprintf("/static/artists/%s", filename)
			}
		}

		movie.CastList = append(movie.CastList, models.Cast{
			Artist: models.Artist{
				Name:         c.Name,
				OriginalName: c.OriginalName,
				Thumb:        artistThumb,
			},
			Character: c.Character,
			Order:     c.Order,
		})
	}

	for _, c := range tmdbObject.Credits.Crew {
		tmdbURL := fmt.Sprintf("https://image.tmdb.org/t/p/w500%s", c.Thumb)

		var artistThumb string

		if c.Thumb != "" {
			filename, err := helpers.SaveImage(
				tmdbURL,
				c.Name,
				"static/artists",
			)

			if err != nil {

				artistThumb = tmdbURL
			} else {
				artistThumb = fmt.Sprintf("/static/artists/%s", filename)
			}
		}

		movie.CrewList = append(movie.CrewList, models.Crew{
			Artist: models.Artist{
				Name:         c.Name,
				OriginalName: c.OriginalName,
				Thumb:        artistThumb,
			},
			Job:        c.Job,
			Department: c.Department,
		})
	}

	for _, v := range tmdbObject.Videos.Results {
		if v.Site == "YouTube" {
			movie.Extras = append(movie.Extras, models.MovieExtra{
				Title: v.Title,
				Url:   fmt.Sprintf("https://www.youtube.com/watch?v=%s", v.Key),
				Kind:  v.Kind,
			})
		}
	}

	for _, countryRelease := range tmdbObject.ReleaseDates.Results {
		if countryRelease.Country == "US" { // Target the USA only
			for _, release := range countryRelease.ReleaseDates {
				if release.Certification != "" {
					movie.ContentRating = release.Certification
					break
				}
			}

			break
		}
	}

	return nil
}
