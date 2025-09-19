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

// Scanner types for error handling and progress tracking
type ScanResult struct {
	Processed int
	Skipped   int
	Errors    []ScanError
	Duration  time.Duration
	StartTime time.Time
	EndTime   time.Time
}

type ScanError struct {
	FilePath   string
	Error      error
	ErrorType  string // "ffprobe", "spotify", "database", "filesystem"
	Timestamp  time.Time
	RetryCount int
}

type ScanStats struct {
	TotalFiles     int
	ProcessedFiles int
	SkippedFiles   int
	ErrorFiles     int
	CurrentFile    string
}
