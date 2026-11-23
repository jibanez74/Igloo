package main

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"

	"igloo/cmd/internal/helpers"
	tmdbapi "igloo/cmd/internal/tmdb"

	"github.com/gorilla/websocket"
)

// exitOnCancel stands in for FFmpeg exiting when its session is stopped,
// which is what releases a request waiting on the session.
func exitOnCancel(session *HLSSession) {
	session.Cancel = func() {
		session.ExitMu.Lock()
		session.Exited = true
		session.ExitErr = errors.New("signal: killed")
		session.ExitMu.Unlock()
	}
}

// Once shutdown has begun, a session that started would have its FFmpeg run
// past the process; the refusal is the retryable 503 a restart deserves.
func TestStopHLSSessions_RefusesNewSessionsWithRetryableError(t *testing.T) {
	app := setupTestApp(t)
	fake := &fakeFFmpeg{plans: []fakeFFmpegRunPlan{hlsRunPlan(transcodeFixture)}}
	app.FFmpeg = fake
	movieID := insertTestHLSMovieFixture(t, app, "h264", 1080)

	app.stopHLSSessions()

	_, _, err := app.GetOrCreateHLSSession(
		context.Background(), movieRef(movieID), helpers.HLS_PROFILE_720P_3MBPS,
		testIntPtr(0), nil, helpers.HLSVideoCodecs{}, testPlaybackSessionID, 0, 100,
	)
	if !errors.Is(err, errHLSShuttingDown) {
		t.Fatalf("GetOrCreateHLSSession err = %v, want errHLSShuttingDown", err)
	}
	if fake.CallCount() != 0 {
		t.Fatalf("RunHLS call count = %d, want 0 once shutdown began", fake.CallCount())
	}

	recorder := httptest.NewRecorder()
	writeHLSSessionError(recorder, err)
	if recorder.Code != http.StatusServiceUnavailable || recorder.Header().Get("Retry-After") == "" {
		t.Fatalf("shutdown refusal = %d (Retry-After %q), want a 503 with Retry-After",
			recorder.Code, recorder.Header().Get("Retry-After"))
	}
}

// A segment request long-polls for up to hlsSegmentWait. Shutdown used to stop
// sessions only after the HTTP server's graceful shutdown, which waited on
// exactly these requests for its whole 30 s budget.
func TestStopHLSSessions_AnswersAWaitingSegmentAsALostSession(t *testing.T) {
	app := setupTestApp(t)
	session := &HLSSession{Media: movieRef(1), TempDir: t.TempDir(), TempFileSegments: true}
	exitOnCancel(session)
	app.HLSSessionCache.Set("shutdown-test", session, hlsPersonalSessionTTL)

	answered := make(chan *httptest.ResponseRecorder, 1)
	go func() {
		recorder := httptest.NewRecorder()
		serveReadyHLSSegment(recorder, httptest.NewRequest(http.MethodGet, "/segment", nil), session, hlsSegmentFilename(3))
		answered <- recorder
	}()
	time.Sleep(50 * time.Millisecond)

	app.stopHLSSessions()

	select {
	case recorder := <-answered:
		if recorder.Code != http.StatusNotFound || recorder.Header().Get(hlsSegmentStatusHeader) != "" {
			t.Fatalf("waiting segment = %d (%s %q), want the plain 404 of a lost session",
				recorder.Code, hlsSegmentStatusHeader, recorder.Header().Get(hlsSegmentStatusHeader))
		}
	case <-time.After(2 * time.Second):
		t.Fatal("the waiting segment request was not answered when shutdown stopped its session")
	}
	_, statErr := os.Stat(session.TempDir)
	if !os.IsNotExist(statErr) {
		t.Fatalf("session temp dir survived shutdown: %v", statErr)
	}
	if app.HLSSessionCache.ItemCount() != 0 {
		t.Fatalf("HLS cache holds %d sessions after shutdown", app.HLSSessionCache.ItemCount())
	}
}

