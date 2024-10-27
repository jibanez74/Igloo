package tmdb

import (
	"encoding/json"
	"errors"
	"fmt"
	"igloo/cmd/database/models"
	"io"
	"net/http"
	"strings"

	"github.com/gofiber/fiber/v2"
)

func (t *tmdb) GetTmdbMovieByID(movie *models.Movie) error {
	if movie.TmdbID == "" {
		return errors.New("tmdb id is required")
	}

	url := fmt.Sprintf("%s/movie/%s?api_key=%s&append_to_response=credits", t.baseUrl, movie.TmdbID, t.key)

	req, err := http.NewRequest("GET", url, nil)
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
		if len(tmdbObject.SpokenLanguages) == 1 {
			movie.SpokenLanguages = tmdbObject.SpokenLanguages[0].Name
		} else {
			var languages = make([]string, 0)

			for _, l := range tmdbObject.SpokenLanguages {
				languages = append(languages, l.Name)
			}

			movie.SpokenLanguages = strings.Join(languages, ", ")
			movie.SpokenLanguages = movie.SpokenLanguages[:len(movie.SpokenLanguages)-2]
		}
	}

	releaseDate, err := t.formatReleaseDate(tmdbObject.ReleaseDate)
	if err == nil {
		movie.ReleaseDate = releaseDate
		movie.Year = uint(releaseDate.Year())
	}

	if tmdbObject.Thumb != "" {
		movie.Thumb = fmt.Sprintf("https://image.tmdb.org/t/p/original%s", tmdbObject.Thumb)
	}

	if tmdbObject.Art != "" {
		movie.Art = fmt.Sprintf("https://image.tmdb.org/t/p/original%s", tmdbObject.Art)
	}

	for _, g := range tmdbObject.Genres {
		movie.Genres = append(movie.Genres, &models.Genre{
			Tag:       g.Tag,
			GenreType: "movie",
		})
	}

	for _, s := range tmdbObject.Studios {
		movie.Studios = append(movie.Studios, &models.Studio{
			Name:    s.Name,
			Logo:    fmt.Sprintf("https://image.tmdb.org/t/p/original%s", s.Logo),
			Country: s.Country,
		})
	}

	for _, c := range tmdbObject.Credits.Cast {
		movie.CastList = append(movie.CastList, models.Cast{
			Artist: models.Artist{
				Name:         c.Name,
				OriginalName: c.OriginalName,
				Thumb:        fmt.Sprintf("https://image.tmdb.org/t/p/original%s", c.Thumb),
			},
			Character: c.Character,
			Order:     c.Order,
		})
	}

	for _, c := range tmdbObject.Credits.Crew {
		movie.CrewList = append(movie.CrewList, models.Crew{
			Artist: models.Artist{
				Name:         c.Name,
				OriginalName: c.OriginalName,
				Thumb:        fmt.Sprintf("https://image.tmdb.org/t/p/original%s", c.Thumb),
			},
			Job:        c.Job,
			Department: c.Department,
		})
	}

	return nil
}
