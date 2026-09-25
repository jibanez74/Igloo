package main

import (
	"context"
	"database/sql"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
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

// newTestDBApp returns an Application holding only a logger and a fresh
// in-memory database with the schema applied, for tests of the schema and of
// startup steps that run before anything else is initialised.
func newTestDBApp(t *testing.T) *Application {
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

	app := &Application{DB: db}
	setupTestLogger(t, app)

	err = app.InitTables()
	if err != nil {
		t.Fatalf("InitTables failed: %v", err)
	}

	return app
}

func setupTestApp(t *testing.T) *Application {
	t.Helper()

	dataDir := t.TempDir()
	return bootTestApp(t, newTestDBApp(t).DB, RuntimeConfig{
		// Both are absolute: InitSettings persists them, and handlers that
		// write beneath StaticDir would otherwise create ./static in the
		// package directory.
		StaticDir:                  filepath.Join(dataDir, "static"),
		TranscodeDir:               filepath.Join(dataDir, "transcode"),
		Port:                       defaultAppPort,
		DefaultAdminName:           defaultAdminName,
		DefaultAdminEmail:          defaultAdminEmail,
		HardwareAccelerationDevice: helpers.HARDWARE_ACCELERATION_DEVICE_CPU,
	})
}

// bootTestApp is the harness twin of InitApp over an existing database: it
// prepares statements exactly as application.go does at boot, loads settings,
// then wires the runtime caches and scanners.
func bootTestApp(t *testing.T, db *sql.DB, config RuntimeConfig) *Application {
	t.Helper()

	app := &Application{
		DB:             db,
		Config:         config,
		FrontendAssets: FrontendFS,
		// initScanners hands this group to every scanner, and movie.New,
		// music.New and show.New silently substitute a private one when it is
		// nil. Leaving it unset gave every scanner its own group, so
		// app.Wait.Wait() fenced nothing and tests that use it to await a scan
		// raced the still-running scan goroutine.
		Wait: &sync.WaitGroup{},
	}
	setupTestLogger(t, app)

	var err error
	app.Queries, err = database.Prepare(t.Context(), db)
	if err != nil {
		t.Fatalf("Failed to prepare queries: %v", err)
	}

	// Production boots through InitSettings before anything serves, so app
	// settings are never nil. Tests share that invariant rather than exercising
	// a state the running server cannot reach.
	err = app.InitSettings(t.Context())
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

// serveRequest serves one request through handler, sending body as JSON
// when it is not empty.
func serveRequest(t *testing.T, handler http.Handler, method, target, body string) *httptest.ResponseRecorder {
	t.Helper()

	req := httptest.NewRequest(method, target, strings.NewReader(body))
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	return w
}

// serveAs is serveRequest through the production router as userID.
func serveAs(t *testing.T, app *Application, userID int64, method, target, body string) *httptest.ResponseRecorder {
	t.Helper()

	return serveRequest(t, authenticatedRouter(t, app, userID), method, target, body)
}

// restartTestApp simulates a server restart: a fresh Application with fresh
// in-memory caches over the same database, so only state persisted in the
// database survives the boundary. Statements are re-prepared, so a query left
// out of database.Prepare shows up here rather than riding on the original
// app's prepared set.
func restartTestApp(t *testing.T, app *Application) *Application {
	t.Helper()

	return bootTestApp(t, app.DB, app.Config)
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

// insertTestAudioStream stores one audio stream, filling in an AAC stereo
// track's codec and channel count when params leaves them unset.
func insertTestAudioStream(t *testing.T, app *Application, params database.InsertAudioStreamParams) {
	t.Helper()

	if params.Codec == "" {
		params.Codec = "aac"
	}
	if params.Channels == 0 {
		params.Channels = 2
	}
	err := app.Queries.InsertAudioStream(context.Background(), params)
	if err != nil {
		t.Fatalf("insert audio stream %d: %v", params.StreamIndex, err)
	}
}

// insertTestSubtitle stores one subtitle stream; an empty language is NULL.
func insertTestSubtitle(t *testing.T, app *Application, movieID, streamIndex int64, codec, language string) {
	t.Helper()

	err := app.Queries.InsertSubtitle(context.Background(), database.InsertSubtitleParams{
		MovieID:     movieID,
		StreamIndex: streamIndex,
		Codec:       codec,
		Language:    sql.NullString{String: language, Valid: language != ""},
	})
	if err != nil {
		t.Fatalf("insert subtitle %d: %v", streamIndex, err)
	}
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

func createTestUserAndMovie(t *testing.T, app *Application) (userID, movieID int64) {
	t.Helper()
	ctx := context.Background()

	user := createTestUser(t, app, "Test User", "test@example.com", false)

	// MP4 on purpose: watch-room tests create direct-mode rooms with this
	// movie, and direct playback is refused for non-MP4 containers.
	movieID, err := app.Queries.UpsertMovie(ctx, database.UpsertMovieParams{
		Title:     "Test Movie",
		FilePath:  "/movies/test.mp4",
		FileName:  "test.mp4",
		Size:      1024,
		Container: "mp4",
		MimeType:  helpers.VideoMimeTypes["mp4"],
	})
	if err != nil {
		t.Fatalf("failed to create test movie: %v", err)
	}

	return user.ID, movieID
}

func createTestMusician(t *testing.T, app *Application, name string) int64 {
	t.Helper()

	musicianIdentity, err := app.Queries.UpsertMusician(context.Background(), database.UpsertMusicianParams{
		Name:     name,
		SortName: strings.ToLower(name),
	})
	if err != nil {
		t.Fatalf("create musician %q: %v", name, err)
	}
	return musicianIdentity.ID
}

func createTestAlbum(t *testing.T, app *Application, title, musician string) int64 {
	t.Helper()

	albumIdentity, err := app.Queries.UpsertAlbum(context.Background(), database.UpsertAlbumParams{
		Title:     title,
		SortTitle: strings.ToLower(title),
		Musician:  sql.NullString{String: musician, Valid: true},
	})
	if err != nil {
		t.Fatalf("create album %q: %v", title, err)
	}
	return albumIdentity.ID
}

func createTestTrack(t *testing.T, app *Application, title, filePath string, albumID, musicianID int64) int64 {
	t.Helper()

	track, err := app.Queries.UpsertTrack(context.Background(), database.UpsertTrackParams{
		Title:         title,
		SortTitle:     strings.ToLower(title),
		FilePath:      filePath,
		FileName:      strings.TrimPrefix(filePath, "/music/"),
		Container:     "flac",
		MimeType:      "audio/flac",
		Codec:         "flac",
		Size:          1,
		TrackIndex:    1,
		Duration:      180,
		Disc:          1,
		Channels:      "2",
		ChannelLayout: "stereo",
		BitRate:       1000,
		Profile:       "",
		AlbumID:       sql.NullInt64{Int64: albumID, Valid: true},
		MusicianID:    sql.NullInt64{Int64: musicianID, Valid: true},
	})
	if err != nil {
		t.Fatalf("create track %q: %v", title, err)
	}
	return track
}

// createTestDevice inserts a device row directly and returns its bearer token.
func createTestDevice(t *testing.T, app *Application, userID int64, name, platform string) string {
	t.Helper()

	token, tokenHash, err := generateDeviceToken()
	if err != nil {
		t.Fatalf("generate device token: %v", err)
	}

	_, err = app.Queries.CreateDevice(context.Background(), database.CreateDeviceParams{
		UserID:    userID,
		Name:      name,
		Platform:  platform,
		TokenHash: tokenHash,
	})
	if err != nil {
		t.Fatalf("create device: %v", err)
	}

	return token
}

func createMovieGenre(t *testing.T, app *Application, movieID int64, name string) int64 {
	t.Helper()

	genreID, err := app.Queries.GetOrCreateGenre(t.Context(), database.GetOrCreateGenreParams{
		Tag:       name,
		GenreType: "movie",
	})
	if err != nil {
		t.Fatalf("create genre %q: %v", name, err)
	}
	err = app.Queries.CreateMovieGenre(t.Context(), database.CreateMovieGenreParams{
		MovieID: movieID,
		GenreID: genreID,
	})
	if err != nil {
		t.Fatalf("link genre %q: %v", name, err)
	}
	return genreID
}

func createMoviePlaylist(t *testing.T, app *Application, userID, movieID int64) int64 {
	t.Helper()

	playlist, err := app.Queries.CreateMoviePlaylist(t.Context(), database.CreateMoviePlaylistParams{
		UserID:   userID,
		Name:     "Contract Playlist",
		IsPublic: false,
	})
	if err != nil {
		t.Fatalf("create movie playlist: %v", err)
	}
	_, err = app.DB.Exec(
		"INSERT INTO playlist_movies (playlist_id, movie_id, position, added_by) VALUES (?, ?, 1, ?)",
		playlist.ID, movieID, userID,
	)
	if err != nil {
		t.Fatalf("add movie to playlist: %v", err)
	}
	return playlist.ID
}

func seedWatchProgress(t *testing.T, app *Application, userID, movieID int64) {
	t.Helper()

	_, err := app.DB.Exec(
		"INSERT INTO movie_watch_progress (user_id, movie_id, progress_sec, duration_sec) VALUES (?, ?, ?, ?)",
		userID, movieID, 600, 5400,
	)
	if err != nil {
		t.Fatalf("seed watch progress: %v", err)
	}
}

// seedContractShow builds a show through the scanner's own upserts, so the
// fixture cannot drift from what a real scan produces. It returns the show id.
//
// Shape: two seasons plus specials. Season 1 is complete (2 of 2 episodes),
// season 2 is partial (1 episode present against a TMDB count of 8), and the
// specials season sorts last despite having the lowest number.
func seedContractShow(t *testing.T, app *Application) int64 {
	t.Helper()

	ctx := context.Background()
	q := app.Queries

	show, err := q.UpsertLocalShow(ctx, database.UpsertLocalShowParams{
		DirectoryPath: "/shows/Contract Detail Show (2024)",
		LocalName:     "Contract Detail Show",
		PremiereYear:  sql.NullInt64{Int64: 2024, Valid: true},
		Name:          "Contract Detail Show",
	})
	if err != nil {
		t.Fatalf("upsert show: %v", err)
	}

	err = q.UpdateShowMetadata(ctx, database.UpdateShowMetadataParams{
		Name:             "Contract Detail Show",
		TmdbID:           sql.NullInt64{Int64: 90210, Valid: true},
		OriginalName:     sql.NullString{String: "Contract Detail Show", Valid: true},
		Overview:         sql.NullString{String: "A show that exists to validate the details contract.", Valid: true},
		Tagline:          sql.NullString{String: "Every field, once.", Valid: true},
		Language:         sql.NullString{String: "en", Valid: true},
		OriginCountries:  sql.NullString{String: "US", Valid: true},
		FirstAirDate:     sql.NullString{String: "2024-03-01", Valid: true},
		LastAirDate:      sql.NullString{String: "2026-05-20", Valid: true},
		Status:           sql.NullString{String: "Returning Series", Valid: true},
		Type:             sql.NullString{String: "Scripted", Valid: true},
		PosterPath:       sql.NullString{String: "/contract-detail-poster.jpg", Valid: true},
		BackdropPath:     sql.NullString{String: "/contract-detail-backdrop.jpg", Valid: true},
		VoteAverage:      sql.NullFloat64{Float64: 8.4, Valid: true},
		VoteCount:        sql.NullInt64{Int64: 1200, Valid: true},
		Certification:    sql.NullString{String: "TV-14", Valid: true},
		TmdbSeasonCount:  sql.NullInt64{Int64: 2, Valid: true},
		TmdbEpisodeCount: sql.NullInt64{Int64: 10, Valid: true},
		ID:               show.ID,
	})
	if err != nil {
		t.Fatalf("update show metadata: %v", err)
	}

	// Specials are created first so ordering cannot pass by insertion accident.
	seasons := []struct {
		number       int64
		name         string
		tmdbEpisodes int64
		episodes     int64
	}{
		{0, "Specials", 1, 1},
		{1, "Season 1", 2, 2},
		{2, "Season 2", 8, 1},
	}

	for _, s := range seasons {
		season, err := q.UpsertLocalShowSeason(ctx, database.UpsertLocalShowSeasonParams{
			ShowID:       show.ID,
			SeasonNumber: s.number,
			Name:         s.name,
		})
		if err != nil {
			t.Fatalf("upsert season %d: %v", s.number, err)
		}

		err = q.UpdateShowSeasonMetadata(ctx, database.UpdateShowSeasonMetadataParams{
			Name:             s.name,
			TmdbID:           sql.NullInt64{Int64: 5000 + s.number, Valid: true},
			Overview:         sql.NullString{String: s.name + " overview.", Valid: true},
			AirDate:          sql.NullString{String: "2024-03-01", Valid: true},
			PosterPath:       sql.NullString{String: "/season.jpg", Valid: true},
			VoteAverage:      sql.NullFloat64{Float64: 7.9, Valid: true},
			TmdbEpisodeCount: sql.NullInt64{Int64: s.tmdbEpisodes, Valid: true},
			ID:               season.ID,
		})
		if err != nil {
			t.Fatalf("update season %d metadata: %v", s.number, err)
		}

		for n := int64(1); n <= s.episodes; n++ {
			episode, err := q.UpsertLocalShowEpisode(ctx, database.UpsertLocalShowEpisodeParams{
				SeasonID:      season.ID,
				EpisodeNumber: n,
				Name:          fmt.Sprintf("%s Episode %d", s.name, n),
			})
			if err != nil {
				t.Fatalf("upsert episode s%de%d: %v", s.number, n, err)
			}

			// The last episode of season 1 stays unenriched, so the contract is
			// checked against null overview, still, runtime, and votes too.
			if s.number == 1 && n == s.episodes {
				continue
			}

			err = q.UpdateShowEpisodeMetadata(ctx, database.UpdateShowEpisodeMetadataParams{
				Name:        fmt.Sprintf("%s Episode %d", s.name, n),
				TmdbID:      sql.NullInt64{Int64: 70000 + s.number*100 + n, Valid: true},
				Overview:    sql.NullString{String: "Episode overview.", Valid: true},
				AirDate:     sql.NullString{String: "2024-03-08", Valid: true},
				StillPath:   sql.NullString{String: "/still.jpg", Valid: true},
				TmdbRuntime: sql.NullInt64{Int64: 47, Valid: true},
				VoteAverage: sql.NullFloat64{Float64: 8.1, Valid: true},
				VoteCount:   sql.NullInt64{Int64: 220, Valid: true},
				ID:          episode.ID,
			})
			if err != nil {
				t.Fatalf("update episode s%de%d metadata: %v", s.number, n, err)
			}
		}
	}

	leadArtist, err := q.UpsertArtist(ctx, database.UpsertArtistParams{
		Name:    "Ada Contract",
		TmdbID:  4242,
		Profile: sql.NullString{String: "/ada.jpg", Valid: true},
	})
	if err != nil {
		t.Fatalf("upsert lead artist: %v", err)
	}

	// No profile: exercises the nullable artist_profile branch.
	guestArtist, err := q.UpsertArtist(ctx, database.UpsertArtistParams{
		Name:   "Bo Contract",
		TmdbID: 4243,
	})
	if err != nil {
		t.Fatalf("upsert guest artist: %v", err)
	}

	// Two credits for one artist: TMDB aggregate credits do this, and it is why
	// credit_id rather than artist_id is the row identity.
	castRows := []database.CreateShowCastParams{
		{ShowID: show.ID, ArtistID: leadArtist, Character: "Captain", CastOrder: 0, CreditID: "credit-lead-1", EpisodeCount: 10},
		{ShowID: show.ID, ArtistID: leadArtist, Character: "Captain's Double", CastOrder: 1, CreditID: "credit-lead-2", EpisodeCount: 2},
		{ShowID: show.ID, ArtistID: guestArtist, Character: "Navigator", CastOrder: 2, CreditID: "credit-guest-1", EpisodeCount: 4},
	}
	for _, row := range castRows {
		err = q.CreateShowCast(ctx, row)
		if err != nil {
			t.Fatalf("create cast %s: %v", row.CreditID, err)
		}
	}

	crewRows := []database.CreateShowCrewParams{
		{ShowID: show.ID, ArtistID: leadArtist, Department: "Directing", Job: "Director", CreditID: "crew-1", EpisodeCount: 6},
		{ShowID: show.ID, ArtistID: guestArtist, Department: "Writing", Job: "Writer", CreditID: "crew-2", EpisodeCount: 10},
	}
	for _, row := range crewRows {
		err = q.CreateShowCrew(ctx, row)
		if err != nil {
			t.Fatalf("create crew %s: %v", row.CreditID, err)
		}
	}

	err = q.CreateShowCreator(ctx, database.CreateShowCreatorParams{ShowID: show.ID, ArtistID: leadArtist})
	if err != nil {
		t.Fatalf("create creator: %v", err)
	}

	genreID, err := q.GetOrCreateGenre(ctx, database.GetOrCreateGenreParams{Tag: "Drama", GenreType: "show"})
	if err != nil {
		t.Fatalf("create genre: %v", err)
	}
	err = q.CreateShowGenre(ctx, database.CreateShowGenreParams{ShowID: show.ID, GenreID: genreID})
	if err != nil {
		t.Fatalf("link genre: %v", err)
	}

	network, err := q.UpsertNetwork(ctx, database.UpsertNetworkParams{
		TmdbID:  77,
		Name:    "Contract Network",
		Logo:    sql.NullString{String: "/network.png", Valid: true},
		Country: sql.NullString{String: "US", Valid: true},
	})
	if err != nil {
		t.Fatalf("upsert network: %v", err)
	}
	err = q.CreateShowNetwork(ctx, database.CreateShowNetworkParams{ShowID: show.ID, NetworkID: network.ID})
	if err != nil {
		t.Fatalf("link network: %v", err)
	}

	companyID, err := q.UpsertProductionCompany(ctx, database.UpsertProductionCompanyParams{
		Name:   "Contract Pictures",
		TmdbID: 88,
	})
	if err != nil {
		t.Fatalf("upsert production company: %v", err)
	}
	err = q.CreateShowProductionCompany(ctx, database.CreateShowProductionCompanyParams{
		ShowID:              show.ID,
		ProductionCompanyID: companyID,
	})
	if err != nil {
		t.Fatalf("link production company: %v", err)
	}

	videoID, err := q.UpsertExtraVideo(ctx, database.UpsertExtraVideoParams{
		Title:      "Contract Trailer",
		ExternalID: sql.NullString{String: "tmdb-video-1", Valid: true},
		Key:        "dQw4w9WgXcQ",
		Type:       "trailer",
		Site:       "youtube",
	})
	if err != nil {
		t.Fatalf("upsert extra video: %v", err)
	}
	err = q.CreateShowExtraVideo(ctx, database.CreateShowExtraVideoParams{ShowID: show.ID, ExtraVideoID: videoID})
	if err != nil {
		t.Fatalf("link extra video: %v", err)
	}

	return show.ID
}

func testIntPtr(v int) *int {
	return &v
}

func sanitizeTestPathComponent(value string) string {
	value = strings.ReplaceAll(value, "/", "_")
	value = strings.ReplaceAll(value, " ", "_")
	return value
}
