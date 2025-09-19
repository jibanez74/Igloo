package main

import (
	"context"
	"fmt"
	"igloo/cmd/internal/database"
	"igloo/cmd/internal/ffprobe"
	"igloo/cmd/internal/helpers"
	"io/fs"
	"path/filepath"
	"strconv"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

func (app *Application) ScanMusicLibrary() *ScanResult {
	result := &ScanResult{
		StartTime: time.Now(),
		Errors:    make([]ScanError, 0),
	}

	ctx := context.Background()

	tx, err := app.Db.Begin(ctx)
	if err != nil {
		app.Logger.Error(fmt.Sprintf("fail to start database transaction for ScanMusicLibrary function\n%s", err.Error()))
		result.Errors = append(result.Errors, ScanError{
			Error:     err,
			ErrorType: "database",
			Timestamp: time.Now(),
		})

		return result
	}
	defer tx.Rollback(ctx)

	qtx := app.Queries.WithTx(tx)

	err = filepath.WalkDir(app.Settings.MusicDir.String, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			result.Errors = append(result.Errors, ScanError{
				FilePath:  path,
				Error:     err,
				ErrorType: "filesystem",
				Timestamp: time.Now(),
			})

			return ni
		}

		ext := helpers.GetFileExtension(path)
		if !helpers.ValidAudioExtensions[ext] {
			return nil
		}

		// Process individual file with error isolation
		if err := app.processTrackWithErrorHandling(ctx, qtx, path, result); err != nil {
			// Error already added to result.Errors in processTrackWithErrorHandling
			return nil // Continue processing
		}

		result.Processed++
		return nil
	})

	if err != nil {
		app.Logger.Error(fmt.Sprintf("fail to scan music library\n%s", err.Error()))
		result.Errors = append(result.Errors, ScanError{
			Error:     err,
			ErrorType: "filesystem",
			Timestamp: time.Now(),
		})
		return result
	}

	// Commit transaction
	if err = tx.Commit(ctx); err != nil {
		app.Logger.Error(fmt.Sprintf("fail to commit database transaction\n%s", err.Error()))
		result.Errors = append(result.Errors, ScanError{
			Error:     err,
			ErrorType: "database",
			Timestamp: time.Now(),
		})
		return result
	}

	result.EndTime = time.Now()
	result.Duration = result.EndTime.Sub(result.StartTime)

	app.Logger.Info(fmt.Sprintf("Music library scan completed. Processed: %d, Skipped: %d, Errors: %d, Duration: %v",
		result.Processed, result.Skipped, len(result.Errors), result.Duration))

	return result
}

func (app *Application) GetOrCreateMusician(ctx context.Context, qtx *database.Queries, metadata *ffprobe.TrackFfprobeResult) (*database.Musician, error) {
	musician, err := qtx.GetMusicianByName(ctx, metadata.Format.Tags.Artist)
	if err != nil {
		if err == pgx.ErrNoRows {
			createMusician := database.CreateMusicianParams{
				Name:     metadata.Format.Tags.Artist,
				SortName: metadata.Format.Tags.SortArtist,
			}

			artist, err := app.Spotify.SearchArtistByName(createMusician.Name)
			if err != nil {
				app.Logger.Error(fmt.Sprintf("fail to get musician %s details from spotify api\n%s", createMusician.Name, err.Error()))
			} else {
				createMusician.SpotifyFollowers = int32(artist.Followers.Count)
				createMusician.SpotifyPopularity = int32(artist.Popularity)
				createMusician.SpotifyID = pgtype.Text{String: artist.ID.String(), Valid: true}
				createMusician.Summary = pgtype.Text{String: fmt.Sprintf("%s is a musician with %d followers and with a popularity on spotify of %d", artist.Name, artist.Followers.Count, artist.Popularity), Valid: true}

				if len(artist.Images) > 0 {
					createMusician.Thumb = pgtype.Text{String: artist.Images[0].URL, Valid: true}
				}
			}

			musician, err = qtx.CreateMusician(ctx, createMusician)
			if err != nil {
				return nil, fmt.Errorf("failed to create musician: %w", err)
			}
		} else {
			return nil, err
		}
	}

	return &musician, nil
}

