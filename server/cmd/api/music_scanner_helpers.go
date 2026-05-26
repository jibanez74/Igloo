package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"igloo/cmd/internal/database"
	"igloo/cmd/internal/helpers"
	spotifyapi "igloo/cmd/internal/spotify"
	"strings"

	spotifylib "github.com/zmb3/spotify/v2"
)

func generateMusicianSummary(artist *spotifylib.FullArtist) string {
	var parts []string

	parts = append(parts, artist.Name)

	if len(artist.Genres) > 0 {
		maxGenres := 3
		if len(artist.Genres) < maxGenres {
			maxGenres = len(artist.Genres)
		}
		genreStr := strings.Join(artist.Genres[:maxGenres], ", ")
		parts = append(parts, fmt.Sprintf("known for %s", genreStr))
	}

	pop := artist.Popularity
	switch {
	case pop >= 80:
		parts = append(parts, "is a globally recognized artist")
	case pop >= 60:
		parts = append(parts, "is a popular artist")
	case pop >= 40:
		parts = append(parts, "has a dedicated following")
	case pop >= 20:
		parts = append(parts, "is an emerging artist")
	default:
		parts = append(parts, "is an independent artist")
	}

	followers := artist.Followers.Count
	switch {
	case followers >= 10_000_000:
		parts = append(parts, fmt.Sprintf("with over %dM followers on Spotify", followers/1_000_000))
	case followers >= 1_000_000:
		parts = append(parts, fmt.Sprintf("with %.1fM followers on Spotify", float64(followers)/1_000_000))
	case followers >= 100_000:
		parts = append(parts, fmt.Sprintf("with %dK followers on Spotify", followers/1_000))
	case followers >= 1_000:
		parts = append(parts, fmt.Sprintf("with %.1fK followers on Spotify", float64(followers)/1_000))
	default:
		parts = append(parts, fmt.Sprintf("with %d followers on Spotify", followers))
	}

	return strings.Join(parts, " ") + "."
}

func (app *Application) getOrCreateMusician(ctx context.Context, qtx *database.Queries, name, sortName string) (*database.Musician, error) {
	var spotifyErr error

	if app.Spotify != nil {
		artist, err := app.Spotify.SearchArtistByName(ctx, name)
		if err == nil && artist != nil {
			existing, err := qtx.GetMusicianBySpotifyID(ctx, sql.NullString{String: artist.ID.String(), Valid: true})
			if err == nil {
				app.processSpotifyGenres(ctx, qtx, existing.ID, artist.Genres)
				app.recordSpotifyMatch(ctx, qtx, spotifyMatchEntityMusician, existing.ID, artist.ID.String())
				return &existing, nil
			}
			if !errors.Is(err, sql.ErrNoRows) {
				return nil, err
			}

			var thumb sql.NullString
			if len(artist.Images) > 0 {
				thumb = sql.NullString{String: artist.Images[0].URL, Valid: true}
			}

			summary := generateMusicianSummary(artist)

			musician, err := qtx.UpsertMusician(ctx, database.UpsertMusicianParams{
				Name:              name,
				SortName:          sortName,
				Summary:           sql.NullString{String: summary, Valid: true},
				SpotifyPopularity: helpers.NullFloat64(float64(artist.Popularity)),
				SpotifyFollowers:  helpers.NullInt64(int64(artist.Followers.Count)),
				SpotifyID:         sql.NullString{String: artist.ID.String(), Valid: true},
				Thumb:             thumb,
			})
			if err != nil {
				return nil, err
			}

			app.processSpotifyGenres(ctx, qtx, musician.ID, artist.Genres)
			app.recordSpotifyMatch(ctx, qtx, spotifyMatchEntityMusician, musician.ID, artist.ID.String())

			return &musician, nil
		}
		if err != nil {
			spotifyErr = err
		}
	}

	musician, err := qtx.UpsertMusician(ctx, database.UpsertMusicianParams{
		Name:     name,
		SortName: sortName,
	})
	if err != nil {
		return nil, err
	}
	if spotifyErr != nil {
		app.recordSpotifyMatchFailure(ctx, qtx, spotifyMatchEntityMusician, musician.ID, spotifyErr)
	}
	return &musician, nil
}

func (app *Application) processSpotifyGenres(ctx context.Context, qtx *database.Queries, musicianID int64, spotifyGenres []string) {
	for _, genreTag := range spotifyGenres {
		genre, err := qtx.GetOrCreateGenre(ctx, database.GetOrCreateGenreParams{
			Tag:       genreTag,
			GenreType: "music",
		})
		if err != nil {
			app.Logger.Warn("failed to get/create Spotify genre",
				"error", err,
				"genre", genreTag,
			)
			continue
		}

		err = qtx.UpsertMusicianGenre(ctx, database.UpsertMusicianGenreParams{
			MusicianID: musicianID,
			GenreID:    genre.ID,
		})
		if err != nil {
			app.Logger.Warn("failed to create musician-genre relationship for Spotify genre",
				"error", err,
				"musician_id", musicianID,
				"genre_id", genre.ID,
				"genre", genreTag,
			)
		}
	}
}

