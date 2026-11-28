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
	// ScanFinished runs when each run ends, to download the artwork it
	// brought; scoped marks a run of watcher-detected changes.
	ScanFinished func(scoped bool)
	// Pictures extracts the pictures tracks embed; without it albums use
	// only folder images.
	Pictures PictureExtractor
	// StaticDir and LibraryDirs read the current settings: album art is
	// copied under the static directory, never into a library.
	StaticDir   func() string
	LibraryDirs func() []string
	// LookupsChanged runs when the background worker changed album covers,
	// to download the provider covers it handed back.
	LookupsChanged func()
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
	mountPoints              func() ([]string, error)
	outstanding              scanner.Outstanding
	currentMusicDirectory    func() sql.NullString
	invalidateCommittedTrack func(int64)
	pictures                 PictureExtractor
	staticDir                func() string
	libraryDirs              func() []string
	lookupsChanged           func()
	lookups                  *lookups
	statusMu                 sync.RWMutex
	status                   Status
	lookupStatus             LookupStatus
	revision                 scanner.Revision
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
	if deps.StaticDir == nil {
		deps.StaticDir = func() string { return "" }
	}
	if deps.LibraryDirs == nil {
		deps.LibraryDirs = func() []string { return nil }
	}
	if deps.LookupsChanged == nil {
		deps.LookupsChanged = func() {}
	}
	s := &Scanner{
		now:     deps.Now,
		queries: deps.Queries, logger: deps.Logger, ffprobe: deps.Ffprobe,
		spotify: deps.Spotify, apple: deps.Apple, appleLookupsEnabled: deps.AppleLookupsEnabled,
		scanContext: deps.ScanContext, launcher: scanner.Launcher{Wait: deps.Wait, Finished: deps.ScanFinished},
		tx:                       scanner.TxRunner{DB: deps.DB, Mu: deps.ScannerDBMu, Queries: deps.Queries},
		mountPoints:              scanner.MountPoints,
		currentMusicDirectory:    deps.CurrentMusicDirectory,
		invalidateCommittedTrack: deps.InvalidateCommittedTrack,
		pictures:                 deps.Pictures,
		staticDir:                deps.StaticDir,
		libraryDirs:              deps.LibraryDirs,
		lookupsChanged:           deps.LookupsChanged,
	}
	s.lookups = &lookups{ctx: func() context.Context { return s.scanContext }, wait: deps.Wait, run: s.runLookups}
	return s
}
