package main

import (
	"context"
	"errors"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"igloo/cmd/internal/database"
	"igloo/cmd/internal/helpers"
)

func TestCreateHLSSession_ErrorsWhenMovieHasNoDuration(t *testing.T) {
	app := setupTestApp(t)
	defer app.DB.Close()
	app.Settings = &database.Setting{}

	ctx := context.Background()
	_, err := app.DB.Exec(`
		INSERT INTO movies (title, file_path, file_name, size, container, mime_type, adult)
		VALUES ('No Duration', '/tmp/nodur.mkv', 'nodur.mkv', 1, 'mkv', 'video/x-matroska', 0)
	`)
	if err != nil {
		t.Fatalf("insert movie: %v", err)
	}
	var id int64
	err = app.DB.QueryRowContext(ctx, `SELECT id FROM movies WHERE file_path = '/tmp/nodur.mkv'`).Scan(&id)
	if err != nil {
		t.Fatalf("select id: %v", err)
	}

	_, err = createTestHLSSession(app, ctx, id, "720p_3mbps", testIntPtr(0), testPlaybackSessionID, 0, false)
	if err == nil {
		t.Fatal("expected error when duration missing")
	}
	if !strings.Contains(err.Error(), "no valid duration") {
		t.Errorf("error = %v, want mention of no valid duration", err)
	}
}

func TestCreateHLSSession_ErrorsWhenNoVideoStream(t *testing.T) {
	app := setupTestApp(t)
	defer app.DB.Close()
	app.Settings = &database.Setting{}

	ctx := context.Background()
	_, err := app.DB.Exec(`
		INSERT INTO movies (title, file_path, file_name, size, container, mime_type, adult, duration)
		VALUES ('No Video', '/tmp/novid.mkv', 'novid.mkv', 1, 'mkv', 'video/x-matroska', 0, 3600.0)
	`)
	if err != nil {
		t.Fatalf("insert movie: %v", err)
	}
	var id int64
	err = app.DB.QueryRowContext(ctx, `SELECT id FROM movies WHERE file_path = '/tmp/novid.mkv'`).Scan(&id)
	if err != nil {
		t.Fatalf("select id: %v", err)
	}

	_, err = createTestHLSSession(app, ctx, id, "720p_3mbps", testIntPtr(0), testPlaybackSessionID, 0, false)
	if err == nil {
		t.Fatal("expected error when no video stream rows")
	}
	if !strings.Contains(err.Error(), "no playable video track") {
		t.Errorf("error = %v, want mention of no playable video", err)
	}
}

func TestCreateHLSSession_RemuxSafeStaysOnRemux(t *testing.T) {
	app := setupTestApp(t)
	defer app.DB.Close()
	app.Settings = &database.Setting{}

	fake := &fakeFFmpeg{
		plans: []fakeFFmpegRunPlan{
			hlsRunPlan(safeRemuxFixture),
		},
	}
	app.FFmpeg = fake

	movieID := insertTestHLSMovieFixture(t, app, "h264", 1080)

	session, err := createTestHLSSession(app, context.Background(), movieID, helpers.HLS_PROFILE_REMUX, testIntPtr(0), testPlaybackSessionID, 0, false)
	if err != nil {
		t.Fatalf("createHLSSession returned error: %v", err)
	}
	defer cleanupHLSSession(session)

	if !session.CopyVideo {
		t.Fatal("CopyVideo = false, want true for safe remux")
	}

	calls := fake.Calls()
	if len(calls) != 1 {
		t.Fatalf("RunHLS call count = %d, want 1", len(calls))
	}
	if calls[0].Profile != helpers.HLS_PROFILE_REMUX {
		t.Fatalf("first RunHLS profile = %q, want remux", calls[0].Profile)
	}
	if !calls[0].CopyVideo {
		t.Fatal("first RunHLS CopyVideo = false, want true")
	}
}

func TestCreateHLSSession_RemuxUnsafeFallsBackToBestFitTranscode(t *testing.T) {
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
	startSec := 87

	session, err := createTestHLSSession(
		app,
		context.Background(),
		movieID,
		helpers.HLS_PROFILE_REMUX,
		testIntPtr(0),
		testPlaybackSessionID,
		startSec,
		false,
	)
	if err != nil {
		t.Fatalf("createHLSSession returned error: %v", err)
	}
	defer cleanupHLSSession(session)

	if session.CopyVideo {
		t.Fatal("CopyVideo = true, want false after fallback transcode")
	}
	if session.StartSec != float64(startSec) {
		t.Fatalf("StartSec = %v, want %v", session.StartSec, startSec)
	}
	calls := fake.Calls()
	if len(calls) != 2 {
		t.Fatalf("RunHLS call count = %d, want 2", len(calls))
	}
	if calls[0].Profile != helpers.HLS_PROFILE_REMUX {
		t.Fatalf("first RunHLS profile = %q, want remux", calls[0].Profile)
	}
	if !calls[0].CopyVideo {
		t.Fatal("first RunHLS CopyVideo = false, want true")
	}
	if calls[1].Profile != helpers.HLS_PROFILE_1080P_8MBPS {
		t.Fatalf("second RunHLS profile = %q, want %q", calls[1].Profile, helpers.HLS_PROFILE_1080P_8MBPS)
	}
	if calls[1].CopyVideo {
		t.Fatal("second RunHLS CopyVideo = true, want false")
	}
	if calls[1].StartSec != float64(startSec) {
		t.Fatalf("second RunHLS StartSec = %v, want %v", calls[1].StartSec, startSec)
	}
}

