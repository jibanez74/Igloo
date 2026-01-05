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
	if mimeType, ok := helpers.AudioMimeTypes[ext]; ok {
		params.MimeType = mimeType
	}

	// Parse size
	if info.Format.Size != "" {
		if size, err := strconv.ParseInt(info.Format.Size, 10, 64); err == nil {
			params.Size = size
		}
	}

	// Parse duration (convert seconds to milliseconds)
	if info.Format.Duration != "" {
		if duration, err := helpers.ParseDurationMs(info.Format.Duration); err == nil {
			params.Duration = duration
		}
	}

	// Parse track index from "1/12" format
	if info.Format.Tags.Track != "" {
		if index, err := helpers.ParseSlashNumber(info.Format.Tags.Track); err == nil {
			params.TrackIndex = index
		}
	}

	// Parse bit rate
	if info.Format.BitRate != "" {
		if bitRate, err := strconv.ParseInt(info.Format.BitRate, 10, 64); err == nil {
			params.BitRate = bitRate
		}
	}

	// Parse disc number from "1/2" format
	if info.Format.Tags.Disc != "" {
		if disc, err := helpers.ParseSlashNumber(info.Format.Tags.Disc); err == nil {
			params.Disc = disc
		}
	}

	// Optional text fields
	params.Copyright = helpers.NullString(info.Format.Tags.Copyright)
	params.Composer = helpers.NullString(info.Format.Tags.Composer)

	// Parse release date
	if info.Format.Tags.Date != "" {
		if date, err := helpers.ParseDate(info.Format.Tags.Date); err == nil {
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

	// Upsert the track
	track, err := qtx.UpsertTrack(ctx, params)
	if err != nil {
		return fmt.Errorf("upsert track failed: %w", err)
	}

	// Handle genre (clear existing, add new from metadata)
	if info.Format.Tags.Genre != "" {
		// Delete existing track-genre relationships (for metadata updates)
		err = qtx.DeleteTrackGenres(ctx, track.ID)
		if err != nil {
			return fmt.Errorf("delete genres failed: %w", err)
		}

		// Get or create genre
		genre, err := qtx.GetOrCreateGenre(ctx, database.GetOrCreateGenreParams{
			Tag:       info.Format.Tags.Genre,
			GenreType: "music",
		})
		if err != nil {
			return fmt.Errorf("genre failed: %w", err)
		}

		// Create track-genre relationship
		err = qtx.CreateTrackGenre(ctx, database.CreateTrackGenreParams{
			TrackID: track.ID,
			GenreID: genre.ID,
		})
		if err != nil {
			return fmt.Errorf("track-genre relationship failed: %w", err)
		}
	}

	return nil
}