// A creation already past its checks when shutdown empties the cache must not
// publish afterwards: nothing would ever stop it.
func TestStopHLSSessions_RefusesACreationThatFinishesAfterIt(t *testing.T) {
	app := setupTestApp(t)
	started := make(chan struct{}, 1)
	proceed := make(chan struct{})
	plan := hlsRunPlan(transcodeFixture)
	plan.Started = started
	plan.Continue = proceed
	fake := &fakeFFmpeg{plans: []fakeFFmpegRunPlan{plan}}
	app.FFmpeg = fake
	movieID := insertTestHLSMovieFixture(t, app, "h264", 1080)

	type createResult struct {
		session *HLSSession
		err     error
	}
	resultCh := make(chan createResult, 1)
	go func() {
		session, _, err := app.GetOrCreateHLSSession(
			context.Background(), movieRef(movieID), helpers.HLS_PROFILE_720P_3MBPS,
			testIntPtr(0), nil, helpers.HLSVideoCodecs{}, testPlaybackSessionID, 0, 100,
		)
		resultCh <- createResult{session: session, err: err}
	}()

	select {
	case <-started:
	case <-time.After(2 * time.Second):
		t.Fatal("the creation never reached FFmpeg")
	}
	app.stopHLSSessions()
	close(proceed)

	var created createResult
	select {
	case created = <-resultCh:
	case <-time.After(2 * time.Second):
		t.Fatal("the creation never returned")
	}
	if !errors.Is(created.err, errHLSShuttingDown) || created.session != nil {
		t.Fatalf("creation finishing after shutdown = (%v, %v), want errHLSShuttingDown", created.session, created.err)
	}
	if app.HLSSessionCache.ItemCount() != 0 {
		t.Fatal("a session was published after shutdown emptied the cache")
	}
	if len(app.PersonalHLSReservations) != 0 {
		t.Fatalf("reservations left behind: %v", app.PersonalHLSReservations)
	}
	if len(app.PersonalHLSReplacing) != 0 {
		t.Fatalf("replaced windows left behind: %v", app.PersonalHLSReplacing)
	}
	_, statErr := os.Stat(fake.Calls()[0].OutDir)
	if !os.IsNotExist(statErr) {
		t.Fatalf("the refused session's temp dir survived: %v", statErr)
	}
}

