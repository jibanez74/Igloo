package main

import (
	"context"
	"database/sql"
	"embed"
	"fmt"
	"net/http"
	"sync"

	"igloo/cmd/internal/database"
	"igloo/cmd/internal/ffmpeg"
	"igloo/cmd/internal/ffprobe"
	applogger "igloo/cmd/internal/logger"
	"igloo/cmd/internal/spotify"
	"igloo/cmd/internal/tmdb"

	"github.com/alexedwards/scs/v2"
	"github.com/go-chi/chi/v5"
	"github.com/patrickmn/go-cache"
	"golang.org/x/sync/singleflight"
)

type Application struct {
	DB                      *sql.DB
	Queries                 *database.Queries
	Settings                *database.Setting
	Config                  RuntimeConfig
	Logger                  applogger.LoggerInterface
	LoggerCloser            func() error
	Ffprobe                 ffprobe.FfprobeInterface
	FFmpeg                  ffmpeg.FFmpegInterface
	Spotify                 spotify.SpotifyInterface
	Tmdb                    tmdb.TmdbInterface
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
	HLSTranscodeLimiter     *hlsTranscodeLimiter
	PersonalHLSMu           sync.Mutex
	PersonalHLSReservations map[int64]int

	// HLSMaxPersonalSessionsPerUser caps concurrent personal HLS sessions per
	// user; zero falls back to hlsMaxPersonalSessionsPerUserDefault.
	HLSMaxPersonalSessionsPerUser int
	RemuxSafetyCache              *cache.Cache
	SubtitleVTTCache              *cache.Cache
	SubtitleExtractGroup          singleflight.Group
	RoomHLSTombstone              *cache.Cache
	RoomHLSMu                     sync.Mutex
	WatchRoomHub                  *WatchRoomHub
	QuickConnect                  *QuickConnectBroker
	AuthLimiter                   *rateLimiter
	DeviceLastSeen                *cache.Cache
	DeviceAuthCache               *cache.Cache
	StreamFileCache               *cache.Cache
	DeviceExpiryCancel            context.CancelFunc
	ScanCancel                    context.CancelFunc
	ScanContext                   context.Context
}

//go:embed all:webdist
var FrontendFS embed.FS

func InitApp() (initializedApp *Application, err error) {
	config, err := NewRuntimeConfig()
	if err != nil {
		return nil, fmt.Errorf("failed to load runtime config: %v", err)
	}

	app := Application{
		Config: config,
		Wait:   &sync.WaitGroup{},
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
	cleanupStaleHLSTempDirs(app.Logger, app.Settings.TranscodeDir)

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

	if app.Settings.TmdbKey.Valid {
		tmdb, err := tmdb.New(app.Settings.TmdbKey.String)
		if err != nil {
			app.Logger.Warn("failed to initialize tmdb client", "error", err)
		} else {
			app.Tmdb = tmdb
			app.Logger.Info("tmdb client initialized successfully")
		}
	}

	if app.Settings.SpotifyClientID.Valid && app.Settings.SpotifyClientID.String != "" &&
		app.Settings.SpotifyClientSecret.Valid && app.Settings.SpotifyClientSecret.String != "" {
		spotifyClient, err := spotify.New(
			ctx,
			app.Settings.SpotifyClientID.String,
			app.Settings.SpotifyClientSecret.String,
		)

		if err != nil {
			app.Logger.Warn("failed to initialize spotify client", "error", err)
		} else {
			app.Spotify = spotifyClient
			app.Logger.Info("spotify client initialized successfully")
		}
	}

	app.initRuntimeCaches()
	app.ScanContext, app.ScanCancel = context.WithCancel(context.Background())
	app.ScanMoviesLibrary()
	app.ScanMusicLibrary()

	app.InitRouter()

	return &app, nil
}

func (app *Application) initRuntimeCaches() {
	app.HLSTranscodeLimiter = newHLSTranscodeLimiter(configuredHLSMaxCPUTranscodes())
	app.HLSMaxPersonalSessionsPerUser = configuredHLSMaxPersonalSessionsPerUser()
	app.PersonalHLSReservations = make(map[int64]int)

	// Eviction callback removes generated files when an HLS session ages out.
	// It must not make an expiration sweep wait for FFmpeg teardown. Explicit
	// removals also clean up synchronously after releasing their owner lock;
	// CleanupOnce safely deduplicates the two paths. Teardown goroutines are
	// tracked in app.Wait so shutdown drains in-flight FFmpeg kills instead of
	// exiting mid-teardown and orphaning the process.
	// The default TTL only applies to SetDefault, which this cache never uses;
	// personal and room sessions pick their TTL explicitly on every Set.
	hlsCache := cache.New(hlsRoomSessionTTL, hlsSessionCacheSweep)
	hlsCache.OnEvicted(func(_ string, val interface{}) {
		session, ok := val.(*HLSSession)
		if ok {
			app.Wait.Add(1)
			go func() {
				defer app.Wait.Done()
				cleanupHLSSession(session)
			}()
		}
	})
	app.HLSSessionCache = hlsCache
	app.RemuxSafetyCache = cache.New(
		hlsRemuxSafetyCacheTTL,
		hlsRemuxSafetyCacheSweep,
	)

	// Cache extracted WebVTT payloads to avoid repeated subtitle conversion work.
	app.SubtitleVTTCache = cache.New(subtitleCacheTTL, subtitleCacheCleanup)
	app.RoomHLSTombstone = cache.New(hlsRoomSessionTTL, hlsSessionCacheSweep)

	app.QuickConnect = NewQuickConnectBroker()
	app.AuthLimiter = newRateLimiter()

	// Throttles devices.last_used_at writes to at most one per device per TTL.
	app.DeviceLastSeen = cache.New(deviceLastSeenTTL, deviceLastSeenTTL)

	// Keeps bearer-token resolution off SQLite on the media hot path.
	app.DeviceAuthCache = cache.New(deviceAuthCacheTTL, deviceAuthCacheTTL)

	// Keeps the per-range-request file lookup off SQLite.
	app.StreamFileCache = cache.New(streamFileCacheTTL, streamFileCacheSweep)
}
