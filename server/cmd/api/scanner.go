package main

import (
	"context"
	"fmt"
	"igloo/cmd/internal/database"
	"os"
)

func (app *Application) ScanForMusicians() error {
	dirs, err := os.ReadDir(app.Settings.MusicDir)
	if err != nil {
		app.Logger.Error(fmt.Sprintf("unable to scan music directory at %s for musicians", app.Settings.MusicDir))
		app.Logger.Error(err.Error())
		return err
	}

	var musicianList []database.Musician

	if len(dirs) == 0 {
		app.Logger.Info(fmt.Sprintf("no musicians at %s", app.Settings.MusicDir))
		return nil
	}

	for _, d := range dirs {
		if !d.IsDir() {
			continue
		}

		artistList, err := app.Spotify.SearchArtists(d.Name(), 1)
		if err != nil {
			app.Logger.Error(fmt.Sprintf("unable to fetch data for musician %s", d.Name()))
			app.Logger.Error(err.Error())
			continue
		}

		if len(artistList) == 0 {
			app.Logger.Info(fmt.Sprintf("no results on spotify for musician %s", d.Name()))
			continue
		}

		artist, err := app.Spotify.GetArtistBySpotifyID(artistList[0].ID.String())
		if err != nil {
			app.Logger.Error(fmt.Sprintf("an error happened while fetching the details from musician %s from spotify", artistList[0].Name))
			app.Logger.Error(err.Error())
		}

		exist, err := app.Queries.CheckMusicianExistsBySpotifyID(context.Background(), artist.ID.String())
		if err != nil {
			app.Logger.Error(fmt.Sprintf("unable to check if musician %s with id of %s is already in the system", artist.Name, artist.ID))
			continue
		}

		if exist {
			continue
		}

		musician, err := app.Queries.CreateMusician(context.Background(), database.CreateMusicianParams{
			Name:              artist.Name,
			SpotifyID:         artist.ID.String(),
			SpotifyPopularity: int32(artist.Popularity),
			SpotifyFollowers:  int32(artist.Followers.Count),
			Summary:           fmt.Sprintf("% is a artist with a rating of %d and %d followers on spotify", artist.Name, artist.Popularity, artist.Followers.Count),
		})

		if err != nil {
			app.Logger.Error(fmt.Sprintf("unable to create musician %s", artist.Name))
			app.Logger.Info(err.Error())
			continue
		}

		musicianList = append(musicianList, musician)

		if len(artist.Images) > 0 {
			for _, img := range artist.Images {
				_, err = app.Queries.CreateSpotifyImage(context.Background(), database.CreateSpotifyImageParams{
					Path:       img.URL,
					Width:      int32(img.Width),
					Height:     int32(img.Height),
					MusicianID: musician.ID,
				})

				if err != nil {
					app.Logger.Error(fmt.Sprintf("unable to save musician image for url %s", img.URL))
					app.Logger.Error(err.Error())
				}
			}
		}
	}

	return nil
}
