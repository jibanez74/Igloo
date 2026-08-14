package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"igloo/cmd/internal/ffmpeg"
	"igloo/cmd/internal/ffmpeg/fmp4testutil"
	"igloo/cmd/internal/ffprobe"
	"igloo/cmd/internal/helpers"

	"github.com/go-chi/chi/v5"
)

const (
	testPlaybackSessionID      = "4a5d0cb7-66f7-45ec-95d9-93fbe6e9eea4"
	testOtherPlaybackSessionID = "b3c1f6d2-8a4e-4f0b-9c7d-1e2a3b4c5d6e"
)

type fakeFFmpegRunPlan struct {
	StartErr   error
	ExitErr    error
	WriteFiles func(outDir string) error
	Started    chan<- struct{}
	Continue   <-chan struct{}
}

type fakeFFmpeg struct {
	mu            sync.Mutex
	plans         []fakeFFmpegRunPlan
	calls         []ffmpeg.HLSParams
	subtitleCalls int
}

func (f *fakeFFmpeg) RunHLS(
	_ context.Context,
	params ffmpeg.HLSParams,
	onExit func(exitErr error, stderrTail []string),
) (*exec.Cmd, error) {
	f.mu.Lock()
	callIndex := len(f.calls)
	f.calls = append(f.calls, params)

	if callIndex >= len(f.plans) {
		f.mu.Unlock()
		return nil, fmt.Errorf("unexpected RunHLS call %d", callIndex)
	}

	plan := f.plans[callIndex]
	f.mu.Unlock()
	if plan.Started != nil {
		plan.Started <- struct{}{}
	}
	if plan.Continue != nil {
		<-plan.Continue
	}

	if plan.StartErr != nil {
		return nil, plan.StartErr
	}

	if plan.WriteFiles != nil {
		err := plan.WriteFiles(params.OutDir)
		if err != nil {
			return nil, err
		}
	}

	if onExit != nil {
		onExit(plan.ExitErr, nil)
	}

	return &exec.Cmd{}, nil
}

func (f *fakeFFmpeg) ExtractSubtitleAsWebVTT(_ context.Context, _ string, _ int64) ([]byte, error) {
	f.mu.Lock()
	f.subtitleCalls++
	f.mu.Unlock()
	return []byte("WEBVTT\n"), nil
}

func (f *fakeFFmpeg) SubtitleCallCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.subtitleCalls
}

func (f *fakeFFmpeg) Capabilities() ffmpeg.Capabilities {
	return ffmpeg.Capabilities{}
}

func (f *fakeFFmpeg) CallCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.calls)
}

func (f *fakeFFmpeg) Calls() []ffmpeg.HLSParams {
	f.mu.Lock()
	defer f.mu.Unlock()

	out := make([]ffmpeg.HLSParams, len(f.calls))
	copy(out, f.calls)
	return out
}

// createTestHLSSession mirrors the production call sequence: load and
// normalize via loadHLSMovieForSession, then create the session.
func createTestHLSSession(
	app *Application,
	ctx context.Context,
	movieID int64,
	profile string,
	audioTrack *int,
	playbackSession string,
	startSec int,
	isRoom bool,
) (*HLSSession, error) {
	movie, effectiveStartSec, err := app.loadHLSMovieForSession(ctx, movieID, startSec)
	if err != nil {
		return nil, err
	}
	return app.createHLSSession(ctx, &movie, profile, audioTrack, nil, playbackSession, effectiveStartSec, isRoom, 0)
}

type testFMP4Fixture = fmp4testutil.Fixture

func writeTestHLSFixture(outDir string, fixture testFMP4Fixture) error {
	return fmp4testutil.WriteHLSFixture(outDir, fixture)
}

// The output shapes the HLS tests script FFmpeg to produce, named for the
// behaviour they stand in for. Remux preflight inspects the first
// HLS_REMUX_PREVALIDATE_SEGMENTS segments, so a remux verdict needs all of
// them; a transcode session is only ever asked whether it started.
var (
	safeRemuxFixture   = testFMP4Fixture{SafeVideo: true, Segments: helpers.HLS_REMUX_PREVALIDATE_SEGMENTS}
	unsafeRemuxFixture = testFMP4Fixture{SafeVideo: false, Segments: helpers.HLS_REMUX_PREVALIDATE_SEGMENTS}
	transcodeFixture   = testFMP4Fixture{SafeVideo: true, Segments: 1}
)

func hlsRunPlan(fixture testFMP4Fixture) fakeFFmpegRunPlan {
	return fakeFFmpegRunPlan{
		WriteFiles: func(outDir string) error {
			return writeTestHLSFixture(outDir, fixture)
		},
	}
}

// newHLSTestHandler wires the three personal HLS routes behind the session
// middleware with userID already authenticated, mirroring the paths registered
// in routes.go.
func newHLSTestHandler(t *testing.T, app *Application, userID int64) http.Handler {
	t.Helper()

	app.InitSession()
	authenticated := func(handler http.HandlerFunc) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			app.SessionManager.Put(r.Context(), cookieUserID, userID)
			handler(w, r)
		}
	}

	router := chi.NewRouter()
	router.Post("/api/movies/{id}/hls/session/stop", authenticated(app.StopPersonalHLSSession))
	router.Get("/api/movies/{id}/hls/{profile}/"+helpers.HLS_PLAYLIST_FILENAME, authenticated(app.HLSManifest))
	router.Get("/api/movies/{id}/hls/{profile}/{filename}", authenticated(app.HLSSegment))

	return app.SessionManager.LoadAndSave(router)
}

