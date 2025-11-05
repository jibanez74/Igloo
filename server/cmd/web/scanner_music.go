package main

import (
	"context"
	"fmt"
	"igloo/cmd/internal/database"
	"igloo/cmd/internal/helpers"
	"io/fs"
	"path/filepath"
	"strconv"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

func (app *Application) ScanMusicLibrary() {
	if app.Wait != nil {
		app.Wait.Add(1)
		defer app.Wait.Done()
	}

	if app.Settings.MusicDir.String == "" {
		app.Logger.Error("got an empty string in ScanMusicLibrary")
		return
	}

	ctx := context.Background()

	tx, err := app.Db.Begin(ctx)
	if err != nil {
		app.Logger.Error(fmt.Sprintf("fail to start data base transaction to scan music library\n%s", err.Error()))
	}
	defer tx.Rollback(ctx)

	qtx := app.Queries.WithTx(tx)
	errorCount := 0
	tracksScanned := 0
	startTime := time.Now()

	err = filepath.WalkDir(app.Settings.MusicDir.String, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			app.Logger.Error(err.Error())
			errorCount++
			return nil
		}

		ext := helpers.GetFileExtension(path)

		if !helpers.ValidAudioExtensions[ext] {
			return nil
		}

		exists, err := qtx.CheckTrackExistByPath(ctx, path)
		if err != nil {
			app.Logger.Error(fmt.Sprintf("fail to check if track at %s exists\n%s", path, err.Error()))
			errorCount++
			return nil
		}

		if exists {
			return nil
		}

		err = app.ScanTrackFile(ctx, qtx, path, ext)
		if err != nil {
			app.Logger.Error(fmt.Sprintf("ffprobe faile to scan track file %s\n%s", path, err.Error()))
			errorCount++
			return nil
		}

		tracksScanned++
		return nil
	})

	if err != nil {
		app.Logger.Error(fmt.Sprintf("an unexpected error occurred while scanning the music library\n%s", err.Error()))
		return
	}

	err = tx.Commit(ctx)
	if err != nil {
		app.Logger.Error(fmt.Sprintf("fail to commit transaction in ScanMusicLibrary function\n%s", err.Error()))
		return
	}

	app.Logger.Info(fmt.Sprintf("scanned %d tracks with %d errors in %s", tracksScanned, errorCount, helpers.FormatDuration(time.Since(startTime))))
}

func (app *Application) ScanTrackFile(ctx context.Context, qtx *database.Queries, path, ext string) error {
	info, err := app.Ffprobe.GetTrackMetadata(path)
	if err != nil {
		return err
	}

	createTrack := database.CreateTrackParams{
		Title:     info.Format.Tags.Title,
		FilePath:  path,
		FileName:  info.Format.FileName,
		Container: ext,
	}

	size, err := strconv.ParseInt(info.Format.Size, 10, 64)
	if err != nil {
		return err
	}
	createTrack.Size = size

	duration, err := helpers.GetPreciseDecimalFromStr(info.Format.Duration)
	if err != nil {
		return err
	}
	createTrack.Duration = duration

	index, err := helpers.SplitSliceBySlash(info.Format.Tags.Track)
	if err != nil {
		return err
	}
	createTrack.TrackIndex = index[0]

	bitRate, err := strconv.Atoi(info.Format.BitRate)
	if err != nil {
		return err
	}
	createTrack.BitRate = int32(bitRate)

	if info.Format.Tags.Copyright != "" {
		createTrack.Copyright = pgtype.Text{String: info.Format.Tags.Copyright, Valid: true}
	}

	if info.Format.Tags.Composer != "" {
		createTrack.Composer = pgtype.Text{String: info.Format.Tags.Composer, Valid: true}
	}

	if info.Format.Tags.Disc != "" {
		discCount, err := helpers.SplitSliceBySlash(info.Format.Tags.Disc)
		if err == nil {
			createTrack.Disc = discCount[0]
		}
	}

	if info.Format.Tags.Date != "" {
		date, err := helpers.FormatDate(info.Format.Tags.Date)
		if err == nil {
			createTrack.ReleaseDate = pgtype.Date{Time: date, Valid: true}
			createTrack.Year = pgtype.Int4{Int32: int32(date.Year()), Valid: true}
		}
	}

	if info.Format.Tags.Artist != "" {
		musician, err := app.GetOrCreateMusician(ctx, qtx, info.Format.Tags.Artist, info.Format.Tags.SortArtist)
		if err != nil {
			return err
		}

		createTrack.MusicianID = pgtype.Int4{Int32: musician.ID, Valid: true}
	}

	if info.Format.Tags.Album != "" {
		album, err := app.GetOrCreateAlbum(ctx, qtx, createTrack.MusicianID.Int32, info.Format.Tags.Album, info.Format.Tags.SortAlbum, info.Format.Tags.AlbumArtist)
		if err != nil {
			return err
		}

		createTrack.AlbumID = pgtype.Int4{Int32: album.ID, Valid: true}
	}

	if createTrack.MusicianID.Int32 > 0 && createTrack.AlbumID.Int32 > 0 {
		err = app.CreateAlbumMusicianRelation(ctx, qtx, createTrack.MusicianID.Int32, createTrack.AlbumID.Int32)
		if err != nil {
			return err
		}
	}

	for _, s := range info.Streams {
		if s.CodecType == CODEC_TYPE_AUDIO {
			createTrack.Codec = s.CodecName
			createTrack.ChannelLayout = s.ChannelLayout
			createTrack.Channels = int32(s.Channels)

			if s.Tags.Language != "" {
				createTrack.Language = pgtype.Text{String: s.Tags.Language, Valid: true}
			}

			if s.Profile != "" {
				createTrack.Profile = pgtype.Text{String: s.Profile, Valid: true}
			}

			break
		}
	}

	track, err := qtx.CreateTrack(ctx, createTrack)
	if err != nil {
		return err
	}

	if info.Format.Tags.Genre != "" {
		genreIdList, err := helpers.SaveGenres(ctx, qtx, info.Format.Tags.Genre, MUSIC_GENRE_TYPE)
		if err != nil {
			return err
		}

		for _, genreID := range genreIdList {
			err = helpers.SaveGenreMusic(ctx, qtx, &track, genreID)
			if err != nil {
				return err
			}

		}
	}

	return nil
}

