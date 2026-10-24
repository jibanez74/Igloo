package music

import (
	"context"
	"database/sql"
	"sync"
	"time"

	"igloo/cmd/internal/database"
	"igloo/cmd/internal/ffprobe"
	"igloo/cmd/internal/itunes"
	"igloo/cmd/internal/logger"
	"igloo/cmd/internal/scanner"
	spotifyapi "igloo/cmd/internal/spotify"
)

// SpotifyProvider is the part of the Spotify client the scanner uses: name
// searches for artists and albums, and id lookups for the artist an album
// credits and for an album's track list.
type SpotifyProvider interface {
	SearchArtistByName(ctx context.Context, name string) (*spotifyapi.FullArtist, error)
	GetArtist(ctx context.Context, id spotifyapi.ID) (*spotifyapi.FullArtist, error)
	MatchAlbum(ctx context.Context, query spotifyapi.AlbumQuery) (*spotifyapi.FullAlbum, error)
	GetAlbum(ctx context.Context, id spotifyapi.ID) (*spotifyapi.FullAlbum, error)
}

// AppleProvider is the part of the iTunes client the scanner uses: album
// lookups by the collection ids iTunes purchases carry.
type AppleProvider interface {
	LookupAlbums(ctx context.Context, ids []int64) (map[int64]itunes.Album, error)
}

// Dependencies are the application services used by Scanner. DB, Queries,
// Logger and Ffprobe are required. Spotify, Apple and shutdown tracking are
// optional. AppleLookupsEnabled is read when each scan starts and defaults to
// true.
type Dependencies struct {
	// Now controls quiet-period eligibility and defaults to time.Now.
	Now                      func() time.Time
	DB                       *sql.DB
	Queries                  *database.Queries
	Logger                   logger.LoggerInterface
	Ffprobe                  ffprobe.FfprobeInterface
	Spotify                  SpotifyProvider
	Apple                    AppleProvider
	AppleLookupsEnabled      func() bool
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
	spotify                  SpotifyProvider
	apple                    AppleProvider
	appleLookupsEnabled      func() bool
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
	if deps.AppleLookupsEnabled == nil {
		deps.AppleLookupsEnabled = func() bool { return true }
	}
	if deps.InvalidateCommittedTrack == nil {
		deps.InvalidateCommittedTrack = func(int64) {}
	}
	return &Scanner{
		now:     deps.Now,
		queries: deps.Queries, logger: deps.Logger, ffprobe: deps.Ffprobe,
		spotify: deps.Spotify, apple: deps.Apple, appleLookupsEnabled: deps.AppleLookupsEnabled,
		scanContext: deps.ScanContext, launcher: scanner.Launcher{Wait: deps.Wait},
		tx:                       scanner.TxRunner{DB: deps.DB, Mu: deps.ScannerDBMu, Queries: deps.Queries},
		currentMusicDirectory:    deps.CurrentMusicDirectory,
		invalidateCommittedTrack: deps.InvalidateCommittedTrack,
	}
}
