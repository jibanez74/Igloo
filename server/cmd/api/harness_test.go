package main

import (
	"context"
	"database/sql"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"igloo/cmd/internal/database"
	"igloo/cmd/internal/helpers"
	applogger "igloo/cmd/internal/logger"
	"igloo/cmd/internal/scanner"
	"igloo/cmd/internal/scanner/movie"
	"igloo/cmd/internal/scanner/music"
	"igloo/cmd/internal/scanner/show"

	cache "github.com/patrickmn/go-cache"
)

func setupTestLogger(t *testing.T, app *Application) {
	t.Helper()

	logger, _, err := applogger.New(&applogger.LoggerConfig{
		Debug:  true,
		Stdout: true,
	})
	if err != nil {
		t.Fatalf("Failed to create test logger: %v", err)
	}

	app.Logger = logger
}

func changeWorkingDirectory(t *testing.T, dir string) {
	t.Helper()

	oldWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("get working directory: %v", err)
	}
	err = os.Chdir(dir)
	if err != nil {
		t.Fatalf("change working directory: %v", err)
	}
	t.Cleanup(func() {
		restoreErr := os.Chdir(oldWD)
		if restoreErr != nil {
			t.Errorf("restore working directory: %v", restoreErr)
		}
	})
}

func setupTestApp(t *testing.T) *Application {
	t.Helper()

	db, err := sql.Open("sqlite3", ":memory:?_foreign_keys=on")
	if err != nil {
		t.Fatalf("Failed to open in-memory database: %v", err)
	}

	// Every additional pooled connection to ":memory:" gets its own empty database,
	// so a test that runs two queries concurrently could hit one with no schema.
	// Production pins the pool the same way in InitDB.
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)
	t.Cleanup(func() {
		err := db.Close()
		if err != nil {
			t.Errorf("close test database: %v", err)
		}
	})

	dataDir := t.TempDir()
	app := &Application{
		DB:             db,
		FrontendAssets: FrontendFS,
		// initScanners hands this group to every scanner, and movie.New,
		// music.New and show.New silently substitute a private one when it is
		// nil. Leaving it unset gave every scanner its own group, so
		// app.Wait.Wait() fenced nothing and tests that use it to await a scan
		// raced the still-running scan goroutine.
		Wait: &sync.WaitGroup{},
		Config: RuntimeConfig{
			// Both are absolute: InitSettings persists them, and handlers that
			// write beneath StaticDir would otherwise create ./static in the
			// package directory.
			StaticDir:                  filepath.Join(dataDir, "static"),
			TranscodeDir:               filepath.Join(dataDir, "transcode"),
			Port:                       defaultAppPort,
			DefaultAdminName:           defaultAdminName,
			DefaultAdminEmail:          defaultAdminEmail,
			HardwareAccelerationDevice: helpers.HARDWARE_ACCELERATION_DEVICE_CPU,
		},
	}
	setupTestLogger(t, app)

	err = app.InitTables()
	if err != nil {
		t.Fatalf("InitTables failed: %v", err)
	}

	app.Queries, err = database.Prepare(context.Background(), db)
	if err != nil {
		t.Fatalf("Failed to prepare queries: %v", err)
	}

	// Production boots through InitSettings before anything serves, so app
	// settings are never nil. Tests share that invariant rather than exercising
	// a state the running server cannot reach.
	err = app.InitSettings(context.Background())
	if err != nil {
		t.Fatalf("InitSettings failed: %v", err)
	}

	initTestRuntime(app)
	app.ScanContext, app.ScanCancel = context.WithCancel(context.Background())
	stopBackgroundWork(t, app)
	app.initScanners()

	return app
}

// stopBackgroundWork cancels scans at cleanup and waits for app.Wait, so no
// scan or teardown goroutine outlives the test into the database close, which
// was registered earlier and therefore runs later.
func stopBackgroundWork(t *testing.T, app *Application) {
	t.Helper()

	t.Cleanup(func() {
		app.ScanCancel()
		app.Wait.Wait()
	})
}

// initTestRuntime is the harness twin of the InitApp tail: the production
// runtime caches, then the HLS pieces tests need without real FFmpeg
// processes. The session cache gets no eviction hook because tests never
// attach FFmpeg processes to cache entries.
func initTestRuntime(app *Application) {
	app.initRuntimeCaches()
	app.WatchRoomHub = NewWatchRoomHub()
	app.HLSCPUTranscodeLimiter = newHLSTranscodeLimiter(hlsTranscodePoolCPU, 100)
	app.HLSHWTranscodeLimiter = newHLSTranscodeLimiter(hlsTranscodePoolHardware, 100)
	app.HLSMaxPersonalSessionsPerUser = hlsMaxPersonalSessionsPerUserDefault
	app.HLSSessionCache = cache.New(hlsRoomSessionTTL, hlsSessionCacheSweep)
}

// setupSessionTestApp is setupTestApp for handler tests that authenticate
// through a session cookie.
func setupSessionTestApp(t *testing.T) *Application {
	t.Helper()

	app := setupTestApp(t)
	app.InitSession()

	return app
}

// testUserPassword is the password every user createTestUser creates can log
// in with. bcrypt is slow by design, so the hash is computed once per package.
const testUserPassword = "correct horse"

var (
	testUserPasswordHashOnce sync.Once
	testUserPasswordHash     string
	testUserPasswordHashErr  error
)

