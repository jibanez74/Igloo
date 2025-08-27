package main

import (
	"context"
	"errors"
	"fmt"
	"igloo/cmd/internal/database"
	"igloo/cmd/internal/helpers"
	"io/fs"
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

		ext := strings.ToLower(strings.TrimPrefix(entry.Name(), "."))

		if helpers.ValidAudioExtensions[ext] {
			trackMetadata, err := app.Ffprobe.GetTrackMetadata(path)
			if err != nil {
				return err
			}

			if len(trackMetadata.Streams) == 0 {
				return fmt.Errorf("ffprobe detected no streams for track file %s", path)
			}

			musician, err := app.GetOrCreateMusician(ctx, trackMetadata.Format.Tags.Artist)
			if err != nil {
				return err
			}

			data := CreateAlbumParams{
				AlbumTitle:  trackMetadata.Format.Tags.Album,
				MusicianID:  musician.ID,
				TotalTracks: []int32{0, 0},
				DiscCount:   []int32{0, 0},
			}

			if trackMetadata.Format.Tags.Date != "" {
				date, err := helpers.FormatDate(trackMetadata.Format.Tags.Date)
				if err != nil {
					app.Logger.Error(fmt.Sprintf("fail to parse date for album %s\n%s", data.AlbumTitle, err.Error()))
				} else {
					data.ReleaseDate = date
				}
			}

			if trackMetadata.Format.Tags.Track != "" {
				tracks, err := helpers.SplitSliceBySlash(trackMetadata.Format.Tags.Track)
				if err != nil {
					app.Logger.Error(fmt.Sprintf("fail to get track count for album %s\n%s", data.AlbumTitle, err.Error()))
				} else {
					data.TotalTracks = tracks
				}
			}

			if trackMetadata.Format.Tags.Disc != "" {
				disc, err := helpers.SplitSliceBySlash(trackMetadata.Format.Tags.Disc)
				if err != nil {
					app.Logger.Error(fmt.Sprintf("fail to get disc count for album %s\n%s", data.AlbumTitle, err.Error()))
				} else {
					data.DiscCount = disc
				}
			}

			album, err := app.GetOrCreateAlbum(ctx, &data)
			if err != nil {
				return err
			}

			createTrack := database.CreateTrackParams{
				Title:       trackMetadata.Format.Tags.Title,
				Index:       data.TotalTracks[0],
				Disc:        data.DiscCount[0],
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

			duration, err := helpers.GetPreciseDecimalFromStr(trackMetadata.Format.Duration)
			if err != nil {
				return err
			}
			createTrack.Duration = duration

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

			track, err := app.Queries.CreateTrack(ctx, createTrack)
			if err != nil {
				return err
			}

			if trackMetadata.Format.Tags.Genre != "" {
				if strings.Contains(trackMetadata.Format.Tags.Genre, ",") {
					genreList := strings.Split(trackMetadata.Format.Tags.Genre, ",")

					for _, g := range genreList {
						app.SaveGenres(ctx, &SaveGenresParams{
							Tag:        g,
							GenreType:  "music",
							MusicianID: musician.ID,
							AlbumID:    album.ID,
							TrackID:    track.ID,
						})
					}

				} else {
					app.SaveGenres(ctx, &SaveGenresParams{
						Tag:        trackMetadata.Format.Tags.Genre,
						GenreType:  "music",
						MusicianID: data.MusicianID,
						AlbumID:    album.ID,
						TrackID:    track.ID,
					})
				}
			}
		}

		return nil
	})

	if err != nil {
		app.Logger.Error(fmt.Sprintf("fail to scan music directory at %s\n%s", app.Settings.MusicDir, err.Error()))
	}
}

func (app *Application) GetOrCreateMusician(ctx context.Context, name string) (*database.Musician, error) {
	if name == "" {
		return nil, errors.New("got empty name for ScanDirForMusician function")
	}

	artistList, err := app.Spotify.SearchArtistByName(name)
	if err != nil {
		return nil, err
	}

	exist, err := app.Queries.CheckMusicianExistsBySpotifyID(ctx, artistList.ID.String())
	if err != nil {
		return nil, err
	}

	var musician database.Musician

	if exist {
		musician, err = app.Queries.GetMusicianBySpotifyID(ctx, artistList.ID.String())
		if err != nil {
			return nil, err
		}
	} else {
		thumb := ""
		if len(artistList.Images) > 0 {
			thumb = artistList.Images[0].URL
		}

		musician, err = app.Queries.CreateMusician(ctx, database.CreateMusicianParams{
			Name:              artistList.Name,
			SpotifyID:         artistList.ID.String(),
			SpotifyPopularity: int32(artistList.Popularity),
			SpotifyFollowers:  int32(artistList.Followers.Count),
			Summary:           fmt.Sprintf("%s is an artist on spotify with a popularity of %d and %d followers", artistList.Name, artistList.Popularity, artistList.Followers.Count),
			Thumb:             thumb,
		})
	}

	return &musician, nil
}

