package main

import (
	"context"
	"strings"
	"testing"

	"igloo/cmd/internal/database"
)

// TestRoomHLSSessionKey_NoCollisionWithPersonalKey verifies that the room key
// format can never collide with a regular HLSSessionKey value.
func TestRoomHLSSessionKey_NoCollisionWithPersonalKey(t *testing.T) {
	roomKey := RoomHLSSessionKey(1)
	personalKey := HLSSessionKey(1, "720p_3mbps", 0)

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

// TestCleanupRoomHLSSession_NoopWhenNoSession verifies that calling cleanup
// for a room with no cached session does not panic or error.
func TestCleanupRoomHLSSession_NoopWhenNoSession(t *testing.T) {
	app := setupTestApp(t)
	defer app.DB.Close()

	// Must not panic
	app.CleanupRoomHLSSession(99999)
}

// TestCleanupRoomHLSSession_RemovesSessionFromCache verifies that after cleanup
// the session is no longer present in the HLS cache.
func TestCleanupRoomHLSSession_RemovesSessionFromCache(t *testing.T) {
	app := setupTestApp(t)
	defer app.DB.Close()

	const roomID = int64(42)
	key := RoomHLSSessionKey(roomID)

	// Plant a dummy session in the cache.
	dummy := &HLSSession{TempDir: ""}
	app.HLSSessionCache.SetDefault(key, dummy)

	app.CleanupRoomHLSSession(roomID)

	if _, ok := app.HLSSessionCache.Get(key); ok {
		t.Error("expected session to be removed from cache after cleanup")
	}
}

// TestWarmUpRoomHLSSession_FailsWhenMovieHasNoVideoStream verifies that warm-up
// returns an error (and stores nothing) when the movie lacks video streams.
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

	// WarmUpRoomHLSSession should propagate the "no playable video track" error
	// from createHLSSession and must not cache a failed room session.
	err = app.WarmUpRoomHLSSession(background, 1, movieID, "720p_3mbps", 0)
	if err == nil {
		t.Fatal("expected error from warm-up when movie has no video streams")
	}
	if !strings.Contains(err.Error(), "no playable video track") {
		t.Errorf("error = %v, want mention of 'no playable video track'", err)
	}

	// Nothing should have been stored in the cache.
	key := RoomHLSSessionKey(1)
	if _, ok := app.HLSSessionCache.Get(key); ok {
		t.Error("expected no cache entry when warm-up failed")
	}
}

// TestWarmUpRoomHLSSession_IdempotentWhenAlreadyCached verifies that a second
// call to WarmUpRoomHLSSession is a no-op when a session is already cached.
func TestWarmUpRoomHLSSession_IdempotentWhenAlreadyCached(t *testing.T) {
	app := setupTestApp(t)
	defer app.DB.Close()

	const roomID = int64(7)
	key := RoomHLSSessionKey(roomID)

	// Pre-populate the cache with a sentinel session.
	sentinel := &HLSSession{TempDir: "sentinel"}
	app.HLSSessionCache.SetDefault(key, sentinel)

	// WarmUpRoomHLSSession should return nil without touching the sentinel.
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
