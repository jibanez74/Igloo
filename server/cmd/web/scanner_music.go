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

const (
	// BATCH_SIZE is the number of tracks to process before committing a batch
	BATCH_SIZE = 100
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
	errorCount := 0
	tracksScanned := 0
	startTime := time.Now()

	// Batch buffer to collect tracks before processing
	batch := make([]string, 0, BATCH_SIZE)

	err := filepath.WalkDir(app.Settings.MusicDir.String, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			app.Logger.Error(fmt.Sprintf("error walking directory: %s", err.Error()))
			errorCount++
			return nil
		}

		ext := helpers.GetFileExtension(path)
		if !helpers.ValidAudioExtensions[ext] {
			return nil
		}

		// Add to current batch
		batch = append(batch, path)

		// Process batch when it reaches BATCH_SIZE
		if len(batch) >= BATCH_SIZE {
			batchScanned, batchErrors := app.processBatch(ctx, batch)
			tracksScanned += batchScanned
			errorCount += batchErrors

			// Reset batch buffer (reuse underlying array)
			batch = batch[:0]
		}

		return nil
	})

	if err != nil {
		app.Logger.Error(fmt.Sprintf("an unexpected error occurred while walking music directory: %s", err.Error()))
		return
	}

	// Process remaining tracks in the final batch
	if len(batch) > 0 {
		batchScanned, batchErrors := app.processBatch(ctx, batch)
		tracksScanned += batchScanned
		errorCount += batchErrors
	}

	app.Logger.Info(fmt.Sprintf("scanned %d tracks with %d errors in %s", tracksScanned, errorCount, helpers.FormatDuration(time.Since(startTime))))
}

// processBatch processes a batch of tracks in a single transaction with savepoints
func (app *Application) processBatch(ctx context.Context, trackPaths []string) (scanned int, errors int) {
	// Start batch transaction
	tx, err := app.Db.Begin(ctx)
	if err != nil {
		app.Logger.Error(fmt.Sprintf("failed to start batch transaction: %s", err.Error()))
		return 0, len(trackPaths)
	}
	defer func() {
		if err := tx.Rollback(ctx); err != nil && err != pgx.ErrTxClosed {
			app.Logger.Error(fmt.Sprintf("failed to rollback batch transaction: %s", err.Error()))
		}
	}()

	qtx := app.Queries.WithTx(tx)
	savepointCounter := 0

	for _, path := range trackPaths {
		// Create savepoint for this track
		savepointName := fmt.Sprintf("sp_track_%d", savepointCounter)
		savepointCounter++

		_, err := tx.Exec(ctx, fmt.Sprintf("SAVEPOINT %s", savepointName))
		if err != nil {
			app.Logger.Error(fmt.Sprintf("failed to create savepoint %s for track %s: %s", savepointName, path, err.Error()))
			errors++
			continue
		}

		// Check if track already exists by path
		existsByPath, err := qtx.CheckTrackExistByPath(ctx, path)
		if err != nil {
			app.Logger.Error(fmt.Sprintf("failed to check if track at %s exists: %s", path, err.Error()))
			// Rollback to savepoint and release it
			_, _ = tx.Exec(ctx, fmt.Sprintf("ROLLBACK TO SAVEPOINT %s", savepointName))
			_, _ = tx.Exec(ctx, fmt.Sprintf("RELEASE SAVEPOINT %s", savepointName))
			errors++
			continue
		}

		if existsByPath {
			// Release savepoint since we're skipping this track
			_, _ = tx.Exec(ctx, fmt.Sprintf("RELEASE SAVEPOINT %s", savepointName))
			continue
		}

		// Calculate file hash for duplicate detection
		fileHash, err := helpers.CalculateFileHash(path)
		if err != nil {
			app.Logger.Error(fmt.Sprintf("failed to calculate hash for track %s: %s", path, err.Error()))
			// Rollback to savepoint and release it
			_, _ = tx.Exec(ctx, fmt.Sprintf("ROLLBACK TO SAVEPOINT %s", savepointName))
			_, _ = tx.Exec(ctx, fmt.Sprintf("RELEASE SAVEPOINT %s", savepointName))
			errors++
			continue
		}

		// Check if track already exists by hash
		existsByHash, err := qtx.CheckTrackExistByHash(ctx, pgtype.Text{String: fileHash, Valid: true})
		if err != nil {
			app.Logger.Error(fmt.Sprintf("failed to check if track with hash %s exists: %s", fileHash, err.Error()))
			// Rollback to savepoint and release it
			_, _ = tx.Exec(ctx, fmt.Sprintf("ROLLBACK TO SAVEPOINT %s", savepointName))
			_, _ = tx.Exec(ctx, fmt.Sprintf("RELEASE SAVEPOINT %s", savepointName))
			errors++
			continue
		}

		if existsByHash {
			// Release savepoint since we're skipping this duplicate track
			app.Logger.Info(fmt.Sprintf("skipping duplicate track (same hash) at %s", path))
			_, _ = tx.Exec(ctx, fmt.Sprintf("RELEASE SAVEPOINT %s", savepointName))
			continue
		}

		ext := helpers.GetFileExtension(path)

		// Process the track
		err = app.ScanTrackFile(ctx, qtx, path, ext, fileHash)
		if err != nil {
			app.Logger.Error(fmt.Sprintf("failed to scan track file %s: %s", path, err.Error()))
			// Rollback to savepoint to undo this track's changes, then release it
			_, rollbackErr := tx.Exec(ctx, fmt.Sprintf("ROLLBACK TO SAVEPOINT %s", savepointName))
			if rollbackErr != nil {
				app.Logger.Error(fmt.Sprintf("failed to rollback to savepoint %s: %s", savepointName, rollbackErr.Error()))
			} else {
				// Release the savepoint after rollback
				_, _ = tx.Exec(ctx, fmt.Sprintf("RELEASE SAVEPOINT %s", savepointName))
			}
			errors++
			continue
		}

		// Release savepoint since track was processed successfully
		_, err = tx.Exec(ctx, fmt.Sprintf("RELEASE SAVEPOINT %s", savepointName))
		if err != nil {
			app.Logger.Error(fmt.Sprintf("failed to release savepoint %s: %s", savepointName, err.Error()))
			// Rollback to savepoint and release it, then mark as error
			_, _ = tx.Exec(ctx, fmt.Sprintf("ROLLBACK TO SAVEPOINT %s", savepointName))
			_, _ = tx.Exec(ctx, fmt.Sprintf("RELEASE SAVEPOINT %s", savepointName))
			errors++
			continue
		}

		scanned++
	}

	// Commit the batch
	err = tx.Commit(ctx)
	if err != nil {
		app.Logger.Error(fmt.Sprintf("failed to commit batch transaction: %s", err.Error()))
		// All tracks in this batch failed
		return 0, len(trackPaths)
	}

	return scanned, errors
}

func (app *Application) ScanTrackFile(ctx context.Context, qtx *database.Queries, path, ext, fileHash string) error {
	info, err := app.Ffprobe.GetTrackMetadata(path)
	if err != nil {
		return err
	}

	createTrack := database.CreateTrackParams{
		Title:     info.Format.Tags.Title,
		SortTitle: info.Format.Tags.SortName,
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

	// Set file hash
	createTrack.FileHash = pgtype.Text{String: fileHash, Valid: true}

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
