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
	queries                  *database.Queries
	logger                   logger.LoggerInterface
	ffprobe                  ffprobe.FfprobeInterface
	spotify                  spotifyapi.SpotifyInterface
	scanContext              context.Context
	launcher                 scanner.Launcher
	tx                       scanner.TxRunner
	currentMusicDirectory    func() sql.NullString
	invalidateCommittedTrack func(int64)
	statusMu                 sync.RWMutex
	status                   Status
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
		now:     deps.Now,
		queries: deps.Queries, logger: deps.Logger, ffprobe: deps.Ffprobe,
		spotify: deps.Spotify, scanContext: deps.ScanContext, launcher: scanner.Launcher{Wait: deps.Wait},
		tx:                       scanner.TxRunner{DB: deps.DB, Mu: deps.ScannerDBMu, Queries: deps.Queries},
		currentMusicDirectory:    deps.CurrentMusicDirectory,
		invalidateCommittedTrack: deps.InvalidateCommittedTrack,
	}
}
