package main

import (
	"fmt"
	"igloo/cmd/internal/helpers"
	"log"

	"github.com/fsnotify/fsnotify"
)

func (app *Application) StartMoviesWatcher() {
	for {
		select {
		case event, ok := <-app.Watcher.Events:
			if !ok {
				app.Logger.Error("unable to start movie watcher")
				return
			}

			switch {
			case event.Has(fsnotify.Create):
				isDir, err := helpers.IsDir(event.Name)
				if err != nil {
					app.Logger.Error(fmt.Sprintf("unable to detect if %s is a file or a directory", event.Name))
				} else {
					if isDir {
						app.Watcher.Add(event.Name)
					} else {
						isVideo := helpers.IsVideoFile(event.Name)
						if isVideo {
							titleYear, err := helpers.GetTitleAndYearFromFileName(event.Name)
							if err != nil {
								app.Logger.Error(fmt.Sprintf("unable to extract title and year from movie file %s", event.Name))
							} else {
								log.Printf("will add movie to db: %s", titleYear.Title)
							}
						}
					}
				}
			case event.Has(fsnotify.Remove):
				log.Printf("File removed: %s", event.Name)
			case event.Has(fsnotify.Rename):
				log.Printf("File renamed: %s", event.Name)
			}

		case err, ok := <-app.Watcher.Errors:
			if !ok {
				return
			}
			log.Printf("Error watching files: %v", err)
		}
	}
}