func TestCreateHLSSession_CachedUnsafeSkipsRemux(t *testing.T) {
	app := setupTestApp(t)
	defer app.DB.Close()
	app.Settings = &database.Setting{}

	fake := &fakeFFmpeg{
		plans: []fakeFFmpegRunPlan{
			hlsRunPlan(unsafeRemuxFixture),
			hlsRunPlan(transcodeFixture),
			hlsRunPlan(transcodeFixture),
		},
	}
	app.FFmpeg = fake

	movieID := insertTestHLSMovieFixture(t, app, "h264", 1080)

	firstSession, err := createTestHLSSession(app, context.Background(), movieID, helpers.HLS_PROFILE_REMUX, testIntPtr(0), testPlaybackSessionID, 0, false)
	if err != nil {
		t.Fatalf("first createHLSSession returned error: %v", err)
	}
	defer cleanupHLSSession(firstSession)

	secondSession, err := createTestHLSSession(app, context.Background(), movieID, helpers.HLS_PROFILE_REMUX, testIntPtr(0), testPlaybackSessionID, 0, false)
	if err != nil {
		t.Fatalf("second createHLSSession returned error: %v", err)
	}
	defer cleanupHLSSession(secondSession)

	calls := fake.Calls()
	if len(calls) != 3 {
		t.Fatalf("RunHLS call count = %d, want 3", len(calls))
	}
	if calls[0].Profile != helpers.HLS_PROFILE_REMUX {
		t.Fatalf("first RunHLS profile = %q, want remux", calls[0].Profile)
	}
	if calls[1].Profile != helpers.HLS_PROFILE_1080P_8MBPS {
		t.Fatalf("second RunHLS profile = %q, want %q", calls[1].Profile, helpers.HLS_PROFILE_1080P_8MBPS)
	}
	if calls[2].Profile != helpers.HLS_PROFILE_1080P_8MBPS {
		t.Fatalf("third RunHLS profile = %q, want cached fallback %q", calls[2].Profile, helpers.HLS_PROFILE_1080P_8MBPS)
	}
	if secondSession.CopyVideo {
		t.Fatal("second session CopyVideo = true, want false for cached-unsafe fallback")
	}
}

func TestCreateHLSSession_RemuxPreflightFailureDoesNotCacheUnsafe(t *testing.T) {
	app := setupTestApp(t)
	defer app.DB.Close()
	app.Settings = &database.Setting{}

	fake := &fakeFFmpeg{
		plans: []fakeFFmpegRunPlan{
			{
				ExitErr: errors.New("ffmpeg exited before writing remux preflight output"),
			},
			hlsRunPlan(transcodeFixture),
			hlsRunPlan(safeRemuxFixture),
		},
	}
	app.FFmpeg = fake

	movieID := insertTestHLSMovieFixture(t, app, "h264", 1080)

	firstSession, err := createTestHLSSession(app, context.Background(), movieID, helpers.HLS_PROFILE_REMUX, testIntPtr(0), testPlaybackSessionID, 0, false)
	if err != nil {
		t.Fatalf("first createHLSSession returned error: %v", err)
	}
	defer cleanupHLSSession(firstSession)

	if firstSession.CopyVideo {
		t.Fatal("first session CopyVideo = true, want false after preflight fallback")
	}

	secondSession, err := createTestHLSSession(app, context.Background(), movieID, helpers.HLS_PROFILE_REMUX, testIntPtr(0), testPlaybackSessionID, 0, false)
	if err != nil {
		t.Fatalf("second createHLSSession returned error: %v", err)
	}
	defer cleanupHLSSession(secondSession)

	if !secondSession.CopyVideo {
		t.Fatal("second session CopyVideo = false, want true after retrying remux")
	}

	calls := fake.Calls()
	if len(calls) != 3 {
		t.Fatalf("RunHLS call count = %d, want 3", len(calls))
	}
	if calls[0].Profile != helpers.HLS_PROFILE_REMUX {
		t.Fatalf("first RunHLS profile = %q, want remux", calls[0].Profile)
	}
	if calls[1].Profile != helpers.HLS_PROFILE_1080P_8MBPS {
		t.Fatalf("second RunHLS profile = %q, want %q", calls[1].Profile, helpers.HLS_PROFILE_1080P_8MBPS)
	}
	if calls[2].Profile != helpers.HLS_PROFILE_REMUX {
		t.Fatalf("third RunHLS profile = %q, want remux retry", calls[2].Profile)
	}
}

func TestCreateHLSSession_RemuxNonH264StartsDirectlyWithFallback(t *testing.T) {
	app := setupTestApp(t)
	defer app.DB.Close()
	app.Settings = &database.Setting{}

	fake := &fakeFFmpeg{
		plans: []fakeFFmpegRunPlan{
			hlsRunPlan(transcodeFixture),
		},
	}
	app.FFmpeg = fake

	movieID := insertTestHLSMovieFixture(t, app, "hevc", 2160)

	session, err := createTestHLSSession(app, context.Background(), movieID, helpers.HLS_PROFILE_REMUX, testIntPtr(0), testPlaybackSessionID, 0, false)
	if err != nil {
		t.Fatalf("createHLSSession returned error: %v", err)
	}
	defer cleanupHLSSession(session)

	if session.CopyVideo {
		t.Fatal("CopyVideo = true, want false for non-H.264 fallback")
	}
	calls := fake.Calls()
	if len(calls) != 1 {
		t.Fatalf("RunHLS call count = %d, want 1", len(calls))
	}
	if calls[0].Profile != helpers.HLS_PROFILE_2160P_16MBPS {
		t.Fatalf("RunHLS profile = %q, want %q", calls[0].Profile, helpers.HLS_PROFILE_2160P_16MBPS)
	}
	if calls[0].CopyVideo {
		t.Fatal("RunHLS CopyVideo = true, want false")
	}
}

