package main

import (
	"fmt"
	"os"
)

func (app *Application) ScanForMusicians() error {
	dirs, err := os.ReadDir(app.Settings.MusicDir)
	if err != nil {
		app.Logger.Error(fmt.Sprintf("unable to scan music directory at %s for musicians", app.Settings.MusicDir))
		app.Logger.Error(err.Error())
		return err
	}

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

	}

	return nil
}
