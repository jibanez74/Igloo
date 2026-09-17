package main

import (
	"context"
	"database/sql"
	"errors"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"igloo/cmd/internal/database"
	"igloo/cmd/internal/ffprobe"
	"igloo/cmd/internal/helpers"
	"igloo/cmd/internal/scanner"
	"igloo/cmd/internal/scanner/movie"

	"github.com/gorilla/websocket"
)

func TestMovieScanDeletionTearsDownWatchRooms(t *testing.T) {
	app := setupTestApp(t)
	defer closeWatchRoomWSTestApp(t, app)
	ctx := context.Background()
	ownerID, unrelatedMovieID := createTestUserAndMovie(t, app)
	movieID := insertTestHLSMovieFixture(t, app, "h264", 720)
	root := t.TempDir()
	_, err := app.DB.Exec("UPDATE movies SET file_path = ? WHERE id = ?", filepath.Join(root, "missing.mp4"), movieID)
	if err != nil {
		t.Fatal(err)
	}
	app.SetSettings(&database.Setting{MoviesDir: sql.NullString{String: root, Valid: true}, TranscodeDir: t.TempDir()})
	rooms := []database.WatchRoom{
		createTestRoom(t, app, ownerID, movieID),
		createTestRoom(t, app, ownerID, movieID),
		createTestRoom(t, app, ownerID, unrelatedMovieID),
	}
	server := setupWatchRoomWSTestServer(t, app)
	defer server.Close()
	var sockets []*websocket.Conn
	for _, room := range rooms {
		addMembersToRoom(t, app, room.ID, ownerID)
		_, err = app.loadAuthorizedWatchRoom(ctx, room.ID, ownerID)
		if err != nil {
			t.Fatal(err)
		}
		conn, _ := dialWatchRoomSocket(t, app, server.URL, room.ID, ownerID)
		defer conn.Close()
		readUntilEventType(t, conn, "room_snapshot")
		sockets = append(sockets, conn)
	}

	// Hold a membership lookup after its database read until deletion commits.
	lookupStarted, releaseLookup := make(chan struct{}), make(chan struct{})
	lookupResult := make(chan error, 1)
	go func(release <-chan struct{}) {
		_, lookupErr := app.WatchRoomAuthCache.load(ctx, rooms[0].ID, ownerID+100, func(context.Context, int64, int64) (database.WatchRoom, error) {
			close(lookupStarted)
			<-release
			return rooms[0], nil
		})
		lookupResult <- lookupErr
	}(releaseLookup)
	<-lookupStarted
	defer func() {
		if releaseLookup != nil {
			close(releaseLookup)
		}
	}()

	// Start real session creation with the existing fake FFmpeg boundary.
	started, resume := make(chan struct{}, 1), make(chan struct{})
	app.FFmpeg = &fakeFFmpeg{plans: []fakeFFmpegRunPlan{{}, {}, {Started: started, Continue: resume}}}
	audioTrack := 0
	personal, personalKey, err := app.GetOrCreateHLSSession(ctx, movieRef(movieID), helpers.HLS_PROFILE_720P_3MBPS, &audioTrack, nil, testPlaybackSessionID, 0, ownerID)
	if err != nil {
		t.Fatal(err)
	}
	roomSession, err := app.GetOrCreateRoomHLSSession(ctx, rooms[0].ID, movieID, helpers.HLS_PROFILE_720P_3MBPS, 0, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	lateResult := make(chan error, 1)
	go func() {
		_, lateErr := app.GetOrCreateRoomHLSSession(ctx, rooms[1].ID, movieID, helpers.HLS_PROFILE_720P_3MBPS, 0, nil, nil)
		lateResult <- lateErr
	}()
	<-started
	defer func() {
		if resume != nil {
			close(resume)
		}
	}()
	cleanupStarted, releaseCleanup := blockHLSSessionCleanup(t, personal)
	defer releaseCleanup()
	result := app.MovieScanner.Start()
	if result.Status != scanner.StartStarted {
		t.Fatal(result)
	}
	waitForHLSSessionCleanupToBlock(t, cleanupStarted, releaseCleanup)
	// Scanner completion must not depend on FFmpeg teardown.
	app.ScannerDBMu.Lock()
	app.ScannerDBMu.Unlock()
	for i, room := range rooms[:2] {
		event := readUntilEventType(t, sockets[i], "room_deleted")
		if event.RoomID != room.ID {
			t.Fatal(event)
		}
		err = sockets[i].SetReadDeadline(time.Now().Add(2 * time.Second))
		if err != nil {
			t.Fatal(err)
		}
		_, _, err = sockets[i].ReadMessage()
		if !websocket.IsCloseError(err, websocket.CloseNormalClosure, websocket.CloseAbnormalClosure) {
			t.Fatalf("socket not closed normally: %v", err)
		}
		_, err = app.loadAuthorizedWatchRoom(ctx, room.ID, ownerID)
		if !errors.Is(err, sql.ErrNoRows) {
			t.Fatalf("authorization survived: %v", err)
		}
		if !app.isRoomHLSSessionDeleted(room.ID) {
			t.Fatal("missing tombstone")
		}
		var members int
		err = app.DB.QueryRow("SELECT count(*) FROM watch_room_members WHERE room_id = ?", room.ID).Scan(&members)
		if err != nil || members != 0 {
			t.Fatalf("members=%d err=%v", members, err)
		}
	}
	app.WatchRoomHub.mu.Lock()
	remaining := len(app.WatchRoomHub.sessions)
	app.WatchRoomHub.mu.Unlock()
	if remaining != 1 {
		t.Fatalf("hub rooms=%d", remaining)
	}
	_, err = app.Queries.GetMovieByID(ctx, movieID)
	if !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("movie survived: %v", err)
	}
	close(releaseLookup)
	releaseLookup = nil
	err = <-lookupResult
	if err != nil {
		t.Fatal(err)
	}
	_, err = app.loadAuthorizedWatchRoom(ctx, rooms[0].ID, ownerID+100)
	if !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("late authorization fill survived: %v", err)
	}
	close(resume)
	resume = nil
	err = <-lateResult
	if err == nil {
		t.Fatal("late HLS publication succeeded")
	}
	releaseCleanup()
	err = sockets[2].WriteJSON(map[string]any{"type": "ping"})
	if err != nil {
		t.Fatal(err)
	}
	readUntilEventType(t, sockets[2], "pong")
	sockets[2].Close()
	app.Wait.Wait()
	for _, key := range []string{personalKey, RoomHLSSessionKey(rooms[0].ID), RoomHLSSessionKey(rooms[1].ID)} {
		_, cached := app.HLSSessionCache.Get(key)
		if cached {
			t.Fatalf("session %s survived", key)
		}
	}
	for _, dir := range []string{personal.TempDir, roomSession.TempDir} {
		_, err = os.Stat(dir)
		if !os.IsNotExist(err) {
			t.Fatalf("session directory survived: %v", err)
		}
	}
	entries, err := os.ReadDir(app.CurrentSettings().TranscodeDir)
	if err != nil || len(entries) != 0 {
		t.Fatalf("late session resources survived: %v %v", entries, err)
	}
	result = app.MovieScanner.Start()
	if result.Status != scanner.StartStarted {
		t.Fatal(result)
	}
	app.Wait.Wait()
	if app.isRoomHLSSessionDeleted(rooms[2].ID) {
		t.Fatal("unrelated room tombstoned")
	}
	_, err = app.loadAuthorizedWatchRoom(ctx, rooms[2].ID, ownerID)
	if err != nil {
		t.Fatalf("unrelated room lost authorization: %v", err)
	}

}

