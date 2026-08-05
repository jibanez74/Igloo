package main

import (
	"context"
	"os"
	"strings"
	"testing"

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

func TestCleanupRoomHLSSession(t *testing.T) {
	app := setupTestApp(t)
	defer app.DB.Close()

	t.Run("removes the cached session and tombstones the room", func(t *testing.T) {
		const roomID = int64(42)
		key := RoomHLSSessionKey(roomID)
		app.HLSSessionCache.SetDefault(key, &HLSSession{TempDir: t.TempDir()})

		app.CleanupRoomHLSSession(roomID)

		if _, ok := app.HLSSessionCache.Get(key); ok {
			t.Error("expected session to be removed from cache after cleanup")
		}
		if !app.isRoomHLSSessionDeleted(roomID) {
			t.Error("expected cleanup to mark the room hls session as deleted")
		}
	})

	// A room can be closed before anyone ever started playback. The tombstone
	// still has to land, or a late warm-up would resurrect the deleted room.
	t.Run("tombstones a room that never had a session", func(t *testing.T) {
		const roomID = int64(99999)

		app.CleanupRoomHLSSession(roomID)

		if !app.isRoomHLSSessionDeleted(roomID) {
			t.Error("expected cleanup to tombstone a room with no cached session")
		}
	})
}

func TestStoreRoomHLSSessionIfActive_RejectsDeletedRoom(t *testing.T) {
	app := setupTestApp(t)
	defer app.DB.Close()

	const roomID = int64(24)
	key := RoomHLSSessionKey(roomID)

	tempDir := t.TempDir()
	session := &HLSSession{TempDir: tempDir}

	app.CleanupRoomHLSSession(roomID)

	err := app.storeRoomHLSSessionIfActive(roomID, key, session)
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

	session, err := app.GetOrCreateRoomHLSSession(background, roomID, 999, "720p_3mbps", 0, nil, nil)
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

	err = app.WarmUpRoomHLSSession(background, 1, movieID, "720p_3mbps", 0, nil, nil)
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

	err := app.WarmUpRoomHLSSession(background, roomID, 999, "1080p_8mbps", 0, nil, nil)
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
		nil,
		nil,
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

func TestGetOrCreateRoomHLSSession_UsesPreloadedMovieAndAudioStreams(t *testing.T) {
	app := setupTestApp(t)
	defer app.DB.Close()

	fake := &fakeFFmpeg{
		plans: []fakeFFmpegRunPlan{
			hlsRunPlan(transcodeFixture),
		},
	}
	app.FFmpeg = fake

	movieID := insertTestHLSMovieFixture(t, app, "h264", 1080)
	_, err := app.DB.Exec(`
		INSERT INTO audio_streams (movie_id, stream_index, codec, bit_rate, channels, language)
		VALUES (?, ?, ?, ?, ?, ?)
	`, movieID, 3, "ac3", 448000, 6, "spa")
	if err != nil {
		t.Fatalf("insert second audio stream: %v", err)
	}

	movie, err := app.Queries.GetMovieByID(background, movieID)
	if err != nil {
		t.Fatalf("load movie: %v", err)
	}
	audioStreams, err := app.Queries.GetAudioStreamsByMovieID(background, movieID)
	if err != nil {
		t.Fatalf("load audio streams: %v", err)
	}

	// Deleting the rows pins that the warm-up serves from the preloaded slice:
	// a re-fetch would see zero audio streams and reject the audio track.
	_, err = app.DB.Exec(`DELETE FROM audio_streams WHERE movie_id = ?`, movieID)
	if err != nil {
		t.Fatalf("delete audio streams: %v", err)
	}

	session, err := app.GetOrCreateRoomHLSSession(
		background,
		91,
		movieID,
		helpers.HLS_PROFILE_720P_3MBPS,
		1,
		&movie,
		audioStreams,
	)
	if err != nil {
		t.Fatalf("GetOrCreateRoomHLSSession returned error: %v", err)
	}
	defer cleanupHLSSession(session)

	calls := fake.Calls()
	if len(calls) != 1 {
		t.Fatalf("expected 1 RunHLS call, got %d", len(calls))
	}
	if calls[0].AudioStreamIndex != 3 {
		t.Fatalf("AudioStreamIndex = %d, want 3 (absolute ffprobe index for ordinal 1)", calls[0].AudioStreamIndex)
	}
}

func TestInvalidateHLSSessionsForMovie(t *testing.T) {
	app := setupTestApp(t)
	defer app.DB.Close()

	const roomA = int64(41)
	const roomB = int64(42)
	personalKeyA := HLSSessionKey(1, "720p_3mbps", nil, "session-a", 0)
	personalKeyB := HLSSessionKey(2, "720p_3mbps", nil, "session-b", 0)

	app.HLSSessionCache.SetDefault(personalKeyA, &HLSSession{MovieID: 1, TempDir: t.TempDir()})
	app.HLSSessionCache.SetDefault(RoomHLSSessionKey(roomA), &HLSSession{MovieID: 1, IsRoom: true, TempDir: t.TempDir()})
	app.HLSSessionCache.SetDefault(personalKeyB, &HLSSession{MovieID: 2, TempDir: t.TempDir()})
	app.HLSSessionCache.SetDefault(RoomHLSSessionKey(roomB), &HLSSession{MovieID: 2, IsRoom: true, TempDir: t.TempDir()})

	app.invalidateHLSSessionsForMovie(1)

	if _, ok := app.HLSSessionCache.Get(personalKeyA); ok {
		t.Error("expected movie 1 personal session to be removed")
	}
	if _, ok := app.HLSSessionCache.Get(RoomHLSSessionKey(roomA)); ok {
		t.Error("expected movie 1 room session to be removed")
	}
	if _, ok := app.HLSSessionCache.Get(personalKeyB); !ok {
		t.Error("expected movie 2 personal session to survive")
	}
	if _, ok := app.HLSSessionCache.Get(RoomHLSSessionKey(roomB)); !ok {
		t.Error("expected movie 2 room session to survive")
	}

	// The room was not deleted, so no tombstone: the next manifest request
	// must be able to recreate the session instead of erroring.
	session, ok, err := app.getActiveRoomHLSSession(roomA, RoomHLSSessionKey(roomA))
	if err != nil {
		t.Fatalf("expected no tombstone error for invalidated room, got %v", err)
	}
	if ok || session != nil {
		t.Fatal("expected invalidated room session to be absent, not active")
	}
}
