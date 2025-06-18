package watcher

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"tmdb/cmd/helpers"
	"tmdb/cmd/tmdb"

	"github.com/fsnotify/fsnotify"
)

func New(t tmdb.TmdbInterface) (*WatcherClient, error) {
	w, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, fmt.Errorf("failed to create watcher: %v", err)
	}

	if t == nil {
		return nil, fmt.Errorf("tmdb client is nil")
	}

	return &WatcherClient{
		FileWatcher: w,
		Tmdb:        t,
		MoviesDir:   os.Getenv("MOVIES_DIR"),
	}, nil
}

func (w *WatcherClient) MonitorMoviesDir() error {
	err := w.FileWatcher.Add(w.MoviesDir)
	if err != nil {
		return fmt.Errorf("failed to watch directory: %v", err)
	}

	err = filepath.Walk(w.MoviesDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if info.IsDir() {
			w.FileWatcher.Add(path)
		}

		return nil
	})

	if err != nil {
		return fmt.Errorf("failed to watch subdirectories: %v", err)
	}

	go func() {
		for {
			select {
			case event, ok := <-w.FileWatcher.Events:
				if !ok {
					return
				}

				switch {
				case event.Has(fsnotify.Create):
					log.Printf("New file created: %s", event.Name)

					err = helpers.CheckDirExistAndReadable(event.Name)
					if err == nil {
						w.FileWatcher.Add(event.Name)
					} else {
						if helpers.IsVideoFile(event.Name) {
							log.Printf("New video file detected: %s", event.Name)

							titleYear, err := helpers.GetTitleAndYearFromFileName(event.Name)
							if err != nil {
								log.Printf("Error extracting title and year from filename: %v", err)
							} else {
								movies, err := w.Tmdb.SearchMoviesByTitleAndYear(titleYear.Title, titleYear.Year)
								if err != nil {
									log.Printf("Error searching for movie: %v", err)
								} else {
									if len(movies) > 0 {
										log.Printf("Found %d movies for %s (%d)", len(movies), titleYear.Title, titleYear.Year)
									} else {
										log.Printf("No movies found for %s (%d)", titleYear.Title, titleYear.Year)
									}
								}
							}
						}
					}

				case event.Has(fsnotify.Remove):
					log.Printf("File removed: %s", event.Name)
				case event.Has(fsnotify.Rename):
					log.Printf("File renamed: %s", event.Name)
				}

			case err, ok := <-w.FileWatcher.Errors:
				if !ok {
					return
				}
				log.Printf("Error watching files: %v", err)
			}
		}
	}()

	return nil
}

func (w *WatcherClient) Stop() error {
	return w.FileWatcher.Close()
}
