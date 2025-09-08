package main

import (
	"context"
	"fmt"
	"igloo/cmd/internal/database"
	"igloo/cmd/internal/ffprobe"
	"igloo/cmd/internal/helpers"
	"io/fs"
	"log"
	"path/filepath"
	"strconv"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

func (app *Application) ScanMusicLibrary() {
	ctx := context.Background()

	tx, err := app.Db.Begin(ctx)
	if err != nil {
		app.Logger.Error(fmt.Sprintf("fail to start data base transaction for ScanMusicLibrary function\n%s", err.Error()))
		return
	}
	defer tx.Rollback(ctx)

	qtx := app.Queries.WithTx(tx) // Create query interface with transaction

	err = filepath.WalkDir(app.Settings.MusicDir.String, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		ext := helpers.GetFileExtension(path)

		if helpers.ValidAudioExtensions[ext] {
			exist, err := qtx.CheckTrackExistByFilePath(ctx, path)
			if err != nil && err != pgx.ErrNoRows {
				return err
			}

			if !exist {

				metadata, err := app.Ffprobe.GetTrackMetadata(path)
				if err != nil {
					return err
				}

				var musician *database.Musician

				if metadata.Format.Tags.Artist != "" {
					musician, err = app.GetOrCreateMusician(ctx, qtx, metadata)
					if err != nil {
						return err
					}
				}

				var album *database.Album

				if metadata.Format.Tags.Album != "" {
					var musicianID int32

					if musician != nil {
						musicianID = musician.ID
					}

					album, err = app.GetOrCreateAlbum(ctx, qtx, metadata, musicianID)
					if err != nil {
						return err
					}
				}

				createTrack := database.CreateTrackParams{
					Title:     metadata.Format.Tags.Title,
					SortTitle: metadata.Format.Tags.SortName,
					FilePath:  path,
					Container: ext,
					FileName:  metadata.Format.FileName,
					Copyright: pgtype.Text{
						String: metadata.Format.Tags.Copyright,
						Valid:  true,
					},
					Composer: pgtype.Text{
						String: metadata.Format.Tags.Composer,
						Valid:  true,
					},
				}

				trackIndex, err := helpers.SplitSliceBySlash(metadata.Format.Tags.Track)
				if err != nil {
					return err
				}
				createTrack.TrackIndex = trackIndex[0]

				disc, err := helpers.SplitSliceBySlash(metadata.Format.Tags.Disc)
				if err != nil {
					app.Logger.Error(fmt.Sprintf("fail to get disc number for track %s\n%s", createTrack.Title, err.Error()))
					createTrack.Disc = 0
				} else {
					createTrack.Disc = disc[0]
				}

				date, err := helpers.FormatDate(metadata.Format.Tags.Date)
				if err != nil {
					app.Logger.Error(fmt.Sprintf("fail to get release date for track %s\n%s", createTrack.Title, err.Error()))
				} else {
					createTrack.ReleaseDate = pgtype.Date{
						Time:  date,
						Valid: true,
					}
				}

				duration, err := helpers.GetPreciseDecimalFromStr(metadata.Format.Duration)
				if err != nil {
					return err
				}
				createTrack.Duration = duration

				size, err := strconv.Atoi(metadata.Format.Size)
				if err != nil {
					return err
				}
				createTrack.Size = int64(size)

				bitRate, err := strconv.Atoi(metadata.Format.BitRate)
				if err != nil {
					return err
				}
				createTrack.BitRate = pgtype.Int4{
					Int32: int32(bitRate),
					Valid: true,
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

				createTrack.Codec = metadata.Streams[0].CodecName
				createTrack.ChannelLayout = metadata.Streams[0].ChannelLayout

				createTrack.Profile = pgtype.Text{
					String: metadata.Streams[0].Profile,
					Valid:  true,
				}

				createTrack.Language = pgtype.Text{
					String: metadata.Streams[0].Tags.Language,
					Valid:  true,
				}

				sampleRate, err := strconv.Atoi(metadata.Streams[0].SampleRate)
				if err != nil {
					app.Logger.Error(fmt.Sprintf("fail to get sample rate for track %s\n%s", createTrack.Title, err.Error()))
				} else {
					createTrack.SampleRate = pgtype.Int4{
						Int32: int32(sampleRate),
						Valid: true,
					}
				}

				track, err := qtx.CreateTrack(ctx, createTrack)
				if err != nil {
					return err
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
							app.Logger.Error(fmt.Sprintf("fail to save genre %s\n%s", g, err.Error()))
						}
					}
				}

			}
		}

		return nil
	})

	if err != nil {
		app.Logger.Error(fmt.Sprintf("failt to scan music library\n%s", err.Error()))
		return
	}

	err = tx.Commit(ctx)
	if err != nil {
		app.Logger.Error(fmt.Sprintf("fail to commit data base transaction\n%s", err.Error()))
		return
	}

	log.Println("finish scanning music library")
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
				createMusician.SpotifyPopularity = pgtype.Int4{Int32: int32(artist.Popularity), Valid: true}
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
