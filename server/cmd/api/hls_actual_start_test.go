package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"igloo/cmd/internal/ffmpeg"
	"igloo/cmd/internal/helpers"
)

// returnSeekLanding answers every seek measurement with landing.
func returnSeekLanding(landing float64) func(context.Context, ffmpeg.SeekLandingParams) (float64, error) {
	return func(context.Context, ffmpeg.SeekLandingParams) (float64, error) {
		return landing, nil
	}
}

// awaitMeasurement waits for a session's start measurement to finish.
func awaitMeasurement(t *testing.T, session *HLSSession) {
	t.Helper()
	if session.actualStartReady == nil {
		t.Fatal("session has no start measurement")
	}
	select {
	case <-session.actualStartReady:
	case <-time.After(5 * time.Second):
		t.Fatal("start measurement did not finish")
	}
}

func TestStartHLSSession_ActualStartOfCopyVideo(t *testing.T) {
	tests := []struct {
		name        string
		seekLanding func(context.Context, ffmpeg.SeekLandingParams) (float64, error)
		want        float64
	}{
		{
			// Captain Marvel at 1790: the Matroska seek aimed 3/23 s early and
			// landed a whole cue before the 1789.997 keyframe.
			name:        "the measured landing",
			seekLanding: returnSeekLanding(1780.195),
			want:        1780.195,
		},
		{
			// An MP4 seek by decode time can land a few frames past the start;
			// the header promises at or before it.
			name:        "a landing past the start reports the start",
			seekLanding: returnSeekLanding(1790.04),
			want:        1790,
		},
		{
			name: "a failed measurement leaves the start unknown",
			seekLanding: func(context.Context, ffmpeg.SeekLandingParams) (float64, error) {
				return 0, errors.New("ffmpeg seek measurement failed")
			},
			want: hlsUnknownActualStart,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			app := setupTestApp(t)
			fake := &fakeFFmpeg{
				plans:           []fakeFFmpegRunPlan{hlsRunPlan(safeRemuxFixture)},
				seekLandingFunc: tt.seekLanding,
			}
			app.FFmpeg = fake
			movieID := insertTestHLSMovieFixture(t, app, "h264", 1080)

			session, err := createTestHLSSession(app, context.Background(), movieID, helpers.HLS_PROFILE_REMUX, testIntPtr(0), testPlaybackSessionID, 1790, false)
			if err != nil {
				t.Fatalf("createHLSSession: %v", err)
			}
			defer cleanupHLSSession(session)

			awaitMeasurement(t, session)
			if got := session.actualStartSec(); got != tt.want {
				t.Fatalf("actual start = %v, want %v", got, tt.want)
			}

			calls := fake.SeekLandingCalls()
			if len(calls) != 1 {
				t.Fatalf("SeekLanding calls = %d, want 1", len(calls))
			}
			runs := fake.Calls()
			want := ffmpeg.SeekLandingParams{
				SourcePath:       runs[0].SourcePath,
				VideoStreamIndex: runs[0].VideoStreamIndex,
				StartSec:         runs[0].StartSec,
			}
			if calls[0] != want {
				t.Fatalf("SeekLanding params = %+v, want the session's own seek %+v", calls[0], want)
			}
		})
	}
}

