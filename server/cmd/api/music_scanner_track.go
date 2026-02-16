package main

import (
	"context"
	"database/sql"
	"fmt"
	"igloo/cmd/internal/database"
	"igloo/cmd/internal/helpers"
	"path/filepath"
	"strconv"
)

// processTrackFile extracts metadata from an audio file and upserts it into the database.
// Handles related entities (musician, album, genre) creation and linking.
func (app *Application) processTrackFile(ctx context.Context, qtx *database.Queries, path, ext string) error {
	info, err := app.Ffprobe.GetMetadata(path)
	if err != nil {
		return fmt.Errorf("ffprobe failed: %w", err)
	}

	params := database.UpsertTrackParams{
		FilePath: path,
		FileName: filepath.Base(path),
	}

	// Title - use filename if not available
	if info.Format.Tags.Title != "" {
		params.Title = info.Format.Tags.Title
	} else {
		params.Title = filepath.Base(path)
	}

	// Sort title - use title if not available
	if info.Format.Tags.SortName != "" {
		params.SortTitle = info.Format.Tags.SortName
	} else {
		params.SortTitle = params.Title
	}

	// Container (file extension) and MIME type
	params.Container = ext

	mimeType, ok := helpers.AudioMimeTypes[ext]
	if ok {
		params.MimeType = mimeType
	}

	// Parse size
	if info.Format.Size != "" {
		size, err := strconv.ParseInt(info.Format.Size, 10, 64)
		if err == nil {
			params.Size = size
		}
	}

	// Parse duration (convert seconds to milliseconds)
	if info.Format.Duration != "" {
		duration, err := helpers.ParseDurationMs(info.Format.Duration)
		if err == nil {
			params.Duration = duration
		}
	}

	// Parse track index from "1/12" format
	if info.Format.Tags.Track != "" {
		index, err := helpers.ParseSlashNumber(info.Format.Tags.Track)
		if err == nil {
			params.TrackIndex = index
		}
	}

	// Parse bit rate
	if info.Format.BitRate != "" {
		params.BitRate = helpers.ParseBitRate(info.Format.BitRate)
	}

	// Parse disc number from "1/2" format
	if info.Format.Tags.Disc != "" {
		disc, err := helpers.ParseSlashNumber(info.Format.Tags.Disc)
		if err == nil {
			params.Disc = disc
		}
	}

	// Optional text fields
	params.Copyright = helpers.NullString(info.Format.Tags.Copyright)
	params.Composer = helpers.NullString(info.Format.Tags.Composer)

	// Parse release date
	if info.Format.Tags.Date != "" {
		date, err := helpers.ParseDate(info.Format.Tags.Date)
		if err == nil {
			params.ReleaseDate = sql.NullString{String: date.Format("2006-01-02"), Valid: true}
			params.Year = sql.NullInt64{Int64: int64(date.Year()), Valid: true}
		}
	}

	// Get or create musician if artist tag exists
	var musicianID sql.NullInt64

	if info.Format.Tags.Artist != "" {
		sortArtist := info.Format.Tags.SortArtist
		if sortArtist == "" {
			sortArtist = info.Format.Tags.Artist
		}

		musician, err := app.getOrCreateMusician(ctx, qtx, info.Format.Tags.Artist, sortArtist)
		if err != nil {
			return fmt.Errorf("musician failed: %w", err)
		}

		musicianID = sql.NullInt64{Int64: musician.ID, Valid: true}
	}
	params.MusicianID = musicianID

	// Get or create album if album tag exists
	var albumID sql.NullInt64

	if info.Format.Tags.Album != "" {
		sortAlbum := info.Format.Tags.SortAlbum
		if sortAlbum == "" {
			sortAlbum = info.Format.Tags.Album
		}

		album, err := app.getOrCreateAlbum(ctx, qtx, info.Format.Tags.Album, sortAlbum, info.Format.Tags.AlbumArtist)
		if err != nil {
			return fmt.Errorf("album failed: %w", err)
		}

		albumID = sql.NullInt64{Int64: album.ID, Valid: true}
	}
	params.AlbumID = albumID

	// Create musician-album relationship if both exist
	if musicianID.Valid && albumID.Valid {
		err := qtx.CreateMusicianAlbum(ctx, database.CreateMusicianAlbumParams{
			MusicianID: musicianID.Int64,
			AlbumID:    albumID.Int64,
		})

		if err != nil {
			return fmt.Errorf("musician-album relationship failed: %w", err)
		}
	}

	// Extract audio stream info (codec, channels, profile, language)
	for _, stream := range info.Streams {
		if stream.CodecType == "audio" {
			params.Codec = stream.CodecName
			params.Profile = stream.Profile

			// Channel info
			if stream.ChannelLayout != "" {
				params.Channels = stream.ChannelLayout
				params.ChannelLayout = stream.ChannelLayout
			} else {
				params.Channels = strconv.Itoa(stream.Channels)
				params.ChannelLayout = strconv.Itoa(stream.Channels)
			}

			// Language from stream tags
			if stream.Tags.Language != "" {
				params.Language = sql.NullString{String: stream.Tags.Language, Valid: true}
			}

			break // Only process first audio stream
		}
	}

	track, err := qtx.UpsertTrack(ctx, params)
	if err != nil {
		return fmt.Errorf("upsert track failed: %w", err)
	}

	// Handle genre (optimized: only delete stale genres, skip if unchanged)
	if info.Format.Tags.Genre != "" {
		genre, err := qtx.GetOrCreateGenre(ctx, database.GetOrCreateGenreParams{
			Tag:       info.Format.Tags.Genre,
			GenreType: "music",
		})

		if err != nil {
			return fmt.Errorf("genre failed: %w", err)
		}

		// Delete only stale genres (those that don't match the new one)
		err = qtx.DeleteTrackGenresExcept(ctx, database.DeleteTrackGenresExceptParams{
			TrackID: track.ID,
			GenreID: genre.ID,
		})

		if err != nil {
			return fmt.Errorf("delete stale genres failed: %w", err)
		}

		// Create track-genre relationship (ON CONFLICT DO NOTHING handles duplicates)
		err = qtx.CreateTrackGenre(ctx, database.CreateTrackGenreParams{
			TrackID: track.ID,
			GenreID: genre.ID,
		})

		if err != nil {
			return fmt.Errorf("track-genre relationship failed: %w", err)
		}

		// Create musician-genre relationship (if musician exists)
		if musicianID.Valid {
			err = qtx.UpsertMusicianGenre(ctx, database.UpsertMusicianGenreParams{
				MusicianID: musicianID.Int64,
				GenreID:    genre.ID,
			})
			if err != nil {
				// Log but don't fail - genre association is enhancement, not critical
				app.Logger.Warn("failed to create musician-genre relationship",
					"error", err,
					"musician_id", musicianID.Int64,
					"genre_id", genre.ID,
				)
			}
		}

		// Create album-genre relationship (if album exists)
		if albumID.Valid {
			err = qtx.UpsertAlbumGenre(ctx, database.UpsertAlbumGenreParams{
				AlbumID: albumID.Int64,
				GenreID: genre.ID,
			})
			if err != nil {
				// Log but don't fail - genre association is enhancement, not critical
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
