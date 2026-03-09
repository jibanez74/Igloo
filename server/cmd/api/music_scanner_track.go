package main

import (
	"context"
	"database/sql"
	"fmt"
	"igloo/cmd/internal/database"
	"igloo/cmd/internal/helpers"
	"igloo/cmd/internal/musicbrainz"
	"path/filepath"
	"strconv"
)

type preparedTrack struct {
	params      database.UpsertTrackParams
	artistName  string
	sortArtist  string
	albumTitle  string
	sortAlbum   string
	albumArtist string
	genre       string
	mbArtist    *musicbrainz.ArtistResult
	mbAlbum     *musicbrainz.AlbumResult
}

func (app *Application) extractTrackMetadata(ctx context.Context, path, ext string) (*preparedTrack, error) {
	info, err := app.Ffprobe.GetMetadata(path)
	if err != nil {
		return nil, fmt.Errorf("ffprobe failed: %w", err)
	}

	pt := &preparedTrack{}
	pt.params.FilePath = path
	pt.params.FileName = filepath.Base(path)

	if info.Format.Tags.Title != "" {
		pt.params.Title = info.Format.Tags.Title
	} else {
		pt.params.Title = filepath.Base(path)
	}

	if info.Format.Tags.SortName != "" {
		pt.params.SortTitle = info.Format.Tags.SortName
	} else {
		pt.params.SortTitle = pt.params.Title
	}

	pt.params.Container = ext

	if mimeType, ok := helpers.AudioMimeTypes[ext]; ok {
		pt.params.MimeType = mimeType
	}

	if info.Format.Size != "" {
		if size, err := strconv.ParseInt(info.Format.Size, 10, 64); err == nil {
			pt.params.Size = size
		}
	}

	if info.Format.Duration != "" {
		if duration, err := helpers.ParseDurationMs(info.Format.Duration); err == nil {
			pt.params.Duration = duration
		}
	}

	if info.Format.Tags.Track != "" {
		if index, err := helpers.ParseSlashNumber(info.Format.Tags.Track); err == nil {
			pt.params.TrackIndex = index
		}
	}

	if info.Format.BitRate != "" {
		pt.params.BitRate = helpers.ParseBitRate(info.Format.BitRate)
	}

	if info.Format.Tags.Disc != "" {
		if disc, err := helpers.ParseSlashNumber(info.Format.Tags.Disc); err == nil {
			pt.params.Disc = disc
		}
	}

	pt.params.Copyright = helpers.NullString(info.Format.Tags.Copyright)
	pt.params.Composer = helpers.NullString(info.Format.Tags.Composer)

	if info.Format.Tags.Date != "" {
		if date, err := helpers.ParseDate(info.Format.Tags.Date); err == nil {
			pt.params.ReleaseDate = sql.NullString{String: date.Format("2006-01-02"), Valid: true}
			pt.params.Year = sql.NullInt64{Int64: int64(date.Year()), Valid: true}
		}
	}

	for _, stream := range info.Streams {
		if stream.CodecType == "audio" {
			pt.params.Codec = stream.CodecName
			pt.params.Profile = stream.Profile

			if stream.ChannelLayout != "" {
				pt.params.Channels = stream.ChannelLayout
				pt.params.ChannelLayout = stream.ChannelLayout
			} else {
				pt.params.Channels = strconv.Itoa(stream.Channels)
				pt.params.ChannelLayout = strconv.Itoa(stream.Channels)
			}

			if stream.Tags.Language != "" {
				pt.params.Language = sql.NullString{String: stream.Tags.Language, Valid: true}
			}

			break
		}
	}

	if info.Format.Tags.Artist != "" {
		pt.artistName = info.Format.Tags.Artist
		pt.sortArtist = info.Format.Tags.SortArtist
		if pt.sortArtist == "" {
			pt.sortArtist = pt.artistName
		}
		_, dbErr := app.Queries.GetMusicianByName(ctx, pt.artistName)
		if dbErr != nil {
			pt.mbArtist, _ = app.MusicBrainz.SearchArtistByName(pt.artistName)
		}
	}

	if info.Format.Tags.Album != "" {
		pt.albumTitle = info.Format.Tags.Album
		pt.sortAlbum = info.Format.Tags.SortAlbum
		if pt.sortAlbum == "" {
			pt.sortAlbum = pt.albumTitle
		}
		pt.albumArtist = info.Format.Tags.AlbumArtist

		needsLookup := true
		if pt.albumArtist != "" {
			_, dbErr := app.Queries.GetAlbumByTitleAndMusician(ctx, database.GetAlbumByTitleAndMusicianParams{
				Title:    pt.albumTitle,
				Musician: sql.NullString{String: pt.albumArtist, Valid: true},
			})
			if dbErr == nil {
				needsLookup = false
			}
		}

		if needsLookup {
			pt.mbAlbum, _ = app.MusicBrainz.SearchAlbumByName(pt.albumTitle, pt.albumArtist)
		}
	}

	pt.genre = info.Format.Tags.Genre

	return pt, nil
}

func (app *Application) writeTrackToDB(ctx context.Context, qtx *database.Queries, pt *preparedTrack) error {
	var musicianID sql.NullInt64

	if pt.artistName != "" {
		musician, err := app.getOrCreateMusicianWithResult(ctx, qtx, pt.artistName, pt.sortArtist, pt.mbArtist)
		if err != nil {
			return fmt.Errorf("musician failed: %w", err)
		}
		musicianID = sql.NullInt64{Int64: musician.ID, Valid: true}
	}
	pt.params.MusicianID = musicianID

	var albumID sql.NullInt64

	if pt.albumTitle != "" {
		album, _, err := app.getOrCreateAlbumWithResult(ctx, qtx, pt.albumTitle, pt.sortAlbum, pt.albumArtist, pt.mbAlbum)
		if err != nil {
			return fmt.Errorf("album failed: %w", err)
		}
		albumID = sql.NullInt64{Int64: album.ID, Valid: true}
	}
	pt.params.AlbumID = albumID

	if musicianID.Valid && albumID.Valid {
		err := qtx.CreateMusicianAlbum(ctx, database.CreateMusicianAlbumParams{
			MusicianID: musicianID.Int64,
			AlbumID:    albumID.Int64,
		})
		if err != nil {
			return fmt.Errorf("musician-album relationship failed: %w", err)
		}
	}

	track, err := qtx.UpsertTrack(ctx, pt.params)
	if err != nil {
		return fmt.Errorf("upsert track failed: %w", err)
	}

	if pt.genre != "" {
		genre, err := qtx.GetOrCreateGenre(ctx, database.GetOrCreateGenreParams{
			Tag:       pt.genre,
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