// A start parked on a full pool waits up to hlsTranscodeAcquireWait, and the
// permit it waits for would go to a session the shutdown is stopping.
func TestStopHLSSessions_ReleasesAStartParkedOnAFullPool(t *testing.T) {
	app := setupTestApp(t)
	app.FFmpeg = &fakeFFmpeg{plans: []fakeFFmpegRunPlan{{}}}
	withTestHLSTranscodeAcquireWait(t, 30*time.Second)
	app.HLSCPUTranscodeLimiter = newHLSTranscodeLimiter(hlsTranscodePoolCPU, 1)
	holdHLSTranscodePermit(t, app, hlsTranscodePoolCPU)
	movieID := insertTestHLSMovieFixture(t, app, "h264", 1080)

	resultCh := make(chan error, 1)
	go func() {
		_, _, err := app.GetOrCreateHLSSession(
			context.Background(), movieRef(movieID), helpers.HLS_PROFILE_720P_3MBPS,
			testIntPtr(0), nil, helpers.HLSVideoCodecs{}, testPlaybackSessionID, 0, 100,
		)
		resultCh <- err
	}()
	select {
	case err := <-resultCh:
		t.Fatalf("the start resolved before shutdown: %v", err)
	case <-time.After(100 * time.Millisecond):
	}

	app.stopHLSSessions()

	select {
	case err := <-resultCh:
		if !errors.Is(err, errHLSShuttingDown) {
			t.Fatalf("parked start err = %v, want errHLSShuttingDown", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("the parked start kept waiting for a permit through shutdown")
	}
}

func TestStopHLSSessions_RefusesRoomPublication(t *testing.T) {
	app := setupTestApp(t)
	session := &HLSSession{IsRoom: true, TempDir: t.TempDir()}

	app.stopHLSSessions()

	err := app.storeRoomHLSSessionIfActive(7, RoomHLSSessionKey(7), session)
	if !errors.Is(err, errHLSShuttingDown) {
		t.Fatalf("storeRoomHLSSessionIfActive err = %v, want errHLSShuttingDown", err)
	}
	if !session.stopped() {
		t.Fatal("the refused room session was not stopped")
	}
	if app.HLSSessionCache.ItemCount() != 0 {
		t.Fatal("a room session was published after shutdown emptied the cache")
	}
}

// A session whose TTL ran out moments before shutdown is still in the cache
// until the janitor's next sweep, but Items no longer lists it and Flush drops
// it without OnEvicted, so its FFmpeg used to outlive the teardown.
func TestStopHLSSessions_StopsASessionThatExpiredBeforeTheSweep(t *testing.T) {
	app := setupTestApp(t)
	// The harness cache has no eviction hook; this needs production's.
	app.initRuntimeCaches()
	session := &HLSSession{Media: movieRef(1), TempDir: t.TempDir()}
	app.HLSSessionCache.Set("expired", session, time.Millisecond)
	time.Sleep(5 * time.Millisecond)

	app.stopHLSSessions()
	app.Wait.Wait()

	if !session.stopped() {
		t.Fatal("a session that expired before the sweep survived shutdown")
	}
	_, statErr := os.Stat(session.TempDir)
	if !os.IsNotExist(statErr) {
		t.Fatalf("the expired session's temp dir survived shutdown: %v", statErr)
	}
}

const (
	listenForShutdownHelperEnv           = "GO_WANT_LISTEN_FOR_SHUTDOWN_HELPER"
	listenForShutdownHLSProcessHelperEnv = "GO_WANT_LISTEN_FOR_SHUTDOWN_HLS_PROCESS_HELPER"
	listenForShutdownLoggerMarkerEnv     = "IGLOO_LISTEN_FOR_SHUTDOWN_LOGGER_MARKER"
	listenForShutdownSocketMarkerEnv     = "IGLOO_LISTEN_FOR_SHUTDOWN_SOCKET_MARKER"
	listenForShutdownHLSMarkerEnv        = "IGLOO_LISTEN_FOR_SHUTDOWN_HLS_MARKER"
	listenForShutdownCacheMarkerEnv      = "IGLOO_LISTEN_FOR_SHUTDOWN_CACHE_MARKER"
)

type shutdownTestTmdb struct {
	cleared bool
}

func (t *shutdownTestTmdb) GetTmdbMovieByID(_ context.Context, _ *tmdbapi.TmdbMovie) error {
	return nil
}

func (t *shutdownTestTmdb) SearchMoviesByTitleAndYear(_ context.Context, _ string, _ ...int) ([]tmdbapi.TmdbMovie, error) {
	return nil, nil
}

func (t *shutdownTestTmdb) GetMoviesInTheaters(_ context.Context) ([]*tmdbapi.TmdbMovie, error) {
	return nil, nil
}

func (t *shutdownTestTmdb) ClearCache() {
	t.cleared = true
}

var _ tmdbapi.TmdbInterface = (*shutdownTestTmdb)(nil)

func TestListenForShutdown_CleansUpCachesWebSocketsLoggerAndHLSSessions(t *testing.T) {
	if os.Getenv(listenForShutdownHelperEnv) == "1" {
		runListenForShutdownHelper(t)
		return
	}

	tempDir := t.TempDir()
	loggerMarker := filepath.Join(tempDir, "logger-closed.txt")
	socketMarker := filepath.Join(tempDir, "socket-closed.txt")
	hlsMarker := filepath.Join(tempDir, "hls-cleanup.txt")
	cacheMarker := filepath.Join(tempDir, "cache-cleanup.txt")

	cmd := exec.Command(os.Args[0], "-test.run=^TestListenForShutdown_CleansUpCachesWebSocketsLoggerAndHLSSessions$")
	cmd.Env = append(
		os.Environ(),
		listenForShutdownHelperEnv+"=1",
		listenForShutdownLoggerMarkerEnv+"="+loggerMarker,
		listenForShutdownSocketMarkerEnv+"="+socketMarker,
		listenForShutdownHLSMarkerEnv+"="+hlsMarker,
		listenForShutdownCacheMarkerEnv+"="+cacheMarker,
	)

	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("helper process failed: %v\noutput:\n%s", err, string(output))
	}

	loggerData, err := os.ReadFile(loggerMarker)
	if err != nil {
		t.Fatalf("read logger marker: %v", err)
	}
	if strings.TrimSpace(string(loggerData)) != "closed" {
		t.Fatalf("expected logger marker to be %q, got %q", "closed", strings.TrimSpace(string(loggerData)))
	}

	socketData, err := os.ReadFile(socketMarker)
	if err != nil {
		t.Fatalf("read socket marker: %v", err)
	}

	socketResult := strings.TrimSpace(string(socketData))
	if socketResult == "" {
		t.Fatal("expected socket marker to contain a close result")
	}
	if strings.Contains(strings.ToLower(socketResult), "timeout") {
		t.Fatalf("expected websocket to close during shutdown, got timeout marker %q", socketResult)
	}

	hlsData, err := os.ReadFile(hlsMarker)
	if err != nil {
		t.Fatalf("read HLS marker: %v", err)
	}

	hlsResult := strings.TrimSpace(string(hlsData))
	if !strings.Contains(hlsResult, "temp_dir_removed=true") {
		t.Fatalf("expected HLS cleanup marker to confirm temp dir removal, got %q", hlsResult)
	}
	if !strings.Contains(hlsResult, "process_exited=true") {
		t.Fatalf("expected HLS cleanup marker to confirm process exit, got %q", hlsResult)
	}

	cacheData, err := os.ReadFile(cacheMarker)
	if err != nil {
		t.Fatalf("read cache marker: %v", err)
	}

	cacheResult := strings.TrimSpace(string(cacheData))
	for _, expected := range []string{
		"hls_cache_empty=true",
		"subtitle_cache_empty=true",
		"room_tombstone_empty=true",
		"tmdb_cache_cleared=true",
	} {
		if !strings.Contains(cacheResult, expected) {
			t.Fatalf("expected cache marker to contain %q, got %q", expected, cacheResult)
		}
	}
}

func runListenForShutdownHelper(t *testing.T) {
	t.Helper()

	loggerMarker := os.Getenv(listenForShutdownLoggerMarkerEnv)
	socketMarker := os.Getenv(listenForShutdownSocketMarkerEnv)
	hlsMarker := os.Getenv(listenForShutdownHLSMarkerEnv)
	cacheMarker := os.Getenv(listenForShutdownCacheMarkerEnv)
	if loggerMarker == "" || socketMarker == "" || hlsMarker == "" || cacheMarker == "" {
		t.Fatal("missing shutdown helper marker paths")
	}

	app := setupTestApp(t)
	deviceExpiryCtx, cancelDeviceExpiry := context.WithCancel(context.Background())
	app.DeviceExpiryCancel = cancelDeviceExpiry
	app.Wait.Go(func() {
		app.runDeviceExpirySweeper(deviceExpiryCtx)
	})
	app.SubtitleVTTCache.SetDefault("shutdown-test-subtitle", []byte("subtitle"))
	app.RoomHLSTombstone.SetDefault("room:shutdown-test", struct{}{})

	tmdbClient := &shutdownTestTmdb{}
	app.Tmdb = tmdbClient

	session, err := attachShutdownTestHLSSession(t, app)
	if err != nil {
		t.Fatalf("attach shutdown HLS session: %v", err)
	}
	app.LoggerCloser = func() error {
		loggerErr := os.WriteFile(loggerMarker, []byte("closed\n"), 0o644)
		hlsErr := writeShutdownHLSMarker(hlsMarker, session)
		cacheErr := writeShutdownCacheMarker(cacheMarker, app)
		if loggerErr != nil {
			return loggerErr
		}
		if hlsErr != nil {
			return hlsErr
		}
		if cacheErr != nil {
			return cacheErr
		}
		return nil
	}

	err = attachShutdownTestWatchRoomClient(app, socketMarker)
	if err != nil {
		t.Fatalf("attach shutdown websocket client: %v", err)
	}

	// The signal must not land before the process has subscribed to it, or
	// the default action ends the helper. serveUntilShutdown subscribes before
	// ListenAndServe binds, so a listener that accepts connections is the
	// readiness signal.
	probe, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("reserve port: %v", err)
	}
	serverAddr := probe.Addr().String()
	err = probe.Close()
	if err != nil {
		t.Fatalf("release reserved port: %v", err)
	}
	app.Server = &http.Server{
		Addr:    serverAddr,
		Handler: http.NewServeMux(),
	}

	process, err := os.FindProcess(os.Getpid())
	if err != nil {
		t.Fatalf("find process: %v", err)
	}

	go func() {
		waitForListener(serverAddr, 5*time.Second)
		_ = process.Signal(syscall.SIGTERM)
	}()

	// The helper ends the way main does: by returning once serveUntilShutdown
	// does. The success path takes ~2s (graceful-wait timeout before the hung
	// HLS child is killed) plus ~1s of race-runtime latency under -race, so
	// the deadline needs generous headroom.
	served := make(chan error, 1)
	go func() {
		served <- app.serveUntilShutdown()
	}()

	select {
	case err = <-served:
		if err != nil {
			t.Fatalf("serveUntilShutdown: %v", err)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("serveUntilShutdown did not return after SIGTERM")
	}

	// Cleanup must have finished before the return, not merely started: main
	// used to return as soon as the listener closed, and the parent then read
	// markers a still-running goroutine never got to write.
	_, statErr := os.Stat(loggerMarker)
	if statErr != nil {
		t.Fatalf("serveUntilShutdown returned before cleanup finished: %v", statErr)
	}
}

// waitForListener returns once addr accepts a TCP connection, or after timeout.
func waitForListener(addr string, timeout time.Duration) {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		conn, err := net.DialTimeout("tcp", addr, 100*time.Millisecond)
		if err == nil {
			_ = conn.Close()
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func TestListenForShutdown_HLSProcessHelper(t *testing.T) {
	if os.Getenv(listenForShutdownHLSProcessHelperEnv) != "1" {
		t.Skip("helper process only")
	}

	time.Sleep(30 * time.Second)
}

func attachShutdownTestWatchRoomClient(app *Application, socketMarker string) error {
	upgrader := websocket.Upgrader{}
	serverConnCh := make(chan *websocket.Conn, 1)
	serverErrCh := make(chan error, 1)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			serverErrCh <- err
			return
		}
		serverConnCh <- conn
	}))

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http")
	clientConn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		server.Close()
		return err
	}

	var serverConn *websocket.Conn
	select {
	case serverConn = <-serverConnCh:
	case err = <-serverErrCh:
		server.Close()
		_ = clientConn.Close()
		return err
	case <-time.After(2 * time.Second):
		server.Close()
		_ = clientConn.Close()
		return syscall.ETIMEDOUT
	}

	client := newWatchRoomClient(serverConn, 999, userSummary{
		ID: 777,
	})
	go client.writePump()

	app.WatchRoomHub.mu.Lock()
	session := app.WatchRoomHub.getOrCreateSession(client.roomID)
	session.clients[client] = true
	session.connectedIDs[client.user.ID] = true
	app.WatchRoomHub.mu.Unlock()

	app.Wait.Go(func() {
		defer server.Close()
		defer clientConn.Close()

		err := clientConn.SetReadDeadline(time.Now().Add(5 * time.Second))
		if err != nil {
			_ = os.WriteFile(socketMarker, []byte("set-deadline-error: "+err.Error()+"\n"), 0o644)
			return
		}

		_, _, err = clientConn.ReadMessage()
		if err == nil {
			_ = os.WriteFile(socketMarker, []byte("unexpected-message\n"), 0o644)
			return
		}

		_ = os.WriteFile(socketMarker, []byte(err.Error()+"\n"), 0o644)
	})

	return nil
}