func TestCreateHLSSession_RemuxHigh10H264FallsBackToTranscode(t *testing.T) {
	app := setupTestApp(t)
	defer app.DB.Close()
	app.Settings = &database.Setting{}

	fake := &fakeFFmpeg{
		plans: []fakeFFmpegRunPlan{
			hlsRunPlan(transcodeFixture),
		},
	}
	app.FFmpeg = fake

	movieID := insertTestHLSMovieFixture(t, app, "h264", 1080)
	_, err := app.DB.Exec(`
		UPDATE video_streams
		SET codec_profile = ?, bit_depth = ?, pixel_format = ?
		WHERE movie_id = ?
	`, "High 10", 10, "yuv420p10le", movieID)
	if err != nil {
		t.Fatalf("update video stream: %v", err)
	}

	session, err := createTestHLSSession(app, context.Background(), movieID, helpers.HLS_PROFILE_REMUX, testIntPtr(0), testPlaybackSessionID, 0, false)
	if err != nil {
		t.Fatalf("createHLSSession returned error: %v", err)
	}
	defer cleanupHLSSession(session)

	if session.CopyVideo {
		t.Fatal("CopyVideo = true, want false for High 10 H.264 fallback")
	}

	calls := fake.Calls()
	if len(calls) != 1 {
		t.Fatalf("RunHLS call count = %d, want 1", len(calls))
	}
	if calls[0].Profile != helpers.HLS_PROFILE_1080P_8MBPS || calls[0].CopyVideo {
		t.Fatalf("RunHLS call = profile %q copyVideo %v, want fallback transcode", calls[0].Profile, calls[0].CopyVideo)
	}
}

func TestCreateHLSSession_NonRemuxProfilesRemainUnchanged(t *testing.T) {
	app := setupTestApp(t)
	defer app.DB.Close()
	app.Settings = &database.Setting{}

	fake := &fakeFFmpeg{
		plans: []fakeFFmpegRunPlan{
			hlsRunPlan(transcodeFixture),
		},
	}
	app.FFmpeg = fake

	movieID := insertTestHLSMovieFixture(t, app, "h264", 1080)

	session, err := createTestHLSSession(app, context.Background(), movieID, helpers.HLS_PROFILE_1080P_6MBPS, testIntPtr(0), testPlaybackSessionID, 0, false)
	if err != nil {
		t.Fatalf("createHLSSession returned error: %v", err)
	}
	defer cleanupHLSSession(session)

	if session.CopyVideo {
		t.Fatal("CopyVideo = true, want false for non-remux profile")
	}
	calls := fake.Calls()
	if len(calls) != 1 {
		t.Fatalf("RunHLS call count = %d, want 1", len(calls))
	}
	if calls[0].SourceFrameRate != 23.976 {
		t.Fatalf("RunHLS SourceFrameRate = %v, want 23.976", calls[0].SourceFrameRate)
	}
}

func TestCreateHLSSession_CopyVideoBypassesTranscodeLimiter(t *testing.T) {
	app := setupTestApp(t)
	defer app.DB.Close()
	app.Settings = &database.Setting{}

	fake := &fakeFFmpeg{
		plans: []fakeFFmpegRunPlan{
			hlsRunPlan(safeRemuxFixture),
		},
	}
	app.FFmpeg = fake

	// Exhaust the only transcode slot; a copy-video (remux) session must not need it.
	app.HLSTranscodeLimiter = newHLSTranscodeLimiter(1)
	release, err := app.acquireHLSTranscodeSlot()
	if err != nil {
		t.Fatalf("acquireHLSTranscodeSlot: %v", err)
	}
	defer release()

	movieID := insertTestHLSMovieFixture(t, app, "h264", 1080)

	session, err := createTestHLSSession(app, context.Background(), movieID, helpers.HLS_PROFILE_REMUX, testIntPtr(0), testPlaybackSessionID, 0, false)
	if err != nil {
		t.Fatalf("createHLSSession returned error: %v", err)
	}
	defer cleanupHLSSession(session)

	if !session.CopyVideo {
		t.Fatal("CopyVideo = false, want true for safe remux")
	}
}

func TestCreateHLSSession_TranscodeFailsWhenLimiterFull(t *testing.T) {
	app := setupTestApp(t)
	defer app.DB.Close()
	app.Settings = &database.Setting{}
	app.FFmpeg = &fakeFFmpeg{}

	app.HLSTranscodeLimiter = newHLSTranscodeLimiter(1)
	release, err := app.acquireHLSTranscodeSlot()
	if err != nil {
		t.Fatalf("acquireHLSTranscodeSlot: %v", err)
	}
	defer release()

	movieID := insertTestHLSMovieFixture(t, app, "h264", 1080)

	_, err = createTestHLSSession(app, context.Background(), movieID, helpers.HLS_PROFILE_720P_3MBPS, testIntPtr(0), testPlaybackSessionID, 0, false)
	var capacityErr *hlsTranscodeCapacityError
	if !errors.As(err, &capacityErr) {
		t.Fatalf("expected hlsTranscodeCapacityError, got %v", err)
	}
}