func (app *Application) GetOrCreateAlbum(ctx context.Context, qtx *database.Queries, metadata *ffprobe.TrackFfprobeResult, musicianID int32) (*database.Album, error) {
	album, err := qtx.GetAlbumByTitle(ctx, metadata.Format.Tags.Album)
	if err != nil {
		if err == pgx.ErrNoRows {
			createAlbum := database.CreateAlbumParams{
				Title:       metadata.Format.Tags.Album,
				SortTitle:   metadata.Format.Tags.SortAlbum,
				DiscCount:   0,
				TotalTracks: 0,
			}

			discCount, err := helpers.SplitSliceBySlash(metadata.Format.Tags.Disc)
			if err != nil {
				app.Logger.Error(fmt.Sprintf("fail to get disc count for album %s\n%s", createAlbum.Title, err.Error()))
			} else {
				createAlbum.DiscCount = discCount[1]
			}

			trackCount, err := helpers.SplitSliceBySlash(metadata.Format.Tags.Track)
			if err != nil {
				app.Logger.Error(fmt.Sprintf("fail to get track count for album %s\n%s", createAlbum.Title, err.Error()))
			} else {
				createAlbum.TotalTracks = trackCount[1]
			}

			if musicianID > 0 {
				createAlbum.MusicianID = pgtype.Int4{
					Int32: musicianID,
					Valid: true,
				}
			}

			albumList, err := app.Spotify.SearchAlbums(createAlbum.Title, 1)
			if err != nil {
				app.Logger.Error(fmt.Sprintf("fail to get details for album %s from spotify api\n%s", createAlbum.Title, err.Error()))
			} else if len(albumList) > 0 {
				createAlbum.SpotifyID = pgtype.Text{
					String: albumList[0].ID.String(),
					Valid:  true,
				}

				date, err := helpers.FormatDate(albumList[0].ReleaseDate)
				if err != nil {
					date, err = helpers.FormatDate(metadata.Format.Tags.Date)
					if err != nil {
						app.Logger.Error(fmt.Sprintf("fail to get release date for album %s", createAlbum.Title))
					}
				}

				if !date.IsZero() {
					createAlbum.ReleaseDate = pgtype.Date{
						Time:  date,
						Valid: true,
					}
					// Extract year from the date
					createAlbum.Year = pgtype.Int4{
						Int32: int32(date.Year()),
						Valid: true,
					}
				}

				if len(albumList[0].Images) > 0 {
					createAlbum.Cover = pgtype.Text{
						String: albumList[0].Images[0].URL,
						Valid:  true,
					}
				}

				albumDetails, err := app.Spotify.GetAlbumBySpotifyID(albumList[0].ID.String())
				if err != nil {
					app.Logger.Error(fmt.Sprintf("fail to get extra details for album %s from spotify api\n%s", createAlbum.Title, err.Error()))
				} else {
					createAlbum.SpotifyPopularity = pgtype.Int4{
						Int32: int32(albumDetails.Popularity),
						Valid: true,
					}
				}
			} else {
				app.Logger.Error(fmt.Sprintf("no results were found in the spotify api for album %s", createAlbum.Title))
			}

			album, err = qtx.CreateAlbum(ctx, createAlbum)
			if err != nil {
				return nil, fmt.Errorf("failed to create album: %w", err)
			}
		} else {
			return nil, fmt.Errorf("failed to query album: %w", err)
		}
	}

	return &album, nil
}

func (app *Application) processTrackWithErrorHandling(ctx context.Context, qtx *database.Queries, path string, result *ScanResult) error {
	// Check if track already exists
	exist, err := qtx.CheckTrackExistByFilePath(ctx, path)
	if err != nil && err != pgx.ErrNoRows {
		result.Errors = append(result.Errors, ScanError{
			FilePath:  path,
			Error:     err,
			ErrorType: "database",
			Timestamp: time.Now(),
		})
		return err
	}

	if exist {
		result.Skipped++
		return nil
	}

	// Get metadata with error handling
	metadata, err := app.Ffprobe.GetTrackMetadata(path)
	if err != nil {
		result.Errors = append(result.Errors, ScanError{
			FilePath:  path,
			Error:     err,
			ErrorType: "ffprobe",
			Timestamp: time.Now(),
		})
		app.Logger.Error(fmt.Sprintf("fail to get metadata for track %s\n%s", path, err.Error()))
		return err
	}

	// Process track with existing logic but wrapped in error handling
	if err := app.processTrackMetadata(ctx, qtx, path, metadata, result); err != nil {
		result.Errors = append(result.Errors, ScanError{
			FilePath:  path,
			Error:     err,
			ErrorType: "processing",
			Timestamp: time.Now(),
		})
		return err
	}

	return nil
}

