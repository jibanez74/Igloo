package main

import (
	"context"
	"errors"
	"os"
	"sync"
	"testing"
	"time"

	"igloo/cmd/internal/helpers"
)

func TestCleanupPersonalHLSSessionsForOwner_KeepsCurrentWindow(t *testing.T) {
	app := setupTestApp(t)
	defer app.DB.Close()

	userID := int64(100)
	audioTrack := 0
	oldKey := HLSSessionKey(5, helpers.HLS_PROFILE_REMUX, &audioTrack, nil, testPlaybackSessionID, 0, userID)
	keepKey := HLSSessionKey(5, helpers.HLS_PROFILE_REMUX, &audioTrack, nil, testPlaybackSessionID, 40, userID)
	otherKey := HLSSessionKey(5, helpers.HLS_PROFILE_REMUX, &audioTrack, nil, testPlaybackSessionID, 80, userID+1)

	app.HLSSessionCache.SetDefault(oldKey, &HLSSession{MovieID: 5, OwnerUserID: userID, PlaybackSession: testPlaybackSessionID, TempDir: t.TempDir()})
	app.HLSSessionCache.SetDefault(keepKey, &HLSSession{MovieID: 5, OwnerUserID: userID, PlaybackSession: testPlaybackSessionID, TempDir: t.TempDir()})
	app.HLSSessionCache.SetDefault(otherKey, &HLSSession{MovieID: 5, OwnerUserID: userID + 1, PlaybackSession: testPlaybackSessionID, TempDir: t.TempDir()})

	removed := app.cleanupPersonalHLSSessionsForOwner(5, userID, testPlaybackSessionID, keepKey)
	if removed != 1 {
		t.Fatalf("removed=%d, want 1", removed)
	}
	if _, ok := app.HLSSessionCache.Get(oldKey); ok {
		t.Fatal("expected old same-owner window to be removed")
	}
	for _, key := range []string{keepKey, otherKey} {
		if _, ok := app.HLSSessionCache.Get(key); !ok {
			t.Fatalf("expected session %q to remain", key)
		}
	}
}

func TestCleanupPersonalHLSSessionsForOwner_ReleasesLockBeforeTeardown(t *testing.T) {
	app := setupTestApp(t)
	defer app.DB.Close()

	userID := int64(100)
	audioTrack := 0
	key := HLSSessionKey(5, helpers.HLS_PROFILE_REMUX, &audioTrack, nil, testPlaybackSessionID, 0, userID)
	session := &HLSSession{
		MovieID:         5,
		OwnerUserID:     userID,
		PlaybackSession: testPlaybackSessionID,
		TempDir:         t.TempDir(),
	}
	cleanupStarted, releaseCleanup := blockHLSSessionCleanup(t, session)
	app.HLSSessionCache.Set(key, session, hlsPersonalSessionTTL)
	resultCh := make(chan int, 1)
	go func() {
		resultCh <- app.cleanupPersonalHLSSessionsForOwner(5, userID, testPlaybackSessionID, "")
	}()

	waitForHLSSessionCleanupToBlock(t, cleanupStarted, releaseCleanup)
	_, cached := app.HLSSessionCache.Get(key)
	if cached {
		releaseCleanup()
		t.Fatal("personal session remained cached while teardown was blocked")
	}
	if !app.PersonalHLSMu.TryLock() {
		releaseCleanup()
		t.Fatal("PersonalHLSMu remained locked while explicit teardown was blocked")
	}
	app.PersonalHLSMu.Unlock()

	releaseCleanup()
	removed := waitForHLSSessionCleanupResult(t, resultCh)
	if removed != 1 {
		t.Fatalf("removed = %d, want 1", removed)
	}
	_, err := os.Stat(session.TempDir)
	if !os.IsNotExist(err) {
		t.Fatalf("personal session temp dir still exists after cleanup: %v", err)
	}
}

