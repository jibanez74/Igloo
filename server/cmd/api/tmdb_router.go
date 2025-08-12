package main

import (
	"context"
	"errors"
	"fmt"
	"igloo/cmd/internal/database"
	"igloo/cmd/internal/helpers"
	"igloo/cmd/internal/tmdb"
	"math/big"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"unicode/utf8"

	"github.com/jackc/pgx/v5/pgtype"
)

func (app *Application) createTmdbMovie(w http.ResponseWriter, r *http.Request) {
	var request CreateTmdbMovieRequest

	err := helpers.ReadJSON(w, r, &request, 0)
	if err != nil {
		helpers.ErrorJSON(w, err)
		return
	}

	if request.FilePath == "" || request.TmdbID == "" {
		helpers.ErrorJSON(w, errors.New("file path and tmdb id aare required"), http.StatusBadRequest)
		return
	}

	var movie database.CreateMovieParams

	fileInfo, err := os.Stat(request.FilePath)
	if err != nil {
		if os.IsNotExist(err) {
			helpers.ErrorJSON(w, fmt.Errorf("no movie file found at %s", request.FilePath), http.StatusNotFound)
		} else {
			helpers.ErrorJSON(w, fmt.Errorf("unable to stat movie file at %w", err))
		}

		return
	}

	movie.FilePath = request.FilePath
	movie.Size = int64(fileInfo.Size())
	movie.Container = filepath.Ext(movie.FilePath)

	contentType := mime.TypeByExtension(movie.Container)
	if contentType == "" {
		contentType = "application/octet-stream"
	}
	movie.ContentType = contentType

	var movieInfo tmdb.TmdbMovie

	err = app.Tmdb.GetTmdbMovieByID(&movieInfo)
	if err != nil {
		helpers.ErrorJSON(w, fmt.Errorf("unable to get movie information from tmdb: %w", err))
		return
	}

	movie.Title = movieInfo.Title
	movie.TagLine = movieInfo.TagLine
	movie.Adult = movieInfo.Adult
	movie.Summary = movieInfo.Summary
	movie.Budget = movieInfo.Budget
	movie.Revenue = movieInfo.Revenue
	movie.RunTime = movieInfo.RunTime
	movie.AudienceRating = pgtype.Numeric{
		Int:   big.NewInt(int64(movieInfo.AudienceRating * 10)), // Convert to integer (e.g., 8.5 -> 85)
		Exp:   -1,                                               // Decimal places (-1 means 1 decimal place)
		Valid: true,
	}
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

	}

	if movieInfo.Thumb != "" {
		movie.Thumb = fmt.Sprintf("https://image.tmdb.org/t/p/w500%s", movieInfo.Thumb)

		if app.Settings.DownloadImages {
			fileName := fmt.Sprintf("thumb_%s.jpg", movie.TmdbID)
			outputDir := filepath.Join(app.Settings.MoviesImgDir, request.TmdbID)

			err = helpers.SaveTmdbImage(movie.Thumb, outputDir, fileName)
			if err == nil {
				movie.Thumb = fmt.Sprintf("/static/images/movies/%s/%s", request.TmdbID, fileName)
			}
		}
	}

	if movieInfo.Art != "" {
		movie.Art = fmt.Sprintf("https://image.tmdb.org/t/p/w1280%s", movieInfo.Art)

		if app.Settings.DownloadImages {
			fileName := fmt.Sprintf("art_%s.jpg", movie.TmdbID)
			outputDir := filepath.Join(app.Settings.MoviesImgDir, request.TmdbID)

			err = helpers.SaveTmdbImage(movie.Art, outputDir, fileName)
			if err == nil {
				movie.Art = fmt.Sprintf("/static/images/movies/%s/%s", request.TmdbID, fileName)
			}
		}
	}

	tx, err := app.Db.Begin(context.Background())
	if err != nil {
		helpers.ErrorJSON(w, fmt.Errorf("failed to start transaction: %v", err))
		return
	}
	defer tx.Rollback(context.Background())

	qtx := app.Queries.WithTx(tx)

	newMovie, err := qtx.CreateMovie(context.Background(), movie)
	if err != nil {
		helpers.ErrorJSON(w, fmt.Errorf("unable to create the movie %s: %v", movie.Title, err))
		return
	}

	if len(movieInfo.Genres) > 0 {
		for _, g := range movieInfo.Genres {
			genre, err := qtx.GetOrCreateGenre(context.Background(), database.GetOrCreateGenreParams{
				Tag:       g.Tag,
				GenreType: "movie",
			})

			if err != nil {
				helpers.ErrorJSON(w, fmt.Errorf("unable to create genre %s for the movie %s: %v", g.Tag, movie.Title, err))
				return
			}

			err = qtx.AddMovieGenre(context.Background(), database.AddMovieGenreParams{
				MovieID: newMovie.ID,
				GenreID: genre.ID,
			})

			if err != nil {
				helpers.ErrorJSON(w, fmt.Errorf("unable to link genre %s to the movie %s: %v", g.Tag, movie.Title, err))
			}
		}
	}

	if len(movieInfo.Studios) > 0 {
		for _, s := range movieInfo.Studios {
			logoPath := fmt.Sprintf("https://image.tmdb.org/t/p/w500%s", s.Logo)

			if app.Settings.DownloadImages {
				fileName := fmt.Sprintf("logo_%d.jpg", s.ID)
				err = helpers.SaveTmdbImage(logoPath, app.Settings.StudiosImgDir, fileName)
				if err == nil {
					logoPath = fmt.Sprintf("/static/images/studios/%s", fileName)
				}
			}

			studio, err := qtx.GetOrCreateStudio(context.Background(), database.GetOrCreateStudioParams{
				TmdbID:  s.ID,
				Name:    s.Name,
				Country: s.Country,
				Logo:    logoPath,
			})

			if err != nil {
				helpers.ErrorJSON(w, fmt.Errorf("unable to create studio %s for the movie %s: %v", s.Name, movie.Title, err))
				return
			}

			err = qtx.AddMovieStudio(context.Background(), database.AddMovieStudioParams{
				MovieID:  newMovie.ID,
				StudioID: studio.ID,
			})

			if err != nil {
				helpers.ErrorJSON(w, fmt.Errorf("unable to link studio %s to the movie %s: %v", s.Name, movie.Title, err))
				return
			}
		}
	}

	if len(movieInfo.Credits.Cast) > 0 {
		for _, a := range movieInfo.Credits.Cast {
			thumbPath := fmt.Sprintf("https://image.tmdb.org/t/p/w500%s", a.Thumb)

			if app.Settings.DownloadImages {
				fileName := fmt.Sprintf("artist_%d.jpg", a.ID)
				err = helpers.SaveTmdbImage(thumbPath, app.Settings.ArtistsImgDir, fileName)
				if err == nil {
					thumbPath = fmt.Sprintf("/static/images/artists/%s", fileName)
				}
			}

			artist, err := qtx.GetOrCreateArtist(context.Background(), database.GetOrCreateArtistParams{
				TmdbID:       int32(a.ID),
				Name:         a.Name,
				OriginalName: a.OriginalName,
				Thumb:        thumbPath,
			})

			if err != nil {
				helpers.ErrorJSON(w, fmt.Errorf("unable to create artist %s for the movie %s: %v", a.Name, movie.Title, err))
				return
			}

			_, err = qtx.CreateCastMember(context.Background(), database.CreateCastMemberParams{
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
				helpers.ErrorJSON(w, fmt.Errorf("unable to create cast member %s for the movie %s: %v", a.Name, movie.Title, err))
				return
			}
		}
	}

	if len(movieInfo.Credits.Crew) > 0 {
		for _, a := range movieInfo.Credits.Crew {
			thumbPath := fmt.Sprintf("https://image.tmdb.org/t/p/w500%s", a.Thumb)

			if app.Settings.DownloadImages {
				fileName := fmt.Sprintf("artist_%d.jpg", a.ID)
				err = helpers.SaveTmdbImage(thumbPath, app.Settings.ArtistsImgDir, fileName)
				if err == nil {
					thumbPath = fmt.Sprintf("/static/images/artists/%s", fileName)
				}
			}

			artist, err := qtx.GetOrCreateArtist(context.Background(), database.GetOrCreateArtistParams{
				TmdbID:       int32(a.ID),
				Name:         a.Name,
				OriginalName: a.OriginalName,
				Thumb:        thumbPath,
			})

			if err != nil {
				helpers.ErrorJSON(w, fmt.Errorf("unable to create artist %s for the movie %s: %v", a.Name, movie.Title, err))
				return
			}

			_, err = qtx.CreateCrewMember(context.Background(), database.CreateCrewMemberParams{
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
				helpers.ErrorJSON(w, fmt.Errorf("unable to create crew member %s for the movie %s: %v", a.Name, movie.Title, err))
				return
			}
		}
	}

	if len(movieInfo.Videos.Results) > 0 {
		for _, v := range movieInfo.Videos.Results {
			if v.Site != "YouTube" {
				continue
			}

			videoUrl := fmt.Sprintf("https://www.youtube.com/watch?v=%s", v.Key)

			_, err = qtx.CreateMovieExtra(context.Background(), database.CreateMovieExtraParams{
				Title: v.Name,
				Url:   videoUrl,
				Kind:  v.Type,
				MovieID: pgtype.Int4{
					Int32: newMovie.ID,
					Valid: true,
				},
			})

			if err != nil {
				helpers.ErrorJSON(w, fmt.Errorf("unable to create movie extra %s for the movie %s: %v", v.Name, movie.Title, err))
				return
			}
		}
	}

	probeResult, err := app.Ffprobe.GetMovieMetadata(&movie.FilePath)
	if err != nil {
		helpers.ErrorJSON(w, fmt.Errorf("unable to get movie metadata for the movie %s: %v", movie.Title, err))
		return
	}

	for _, v := range probeResult.VideoList {
		v.MovieID = pgtype.Int4{
			Int32: newMovie.ID,
			Valid: true,
		}

		_, err := qtx.CreateVideoStream(context.Background(), v)
		if err != nil {
			helpers.ErrorJSON(w, fmt.Errorf("unable to create video stream for the movie %s: %v", movie.Title, err))
			return
		}
	}

	if len(probeResult.AudioList) > 0 {
		for _, a := range probeResult.AudioList {
			a.MovieID = pgtype.Int4{
				Int32: newMovie.ID,
				Valid: true,
			}

			if len(a.Codec) > 20 {
				helpers.ErrorJSON(w, fmt.Errorf("audio codec is too long: %s", a.Codec))
				return
			}

			_, err := qtx.CreateAudioStream(context.Background(), a)
			if err != nil {
				helpers.ErrorJSON(w, fmt.Errorf("unable to create audio stream for the movie %s: %v", movie.Title, err))
				return
			}
		}
	}

	if len(probeResult.SubtitleList) > 0 {
		for _, s := range probeResult.SubtitleList {
			s.MovieID = pgtype.Int4{
				Int32: newMovie.ID,
				Valid: true,
			}

			_, err = qtx.CreateSubtitle(context.Background(), s)
			if err != nil {
				helpers.ErrorJSON(w, fmt.Errorf("unable to create subtitle for the movie %s: %v", movie.Title, err))
				return
			}
		}
	}

	err = tx.Commit(context.Background())
	if err != nil {
		helpers.ErrorJSON(w, fmt.Errorf("unable to commit the transaction for the movie %s: %v", movie.Title, err))
		return
	}

	res := helpers.JSONResponse{
		Error:   false,
		Message: fmt.Sprintf("Created movie %s", movie.Title),
		Data:    movie,
	}

	helpers.WriteJSON(w, http.StatusCreated, res)
}