func (app *Application) processTrackMetadata(ctx context.Context, qtx *database.Queries, path string, metadata *ffprobe.TrackFfprobeResult, result *ScanResult) error {
	var musician *database.Musician

	if metadata.Format.Tags.Artist != "" {
		var err error
		musician, err = app.GetOrCreateMusician(ctx, qtx, metadata)
		if err != nil {
			// Log error but don't fail the entire track processing
			app.Logger.Error(fmt.Sprintf("fail to get or create musician for track %s\n%s", path, err.Error()))
			// Continue without musician
			musician = nil
		}
	}

	var album *database.Album

	if metadata.Format.Tags.Album != "" {
		var musicianID int32

		if musician != nil {
			musicianID = musician.ID
		}

		var err error
		album, err = app.GetOrCreateAlbum(ctx, qtx, metadata, musicianID)
		if err != nil {
			// Log error but don't fail the entire track processing
			app.Logger.Error(fmt.Sprintf("fail to get or create album for track %s\n%s", path, err.Error()))
			// Continue without album
			album = nil
		}
	}

	createTrack := database.CreateTrackParams{
		Title:     metadata.Format.Tags.Title,
		SortTitle: metadata.Format.Tags.SortName,
		FilePath:  path,
		Container: helpers.GetFileExtension(path),
		FileName:  metadata.Format.FileName,
		Copyright: pgtype.Text{
			String: metadata.Format.Tags.Copyright,
			Valid:  metadata.Format.Tags.Copyright != "",
		},
		Composer: pgtype.Text{
			String: metadata.Format.Tags.Composer,
			Valid:  metadata.Format.Tags.Composer != "",
		},
	}

	trackIndex, err := helpers.SplitSliceBySlash(metadata.Format.Tags.Track)
	if err != nil {
		app.Logger.Error(fmt.Sprintf("fail to get track index for track %s\n%s", path, err.Error()))
		createTrack.TrackIndex = 0
	} else {
		createTrack.TrackIndex = trackIndex[0]
	}

	disc, err := helpers.SplitSliceBySlash(metadata.Format.Tags.Disc)
	if err != nil {
		app.Logger.Error(fmt.Sprintf("fail to get disc number for track %s\n%s", path, err.Error()))
		createTrack.Disc = 0
	} else {
		createTrack.Disc = disc[0]
	}

	if metadata.Format.Tags.Date != "" {
		date, err := helpers.FormatDate(metadata.Format.Tags.Date)
		if err != nil {
			app.Logger.Error(fmt.Sprintf("fail to get release date for track %s\n%s", path, err.Error()))
			// Use album release date as fallback if available
			if album != nil {
				createTrack.ReleaseDate = album.ReleaseDate
			}
		} else {
			createTrack.ReleaseDate = pgtype.Date{
				Time:  date,
				Valid: true,
			}
		}
	} else if album != nil {
		// Use album release date if no track date
		createTrack.ReleaseDate = album.ReleaseDate
	}

	duration, err := helpers.GetPreciseDecimalFromStr(metadata.Format.Duration)
	if err != nil {
		app.Logger.Error(fmt.Sprintf("fail to get duration for track %s\n%s", path, err.Error()))
		// Set a default duration or skip this track
		return fmt.Errorf("invalid duration for track %s: %w", path, err)
	}
	createTrack.Duration = duration

	size, err := strconv.Atoi(metadata.Format.Size)
	if err != nil {
		app.Logger.Error(fmt.Sprintf("fail to get size for track %s\n%s", path, err.Error()))
		return fmt.Errorf("invalid size for track %s: %w", path, err)
	}
	createTrack.Size = int64(size)

	bitRate, err := strconv.Atoi(metadata.Format.BitRate)
	if err != nil {
		app.Logger.Error(fmt.Sprintf("fail to get bit rate for track %s\n%s", path, err.Error()))
		// Continue without bit rate
	} else {
		createTrack.BitRate = pgtype.Int4{
			Int32: int32(bitRate),
			Valid: true,
		}
	}

	if musician != nil {
		createTrack.MusicianID = pgtype.Int4{
			Int32: musician.ID,
			Valid: true,
		}
	}

	if album != nil {
		createTrack.AlbumID = pgtype.Int4{
			Int32: album.ID,
			Valid: true,
		}
	}

	if len(metadata.Streams) == 0 {
		return fmt.Errorf("no audio streams found for track %s", path)
	}

	createTrack.Codec = metadata.Streams[0].CodecName
	createTrack.ChannelLayout = metadata.Streams[0].ChannelLayout

	createTrack.Profile = pgtype.Text{
		String: metadata.Streams[0].Profile,
		Valid:  metadata.Streams[0].Profile != "",
	}

	createTrack.Language = pgtype.Text{
		String: metadata.Streams[0].Tags.Language,
		Valid:  metadata.Streams[0].Tags.Language != "",
	}

	sampleRate, err := strconv.Atoi(metadata.Streams[0].SampleRate)
	if err != nil {
		app.Logger.Error(fmt.Sprintf("fail to get sample rate for track %s\n%s", path, err.Error()))
		// Continue without sample rate
	} else {
		createTrack.SampleRate = pgtype.Int4{
			Int32: int32(sampleRate),
			Valid: true,
		}
	}

	track, err := qtx.CreateTrack(ctx, createTrack)
	if err != nil {
		return fmt.Errorf("failed to create track %s: %w", path, err)
	}

	if metadata.Format.Tags.Genre != "" {
		genreList := helpers.ParseGenres(metadata.Format.Tags.Genre)

		for _, g := range genreList {
			data := helpers.SaveGenresParams{
				Tag:       g,
				GenreType: "music",
				TrackID:   track.ID,
			}

			if musician != nil {
				data.MusicianID = musician.ID
			}

			if album != nil {
				data.AlbumID = album.ID
			}

			err = helpers.SaveGenres(ctx, qtx, &data)
			if err != nil {
				app.Logger.Error(fmt.Sprintf("fail to save genre %s for track %s\n%s", g, path, err.Error()))
				// Continue processing other genres
			}
		}
	}

	return nil
}
