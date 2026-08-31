package main

import (
	"context"
	"database/sql"
	"errors"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"igloo/cmd/internal/database"
	"igloo/cmd/internal/ffmpeg"
	"igloo/cmd/internal/helpers"
)

func TestHLSSessionKey(t *testing.T) {
	audioTrack := 2
	tests := []struct {
		name         string
		audioProfile *helpers.HLSAudioProfileRequest
		wantMode     string
	}{
		{name: "legacy", wantMode: "legacy"},
		{
			name:         "explicit ac3 stereo",
			audioProfile: &helpers.HLSAudioProfileRequest{Codec: helpers.HLSAudioCodecAC3, MaxChannels: 2},
			wantMode:     "explicit:ac3:2",
		},
		{
			name:         "explicit ac3 surround",
			audioProfile: &helpers.HLSAudioProfileRequest{Codec: helpers.HLSAudioCodecAC3, MaxChannels: 6},
			wantMode:     "explicit:ac3:6",
		},
		{
			name:         "explicit eac3 stereo",
			audioProfile: &helpers.HLSAudioProfileRequest{Codec: helpers.HLSAudioCodecEAC3, MaxChannels: 2},
			wantMode:     "explicit:eac3:2",
		},
		{
			name:         "explicit eac3 surround",
			audioProfile: &helpers.HLSAudioProfileRequest{Codec: helpers.HLSAudioCodecEAC3, MaxChannels: 6},
			wantMode:     "explicit:eac3:6",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			key := HLSSessionKey(123, "720p_3mbps", &audioTrack, tt.audioProfile, testPlaybackSessionID, 40, 456)
			want := "user:456:movie:123:720p_3mbps:audio:2:" + tt.wantMode + ":session:" + testPlaybackSessionID + ":start:40"
			if key != want {
				t.Errorf("HLSSessionKey = %q, want %q", key, want)
			}
		})
	}

	base := HLSSessionKey(123, "720p_3mbps", &audioTrack, nil, testPlaybackSessionID, 40, 456)
	otherAudioTrack := 3
	variants := map[string]string{
		"owner":            HLSSessionKey(123, "720p_3mbps", &audioTrack, nil, testPlaybackSessionID, 40, 457),
		"movie":            HLSSessionKey(124, "720p_3mbps", &audioTrack, nil, testPlaybackSessionID, 40, 456),
		"profile":          HLSSessionKey(123, "1080p_8mbps", &audioTrack, nil, testPlaybackSessionID, 40, 456),
		"audio track":      HLSSessionKey(123, "720p_3mbps", &otherAudioTrack, nil, testPlaybackSessionID, 40, 456),
		"audio mode":       HLSSessionKey(123, "720p_3mbps", &audioTrack, explicitAudioRequest(helpers.HLSAudioCodecAC3, 6), testPlaybackSessionID, 40, 456),
		"playback session": HLSSessionKey(123, "720p_3mbps", &audioTrack, nil, testOtherPlaybackSessionID, 40, 456),
		"start":            HLSSessionKey(123, "720p_3mbps", &audioTrack, nil, testPlaybackSessionID, 41, 456),
	}
	for dimension, key := range variants {
		if key == base {
			t.Errorf("changing %s did not change the HLS session key", dimension)
		}
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

	// The transient failure must leave no persisted verdict behind.
	_, err = app.Queries.GetRemuxSafetyVerdict(context.Background(), database.GetRemuxSafetyVerdictParams{
		MovieID:     movieID,
		StreamIndex: 0,
	})
	if !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("GetRemuxSafetyVerdict error = %v, want sql.ErrNoRows after a transient preflight failure", err)
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

// A verdict must outlive the process: before persistence, every server restart
// re-paid the multi-second remux preflight for every file (audit H6).
func TestCreateHLSSession_UnsafeVerdictSurvivesRestart(t *testing.T) {
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

	firstSession, err := createTestHLSSession(app, context.Background(), movieID, helpers.HLS_PROFILE_REMUX, testIntPtr(0), testPlaybackSessionID, 0, false)
	if err != nil {
		t.Fatalf("createHLSSession returned error: %v", err)
	}
	defer cleanupHLSSession(firstSession)

	restarted := restartTestApp(t, app)
	restartedFake := &fakeFFmpeg{
		plans: []fakeFFmpegRunPlan{
			hlsRunPlan(transcodeFixture),
		},
	}
	restarted.FFmpeg = restartedFake

	session, err := createTestHLSSession(restarted, context.Background(), movieID, helpers.HLS_PROFILE_REMUX, testIntPtr(0), testPlaybackSessionID, 0, false)
	if err != nil {
		t.Fatalf("createHLSSession after restart returned error: %v", err)
	}
	defer cleanupHLSSession(session)

	if session.CopyVideo {
		t.Fatal("CopyVideo = true, want the persisted unsafe verdict to force a transcode")
	}
	calls := restartedFake.Calls()
	if len(calls) != 1 {
		t.Fatalf("RunHLS call count after restart = %d, want 1 (no remux attempt)", len(calls))
	}
	if calls[0].Profile != helpers.HLS_PROFILE_1080P_8MBPS {
		t.Fatalf("RunHLS profile after restart = %q, want %q", calls[0].Profile, helpers.HLS_PROFILE_1080P_8MBPS)
	}
}

func TestCreateHLSSession_SafeVerdictSurvivesRestart(t *testing.T) {
	app := setupTestApp(t)
	defer app.DB.Close()

	fake := &fakeFFmpeg{
		plans: []fakeFFmpegRunPlan{
			hlsRunPlan(safeRemuxFixture),
		},
	}
	app.FFmpeg = fake

	movieID := insertTestHLSMovieFixture(t, app, "h264", 1080)

	firstSession, err := createTestHLSSession(app, context.Background(), movieID, helpers.HLS_PROFILE_REMUX, testIntPtr(0), testPlaybackSessionID, 0, false)
	if err != nil {
		t.Fatalf("createHLSSession returned error: %v", err)
	}
	defer cleanupHLSSession(firstSession)

	restarted := restartTestApp(t, app)
	restartedFake := &fakeFFmpeg{
		plans: []fakeFFmpegRunPlan{
			// One segment is not enough to pass preflight, so a restarted server
			// that still ran it would fall back to a transcode.
			hlsRunPlan(transcodeFixture),
		},
	}
	restarted.FFmpeg = restartedFake

	session, err := createTestHLSSession(restarted, context.Background(), movieID, helpers.HLS_PROFILE_REMUX, testIntPtr(0), testPlaybackSessionID, 0, false)
	if err != nil {
		t.Fatalf("createHLSSession after restart returned error: %v", err)
	}
	defer cleanupHLSSession(session)

	if !session.CopyVideo {
		t.Fatal("CopyVideo = false, want the persisted safe verdict to skip the preflight")
	}
	calls := restartedFake.Calls()
	if len(calls) != 1 {
		t.Fatalf("RunHLS call count after restart = %d, want 1", len(calls))
	}
	if calls[0].Profile != helpers.HLS_PROFILE_REMUX {
		t.Fatalf("RunHLS profile after restart = %q, want remux", calls[0].Profile)
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

// Browsers do not deinterlace, so an interlaced source must be rejected from
// remux and the fallback transcode must carry the deinterlace flag.
func TestCreateHLSSession_InterlacedFallsBackToTranscodeWithDeinterlace(t *testing.T) {
	app := setupTestApp(t)
	defer app.DB.Close()

	fake := &fakeFFmpeg{
		plans: []fakeFFmpegRunPlan{
			hlsRunPlan(transcodeFixture),
		},
	}
	app.FFmpeg = fake

	movieID := insertTestHLSMovieFixture(t, app, "h264", 1080)
	_, err := app.DB.Exec(`UPDATE video_streams SET field_order = ? WHERE movie_id = ?`, "tt", movieID)
	if err != nil {
		t.Fatalf("update video stream: %v", err)
	}

	session, err := createTestHLSSession(app, context.Background(), movieID, helpers.HLS_PROFILE_REMUX, testIntPtr(0), testPlaybackSessionID, 0, false)
	if err != nil {
		t.Fatalf("createHLSSession returned error: %v", err)
	}
	defer cleanupHLSSession(session)

	if session.CopyVideo {
		t.Fatal("CopyVideo = true, want false for an interlaced source")
	}

	calls := fake.Calls()
	if len(calls) != 1 {
		t.Fatalf("RunHLS call count = %d, want 1", len(calls))
	}
	if calls[0].Profile != helpers.HLS_PROFILE_1080P_8MBPS || calls[0].CopyVideo {
		t.Fatalf("RunHLS call = profile %q copyVideo %v, want fallback transcode", calls[0].Profile, calls[0].CopyVideo)
	}
	if !calls[0].Deinterlace {
		t.Fatal("Deinterlace = false, want true for an interlaced source transcode")
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

func TestCreateHLSSession_TranscodeLimiterParticipation(t *testing.T) {
	tests := []struct {
		name         string
		profile      string
		videoOnly    bool
		audioCodec   string
		audioProfile any
		request      *helpers.HLSAudioProfileRequest
		wantSlot     bool
	}{
		{name: "copy-only remux", profile: helpers.HLS_PROFILE_REMUX, audioCodec: "aac", audioProfile: "LC"},
		{name: "video-only remux", profile: helpers.HLS_PROFILE_REMUX, videoOnly: true},
		{name: "legacy audio encode", profile: helpers.HLS_PROFILE_REMUX, audioCodec: "dts", wantSlot: true},
		{name: "explicit AC-3 encode", profile: helpers.HLS_PROFILE_REMUX, audioCodec: "aac", audioProfile: "LC", request: explicitAudioRequest(helpers.HLSAudioCodecAC3, 6), wantSlot: true},
		{name: "explicit E-AC-3 encode", profile: helpers.HLS_PROFILE_REMUX, audioCodec: "aac", audioProfile: "LC", request: explicitAudioRequest(helpers.HLSAudioCodecEAC3, 6), wantSlot: true},
		{name: "video encode", profile: helpers.HLS_PROFILE_720P_3MBPS, audioCodec: "aac", audioProfile: "LC", wantSlot: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			app := setupTestApp(t)
			defer app.DB.Close()
			fake := &fakeFFmpeg{plans: []fakeFFmpegRunPlan{hlsRunPlan(safeRemuxFixture)}}
			app.FFmpeg = fake

			movieID := insertTestHLSMovieFixture(t, app, "h264", 1080)
			audioTrack := testIntPtr(0)
			if tt.videoOnly {
				_, err := app.DB.Exec(`DELETE FROM audio_streams WHERE movie_id = ?`, movieID)
				if err != nil {
					t.Fatalf("delete audio streams: %v", err)
				}
				audioTrack = nil
			} else {
				setTestHLSAudioStream(t, app, movieID, tt.audioCodec, tt.audioProfile, 6, "5.1(side)")
			}

			app.HLSTranscodeLimiter = newHLSTranscodeLimiter(1)
			release, err := app.acquireHLSTranscodeSlot(context.Background(), 0)
			if err != nil {
				t.Fatalf("acquireHLSTranscodeSlot: %v", err)
			}
			defer release()

			session, err := createTestHLSSessionWithAudio(
				app, context.Background(), movieID, tt.profile, audioTrack,
				tt.request, testPlaybackSessionID, 0, false,
			)
			if tt.wantSlot {
				var capacityErr *hlsTranscodeCapacityError
				if !errors.As(err, &capacityErr) {
					t.Fatalf("error = %v, want hlsTranscodeCapacityError", err)
				}
				if fake.CallCount() != 0 {
					t.Fatalf("RunHLS call count = %d, want 0", fake.CallCount())
				}

				release()
				session, err = createTestHLSSessionWithAudio(
					app, context.Background(), movieID, tt.profile, audioTrack,
					tt.request, testPlaybackSessionID, 0, false,
				)
				if err != nil {
					t.Fatalf("createHLSSession after releasing capacity: %v", err)
				}
				defer cleanupHLSSession(session)
				if !session.RequiresTranscodeSlot {
					t.Fatal("RequiresTranscodeSlot = false, want true")
				}
				return
			}

			if err != nil {
				t.Fatalf("createHLSSession returned error: %v", err)
			}
			defer cleanupHLSSession(session)
			if session.RequiresTranscodeSlot {
				t.Fatal("RequiresTranscodeSlot = true, want false")
			}
		})
	}
}

func TestCreateHLSSession_TranscodeFailsWhenLimiterFull(t *testing.T) {
	app := setupTestApp(t)
	defer app.DB.Close()
	app.FFmpeg = &fakeFFmpeg{}

	app.HLSTranscodeLimiter = newHLSTranscodeLimiter(1)
	release, err := app.acquireHLSTranscodeSlot(context.Background(), 0)
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

// audio_track and the movie's audio streams have to agree: a movie with audio
// needs a track chosen, and a video-only movie must not be handed one, or
// FFmpeg is asked to -map a stream that does not exist.
func TestCreateHLSSession_AudioTrackValidation(t *testing.T) {
	tests := []struct {
		name       string
		videoOnly  bool
		audioTrack *int
		wantErr    string
	}{
		{
			name:      "video-only movie without a track starts",
			videoOnly: true,
		},
		{
			name:       "video-only movie rejects a track",
			videoOnly:  true,
			audioTrack: testIntPtr(0),
			wantErr:    "not valid for video-only",
		},
		{
			name:    "movie with audio requires a track",
			wantErr: "audio_track is required",
		},
		{
			name:       "movie with audio rejects a track past the stream count",
			audioTrack: testIntPtr(1),
			wantErr:    "out of range",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			app := setupTestApp(t)
			defer app.DB.Close()
			app.FFmpeg = &fakeFFmpeg{plans: []fakeFFmpegRunPlan{{}}}

			movieID := insertTestHLSMovieFixture(t, app, "h264", 1080)
			if tt.videoOnly {
				_, err := app.DB.Exec(`DELETE FROM audio_streams WHERE movie_id = ?`, movieID)
				if err != nil {
					t.Fatalf("delete audio streams: %v", err)
				}
			}

			session, err := createTestHLSSession(
				app,
				context.Background(),
				movieID,
				helpers.HLS_PROFILE_720P_3MBPS,
				tt.audioTrack,
				testPlaybackSessionID,
				0,
				false,
			)

			if tt.wantErr == "" {
				if err != nil {
					t.Fatalf("createHLSSession returned error: %v", err)
				}
				cleanupHLSSession(session)
				return
			}
			if err == nil {
				cleanupHLSSession(session)
				t.Fatalf("createHLSSession error = nil, want it to contain %q", tt.wantErr)
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("error = %v, want it to contain %q", err, tt.wantErr)
			}
		})
	}
}

func TestLoadHLSMovieForSession_RejectsNegativeStart(t *testing.T) {
	app := setupTestApp(t)
	defer app.DB.Close()

	movieID := insertTestHLSMovieFixture(t, app, "h264", 1080)

	_, _, err := app.loadHLSMovieForSession(context.Background(), movieID, -1)
	if err == nil {
		t.Fatal("loadHLSMovieForSession error = nil, want a rejection")
	}
	if !strings.Contains(err.Error(), "outside movie duration") {
		t.Fatalf("error = %v, want it to mention the duration bound", err)
	}
}

// The copy-video start probe is wired up inside startHLSSession. Without it the
// client maps session time to movie time using the requested offset, so the
// clock and every watch-progress write run ahead of the picture.
func TestStartHLSSession_MeasuresActualStartForCopyVideo(t *testing.T) {
	app := setupTestApp(t)
	defer app.DB.Close()
	app.FFmpeg = &fakeFFmpeg{plans: []fakeFFmpegRunPlan{hlsRunPlan(safeRemuxFixture)}}
	app.Wait = &sync.WaitGroup{}
	app.Ffprobe = &stubKeyframeFfprobe{keyframeSec: 84}

	movieID := insertTestHLSMovieFixture(t, app, "h264", 1080)

	session, err := createTestHLSSession(
		app,
		context.Background(),
		movieID,
		helpers.HLS_PROFILE_REMUX,
		testIntPtr(0),
		testPlaybackSessionID,
		100,
		false,
	)
	if err != nil {
		t.Fatalf("createHLSSession returned error: %v", err)
	}
	defer cleanupHLSSession(session)

	app.Wait.Wait()

	if got := session.actualStartSec(); got != 84 {
		t.Fatalf("actual start = %v, want the measured keyframe 84", got)
	}
}

func TestIsHDRStream(t *testing.T) {
	tests := []struct {
		name          string
		colorTransfer sql.NullString
		want          bool
	}{
		{name: "unset transfer is not HDR", colorTransfer: sql.NullString{}},
		{name: "empty transfer is not HDR", colorTransfer: sql.NullString{String: "", Valid: true}},
		{name: "SDR transfer is not HDR", colorTransfer: sql.NullString{String: "bt709", Valid: true}},
		{name: "PQ is HDR10", colorTransfer: sql.NullString{String: "smpte2084", Valid: true}, want: true},
		// ffprobe reports HLG with mixed case and the scanner stores it verbatim.
		{name: "HLG is matched case-insensitively", colorTransfer: sql.NullString{String: " ARIB-STD-B67 ", Valid: true}, want: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isHDRStream(&database.VideoStream{ColorTransfer: tt.colorTransfer})
			if got != tt.want {
				t.Fatalf("isHDRStream(%q) = %v, want %v", tt.colorTransfer.String, got, tt.want)
			}
		})
	}
}

func TestIsInterlacedStream(t *testing.T) {
	tests := []struct {
		name       string
		fieldOrder sql.NullString
		want       bool
	}{
		// NULL rows predate the field_order column and must stay eligible for
		// everything progressive content is.
		{name: "unset field order is progressive", fieldOrder: sql.NullString{}},
		{name: "progressive", fieldOrder: sql.NullString{String: "progressive", Valid: true}},
		{name: "empty string is progressive", fieldOrder: sql.NullString{String: "", Valid: true}},
		{name: "unrecognized value is progressive", fieldOrder: sql.NullString{String: "unknown", Valid: true}},
		{name: "top field first", fieldOrder: sql.NullString{String: "tt", Valid: true}, want: true},
		{name: "bottom field first", fieldOrder: sql.NullString{String: "bb", Valid: true}, want: true},
		{name: "top coded first displayed first", fieldOrder: sql.NullString{String: "tb", Valid: true}, want: true},
		{name: "bottom coded first displayed first", fieldOrder: sql.NullString{String: "bt", Valid: true}, want: true},
		{name: "matched case-insensitively", fieldOrder: sql.NullString{String: " TT ", Valid: true}, want: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isInterlacedStream(&database.VideoStream{FieldOrder: tt.fieldOrder})
			if got != tt.want {
				t.Fatalf("isInterlacedStream(%q) = %v, want %v", tt.fieldOrder.String, got, tt.want)
			}
		})
	}
}

func TestIsVFRStream(t *testing.T) {
	tests := []struct {
		name         string
		frameRate    float64
		avgFrameRate sql.NullString
		want         bool
	}{
		{name: "matching rates are constant", frameRate: 23.976, avgFrameRate: sql.NullString{String: "24000/1001", Valid: true}},
		{name: "rounding noise is constant", frameRate: 23.976, avgFrameRate: sql.NullString{String: "23976/1000", Valid: true}},
		{name: "diverging average is VFR", frameRate: 30, avgFrameRate: sql.NullString{String: "18574/1000", Valid: true}, want: true},
		{name: "unset average cannot prove VFR", frameRate: 23.976, avgFrameRate: sql.NullString{}},
		{name: "zero average cannot prove VFR", frameRate: 23.976, avgFrameRate: sql.NullString{String: "0/0", Valid: true}},
		{name: "unparseable average cannot prove VFR", frameRate: 23.976, avgFrameRate: sql.NullString{String: "garbage", Valid: true}},
		{name: "zero nominal rate cannot prove VFR", frameRate: 0, avgFrameRate: sql.NullString{String: "24/1", Valid: true}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isVFRStream(&database.VideoStream{FrameRate: tt.frameRate, AvgFrameRate: tt.avgFrameRate})
			if got != tt.want {
				t.Fatalf("isVFRStream(rate=%v, avg=%q) = %v, want %v", tt.frameRate, tt.avgFrameRate.String, got, tt.want)
			}
		})
	}
}

func TestIsCopySafeAACStream(t *testing.T) {
	tests := []struct {
		name     string
		codec    string
		profile  sql.NullString
		channels int64
		want     bool
	}{
		{name: "confirmed LC copies", codec: "aac", profile: sql.NullString{String: "LC", Valid: true}, channels: 2, want: true},
		// Channel count is deliberately not part of the gate: every browser
		// that decodes AAC-LC in fMP4 decodes it multichannel and downmixes at
		// the output device, so surround survives instead of being re-encoded
		// to stereo.
		{name: "5.1 copies", codec: "aac", profile: sql.NullString{String: "LC", Valid: true}, channels: 6, want: true},
		{name: "7.1 copies", codec: "aac", profile: sql.NullString{String: "LC", Valid: true}, channels: 8, want: true},
		// ffprobe reports the profile verbatim; case and padding must not matter.
		{name: "profile matched case-insensitively", codec: "aac", profile: sql.NullString{String: "lc", Valid: true}, want: true},
		{name: "profile trimmed", codec: "aac", profile: sql.NullString{String: " LC ", Valid: true}, want: true},
		{name: "codec matched case-insensitively", codec: "AAC", profile: sql.NullString{String: "LC", Valid: true}, want: true},
		{name: "HE-AAC transcodes", codec: "aac", profile: sql.NullString{String: "HE-AAC", Valid: true}},
		{name: "HE-AACv2 transcodes", codec: "aac", profile: sql.NullString{String: "HE-AACv2", Valid: true}},
		{name: "unknown profile cannot prove safety", codec: "aac", profile: sql.NullString{}},
		{name: "empty profile cannot prove safety", codec: "aac", profile: sql.NullString{String: "", Valid: true}},
		{name: "non-AAC never copies", codec: "ac3", profile: sql.NullString{String: "LC", Valid: true}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isCopySafeAACStream(&database.AudioStream{
				Codec:        tt.codec,
				CodecProfile: tt.profile,
				Channels:     tt.channels,
			})
			if got != tt.want {
				t.Fatalf("isCopySafeAACStream(%q, %q, %dch) = %v, want %v", tt.codec, tt.profile.String, tt.channels, got, tt.want)
			}
		})
	}
}