func TestGetOrCreateHLSSession_ReservationsCapConcurrentRemuxStarts(t *testing.T) {
	app := setupTestApp(t)
	defer app.DB.Close()
	app.Settings = &database.Setting{}
	app.HLSMaxPersonalSessionsPerUser = 2
	app.DB.SetMaxOpenConns(1)

	started := make(chan struct{}, 2)
	continueStarts := make(chan struct{})
	plan := hlsRunPlan(safeRemuxFixture)
	plan.Started = started
	plan.Continue = continueStarts
	fake := &fakeFFmpeg{plans: []fakeFFmpegRunPlan{plan, plan}}
	app.FFmpeg = fake

	movieID := insertTestHLSMovieFixture(t, app, "h264", 1080)
	userID := int64(100)
	playbackSessions := []string{
		testPlaybackSessionID,
		testOtherPlaybackSessionID,
		"11111111-1111-4111-8111-111111111111",
		"22222222-2222-4222-8222-222222222222",
	}
	type result struct {
		session *HLSSession
		err     error
	}
	results := make(chan result, 2)
	for i := 0; i < 2; i++ {
		playbackSession := playbackSessions[i]
		startSec := i * 20
		go func() {
			session, _, err := app.GetOrCreateHLSSession(
				context.Background(),
				movieID,
				helpers.HLS_PROFILE_REMUX,
				testIntPtr(0),
				playbackSession,
				startSec,
				userID,
			)
			results <- result{session: session, err: err}
		}()
	}

	<-started
	<-started

	for i := 2; i < len(playbackSessions); i++ {
		_, _, err := app.GetOrCreateHLSSession(
			context.Background(),
			movieID,
			helpers.HLS_PROFILE_REMUX,
			testIntPtr(0),
			playbackSessions[i],
			i*20,
			userID,
		)
		var capacityErr *hlsPersonalSessionCapacityError
		if !errors.As(err, &capacityErr) {
			t.Fatalf("request %d error = %v, want personal-session capacity error", i, err)
		}
	}
	if fake.CallCount() != 2 {
		t.Fatalf("RunHLS call count while reservations are pending = %d, want 2", fake.CallCount())
	}

	close(continueStarts)
	for i := 0; i < 2; i++ {
		created := <-results
		if created.err != nil {
			t.Fatalf("admitted request returned error: %v", created.err)
		}
		defer cleanupHLSSession(created.session)
	}

	app.PersonalHLSMu.Lock()
	reserved := app.PersonalHLSReservations[userID]
	cached := len(app.personalHLSSessionsForOwnerLocked(userID))
	app.PersonalHLSMu.Unlock()
	if reserved != 0 {
		t.Fatalf("pending reservations = %d, want 0", reserved)
	}
	if cached != 2 {
		t.Fatalf("cached personal sessions = %d, want 2", cached)
	}
}

func TestReservePersonalHLSSession_UpdatesCapacityBeforeBlockingTeardown(t *testing.T) {
	app := setupTestApp(t)
	defer app.DB.Close()
	app.HLSMaxPersonalSessionsPerUser = 1

	userID := int64(100)
	movieID := int64(5)
	oldKey := HLSSessionKey(movieID, helpers.HLS_PROFILE_720P_3MBPS, nil, testOtherPlaybackSessionID, 0)
	oldSession := &HLSSession{
		MovieID:         movieID,
		OwnerUserID:     userID,
		PlaybackSession: testOtherPlaybackSessionID,
		TempDir:         t.TempDir(),
	}
	cleanupStarted, releaseCleanup := blockHLSSessionCleanup(t, oldSession)
	app.HLSSessionCache.Set(oldKey, oldSession, hlsPersonalSessionTTL)

	type result struct {
		reservation *hlsPersonalSessionReservation
		err         error
	}
	resultCh := make(chan result, 1)
	go func() {
		reservation, err := app.reservePersonalHLSSession(movieID, userID, testPlaybackSessionID)
		resultCh <- result{reservation: reservation, err: err}
	}()

	waitForHLSSessionCleanupToBlock(t, cleanupStarted, releaseCleanup)
	_, cached := app.HLSSessionCache.Get(oldKey)
	if cached {
		releaseCleanup()
		t.Fatal("capacity victim remained cached while teardown was blocked")
	}
	if !app.PersonalHLSMu.TryLock() {
		releaseCleanup()
		t.Fatal("PersonalHLSMu remained locked while capacity-victim teardown was blocked")
	}
	reserved := app.PersonalHLSReservations[userID]
	app.PersonalHLSMu.Unlock()
	if reserved != 1 {
		releaseCleanup()
		t.Fatalf("pending reservations during teardown = %d, want 1", reserved)
	}

	releaseCleanup()
	reserveResult := waitForHLSSessionCleanupResult(t, resultCh)
	if reserveResult.err != nil {
		t.Fatalf("reservePersonalHLSSession returned error: %v", reserveResult.err)
	}
	reserveResult.reservation.release()
	_, err := os.Stat(oldSession.TempDir)
	if !os.IsNotExist(err) {
		t.Fatalf("capacity victim temp dir still exists after cleanup: %v", err)
	}
}

