package movie

import (
	"context"
	"database/sql"
	"sync"

	"igloo/cmd/internal/database"
	"igloo/cmd/internal/ffprobe"
	"igloo/cmd/internal/logger"
	"igloo/cmd/internal/scanner"
	"igloo/cmd/internal/tmdb"
)

// Dependencies are the application services used by Scanner. Callbacks keep
// the scanner independent from the HTTP application package.
type Dependencies struct {
	DB                       *sql.DB
	Queries                  *database.Queries
	Logger                   logger.LoggerInterface
	Ffprobe                  ffprobe.FfprobeInterface
	Tmdb                     tmdb.TmdbInterface
	ScanContext              context.Context
	Wait                     *sync.WaitGroup
	ScannerDBMu              *sync.Mutex
	CurrentMoviesDirectory   func() sql.NullString
	InvalidateCommittedMovie func(movieID int64)
}

// Scanner scans and persists the configured movie library.
type Scanner struct {
	db                       *sql.DB
	queries                  *database.Queries
	logger                   logger.LoggerInterface
	ffprobe                  ffprobe.FfprobeInterface
	tmdb                     tmdb.TmdbInterface
	scanContext              context.Context
	wait                     *sync.WaitGroup
	scannerDBMu              *sync.Mutex
	currentMoviesDirectory   func() sql.NullString
	invalidateCommittedMovie func(int64)
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

// New constructs a scanner. Dependencies are intentionally not validated so
// startup keeps the same failure behavior as the previous application-owned
// implementation.
func New(deps Dependencies) *Scanner {
	return &Scanner{
		db: deps.DB, queries: deps.Queries, logger: deps.Logger, ffprobe: deps.Ffprobe,
		tmdb: deps.Tmdb, scanContext: deps.ScanContext, wait: deps.Wait,
		scannerDBMu: deps.ScannerDBMu, currentMoviesDirectory: deps.CurrentMoviesDirectory,
		invalidateCommittedMovie: deps.InvalidateCommittedMovie,
	}
}

func (s *Scanner) context() context.Context {
	if s.scanContext != nil {
		return s.scanContext
	}

	return context.Background()
}
