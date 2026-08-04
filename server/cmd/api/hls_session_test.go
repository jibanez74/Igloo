package main

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"

	"igloo/cmd/internal/helpers"
)

func TestHLSSessionKey(t *testing.T) {
	audioTrack := 2
	key := HLSSessionKey(123, "720p_3mbps", &audioTrack, testPlaybackSessionID, 40)
	want := "movie:123:720p_3mbps:audio:2:session:" + testPlaybackSessionID + ":start:40"
	if key != want {
		t.Errorf("HLSSessionKey = %q, want %q", key, want)
	}
}

func TestCreateHLSSession_ErrorsWhenMovieHasNoDuration(t *testing.T) {
	app := setupTestApp(t)
	defer app.DB.Close()

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

func TestCreateHLSSession_ClampsStartToZeroForTinyDurations(t *testing.T) {
	app := setupTestApp(t)
	defer app.DB.Close()
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

func TestMeasureHLSSessionStart_RecordsKeyframeBeforeRequestedStart(t *testing.T) {
	app := setupTestApp(t)
	defer app.DB.Close()

	app.Wait = &sync.WaitGroup{}
	prober := &stubKeyframeFfprobe{keyframeSec: 591.174}
	app.Ffprobe = prober

	session := &HLSSession{MovieID: 1, CopyVideo: true, StartSec: 600}
	session.setActualStartSec(hlsUnknownActualStart)

	app.Wait.Add(1)
	app.measureHLSSessionStart(context.Background(), session, "/movies/example.mp4", 0, 600)
	app.Wait.Wait()

	if prober.calls != 1 {
		t.Fatalf("keyframe probe calls = %d, want 1", prober.calls)
	}
	if prober.target != 600 {
		t.Fatalf("probe target = %v, want 600", prober.target)
	}
	if got := session.actualStartSec(); got != 591.174 {
		t.Fatalf("actual start = %v, want 591.174", got)
	}
}

// The probe is advisory: a failure must leave the start unknown rather than
// publish a wrong one, so the client keeps its existing fallback.
func TestMeasureHLSSessionStart_LeavesStartUnknownOnFailure(t *testing.T) {
	app := setupTestApp(t)
	defer app.DB.Close()

	app.Wait = &sync.WaitGroup{}
	app.Ffprobe = &stubKeyframeFfprobe{err: errors.New("probe failed")}

	session := &HLSSession{MovieID: 1, CopyVideo: true, StartSec: 600}
	session.setActualStartSec(hlsUnknownActualStart)

	app.Wait.Add(1)
	app.measureHLSSessionStart(context.Background(), session, "/movies/example.mp4", 0, 600)
	app.Wait.Wait()

	if got := session.actualStartSec(); got >= 0 {
		t.Fatalf("actual start = %v, want it to stay unknown", got)
	}
}