func TestPersonalHLSSessionReservationCommit_UpdatesAccountingBeforeBlockingTeardown(t *testing.T) {
	app := setupTestApp(t)
	defer app.DB.Close()

	userID := int64(100)
	movieID := int64(5)
	oldKey := HLSSessionKey(movieID, helpers.HLS_PROFILE_720P_3MBPS, nil, testPlaybackSessionID, 0)
	newKey := HLSSessionKey(movieID, helpers.HLS_PROFILE_720P_3MBPS, nil, testPlaybackSessionID, 30)
	oldSession := &HLSSession{
		MovieID:         movieID,
		OwnerUserID:     userID,
		PlaybackSession: testPlaybackSessionID,
		TempDir:         t.TempDir(),
	}
	newSession := &HLSSession{
		MovieID:         movieID,
		OwnerUserID:     userID,
		PlaybackSession: testPlaybackSessionID,
		TempDir:         t.TempDir(),
		Exited:          true,
	}
	defer cleanupHLSSession(newSession)
	cleanupStarted, releaseCleanup := blockHLSSessionCleanup(t, oldSession)
	app.HLSSessionCache.Set(oldKey, oldSession, hlsPersonalSessionTTL)
	app.PersonalHLSReservations[userID] = 1
	reservation := &hlsPersonalSessionReservation{app: app, ownerUserID: userID}
	commitDone := make(chan struct{})
	go func() {
		reservation.commit(movieID, newKey, newSession)
		close(commitDone)
	}()

	waitForHLSSessionCleanupToBlock(t, cleanupStarted, releaseCleanup)
	_, oldCached := app.HLSSessionCache.Get(oldKey)
	if oldCached {
		releaseCleanup()
		t.Fatal("superseded session remained cached while teardown was blocked")
	}
	raw, newCached := app.HLSSessionCache.Get(newKey)
	if !newCached || raw != newSession {
		releaseCleanup()
		t.Fatal("replacement session was not cached before superseded teardown")
	}
	if !app.PersonalHLSMu.TryLock() {
		releaseCleanup()
		t.Fatal("PersonalHLSMu remained locked while superseded teardown was blocked")
	}
	reserved := app.PersonalHLSReservations[userID]
	app.PersonalHLSMu.Unlock()
	if reserved != 0 {
		releaseCleanup()
		t.Fatalf("pending reservations during superseded teardown = %d, want 0", reserved)
	}

	releaseCleanup()
	waitForHLSSessionCleanupResult(t, commitDone)
	_, err := os.Stat(oldSession.TempDir)
	if !os.IsNotExist(err) {
		t.Fatalf("superseded session temp dir still exists after cleanup: %v", err)
	}
}

func TestReclaimIdlePersonalHLSSession_ReleasesLockBeforeTeardown(t *testing.T) {
	app := setupTestApp(t)
	defer app.DB.Close()

	userID := int64(100)
	movieID := int64(5)
	key := HLSSessionKey(movieID, helpers.HLS_PROFILE_720P_3MBPS, nil, testOtherPlaybackSessionID, 0)
	session := &HLSSession{
		MovieID:         movieID,
		OwnerUserID:     userID,
		PlaybackSession: testOtherPlaybackSessionID,
		TempDir:         t.TempDir(),
	}
	cleanupStarted, releaseCleanup := blockHLSSessionCleanup(t, session)
	app.HLSSessionCache.Set(
		key,
		session,
		hlsPersonalSessionTTL-hlsIdlePermitReclaimThreshold-time.Second,
	)
	resultCh := make(chan bool, 1)
	go func() {
		resultCh <- app.reclaimIdlePersonalHLSSessionForOwner(userID)
	}()

	waitForHLSSessionCleanupToBlock(t, cleanupStarted, releaseCleanup)
	_, cached := app.HLSSessionCache.Get(key)
	if cached {
		releaseCleanup()
		t.Fatal("idle session remained cached while teardown was blocked")
	}
	if !app.PersonalHLSMu.TryLock() {
		releaseCleanup()
		t.Fatal("PersonalHLSMu remained locked while idle-session teardown was blocked")
	}
	app.PersonalHLSMu.Unlock()

	releaseCleanup()
	reclaimed := waitForHLSSessionCleanupResult(t, resultCh)
	if !reclaimed {
		t.Fatal("reclaimIdlePersonalHLSSessionForOwner returned false")
	}
	_, err := os.Stat(session.TempDir)
	if !os.IsNotExist(err) {
		t.Fatalf("idle session temp dir still exists after cleanup: %v", err)
	}
}

func TestGetOrCreateHLSSession_FailedCreationReleasesReservation(t *testing.T) {
	app := setupTestApp(t)
	defer app.DB.Close()
	app.Settings = &database.Setting{}
	app.HLSMaxPersonalSessionsPerUser = 1
	app.FFmpeg = &fakeFFmpeg{plans: []fakeFFmpegRunPlan{
		{StartErr: errors.New("ffmpeg startup failed")},
		{},
	}}

	movieID := insertTestHLSMovieFixture(t, app, "h264", 1080)
	userID := int64(100)
	_, _, err := app.GetOrCreateHLSSession(
		context.Background(),
		movieID,
		helpers.HLS_PROFILE_720P_3MBPS,
		testIntPtr(0),
		testPlaybackSessionID,
		0,
		userID,
	)
	if err == nil {
		t.Fatal("first creation error = nil, want FFmpeg startup error")
	}

	app.PersonalHLSMu.Lock()
	reserved := app.PersonalHLSReservations[userID]
	app.PersonalHLSMu.Unlock()
	if reserved != 0 {
		t.Fatalf("pending reservations after failed creation = %d, want 0", reserved)
	}

	session, _, err := app.GetOrCreateHLSSession(
		context.Background(),
		movieID,
		helpers.HLS_PROFILE_720P_3MBPS,
		testIntPtr(0),
		testOtherPlaybackSessionID,
		20,
		userID,
	)
	if err != nil {
		t.Fatalf("later creation returned error: %v", err)
	}
	defer cleanupHLSSession(session)
}

