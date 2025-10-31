package main

import (
	"igloo/cmd/internal/database"
	"igloo/cmd/internal/ffprobe"
	"igloo/cmd/internal/logger"
	"igloo/cmd/internal/spotify"
	"igloo/cmd/internal/tmdb"

	"github.com/alexedwards/scs/v2"
	"github.com/fsnotify/fsnotify"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Application struct {
	Logger   logger.LoggerInterface
	Db       *pgxpool.Pool
	Queries  *database.Queries
	Settings *database.GlobalSetting
	Session  *scs.SessionManager
	Tmdb     tmdb.TmdbInterface
	Spotify  spotify.SpotifyInterface
	Ffprobe  ffprobe.FfprobeInterface
	Watcher  *fsnotify.Watcher
}

type AuthRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}
