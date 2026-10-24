package main

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"igloo/cmd/internal/helpers"
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
		testIntPtr(0), nil, testPlaybackSessionID, 0, 100,
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
			testIntPtr(0), nil, testPlaybackSessionID, 0, 100,
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
			testIntPtr(0), nil, testPlaybackSessionID, 0, 100,
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
