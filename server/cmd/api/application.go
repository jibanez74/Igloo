package main

import (
	"context"
	"database/sql"
	"embed"
	"fmt"
	"io/fs"
	"net/http"
	"sync"

	"igloo/cmd/internal/database"
	"igloo/cmd/internal/ffmpeg"
	"igloo/cmd/internal/ffprobe"
	"igloo/cmd/internal/helpers"
	"igloo/cmd/internal/itunes"
	applogger "igloo/cmd/internal/logger"
	"igloo/cmd/internal/preroll"
	"igloo/cmd/internal/scanner"
	"igloo/cmd/internal/scanner/movie"
	"igloo/cmd/internal/scanner/music"
	"igloo/cmd/internal/scanner/show"
	"igloo/cmd/internal/scanner/watch"
	"igloo/cmd/internal/spotify"
	"igloo/cmd/internal/tmdb"

	"github.com/alexedwards/scs/v2"
	"github.com/go-chi/chi/v5"
	"github.com/patrickmn/go-cache"
	"golang.org/x/sync/singleflight"
)

type Application struct {
	DB      *sql.DB
	Queries *database.Queries

	// settings is the in-memory copy of the settings row, read concurrently and
	// replaced by the admin writers. Outside startup go through
	// CurrentSettings/SetSettings, or withSettingsWrite for a read-modify-write.
	// InitSettings fails the boot without a row, so it is never nil.
	settingsMu sync.RWMutex
	settings   *database.Setting
	// settingsWriteMu serializes withSettingsWrite, so settingsMu is held only
	// to swap the pointer.
	settingsWriteMu sync.Mutex

	Config RuntimeConfig
	// FrontendAssets is the filesystem ServeFrontend serves the SPA from:
	// the embedded webdist in production, a test filesystem in tests. The
	// decoded assets are cached per application on first use.
	FrontendAssets          fs.FS
	frontendAssetsOnce      sync.Once
	frontendAssets          map[string]*frontendAsset
	Logger                  applogger.LoggerInterface
	LoggerCloser            func() error
	Ffprobe                 ffprobe.FfprobeInterface
	FFmpeg                  ffmpeg.FFmpegInterface
	Spotify                 spotifySearcher
	Tmdb                    tmdb.TmdbInterface
	PrerollTheaters         *preroll.TheatersPool
	TmdbImageBaseURL        string
	TmdbImageHTTPClient     *http.Client
	YouTubeThumbBaseURL     string
	YouTubeThumbHTTPClient  *http.Client
	SessionManager          *scs.SessionManager
	Wait                    *sync.WaitGroup
	Router                  *chi.Mux
	Server                  *http.Server
	ScannerDBMu             sync.Mutex
	SearchVocab             searchVocabCache
	HLSSessionCache         *cache.Cache
	HLSSessionGroup         singleflight.Group
	HLSTranscodeLimiterMu   sync.Mutex
	HLSCPUTranscodeLimiter  *hlsTranscodeLimiter
	HLSHWTranscodeLimiter   *hlsTranscodeLimiter
	PersonalHLSMu           sync.Mutex
	PersonalHLSReservations map[int64]int
	// HLSShutdown is cancelled when shutdown begins (beginHLSShutdown): from
	// then on no HLS session starts or is published, and a start parked for a
	// transcode permit gives up.
	HLSShutdown      context.Context
	beginHLSShutdown context.CancelFunc

	// HLSMaxPersonalSessionsPerUser caps concurrent personal HLS sessions per
	// user; zero falls back to hlsMaxPersonalSessionsPerUserDefault.
	HLSMaxPersonalSessionsPerUser int
	SubtitleVTTCache              *cache.Cache
	SubtitleExtractGroup          singleflight.Group
	KeyframeExtractGroup          singleflight.Group
	RoomHLSTombstone              *cache.Cache
	RoomHLSMu                     sync.Mutex
	WatchRoomHub                  *WatchRoomHub
	QuickConnect                  *QuickConnectBroker
	AuthLimiter                   *rateLimiter
	DeviceLastSeen                *cache.Cache
	DeviceAuthCache               *cache.Cache
	SessionVersionCache           *cache.Cache
	WatchRoomAuthCache            *watchRoomAuthCache
	StreamFileCache               *generationCache[streamFile]
	MovieStreamsCache             *generationCache[movieStreams]
	DeviceExpiryCancel            context.CancelFunc
	ScanCancel                    context.CancelFunc
	ScanContext                   context.Context
	MovieScanner                  interface {
		Start() scanner.StartResult
		StartChanges(scanner.Changes) scanner.StartResult
		Status() movie.Status
	}
	ShowScanner interface {
		Start() scanner.StartResult
		StartChanges(scanner.Changes) scanner.StartResult
		Status() show.Status
	}
	MusicScanner interface {
		Start() scanner.StartResult
		StartChanges(scanner.Changes) scanner.StartResult
		Status() music.Status
	}
	// Watcher follows the library roots while enable_watcher is on; Sync
	// starts and stops it.
	Watcher *watch.Watcher
}

