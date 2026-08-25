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
//
// DB, Queries, Logger and Ffprobe are required. Tmdb is optional -- without it
// movies are indexed from their filenames alone. The remaining fields are
// defaulted by New, so a caller that does not care about scan cancellation,
// shutdown tracking, cross-scanner write serialization, or cache invalidation
// may leave them zero.
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

// New constructs a scanner, defaulting the optional dependencies so the rest
// of the package can use them unconditionally. The required ones are left
// as-is: startup keeps the same failure behavior as the previous
// application-owned implementation.
func New(deps Dependencies) *Scanner {
	if deps.ScanContext == nil {
		deps.ScanContext = context.Background()
	}
	if deps.Wait == nil {
		deps.Wait = &sync.WaitGroup{}
	}
	if deps.ScannerDBMu == nil {
		deps.ScannerDBMu = &sync.Mutex{}
	}
	if deps.CurrentMoviesDirectory == nil {
		deps.CurrentMoviesDirectory = func() sql.NullString { return sql.NullString{} }
	}
	if deps.InvalidateCommittedMovie == nil {
		deps.InvalidateCommittedMovie = func(int64) {}
	}

	return &Scanner{
		db: deps.DB, queries: deps.Queries, logger: deps.Logger, ffprobe: deps.Ffprobe,
		tmdb: deps.Tmdb, scanContext: deps.ScanContext, wait: deps.Wait,
		scannerDBMu: deps.ScannerDBMu, currentMoviesDirectory: deps.CurrentMoviesDirectory,
		invalidateCommittedMovie: deps.InvalidateCommittedMovie,
	}
}