// The copy gate must read the scanned profile end to end: the fixture's AAC-LC
// track copies, and the same track with its profile wiped (a row scanned
// before profiles were persisted) transcodes.
func TestCreateHLSSession_AACProfileGatesAudioCopy(t *testing.T) {
	tests := []struct {
		name          string
		clearProfile  bool
		wantCopyAudio bool
	}{
		{name: "confirmed LC copies", wantCopyAudio: true},
		{name: "unknown profile transcodes", clearProfile: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			app := setupTestApp(t)
			defer app.DB.Close()

			fake := &fakeFFmpeg{plans: []fakeFFmpegRunPlan{hlsRunPlan(transcodeFixture)}}
			app.FFmpeg = fake

			movieID := insertTestHLSMovieFixture(t, app, "h264", 1080)
			if tt.clearProfile {
				_, err := app.DB.Exec(`UPDATE audio_streams SET codec_profile = NULL WHERE movie_id = ?`, movieID)
				if err != nil {
					t.Fatalf("clear codec_profile: %v", err)
				}
			}

			session, err := createTestHLSSession(app, context.Background(), movieID, helpers.HLS_PROFILE_720P_3MBPS, testIntPtr(0), testPlaybackSessionID, 0, false)
			if err != nil {
				t.Fatalf("createHLSSession returned error: %v", err)
			}
			defer cleanupHLSSession(session)

			calls := fake.Calls()
			if len(calls) != 1 {
				t.Fatalf("expected 1 RunHLS call, got %d", len(calls))
			}
			if calls[0].CopyAudio != tt.wantCopyAudio {
				t.Fatalf("CopyAudio = %v, want %v", calls[0].CopyAudio, tt.wantCopyAudio)
			}
		})
	}
}

