package main

import (
	"context"
	"database/sql"
	"fmt"
	"igloo/cmd/internal/database"
	"igloo/cmd/internal/helpers"
	spotifyapi "igloo/cmd/internal/spotify"
	"path/filepath"
	"strconv"
	"strings"
)

func (app *Application) processTrackFile(ctx context.Context, qtx *database.Queries, path, ext string) error {
	info, err := app.Ffprobe.GetMetadata(path)
	if err != nil {
		return fmt.Errorf("ffprobe failed: %w", err)
	}

	params := database.UpsertTrackParams{
		FilePath: path,
		FileName: filepath.Base(path),
	}

	if info.Format.Tags.Title != "" {
		params.Title = info.Format.Tags.Title
	} else {
		params.Title = filepath.Base(path)
	}

	if info.Format.Tags.SortName != "" {
		params.SortTitle = info.Format.Tags.SortName
	} else {
		params.SortTitle = params.Title
	}

	params.Container = ext

	mimeType, ok := helpers.AudioMimeTypes[ext]
	if ok {
		params.MimeType = mimeType
	}

	if info.Format.Size != "" {
		size, err := strconv.ParseInt(info.Format.Size, 10, 64)
		if err == nil {
			params.Size = size
		}
	}

	if info.Format.Duration != "" {
		duration, err := helpers.ParseDurationMs(info.Format.Duration)
		if err == nil {
			params.Duration = duration
		}
	}

	if info.Format.Tags.Track != "" {
		index, err := helpers.ParseSlashNumber(info.Format.Tags.Track)
		if err == nil {
			params.TrackIndex = index
		}
	}

	if info.Format.BitRate != "" {
		params.BitRate = helpers.ParseBitRate(info.Format.BitRate)
	}

	if info.Format.Tags.Disc != "" {
		disc, err := helpers.ParseSlashNumber(info.Format.Tags.Disc)
		if err == nil {
			params.Disc = disc
		}
	}

	params.Copyright = helpers.NullString(info.Format.Tags.Copyright)
	params.Composer = helpers.NullString(info.Format.Tags.Composer)

	trackYear := 0
	if info.Format.Tags.Date != "" {
		date, err := helpers.ParseDate(info.Format.Tags.Date)
		if err == nil {
			params.ReleaseDate = sql.NullString{String: date.Format("2006-01-02"), Valid: true}
			params.Year = sql.NullInt64{Int64: int64(date.Year()), Valid: true}
			trackYear = date.Year()
		}
	}

	var musicianID sql.NullInt64
	var allMusicianIDs []int64

	if info.Format.Tags.Artist != "" {
		sortArtist := info.Format.Tags.SortArtist
		if sortArtist == "" {
			sortArtist = info.Format.Tags.Artist
		}

		artistTag := info.Format.Tags.Artist
		isCompound := strings.Contains(artistTag, " & ") || strings.Contains(artistTag, ", ")

		// Probe compound-looking artist names before creating a possibly bogus combined artist.
		splitArtists := false
		if isCompound && app.Spotify != nil {
			_, err := app.Spotify.SearchArtistByName(ctx, artistTag)
			if err != nil {
				splitArtists = shouldSplitCompoundArtistCredits(err)
			}
		}

		parts := splitCompoundArtistCredits(artistTag)
		if len(parts) < 2 {
			splitArtists = false
		}

		if !splitArtists {
			musician, err := app.getOrCreateMusician(ctx, qtx, artistTag, sortArtist)
			if err != nil {
				return fmt.Errorf("musician failed: %w", err)
			}
			musicianID = sql.NullInt64{Int64: musician.ID, Valid: true}
			allMusicianIDs = append(allMusicianIDs, musician.ID)
		} else {
			for _, part := range parts {
				m, err := app.getOrCreateMusician(ctx, qtx, part, part)
				if err != nil {
					app.Logger.Warn("failed to resolve compound artist part", "part", part, "error", err)
					return fmt.Errorf("compound musician failed for %q: %w", part, err)
				}
				if !musicianID.Valid {
					musicianID = sql.NullInt64{Int64: m.ID, Valid: true}
				}
				allMusicianIDs = append(allMusicianIDs, m.ID)
			}
		}
	}
	params.MusicianID = musicianID

	var albumID sql.NullInt64

	if info.Format.Tags.Album != "" {
		sortAlbum := info.Format.Tags.SortAlbum
		if sortAlbum == "" {
			sortAlbum = info.Format.Tags.Album
		}

		effectiveAlbumArtist := info.Format.Tags.AlbumArtist
		if effectiveAlbumArtist == "" {
			effectiveAlbumArtist = info.Format.Tags.Artist
		}

		album, err := app.getOrCreateAlbum(ctx, qtx, albumScanInput{
			Title:            info.Format.Tags.Album,
			SortTitle:        sortAlbum,
			AlbumArtist:      effectiveAlbumArtist,
			Year:             trackYear,
			TrackTitles:      []string{params.Title},
			TrackPaths:       []string{path},
			CurrentTrackPath: path,
			CurrentMetadata:  info,
		})
		if err != nil {
			return fmt.Errorf("album failed: %w", err)
		}

		albumID = sql.NullInt64{Int64: album.ID, Valid: true}
	}
	params.AlbumID = albumID

	if albumID.Valid {
		for _, mID := range allMusicianIDs {
			err := qtx.CreateMusicianAlbum(ctx, database.CreateMusicianAlbumParams{
				MusicianID: mID,
				AlbumID:    albumID.Int64,
			})
			if err != nil {
				app.Logger.Warn("failed to create musician-album relationship",
					"error", err,
					"musician_id", mID,
					"album_id", albumID.Int64,
				)
			}
		}
	}

	for _, stream := range info.Streams {
		if stream.CodecType == "audio" {
			params.Codec = stream.CodecName
			params.Profile = stream.Profile

			if stream.ChannelLayout != "" {
				params.Channels = stream.ChannelLayout
				params.ChannelLayout = stream.ChannelLayout
			} else {
				params.Channels = strconv.Itoa(stream.Channels)
				params.ChannelLayout = strconv.Itoa(stream.Channels)
			}

			if stream.Tags.Language != "" {
				params.Language = sql.NullString{String: stream.Tags.Language, Valid: true}
			}

			break
		}
	}

	track, err := qtx.UpsertTrack(ctx, params)
	if err != nil {
		return fmt.Errorf("upsert track failed: %w", err)
	}

	if info.Format.Tags.Genre != "" {
		genre, err := qtx.GetOrCreateGenre(ctx, database.GetOrCreateGenreParams{
			Tag:       info.Format.Tags.Genre,
			GenreType: "music",
		})

		if err != nil {
			return fmt.Errorf("genre failed: %w", err)
		}

		err = qtx.DeleteTrackGenresExcept(ctx, database.DeleteTrackGenresExceptParams{
			TrackID: track.ID,
			GenreID: genre.ID,
		})

		if err != nil {
			return fmt.Errorf("delete stale genres failed: %w", err)
		}

		err = qtx.CreateTrackGenre(ctx, database.CreateTrackGenreParams{
			TrackID: track.ID,
			GenreID: genre.ID,
		})

		if err != nil {
			return fmt.Errorf("track-genre relationship failed: %w", err)
		}

		if musicianID.Valid {
			err = qtx.UpsertMusicianGenre(ctx, database.UpsertMusicianGenreParams{
				MusicianID: musicianID.Int64,
				GenreID:    genre.ID,
			})
			if err != nil {
				app.Logger.Warn("failed to create musician-genre relationship",
					"error", err,
					"musician_id", musicianID.Int64,
					"genre_id", genre.ID,
				)
			}
		}

		if albumID.Valid {
			err = qtx.UpsertAlbumGenre(ctx, database.UpsertAlbumGenreParams{
				AlbumID: albumID.Int64,
				GenreID: genre.ID,
			})
			if err != nil {
				app.Logger.Warn("failed to create album-genre relationship",
					"error", err,
					"album_id", albumID.Int64,
					"genre_id", genre.ID,
				)
			}
		}
	}

	return nil
}

func shouldSplitCompoundArtistCredits(err error) bool {
	matchErr, ok := spotifyapi.AsMatchError(err)
	if !ok {
		return false
	}

	return matchErr.Info.Reason == "no_results" || matchErr.Info.Reason == "score_below_threshold"
}

func splitCompoundArtistCredits(artistTag string) []string {
	normalized := strings.ReplaceAll(artistTag, " & ", ", ")
	rawParts := strings.Split(normalized, ", ")
	parts := make([]string, 0, len(rawParts))
	seen := make(map[string]struct{}, len(rawParts))

	for _, rawPart := range rawParts {
		part := strings.TrimSpace(rawPart)
		if part == "" {
			continue
		}

		if _, exists := seen[part]; exists {
			continue
		}

		seen[part] = struct{}{}
		parts = append(parts, part)
	}

	return parts
}