func TestGetOrCreateHLSSession_MetadataFailureReleasesReservation(t *testing.T) {
	app := setupTestApp(t)
	defer app.DB.Close()
	app.Settings = &database.Setting{}
	app.HLSMaxPersonalSessionsPerUser = 1
	app.FFmpeg = &fakeFFmpeg{plans: []fakeFFmpegRunPlan{{}}}

	movieID := insertTestHLSMovieFixture(t, app, "h264", 1080)
	userID := int64(100)
	_, _, err := app.GetOrCreateHLSSession(
		context.Background(),
		movieID,
		helpers.HLS_PROFILE_720P_3MBPS,
		testIntPtr(9),
		testPlaybackSessionID,
		0,
		userID,
	)
	if err == nil || !strings.Contains(err.Error(), "out of range") {
		t.Fatalf("metadata validation error = %v, want audio-track range error", err)
	}

	app.PersonalHLSMu.Lock()
	reserved := app.PersonalHLSReservations[userID]
	app.PersonalHLSMu.Unlock()
	if reserved != 0 {
		t.Fatalf("pending reservations after metadata failure = %d, want 0", reserved)
	}

	session, _, err := app.GetOrCreateHLSSession(
		context.Background(),
		movieID,
		helpers.HLS_PROFILE_720P_3MBPS,
		testIntPtr(0),
		testOtherPlaybackSessionID,
		20,
		userID,
	)
	if err != nil {
		t.Fatalf("later creation returned error: %v", err)
	}
	defer cleanupHLSSession(session)
}

func TestGetOrCreateHLSSession_EvictsLRUBeforeStartingReplacement(t *testing.T) {
	app := setupTestApp(t)
	defer app.DB.Close()
	app.Settings = &database.Setting{}
	app.HLSMaxPersonalSessionsPerUser = 1

	started := make(chan struct{}, 1)
	continueStart := make(chan struct{})
	app.FFmpeg = &fakeFFmpeg{plans: []fakeFFmpegRunPlan{{
		Started:  started,
		Continue: continueStart,
	}}}

	movieID := insertTestHLSMovieFixture(t, app, "h264", 1080)
	userID := int64(100)
	oldKey := HLSSessionKey(movieID, helpers.HLS_PROFILE_720P_3MBPS, testIntPtr(0), testOtherPlaybackSessionID, 0)
	otherOwnerKey := HLSSessionKey(movieID, helpers.HLS_PROFILE_720P_3MBPS, testIntPtr(0), "11111111-1111-4111-8111-111111111111", 0)
	roomKey := RoomHLSSessionKey(9)
	app.HLSSessionCache.Set(oldKey, &HLSSession{
		MovieID: movieID, OwnerUserID: userID, PlaybackSession: testOtherPlaybackSessionID,
		TempDir: t.TempDir(), Exited: true,
	}, 2*time.Minute)
	app.HLSSessionCache.Set(otherOwnerKey, &HLSSession{
		MovieID: movieID, OwnerUserID: userID + 1, PlaybackSession: "11111111-1111-4111-8111-111111111111",
		TempDir: t.TempDir(), Exited: true,
	}, time.Minute)
	app.HLSSessionCache.Set(roomKey, &HLSSession{
		MovieID: movieID, IsRoom: true, TempDir: t.TempDir(), Exited: true,
	}, time.Minute)

	type result struct {
		session *HLSSession
		err     error
	}
	resultCh := make(chan result, 1)
	go func() {
		session, _, err := app.GetOrCreateHLSSession(
			context.Background(),
			movieID,
			helpers.HLS_PROFILE_720P_3MBPS,
			testIntPtr(0),
			testPlaybackSessionID,
			20,
			userID,
		)
		resultCh <- result{session: session, err: err}
	}()

	<-started
	_, oldCached := app.HLSSessionCache.Get(oldKey)
	if oldCached {
		t.Fatal("LRU session remained cached when replacement FFmpeg started")
	}
	for _, key := range []string{otherOwnerKey, roomKey} {
		_, cached := app.HLSSessionCache.Get(key)
		if !cached {
			t.Fatalf("unrelated session %q was evicted", key)
		}
	}
	app.PersonalHLSMu.Lock()
	reserved := app.PersonalHLSReservations[userID]
	app.PersonalHLSMu.Unlock()
	if reserved != 1 {
		t.Fatalf("pending reservations while FFmpeg starts = %d, want 1", reserved)
	}

	close(continueStart)
	created := <-resultCh
	if created.err != nil {
		t.Fatalf("replacement creation returned error: %v", created.err)
	}
	defer cleanupHLSSession(created.session)
}