func attachShutdownTestHLSSession(t *testing.T, app *Application) (*HLSSession, error) {
	t.Helper()

	tempDir, err := os.MkdirTemp("", "listen-for-shutdown-hls-*")
	if err != nil {
		return nil, err
	}

	cmd := exec.Command(os.Args[0], "-test.run=^TestListenForShutdown_HLSProcessHelper$")
	cmd.Env = append(os.Environ(), listenForShutdownHLSProcessHelperEnv+"=1")

	err = cmd.Start()
	if err != nil {
		_ = os.RemoveAll(tempDir)
		return nil, err
	}

	session := &HLSSession{
		TempDir: tempDir,
		Cmd:     cmd,
	}

	app.HLSSessionCache.SetDefault("shutdown-test-hls", session)
	app.Wait.Go(func() {
		waitErr := cmd.Wait()

		session.ExitMu.Lock()
		session.Exited = true
		session.ExitErr = waitErr
		session.ExitMu.Unlock()
	})

	return session, nil
}

func writeShutdownHLSMarker(markerPath string, session *HLSSession) error {
	if session == nil {
		return os.WriteFile(markerPath, []byte("temp_dir_removed=false process_exited=false missing_session=true\n"), 0o644)
	}

	_, statErr := os.Stat(session.TempDir)
	tempDirRemoved := os.IsNotExist(statErr)

	session.ExitMu.Lock()
	processExited := session.Exited
	exitErr := session.ExitErr
	session.ExitMu.Unlock()

	if exitErr != nil && !isExpectedShutdownProcessExit(exitErr) {
		processExited = false
	}

	content := fmt.Sprintf(
		"temp_dir_removed=%t process_exited=%t\n",
		tempDirRemoved,
		processExited,
	)

	return os.WriteFile(markerPath, []byte(content), 0o644)
}

