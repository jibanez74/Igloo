package music

import (
	"context"
	"database/sql"
	"sync"
	"time"

	"igloo/cmd/internal/database"
	"igloo/cmd/internal/ffprobe"
	"igloo/cmd/internal/logger"
	"igloo/cmd/internal/scanner"
	spotifyapi "igloo/cmd/internal/spotify"
)

// Dependencies are the application services used by Scanner. DB, Queries,
// Logger and Ffprobe are required. Spotify and shutdown tracking are optional.
type Dependencies struct {
	// Now controls quiet-period eligibility and defaults to time.Now.
	Now                      func() time.Time
	DB                       *sql.DB
	Queries                  *database.Queries
	Logger                   logger.LoggerInterface
	Ffprobe                  ffprobe.FfprobeInterface
	Spotify                  spotifyapi.SpotifyInterface
	ScanContext              context.Context
	Wait                     *sync.WaitGroup
	ScannerDBMu              *sync.Mutex
	CurrentMusicDirectory    func() sql.NullString
	InvalidateCommittedTrack func(trackID int64)
}

// Scanner scans and persists the configured music library.
type Scanner struct {
	now                      func() time.Time
	db                       *sql.DB
	queries                  *database.Queries
	logger                   logger.LoggerInterface
	ffprobe                  ffprobe.FfprobeInterface
	spotify                  spotifyapi.SpotifyInterface
	scanContext              context.Context
	wait                     *sync.WaitGroup
	scannerDBMu              *sync.Mutex
	currentMusicDirectory    func() sql.NullString
	invalidateCommittedTrack func(int64)
	guard                    scanner.ScanGuard
}

// StartStatus describes whether a scan goroutine was launched.
type StartStatus int

const (
	StartStarted StartStatus = iota
	StartNotConfigured
	StartAlreadyRunning
)

// StartResult records the observed directory and start outcome.
type StartResult struct {
	Directory string
	Status    StartStatus
}

// New constructs a scanner and defaults optional lifecycle and cache callbacks.
func New(deps Dependencies) *Scanner {
	if deps.Now == nil {
		deps.Now = time.Now
	}
	if deps.ScanContext == nil {
		deps.ScanContext = context.Background()
	}
	if deps.Wait == nil {
		deps.Wait = &sync.WaitGroup{}
	}
	if deps.ScannerDBMu == nil {
		deps.ScannerDBMu = &sync.Mutex{}
	}
	if deps.CurrentMusicDirectory == nil {
		deps.CurrentMusicDirectory = func() sql.NullString { return sql.NullString{} }
	}
	if deps.InvalidateCommittedTrack == nil {
		deps.InvalidateCommittedTrack = func(int64) {}
	}
	return &Scanner{
		now: deps.Now,
		db:  deps.DB, queries: deps.Queries, logger: deps.Logger, ffprobe: deps.Ffprobe,
		spotify: deps.Spotify, scanContext: deps.ScanContext, wait: deps.Wait,
		scannerDBMu: deps.ScannerDBMu, currentMusicDirectory: deps.CurrentMusicDirectory,
		invalidateCommittedTrack: deps.InvalidateCommittedTrack,
	}
}
