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
	"sync"
	"syscall"
	"testing"
	"time"

	spotifyapi "igloo/cmd/internal/spotify"
	tmdbapi "igloo/cmd/internal/tmdb"

	"github.com/gorilla/websocket"
	"github.com/zmb3/spotify/v2"
)

const (
	listenForShutdownHelperEnv           = "GO_WANT_LISTEN_FOR_SHUTDOWN_HELPER"
	listenForShutdownHLSProcessHelperEnv = "GO_WANT_LISTEN_FOR_SHUTDOWN_HLS_PROCESS_HELPER"
	listenForShutdownLoggerMarkerEnv     = "IGLOO_LISTEN_FOR_SHUTDOWN_LOGGER_MARKER"
	listenForShutdownSocketMarkerEnv     = "IGLOO_LISTEN_FOR_SHUTDOWN_SOCKET_MARKER"
	listenForShutdownHLSMarkerEnv        = "IGLOO_LISTEN_FOR_SHUTDOWN_HLS_MARKER"
	listenForShutdownCacheMarkerEnv      = "IGLOO_LISTEN_FOR_SHUTDOWN_CACHE_MARKER"
)

type shutdownTestSpotify struct {
	cleared bool
}

func (s *shutdownTestSpotify) SearchAndGetAlbumDetails(_ context.Context, _ spotifyapi.AlbumSearchInput) (*spotify.FullAlbum, error) {
	return nil, nil
}

func (s *shutdownTestSpotify) SearchArtistByName(_ context.Context, _ string) (*spotify.FullArtist, error) {
	return nil, nil
}

func (s *shutdownTestSpotify) ClearAllCaches() {
	s.cleared = true
}

var _ spotifyapi.SpotifyInterface = (*shutdownTestSpotify)(nil)

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
		"remux_cache_empty=true",
		"subtitle_cache_empty=true",
		"room_tombstone_empty=true",
		"spotify_cache_cleared=true",
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
	app.Wait = &sync.WaitGroup{}
	app.RemuxSafetyCache.SetDefault("shutdown-test-remux", struct{}{})
	app.SubtitleVTTCache.SetDefault("shutdown-test-subtitle", []byte("subtitle"))
	app.RoomHLSTombstone.SetDefault("room:shutdown-test", struct{}{})

	spotifyClient := &shutdownTestSpotify{}
	tmdbClient := &shutdownTestTmdb{}
	app.Spotify = spotifyClient
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

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	app.Server = &http.Server{
		Handler: http.NewServeMux(),
	}

	serverDone := make(chan error, 1)
	go func() {
		serverDone <- app.Server.Serve(listener)
	}()

	go app.ListenForShutdown()

	process, err := os.FindProcess(os.Getpid())
	if err != nil {
		t.Fatalf("find process: %v", err)
	}

	time.Sleep(150 * time.Millisecond)

	err = process.Signal(syscall.SIGTERM)
	if err != nil {
		t.Fatalf("signal process: %v", err)
	}

	time.Sleep(3 * time.Second)
	t.Fatal("ListenForShutdown did not exit helper process")
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

	client := &watchRoomClient{
		conn:   serverConn,
		roomID: 999,
		user: watchRoomMemberSummary{
			ID: 777,
		},
	}

	app.WatchRoomHub.mu.Lock()
	session := app.WatchRoomHub.getOrCreateSession(client.roomID)
	session.clients[client] = true
	session.connectedIDs[client.user.ID] = true
	app.WatchRoomHub.mu.Unlock()

	app.Wait.Add(1)
	go func() {
		defer app.Wait.Done()
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
	}()

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
	app.Wait.Add(1)
	go func() {
		defer app.Wait.Done()

		waitErr := cmd.Wait()

		session.ExitMu.Lock()
		session.Exited = true
		session.ExitErr = waitErr
		session.ExitMu.Unlock()
	}()

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
	spotifyCleared := false
	if spotifyClient, ok := app.Spotify.(*shutdownTestSpotify); ok {
		spotifyCleared = spotifyClient.cleared
	}

	tmdbCleared := false
	if tmdbClient, ok := app.Tmdb.(*shutdownTestTmdb); ok {
		tmdbCleared = tmdbClient.cleared
	}

	content := fmt.Sprintf(
		"hls_cache_empty=%t remux_cache_empty=%t subtitle_cache_empty=%t room_tombstone_empty=%t spotify_cache_cleared=%t tmdb_cache_cleared=%t\n",
		cacheIsEmpty(app.HLSSessionCache),
		cacheIsEmpty(app.RemuxSafetyCache),
		cacheIsEmpty(app.SubtitleVTTCache),
		cacheIsEmpty(app.RoomHLSTombstone),
		spotifyCleared,
		tmdbCleared,
	)

	return os.WriteFile(markerPath, []byte(content), 0o644)
}

func cacheIsEmpty(c interface{ ItemCount() int }) bool {
	if c == nil {
		return true
	}
	return c.ItemCount() == 0
}

func isExpectedShutdownProcessExit(err error) bool {
	if err == nil {
		return true
	}

	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		waitStatus, ok := exitErr.Sys().(syscall.WaitStatus)
		if ok && waitStatus.Signaled() {
			return true
		}
	}

	return strings.Contains(strings.ToLower(err.Error()), "killed")
}