func writeShutdownCacheMarker(markerPath string, app *Application) error {
	tmdbCleared := false
	if tmdbClient, ok := app.Tmdb.(*shutdownTestTmdb); ok {
		tmdbCleared = tmdbClient.cleared
	}

	content := fmt.Sprintf(
		"hls_cache_empty=%t subtitle_cache_empty=%t room_tombstone_empty=%t tmdb_cache_cleared=%t\n",
		cacheIsEmpty(app.HLSSessionCache),
		cacheIsEmpty(app.SubtitleVTTCache),
		cacheIsEmpty(app.RoomHLSTombstone),
		tmdbCleared,
	)

	return os.WriteFile(markerPath, []byte(content), 0o644)
}

func cacheIsEmpty(c interface{ ItemCount() int }) bool {
	return c.ItemCount() == 0
}

func isExpectedShutdownProcessExit(err error) bool {
	if err == nil {
		return true
	}

	exitErr, isExitError := errors.AsType[*exec.ExitError](err)
	if isExitError {
		waitStatus, ok := exitErr.Sys().(syscall.WaitStatus)
		if ok && waitStatus.Signaled() {
			return true
		}
	}

	return strings.Contains(strings.ToLower(err.Error()), "killed")
}

func (*shutdownTestTmdb) SearchShowsByTitleAndYear(context.Context, string, ...int) ([]tmdbapi.TVShow, error) {
	return nil, tmdbapi.ErrNoShowsFound
}
func (*shutdownTestTmdb) GetShowDetails(context.Context, int) (*tmdbapi.TVShow, error) {
	return nil, tmdbapi.ErrNoShowsFound
}
func (*shutdownTestTmdb) GetSeasonDetails(context.Context, int, int) (*tmdbapi.TVSeason, error) {
	return nil, tmdbapi.ErrNoShowsFound
}