// noMetadataProbe completes ffprobe.FfprobeInterface for stubs that only serve
// the HLS keyframe lookup. Mirror of noKeyframeProbe in movies_scanner_test.go.
type noMetadataProbe struct{}

func (noMetadataProbe) GetMetadata(string) (*ffprobe.FfprobeResult, error) {
	return nil, errors.New("metadata lookup is not part of the HLS session path")
}

func (noMetadataProbe) GetAudioMetadata(string) (*ffprobe.FfprobeResult, error) {
	return nil, errors.New("audio metadata lookup is not part of the HLS session path")
}

// stubKeyframeFfprobe answers the keyframe lookup that measures where a
// copy-video session's media really begins.
type stubKeyframeFfprobe struct {
	noMetadataProbe

	keyframeSec float64
	err         error

	mu     sync.Mutex
	target float64
	calls  int
}

func (s *stubKeyframeFfprobe) KeyframeAtOrBefore(
	_ context.Context,
	_ string,
	_ int64,
	targetSec float64,
) (float64, error) {
	s.mu.Lock()
	s.target = targetSec
	s.calls++
	s.mu.Unlock()

	if s.err != nil {
		return 0, s.err
	}
	return s.keyframeSec, nil
}

// blockHLSSessionCleanup wedges a session's teardown open so a test can observe
// the cache and lock state the caller published before teardown began.
func blockHLSSessionCleanup(t *testing.T, session *HLSSession) (<-chan struct{}, func()) {
	t.Helper()

	started := make(chan struct{})
	unblock := make(chan struct{})
	var unblockOnce sync.Once
	session.Cancel = func() {
		close(started)
		<-unblock
	}
	release := func() {
		unblockOnce.Do(func() {
			close(unblock)
		})
	}
	t.Cleanup(release)
	return started, release
}

func waitForHLSSessionCleanupToBlock(t *testing.T, started <-chan struct{}, release func()) {
	t.Helper()

	select {
	case <-started:
	case <-time.After(2 * time.Second):
		release()
		t.Fatal("timed out waiting for HLS session cleanup to start")
	}
}

func waitForHLSSessionCleanupResult[T any](t *testing.T, result <-chan T) T {
	t.Helper()

	select {
	case value := <-result:
		return value
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for HLS session cleanup to finish")
		var zero T
		return zero
	}
}

// insertTestHLSMovieFixture seeds a 7200s H.264 movie with one AAC audio track
// at absolute stream index 1. The file path is synthetic and does not exist.
func insertTestHLSMovieFixture(t *testing.T, app *Application, videoCodec string, height int64) int64 {
	t.Helper()

	path := fmt.Sprintf("/tmp/%s-%s.mkv", sanitizeTestPathComponent(t.Name()), sanitizeTestPathComponent(videoCodec))
	return insertTestHLSMovieFixtureAt(t, app, videoCodec, height, path, "mkv")
}

// insertTestHLSMovieFixtureAt is insertTestHLSMovieFixture with an explicit
// file path and container, for tests that put a real media file on disk.
func insertTestHLSMovieFixtureAt(
	t *testing.T,
	app *Application,
	videoCodec string,
	height int64,
	path string,
	container string,
) int64 {
	t.Helper()

	result, err := app.DB.Exec(`
		INSERT INTO movies (title, file_path, file_name, size, container, mime_type, adult, duration)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	`,
		t.Name(),
		path,
		filepath.Base(path),
		1_000_000,
		container,
		helpers.VideoMimeTypes[container],
		0,
		7200.0,
	)
	if err != nil {
		t.Fatalf("insert movie: %v", err)
	}

	movieID, err := result.LastInsertId()
	if err != nil {
		t.Fatalf("movie last insert id: %v", err)
	}

	_, err = app.DB.Exec(`
		INSERT INTO video_streams (movie_id, stream_index, codec, bit_rate, width, height, frame_rate)
		VALUES (?, ?, ?, ?, ?, ?, ?)
	`,
		movieID,
		0,
		videoCodec,
		5_000_000,
		1920,
		height,
		23.976,
	)
	if err != nil {
		t.Fatalf("insert video stream: %v", err)
	}

	// codec_profile LC keeps the fixture on the audio-copy path: the copy gate
	// requires a confirmed AAC-LC profile.
	_, err = app.DB.Exec(`
		INSERT INTO audio_streams (movie_id, stream_index, codec, codec_profile, bit_rate, channels)
		VALUES (?, ?, ?, ?, ?, ?)
	`,
		movieID,
		1,
		"aac",
		"LC",
		192000,
		2,
	)
	if err != nil {
		t.Fatalf("insert audio stream: %v", err)
	}

	return movieID
}

func testIntPtr(v int) *int {
	return &v
}

func sanitizeTestPathComponent(value string) string {
	value = strings.ReplaceAll(value, "/", "_")
	value = strings.ReplaceAll(value, " ", "_")
	return value
}