func (app *Application) processSpotifyAlbumGenres(ctx context.Context, qtx *database.Queries, albumID int64, spotifyGenres []string) {
	for _, genreTag := range spotifyGenres {
		genre, err := qtx.GetOrCreateGenre(ctx, database.GetOrCreateGenreParams{
			Tag:       genreTag,
			GenreType: "music",
		})
		if err != nil {
			app.Logger.Warn("failed to get/create Spotify genre for album",
				"error", err,
				"genre", genreTag,
			)
			continue
		}

		err = qtx.UpsertAlbumGenre(ctx, database.UpsertAlbumGenreParams{
			AlbumID: albumID,
			GenreID: genre.ID,
		})
		if err != nil {
			app.Logger.Warn("failed to create album-genre relationship for Spotify genre",
				"error", err,
				"album_id", albumID,
				"genre_id", genre.ID,
				"genre", genreTag,
			)
		}
	}
}

func (app *Application) getOrCreateAlbum(ctx context.Context, qtx *database.Queries, input albumScanInput) (*database.Album, error) {
	title := strings.TrimSpace(input.Title)
	sortTitle := strings.TrimSpace(input.SortTitle)
	albumArtist := strings.TrimSpace(input.AlbumArtist)
	if sortTitle == "" {
		sortTitle = title
	}

	var spotifyErr error
	if app.Spotify != nil {
		album, err := app.getOrCreateSpotifyAlbum(ctx, qtx, input)
		if err == nil && album != nil {
			return album, nil
		}
		if err != nil {
			_, isMatchErr := spotifyapi.AsMatchError(err)
			if !isMatchErr {
				return nil, err
			}
			spotifyErr = err
		}
	}

	params := database.UpsertAlbumParams{
		Title:     title,
		SortTitle: sortTitle,
	}
	if albumArtist != "" {
		params.Musician = sql.NullString{String: albumArtist, Valid: true}
	}

	album, err := qtx.UpsertAlbum(ctx, params)
	if err != nil {
		return nil, err
	}
	album, err = app.resolveAlbumCoverIfMissing(ctx, qtx, album, "", input)
	if err != nil {
		return nil, err
	}
	if spotifyErr != nil {
		app.recordSpotifyMatchFailure(ctx, qtx, spotifyMatchEntityAlbum, album.ID, spotifyErr)
	}
	return &album, nil
}

func (app *Application) getOrCreateSpotifyAlbum(ctx context.Context, qtx *database.Queries, input albumScanInput) (*database.Album, error) {
	if app.Spotify == nil {
		return nil, nil
	}

	title := strings.TrimSpace(input.Title)
	sortTitle := strings.TrimSpace(input.SortTitle)
	albumArtist := strings.TrimSpace(input.AlbumArtist)
	if sortTitle == "" {
		sortTitle = title
	}

	albumDetails, err := app.Spotify.SearchAndGetAlbumDetails(ctx, spotifyapi.AlbumSearchInput{
		Title:       title,
		Artist:      albumArtist,
		Year:        input.Year,
		TrackTitles: input.TrackTitles,
	})
	if err != nil {
		return nil, err
	}
	if albumDetails == nil {
		return nil, nil
	}

	var spotifyCoverURL string
	if len(albumDetails.Images) > 0 {
		spotifyCoverURL = albumDetails.Images[0].URL
	}

	existing, err := qtx.GetAlbumBySpotifyID(ctx, sql.NullString{String: albumDetails.ID.String(), Valid: true})
	if err == nil {
		app.processSpotifyAlbumGenres(ctx, qtx, existing.ID, albumDetails.Genres)
		app.recordSpotifyMatch(ctx, qtx, spotifyMatchEntityAlbum, existing.ID, albumDetails.ID.String())
		existing, err = app.resolveAlbumCoverIfMissing(ctx, qtx, existing, spotifyCoverURL, input)
		if err != nil {
			return nil, err
		}
		return &existing, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return nil, err
	}

	params := database.UpsertAlbumParams{
		Title:             title,
		SortTitle:         sortTitle,
		SpotifyID:         sql.NullString{String: albumDetails.ID.String(), Valid: true},
		SpotifyPopularity: helpers.NullFloat64(float64(albumDetails.Popularity)),
		TotalTracks:       helpers.NullInt64(int64(albumDetails.TotalTracks)),
	}

	releaseDate := albumDetails.ReleaseDateTime()
	if !releaseDate.IsZero() {
		params.ReleaseDate = sql.NullString{String: releaseDate.Format("2006-01-02"), Valid: true}
		params.Year = sql.NullInt64{Int64: int64(releaseDate.Year()), Valid: true}
	}

	if albumArtist != "" {
		params.Musician = sql.NullString{String: albumArtist, Valid: true}
	}

	if spotifyCoverURL != "" && (app.Settings == nil || !app.Settings.DownloadImages) {
		params.Cover = sql.NullString{String: spotifyCoverURL, Valid: true}
	}

	album, err := qtx.UpsertAlbum(ctx, params)
	if err != nil {
		return nil, err
	}
	app.processSpotifyAlbumGenres(ctx, qtx, album.ID, albumDetails.Genres)
	app.recordSpotifyMatch(ctx, qtx, spotifyMatchEntityAlbum, album.ID, albumDetails.ID.String())
	album, err = app.resolveAlbumCoverIfMissing(ctx, qtx, album, spotifyCoverURL, input)
	if err != nil {
		return nil, err
	}
	return &album, nil
}