func TestHLSTranscodeRoot(t *testing.T) {
	app := setupTestApp(t)
	defer app.DB.Close()

	t.Run("a configured transcode dir wins", func(t *testing.T) {
		app.SetSettings(&database.Setting{TranscodeDir: "/mnt/fast/transcode"})
		if got := app.hlsTranscodeRoot(); got != "/mnt/fast/transcode" {
			t.Fatalf("hlsTranscodeRoot() = %q, want the settings override", got)
		}
	})

	// Settings are never nil (see Application.settings), so a blank column is
	// the only way the runtime config is used.
	t.Run("a blank transcode dir falls back to the runtime config", func(t *testing.T) {
		app.SetSettings(&database.Setting{TranscodeDir: "   "})
		if got := app.hlsTranscodeRoot(); got != app.Config.effectiveTranscodeDir() {
			t.Fatalf("hlsTranscodeRoot() = %q, want %q", got, app.Config.effectiveTranscodeDir())
		}
	})
}

// Free space is advisory: an unreadable filesystem must not take playback down.
func TestCheckHLSTranscodeSpace_UnreadableFilesystemDoesNotBlockPlayback(t *testing.T) {
	app := setupTestApp(t)
	defer app.DB.Close()

	err := app.checkHLSTranscodeSpace(filepath.Join(t.TempDir(), "gone"))
	if err != nil {
		t.Fatalf("checkHLSTranscodeSpace returned error: %v", err)
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
				nil,
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
				nil,
				testPlaybackSessionID,
				tt.wantStart,
				userID,
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

// A file the extractor cannot open (nonexistent path) must fall back to the
// bounded ffprobe probe exactly as before the keyframe index existed.
func TestResolveHLSActualStart_FallsBackToProbeWithoutContainerIndex(t *testing.T) {
	app := setupTestApp(t)
	defer app.DB.Close()

	app.Wait = &sync.WaitGroup{}
	prober := &stubKeyframeFfprobe{keyframeSec: 591.174}
	app.Ffprobe = prober

	session := &HLSSession{MovieID: 1, CopyVideo: true, StartSec: 600}
	session.setActualStartSec(hlsUnknownActualStart)

	app.Wait.Add(1)
	app.resolveHLSActualStart(context.Background(), hlsActualStartParams{
		Session:           session,
		FilePath:          "/movies/example.mp4",
		Container:         "mp4",
		MovieID:           1,
		StreamIndex:       0,
		RequestedStartSec: 600,
	})
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

// The resolver is advisory: a failure must leave the start unknown rather
// than publish a wrong one, so the client keeps its existing fallback. A
// failed extraction must also persist nothing.
func TestResolveHLSActualStart_LeavesStartUnknownOnFailure(t *testing.T) {
	app := setupTestApp(t)
	defer app.DB.Close()

	app.Wait = &sync.WaitGroup{}
	app.Ffprobe = &stubKeyframeFfprobe{err: errors.New("probe failed")}

	session := &HLSSession{MovieID: 1, CopyVideo: true, StartSec: 600}
	session.setActualStartSec(hlsUnknownActualStart)

	app.Wait.Add(1)
	app.resolveHLSActualStart(context.Background(), hlsActualStartParams{
		Session:           session,
		FilePath:          "/movies/example.mp4",
		Container:         "mp4",
		MovieID:           1,
		StreamIndex:       0,
		RequestedStartSec: 600,
	})
	app.Wait.Wait()

	if got := session.actualStartSec(); got >= 0 {
		t.Fatalf("actual start = %v, want it to stay unknown", got)
	}

	_, err := app.Queries.GetKeyframeIndex(context.Background(), database.GetKeyframeIndexParams{
		MovieID:     1,
		StreamIndex: 0,
	})
	if !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("GetKeyframeIndex error = %v, want sql.ErrNoRows after a failed extraction", err)
	}
}

func explicitAudioRequest(codec helpers.HLSAudioCodec, maxChannels int) *helpers.HLSAudioProfileRequest {
	return &helpers.HLSAudioProfileRequest{Codec: codec, MaxChannels: maxChannels}
}

// Explicit requests are deterministic about codec and channels: they always
// encode with the resolved server-owned profile — the legacy AAC copy gate
// never applies — and the channel maximum is a ceiling resolved against the
// selected source row regardless of its codec.
func TestCreateHLSSession_ExplicitAudioResolution(t *testing.T) {
	tests := []struct {
		name           string
		sourceCodec    string
		sourceProfile  any
		sourceChannels int64
		sourceLayout   any
		request        *helpers.HLSAudioProfileRequest
		wantEncoder    string
		wantChannels   int
		wantBitrate    string
		wantLayout     string
	}{
		{
			name:        "copy-safe AAC 5.1 encodes to ac3 and keeps six channels",
			sourceCodec: "aac", sourceProfile: "LC", sourceChannels: 6, sourceLayout: "5.1(side)",
			request:     explicitAudioRequest(helpers.HLSAudioCodecAC3, 6),
			wantEncoder: "ac3", wantChannels: 6, wantBitrate: "640k", wantLayout: "5.1(side)",
		},
		{
			name:        "copy-safe AAC 5.1 encodes to eac3 and keeps six channels",
			sourceCodec: "aac", sourceProfile: "LC", sourceChannels: 6, sourceLayout: "5.1(side)",
			request:     explicitAudioRequest(helpers.HLSAudioCodecEAC3, 6),
			wantEncoder: "eac3", wantChannels: 6, wantBitrate: "768k", wantLayout: "5.1(side)",
		},
		{
			name:        "DTS-HD MA 5.1 encodes to eac3 with six channels",
			sourceCodec: "dts", sourceProfile: "DTS-HD MA", sourceChannels: 6, sourceLayout: "5.1(side)",
			request:     explicitAudioRequest(helpers.HLSAudioCodecEAC3, 6),
			wantEncoder: "eac3", wantChannels: 6, wantBitrate: "768k", wantLayout: "5.1(side)",
		},
		{
			name:        "DTS-HD MA 5.1 encodes to ac3 with six channels",
			sourceCodec: "dts", sourceProfile: "DTS-HD MA", sourceChannels: 6, sourceLayout: "5.1(side)",
			request:     explicitAudioRequest(helpers.HLSAudioCodecAC3, 6),
			wantEncoder: "ac3", wantChannels: 6, wantBitrate: "640k", wantLayout: "5.1(side)",
		},
		{
			// The URL is deterministic: a source that already matches the
			// requested codec is still encoded in the first version.
			name:        "matching ac3 source still encodes",
			sourceCodec: "ac3", sourceProfile: nil, sourceChannels: 6, sourceLayout: "5.1(side)",
			request:     explicitAudioRequest(helpers.HLSAudioCodecAC3, 6),
			wantEncoder: "ac3", wantChannels: 6, wantBitrate: "640k", wantLayout: "5.1(side)",
		},
		{
			name:        "mono is never upmixed",
			sourceCodec: "aac", sourceProfile: "LC", sourceChannels: 1, sourceLayout: "mono",
			request:     explicitAudioRequest(helpers.HLSAudioCodecAC3, 6),
			wantEncoder: "ac3", wantChannels: 1, wantBitrate: "192k", wantLayout: "mono",
		},
		{
			name:        "stereo is never upmixed",
			sourceCodec: "aac", sourceProfile: "LC", sourceChannels: 2, sourceLayout: "stereo",
			request:     explicitAudioRequest(helpers.HLSAudioCodecEAC3, 6),
			wantEncoder: "eac3", wantChannels: 2, wantBitrate: "384k", wantLayout: "stereo",
		},
		{
			name:        "5.0 keeps its channel count and layout under a maximum of six",
			sourceCodec: "dts", sourceProfile: nil, sourceChannels: 5, sourceLayout: "5.0(side)",
			request:     explicitAudioRequest(helpers.HLSAudioCodecAC3, 6),
			wantEncoder: "ac3", wantChannels: 5, wantBitrate: "640k", wantLayout: "5.0(side)",
		},
		{
			name:        "5.1 downmixes to standard stereo under a maximum of two",
			sourceCodec: "dts", sourceProfile: "DTS-HD MA", sourceChannels: 6, sourceLayout: "5.1(side)",
			request:     explicitAudioRequest(helpers.HLSAudioCodecEAC3, 2),
			wantEncoder: "eac3", wantChannels: 2, wantBitrate: "384k", wantLayout: "stereo",
		},
		{
			name:        "7.1 downmixes to standard 5.1 under a maximum of six",
			sourceCodec: "truehd", sourceProfile: nil, sourceChannels: 8, sourceLayout: "7.1",
			request:     explicitAudioRequest(helpers.HLSAudioCodecEAC3, 6),
			wantEncoder: "eac3", wantChannels: 6, wantBitrate: "768k", wantLayout: "5.1",
		},
		{
			name:        "a row without a stored layout resolves the standard name",
			sourceCodec: "dts", sourceProfile: nil, sourceChannels: 6, sourceLayout: nil,
			request:     explicitAudioRequest(helpers.HLSAudioCodecAC3, 6),
			wantEncoder: "ac3", wantChannels: 6, wantBitrate: "640k", wantLayout: "5.1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			app := setupTestApp(t)
			defer app.DB.Close()

			fake := &fakeFFmpeg{plans: []fakeFFmpegRunPlan{hlsRunPlan(transcodeFixture)}}
			app.FFmpeg = fake

			movieID := insertTestHLSMovieFixture(t, app, "h264", 1080)
			setTestHLSAudioStream(t, app, movieID, tt.sourceCodec, tt.sourceProfile, tt.sourceChannels, tt.sourceLayout)

			session, err := createTestHLSSessionWithAudio(
				app,
				context.Background(),
				movieID,
				helpers.HLS_PROFILE_720P_3MBPS,
				testIntPtr(0),
				tt.request,
				testPlaybackSessionID,
				0,
				false,
			)
			if err != nil {
				t.Fatalf("createHLSSession returned error: %v", err)
			}
			defer cleanupHLSSession(session)

			calls := fake.Calls()
			if len(calls) != 1 {
				t.Fatalf("RunHLS call count = %d, want 1", len(calls))
			}
			if calls[0].CopyAudio {
				t.Fatal("CopyAudio = true, want false: the legacy copy decision must not leak into explicit mode")
			}
			profile := calls[0].AudioProfile
			if profile == nil {
				t.Fatal("RunHLS AudioProfile = nil, want the resolved explicit profile")
			}
			if profile.Encoder != tt.wantEncoder {
				t.Errorf("Encoder = %q, want %q", profile.Encoder, tt.wantEncoder)
			}
			if profile.Channels != tt.wantChannels {
				t.Errorf("Channels = %d, want %d", profile.Channels, tt.wantChannels)
			}
			if profile.Bitrate != tt.wantBitrate {
				t.Errorf("Bitrate = %q, want %q", profile.Bitrate, tt.wantBitrate)
			}
			if profile.ChannelLayout != tt.wantLayout {
				t.Errorf("ChannelLayout = %q, want %q", profile.ChannelLayout, tt.wantLayout)
			}
			if profile.SampleRate != helpers.HLS_EXPLICIT_AUDIO_SAMPLE_RATE {
				t.Errorf("SampleRate = %d, want %d", profile.SampleRate, helpers.HLS_EXPLICIT_AUDIO_SAMPLE_RATE)
			}
			if session.RequestedAudioProfile != tt.request {
				t.Error("session did not store the requested audio profile")
			}
			if session.EffectiveAudioProfile == nil || *session.EffectiveAudioProfile != *profile {
				t.Errorf("session EffectiveAudioProfile = %+v, want %+v", session.EffectiveAudioProfile, profile)
			}
		})
	}
}

// The selected audio row, not the first one, drives channel resolution.
func TestCreateHLSSession_ExplicitAudioUsesSelectedTrack(t *testing.T) {
	app := setupTestApp(t)
	defer app.DB.Close()

	fake := &fakeFFmpeg{plans: []fakeFFmpegRunPlan{hlsRunPlan(transcodeFixture)}}
	app.FFmpeg = fake

	// The fixture's first row stays stereo AAC; the second row is a 5.1 DTS
	// track at absolute index 3, so ordinal 1 must resolve six channels.
	movieID := insertTestHLSMovieFixture(t, app, "h264", 1080)
	_, err := app.DB.Exec(`
		INSERT INTO audio_streams (movie_id, stream_index, codec, bit_rate, channels, channel_layout, language)
		VALUES (?, ?, ?, ?, ?, ?, ?)
	`, movieID, 3, "dts", 1_500_000, 6, "5.1(side)", "spa")
	if err != nil {
		t.Fatalf("insert second audio stream: %v", err)
	}

	session, err := createTestHLSSessionWithAudio(
		app,
		context.Background(),
		movieID,
		helpers.HLS_PROFILE_720P_3MBPS,
		testIntPtr(1),
		explicitAudioRequest(helpers.HLSAudioCodecEAC3, 6),
		testPlaybackSessionID,
		0,
		false,
	)
	if err != nil {
		t.Fatalf("createHLSSession returned error: %v", err)
	}
	defer cleanupHLSSession(session)

	calls := fake.Calls()
	if len(calls) != 1 {
		t.Fatalf("RunHLS call count = %d, want 1", len(calls))
	}
	if calls[0].AudioStreamIndex != 3 {
		t.Fatalf("AudioStreamIndex = %d, want 3", calls[0].AudioStreamIndex)
	}
	profile := calls[0].AudioProfile
	if profile == nil || profile.Channels != 6 || profile.Bitrate != "768k" {
		t.Fatalf("AudioProfile = %+v, want six channels at 768k from the selected row", profile)
	}
}

// A stream without stored channel metadata cannot resolve a safe explicit
// profile: the typed 422 error must fire before FFmpeg is ever started.
func TestCreateHLSSession_ExplicitAudioMissingChannelMetadata(t *testing.T) {
	app := setupTestApp(t)
	defer app.DB.Close()

	fake := &fakeFFmpeg{}
	app.FFmpeg = fake

	movieID := insertTestHLSMovieFixture(t, app, "h264", 1080)
	setTestHLSAudioStream(t, app, movieID, "dts", nil, 0, nil)

	_, err := createTestHLSSessionWithAudio(
		app,
		context.Background(),
		movieID,
		helpers.HLS_PROFILE_720P_3MBPS,
		testIntPtr(0),
		explicitAudioRequest(helpers.HLSAudioCodecAC3, 6),
		testPlaybackSessionID,
		0,
		false,
	)

	var metadataErr *hlsAudioMetadataError
	if !errors.As(err, &metadataErr) {
		t.Fatalf("error = %v, want hlsAudioMetadataError", err)
	}
	if fake.CallCount() != 0 {
		t.Fatalf("RunHLS call count = %d, want 0", fake.CallCount())
	}
}

func TestCreateHLSSession_RejectsExplicitAudioForVideoOnlyMovie(t *testing.T) {
	app := setupTestApp(t)
	defer app.DB.Close()
	app.FFmpeg = &fakeFFmpeg{}

	movieID := insertTestHLSMovieFixture(t, app, "h264", 1080)
	if _, err := app.DB.Exec(`DELETE FROM audio_streams WHERE movie_id = ?`, movieID); err != nil {
		t.Fatalf("delete audio streams: %v", err)
	}

	_, err := createTestHLSSessionWithAudio(
		app,
		context.Background(),
		movieID,
		helpers.HLS_PROFILE_720P_3MBPS,
		nil,
		explicitAudioRequest(helpers.HLSAudioCodecAC3, 2),
		testPlaybackSessionID,
		0,
		false,
	)
	var selectionErr *hlsInvalidAudioSelectionError
	if !errors.As(err, &selectionErr) {
		t.Fatalf("error = %v, want hlsInvalidAudioSelectionError", err)
	}
	if !strings.Contains(selectionErr.PublicMessage, "video-only") {
		t.Fatalf("public message = %q, want video-only rejection", selectionErr.PublicMessage)
	}
}

// A missing Dolby encoder is a server installation problem surfaced before any
// session resources (temp directory, transcode permit) are allocated. Legacy
// requests must stay unaffected by the same build.
func TestCreateHLSSession_ExplicitAudioEncoderUnavailable(t *testing.T) {
	app := setupTestApp(t)
	defer app.DB.Close()

	// A probed build that only provides AAC, like a swapped external binary.
	fake := &fakeFFmpeg{
		plans:        []fakeFFmpegRunPlan{hlsRunPlan(transcodeFixture)},
		capabilities: &ffmpeg.Capabilities{Probed: true, Encoders: map[string]bool{"aac": true}},
	}
	app.FFmpeg = fake

	// Exhaust the only transcode permit: the rejection must come from the
	// encoder gate, which runs before limiter acquisition would block.
	app.HLSTranscodeLimiter = newHLSTranscodeLimiter(1)
	release, err := app.acquireHLSTranscodeSlot(context.Background(), 0)
	if err != nil {
		t.Fatalf("acquireHLSTranscodeSlot: %v", err)
	}

	movieID := insertTestHLSMovieFixture(t, app, "h264", 1080)
	setTestHLSAudioStream(t, app, movieID, "aac", "LC", 6, "5.1(side)")

	_, err = createTestHLSSessionWithAudio(
		app,
		context.Background(),
		movieID,
		helpers.HLS_PROFILE_720P_3MBPS,
		testIntPtr(0),
		explicitAudioRequest(helpers.HLSAudioCodecEAC3, 6),
		testPlaybackSessionID,
		0,
		false,
	)

	var encoderErr *hlsAudioEncoderUnavailableError
	if !errors.As(err, &encoderErr) {
		t.Fatalf("error = %v, want hlsAudioEncoderUnavailableError", err)
	}
	if encoderErr.Encoder != "eac3" {
		t.Fatalf("Encoder = %q, want eac3", encoderErr.Encoder)
	}
	if fake.CallCount() != 0 {
		t.Fatalf("RunHLS call count = %d, want 0", fake.CallCount())
	}

	// The same build keeps serving legacy sessions.
	release()
	session, err := createTestHLSSession(app, context.Background(), movieID, helpers.HLS_PROFILE_720P_3MBPS, testIntPtr(0), testPlaybackSessionID, 0, false)
	if err != nil {
		t.Fatalf("legacy createHLSSession returned error: %v", err)
	}
	cleanupHLSSession(session)
}

// A remux request that fails the safety gate restarts as a transcode; the
// explicit audio profile must survive that fallback.
func TestCreateHLSSession_RemuxFallbackKeepsExplicitAudio(t *testing.T) {
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
	setTestHLSAudioStream(t, app, movieID, "aac", "LC", 6, "5.1(side)")

	request := explicitAudioRequest(helpers.HLSAudioCodecEAC3, 6)
	session, err := createTestHLSSessionWithAudio(
		app,
		context.Background(),
		movieID,
		helpers.HLS_PROFILE_REMUX,
		testIntPtr(0),
		request,
		testPlaybackSessionID,
		0,
		false,
	)
	if err != nil {
		t.Fatalf("createHLSSession returned error: %v", err)
	}
	defer cleanupHLSSession(session)

	if session.CopyVideo {
		t.Fatal("CopyVideo = true, want false after the remux fallback")
	}
	calls := fake.Calls()
	if len(calls) != 2 {
		t.Fatalf("RunHLS call count = %d, want 2", len(calls))
	}
	for i, call := range calls {
		if call.AudioProfile == nil ||
			call.AudioProfile.Encoder != "eac3" ||
			call.AudioProfile.Channels != 6 ||
			call.AudioProfile.Bitrate != "768k" {
			t.Fatalf("RunHLS call %d AudioProfile = %+v, want the explicit eac3 5.1 profile", i, call.AudioProfile)
		}
	}
	if session.RequestedAudioProfile != request {
		t.Fatal("session lost the requested audio profile across the fallback")
	}
}

// Watch rooms stay in legacy audio mode: room creation has no audio-profile
// contract, so the shared session path must pass none through.
func TestGetOrCreateRoomHLSSession_UsesLegacyAudioMode(t *testing.T) {
	app := setupTestApp(t)
	defer app.DB.Close()

	fake := &fakeFFmpeg{plans: []fakeFFmpegRunPlan{hlsRunPlan(transcodeFixture)}}
	app.FFmpeg = fake

	movieID := insertTestHLSMovieFixture(t, app, "h264", 1080)

	session, err := app.GetOrCreateRoomHLSSession(context.Background(), 9, movieID, helpers.HLS_PROFILE_720P_3MBPS, 0, nil, nil)
	if err != nil {
		t.Fatalf("GetOrCreateRoomHLSSession returned error: %v", err)
	}
	defer app.CleanupRoomHLSSession(9)

	calls := fake.Calls()
	if len(calls) != 1 {
		t.Fatalf("RunHLS call count = %d, want 1", len(calls))
	}
	if calls[0].AudioProfile != nil {
		t.Fatalf("room RunHLS AudioProfile = %+v, want nil (legacy mode)", calls[0].AudioProfile)
	}
	if session.RequestedAudioProfile != nil {
		t.Fatal("room session stored a requested audio profile, want legacy mode")
	}
}

// legacyEffectiveHLSAudio backs the effective-audio headers and logs for
// legacy sessions: copied tracks report their stored source values, encoded
// tracks report the stereo AAC fallback.
func TestLegacyEffectiveHLSAudio(t *testing.T) {
	copied := legacyEffectiveHLSAudio(&database.AudioStream{
		Codec:         "aac",
		BitRate:       192000,
		Channels:      6,
		ChannelLayout: sql.NullString{String: "5.1(side)", Valid: true},
		SampleRate:    sql.NullInt64{Int64: 48000, Valid: true},
	}, true)
	want := helpers.HLSResolvedAudioProfile{
		Codec:         helpers.HLSAudioCodecAAC,
		Channels:      6,
		ChannelLayout: "5.1(side)",
		Bitrate:       "192000",
		SampleRate:    48000,
	}
	if *copied != want {
		t.Fatalf("copied profile = %+v, want %+v", *copied, want)
	}

	encoded := legacyEffectiveHLSAudio(&database.AudioStream{Codec: "dts", Channels: 6}, false)
	wantEncoded := helpers.HLSResolvedAudioProfile{
		Codec:         helpers.HLSAudioCodecAAC,
		Encoder:       "aac",
		Channels:      2,
		ChannelLayout: "stereo",
		Bitrate:       helpers.HLS_LEGACY_AUDIO_BITRATE,
	}
	if *encoded != wantEncoded {
		t.Fatalf("encoded profile = %+v, want %+v", *encoded, wantEncoded)
	}
}