func (*shutdownTestTmdb) GetPersonDetails(_ context.Context, _ int) (*tmdbapi.Person, error) {
	return nil, nil
}

const listenForShutdownLongPollHelperEnv = "GO_WANT_LISTEN_FOR_SHUTDOWN_LONG_POLL_HELPER"

// A segment request long-polls for up to two minutes. Shutdown used to run
// the HTTP server's graceful shutdown first, which waited on that request for
// its whole 30 s budget before any HLS session was stopped, so every restart
// with a viewer mid-segment took half a minute.
func TestListenForShutdown_AnswersHLSLongPollsBeforeTheGracePeriod(t *testing.T) {
	if os.Getenv(listenForShutdownLongPollHelperEnv) == "1" {
		runListenForShutdownLongPollHelper(t)
		return
	}

	cmd := exec.Command(os.Args[0], "-test.run=^TestListenForShutdown_AnswersHLSLongPollsBeforeTheGracePeriod$", "-test.v")
	cmd.Env = append(os.Environ(), listenForShutdownLongPollHelperEnv+"=1")
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("helper process failed: %v\noutput:\n%s", err, string(output))
	}
}

func runListenForShutdownLongPollHelper(t *testing.T) {
	t.Helper()

	app := setupTestApp(t)
	app.DeviceExpiryCancel = func() {}

	session := &HLSSession{Media: movieRef(1), TempDir: t.TempDir(), TempFileSegments: true}
	exitOnCancel(session)
	app.HLSSessionCache.Set("long-poll", session, hlsPersonalSessionTTL)

	probe, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("reserve port: %v", err)
	}
	serverAddr := probe.Addr().String()
	err = probe.Close()
	if err != nil {
		t.Fatalf("release reserved port: %v", err)
	}
	inFlight := make(chan struct{})
	app.Server = &http.Server{
		Addr: serverAddr,
		Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			close(inFlight)
			serveReadyHLSSegment(w, r, session, hlsSegmentFilename(3))
		}),
	}

	served := make(chan error, 1)
	go func() {
		served <- app.serveUntilShutdown()
	}()
	waitForListener(serverAddr, 5*time.Second)

	status := make(chan int, 1)
	go func() {
		response, getErr := http.Get("http://" + serverAddr + "/segment")
		if getErr != nil {
			status <- -1
			return
		}
		_ = response.Body.Close()
		status <- response.StatusCode
	}()
	select {
	case <-inFlight:
	case <-time.After(5 * time.Second):
		t.Fatal("the segment request never reached the server")
	}

	process, err := os.FindProcess(os.Getpid())
	if err != nil {
		t.Fatalf("find process: %v", err)
	}
	startedShutdown := time.Now()
	err = process.Signal(syscall.SIGTERM)
	if err != nil {
		t.Fatalf("signal shutdown: %v", err)
	}

	select {
	case code := <-status:
		if code != http.StatusNotFound {
			t.Fatalf("long-polled segment answered %d, want the 404 clients rebase on", code)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("the long-polled segment was not answered during shutdown")
	}
	select {
	case err = <-served:
		if err != nil {
			t.Fatalf("serveUntilShutdown: %v", err)
		}
	case <-time.After(shutdownGracePeriod):
		t.Fatal("shutdown waited out the grace period on an HLS long-poll")
	}
	elapsed := time.Since(startedShutdown)
	if elapsed > 5*time.Second {
		t.Fatalf("shutdown took %s with an HLS long-poll in flight", elapsed)
	}
}
