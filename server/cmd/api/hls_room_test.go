package main

import (
	"context"
	"os"
	"strings"
	"testing"

	"igloo/cmd/internal/database"
	"igloo/cmd/internal/helpers"
)

func TestRoomHLSSessionKey_NoCollisionWithPersonalKey(t *testing.T) {
	roomKey := RoomHLSSessionKey(1)
	audioTrack := 0
	personalKey := HLSSessionKey(1, "720p_3mbps", &audioTrack, testPlaybackSessionID, 0)

	if roomKey == personalKey {
		t.Errorf("room key %q collides with personal key %q", roomKey, personalKey)
	}
	if !strings.HasPrefix(roomKey, "room:") {
		t.Errorf("room key %q should start with 'room:'", roomKey)
	}
	if strings.HasPrefix(personalKey, "room:") {
		t.Errorf("personal key %q should not start with 'room:'", personalKey)
	}
}

func TestCleanupRoomHLSSession_NoopWhenNoSession(t *testing.T) {
	app := setupTestApp(t)
	defer app.DB.Close()

	app.CleanupRoomHLSSession(99999)
}

func TestCleanupRoomHLSSession_RemovesSessionFromCache(t *testing.T) {
	app := setupTestApp(t)
	defer app.DB.Close()

	const roomID = int64(42)
	key := RoomHLSSessionKey(roomID)

	dummy := &HLSSession{TempDir: ""}
	app.HLSSessionCache.SetDefault(key, dummy)

	app.CleanupRoomHLSSession(roomID)

	if _, ok := app.HLSSessionCache.Get(key); ok {
		t.Error("expected session to be removed from cache after cleanup")
	}
	if !app.isRoomHLSSessionDeleted(roomID) {
		t.Error("expected cleanup to mark the room hls session as deleted")
	}
}

func TestStoreRoomHLSSessionIfActive_RejectsDeletedRoom(t *testing.T) {
	app := setupTestApp(t)
	defer app.DB.Close()

	const roomID = int64(24)
	key := RoomHLSSessionKey(roomID)

	tempDir, err := os.MkdirTemp("", "igloo-room-hls-*")
	if err != nil {
		t.Fatalf("mkdir temp: %v", err)
	}

	session := &HLSSession{TempDir: tempDir}

	app.CleanupRoomHLSSession(roomID)

	err = app.storeRoomHLSSessionIfActive(roomID, key, session)
	if err == nil {
		t.Fatal("expected deleted room session storage to fail")
	}
	if !strings.Contains(err.Error(), "was deleted") {
		t.Fatalf("error = %v, want deletion message", err)
	}
	if _, ok := app.HLSSessionCache.Get(key); ok {
		t.Fatal("expected no cached session for deleted room")
	}
	_, statErr := os.Stat(tempDir)
	if !os.IsNotExist(statErr) {
		t.Fatalf("expected temp dir cleanup, stat err = %v", statErr)
	}
}

func TestGetActiveRoomHLSSession_RejectsDeletedRoom(t *testing.T) {
	app := setupTestApp(t)
	defer app.DB.Close()

	const roomID = int64(51)
	key := RoomHLSSessionKey(roomID)
	sentinel := &HLSSession{TempDir: "sentinel"}

	app.HLSSessionCache.SetDefault(key, sentinel)
	app.CleanupRoomHLSSession(roomID)

	session, found, err := app.getActiveRoomHLSSession(roomID, key)
	if err == nil {
		t.Fatal("expected deleted room lookup to fail")
	}
	if !strings.Contains(err.Error(), "was deleted") {
		t.Fatalf("error = %v, want deletion message", err)
	}
	if found {
		t.Fatal("expected deleted room lookup to report no session")
	}
	if session != nil {
		t.Fatal("expected no session for deleted room")
	}
	if _, ok := app.HLSSessionCache.Get(key); ok {
		t.Fatal("expected cleanup to keep deleted room out of cache")
	}
}

func TestGetOrCreateRoomHLSSession_RejectsDeletedRoomCacheHit(t *testing.T) {
	app := setupTestApp(t)
	defer app.DB.Close()

	const roomID = int64(52)
	key := RoomHLSSessionKey(roomID)
	sentinel := &HLSSession{TempDir: "sentinel"}

	app.HLSSessionCache.SetDefault(key, sentinel)
	app.CleanupRoomHLSSession(roomID)

	session, err := app.GetOrCreateRoomHLSSession(background, roomID, 999, "720p_3mbps", 0)
	if err == nil {
		t.Fatal("expected deleted room cache hit to fail")
	}
	if !strings.Contains(err.Error(), "was deleted") {
		t.Fatalf("error = %v, want deletion message", err)
	}
	if session != nil {
		t.Fatal("expected no session for deleted room")
	}
	if _, ok := app.HLSSessionCache.Get(key); ok {
		t.Fatal("expected deleted room cache hit to remain absent from cache")
	}
}