func (app *Application) GetOrCreateAlbum(ctx context.Context, data *CreateAlbumParams) (*database.Album, error) {
	if data == nil {
		return nil, errors.New("got nil value for data in GetOrCreateAlbum function")
	}

	albumList, err := app.Spotify.SearchAlbums(data.AlbumTitle, 1)
	if err != nil {
		return nil, err
	}

	if len(albumList) == 0 {
		return nil, fmt.Errorf("no results were returned from spotify for album %s", data.AlbumTitle)
	}

	exist, err := app.Queries.CheckAlbumExistsBySpotifyID(ctx, albumList[0].ID.String())
	if err != nil {
		return nil, err
	}

	var album database.Album

	if exist {
		album, err = app.Queries.GetAlbumBySpotifyID(ctx, albumList[0].ID.String())
		if err != nil {
			return nil, err
		}
	} else {
		createAlbum := database.CreateAlbumParams{
			Title:             albumList[0].Name,
			SpotifyID:         albumList[0].ID.String(),
			DiscCount:         data.DiscCount[1],
			TotalTracks:       data.TotalTracks[1],
			SpotifyPopularity: 0,
			ReleaseDate: pgtype.Date{
				Time:  data.ReleaseDate,
				Valid: true,
			},
			MusicianID: pgtype.Int4{
				Int32: data.MusicianID,
				Valid: true,
			},
		}

		if len(albumList[0].Images) > 0 {
			createAlbum.Cover = albumList[0].Images[0].URL
		}

		albumDetails, err := app.Spotify.GetAlbumBySpotifyID(albumList[0].ID.String())
		if err != nil {
			app.Logger.Error(fmt.Sprintf("fail to get details from spotify for album %s\n%s", albumList[0].Name, err.Error()))
		} else {
			createAlbum.SpotifyPopularity = int32(albumDetails.Popularity)
		}

		album, err = app.Queries.CreateAlbum(ctx, createAlbum)
		if err != nil {
			return nil, err
		}
	}

	return &album, nil
}

func (app *Application) SaveGenres(ctx context.Context, data *SaveGenresParams) {
	if data == nil {
		app.Logger.Error(fmt.Sprintf("got nil value for data in SaveGenres function"))
		return
	}

	if data.Tag == "" {
		app.Logger.Error("got empty data.Tag string in SaveGenres function")
		return
	}

	if data.GenreType == "" {
		app.Logger.Error("got empty string for data.GenreType in SaveGenres function")
		return
	}

	tag := strings.ToLower(strings.TrimSpace(data.Tag))

	genre, err := app.Queries.UpsertGenre(ctx, database.UpsertGenreParams{
		Tag:       tag,
		GenreType: data.GenreType,
	})

	if err != nil {
		app.Logger.Error(fmt.Sprintf("fail to save genre %s\n%s", data.Tag, err.Error()))
		return
	}

	if genre.GenreType == "music" {
		err = app.Queries.CreateMusicianGenre(ctx, database.CreateMusicianGenreParams{
			MusicianID: data.MusicianID,
			GenreID:    genre.ID,
		})

		if err != nil {
			app.Logger.Error(fmt.Sprintf("fail to save relationship between musician and genre\n%s", err.Error()))
			return
		}

		err = app.Queries.CreateAlbumGenre(ctx, database.CreateAlbumGenreParams{
			AlbumID: data.AlbumID,
			GenreID: genre.ID,
		})

		if err != nil {
			app.Logger.Error(fmt.Sprintf("fail to save relationship between album and genre\n%s", err.Error()))
			return
		}

		err = app.Queries.CreateTrackGenre(ctx, database.CreateTrackGenreParams{
			TrackID: data.TrackID,
			GenreID: genre.ID,
		})

		if err != nil {
			app.Logger.Error(fmt.Sprintf("fail to save relationship between track and genre\n%s", err.Error()))
			return
		}
	}

	if genre.GenreType == "movie" {
		err = app.Queries.AddMovieGenre(ctx, database.AddMovieGenreParams{
			MovieID: data.MovieID,
			GenreID: genre.ID,
		})

		if err != nil {
			app.Logger.Error(fmt.Sprintf("fail to save relationship between movie and genre\n%s", err.Error()))
			return
		}
	}
}
