package main

import (
	"context"
	"fmt"
	"igloo/cmd/internal/database"
	"igloo/cmd/internal/helpers"
	"io/fs"
	"log"
	"math"
	"math/big"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/jackc/pgx/v5/pgtype"
)

func (app *Application) ScanMusicLibrary() {
	if app.Settings == nil {
		app.Logger.Error("got nil value for Settings in ScanMusicLibrary function")
		return
	}

	if app.Settings.MusicDir == "" {
		app.Logger.Error("got empty value for music library in ScanMusicLibrary function")
		return
	}

	if app.Ffprobe == nil {
		app.Logger.Error("got nil value for ffprobe on ScanMusicLibrary function")
		return
	}

	if app.Spotify == nil {
		app.Logger.Error("got nil value for spotify on ScanMusicLibrary function")
		return
	}

	ctx := context.Background()

	err := filepath.WalkDir(app.Settings.MusicDir, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		ext := strings.ToLower(strings.TrimPrefix(filepath.Ext(path), "."))

		if helpers.ValidAudioExtensions[ext] {
			exist, err := app.Queries.CheckTrackExistByFilePath(ctx, path)
			if err != nil {
				return err
			}

			if !exist {
				trackMetadata, err := app.Ffprobe.GetTrackMetadata(path)
				if err != nil {
					return err
				}

				if len(trackMetadata.Streams) == 0 {
					return fmt.Errorf("ffprobe failed to detect any streams for track file %s", path)
				}

				artist, err := app.Spotify.SearchArtistByName(entry.Name())
				if err != nil {
					return err
				}

				tx, err := app.Db.Begin(ctx)
				if err != nil {
					return err
				}
				defer tx.Rollback(ctx)

				qtx := app.Queries.WithTx(tx)

				exist, err = qtx.CheckMusicianExistsBySpotifyID(ctx, artist.ID.String())
				if err != nil {
					return err
				}

				var musician database.Musician

				if !exist {
					createMusician := database.CreateMusicianParams{
						Name:              artist.Name,
						SortName:          trackMetadata.Format.Tags.SortArtist,
						SpotifyID:         artist.ID.String(),
						SpotifyPopularity: int32(artist.Popularity),
						SpotifyFollowers:  int32(artist.Followers.Count),
						Summary:           fmt.Sprintf("%s is an artist on spotify with %d followers and %d of popularity", artist.Name, artist.Followers.Count, artist.Popularity),
					}

					if len(artist.Images) > 0 {
						createMusician.Thumb = artist.Images[0].URL
					}

					musician, err = qtx.CreateMusician(ctx, createMusician)
					if err != nil {
						return err
					}
				} else {
					musician, err = qtx.GetMusicianBySpotifyID(ctx, artist.ID.String())
					if err != nil {
						return err
					}
				}

				var album database.Album
				discCount := []int32{0, 0}
				trackCount := []int32{0, 0}

				if trackMetadata.Format.Tags.Album != "" {
					albumList, err := app.Spotify.SearchAlbums(trackMetadata.Format.Tags.Album, 1)
					if err != nil {
						return nil
					}

					if len(albumList) == 0 {
						return fmt.Errorf("fail to get any results for album %s from spotify api", trackMetadata.Format.Tags.Album)
					}

					exist, err = qtx.CheckAlbumExistsBySpotifyID(ctx, albumList[0].ID.String())
					if err != nil {
						return err
					}

					if !exist {
						createAlbum := database.CreateAlbumParams{
							Title:             albumList[0].Name,
							SortTitle:         trackMetadata.Format.Tags.SortAlbum,
							SpotifyID:         albumList[0].ID.String(),
							SpotifyPopularity: 0,
							TotalTracks:       0,
							DiscCount:         0,
							MusicianID: pgtype.Int4{
								Int32: musician.ID,
								Valid: true,
							},
						}

						if trackMetadata.Format.Tags.Date != "" {
							date, err := helpers.FormatDate(trackMetadata.Format.Tags.Date)
							if err != nil {
								return err
							}

							createAlbum.ReleaseDate = pgtype.Date{
								Time:  date,
								Valid: true,
							}
						}

						if trackMetadata.Format.Tags.Track != "" {
							trackCount, err = helpers.SplitSliceBySlash(trackMetadata.Format.Tags.Track)
							if err != nil {
								app.Logger.Error(fmt.Sprintf("fail to get track count for album %s\n%s", createAlbum.Title, err.Error()))
							} else {
								createAlbum.TotalTracks = trackCount[1]
							}
						}

						if trackMetadata.Format.Tags.Disc != "" {
							discCount, err = helpers.SplitSliceBySlash(trackMetadata.Format.Tags.Disc)
							if err != nil {
								app.Logger.Error(fmt.Sprintf("fail to get disc count for album %s\n%s", createAlbum.Title, err.Error()))
							} else {
								createAlbum.DiscCount = discCount[1]
							}
						}

						if len(albumList[0].Images) > 0 {
							createAlbum.Cover = albumList[0].Images[0].URL
						}

						albumDetails, err := app.Spotify.GetAlbumBySpotifyID(albumList[0].ID.String())
						if err != nil {
							app.Logger.Error(fmt.Sprintf("fail to get details for album %s from spotify api\n%s", createAlbum.Title, err.Error()))
						} else {
							createAlbum.SpotifyPopularity = int32(albumDetails.Popularity)
						}

						album, err = qtx.CreateAlbum(ctx, createAlbum)
						if err != nil {
							return err
						}
					} else {
						album, err = qtx.GetAlbumBySpotifyID(ctx, albumList[0].ID.String())
						if err != nil {
							return err
						}
					}
				}

				createTrack := database.CreateTrackParams{
					Title:       trackMetadata.Format.Tags.Title,
					SortTitle:   trackMetadata.Format.Tags.SortName,
					Index:       trackCount[0],
					Disc:        discCount[0],
					Composer:    trackMetadata.Format.Tags.Composer,
					Copyright:   trackMetadata.Format.Tags.Copyright,
					Container:   ext,
					ReleaseDate: album.ReleaseDate,
					FilePath:    path,
					FileName:    entry.Name(),
					AlbumID: pgtype.Int4{
						Int32: album.ID,
						Valid: true,
					},
				}

				duration, err := strconv.ParseFloat(trackMetadata.Format.Duration, 64)
				if err != nil {
					return err
				}

				precision := int32(6)
				multiplier := math.Pow(10, float64(precision))
				integerPart := int64(duration * multiplier)

				createTrack.Duration = pgtype.Numeric{
					Int:   big.NewInt(integerPart),
					Exp:   -precision,
					Valid: true,
				}

				size, err := strconv.Atoi(trackMetadata.Format.Size)
				if err != nil {
					return err
				}
				createTrack.Size = int64(size)

				bitRate, err := strconv.Atoi(trackMetadata.Format.BitRate)
				if err != nil {
					return err
				}
				createTrack.BitRate = int32(bitRate)

				var streamIndex int

				for i, s := range trackMetadata.Streams {
					if s.CodecType == "audio" {
						streamIndex = i
						break
					}
				}

				sampleRate, err := strconv.Atoi(trackMetadata.Streams[streamIndex].SampleRate)
				if err != nil {
					return err
				}
				createTrack.SampleRate = int32(sampleRate)

				createTrack.Codec = trackMetadata.Streams[streamIndex].CodecName
				createTrack.Profile = trackMetadata.Streams[streamIndex].Profile
				createTrack.ChannelLayout = trackMetadata.Streams[streamIndex].ChannelLayout
				createTrack.Language = trackMetadata.Streams[streamIndex].Tags.Language

				track, err := qtx.CreateTrack(ctx, createTrack)
				if err != nil {
					return err
				}

				if trackMetadata.Format.Tags.Genre != "" {
					if strings.Contains(trackMetadata.Format.Tags.Genre, ",") {
						genreList := strings.Split(trackMetadata.Format.Tags.Genre, ",")

						for _, g := range genreList {
							err := helpers.SaveGenres(ctx, qtx, &helpers.SaveGenresParams{
								Tag:        g,
								GenreType:  "music",
								MusicianID: musician.ID,
								AlbumID:    album.ID,
								TrackID:    track.ID,
							})

							if err != nil {
								return err
							}
						}
					} else {
						err = helpers.SaveGenres(ctx, qtx, &helpers.SaveGenresParams{
							Tag:        trackMetadata.Format.Tags.Genre,
							GenreType:  "music",
							MusicianID: musician.ID,
							AlbumID:    album.ID,
							TrackID:    track.ID,
						})

						if err != nil {
							return err
						}
					}
				}

				err = tx.Commit(ctx)
				if err != nil {
					return err
				}
			}
		}

		return nil
	})

	if err != nil {
		log.Printf("your error is \n%v", err)
		return
	}

	app.Logger.Info("finished with music library scan")
}