func TestRefreshHLSSessionTTL_PersonalAndRoomTTLs(t *testing.T) {
	app := setupTestApp(t)
	defer app.DB.Close()

	audioTrack := 0
	personalKey := HLSSessionKey(5, helpers.HLS_PROFILE_REMUX, &audioTrack, nil, testPlaybackSessionID, 0, 100)
	roomKey := RoomHLSSessionKey(9)
	personalSession := &HLSSession{MovieID: 5, OwnerUserID: 100, PlaybackSession: testPlaybackSessionID}

	before := time.Now()
	app.HLSSessionCache.Set(personalKey, personalSession, time.Minute)
	app.RefreshHLSSessionTTL(personalKey, personalSession)
	app.RefreshHLSSessionTTL(roomKey, &HLSSession{MovieID: 5, IsRoom: true})
	after := time.Now()

	items := app.HLSSessionCache.Items()
	checks := []struct {
		key string
		ttl time.Duration
	}{
		{personalKey, hlsPersonalSessionTTL},
		{roomKey, hlsRoomSessionTTL},
	}
	for _, check := range checks {
		item, ok := items[check.key]
		if !ok {
			t.Fatalf("expected session %q to be cached", check.key)
		}
		expiration := time.Unix(0, item.Expiration)
		expiresTooEarly := expiration.Before(before.Add(check.ttl))
		expiresTooLate := expiration.After(after.Add(check.ttl))
		if expiresTooEarly || expiresTooLate {
			t.Fatalf("session %q expires at %v, want ~%v after refresh", check.key, expiration, check.ttl)
		}
	}
}

func TestRefreshHLSSessionTTL_DoesNotReinsertEvictedPersonalSession(t *testing.T) {
	app := setupTestApp(t)
	defer app.DB.Close()

	audioTrack := 0
	key := HLSSessionKey(5, helpers.HLS_PROFILE_REMUX, &audioTrack, nil, testPlaybackSessionID, 0, 100)
	session := &HLSSession{
		MovieID: 5, OwnerUserID: 100, PlaybackSession: testPlaybackSessionID,
	}
	app.HLSSessionCache.Set(key, session, time.Minute)
	app.removePersonalHLSSession(key)

	refreshed := app.RefreshHLSSessionTTL(key, session)
	if refreshed {
		t.Fatal("evicted personal session was refreshed")
	}
	_, cached := app.HLSSessionCache.Get(key)
	if cached {
		t.Fatal("evicted personal session was reinserted")
	}
}

