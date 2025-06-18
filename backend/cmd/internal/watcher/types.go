package watcher

import (
	"tmdb/cmd/tmdb"

	"github.com/fsnotify/fsnotify"
)

type WatcherClient struct {
	FileWatcher *fsnotify.Watcher
	Tmdb        tmdb.TmdbInterface
	MoviesDir   string
}