func TestGetOrCreateHLSSession_ReclaimsOwnStaleSessionCapacity(t *testing.T) {
	app := setupTestApp(t)
	defer app.DB.Close()
	app.Settings = &database.Setting{}
	app.FFmpeg = &fakeFFmpeg{plans: []fakeFFmpegRunPlan{{}}}

	// A completed session sorts first but no longer owns a permit. A later idle,
	// still-running session owns the only permit and must be the reclaim victim.
	app.HLSTranscodeLimiter = newHLSTranscodeLimiter(1)
	release, err := app.acquireHLSTranscodeSlot()
	if err != nil {
		t.Fatalf("acquireHLSTranscodeSlot: %v", err)
	}
	var releaseOnce sync.Once
	releaseRunningPermit := func() {
		releaseOnce.Do(release)
	}
	defer releaseRunningPermit()

	userID := int64(100)
	movieID := insertTestHLSMovieFixture(t, app, "h264", 1080)
	completedKey := HLSSessionKey(movieID, helpers.HLS_PROFILE_720P_3MBPS, testIntPtr(0), testOtherPlaybackSessionID, 0)
	runningKey := HLSSessionKey(movieID, helpers.HLS_PROFILE_720P_3MBPS, testIntPtr(0), "11111111-1111-4111-8111-111111111111", 10)
	completedSession := &HLSSession{
		MovieID:         movieID,
		OwnerUserID:     userID,
		PlaybackSession: testOtherPlaybackSessionID,
		TempDir:         t.TempDir(),
		Exited:          true,
	}
	runningSession := &HLSSession{
		MovieID:         movieID,
		OwnerUserID:     userID,
		PlaybackSession: "11111111-1111-4111-8111-111111111111",
		TempDir:         t.TempDir(),
		Cancel:          releaseRunningPermit,
	}
	app.HLSSessionCache.Set(completedKey, completedSession, hlsPersonalSessionTTL-hlsIdlePermitReclaimThreshold-2*time.Second)
	app.HLSSessionCache.Set(runningKey, runningSession, hlsPersonalSessionTTL-hlsIdlePermitReclaimThreshold-time.Second)

	session, _, err := app.GetOrCreateHLSSession(context.Background(), movieID, helpers.HLS_PROFILE_720P_3MBPS, testIntPtr(0), testPlaybackSessionID, 0, userID)
	if err != nil {
		t.Fatalf("GetOrCreateHLSSession returned error: %v", err)
	}
	defer cleanupHLSSession(session)

	_, completedCached := app.HLSSessionCache.Get(completedKey)
	if !completedCached {
		t.Fatal("completed session was removed instead of being skipped")
	}
	_, runningCached := app.HLSSessionCache.Get(runningKey)
	if runningCached {
		t.Fatal("running idle session was not reclaimed before retrying")
	}
	if session.OwnerUserID != userID {
		t.Fatalf("OwnerUserID = %d, want %d", session.OwnerUserID, userID)
	}
}

func TestGetOrCreateHLSSession_DoesNotReclaimActiveSessionOnCapacity(t *testing.T) {
	app := setupTestApp(t)
	defer app.DB.Close()
	app.Settings = &database.Setting{}
	app.FFmpeg = &fakeFFmpeg{plans: []fakeFFmpegRunPlan{{}}}

	// Another device on the same account is actively playing (its TTL is fresh
	// because segment fetches refresh it); a full pool must 503 the newcomer
	// instead of killing the active stream.
	app.HLSTranscodeLimiter = newHLSTranscodeLimiter(1)
	release, err := app.acquireHLSTranscodeSlot()
	if err != nil {
		t.Fatalf("acquireHLSTranscodeSlot: %v", err)
	}
	defer release()

	userID := int64(100)
	movieID := insertTestHLSMovieFixture(t, app, "h264", 1080)
	activeKey := HLSSessionKey(movieID, helpers.HLS_PROFILE_720P_3MBPS, testIntPtr(0), testOtherPlaybackSessionID, 0)
	activeSession := &HLSSession{
		MovieID:         movieID,
		OwnerUserID:     userID,
		PlaybackSession: testOtherPlaybackSessionID,
		TempDir:         t.TempDir(),
		Exited:          true,
	}
	app.HLSSessionCache.Set(activeKey, activeSession, hlsPersonalSessionTTL)

	_, _, err = app.GetOrCreateHLSSession(context.Background(), movieID, helpers.HLS_PROFILE_720P_3MBPS, testIntPtr(0), testPlaybackSessionID, 0, userID)
	var capacityErr *hlsTranscodeCapacityError
	if !errors.As(err, &capacityErr) {
		t.Fatalf("expected hlsTranscodeCapacityError, got %v", err)
	}
	if _, ok := app.HLSSessionCache.Get(activeKey); !ok {
		t.Fatal("expected the active device's session to remain cached")
	}
}

func TestCreateHLSSession_ClampsStartToZeroForTinyDurations(t *testing.T) {
	app := setupTestApp(t)
	defer app.DB.Close()
	app.Settings = &database.Setting{}
	app.FFmpeg = &fakeFFmpeg{plans: []fakeFFmpegRunPlan{{}}}

	ctx := context.Background()
	res, err := app.DB.Exec(`
		INSERT INTO movies (title, file_path, file_name, size, container, mime_type, adult, duration)
		VALUES ('Tiny', '/tmp/tiny.mkv', 'tiny.mkv', 1, 'mkv', 'video/x-matroska', 0, 3.0)
	`)
	if err != nil {
		t.Fatalf("insert movie: %v", err)
	}
	movieID, err := res.LastInsertId()
	if err != nil {
		t.Fatalf("last insert id: %v", err)
	}
	_, err = app.DB.Exec(`
		INSERT INTO video_streams (movie_id, stream_index, codec, bit_rate, width, height, frame_rate)
		VALUES (?, 0, 'h264', 5000000, 1920, 1080, 23.976)
	`, movieID)
	if err != nil {
		t.Fatalf("insert video stream: %v", err)
	}

	session, err := createTestHLSSession(app, ctx, movieID, "720p_3mbps", nil, testPlaybackSessionID, 10, false)
	if err != nil {
		t.Fatalf("createHLSSession returned error: %v", err)
	}
	defer cleanupHLSSession(session)

	if session.StartSec != 0 {
		t.Fatalf("StartSec = %v, want 0", session.StartSec)
	}
}

