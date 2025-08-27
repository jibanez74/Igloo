package main

import (
	"igloo/cmd/internal/database"
	"igloo/cmd/internal/ffprobe"
	"igloo/cmd/internal/logger"
	"igloo/cmd/internal/spotify"
	"igloo/cmd/internal/tmdb"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Application struct {
	Logger   logger.LoggerInterface
	Db       *pgxpool.Pool
	Queries  *database.Queries
	Settings *database.GlobalSetting
	Tmdb     tmdb.TmdbInterface
	Spotify  spotify.SpotifyInterface
	Ffprobe  ffprobe.FfprobeInterface
	Watcher  *fsnotify.Watcher
}

type CreateAlbumParams struct {
	AlbumTitle  string
	MusicianID  int32
	TotalTracks []int32
	DiscCount   []int32
	ReleaseDate time.Time
}
