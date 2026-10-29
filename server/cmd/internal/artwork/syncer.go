// Package artwork keeps a copy of the library's artwork in the static
// directory while the download_images setting is on, and points each image
// column at its copy once the copy is saved, so the API serves it in place of
// the provider's image. Turning the setting off points the columns back at
// the provider and deletes the copies.
package artwork

import (
	"context"
	"database/sql"
	"net/http"
	"sync"
	"time"

	"igloo/cmd/internal/database"
	"igloo/cmd/internal/helpers"
	"igloo/cmd/internal/logger"
	"igloo/cmd/internal/scanner"
)

const (
	// pageSize is how many catalog rows one listing query reads.
	pageSize = 500
	// downloadWorkers bounds the concurrent downloads of a pass.
	downloadWorkers = 4
	// applyBatchSize is how many column writes share one transaction.
	applyBatchSize = 50
	// failureRetryAfter is how long an image the provider refused, or sent
	// in a type Igloo does not store, waits before it is requested again.
	failureRetryAfter = 24 * time.Hour
	// progressLogInterval spaces the progress lines of a long pass.
	progressLogInterval = 10 * time.Second
)

// Dependencies are the application services a Syncer uses. DB, Queries and
// Logger are required; New defaults the rest.
type Dependencies struct {
	DB      *sql.DB
	Queries *database.Queries
	Logger  logger.LoggerInterface
	// ScannerDBMu is the scanners' write mutex, which the syncer's writes
	// share.
	ScannerDBMu *sync.Mutex
	// Context is canceled at shutdown, which stops a pass.
	Context context.Context
	Wait    *sync.WaitGroup
	// Enabled, StaticDir and LibraryDirs read the current settings.
	Enabled     func() bool
	StaticDir   func() string
	LibraryDirs func() []string
	// TmdbImageBaseURL is where TMDB images are fetched from.
	TmdbImageBaseURL func() string
	HTTPClient       *http.Client
	Now              func() time.Time
}

// Syncer runs the passes that keep the copies in line with the setting, one
// at a time on a goroutine of its own.
type Syncer struct {
	queries          *database.Queries
	logger           logger.LoggerInterface
	tx               scanner.TxRunner
	ctx              context.Context
	wait             *sync.WaitGroup
	enabled          func() bool
	staticDir        func() string
	libraryDirs      func() []string
	tmdbImageBaseURL func() string
	client           *http.Client
	now              func() time.Time

	mu      sync.Mutex
	running bool
	pending bool

	// Only the pass goroutine reads and writes these.
	failures map[string]time.Time
	// cleanedDir is the static directory the copies were last removed
	// from. Nothing can make copies while the setting stays off, so the
	// passes after scans skip it until the setting or the directory changes.
	cleanedDir string
	// reportedOverlaps names the managed folders last reported as
	// overlapping a library, so later passes do not report them again.
	reportedOverlaps string
}

func New(deps Dependencies) *Syncer {
	if deps.ScannerDBMu == nil {
		deps.ScannerDBMu = &sync.Mutex{}
	}
	if deps.Context == nil {
		deps.Context = context.Background()
	}
	if deps.Wait == nil {
		deps.Wait = &sync.WaitGroup{}
	}
	if deps.Enabled == nil {
		deps.Enabled = func() bool { return false }
	}
	if deps.StaticDir == nil {
		deps.StaticDir = func() string { return "" }
	}
	if deps.LibraryDirs == nil {
		deps.LibraryDirs = func() []string { return nil }
	}
	if deps.TmdbImageBaseURL == nil {
		deps.TmdbImageBaseURL = func() string { return helpers.TMDB_IMAGE_BASE_URL }
	}
	if deps.HTTPClient == nil {
		deps.HTTPClient = &http.Client{Timeout: helpers.PROVIDER_HTTP_TIMEOUT}
	}
	if deps.Now == nil {
		deps.Now = time.Now
	}

	return &Syncer{
		queries:          deps.Queries,
		logger:           deps.Logger,
		tx:               scanner.TxRunner{DB: deps.DB, Mu: deps.ScannerDBMu, Queries: deps.Queries},
		ctx:              deps.Context,
		wait:             deps.Wait,
		enabled:          deps.Enabled,
		staticDir:        deps.StaticDir,
		libraryDirs:      deps.LibraryDirs,
		tmdbImageBaseURL: deps.TmdbImageBaseURL,
		client:           deps.HTTPClient,
		now:              deps.Now,
		failures:         make(map[string]time.Time),
	}
}

// Request asks for a pass. Requests made while a pass runs share one pass
// after it, which reads the setting and the catalog as they are then.
func (s *Syncer) Request() {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.running {
		s.pending = true
		return
	}

	shuttingDown := s.ctx.Err() != nil
	if shuttingDown {
		return
	}

	s.running = true
	s.wait.Go(s.loop)
}

func (s *Syncer) loop() {
	for {
		s.pass(s.ctx)

		s.mu.Lock()
		again := s.pending && s.ctx.Err() == nil
		s.pending = false
		if !again {
			s.running = false
			s.mu.Unlock()
			return
		}
		s.mu.Unlock()
	}
}
