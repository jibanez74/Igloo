package main
import (
	"igloo/cmd/internal/database"
	"igloo/cmd/internal/ffprobe"
	"igloo/cmd/internal/logger"
	"igloo/cmd/internal/spotify"
	"igloo/cmd/internal/tmdb"

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

type CreateTmdbMovieRequest struct {
	FilePath string `json:"file_path"`
	TmdbID   string `json:"tmdb_id"`
}