func TestGetOrCreateHLSSession_ReservationsCapConcurrentRemuxStarts(t *testing.T) {
	app := setupTestApp(t)
	defer app.DB.Close()
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
				nil,
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
			nil,
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
	oldKey := HLSSessionKey(movieID, helpers.HLS_PROFILE_720P_3MBPS, nil, nil, testOtherPlaybackSessionID, 0, userID)
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
	oldKey := HLSSessionKey(movieID, helpers.HLS_PROFILE_720P_3MBPS, nil, nil, testPlaybackSessionID, 0, userID)
	newKey := HLSSessionKey(movieID, helpers.HLS_PROFILE_720P_3MBPS, nil, nil, testPlaybackSessionID, 30, userID)
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
	key := HLSSessionKey(movieID, helpers.HLS_PROFILE_720P_3MBPS, nil, nil, testOtherPlaybackSessionID, 0, userID)
	session := &HLSSession{
		MovieID:               movieID,
		OwnerUserID:           userID,
		PlaybackSession:       testOtherPlaybackSessionID,
		TempDir:               t.TempDir(),
		RequiresTranscodeSlot: true,
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
		nil,
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
		nil,
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
	app.HLSMaxPersonalSessionsPerUser = 1

	started := make(chan struct{}, 1)
	continueStart := make(chan struct{})
	app.FFmpeg = &fakeFFmpeg{plans: []fakeFFmpegRunPlan{{
		Started:  started,
		Continue: continueStart,
	}}}

	movieID := insertTestHLSMovieFixture(t, app, "h264", 1080)
	userID := int64(100)
	oldKey := HLSSessionKey(movieID, helpers.HLS_PROFILE_720P_3MBPS, testIntPtr(0), nil, testOtherPlaybackSessionID, 0, userID)
	otherOwnerKey := HLSSessionKey(movieID, helpers.HLS_PROFILE_720P_3MBPS, testIntPtr(0), nil, "11111111-1111-4111-8111-111111111111", 0, userID+1)
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
			nil,
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
	app.FFmpeg = &fakeFFmpeg{plans: []fakeFFmpegRunPlan{{}}}

	// A completed session sorts first but no longer owns a permit. A later idle,
	// still-running session owns the only permit and must be the reclaim victim.
	app.HLSTranscodeLimiter = newHLSTranscodeLimiter(1)
	release, err := app.acquireHLSTranscodeSlot(context.Background(), 0)
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
	completedKey := HLSSessionKey(movieID, helpers.HLS_PROFILE_720P_3MBPS, testIntPtr(0), nil, testOtherPlaybackSessionID, 0, userID)
	runningKey := HLSSessionKey(movieID, helpers.HLS_PROFILE_720P_3MBPS, testIntPtr(0), nil, "11111111-1111-4111-8111-111111111111", 10, userID)
	completedSession := &HLSSession{
		MovieID:         movieID,
		OwnerUserID:     userID,
		PlaybackSession: testOtherPlaybackSessionID,
		TempDir:         t.TempDir(),
		Exited:          true,
	}
	runningSession := &HLSSession{
		MovieID:               movieID,
		OwnerUserID:           userID,
		PlaybackSession:       "11111111-1111-4111-8111-111111111111",
		TempDir:               t.TempDir(),
		Cancel:                releaseRunningPermit,
		RequiresTranscodeSlot: true,
	}
	app.HLSSessionCache.Set(completedKey, completedSession, hlsPersonalSessionTTL-hlsIdlePermitReclaimThreshold-2*time.Second)
	app.HLSSessionCache.Set(runningKey, runningSession, hlsPersonalSessionTTL-hlsIdlePermitReclaimThreshold-time.Second)

	session, _, err := app.GetOrCreateHLSSession(context.Background(), movieID, helpers.HLS_PROFILE_720P_3MBPS, testIntPtr(0), nil, testPlaybackSessionID, 0, userID)
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
	app.FFmpeg = &fakeFFmpeg{plans: []fakeFFmpegRunPlan{{}}}

	// Another device on the same account is actively playing (its TTL is fresh
	// because segment fetches refresh it); a full pool must wait out its budget
	// and then 503 the newcomer instead of killing the active stream.
	withTestHLSTranscodeAcquireWait(t, 50*time.Millisecond)

	app.HLSTranscodeLimiter = newHLSTranscodeLimiter(1)
	release, err := app.acquireHLSTranscodeSlot(context.Background(), 0)
	if err != nil {
		t.Fatalf("acquireHLSTranscodeSlot: %v", err)
	}
	defer release()

	userID := int64(100)
	movieID := insertTestHLSMovieFixture(t, app, "h264", 1080)
	activeKey := HLSSessionKey(movieID, helpers.HLS_PROFILE_720P_3MBPS, testIntPtr(0), nil, testOtherPlaybackSessionID, 0, userID)
	activeSession := &HLSSession{
		MovieID:         movieID,
		OwnerUserID:     userID,
		PlaybackSession: testOtherPlaybackSessionID,
		TempDir:         t.TempDir(),
		Exited:          true,
	}
	app.HLSSessionCache.Set(activeKey, activeSession, hlsPersonalSessionTTL)

	_, _, err = app.GetOrCreateHLSSession(context.Background(), movieID, helpers.HLS_PROFILE_720P_3MBPS, testIntPtr(0), nil, testPlaybackSessionID, 0, userID)
	var capacityErr *hlsTranscodeCapacityError
	if !errors.As(err, &capacityErr) {
		t.Fatalf("expected hlsTranscodeCapacityError, got %v", err)
	}
	if _, ok := app.HLSSessionCache.Get(activeKey); !ok {
		t.Fatal("expected the active device's session to remain cached")
	}
}

// The starvation this replaces: with fast segment serving, running sessions
// never go idle long enough for reclaim to free a permit, so an instant refusal
// meant a queued stream never started at all. It must wait its turn instead.
func TestGetOrCreateHLSSession_WaitsForAPermitInsteadOfRefusing(t *testing.T) {
	app := setupTestApp(t)
	defer app.DB.Close()
	app.FFmpeg = &fakeFFmpeg{plans: []fakeFFmpegRunPlan{{}}}

	withTestHLSTranscodeAcquireWait(t, 10*time.Second)

	// The pool is full and nothing is reclaimable: the held permit belongs to a
	// session that is genuinely playing, which is precisely the case reclaim
	// cannot resolve.
	app.HLSTranscodeLimiter = newHLSTranscodeLimiter(1)
	release, err := app.acquireHLSTranscodeSlot(context.Background(), 0)
	if err != nil {
		t.Fatalf("acquireHLSTranscodeSlot: %v", err)
	}

	userID := int64(100)
	movieID := insertTestHLSMovieFixture(t, app, "h264", 1080)
	activeKey := HLSSessionKey(movieID, helpers.HLS_PROFILE_720P_3MBPS, testIntPtr(0), nil, testOtherPlaybackSessionID, 0, userID)
	app.HLSSessionCache.Set(activeKey, &HLSSession{
		MovieID:         movieID,
		OwnerUserID:     userID,
		PlaybackSession: testOtherPlaybackSessionID,
		TempDir:         t.TempDir(),
		Exited:          true,
	}, hlsPersonalSessionTTL)

	type createResult struct {
		session *HLSSession
		err     error
	}
	resultCh := make(chan createResult, 1)
	go func() {
		session, _, createErr := app.GetOrCreateHLSSession(
			context.Background(), movieID, helpers.HLS_PROFILE_720P_3MBPS,
			testIntPtr(0), nil, testPlaybackSessionID, 0, userID,
		)
		resultCh <- createResult{session: session, err: createErr}
	}()

	// Nothing may be admitted while the permit is held.
	select {
	case created := <-resultCh:
		t.Fatalf("session resolved before a permit freed: session=%v err=%v", created.session, created.err)
	case <-time.After(100 * time.Millisecond):
	}

	release()

	select {
	case created := <-resultCh:
		if created.err != nil {
			t.Fatalf("GetOrCreateHLSSession after the permit freed: %v", created.err)
		}
		defer cleanupHLSSession(created.session)
	case <-time.After(10 * time.Second):
		t.Fatal("GetOrCreateHLSSession never resolved after a permit freed")
	}

	if _, ok := app.HLSSessionCache.Get(activeKey); !ok {
		t.Fatal("the active device's session was reclaimed instead of waited out")
	}
}

// Reclaim exists to free a transcode permit. A copy-video session never held
// one, so killing it would interrupt playback and buy nothing.
func TestReclaimIdlePersonalHLSSession_SkipsCopyVideoSessions(t *testing.T) {
	app := setupTestApp(t)
	defer app.DB.Close()

	userID := int64(100)
	key := HLSSessionKey(5, helpers.HLS_PROFILE_REMUX, testIntPtr(0), nil, testOtherPlaybackSessionID, 0, userID)
	session := &HLSSession{
		MovieID:               5,
		OwnerUserID:           userID,
		PlaybackSession:       testOtherPlaybackSessionID,
		TempDir:               t.TempDir(),
		CopyVideo:             true,
		RequiresTranscodeSlot: false,
	}
	app.HLSSessionCache.Set(key, session, hlsPersonalSessionTTL-hlsIdlePermitReclaimThreshold-time.Second)

	if app.reclaimIdlePersonalHLSSessionForOwner(userID) {
		t.Fatal("an idle copy-video session was reclaimed")
	}
	if _, cached := app.HLSSessionCache.Get(key); !cached {
		t.Fatal("expected the copy-video session to stay cached")
	}
}

func TestReclaimIdlePersonalHLSSession_ReclaimsCopyVideoAudioEncode(t *testing.T) {
	app := setupTestApp(t)
	defer app.DB.Close()

	userID := int64(100)
	key := HLSSessionKey(5, helpers.HLS_PROFILE_REMUX, testIntPtr(0), nil, testOtherPlaybackSessionID, 0, userID)
	canceled := make(chan struct{})
	session := &HLSSession{
		MovieID:               5,
		OwnerUserID:           userID,
		PlaybackSession:       testOtherPlaybackSessionID,
		TempDir:               t.TempDir(),
		CopyVideo:             true,
		RequiresTranscodeSlot: true,
		Cancel:                func() { close(canceled) },
	}
	app.HLSSessionCache.Set(key, session, hlsPersonalSessionTTL-hlsIdlePermitReclaimThreshold-time.Second)

	if !app.reclaimIdlePersonalHLSSessionForOwner(userID) {
		t.Fatal("idle copy-video session that encodes audio was not reclaimed")
	}
	if _, cached := app.HLSSessionCache.Get(key); cached {
		t.Fatal("reclaimed audio-encoding remux session remained cached")
	}
	select {
	case <-canceled:
	default:
		t.Fatal("reclaim did not stop the audio-encoding remux session")
	}
}

// hlsMaxPersonalSessionsPerUser is the per-user cap the session cache enforces.
// An unconfigured server must fall back to the default rather than to zero,
// which would refuse every personal playback session.
func TestHLSMaxPersonalSessionsPerUser(t *testing.T) {
	app := &Application{}

	app.HLSMaxPersonalSessionsPerUser = 0
	if got := app.hlsMaxPersonalSessionsPerUser(); got != hlsMaxPersonalSessionsPerUserDefault {
		t.Fatalf("hlsMaxPersonalSessionsPerUser() = %d, want the default %d", got, hlsMaxPersonalSessionsPerUserDefault)
	}

	app.HLSMaxPersonalSessionsPerUser = 7
	if got := app.hlsMaxPersonalSessionsPerUser(); got != 7 {
		t.Fatalf("hlsMaxPersonalSessionsPerUser() = %d, want the configured 7", got)
	}
}

func TestDeleteHLSSession(t *testing.T) {
	app := setupTestApp(t)
	defer app.DB.Close()

	t.Run("returns nil for a key that is not cached", func(t *testing.T) {
		if session := app.deleteHLSSession("missing"); session != nil {
			t.Fatalf("deleteHLSSession() = %v, want nil", session)
		}
	})

	t.Run("removes an entry that is not a session", func(t *testing.T) {
		app.HLSSessionCache.SetDefault("poisoned", "not a session")

		if session := app.deleteHLSSession("poisoned"); session != nil {
			t.Fatalf("deleteHLSSession() = %v, want nil", session)
		}
		if _, cached := app.HLSSessionCache.Get("poisoned"); cached {
			t.Fatal("expected the unusable entry to be removed")
		}
	})

	t.Run("returns and removes a cached session", func(t *testing.T) {
		session := &HLSSession{MovieID: 5, TempDir: t.TempDir()}
		app.HLSSessionCache.SetDefault("live", session)

		if got := app.deleteHLSSession("live"); got != session {
			t.Fatalf("deleteHLSSession() = %v, want the cached session", got)
		}
		if _, cached := app.HLSSessionCache.Get("live"); cached {
			t.Fatal("expected the session to be removed from the cache")
		}
	})
}

func TestGetOrCreateHLSSession_WarmPathNeedsNoDatabase(t *testing.T) {
	app := setupTestApp(t)
	defer app.DB.Close()
	fake := &fakeFFmpeg{plans: []fakeFFmpegRunPlan{hlsRunPlan(safeRemuxFixture)}}
	app.FFmpeg = fake

	movieID := insertTestHLSMovieFixture(t, app, "h264", 1080)
	userID := int64(100)

	created, key, err := app.GetOrCreateHLSSession(
		context.Background(),
		movieID,
		helpers.HLS_PROFILE_REMUX,
		testIntPtr(0),
		nil,
		testPlaybackSessionID,
		0,
		userID,
	)
	if err != nil {
		t.Fatalf("initial creation error: %v", err)
	}
	defer cleanupHLSSession(created)

	// Deleting the movie row proves the warm path below never consults the
	// database: an unclamped re-request must be served entirely from the
	// session cache.
	_, err = app.DB.Exec("DELETE FROM movies WHERE id = ?", movieID)
	if err != nil {
		t.Fatalf("delete movie row: %v", err)
	}

	cached, cachedKey, err := app.GetOrCreateHLSSession(
		context.Background(),
		movieID,
		helpers.HLS_PROFILE_REMUX,
		testIntPtr(0),
		nil,
		testPlaybackSessionID,
		0,
		userID,
	)
	if err != nil {
		t.Fatalf("warm re-request error: %v", err)
	}
	if cached != created {
		t.Fatal("warm re-request did not return the cached session")
	}
	if cachedKey != key {
		t.Fatalf("warm re-request key = %q, want %q", cachedKey, key)
	}
	if fake.CallCount() != 1 {
		t.Fatalf("RunHLS call count = %d, want 1", fake.CallCount())
	}
}

func TestGetOrCreateHLSSession_SingleflightSharesIdenticalOwnerRequest(t *testing.T) {
	app := setupTestApp(t)
	defer app.DB.Close()

	started := make(chan struct{}, 1)
	continueStart := make(chan struct{})
	plan := hlsRunPlan(safeRemuxFixture)
	plan.Started = started
	plan.Continue = continueStart
	fake := &fakeFFmpeg{plans: []fakeFFmpegRunPlan{plan}}
	app.FFmpeg = fake

	movieID := insertTestHLSMovieFixture(t, app, "h264", 1080)
	userID := int64(100)
	type result struct {
		session *HLSSession
		key     string
		err     error
	}
	results := make(chan result, 2)
	create := func() {
		session, key, err := app.GetOrCreateHLSSession(
			context.Background(), movieID, helpers.HLS_PROFILE_REMUX,
			testIntPtr(0), nil, testPlaybackSessionID, 0, userID,
		)
		results <- result{session: session, key: key, err: err}
	}
	go create()
	<-started
	go create()

	time.Sleep(50 * time.Millisecond)
	if fake.CallCount() != 1 {
		close(continueStart)
		t.Fatalf("RunHLS call count while identical requests are in flight = %d, want 1", fake.CallCount())
	}
	close(continueStart)

	first := <-results
	second := <-results
	if first.err != nil || second.err != nil {
		t.Fatalf("creation errors = %v, %v", first.err, second.err)
	}
	defer cleanupHLSSession(first.session)
	if first.session != second.session {
		t.Fatal("identical requests from one owner did not share a session")
	}
	if first.key != second.key {
		t.Fatalf("singleflight keys differ: %q and %q", first.key, second.key)
	}
	if first.session.OwnerUserID != userID {
		t.Fatalf("OwnerUserID = %d, want %d", first.session.OwnerUserID, userID)
	}
}

func TestGetOrCreateHLSSession_SingleflightIsolatedByOwner(t *testing.T) {
	app := setupTestApp(t)
	defer app.DB.Close()

	started := make(chan struct{}, 2)
	continueStarts := make(chan struct{})
	plan := hlsRunPlan(safeRemuxFixture)
	plan.Started = started
	plan.Continue = continueStarts
	fake := &fakeFFmpeg{plans: []fakeFFmpegRunPlan{plan, plan}}
	app.FFmpeg = fake

	movieID := insertTestHLSMovieFixture(t, app, "h264", 1080)
	userIDs := []int64{100, 200}
	type result struct {
		userID  int64
		session *HLSSession
		key     string
		err     error
	}
	results := make(chan result, len(userIDs))
	for _, userID := range userIDs {
		go func() {
			session, key, err := app.GetOrCreateHLSSession(
				context.Background(), movieID, helpers.HLS_PROFILE_REMUX,
				testIntPtr(0), nil, testPlaybackSessionID, 0, userID,
			)
			results <- result{userID: userID, session: session, key: key, err: err}
		}()
	}
	<-started
	<-started
	close(continueStarts)

	created := make(map[int64]result, len(userIDs))
	for range userIDs {
		result := <-results
		if result.err != nil {
			t.Fatalf("owner %d creation error: %v", result.userID, result.err)
		}
		created[result.userID] = result
		defer cleanupHLSSession(result.session)
	}

	first := created[userIDs[0]]
	second := created[userIDs[1]]
	if first.key == second.key {
		t.Fatalf("different owners shared key %q", first.key)
	}
	if first.session == second.session {
		t.Fatal("different owners shared an HLS session")
	}
	if fake.CallCount() != 2 {
		t.Fatalf("RunHLS call count = %d, want 2", fake.CallCount())
	}
	for _, result := range created {
		if result.session.OwnerUserID != result.userID {
			t.Fatalf("session owner = %d, want %d", result.session.OwnerUserID, result.userID)
		}
		raw, cached := app.HLSSessionCache.Get(result.key)
		if !cached || raw != result.session {
			t.Fatalf("owner %d session was not cached under %q", result.userID, result.key)
		}
	}
}

// Legacy and explicit audio modes are isolated sessions: a legacy request can
// never reuse an explicit session's segments (or vice versa), identical
// normalized explicit requests still share one singleflight session, and
// switching profiles inside one playback_session follows the existing
// superseded-window cleanup.
func TestGetOrCreateHLSSession_AudioProfileIsolation(t *testing.T) {
	app := setupTestApp(t)
	defer app.DB.Close()
	fake := &fakeFFmpeg{plans: []fakeFFmpegRunPlan{
		hlsRunPlan(transcodeFixture),
		hlsRunPlan(transcodeFixture),
		hlsRunPlan(transcodeFixture),
	}}
	app.FFmpeg = fake

	movieID := insertTestHLSMovieFixture(t, app, "h264", 1080)
	setTestHLSAudioStream(t, app, movieID, "aac", "LC", 6, "5.1(side)")
	userID := int64(100)
	eac3Surround := &helpers.HLSAudioProfileRequest{Codec: helpers.HLSAudioCodecEAC3, MaxChannels: 6}

	legacySession, legacyKey, err := app.GetOrCreateHLSSession(
		context.Background(), movieID, helpers.HLS_PROFILE_720P_3MBPS,
		testIntPtr(0), nil, testPlaybackSessionID, 0, userID,
	)
	if err != nil {
		t.Fatalf("legacy creation error: %v", err)
	}
	defer cleanupHLSSession(legacySession)

	// Switching to an explicit profile in the same playback session starts a
	// new FFmpeg process and supersedes the legacy window.
	explicitSession, explicitKey, err := app.GetOrCreateHLSSession(
		context.Background(), movieID, helpers.HLS_PROFILE_720P_3MBPS,
		testIntPtr(0), eac3Surround, testPlaybackSessionID, 0, userID,
	)
	if err != nil {
		t.Fatalf("explicit creation error: %v", err)
	}
	defer cleanupHLSSession(explicitSession)

	if explicitKey == legacyKey {
		t.Fatalf("explicit key %q must differ from legacy key %q", explicitKey, legacyKey)
	}
	if fake.CallCount() != 2 {
		t.Fatalf("RunHLS call count after profile switch = %d, want 2", fake.CallCount())
	}
	if _, cached := app.HLSSessionCache.Get(legacyKey); cached {
		t.Fatal("legacy session survived an audio profile switch in the same playback session")
	}

	// An identical normalized explicit request reuses the cached session.
	reused, reusedKey, err := app.GetOrCreateHLSSession(
		context.Background(), movieID, helpers.HLS_PROFILE_720P_3MBPS,
		testIntPtr(0), &helpers.HLSAudioProfileRequest{Codec: helpers.HLSAudioCodecEAC3, MaxChannels: 6},
		testPlaybackSessionID, 0, userID,
	)
	if err != nil {
		t.Fatalf("explicit re-request error: %v", err)
	}
	if reused != explicitSession || reusedKey != explicitKey {
		t.Fatal("identical explicit request did not reuse the cached session")
	}
	if fake.CallCount() != 2 {
		t.Fatalf("RunHLS call count after re-request = %d, want 2", fake.CallCount())
	}

	// A different playback session's explicit window is isolated: creating it
	// must not touch this one.
	otherSession, _, err := app.GetOrCreateHLSSession(
		context.Background(), movieID, helpers.HLS_PROFILE_720P_3MBPS,
		testIntPtr(0), eac3Surround, testOtherPlaybackSessionID, 0, userID,
	)
	if err != nil {
		t.Fatalf("other playback session creation error: %v", err)
	}
	defer cleanupHLSSession(otherSession)

	if _, cached := app.HLSSessionCache.Get(explicitKey); !cached {
		t.Fatal("explicit session was removed by a different playback session's creation")
	}
}