// Only stream copy can land away from the requested start: a transcode seeks
// accurately and a start of 0 is exact, so neither spends a measurement.
func TestStartHLSSession_ExactStartsNeedNoMeasurement(t *testing.T) {
	tests := []struct {
		name     string
		profile  string
		plan     fakeFFmpegRunPlan
		startSec int
	}{
		{name: "copy video from zero", profile: helpers.HLS_PROFILE_REMUX, plan: hlsRunPlan(safeRemuxFixture), startSec: 0},
		{name: "transcode", profile: helpers.HLS_PROFILE_720P_3MBPS, plan: hlsRunPlan(transcodeFixture), startSec: 1790},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			app := setupTestApp(t)
			fake := &fakeFFmpeg{plans: []fakeFFmpegRunPlan{tt.plan}, seekLandingFunc: returnSeekLanding(1)}
			app.FFmpeg = fake
			movieID := insertTestHLSMovieFixture(t, app, "h264", 1080)

			session, err := createTestHLSSession(app, context.Background(), movieID, tt.profile, testIntPtr(0), testPlaybackSessionID, tt.startSec, false)
			if err != nil {
				t.Fatalf("createHLSSession: %v", err)
			}
			defer cleanupHLSSession(session)

			if session.actualStartReady != nil {
				t.Fatal("an exact start must not wait for a measurement")
			}
			if got := session.actualStartSec(); got != float64(tt.startSec) {
				t.Fatalf("actual start = %v, want %d", got, tt.startSec)
			}
			if calls := fake.SeekLandingCalls(); len(calls) != 0 {
				t.Fatalf("SeekLanding calls = %d, want 0", len(calls))
			}
		})
	}
}

// The measurement belongs to the session: a stop, a seek that supersedes it,
// or shutdown ends it, and app.Wait sees it finish.
func TestStartHLSSession_StopEndsTheMeasurement(t *testing.T) {
	app := setupTestApp(t)
	entered := make(chan struct{})
	app.FFmpeg = &fakeFFmpeg{
		plans: []fakeFFmpegRunPlan{hlsRunPlan(safeRemuxFixture)},
		seekLandingFunc: func(ctx context.Context, _ ffmpeg.SeekLandingParams) (float64, error) {
			close(entered)
			<-ctx.Done()
			return 0, ctx.Err()
		},
	}
	movieID := insertTestHLSMovieFixture(t, app, "h264", 1080)

	session, err := createTestHLSSession(app, context.Background(), movieID, helpers.HLS_PROFILE_REMUX, testIntPtr(0), testPlaybackSessionID, 1790, false)
	if err != nil {
		t.Fatalf("createHLSSession: %v", err)
	}
	<-entered
	cleanupHLSSession(session)

	awaitMeasurement(t, session)
	app.Wait.Wait()
	if got := session.actualStartSec(); got != hlsUnknownActualStart {
		t.Fatalf("actual start = %v, want unknown after a stop", got)
	}
}

// heldSeekLanding holds a fake measurement until the test releases it. The
// measurement returns landing once released, or ctx's error when the session
// ends first; the release also runs at cleanup, so a failed assertion cannot
// leave the measurement blocking teardown.
func heldSeekLanding(t *testing.T) (func(ctx context.Context, landing float64) (float64, error), func()) {
	t.Helper()
	released := make(chan struct{})
	release := sync.OnceFunc(func() { close(released) })
	t.Cleanup(release)
	hold := func(ctx context.Context, landing float64) (float64, error) {
		select {
		case <-released:
			return landing, nil
		case <-ctx.Done():
			return 0, ctx.Err()
		}
	}
	return hold, release
}

func copyVideoManifestRequest(movieID int64) *http.Request {
	return httptest.NewRequest(http.MethodGet, fmt.Sprintf(
		"/api/movies/%d/hls/remux/playlist.m3u8?audio_track=0&playback_session=%s&start=1790",
		movieID, testPlaybackSessionID,
	), nil)
}

func TestHLSManifest_ActualStartUsesPlaybackTimeline(t *testing.T) {
	app := setupTestApp(t)
	fake := &fakeFFmpeg{
		plans:           []fakeFFmpegRunPlan{hlsRunPlan(safeRemuxFixture)},
		seekLandingFunc: returnSeekLanding(10),
	}
	app.FFmpeg = fake
	movieID := insertTestHLSMovieFixture(t, app, "h264", 1080)

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, fmt.Sprintf(
		"/api/movies/%d/hls/remux/playlist.m3u8?audio_track=0&playback_session=%s&start=12",
		movieID, testPlaybackSessionID,
	), nil)
	authenticatedRouter(t, app, 42).ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("manifest status = %d: %s", recorder.Code, recorder.Body.String())
	}
	got := recorder.Header().Get(hlsActualStartHeader)
	if got != "10.000" {
		t.Fatalf("actual start header = %q, want 10.000", got)
	}
	calls := fake.SeekLandingCalls()
	if len(calls) != 1 || calls[0].StartSec != 12 {
		t.Fatalf("SeekLanding calls = %+v, want one measurement at start 12", calls)
	}
}