type cleanupRescanProbe struct {
	ffprobe.FfprobeInterface
	calls int
}

func (p *cleanupRescanProbe) GetMetadata(context.Context, string) (*ffprobe.FfprobeResult, error) {
	p.calls++
	return &ffprobe.FfprobeResult{
		Format:  ffprobe.Format{Duration: "120"},
		Streams: []ffprobe.Stream{{Index: 0, CodecName: "h264", CodecType: "video", Width: 1280, Height: 720}},
	}, nil
}

func TestMovieRescanPreservesWatchRooms(t *testing.T) {
	app := setupTestApp(t)
	defer closeWatchRoomWSTestApp(t, app)
	ownerID, movieID := createTestUserAndMovie(t, app)
	root := t.TempDir()
	path := filepath.Join(root, "movie.mp4")
	err := os.WriteFile(path, []byte("original"), 0600)
	if err != nil {
		t.Fatal(err)
	}
	_, err = app.DB.Exec("UPDATE movies SET file_path = ? WHERE id = ?", path, movieID)
	if err != nil {
		t.Fatal(err)
	}
	room := createTestRoom(t, app, ownerID, movieID)
	addMembersToRoom(t, app, room.ID, ownerID)
	_, err = app.loadAuthorizedWatchRoom(context.Background(), room.ID, ownerID)
	if err != nil {
		t.Fatal(err)
	}
	server := setupWatchRoomWSTestServer(t, app)
	defer server.Close()
	conn, _ := dialWatchRoomSocket(t, app, server.URL, room.ID, ownerID)
	defer conn.Close()
	readUntilEventType(t, conn, "room_snapshot")
	var scanWait sync.WaitGroup
	probe := &cleanupRescanProbe{}
	movieScanner := movie.New(movie.Dependencies{
		DB: app.DB, Queries: app.Queries, Logger: app.Logger, Ffprobe: probe,
		Wait: &scanWait, ScannerDBMu: &app.ScannerDBMu,
		Now:                         func() time.Time { return time.Now().Add(2 * time.Minute) },
		CurrentMoviesDirectory:      func() sql.NullString { return sql.NullString{String: root, Valid: true} },
		InvalidateCommittedMovie:    app.invalidateCommittedMovie,
		InvalidateDeletedWatchRooms: app.invalidateDeletedWatchRooms,
	})
	for _, content := range []string{"original", "changed bytes"} {
		err = os.WriteFile(path, []byte(content), 0600)
		if err != nil {
			t.Fatal(err)
		}
		result := movieScanner.Start()
		if result.Status != scanner.StartStarted {
			t.Fatal(result)
		}
		scanWait.Wait()
		if app.isRoomHLSSessionDeleted(room.ID) {
			t.Fatal("rescan tombstoned room")
		}
		app.WatchRoomAuthCache.mu.Lock()
		_, cached := app.WatchRoomAuthCache.entries[watchRoomAuthKey{roomID: room.ID, userID: ownerID}]
		app.WatchRoomAuthCache.mu.Unlock()
		if !cached {
			t.Fatal("rescan invalidated authorization")
		}
		err = conn.WriteJSON(map[string]any{"type": "ping"})
		if err != nil {
			t.Fatal(err)
		}
		readUntilEventType(t, conn, "pong")
	}
	if probe.calls != 2 {
		t.Fatalf("probes=%d, expected initial and changed-file rescans", probe.calls)
	}
	_, err = app.Queries.GetWatchRoomByID(context.Background(), room.ID)
	if err != nil {
		t.Fatal(err)
	}
}