func TestWarmUpRoomHLSSession_FailsWhenMovieHasNoVideoStream(t *testing.T) {
	app := setupTestApp(t)
	defer app.DB.Close()
	app.Settings = &database.Setting{}

	ctx := context.Background()
	_, err := app.DB.Exec(`
		INSERT INTO movies (title, file_path, file_name, size, container, mime_type, adult, duration)
		VALUES ('Room No Video', '/tmp/room-no-video.mkv', 'room-no-video.mkv', 1, 'mkv', 'video/x-matroska', 0, 3600.0)
	`)
	if err != nil {
		t.Fatalf("insert movie: %v", err)
	}

	var movieID int64
	err = app.DB.QueryRowContext(ctx, `SELECT id FROM movies WHERE file_path = '/tmp/room-no-video.mkv'`).Scan(&movieID)
	if err != nil {
		t.Fatalf("select movie id: %v", err)
	}

	err = app.WarmUpRoomHLSSession(background, 1, movieID, "720p_3mbps", 0)
	if err == nil {
		t.Fatal("expected error from warm-up when movie has no video streams")
	}
	if !strings.Contains(err.Error(), "no playable video track") {
		t.Errorf("error = %v, want mention of 'no playable video track'", err)
	}

	key := RoomHLSSessionKey(1)
	if _, ok := app.HLSSessionCache.Get(key); ok {
		t.Error("expected no cache entry when warm-up failed")
	}
}

func TestWarmUpRoomHLSSession_IdempotentWhenAlreadyCached(t *testing.T) {
	app := setupTestApp(t)
	defer app.DB.Close()

	const roomID = int64(7)
	key := RoomHLSSessionKey(roomID)

	sentinel := &HLSSession{TempDir: "sentinel"}
	app.HLSSessionCache.SetDefault(key, sentinel)

	err := app.WarmUpRoomHLSSession(background, roomID, 999, "1080p_8mbps", 0)
	if err != nil {
		t.Fatalf("unexpected error on second warm-up call: %v", err)
	}

	raw, ok := app.HLSSessionCache.Get(key)
	if !ok {
		t.Fatal("expected session to still be in cache")
	}
	if raw.(*HLSSession).TempDir != "sentinel" {
		t.Error("expected sentinel session to be unchanged")
	}
}

func TestGetOrCreateRoomHLSSession_RemuxUnsafeFallsBackAndCachesRoomKey(t *testing.T) {
	app := setupTestApp(t)
	defer app.DB.Close()
	app.Settings = &database.Setting{}

	fake := &fakeFFmpeg{
		plans: []fakeFFmpegRunPlan{
			hlsRunPlan(unsafeRemuxFixture),
			hlsRunPlan(transcodeFixture),
		},
	}
	app.FFmpeg = fake

	movieID := insertTestHLSMovieFixture(t, app, "h264", 1080)
	roomID := int64(77)

	session, err := app.GetOrCreateRoomHLSSession(
		background,
		roomID,
		movieID,
		helpers.HLS_PROFILE_REMUX,
		0,
	)
	if err != nil {
		t.Fatalf("GetOrCreateRoomHLSSession returned error: %v", err)
	}
	defer cleanupHLSSession(session)

	calls := fake.Calls()
	if len(calls) != 2 {
		t.Fatalf("RunHLS call count = %d, want 2", len(calls))
	}
	if calls[1].Profile != helpers.HLS_PROFILE_1080P_8MBPS {
		t.Fatalf("fallback RunHLS profile = %q, want %q", calls[1].Profile, helpers.HLS_PROFILE_1080P_8MBPS)
	}
	if session.CopyVideo {
		t.Fatal("CopyVideo = true, want false after room fallback")
	}

	key := RoomHLSSessionKey(roomID)
	raw, ok := app.HLSSessionCache.Get(key)
	if !ok {
		t.Fatalf("expected room session cache entry for key %q", key)
	}
	cachedSession, typeOK := raw.(*HLSSession)
	if !typeOK {
		t.Fatalf("cached session type = %T, want *HLSSession", raw)
	}
	if cachedSession != session {
		t.Fatal("expected cached room session to match returned session")
	}
}