//go:embed all:webdist
var FrontendFS embed.FS

func InitApp() (initializedApp *Application, err error) {
	config, err := NewRuntimeConfig()
	if err != nil {
		return nil, fmt.Errorf("failed to load runtime config: %v", err)
	}

	app := Application{
		Config:         config,
		FrontendAssets: FrontendFS,
		Wait:           &sync.WaitGroup{},
	}
	defer func() {
		if err != nil {
			app.cleanupStartupResources()
		}
	}()

	ctx := context.Background()

	// Logger comes first so later startup failures are captured consistently.
	err = app.InitLogger()
	if err != nil {
		return nil, fmt.Errorf("failed to initialize logger: %v", err)
	}

	err = app.InitDB()
	if err != nil {
		return nil, fmt.Errorf("failed to initialize database: %v", err)
	}

	err = app.InitTables()
	if err != nil {
		return nil, fmt.Errorf("failed to initialize database tables: %v", err)
	}

	err = app.RefreshQueryPlannerStats()
	if err != nil {
		return nil, fmt.Errorf("failed to refresh query planner statistics: %v", err)
	}

	app.Queries, err = database.Prepare(ctx, app.DB)
	if err != nil {
		return nil, fmt.Errorf("failed to prepare database queries: %v", err)
	}

	err = app.InitSettings(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize settings: %v", err)
	}

	app.WatchRoomHub = NewWatchRoomHub()

	// Directory paths come from settings, so this must run after InitSettings.
	err = app.InitDirs()
	if err != nil {
		return nil, fmt.Errorf("failed to initialize directories: %v", err)
	}

	// Remove any leftover HLS temp directories from a previous run that did not
	// shut down cleanly (crash, SIGKILL from systemd, power loss, etc.).
	settings := app.CurrentSettings()
	cleanupStaleHLSTempDirs(app.Logger, settings.TranscodeDir)

	err = app.InitDefaultUser(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize default user: %v", err)
	}

	app.InitSession()

	ffprobeApp, err := ffprobe.New()
	if err != nil {
		return nil, fmt.Errorf("failed to initialize ffprobe: %v", err)
	}
	app.Ffprobe = ffprobeApp

	ffmpegApp, err := ffmpeg.New()
	if err != nil {
		return nil, fmt.Errorf("failed to initialize ffmpeg: %v", err)
	}
	app.FFmpeg = ffmpegApp

	if settings.TmdbKey.Valid {
		tmdb, err := tmdb.New(settings.TmdbKey.String)
		if err != nil {
			app.Logger.Warn("failed to initialize tmdb client", "error", err)
		} else {
			app.Tmdb = tmdb
			app.Logger.Info("tmdb client initialized successfully")
		}
	}

	// The one client serves the request pickers through app.Spotify and the
	// music scanner through its own narrower view. Both stay nil interfaces
	// when Spotify is not configured; a typed nil would read as available.
	var musicSpotify music.SpotifyProvider
	if settings.SpotifyClientID.Valid && settings.SpotifyClientID.String != "" &&
		settings.SpotifyClientSecret.Valid && settings.SpotifyClientSecret.String != "" {
		spotifyClient, err := spotify.New(
			ctx,
			settings.SpotifyClientID.String,
			settings.SpotifyClientSecret.String,
		)

		if err != nil {
			app.Logger.Warn("failed to initialize spotify client", "error", err)
		} else {
			app.Spotify = spotifyClient
			musicSpotify = spotifyClient
			app.Logger.Info("spotify client initialized successfully")
		}
	}

	app.initRuntimeCaches()
	app.ScanContext, app.ScanCancel = context.WithCancel(context.Background())
	// Apple lookups take no credentials, so the client always exists; the
	// enable_apple_lookups setting gates its use at each music scan.
	app.initScanners(musicSpotify, itunes.New())

	app.InitRouter()

	return &app, nil
}