func (app *Application) GetOrCreateMusician(ctx context.Context, qtx *database.Queries, name, sortName string) (*database.Musician, error) {
	artist, err := app.Spotify.SearchArtistByName(name)
	if err != nil {
		musician, err := qtx.GetMusicianByName(ctx, name)
		if err != nil {
			if err == pgx.ErrNoRows {
				musician, err = qtx.CreateMusician(ctx, database.CreateMusicianParams{
					Name:     name,
					SortName: sortName,
				})
				if err != nil {
					return nil, err
				}
				return &musician, nil
			}

			return nil, err
		}

		return &musician, nil
	}

	musician, err := qtx.GetMusicianBySpotifyID(ctx, pgtype.Text{String: artist.ID.String(), Valid: true})
	if err != nil {
		if err == pgx.ErrNoRows {
			musician, err = qtx.CreateMusician(ctx, database.CreateMusicianParams{
				Name:              name,
				SortName:          sortName,
				SpotifyID:         pgtype.Text{String: artist.ID.String(), Valid: true},
				SpotifyPopularity: int32(artist.Popularity),
				SpotifyFollowers:  int32(artist.Followers.Count),
				Summary: pgtype.Text{
					String: fmt.Sprintf("%s is a musician with a popularity of %d and %d followers on Spotify", artist.Name, artist.Popularity, artist.Followers.Count),
					Valid:  true,
				},
			})
			if err != nil {
				return nil, err
			}
			return &musician, nil
		}

		return nil, err
	}

	return &musician, nil
}

func (app *Application) GetOrCreateAlbum(ctx context.Context, qtx *database.Queries, musicianID int32, title, sortTitle, albumArtist string) (*database.Album, error) {
	albumDetails, err := app.Spotify.SearchAndGetAlbumDetails(title)
	if err != nil {
		album, err := qtx.GetAlbumByTitle(ctx, title)
		if err != nil {
			if err == pgx.ErrNoRows {
				createAlbum := database.CreateAlbumParams{
					Title:     title,
					SortTitle: sortTitle,
				}

				if albumArtist != "" {
					createAlbum.Musician = pgtype.Text{String: albumArtist, Valid: true}
				}

				album, err = qtx.CreateAlbum(ctx, createAlbum)
				if err != nil {
					return nil, err
				}

				return &album, nil
			}

			return nil, err
		}

		return &album, nil
	}

	album, err := qtx.GetAlbumBySpotifyID(ctx, pgtype.Text{String: albumDetails.ID.String(), Valid: true})
	if err != nil {
		if err == pgx.ErrNoRows {
			createAlbum := database.CreateAlbumParams{
				Title:             title,
				SortTitle:         sortTitle,
				SpotifyID:         pgtype.Text{String: albumDetails.ID.String(), Valid: true},
				SpotifyPopularity: pgtype.Int4{Int32: int32(albumDetails.Popularity), Valid: true},
				TotalTracks:       int32(albumDetails.TotalTracks),
				ReleaseDate:       pgtype.Date{Time: albumDetails.ReleaseDateTime(), Valid: true},
				Year:              pgtype.Int4{Int32: int32(albumDetails.ReleaseDateTime().Year()), Valid: true},
			}

			if albumArtist != "" {
				createAlbum.Musician = pgtype.Text{String: albumArtist, Valid: true}
			}

			if len(albumDetails.Images) > 0 {
				createAlbum.Cover = pgtype.Text{String: albumDetails.Images[0].URL, Valid: true}
			}

			album, err = qtx.CreateAlbum(ctx, createAlbum)
			if err != nil {
				return nil, err
			}

			return &album, nil

		}

		return nil, err
	}

	return &album, nil
}

func (app *Application) CreateAlbumMusicianRelation(ctx context.Context, qtx *database.Queries, musicianID, albumID int32) error {
	exists, err := qtx.CheckAlbumMusicianExists(ctx, database.CheckAlbumMusicianExistsParams{
		AlbumID:    albumID,
		MusicianID: musicianID,
	})

	if err != nil {
		return fmt.Errorf("failed to check if album-musician relationship exists: %w", err)
	}

	if exists {
		return nil
	}

	_, err = qtx.CreateAlbumMusician(ctx, database.CreateAlbumMusicianParams{
		AlbumID:    albumID,
		MusicianID: musicianID,
	})

	if err != nil {
		return fmt.Errorf("failed to create album-musician relationship: %w", err)
	}

	return nil
}
