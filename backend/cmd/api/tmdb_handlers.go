package main

import (
	"fmt"
	"igloo/cmd/internal/database"
	"igloo/cmd/internal/helpers"
	"mime"
	"os"
	"path/filepath"
	"strings"
	"unicode/utf8"

	"github.com/gofiber/fiber/v2"
	"github.com/jackc/pgx/v5/pgtype"
)

type createRequest struct {
	FilePath string `json:"file_path"`
	TmdbID   string `json:"tmdb_id"`
}

func (app *application) createTmdbMovie(c *fiber.Ctx) error {
	var request createRequest

	err := c.BodyParser(&request)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": fmt.Sprintf("unable to parse request body: %v", err),
		})
	}

	if request.FilePath == "" || request.TmdbID == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "file path and tmdb id are required",
		})
	}

	var movie database.CreateMovieParams

	fileInfo, err := os.Stat(request.FilePath)
	if err != nil {
		if os.IsNotExist(err) {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
				"error": fmt.Sprintf("unable to find the file at %s", request.FilePath),
			})
		}

		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": fmt.Sprintf("unable to stat the file at %s: %v", request.FilePath, err),
		})
	}

	movie.FilePath = request.FilePath
	movie.Size = int64(fileInfo.Size())
	movie.FileName = fileInfo.Name()
	movie.Container = filepath.Ext(movie.FilePath)

	contentType := mime.TypeByExtension(movie.Container)
	if contentType == "" {
		contentType = "application/octet-stream"
	}
	movie.ContentType = contentType

	movieInfo, err := app.tmdb.GetTmdbMovieByID(&request.TmdbID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": fmt.Sprintf("unable to get movie info from tmdb: %v", err),
		})
	}

	movie.Title = movieInfo.Title
	movie.TagLine = movieInfo.TagLine
	movie.Adult = movieInfo.Adult
	movie.Summary = movieInfo.Summary
	movie.Budget = movieInfo.Budget
	movie.Revenue = movieInfo.Revenue
	movie.RunTime = movieInfo.RunTime
	movie.AudienceRating = movieInfo.AudienceRating
	movie.TmdbID = request.TmdbID
	movie.ImdbID = movieInfo.ImdbID

	if len(movieInfo.SpokenLanguages) > 0 {
		var languages []string

		for _, l := range movieInfo.SpokenLanguages {
			if utf8.ValidString(l.Name) {
				languages = append(languages, l.Name)
			} else {
				validLanguageName := helpers.SanitizeString(l.Name)
				languages = append(languages, validLanguageName)
			}
		}

		movie.SpokenLanguages = strings.Join(languages, ", ")
	} else {
		movie.SpokenLanguages = "unknown"
	}

	releaseDate, err := helpers.FormatDate(movieInfo.ReleaseDate)
	if err == nil {
		movie.ReleaseDate = pgtype.Date{
			Time:  releaseDate,
			Valid: true,
		}

		movie.Year = int32(releaseDate.Year())
	}

	if movieInfo.Thumb != "" {
		movie.Thumb = fmt.Sprintf("https://image.tmdb.org/t/p/w500%s", movieInfo.Thumb)
	}

	if movieInfo.Art != "" {
		movie.Art = fmt.Sprintf("https://image.tmdb.org/t/p/w1280%s", movieInfo.Art)
	}

	tx, err := app.db.Begin(c.Context())
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": fmt.Sprintf("failed to start transaction: %v", err),
		})
	}
	defer tx.Rollback(c.Context())

	qtx := app.queries.WithTx(tx)

	newMovie, err := qtx.CreateMovie(c.Context(), movie)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": fmt.Sprintf("unable to create the movie %s: %v", movie.Title, err),
		})
	}

	if len(movieInfo.Genres) > 0 {
		for _, g := range movieInfo.Genres {
			genre, err := qtx.GetOrCreateGenre(c.Context(), database.GetOrCreateGenreParams{
				Tag: g.Tag,
			})

			if err != nil {
				return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
					"error": fmt.Sprintf("unable to create genre %s for the movie %s: %v", g.Tag, movie.Title, err),
				})
			}

			err = qtx.AddMovieGenre(c.Context(), database.AddMovieGenreParams{
				MovieID: newMovie.ID,
				GenreID: genre.ID,
			})

			if err != nil {
				return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
					"error": fmt.Sprintf("unable to link genre %s to the movie %s: %v", g.Tag, movie.Title, err),
				})
			}
		}
	}

	if len(movieInfo.Studios) > 0 {
		for _, s := range movieInfo.Studios {
			studio, err := qtx.GetOrCreateStudio(c.Context(), database.GetOrCreateStudioParams{
				TmdbID:  s.ID,
				Name:    s.Name,
				Country: s.Country,
				Logo:    fmt.Sprintf("https://image.tmdb.org/t/p/w500%s", s.Logo),
			})

			if err != nil {
				return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
					"error": fmt.Sprintf("unable to create studio %s for the movie %s: %v", s.Name, movie.Title, err),
				})
			}

			err = qtx.AddMovieStudio(c.Context(), database.AddMovieStudioParams{
				MovieID:  newMovie.ID,
				StudioID: studio.ID,
			})

			if err != nil {
				return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
					"error": fmt.Sprintf("unable to link studio %s to the movie %s: %v", s.Name, movie.Title, err),
				})
			}
		}
	}

	if len(movieInfo.Credits.Cast) > 0 {
		for _, a := range movieInfo.Credits.Cast {

			artist, err := qtx.GetOrCreateArtist(c.Context(), database.GetOrCreateArtistParams{
				TmdbID:       int32(a.ID),
				Name:         a.Name,
				OriginalName: a.OriginalName,
				Thumb:        fmt.Sprintf("https://image.tmdb.org/t/p/w500%s", a.Thumb),
			})

			if err != nil {
				return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
					"error": fmt.Sprintf("unable to create artist %s for the movie %s: %v", a.Name, movie.Title, err),
				})
			}

			_, err = qtx.CreateCastMember(c.Context(), database.CreateCastMemberParams{
				ArtistID: pgtype.Int4{
					Int32: artist.ID,
					Valid: true,
				},
				MovieID: pgtype.Int4{
					Int32: newMovie.ID,
					Valid: true,
				},
				Character: a.Character,
				SortOrder: a.SortOrder,
			})

			if err != nil {
				return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
					"error": fmt.Sprintf("unable to create cast member %s for the movie %s: %v", a.Name, movie.Title, err),
				})
			}
		}
	}

	if len(movieInfo.Credits.Crew) > 0 {
		for _, a := range movieInfo.Credits.Crew {
			artist, err := qtx.GetOrCreateArtist(c.Context(), database.GetOrCreateArtistParams{
				TmdbID:       int32(a.ID),
				Name:         a.Name,
				OriginalName: a.OriginalName,
				Thumb:        fmt.Sprintf("https://image.tmdb.org/t/p/w500%s", a.Thumb),
			})

			if err != nil {
				return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
					"error": fmt.Sprintf("unable to create artist %s for the movie %s: %v", a.Name, movie.Title, err),
				})
			}

			_, err = qtx.CreateCrewMember(c.Context(), database.CreateCrewMemberParams{
				ArtistID: pgtype.Int4{
					Int32: artist.ID,
					Valid: true,
				},
				MovieID: pgtype.Int4{
					Int32: newMovie.ID,
					Valid: true,
				},
				Job:        a.Job,
				Department: a.Department,
			})

			if err != nil {
				return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
					"error": fmt.Sprintf("unable to create crew member %s for the movie %s: %v", a.Name, movie.Title, err),
				})
			}
		}
	}

	if len(movieInfo.Videos.Results) > 0 {
		for _, v := range movieInfo.Videos.Results {
			if v.Site != "YouTube" {
				continue
			}

			videoUrl := fmt.Sprintf("https://www.youtube.com/watch?v=%s", v.Key)

			_, err = qtx.CreateMovieExtra(c.Context(), database.CreateMovieExtraParams{
				Title: v.Name,
				Url:   videoUrl,
				Kind:  v.Type,
				MovieID: pgtype.Int4{
					Int32: newMovie.ID,
					Valid: true,
				},
			})

			if err != nil {
				return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
					"error": fmt.Sprintf("unable to create movie extra %s for the movie %s: %v", v.Name, movie.Title, err),
				})
			}
		}
	}

	probeResult, err := app.ffprobe.GetMovieMetadata(&movie.FilePath)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": fmt.Sprintf("unable to get movie metadata for the movie %s: %v", movie.Title, err),
		})
	}

	for _, v := range probeResult.VideoList {
		v.MovieID = pgtype.Int4{
			Int32: newMovie.ID,
			Valid: true,
		}

		_, err := qtx.CreateVideoStream(c.Context(), v)
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"error": fmt.Sprintf("unable to create video stream for the movie %s: %v", movie.Title, err),
			})
		}
	}

	if len(probeResult.AudioList) > 0 {
		for _, a := range probeResult.AudioList {
			a.MovieID = pgtype.Int4{
				Int32: newMovie.ID,
				Valid: true,
			}

			if len(a.Codec) > 20 {
				msg := fmt.Sprintf("audio codec is too long: %s", a.Codec)

				return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
					"error": msg,
				})
			}

			_, err := qtx.CreateAudioStream(c.Context(), a)
			if err != nil {
				return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
					"error": fmt.Sprintf("unable to create audio stream for the movie %s: %v", movie.Title, err),
				})
			}
		}
	}

	if len(probeResult.SubtitleList) > 0 {
		for _, s := range probeResult.SubtitleList {
			s.MovieID = pgtype.Int4{
				Int32: newMovie.ID,
				Valid: true,
			}

			_, err = qtx.CreateSubtitle(c.Context(), s)
			if err != nil {
				return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
					"error": fmt.Sprintf("unable to create subtitle for the movie %s: %v", movie.Title, err),
				})
			}
		}
	}

	err = tx.Commit(c.Context())
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": fmt.Sprintf("unable to commit the transaction for the movie %s: %v", movie.Title, err),
		})
	}

	return c.Status(fiber.StatusCreated).JSON(fiber.Map{
		"message": fmt.Sprintf("successfully added the movie %s", movie.Title),
	})
}
