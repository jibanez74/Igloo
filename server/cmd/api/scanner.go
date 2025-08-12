package main

import (
	"context"
	"fmt"
	"igloo/cmd/internal/database"
	"igloo/cmd/internal/helpers"
	"os"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
)

func (app *Application) ScanDirsForMusicians(dir os.DirEntry) (*database.Musician, error) {
	artistList, err := app.Spotify.SearchAlbums(dir.Name(), 1)
	if err != nil {
		return nil, fmt.Errorf("fail to fetch results from spotify for musician %s\n%s", dir.Name(), err.Error())
	}

	artist, err := app.Spotify.GetArtistBySpotifyID(artistList[0].ID.String())
	if err != nil {
		return nil, fmt.Errorf("fail to get details for musician %s\n%s", dir.Name(), err.Error())
	}

	exist, err := app.Queries.CheckMusicianExistsBySpotifyID(context.Background(), artist.ID.String())
	if err != nil {
		return nil, fmt.Errorf("fail to check if %s exists in the data base\n%s", artist.Name, err.Error())
	}

	var musician database.Musician

	if !exist {
		musician, err = app.Queries.CreateMusician(context.Background(), database.CreateMusicianParams{
			SpotifyID:         artist.ID.String(),
			Name:              artist.Name,
			SpotifyPopularity: int32(artist.Popularity),
			SpotifyFollowers:  int32(artist.Followers.Count),
			Summary:           fmt.Sprintf("%s is a talented artist with %d followers and a popularity rating of %d on Spotify. Their music has resonated with listeners worldwide, establishing them as a notable presence in the music industry.", artist.Name, artist.Followers.Count, artist.Popularity),
		})

		if err != nil {
			return nil, fmt.Errorf("fail to save %s musician to data base\n%s", artist.Name, err.Error())
		}

		if len(artist.Images) > 0 {
			go func() {
				for _, img := range artist.Images {
					_, err = app.Queries.CreateSpotifyImage(context.Background(), database.CreateSpotifyImageParams{
						Path:   img.URL,
						Width:  int32(img.Width),
						Height: int32(img.Height),
						MusicianID: pgtype.Int4{
							Int32: musician.ID,
							Valid: true,
						},
					})

					if err != nil {
						app.Logger.Error(fmt.Sprintf("fail to save image %s for musician %s\n%s", img.URL, musician.Name, err.Error()))
					}
				}
			}()
		}

	} else {
		musician, err = app.Queries.GetMusicianBySpotifyID(context.Background(), artist.ID.String())
		if err != nil {
			return nil, fmt.Errorf("fail to fetch %s with spotify id %s from the data base\n%s", artist.Name, artist.ID, err.Error())
		}
	}

	return &musician, nil
}

func (app *Application) ScanDIrsForAlbums(dir os.DirEntry, musicianID int32) (*database.Album, error) {
	albumList, err := app.Spotify.SearchAlbums(dir.Name(), 1)
	if err != nil {
		return nil, fmt.Errorf("fail to get data for album %s\n%s", dir.Name(), err.Error())
	}

	if len(albumList) == 0 {
		return nil, fmt.Errorf("fail to fetch any data from spotify for album %s", dir.Name())
	}

	album, err := app.Spotify.GetAlbumBySpotifyID(albumList[0].ID.String())
	if err != nil {
		return nil, fmt.Errorf("fail to fetch details for %s with spotify id %s\n%s", dir.Name(), albumList[0].ID.String(), err.Error())
	}

	exist, err := app.Queries.CheckAlbumExistsBySpotifyID(context.Background(), album.ID.String())
	if err != nil {
		return nil, fmt.Errorf("fail to check if %s album exist in the data base\n%s", album.Name, err.Error())
	}

	var dbAlbum database.Album

	if !exist {
		releaseDate, err := helpers.FormatDate(album.ReleaseDate)
		if err != nil {
			app.Logger.Error(fmt.Sprintf("fail to format release date for album %s\n%s", album.Name, err.Error()))
			releaseDate = time.Now()
		}

		tx, err := app.Db.Begin(context.Background())
		if err != nil {
			return nil, fmt.Errorf("fail to start data base transaction to save album %s data\n%s", album.Name, err.Error())
		}
		defer tx.Rollback(context.Background())

		qtx := app.Queries.WithTx(tx)

		dbAlbum, err = qtx.CreateAlbum(context.Background(), database.CreateAlbumParams{
			Title:                album.Name,
			SpotifyID:            album.ID.String(),
			TotalTracks:          int32(album.TotalTracks),
			TotalAvailableTracks: int32(album.TotalTracks), // Assuming all tracks are available initially
			SpotifyPopularity:    int32(album.Popularity),
			ReleaseDate: pgtype.Date{
				Time:  releaseDate,
				Valid: true,
			},
		})

		if err != nil {
			return nil, fmt.Errorf("fail to create %s album in the data base\n%s", album.Name, err.Error())
		}

		err = qtx.CreateAlbumMusician(context.Background(), database.CreateAlbumMusicianParams{
			MusicianID: musicianID,
			AlbumID:    dbAlbum.ID,
		})

		if err != nil {
			return nil, fmt.Errorf("fail to create relation between musciian with id of %d and album with id of %d\n%s", musicianID, dbAlbum.ID, err.Error())
		}

		err = tx.Commit(context.Background())
		if err != nil {
			return nil, fmt.Errorf("fail to commit transaction to save album %s to data base\n%s", album.Name, err.Error())
		}
	} else {
		dbAlbum, err = app.Queries.GetAlbumBySpotifyID(context.Background(), album.ID.String())
		if err != nil {
			return nil, fmt.Errorf("fail to fetch album %s from data base\n%s", album.Name, err.Error())
		}
	}

	return &dbAlbum, nil
}