func createTestUser(t *testing.T, app *Application, name, email string, isAdmin bool) database.User {
	t.Helper()

	testUserPasswordHashOnce.Do(func() {
		testUserPasswordHash, testUserPasswordHashErr = helpers.HashPassword(testUserPassword)
	})
	if testUserPasswordHashErr != nil {
		t.Fatalf("hash test password: %v", testUserPasswordHashErr)
	}

	user, err := app.Queries.CreateUser(context.Background(), database.CreateUserParams{
		Name:     name,
		Email:    email,
		Password: testUserPasswordHash,
		IsAdmin:  isAdmin,
		Avatar:   sql.NullString{},
	})
	if err != nil {
		t.Fatalf("create user %q: %v", email, err)
	}

	stored, err := app.Queries.GetUser(context.Background(), user.ID)
	if err != nil {
		t.Fatal(err)
	}
	return stored
}

func newAuthSessionCookie(t *testing.T, app *Application, userID int64) *http.Cookie {
	t.Helper()

	ctx, err := app.SessionManager.Load(context.Background(), "")
	if err != nil {
		t.Fatalf("load test session: %v", err)
	}

	app.SessionManager.Put(ctx, cookieUserID, userID)
	token, _, err := app.SessionManager.Commit(ctx)
	if err != nil {
		t.Fatalf("commit test session: %v", err)
	}

	return &http.Cookie{
		Name:  app.SessionManager.Cookie.Name,
		Value: token,
	}
}

// authenticatedRouter serves requests through the production router as
// userID, so the middleware routes.go installs (session, device auth, IsAuth,
// RequireAdmin) runs exactly as on the server. userID 0 sends no cookie and
// exercises the unauthenticated path.
func authenticatedRouter(t *testing.T, app *Application, userID int64) http.Handler {
	t.Helper()

	if app.SessionManager == nil {
		app.InitSession()
	}
	if app.Router == nil {
		app.InitRouter()
	}

	var cookie *http.Cookie
	if userID != 0 {
		cookie = newAuthSessionCookie(t, app, userID)
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if cookie != nil {
			r.AddCookie(cookie)
		}
		app.Router.ServeHTTP(w, r)
	})
}

// restartTestApp simulates a server restart: a fresh Application with fresh
// in-memory caches over the same database, so only state persisted in the
// database survives the boundary. Statements are re-prepared exactly as
// application.go does at boot, so a query left out of database.Prepare shows
// up here rather than riding on the original app's prepared set.
func restartTestApp(t *testing.T, app *Application) *Application {
	t.Helper()

	queries, err := database.Prepare(t.Context(), app.DB)
	if err != nil {
		t.Fatalf("prepare queries for restarted app: %v", err)
	}

	restarted := &Application{
		DB:      app.DB,
		Queries: queries,
		Config:  app.Config,
		Wait:    &sync.WaitGroup{},
	}
	setupTestLogger(t, restarted)

	// A real restart loads the persisted settings row, exactly as InitApp
	// does; without it the restarted app would carry a nil settings cache that
	// production never has.
	err = restarted.InitSettings(t.Context())
	if err != nil {
		t.Fatalf("InitSettings for restarted app: %v", err)
	}

	initTestRuntime(restarted)
	restarted.ScanContext, restarted.ScanCancel = context.WithCancel(context.Background())
	stopBackgroundWork(t, restarted)
	restarted.initScanners()

	return restarted
}

// clearSettingsRow empties the settings table so InitSettings takes its
// create-defaults path. setupTestApp already booted settings the way production
// does, so a test that exercises the create path has to undo that first --
// without ever nil-ing the cache, which the running server never does.
func clearSettingsRow(t *testing.T, app *Application) {
	t.Helper()

	_, err := app.DB.Exec("DELETE FROM settings")
	if err != nil {
		t.Fatalf("clear settings row: %v", err)
	}
}

// createTestMovie stores a bare MKV movie row: no streams, no duration, no
// metadata. Tests that need a playable file use the HLS or stream fixtures.
func createTestMovie(t *testing.T, app *Application, title, filePath string) int64 {
	t.Helper()

	movieID, err := app.Queries.UpsertMovie(context.Background(), database.UpsertMovieParams{
		Title:     title,
		FilePath:  filePath,
		FileName:  filepath.Base(filePath),
		Size:      1,
		Container: "mkv",
		MimeType:  helpers.VideoMimeTypes["mkv"],
	})
	if err != nil {
		t.Fatalf("create movie %q: %v", title, err)
	}
	return movieID
}

// movieStartFunc, showStartFunc and musicStartFunc stand in for a library
// scanner whose Start result a test decides; Status reports an idle scanner.
type movieStartFunc func() scanner.StartResult

func (f movieStartFunc) Start() scanner.StartResult { return f() }

func (movieStartFunc) Status() movie.Status { return movie.Status{Progress: idleScanProgress()} }

type showStartFunc func() scanner.StartResult

func (f showStartFunc) Start() scanner.StartResult { return f() }

func (showStartFunc) Status() show.Status { return show.Status{Progress: idleScanProgress()} }

type musicStartFunc func() scanner.StartResult

func (f musicStartFunc) Start() scanner.StartResult { return f() }

func (musicStartFunc) Status() music.Status { return music.Status{Progress: idleScanProgress()} }

func idleScanProgress() scanner.Progress {
	return scanner.Progress{State: scanner.StateIdle, Phase: scanner.PhaseIdle, ActiveFiles: []string{}, Issues: []scanner.Issue{}}
}
