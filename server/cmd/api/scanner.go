package main

import (
	"context"
	"fmt"
	"igloo/cmd/internal/database"
	"os"
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
			Summary:           fmt.Sprintf("%s is a artist with %D followers and %d rating on spotify", artist.Name, artist.Followers, artist.Popularity),
		})

		if err != nil {
			return nil, fmt.Errorf("fail to save %s musician to data base\n%s", artist.Name, err.Error())
		}

		if len(artist.Images) > 0 {
			go func() {
				for _, img := range artist.Images {
					_, err = app.Queries.CreateSpotifyImage(context.Background(), database.CreateSpotifyImageParams{
						Path:       img.URL,
						Width:      int32(img.Width),
						Height:     int32(img.Height),
						MusicianID: musician.ID,
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
