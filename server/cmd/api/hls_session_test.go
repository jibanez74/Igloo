package main

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

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

	_, err = app.createHLSSession(ctx, id, "720p_3mbps", testIntPtr(0), testPlaybackSessionID, 0, false)
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

	_, err = app.createHLSSession(ctx, id, "720p_3mbps", testIntPtr(0), testPlaybackSessionID, 0, false)
	if err == nil {
		t.Fatal("expected error when no video stream rows")
	}
	if !strings.Contains(err.Error(), "no playable video track") {
		t.Errorf("error = %v, want mention of no playable video", err)
	}
}

func TestCreateHLSSession_ErrorsWhenAudioTrackOutOfRange(t *testing.T) {
	app := setupTestApp(t)
	defer app.DB.Close()
	app.Settings = &database.Setting{}

	ctx := context.Background()
	res, err := app.DB.Exec(`
		INSERT INTO movies (title, file_path, file_name, size, container, mime_type, adult, duration)
		VALUES ('One Audio', '/tmp/oneaud.mkv', 'oneaud.mkv', 1, 'mkv', 'video/x-matroska', 0, 100.0)
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
	_, err = app.DB.Exec(`
		INSERT INTO audio_streams (movie_id, stream_index, codec, bit_rate, channels)
		VALUES (?, 1, 'aac', 192000, 2)
	`, movieID)
	if err != nil {
		t.Fatalf("insert audio stream: %v", err)
	}

	_, err = app.createHLSSession(ctx, movieID, "720p_3mbps", testIntPtr(1), testPlaybackSessionID, 0, false)
	if err == nil {
		t.Fatal("expected error when audio track index out of range")
	}
	if !strings.Contains(err.Error(), "out of range") {
		t.Errorf("error = %v, want out of range", err)
	}
}

func TestCreateHLSSession_RemuxSafeStaysOnRemux(t *testing.T) {
	app := setupTestApp(t)
	defer app.DB.Close()
	app.Settings = &database.Setting{}

	fake := &fakeFFmpeg{
		plans: []fakeFFmpegRunPlan{
			{
				WriteFiles: func(outDir string) error {
					return writeTestHLSFixture(outDir, testFMP4Fixture{
						SafeVideo: true,
						Segments:  helpers.HLS_REMUX_PREVALIDATE_SEGMENTS,
					})
				},
			},
		},
	}
	app.FFmpeg = fake

	movieID := insertTestHLSMovieFixture(t, app, "h264", 1080)

	session, err := app.createHLSSession(context.Background(), movieID, helpers.HLS_PROFILE_REMUX, testIntPtr(0), testPlaybackSessionID, 0, false)
	if err != nil {
		t.Fatalf("createHLSSession returned error: %v", err)
	}
	defer cleanupHLSSession(session)

	if session.RequestedProfile != helpers.HLS_PROFILE_REMUX {
		t.Fatalf("RequestedProfile = %q, want remux", session.RequestedProfile)
	}
	if session.EffectiveProfile != helpers.HLS_PROFILE_REMUX {
		t.Fatalf("EffectiveProfile = %q, want remux", session.EffectiveProfile)
	}
	if !session.CopyVideo {
		t.Fatal("CopyVideo = false, want true for safe remux")
	}
	if fake.CallCount() != 1 {
		t.Fatalf("RunHLS call count = %d, want 1", fake.CallCount())
	}

	calls := fake.Calls()
	if len(calls) != 1 {
		t.Fatalf("calls length = %d, want 1", len(calls))
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
			{
				WriteFiles: func(outDir string) error {
					return writeTestHLSFixture(outDir, testFMP4Fixture{
						SafeVideo: false,
						Segments:  helpers.HLS_REMUX_PREVALIDATE_SEGMENTS,
					})
				},
			},
			{
				WriteFiles: func(outDir string) error {
					return writeTestHLSFixture(outDir, testFMP4Fixture{
						SafeVideo: true,
						Segments:  1,
					})
				},
			},
		},
	}
	app.FFmpeg = fake

	movieID := insertTestHLSMovieFixture(t, app, "h264", 1080)
	startSec := 87

	session, err := app.createHLSSession(
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

	if session.RequestedProfile != helpers.HLS_PROFILE_REMUX {
		t.Fatalf("RequestedProfile = %q, want remux", session.RequestedProfile)
	}
	if session.EffectiveProfile != helpers.HLS_PROFILE_1080P_8MBPS {
		t.Fatalf("EffectiveProfile = %q, want %q", session.EffectiveProfile, helpers.HLS_PROFILE_1080P_8MBPS)
	}
	if session.CopyVideo {
		t.Fatal("CopyVideo = true, want false after fallback transcode")
	}
	if session.StartSec != float64(startSec) {
		t.Fatalf("StartSec = %v, want %v", session.StartSec, startSec)
	}
	if fake.CallCount() != 2 {
		t.Fatalf("RunHLS call count = %d, want 2", fake.CallCount())
	}

	calls := fake.Calls()
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
			{
				WriteFiles: func(outDir string) error {
					return writeTestHLSFixture(outDir, testFMP4Fixture{
						SafeVideo: false,
						Segments:  helpers.HLS_REMUX_PREVALIDATE_SEGMENTS,
					})
				},
			},
			{
				WriteFiles: func(outDir string) error {
					return writeTestHLSFixture(outDir, testFMP4Fixture{
						SafeVideo: true,
						Segments:  1,
					})
				},
			},
			{
				WriteFiles: func(outDir string) error {
					return writeTestHLSFixture(outDir, testFMP4Fixture{
						SafeVideo: true,
						Segments:  1,
					})
				},
			},
		},
	}
	app.FFmpeg = fake

	movieID := insertTestHLSMovieFixture(t, app, "h264", 1080)

	firstSession, err := app.createHLSSession(context.Background(), movieID, helpers.HLS_PROFILE_REMUX, testIntPtr(0), testPlaybackSessionID, 0, false)
	if err != nil {
		t.Fatalf("first createHLSSession returned error: %v", err)
	}
	defer cleanupHLSSession(firstSession)

	secondSession, err := app.createHLSSession(context.Background(), movieID, helpers.HLS_PROFILE_REMUX, testIntPtr(0), testPlaybackSessionID, 0, false)
	if err != nil {
		t.Fatalf("second createHLSSession returned error: %v", err)
	}
	defer cleanupHLSSession(secondSession)

	if fake.CallCount() != 3 {
		t.Fatalf("RunHLS call count = %d, want 3", fake.CallCount())
	}

	calls := fake.Calls()
	if calls[0].Profile != helpers.HLS_PROFILE_REMUX {
		t.Fatalf("first RunHLS profile = %q, want remux", calls[0].Profile)
	}
	if calls[1].Profile != helpers.HLS_PROFILE_1080P_8MBPS {
		t.Fatalf("second RunHLS profile = %q, want %q", calls[1].Profile, helpers.HLS_PROFILE_1080P_8MBPS)
	}
	if calls[2].Profile != helpers.HLS_PROFILE_1080P_8MBPS {
		t.Fatalf("third RunHLS profile = %q, want cached fallback %q", calls[2].Profile, helpers.HLS_PROFILE_1080P_8MBPS)
	}
	if secondSession.EffectiveProfile != helpers.HLS_PROFILE_1080P_8MBPS {
		t.Fatalf("second session EffectiveProfile = %q, want %q", secondSession.EffectiveProfile, helpers.HLS_PROFILE_1080P_8MBPS)
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
			{
				WriteFiles: func(outDir string) error {
					return writeTestHLSFixture(outDir, testFMP4Fixture{
						SafeVideo: true,
						Segments:  1,
					})
				},
			},
			{
				WriteFiles: func(outDir string) error {
					return writeTestHLSFixture(outDir, testFMP4Fixture{
						SafeVideo: true,
						Segments:  helpers.HLS_REMUX_PREVALIDATE_SEGMENTS,
					})
				},
			},
		},
	}
	app.FFmpeg = fake

	movieID := insertTestHLSMovieFixture(t, app, "h264", 1080)

	firstSession, err := app.createHLSSession(context.Background(), movieID, helpers.HLS_PROFILE_REMUX, testIntPtr(0), testPlaybackSessionID, 0, false)
	if err != nil {
		t.Fatalf("first createHLSSession returned error: %v", err)
	}
	defer cleanupHLSSession(firstSession)

	if firstSession.EffectiveProfile != helpers.HLS_PROFILE_1080P_8MBPS {
		t.Fatalf("first session EffectiveProfile = %q, want %q", firstSession.EffectiveProfile, helpers.HLS_PROFILE_1080P_8MBPS)
	}

	secondSession, err := app.createHLSSession(context.Background(), movieID, helpers.HLS_PROFILE_REMUX, testIntPtr(0), testPlaybackSessionID, 0, false)
	if err != nil {
		t.Fatalf("second createHLSSession returned error: %v", err)
	}
	defer cleanupHLSSession(secondSession)

	if secondSession.EffectiveProfile != helpers.HLS_PROFILE_REMUX {
		t.Fatalf("second session EffectiveProfile = %q, want %q", secondSession.EffectiveProfile, helpers.HLS_PROFILE_REMUX)
	}
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
			{
				WriteFiles: func(outDir string) error {
					return writeTestHLSFixture(outDir, testFMP4Fixture{
						SafeVideo: true,
						Segments:  1,
					})
				},
			},
		},
	}
	app.FFmpeg = fake

	movieID := insertTestHLSMovieFixture(t, app, "hevc", 2160)

	session, err := app.createHLSSession(context.Background(), movieID, helpers.HLS_PROFILE_REMUX, testIntPtr(0), testPlaybackSessionID, 0, false)
	if err != nil {
		t.Fatalf("createHLSSession returned error: %v", err)
	}
	defer cleanupHLSSession(session)

	if session.EffectiveProfile != helpers.HLS_PROFILE_2160P_16MBPS {
		t.Fatalf("EffectiveProfile = %q, want %q", session.EffectiveProfile, helpers.HLS_PROFILE_2160P_16MBPS)
	}
	if session.CopyVideo {
		t.Fatal("CopyVideo = true, want false for non-H.264 fallback")
	}
	if fake.CallCount() != 1 {
		t.Fatalf("RunHLS call count = %d, want 1", fake.CallCount())
	}

	calls := fake.Calls()
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
			{
				WriteFiles: func(outDir string) error {
					return writeTestHLSFixture(outDir, testFMP4Fixture{
						SafeVideo: true,
						Segments:  1,
					})
				},
			},
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

	session, err := app.createHLSSession(context.Background(), movieID, helpers.HLS_PROFILE_REMUX, testIntPtr(0), testPlaybackSessionID, 0, false)
	if err != nil {
		t.Fatalf("createHLSSession returned error: %v", err)
	}
	defer cleanupHLSSession(session)

	if session.EffectiveProfile != helpers.HLS_PROFILE_1080P_8MBPS {
		t.Fatalf("EffectiveProfile = %q, want %q", session.EffectiveProfile, helpers.HLS_PROFILE_1080P_8MBPS)
	}
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
			{
				WriteFiles: func(outDir string) error {
					return writeTestHLSFixture(outDir, testFMP4Fixture{
						SafeVideo: true,
						Segments:  1,
					})
				},
			},
		},
	}
	app.FFmpeg = fake

	movieID := insertTestHLSMovieFixture(t, app, "h264", 1080)

	session, err := app.createHLSSession(context.Background(), movieID, helpers.HLS_PROFILE_1080P_6MBPS, testIntPtr(0), testPlaybackSessionID, 0, false)
	if err != nil {
		t.Fatalf("createHLSSession returned error: %v", err)
	}
	defer cleanupHLSSession(session)

	if session.RequestedProfile != helpers.HLS_PROFILE_1080P_6MBPS {
		t.Fatalf("RequestedProfile = %q, want %q", session.RequestedProfile, helpers.HLS_PROFILE_1080P_6MBPS)
	}
	if session.EffectiveProfile != helpers.HLS_PROFILE_1080P_6MBPS {
		t.Fatalf("EffectiveProfile = %q, want %q", session.EffectiveProfile, helpers.HLS_PROFILE_1080P_6MBPS)
	}
	if session.CopyVideo {
		t.Fatal("CopyVideo = true, want false for non-remux profile")
	}
	if fake.CallCount() != 1 {
		t.Fatalf("RunHLS call count = %d, want 1", fake.CallCount())
	}
	calls := fake.Calls()
	if calls[0].SourceFrameRate != 23.976 {
		t.Fatalf("RunHLS SourceFrameRate = %v, want 23.976", calls[0].SourceFrameRate)
	}
}

func insertTestHLSMovieFixture(t *testing.T, app *Application, videoCodec string, height int64) int64 {
	t.Helper()

	path := fmt.Sprintf("/tmp/%s-%s.mkv", sanitizeTestPathComponent(t.Name()), sanitizeTestPathComponent(videoCodec))

	result, err := app.DB.Exec(`
		INSERT INTO movies (title, file_path, file_name, size, container, mime_type, adult, duration)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	`,
		t.Name(),
		path,
		filePathBase(path),
		1_000_000,
		"mkv",
		"video/x-matroska",
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

	_, err = app.DB.Exec(`
		INSERT INTO audio_streams (movie_id, stream_index, codec, bit_rate, channels)
		VALUES (?, ?, ?, ?, ?)
	`,
		movieID,
		1,
		"aac",
		192000,
		2,
	)
	if err != nil {
		t.Fatalf("insert audio stream: %v", err)
	}

	return movieID
}

func sanitizeTestPathComponent(value string) string {
	value = strings.ReplaceAll(value, "/", "_")
	value = strings.ReplaceAll(value, " ", "_")
	return value
}

func filePathBase(path string) string {
	lastSlash := strings.LastIndex(path, "/")
	if lastSlash < 0 {
		return path
	}
	return path[lastSlash+1:]
}
