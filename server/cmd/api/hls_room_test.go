package main

import (
	"errors"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"igloo/cmd/internal/helpers"
)

func TestCleanupRoomHLSSession(t *testing.T) {
	app := setupTestApp(t)

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

// go-cache fires OnEvicted from Delete and DeleteExpired but not from Set, so
// overwriting a key whose entry expired before the janitor swept it dropped the
// old session with no teardown: its FFmpeg process ran on to the end of the
// movie and its temp dir survived until the next boot sweep.
// Built on initRuntimeCaches rather than setupTestApp: the shared helper
// installs a session cache with no eviction hook, and the eviction hook is
// exactly what this test is about.
func TestStoreRoomHLSSessionIfActive_TearsDownAnExpiredPredecessor(t *testing.T) {
	app := &Application{Wait: &sync.WaitGroup{}}
	app.initRuntimeCaches()

	const roomID = int64(77)
	key := RoomHLSSessionKey(roomID)

	staleDir := t.TempDir()
	stale := &HLSSession{TempDir: staleDir, IsRoom: true}
	app.HLSSessionCache.Set(key, stale, time.Millisecond)
	time.Sleep(5 * time.Millisecond)

	replacement := &HLSSession{TempDir: t.TempDir(), IsRoom: true}
	err := app.storeRoomHLSSessionIfActive(roomID, key, replacement)
	if err != nil {
		t.Fatalf("unexpected error storing the replacement session: %v", err)
	}

	// Eviction hands teardown to a goroutine so the sweep never blocks on
	// FFmpeg exiting, so the removal is observed rather than assumed.
	deadline := time.Now().Add(5 * time.Second)
	for {
		_, statErr := os.Stat(staleDir)
		if os.IsNotExist(statErr) {
			break
		}
		if !time.Now().Before(deadline) {
			t.Fatalf("expected the superseded session's temp dir to be removed, stat err = %v", statErr)
		}
		time.Sleep(5 * time.Millisecond)
	}

	raw, ok := app.HLSSessionCache.Get(key)
	if !ok {
		t.Fatal("expected the replacement session to be cached")
	}
	if raw != replacement {
		t.Fatalf("cached session = %v, want the replacement", raw)
	}
}

func TestGetOrCreateRoomHLSSession_RejectsDeletedRoomCacheHit(t *testing.T) {
	app := setupTestApp(t)

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

	movieID := createTestMovie(t, app, "Room No Video", "/tmp/room-no-video.mkv")
	_, err := app.DB.Exec(`UPDATE movies SET duration = 3600.0 WHERE id = ?`, movieID)
	if err != nil {
		t.Fatalf("set movie duration: %v", err)
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
	if calls[1].Profile != helpers.HLS_PROFILE_1080P_4MBPS {
		t.Fatalf("fallback RunHLS profile = %q, want %q", calls[1].Profile, helpers.HLS_PROFILE_1080P_4MBPS)
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

	fake := &fakeFFmpeg{
		plans: []fakeFFmpegRunPlan{
			hlsRunPlan(transcodeFixture),
		},
	}
	app.FFmpeg = fake

	movieID := insertTestHLSMovieFixture(t, app, "h264", 1080)
	insertTestSecondaryAudioStream(t, app, movieID, "ac3", 448000, 6, "")

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

	const roomA = int64(41)
	const roomB = int64(42)
	personalKeyA := HLSSessionKey(movieRef(1), "720p_3mbps", nil, nil, "session-a", 0, 100)
	personalKeyB := HLSSessionKey(movieRef(2), "720p_3mbps", nil, nil, "session-b", 0, 100)

	app.HLSSessionCache.SetDefault(personalKeyA, &HLSSession{Media: movieRef(1), FileID: 1, TempDir: t.TempDir()})
	app.HLSSessionCache.SetDefault(RoomHLSSessionKey(roomA), &HLSSession{Media: movieRef(1), FileID: 1, IsRoom: true, TempDir: t.TempDir()})
	app.HLSSessionCache.SetDefault(personalKeyB, &HLSSession{Media: movieRef(2), FileID: 2, TempDir: t.TempDir()})
	app.HLSSessionCache.SetDefault(RoomHLSSessionKey(roomB), &HLSSession{Media: movieRef(2), FileID: 2, IsRoom: true, TempDir: t.TempDir()})

	app.invalidateHLSSessionsForFile(mediaKindMovie, 1)

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

// One room session serves every member and each of their requests refreshes
// it, so a failed one used to stay until the room was deleted. The segment
// path keeps serving what it wrote; the next manifest request replaces it.
func TestGetOrCreateRoomHLSSession_ReplacesFailedSession(t *testing.T) {
	app := setupTestApp(t)
	fake := &fakeFFmpeg{plans: []fakeFFmpegRunPlan{hlsRunPlan(transcodeFixture)}}
	app.FFmpeg = fake

	movieID := insertTestHLSMovieFixture(t, app, "h264", 1080)
	const roomID = int64(31)
	key := RoomHLSSessionKey(roomID)
	oldDir := t.TempDir()
	old := &HLSSession{
		Media:   movieRef(movieID),
		FileID:  movieID,
		IsRoom:  true,
		TempDir: oldDir,
		Exited:  true,
		ExitErr: errors.New("exit status 1"),
	}
	app.HLSSessionCache.Set(key, old, hlsRoomSessionTTL)

	segmentSession, found, err := app.getActiveRoomHLSSession(roomID, key)
	if err != nil || !found || segmentSession != old {
		t.Fatalf("segment lookup = (%p, %v, %v), want the failed session", segmentSession, found, err)
	}

	session, err := app.GetOrCreateRoomHLSSession(background, roomID, movieID, helpers.HLS_PROFILE_720P_3MBPS, 0, nil, nil)
	if err != nil {
		t.Fatalf("GetOrCreateRoomHLSSession error: %v", err)
	}
	defer cleanupHLSSession(session)

	if session == old {
		t.Fatal("failed room session was served again, want a replacement")
	}
	if fake.CallCount() != 1 {
		t.Fatalf("RunHLS call count = %d, want 1", fake.CallCount())
	}
	raw, ok := app.HLSSessionCache.Get(key)
	if !ok || raw != session {
		t.Fatal("replacement is not the cached room session")
	}
	if _, statErr := os.Stat(oldDir); !os.IsNotExist(statErr) {
		t.Fatalf("failed room session temp dir still exists (stat error %v)", statErr)
	}
}

func TestHLSSessionCacheExpirationDoesNotWaitForTeardown(t *testing.T) {
	app := &Application{Wait: &sync.WaitGroup{}}
	app.initRuntimeCaches()

	session := &HLSSession{TempDir: t.TempDir()}
	cleanupStarted, releaseCleanup := blockHLSSessionCleanup(t, session)
	app.HLSSessionCache.Set("expired-session", session, time.Millisecond)
	time.Sleep(5 * time.Millisecond)

	deleteDone := make(chan struct{})
	go func() {
		app.HLSSessionCache.DeleteExpired()
		close(deleteDone)
	}()
	waitForHLSSessionCleanupToBlock(t, cleanupStarted, releaseCleanup)
	select {
	case <-deleteDone:
	default:
		releaseCleanup()
		t.Fatal("DeleteExpired waited for HLS session teardown")
	}
	_, cached := app.HLSSessionCache.Get("expired-session")
	if cached {
		releaseCleanup()
		t.Fatal("expired session remained cached while teardown was blocked")
	}

	releaseCleanup()
	cleanupHLSSession(session)
	_, err := os.Stat(session.TempDir)
	if !os.IsNotExist(err) {
		t.Fatalf("expired session temp dir still exists after cleanup: %v", err)
	}
}