// initScanners wires the three library scanners from what InitApp has set up
// by this point: logger, prepared queries, settings, ffprobe, the TMDB,
// Spotify and iTunes clients, the scan context and the runtime caches. The
// scanners capture those dependencies at construction, so swapping one
// afterwards means calling this again.
func (app *Application) initScanners(musicSpotify music.SpotifyProvider, musicApple music.AppleProvider) {
	app.MusicScanner = music.New(music.Dependencies{
		DB:      app.DB,
		Queries: app.Queries,
		Logger:  app.Logger,
		Ffprobe: app.Ffprobe,
		Spotify: musicSpotify,
		Apple:   musicApple,
		AppleLookupsEnabled: func() bool {
			return app.CurrentSettings().EnableAppleLookups
		},
		ScanContext: app.ScanContext,
		Wait:        app.Wait,
		ScannerDBMu: &app.ScannerDBMu,
		CurrentMusicDirectory: func() sql.NullString {
			return app.CurrentSettings().MusicDir
		},
		InvalidateCommittedTrack: func(trackID int64) {
			app.StreamFileCache.invalidate(trackStreamFileKey(trackID))
		},
	})

	app.ShowScanner = show.New(show.Dependencies{
		DB:          app.DB,
		Queries:     app.Queries,
		Logger:      app.Logger,
		Ffprobe:     app.Ffprobe,
		Tmdb:        app.Tmdb,
		ScanContext: app.ScanContext,
		Wait:        app.Wait,
		ScannerDBMu: &app.ScannerDBMu,
		CurrentShowsDirectory: func() sql.NullString {
			return app.CurrentSettings().ShowsDir
		},
		InvalidateCommittedShowFile: app.invalidateCommittedShowFile,
	})

	app.MovieScanner = movie.New(movie.Dependencies{
		DB:          app.DB,
		Queries:     app.Queries,
		Logger:      app.Logger,
		Ffprobe:     app.Ffprobe,
		Tmdb:        app.Tmdb,
		ScanContext: app.ScanContext,
		Wait:        app.Wait,
		ScannerDBMu: &app.ScannerDBMu,
		CurrentMoviesDirectory: func() sql.NullString {
			return app.CurrentSettings().MoviesDir
		},
		InvalidateCommittedMovie:    app.invalidateCommittedMovie,
		InvalidateDeletedWatchRooms: app.invalidateDeletedWatchRooms,
	})

	// The watcher reads the scanner fields at each run, so a test that swaps
	// a scanner after this still feeds the new one.
	app.Watcher = watch.New(watch.Dependencies{
		Context: app.ScanContext,
		Wait:    app.Wait,
		Logger:  app.Logger,
		Libraries: []watch.Library{
			{
				Name:         movieLibrary.directory,
				Directory:    func() sql.NullString { return app.CurrentSettings().MoviesDir },
				Extensions:   helpers.ValidVideoExtensions,
				StartChanges: func(changes scanner.Changes) scanner.StartResult { return app.MovieScanner.StartChanges(changes) },
			},
			{
				Name:           showLibrary.directory,
				Directory:      func() sql.NullString { return app.CurrentSettings().ShowsDir },
				Extensions:     helpers.ValidVideoExtensions,
				AllowDirectory: show.AllowDirectory,
				StartChanges:   func(changes scanner.Changes) scanner.StartResult { return app.ShowScanner.StartChanges(changes) },
			},
			{
				Name:         musicLibrary.directory,
				Directory:    func() sql.NullString { return app.CurrentSettings().MusicDir },
				Extensions:   helpers.ValidAudioExtensions,
				StartChanges: func(changes scanner.Changes) scanner.StartResult { return app.MusicScanner.StartChanges(changes) },
			},
		},
	})
}

// invalidateDeletedWatchRooms runs after scanner deletion commits, before movie
// cache eviction, so even rooms without a cached session reject late HLS fills.
func (app *Application) invalidateDeletedWatchRooms(roomIDs []int64) {
	for _, roomID := range roomIDs {
		app.WatchRoomAuthCache.invalidateRoom(roomID)
	}
	app.RoomHLSMu.Lock()
	for _, roomID := range roomIDs {
		app.markRoomHLSSessionDeleted(roomID)
	}
	app.RoomHLSMu.Unlock()
	for _, roomID := range roomIDs {
		app.WatchRoomHub.deleteRoom(roomID)
	}
}

// endDeletedOwnerRooms runs after a user deletion commits. The cascade removed
// the rooms the user owned, so their members get room_deleted, and each room's
// HLS session is torn down here: unlike a deleted movie's, nothing else would
// stop it before its 30-minute idle expiry.
func (app *Application) endDeletedOwnerRooms(roomIDs []int64) {
	app.invalidateDeletedWatchRooms(roomIDs)
	for _, roomID := range roomIDs {
		app.CleanupRoomHLSSession(roomID)
	}
}