// IglooTV reads X-Igloo-Actual-Start from the first manifest only, so that
// response waits for the measurement rather than answering without it.
func TestHLSManifest_FirstResponseWaitsForTheActualStart(t *testing.T) {
	app := setupTestApp(t)
	entered := make(chan struct{})
	release, releaseMeasurement := heldSeekLanding(t)
	app.FFmpeg = &fakeFFmpeg{
		plans: []fakeFFmpegRunPlan{hlsRunPlan(safeRemuxFixture)},
		seekLandingFunc: func(ctx context.Context, _ ffmpeg.SeekLandingParams) (float64, error) {
			close(entered)
			return release(ctx, 1780.195)
		},
	}
	movieID := insertTestHLSMovieFixture(t, app, "h264", 1080)
	handler := authenticatedRouter(t, app, 42)

	recorder := httptest.NewRecorder()
	done := make(chan struct{})
	go func() {
		defer close(done)
		handler.ServeHTTP(recorder, copyVideoManifestRequest(movieID))
	}()

	<-entered
	select {
	case <-done:
		t.Fatalf("manifest answered before the measurement finished: %d %q",
			recorder.Code, recorder.Header().Get(hlsActualStartHeader))
	case <-time.After(100 * time.Millisecond):
	}
	releaseMeasurement()
	<-done

	if recorder.Code != http.StatusOK {
		t.Fatalf("manifest status = %d: %s", recorder.Code, recorder.Body.String())
	}
	if got := recorder.Header().Get(hlsActualStartHeader); got != "1780.195" {
		t.Fatalf("actual start header = %q, want 1780.195", got)
	}
}

// A stuck measurement costs the manifest at most hlsActualStartWait; the
// response then goes out without the header, and a later one carries it.
func TestHLSManifest_ActualStartWaitIsBounded(t *testing.T) {
	previousWait := hlsActualStartWait
	hlsActualStartWait = 50 * time.Millisecond
	t.Cleanup(func() { hlsActualStartWait = previousWait })

	app := setupTestApp(t)
	release, releaseMeasurement := heldSeekLanding(t)
	app.FFmpeg = &fakeFFmpeg{
		plans: []fakeFFmpegRunPlan{hlsRunPlan(safeRemuxFixture)},
		seekLandingFunc: func(ctx context.Context, _ ffmpeg.SeekLandingParams) (float64, error) {
			return release(ctx, 1780.195)
		},
	}
	movieID := insertTestHLSMovieFixture(t, app, "h264", 1080)
	handler := authenticatedRouter(t, app, 42)

	first := httptest.NewRecorder()
	handler.ServeHTTP(first, copyVideoManifestRequest(movieID))
	if first.Code != http.StatusOK {
		t.Fatalf("first manifest status = %d: %s", first.Code, first.Body.String())
	}
	if got := first.Header().Get(hlsActualStartHeader); got != "" {
		t.Fatalf("actual start header = %q before the measurement finished, want none", got)
	}

	key := HLSSessionKey(movieRef(movieID), helpers.HLS_PROFILE_REMUX, testIntPtr(0), nil, testPlaybackSessionID, 1790, 42)
	cached, found := app.HLSSessionCache.Get(key)
	if !found {
		t.Fatalf("session %q was not cached", key)
	}
	session := cached.(*HLSSession)
	t.Cleanup(func() { cleanupHLSSession(session) })
	releaseMeasurement()
	awaitMeasurement(t, session)

	second := httptest.NewRecorder()
	handler.ServeHTTP(second, copyVideoManifestRequest(movieID))
	if got := second.Header().Get(hlsActualStartHeader); got != "1780.195" {
		t.Fatalf("later actual start header = %q, want 1780.195", got)
	}
}