func TestGetOrCreateHLSSession_EffectiveStartControlsKeyAndFFmpeg(t *testing.T) {
	tests := []struct {
		name           string
		durationSec    float64
		requestedStart int
		wantStart      int
	}{
		{
			name:           "duration tail",
			durationSec:    7200,
			requestedStart: 9000,
			wantStart:      7195,
		},
		{
			name:           "tiny duration",
			durationSec:    3,
			requestedStart: 10,
			wantStart:      0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			app := setupTestApp(t)
			defer app.DB.Close()
			app.Settings = &database.Setting{}
			fake := &fakeFFmpeg{plans: []fakeFFmpegRunPlan{{}}}
			app.FFmpeg = fake

			movieID := insertTestHLSMovieFixture(t, app, "h264", 1080)
			_, err := app.DB.Exec(`UPDATE movies SET duration = ? WHERE id = ?`, tt.durationSec, movieID)
			if err != nil {
				t.Fatalf("update duration: %v", err)
			}

			userID := int64(100)
			session, key, err := app.GetOrCreateHLSSession(
				context.Background(),
				movieID,
				helpers.HLS_PROFILE_720P_3MBPS,
				testIntPtr(0),
				testPlaybackSessionID,
				tt.requestedStart,
				userID,
			)
			if err != nil {
				t.Fatalf("GetOrCreateHLSSession returned error: %v", err)
			}
			defer cleanupHLSSession(session)

			wantKey := HLSSessionKey(
				movieID,
				helpers.HLS_PROFILE_720P_3MBPS,
				testIntPtr(0),
				testPlaybackSessionID,
				tt.wantStart,
			)
			if key != wantKey {
				t.Fatalf("session key = %q, want %q", key, wantKey)
			}
			if session.StartSec != float64(tt.wantStart) {
				t.Fatalf("session StartSec = %v, want %d", session.StartSec, tt.wantStart)
			}
			calls := fake.Calls()
			if len(calls) != 1 {
				t.Fatalf("RunHLS call count = %d, want 1", len(calls))
			}
			if calls[0].StartSec != float64(tt.wantStart) {
				t.Fatalf("RunHLS StartSec = %v, want %d", calls[0].StartSec, tt.wantStart)
			}
			_, cached := app.HLSSessionCache.Get(wantKey)
			if !cached {
				t.Fatalf("effective session key %q was not cached", wantKey)
			}
		})
	}
}

// The audio_track parameter is an ordinal into the movie's audio streams while
// ffmpeg's -map needs the absolute ffprobe index. Pin the translation with a
// fixture where the two deliberately disagree.
func TestCreateHLSSession_AudioTrackOrdinalMapsToAbsoluteStreamIndex(t *testing.T) {
	app := setupTestApp(t)
	defer app.DB.Close()
	app.Settings = &database.Setting{}

	fake := &fakeFFmpeg{
		plans: []fakeFFmpegRunPlan{
			hlsRunPlan(transcodeFixture),
		},
	}
	app.FFmpeg = fake

	movieID := insertTestHLSMovieFixture(t, app, "h264", 1080)
	// The fixture holds one audio stream at absolute index 1; adding a Spanish
	// stream at absolute index 3 makes ordinal 1 resolve to index 3.
	_, err := app.DB.Exec(`
		INSERT INTO audio_streams (movie_id, stream_index, codec, bit_rate, channels, language)
		VALUES (?, ?, ?, ?, ?, ?)
	`, movieID, 3, "ac3", 448000, 6, "spa")
	if err != nil {
		t.Fatalf("insert second audio stream: %v", err)
	}

	session, err := createTestHLSSession(app, context.Background(), movieID, helpers.HLS_PROFILE_720P_3MBPS, testIntPtr(1), testPlaybackSessionID, 0, false)
	if err != nil {
		t.Fatalf("createHLSSession returned error: %v", err)
	}
	defer cleanupHLSSession(session)

	calls := fake.Calls()
	if len(calls) != 1 {
		t.Fatalf("expected 1 RunHLS call, got %d", len(calls))
	}
	if calls[0].AudioStreamIndex != 3 {
		t.Fatalf("AudioStreamIndex = %d, want 3 (absolute ffprobe index for ordinal 1)", calls[0].AudioStreamIndex)
	}
	// ac3 is not AAC, so it must be transcoded rather than copied.
	if calls[0].CopyAudio {
		t.Fatal("CopyAudio = true, want false for an ac3 source track")
	}
}

func TestCreateHLSSession_AudioTrackBeyondStreamCountRejected(t *testing.T) {
	app := setupTestApp(t)
	defer app.DB.Close()
	app.Settings = &database.Setting{}
	app.FFmpeg = &fakeFFmpeg{}

	movieID := insertTestHLSMovieFixture(t, app, "h264", 1080)

	_, err := createTestHLSSession(app, context.Background(), movieID, helpers.HLS_PROFILE_720P_3MBPS, testIntPtr(1), testPlaybackSessionID, 0, false)
	if err == nil {
		t.Fatal("expected an out-of-range audio track to fail")
	}
	if !strings.Contains(err.Error(), "out of range") {
		t.Fatalf("expected an out-of-range error, got %v", err)
	}
}