// invalidateCommittedMovie drops everything derived from one movie's rows after
// the scanner commits a rescan or deletion. It is a method rather than a closure so
// the test harness can wire the same list; a cache missing from here serves
// pre-rescan data until its TTL expires.
func (app *Application) invalidateCommittedMovie(movieID int64) {
	app.invalidateSubtitleVTTCache(mediaKindMovie, movieID)
	app.StreamFileCache.invalidate(movieStreamFileKey(movieID))
	app.MovieStreamsCache.invalidate(movieStreamsKey(movieID))
	app.invalidateHLSSessionsForFile(mediaKindMovie, movieID)
}

// invalidateCommittedShowFile is the TV twin of invalidateCommittedMovie. The
// subtitle cache and HLS sessions are keyed on the physical file, so one call
// covers every episode a combined file backs; the direct-stream cache is keyed
// per episode, so the scanner passes the ids it linked (or, on deletion, the
// ids it read before the cascade removed the links). There is no
// MovieStreamsCache twin: that cache only serves watch-room track pins, and
// rooms do not play episodes.
func (app *Application) invalidateCommittedShowFile(fileID int64, episodeIDs []int64) {
	app.invalidateSubtitleVTTCache(mediaKindEpisode, fileID)
	for _, episodeID := range episodeIDs {
		app.StreamFileCache.invalidate(episodeStreamFileKey(episodeID))
	}
	app.invalidateHLSSessionsForFile(mediaKindEpisode, fileID)
}

func (app *Application) initRuntimeCaches() {
	app.HLSCPUTranscodeLimiter = newHLSTranscodeLimiter(hlsTranscodePoolCPU, configuredHLSMaxCPUTranscodes())
	app.HLSHWTranscodeLimiter = newHLSTranscodeLimiter(hlsTranscodePoolHardware, configuredHLSMaxHWTranscodes())
	app.HLSMaxPersonalSessionsPerUser = configuredHLSMaxPersonalSessionsPerUser()
	app.PersonalHLSReservations = make(map[int64]int)
	app.HLSShutdown, app.beginHLSShutdown = context.WithCancel(context.Background())

	// Eviction tears an aged-out session down off the sweep goroutine, tracked in
	// app.Wait so shutdown drains in-flight FFmpeg kills; CleanupOnce dedupes it
	// with explicit removals. The default TTL is unused: every Set picks its own.
	hlsCache := cache.New(hlsRoomSessionTTL, hlsSessionCacheSweep)
	hlsCache.OnEvicted(func(_ string, val interface{}) {
		session, ok := val.(*HLSSession)
		if ok {
			app.Wait.Go(func() {
				cleanupHLSSession(session)
			})
		}
	})
	app.HLSSessionCache = hlsCache

	// Cache extracted WebVTT payloads to avoid repeated subtitle conversion work.
	app.SubtitleVTTCache = cache.New(subtitleCacheTTL, subtitleCacheCleanup)
	// Reads app.Tmdb as initialized above and yields an empty pool while it is nil.
	app.PrerollTheaters = preroll.NewTheatersPool(app.Tmdb, app.Logger)
	app.RoomHLSTombstone = cache.New(hlsRoomSessionTTL, hlsSessionCacheSweep)

	app.QuickConnect = NewQuickConnectBroker()
	app.AuthLimiter = newRateLimiter()

	// Throttles devices.last_used_at writes to at most one per device per TTL.
	app.DeviceLastSeen = cache.New(deviceLastSeenTTL, deviceLastSeenTTL)

	// Keeps bearer-token resolution off SQLite on the media hot path.
	app.DeviceAuthCache = cache.New(deviceAuthCacheTTL, deviceAuthCacheTTL)

	// Keeps the per-request session version check off SQLite, like the
	// session reads themselves.
	app.SessionVersionCache = cache.New(sessionVersionCacheTTL, sessionVersionCacheSweep)

	app.WatchRoomAuthCache = newWatchRoomAuthCache()

	// Keeps the per-range-request file lookup off SQLite.
	app.StreamFileCache = newGenerationCache[streamFile](streamFileCacheTTL, streamFileCacheSweep)

	// Keeps the watch-room stream-pin checks off SQLite on the same hot path.
	app.MovieStreamsCache = newGenerationCache[movieStreams](movieStreamsCacheTTL, movieStreamsCacheSweep)
}
