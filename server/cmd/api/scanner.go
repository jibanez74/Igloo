package main

import (
	"context"
	"errors"
	"fmt"
	"igloo/cmd/internal/database"
	"igloo/cmd/internal/helpers"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

func (app *Application) ScanMusicLibrary() {
	entries, err := os.ReadDir(app.Settings.MusicDir.String)
	if err != nil {
		app.Logger.Error(fmt.Sprintf("faile to scan music directory at %s\n%s", app.Settings.MusicDir.String, err.Error()))
		return
	}

	if len(entries) == 0 {
		app.Logger.Info(fmt.Sprintf("music directory at %s appears to be empty", app.Settings.MusicDir.String))
		return
	}

	ctx := context.Background()

	tx, err := app.Db.Begin(ctx)
	if err != nil {
		app.Logger.Error(fmt.Sprintf("fail to start data base transaction for ScanMusicDir function\n%s", err.Error()))
		return
	}
	defer tx.Rollback(ctx)

	qtx := app.Queries.WithTx(tx)

	for _, entry := range entries {
		if !entry.IsDir() || entry.Name() == "Compilations" {
			continue
		}

		musician, err := app.ScanDirsForMusicians(ctx, qtx, entry.Name())
		if err != nil {
			app.Logger.Error(fmt.Sprintf("fail to scan directory %s\n%s", entry.Name(), err.Error()))
			continue
		}

		_, err = app.ScanDirForAlbums(ctx, qtx, musician)
		if err != nil {
			app.Logger.Error(fmt.Sprintf("an error occurred while scanning directory %s for albums\n%s", musician.DirectoryPath, err.Error()))
			continue
		}
	}

	err = tx.Commit(ctx)
	if err != nil {
		app.Logger.Error(fmt.Sprintf("An error occurred while trying to commit the database transaction\n%s", err.Error()))
		return
	}

	app.Logger.Info(fmt.Sprintf("Successfully scanned %d musicians from music directory", len(entries)))
}

func (app *Application) ScanDirsForMusicians(ctx context.Context, qtx *database.Queries, name string) (*database.Musician, error) {
	if name == "" {
		return nil, errors.New("got empty name string in ScanDirsForMusician function")
	}

	dir := filepath.Join(app.Settings.MusicDir.String, name)

	musician, err := qtx.GetMusicianByPath(ctx, dir)
	if err != nil {
		if err == pgx.ErrNoRows {
			createMusician := database.CreateMusicianParams{
				Name:          name,
				SortName:      name,
				DirectoryPath: dir,
			}

			artist, err := app.Spotify.SearchArtistByName(name)
			if err != nil {
				app.Logger.Error(fmt.Sprintf("fail to get meta data for musician %s\n%s", name, err.Error()))
			} else {
				createMusician.Name = artist.Name
				createMusician.SpotifyFollowers = int32(artist.Followers.Count)
				createMusician.SpotifyPopularity = int32(artist.Popularity)
				createMusician.Summary = pgtype.Text{
					String: fmt.Sprintf("%s is a musician on Spotify with a popularity of %d and %d followers", artist.Name, artist.Popularity, artist.Followers.Count),
					Valid:  true,
				}
				createMusician.SpotifyID = pgtype.Text{
					String: artist.ID.String(),
					Valid:  true,
				}

				if len(artist.Images) > 0 {
					createMusician.Thumb = pgtype.Text{
						String: artist.Images[0].URL,
						Valid:  true,
					}
				}

				musician, err = qtx.CreateMusician(ctx, createMusician)
				if err != nil {
					return nil, err
				}
			}
		} else {
			return nil, fmt.Errorf("fail to check if musician %s exists\n%s", name, err.Error())
		}
	}

	return &musician, nil
}

func (app *Application) ScanDirForAlbums(ctx context.Context, qtx *database.Queries, musician *database.Musician) ([]*database.Album, error) {
	if musician == nil {
		return nil, errors.New("got nil value for musician in ScanDirForAlbums function")
	}

	albumEntries, err := os.ReadDir(musician.DirectoryPath)
	if err != nil {
		return nil, err
	}

	var albums []*database.Album

	if len(albumEntries) == 0 {
		return albums, nil
	}

	for _, albumEntry := range albumEntries {
		albumDir := filepath.Join(musician.DirectoryPath, albumEntry.Name())

		album, err := qtx.GetAlbumByPathAndTitle(ctx, database.GetAlbumByPathAndTitleParams{
			DirectoryPath: albumDir,
			Title:         albumEntry.Name(),
		})

		if err != nil {
			if err == pgx.ErrNoRows {
				createAlbum := database.CreateAlbumParams{
					Title:         albumEntry.Name(),
					SortTitle:     albumEntry.Name(),
					DirectoryPath: albumDir,
				}

				albumDetails, err := app.Spotify.SearchAndGetAlbumDetails(albumEntry.Name())
				if err != nil {
					app.Logger.Error(fmt.Sprintf("fail to get meta data for album %s\n%s", albumEntry.Name(), err.Error()))
					// Leave date fields null when Spotify metadata fails
				} else {
					createAlbum.Title = albumDetails.Name
					createAlbum.TotalTracks = int32(albumDetails.TotalTracks)
					createAlbum.MusicianID = pgtype.Int4{
						Int32: musician.ID,
						Valid: true,
					}

					date, err := helpers.FormatDate(albumDetails.ReleaseDate)
					if err != nil {
						app.Logger.Error(fmt.Sprintf("fail to parse date %s for album %s\n%s", albumDetails.ReleaseDate, albumDetails.Name, err.Error()))
						// Leave date fields null when parsing fails
					} else {
						createAlbum.ReleaseDate = pgtype.Date{
							Time:  date,
							Valid: true,
						}
						createAlbum.Year = pgtype.Int4{
							Int32: int32(date.Year()),
							Valid: true,
						}
					}

					if len(albumDetails.Images) > 0 {
						createAlbum.Cover = pgtype.Text{
							String: albumDetails.Images[0].URL,
							Valid:  true,
						}
					}
				}

				album, err = qtx.CreateAlbum(ctx, createAlbum)
				if err != nil {
					return nil, err
				}

				albums = append(albums, &album)

				err = app.ScanDirForTracks(ctx, qtx, &album)
				if err != nil {
					app.Logger.Error(fmt.Sprintf("failed to scan tracks for album %s: %s", album.Title, err.Error()))
				}
			} else {
				app.Logger.Error(fmt.Sprintf("fail to check if album at %s exists in the data base\n%s", albumDir, err.Error()))
			}
		} else {
			albums = append(albums, &album)

			// Scan tracks for existing album
			err = app.ScanDirForTracks(ctx, qtx, &album)
			if err != nil {
				app.Logger.Error(fmt.Sprintf("failed to scan tracks for existing album %s: %s", album.Title, err.Error()))
			}
		}
	}

	return albums, nil
}

func (app *Application) ScanDirForTracks(ctx context.Context, qtx *database.Queries, album *database.Album) error {
	if album == nil {
		return errors.New("got a nil value for album in ScanDirForTracks function")
	}

	err := filepath.WalkDir(album.DirectoryPath, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		ext := helpers.GetFileExtension(path)

		if !helpers.ValidAudioExtensions[ext] {
			return nil
		}

		info, err := app.Ffprobe.GetTrackMetadata(path)
		if err != nil {
			return err
		}
		_, err = qtx.GetTrackByPath(ctx, path)
		if err != nil {
			if err == pgx.ErrNoRows {
				createTrack := database.CreateTrackParams{
					Title:     info.Format.Tags.Title,
					SortTitle: info.Format.Tags.SortName,
					FilePath:  path,
					FileName:  entry.Name(),
					Container: ext,
					MusicianID: pgtype.Int4{
						Int32: album.MusicianID.Int32,
						Valid: true,
					},
					AlbumID: pgtype.Int4{
						Int32: album.ID,
						Valid: true,
					},
				}

				duration, err := helpers.GetPreciseDecimalFromStr(info.Format.Duration)
				if err != nil {
					return err
				}
				createTrack.Duration = duration

				size, err := strconv.ParseInt(info.Format.Size, 10, 64)
				if err != nil {
					return err
				}
				createTrack.Size = size

				trackCount, err := helpers.SplitSliceBySlash(info.Format.Tags.Track)
				if err != nil {
					return err
				}
				createTrack.TrackIndex = trackCount[0]

				if info.Format.BitRate != "" {
					bitRate, err := strconv.Atoi(info.Format.BitRate)
					if err == nil {
						createTrack.BitRate = pgtype.Int4{
							Int32: int32(bitRate),
							Valid: true,
						}
					}
				}

				disc, err := helpers.SplitSliceBySlash(info.Format.Tags.Disc)
				if err != nil {
					createTrack.Disc = 1
				} else {
					createTrack.Disc = disc[0]
				}

				if info.Format.Tags.Date != "" {
					date, err := helpers.FormatDate(info.Format.Tags.Date)
					if err == nil {
						createTrack.ReleaseDate = pgtype.Date{
							Time:  date,
							Valid: true,
						}

						createTrack.Year = pgtype.Int4{
							Int32: int32(date.Year()),
							Valid: true,
						}
					}
				}

				if info.Format.Tags.Composer != "" {
					createTrack.Composer = pgtype.Text{
						String: info.Format.Tags.Composer,
						Valid:  true,
					}
				}

				if info.Format.Tags.Copyright != "" {
					createTrack.Copyright = pgtype.Text{
						String: info.Format.Tags.Copyright,
						Valid:  true,
					}
				}

				for _, s := range info.Streams {
					if s.CodecType == CODEC_TYPE_AUDIO {
						createTrack.ChannelLayout = s.ChannelLayout
						createTrack.Codec = s.CodecName
						createTrack.Channels = int32(s.Channels)

						if s.SampleRate != "" {
							sampleRate, err := strconv.Atoi(s.SampleRate)
							if err == nil {
								createTrack.SampleRate = pgtype.Int4{
									Int32: int32(sampleRate),
									Valid: true,
								}
							}
						}

						if s.Profile != "" {
							createTrack.Profile = pgtype.Text{
								String: s.Profile,
								Valid:  true,
							}
						}

						if s.Tags.Language != "" {
							createTrack.Language = pgtype.Text{
								String: s.Tags.Language,
								Valid:  true,
							}
						}

						break
					}
				}

				_, err = qtx.CreateTrack(ctx, createTrack)
				if err != nil {
					return err
				}
			}
		}

		return nil
	})

	if err != nil {
		return fmt.Errorf("failed to walk directory %s: %w", album.DirectoryPath, err)
	}

	return nil
}
