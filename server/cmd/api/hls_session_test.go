package main

import (
	"context"
	"database/sql"
	"errors"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"igloo/cmd/internal/database"
	"igloo/cmd/internal/ffmpeg"
	"igloo/cmd/internal/ffmpeg/fmp4testutil"
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
			key := HLSSessionKey(movieRef(123), "720p_3mbps", &audioTrack, tt.audioProfile, helpers.HLSVideoCodecs{}, testPlaybackSessionID, 40, 456)
			want := "user:456:movie:123:720p_3mbps:audio:2:" + tt.wantMode + ":video:h264:session:" + testPlaybackSessionID + ":start:40"
			if key != want {
				t.Errorf("HLSSessionKey = %q, want %q", key, want)
			}
		})
	}

	base := HLSSessionKey(movieRef(123), "720p_3mbps", &audioTrack, nil, helpers.HLSVideoCodecs{}, testPlaybackSessionID, 40, 456)
	otherAudioTrack := 3
	variants := map[string]string{
		"owner":            HLSSessionKey(movieRef(123), "720p_3mbps", &audioTrack, nil, helpers.HLSVideoCodecs{}, testPlaybackSessionID, 40, 457),
		"movie":            HLSSessionKey(movieRef(124), "720p_3mbps", &audioTrack, nil, helpers.HLSVideoCodecs{}, testPlaybackSessionID, 40, 456),
		"profile":          HLSSessionKey(movieRef(123), "1080p_8mbps", &audioTrack, nil, helpers.HLSVideoCodecs{}, testPlaybackSessionID, 40, 456),
		"audio track":      HLSSessionKey(movieRef(123), "720p_3mbps", &otherAudioTrack, nil, helpers.HLSVideoCodecs{}, testPlaybackSessionID, 40, 456),
		"audio mode":       HLSSessionKey(movieRef(123), "720p_3mbps", &audioTrack, explicitAudioRequest(helpers.HLSAudioCodecAC3, 6), helpers.HLSVideoCodecs{}, testPlaybackSessionID, 40, 456),
		"playback session": HLSSessionKey(movieRef(123), "720p_3mbps", &audioTrack, nil, helpers.HLSVideoCodecs{}, testOtherPlaybackSessionID, 40, 456),
		"start":            HLSSessionKey(movieRef(123), "720p_3mbps", &audioTrack, nil, helpers.HLSVideoCodecs{}, testPlaybackSessionID, 41, 456),
		"media kind":       HLSSessionKey(episodeRef(123), "720p_3mbps", &audioTrack, nil, helpers.HLSVideoCodecs{}, testPlaybackSessionID, 40, 456),
		"video codecs":     HLSSessionKey(movieRef(123), "720p_3mbps", &audioTrack, nil, helpers.HLSVideoCodecs{HEVC: true}, testPlaybackSessionID, 40, 456),
	}
	for dimension, key := range variants {
		if key == base {
			t.Errorf("changing %s did not change the HLS session key", dimension)
		}
	}
	if !strings.Contains(variants["media kind"], ":episode:123:") {
		t.Errorf("episode key %q does not name its media kind", variants["media kind"])
	}

	// Room sessions share the cache, so their keys live in their own namespace.
	roomKey := RoomHLSSessionKey(123)
	if roomKey == base || !strings.HasPrefix(roomKey, "room:") || strings.HasPrefix(base, "room:") {
		t.Errorf("room key %q is not distinct from personal key %q", roomKey, base)
	}
}

func TestCreateHLSSession_RemuxSafeStaysOnRemux(t *testing.T) {
	app := setupTestApp(t)

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
	if calls[1].Profile != helpers.HLS_PROFILE_1080P_4MBPS {
		t.Fatalf("second RunHLS profile = %q, want %q", calls[1].Profile, helpers.HLS_PROFILE_1080P_4MBPS)
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
	if calls[1].Profile != helpers.HLS_PROFILE_1080P_4MBPS {
		t.Fatalf("second RunHLS profile = %q, want %q", calls[1].Profile, helpers.HLS_PROFILE_1080P_4MBPS)
	}
	if calls[2].Profile != helpers.HLS_PROFILE_REMUX {
		t.Fatalf("third RunHLS profile = %q, want remux retry", calls[2].Profile)
	}
}

// A verdict must outlive the process: before persistence, every server restart
// re-paid the multi-second remux preflight for every file (audit H6).
func TestCreateHLSSession_UnsafeVerdictSurvivesRestart(t *testing.T) {
	app := setupTestApp(t)

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
	if calls[0].Profile != helpers.HLS_PROFILE_1080P_4MBPS {
		t.Fatalf("RunHLS profile after restart = %q, want %q", calls[0].Profile, helpers.HLS_PROFILE_1080P_4MBPS)
	}
}

func TestCreateHLSSession_SafeVerdictSurvivesRestart(t *testing.T) {
	app := setupTestApp(t)

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

// The test app is configured for CPU encoding, and the CPU is capped at
// 1080p, so the 4K source's best-fit 2160p fallback is lowered to the richest
// 1080p profile. TestCreateHLSSession_CapsProfileToDeviceAndUpload covers the
// hardware and upload variants.
func TestCreateHLSSession_RemuxNonH264StartsDirectlyWithFallback(t *testing.T) {
	app := setupTestApp(t)

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
	if calls[0].Profile != helpers.HLS_PROFILE_1080P_8MBPS {
		t.Fatalf("RunHLS profile = %q, want %q", calls[0].Profile, helpers.HLS_PROFILE_1080P_8MBPS)
	}
	if calls[0].CopyVideo {
		t.Fatal("RunHLS CopyVideo = true, want false")
	}
}

// The planner holds a session to what the effective device and the server
// upload can carry. The CPU tops out at 1080p; a remux fallback is also held
// to the configured upload with the web client's 0.8 headroom, but an explicit
// request is not, because its bitrate was the viewer's own choice.
func TestCreateHLSSession_CapsProfileToDeviceAndUpload(t *testing.T) {
	nvenc := &ffmpeg.Capabilities{
		Probed:                 true,
		Encoders:               map[string]bool{"h264_nvenc": true, "aac": true},
		H264NVENCRuntimeUsable: true,
	}

	tests := []struct {
		name         string
		device       string
		uploadMbps   float64
		capabilities *ffmpeg.Capabilities
		profile      string
		want         string
	}{
		{
			name:         "hardware fallback keeps 2160p",
			device:       helpers.HARDWARE_ACCELERATION_DEVICE_NVIDIA,
			capabilities: nvenc,
			profile:      helpers.HLS_PROFILE_REMUX,
			want:         helpers.HLS_PROFILE_2160P_16MBPS,
		},
		{
			name:         "fallback fits the server upload",
			device:       helpers.HARDWARE_ACCELERATION_DEVICE_NVIDIA,
			uploadMbps:   6,
			capabilities: nvenc,
			profile:      helpers.HLS_PROFILE_REMUX,
			want:         helpers.HLS_PROFILE_1080P_4MBPS,
		},
		{
			name:       "explicit 2160p on cpu is capped by height only",
			device:     helpers.HARDWARE_ACCELERATION_DEVICE_CPU,
			uploadMbps: 6,
			profile:    helpers.HLS_PROFILE_2160P_16MBPS,
			want:       helpers.HLS_PROFILE_1080P_8MBPS,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			app := setupTestApp(t)
			seedServerPlaybackSettings(t, app, tc.device, tc.uploadMbps)

			fake := &fakeFFmpeg{
				plans: []fakeFFmpegRunPlan{
					hlsRunPlan(transcodeFixture),
				},
				capabilities: tc.capabilities,
			}
			app.FFmpeg = fake

			movieID := insertTestHLSMovieFixture(t, app, "hevc", 2160)

			session, err := createTestHLSSession(app, context.Background(), movieID, tc.profile, testIntPtr(0), testPlaybackSessionID, 0, false)
			if err != nil {
				t.Fatalf("createHLSSession returned error: %v", err)
			}
			defer cleanupHLSSession(session)

			if session.EffectiveProfile != tc.want {
				t.Fatalf("EffectiveProfile = %q, want %q", session.EffectiveProfile, tc.want)
			}
			calls := fake.Calls()
			if len(calls) != 1 {
				t.Fatalf("RunHLS call count = %d, want 1", len(calls))
			}
			if calls[0].Profile != tc.want {
				t.Fatalf("RunHLS profile = %q, want %q", calls[0].Profile, tc.want)
			}
		})
	}
}

func TestCreateHLSSession_RemuxHigh10H264FallsBackToTranscode(t *testing.T) {
	app := setupTestApp(t)

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
	if calls[0].Profile != helpers.HLS_PROFILE_1080P_4MBPS || calls[0].CopyVideo {
		t.Fatalf("RunHLS call = profile %q copyVideo %v, want fallback transcode", calls[0].Profile, calls[0].CopyVideo)
	}
}

// Browsers do not deinterlace, so an interlaced source must be rejected from
// remux and the fallback transcode must carry the deinterlace flag.
func TestCreateHLSSession_InterlacedFallsBackToTranscodeWithDeinterlace(t *testing.T) {
	app := setupTestApp(t)

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
	if calls[0].Profile != helpers.HLS_PROFILE_1080P_4MBPS || calls[0].CopyVideo {
		t.Fatalf("RunHLS call = profile %q copyVideo %v, want fallback transcode", calls[0].Profile, calls[0].CopyVideo)
	}
	if !calls[0].Deinterlace {
		t.Fatal("Deinterlace = false, want true for an interlaced source transcode")
	}
}

func TestCreateHLSSession_NonRemuxProfilesRemainUnchanged(t *testing.T) {
	app := setupTestApp(t)

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

// Each session draws from exactly one pool, decided by what it does with
// video: copied video holds nothing, whatever the audio needs, and an encode
// takes the CPU or hardware pool by its effective encoder. Both pools start
// full here so the test can show which one the session was waiting on.
func TestCreateHLSSession_TranscodeLimiterParticipation(t *testing.T) {
	tests := []struct {
		name         string
		profile      string
		videoOnly    bool
		audioCodec   string
		audioProfile any
		request      *helpers.HLSAudioProfileRequest
		device       string
		wantPool     hlsTranscodePool
	}{
		{name: "copy-only remux", profile: helpers.HLS_PROFILE_REMUX, audioCodec: "aac", audioProfile: "LC"},
		{name: "video-only remux", profile: helpers.HLS_PROFILE_REMUX, videoOnly: true},
		{name: "legacy audio encode", profile: helpers.HLS_PROFILE_REMUX, audioCodec: "dts"},
		{name: "explicit AC-3 encode", profile: helpers.HLS_PROFILE_REMUX, audioCodec: "aac", audioProfile: "LC", request: explicitAudioRequest(helpers.HLSAudioCodecAC3, 6)},
		{name: "explicit E-AC-3 encode", profile: helpers.HLS_PROFILE_REMUX, audioCodec: "aac", audioProfile: "LC", request: explicitAudioRequest(helpers.HLSAudioCodecEAC3, 6)},
		{name: "video encode on the CPU", profile: helpers.HLS_PROFILE_720P_3MBPS, audioCodec: "aac", audioProfile: "LC", wantPool: hlsTranscodePoolCPU},
		{name: "video encode on hardware", profile: helpers.HLS_PROFILE_720P_3MBPS, audioCodec: "aac", audioProfile: "LC", device: helpers.HARDWARE_ACCELERATION_DEVICE_NVIDIA, wantPool: hlsTranscodePoolHardware},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			app := setupTestApp(t)
			fake := &fakeFFmpeg{plans: []fakeFFmpegRunPlan{hlsRunPlan(safeRemuxFixture)}}
			app.FFmpeg = fake
			if tt.device != "" {
				// The fake FFmpeg reports unprobed capabilities, which ResolveHLSDevice
				// trusts, so the configured device is also the effective one.
				current := *app.CurrentSettings()
				current.HardwareAccelerationDevice = sql.NullString{String: tt.device, Valid: true}
				app.SetSettings(&current)
			}

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

			app.HLSCPUTranscodeLimiter = newHLSTranscodeLimiter(hlsTranscodePoolCPU, 1)
			app.HLSHWTranscodeLimiter = newHLSTranscodeLimiter(hlsTranscodePoolHardware, 1)
			releaseCPU := holdHLSTranscodePermit(t, app, hlsTranscodePoolCPU)
			releaseHW := holdHLSTranscodePermit(t, app, hlsTranscodePoolHardware)

			create := func() (*HLSSession, error) {
				return createTestHLSSessionWithAudio(
					app, context.Background(), movieID, tt.profile, audioTrack,
					tt.request, testPlaybackSessionID, 0, false,
				)
			}

			session, err := create()
			if tt.wantPool == hlsTranscodePoolNone {
				if err != nil {
					t.Fatalf("createHLSSession with both pools full: %v", err)
				}
				defer cleanupHLSSession(session)
				if session.TranscodePool != hlsTranscodePoolNone {
					t.Fatalf("TranscodePool = %s, want none", session.TranscodePool)
				}
				return
			}

			var capacityErr *hlsTranscodeCapacityError
			if !errors.As(err, &capacityErr) {
				t.Fatalf("error = %v, want hlsTranscodeCapacityError", err)
			}
			if capacityErr.Pool != tt.wantPool {
				t.Fatalf("capacity error pool = %s, want %s", capacityErr.Pool, tt.wantPool)
			}
			if fake.CallCount() != 0 {
				t.Fatalf("RunHLS call count = %d, want 0", fake.CallCount())
			}

			// A permit in the other pool is no use to this session.
			releaseOwn := releaseCPU
			releaseOther := releaseHW
			if tt.wantPool == hlsTranscodePoolHardware {
				releaseOwn, releaseOther = releaseHW, releaseCPU
			}
			releaseOther()
			_, err = create()
			if !errors.As(err, &capacityErr) {
				t.Fatalf("error after freeing the other pool = %v, want the session still refused", err)
			}

			releaseOwn()
			session, err = create()
			if err != nil {
				t.Fatalf("createHLSSession after freeing its pool: %v", err)
			}
			defer cleanupHLSSession(session)
			if session.TranscodePool != tt.wantPool {
				t.Fatalf("TranscodePool = %s, want %s", session.TranscodePool, tt.wantPool)
			}
			if tt.device != "" {
				calls := fake.Calls()
				if calls[0].HWDevice != tt.device {
					t.Fatalf("RunHLS HWDevice = %q, want %q", calls[0].HWDevice, tt.device)
				}
			}
		})
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

func TestLoadHLSSourceForSession_RejectsNegativeStart(t *testing.T) {
	app := setupTestApp(t)

	movieID := insertTestHLSMovieFixture(t, app, "h264", 1080)

	_, _, err := app.loadHLSSourceForSession(context.Background(), movieRef(movieID), -1)
	if err == nil {
		t.Fatal("loadHLSSourceForSession error = nil, want a rejection")
	}
	if !strings.Contains(err.Error(), "outside media duration") {
		t.Fatalf("error = %v, want it to mention the duration bound", err)
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
		// NULL means ffprobe reported no field order; it must stay eligible for
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

func TestIsBT601Stream(t *testing.T) {
	tagged := func(space string, width, height int64) *database.VideoStream {
		return &database.VideoStream{
			ColorSpace: sql.NullString{String: space, Valid: space != ""},
			Width:      width,
			Height:     height,
		}
	}

	tests := []struct {
		name   string
		stream *database.VideoStream
		want   bool
	}{
		{name: "NTSC tag", stream: tagged("smpte170m", 720, 480), want: true},
		{name: "PAL tag", stream: tagged("bt470bg", 720, 576), want: true},
		// The tag wins over the size either way.
		{name: "BT.601 tag on an HD frame", stream: tagged("smpte170m", 1920, 1080), want: true},
		{name: "BT.709 tag on an SD frame", stream: tagged("bt709", 720, 480)},
		{name: "untagged DVD frame", stream: tagged("", 720, 480), want: true},
		{name: "untagged PAL frame", stream: tagged("", 720, 576), want: true},
		{name: "unknown tag on an SD frame", stream: tagged("unknown", 640, 480), want: true},
		{name: "untagged 720p frame", stream: tagged("", 1280, 720)},
		{name: "untagged 1080p frame", stream: tagged("", 1920, 1080)},
		{name: "untagged frame taller than PAL", stream: tagged("", 1024, 600)},
		{name: "BT.2020 is never BT.601", stream: tagged("bt2020nc", 720, 480)},
		{name: "tags match case-insensitively", stream: tagged(" BT470BG ", 1920, 1080), want: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isBT601Stream(tt.stream)
			if got != tt.want {
				t.Fatalf("isBT601Stream(%q %dx%d) = %v, want %v",
					tt.stream.ColorSpace.String, tt.stream.Width, tt.stream.Height, got, tt.want)
			}
		})
	}
}

// The transcode tags its output BT.709, so an SD source has to reach FFmpeg
// with the conversion asked for; a copy keeps the source's own tags and an
// HDR source goes through the tone-map, which reads the matrix itself.
func TestCreateHLSSession_ConvertsBT601SourcesOnTranscode(t *testing.T) {
	tests := []struct {
		name     string
		width    int64
		height   int64
		space    any
		transfer any
		profile  string
		want     bool
	}{
		{name: "untagged DVD transcode", width: 720, height: 480, profile: helpers.HLS_PROFILE_720P_3MBPS, want: true},
		{name: "tagged PAL transcode", width: 720, height: 576, space: "bt470bg", profile: helpers.HLS_PROFILE_720P_3MBPS, want: true},
		{name: "untagged HD transcode", width: 1920, height: 1080, profile: helpers.HLS_PROFILE_1080P_4MBPS},
		{name: "HDR tone-map", width: 720, height: 480, space: "bt470bg", transfer: "smpte2084", profile: helpers.HLS_PROFILE_720P_3MBPS},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			app := setupTestApp(t)
			fake := &fakeFFmpeg{plans: []fakeFFmpegRunPlan{hlsRunPlan(transcodeFixture)}}
			app.FFmpeg = fake

			movieID := insertTestHLSMovieFixture(t, app, "h264", tt.height)
			_, err := app.DB.Exec(
				`UPDATE video_streams SET width = ?, color_space = ?, color_transfer = ? WHERE movie_id = ?`,
				tt.width, tt.space, tt.transfer, movieID,
			)
			if err != nil {
				t.Fatalf("update video stream: %v", err)
			}

			session, err := createTestHLSSession(app, context.Background(), movieID, tt.profile, testIntPtr(0), testPlaybackSessionID, 0, false)
			if err != nil {
				t.Fatalf("createHLSSession returned error: %v", err)
			}
			defer cleanupHLSSession(session)

			calls := fake.Calls()
			if len(calls) != 1 {
				t.Fatalf("RunHLS call count = %d, want 1", len(calls))
			}
			if calls[0].ConvertBT601 != tt.want {
				t.Fatalf("ConvertBT601 = %v, want %v", calls[0].ConvertBT601, tt.want)
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

// The playlist rate must never exceed the real frame rate: HLSSegmentCount
// survives an underestimate but an overestimate re-advertises the phantom
// final segment.
func TestHLSPlaylistFrameRate(t *testing.T) {
	tests := []struct {
		name         string
		frameRate    float64
		avgFrameRate sql.NullString
		want         float64
	}{
		{name: "matching rates keep the nominal rate", frameRate: 24, avgFrameRate: sql.NullString{String: "24/1", Valid: true}, want: 24},
		{name: "inflated nominal rate yields the average", frameRate: 1000, avgFrameRate: sql.NullString{String: "24000/1001", Valid: true}, want: 24000.0 / 1001.0},
		{name: "inflated average rate yields the nominal", frameRate: 25, avgFrameRate: sql.NullString{String: "50/1", Valid: true}, want: 25},
		{name: "unset average keeps the nominal rate", frameRate: 23.976, avgFrameRate: sql.NullString{}, want: 23.976},
		{name: "unparseable average keeps the nominal rate", frameRate: 23.976, avgFrameRate: sql.NullString{String: "garbage", Valid: true}, want: 23.976},
		{name: "zero average keeps the nominal rate", frameRate: 23.976, avgFrameRate: sql.NullString{String: "0/0", Valid: true}, want: 23.976},
		{name: "unknown nominal rate uses the average", frameRate: 0, avgFrameRate: sql.NullString{String: "30/1", Valid: true}, want: 30},
		{name: "both unknown is unknown", frameRate: 0, avgFrameRate: sql.NullString{}, want: 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := hlsPlaylistFrameRate(&database.VideoStream{FrameRate: tt.frameRate, AvgFrameRate: tt.avgFrameRate})
			if math.Abs(got-tt.want) > 1e-9 {
				t.Fatalf("hlsPlaylistFrameRate(rate=%v, avg=%q) = %v, want %v", tt.frameRate, tt.avgFrameRate.String, got, tt.want)
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
		if got := app.hlsTranscodeRoot(); got != app.Config.TranscodeDir {
			t.Fatalf("hlsTranscodeRoot() = %q, want %q", got, app.Config.TranscodeDir)
		}
	})
}

// Free space is advisory: an unreadable filesystem must not take playback down.
func TestCheckHLSTranscodeSpace_UnreadableFilesystemDoesNotBlockPlayback(t *testing.T) {
	app := setupTestApp(t)

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
				movieRef(movieID),
				helpers.HLS_PROFILE_720P_3MBPS,
				testIntPtr(0),
				nil,
				helpers.HLSVideoCodecs{},
				testPlaybackSessionID,
				tt.requestedStart,
				userID,
			)
			if err != nil {
				t.Fatalf("GetOrCreateHLSSession returned error: %v", err)
			}
			defer cleanupHLSSession(session)

			wantKey := HLSSessionKey(
				movieRef(movieID),
				helpers.HLS_PROFILE_720P_3MBPS,
				testIntPtr(0),
				nil,
				helpers.HLSVideoCodecs{},
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

	fake := &fakeFFmpeg{
		plans: []fakeFFmpegRunPlan{
			hlsRunPlan(transcodeFixture),
		},
	}
	app.FFmpeg = fake

	movieID := insertTestHLSMovieFixture(t, app, "h264", 1080)
	insertTestSecondaryAudioStream(t, app, movieID, "ac3", 448000, 6, "")

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

	fake := &fakeFFmpeg{plans: []fakeFFmpegRunPlan{hlsRunPlan(transcodeFixture)}}
	app.FFmpeg = fake

	// The fixture's first row stays stereo AAC; the second row is a 5.1 DTS
	// track at absolute index 3, so ordinal 1 must resolve six channels.
	movieID := insertTestHLSMovieFixture(t, app, "h264", 1080)
	insertTestSecondaryAudioStream(t, app, movieID, "dts", 1_500_000, 6, "5.1(side)")

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

	_, isAudioMetadataError := errors.AsType[*hlsAudioMetadataError](err)
	if !isAudioMetadataError {
		t.Fatalf("error = %v, want hlsAudioMetadataError", err)
	}
	if fake.CallCount() != 0 {
		t.Fatalf("RunHLS call count = %d, want 0", fake.CallCount())
	}
}

func TestCreateHLSSession_RejectsExplicitAudioForVideoOnlyMovie(t *testing.T) {
	app := setupTestApp(t)
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

	// A probed build that only provides AAC, like a swapped external binary.
	fake := &fakeFFmpeg{
		plans:        []fakeFFmpegRunPlan{hlsRunPlan(transcodeFixture)},
		capabilities: &ffmpeg.Capabilities{Probed: true, Encoders: map[string]bool{"aac": true}},
	}
	app.FFmpeg = fake

	// Exhaust the only transcode permit: the rejection must come from the
	// encoder gate, which runs before limiter acquisition would block.
	app.HLSCPUTranscodeLimiter = newHLSTranscodeLimiter(hlsTranscodePoolCPU, 1)
	release := holdHLSTranscodePermit(t, app, hlsTranscodePoolCPU)

	movieID := insertTestHLSMovieFixture(t, app, "h264", 1080)
	setTestHLSAudioStream(t, app, movieID, "aac", "LC", 6, "5.1(side)")

	_, err := createTestHLSSessionWithAudio(
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

// A remux preflight that fails while the pool is full used to be paid twice:
// the fallback was refused for lack of a permit, and the retry re-planned from
// scratch, so the whole preflight ran again before the request parked. The
// retry must rerun the same plan and go straight to the fallback.
func TestGetOrCreateHLSSession_PreflightFallbackDoesNotRerunPreflight(t *testing.T) {
	app := setupTestApp(t)
	fake := &fakeFFmpeg{
		plans: []fakeFFmpegRunPlan{
			{ExitErr: errors.New("ffmpeg exited before writing remux preflight output")},
			hlsRunPlan(transcodeFixture),
		},
	}
	app.FFmpeg = fake
	withTestHLSTranscodeAcquireWait(t, 10*time.Second)

	app.HLSCPUTranscodeLimiter = newHLSTranscodeLimiter(hlsTranscodePoolCPU, 1)
	release := holdHLSTranscodePermit(t, app, hlsTranscodePoolCPU)

	userID := int64(100)
	movieID := insertTestHLSMovieFixture(t, app, "h264", 1080)

	type createResult struct {
		session *HLSSession
		err     error
	}
	resultCh := make(chan createResult, 1)
	go func() {
		session, _, createErr := app.GetOrCreateHLSSession(
			context.Background(), movieRef(movieID), helpers.HLS_PROFILE_REMUX,
			testIntPtr(0), nil, helpers.HLSVideoCodecs{}, testPlaybackSessionID, 0, userID,
		)
		resultCh <- createResult{session: session, err: createErr}
	}()

	// The remux attempt needs no permit and fails at once; its fallback parks.
	select {
	case created := <-resultCh:
		t.Fatalf("session resolved while the pool was full: session=%v err=%v", created.session, created.err)
	case <-time.After(100 * time.Millisecond):
	}

	release()

	var created createResult
	select {
	case created = <-resultCh:
	case <-time.After(10 * time.Second):
		t.Fatal("GetOrCreateHLSSession never resolved after the permit freed")
	}
	if created.err != nil {
		t.Fatalf("GetOrCreateHLSSession: %v", created.err)
	}
	defer cleanupHLSSession(created.session)

	if created.session.CopyVideo {
		t.Fatal("CopyVideo = true, want the transcode fallback")
	}
	calls := fake.Calls()
	if len(calls) != 2 {
		t.Fatalf("RunHLS call count = %d, want the remux attempt and its fallback only", len(calls))
	}
	if calls[0].Profile != helpers.HLS_PROFILE_REMUX {
		t.Fatalf("first RunHLS profile = %q, want remux", calls[0].Profile)
	}
	if calls[1].Profile != helpers.HLS_PROFILE_1080P_4MBPS {
		t.Fatalf("second RunHLS profile = %q, want %q", calls[1].Profile, helpers.HLS_PROFILE_1080P_4MBPS)
	}

	_, err := app.Queries.GetRemuxSafetyVerdict(context.Background(), database.GetRemuxSafetyVerdictParams{
		MovieID:     movieID,
		StreamIndex: 0,
	})
	if !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("GetRemuxSafetyVerdict error = %v, want sql.ErrNoRows after a transient preflight failure", err)
	}
}

// A file shorter than the preflight sample, or a start near the end of one,
// exits cleanly with fewer than HLS_REMUX_PREVALIDATE_SEGMENTS segments. That
// used to read as a failed preflight and transcode something the browser
// could have played as-is. The shorter sample is validated but not persisted,
// so the file-wide verdict still comes from a full sample.
func TestCreateHLSSession_RemuxCleanExitWithFewerSegmentsIsValidated(t *testing.T) {
	const shortSegments = helpers.HLS_REMUX_PREVALIDATE_SEGMENTS - 2

	readVerdict := func(t *testing.T, app *Application, movieID int64) (database.GetRemuxSafetyVerdictRow, error) {
		t.Helper()
		return app.Queries.GetRemuxSafetyVerdict(context.Background(), database.GetRemuxSafetyVerdictParams{
			MovieID:     movieID,
			StreamIndex: 0,
		})
	}

	t.Run("safe output stays on remux without a persisted verdict", func(t *testing.T) {
		app := setupTestApp(t)
		fake := &fakeFFmpeg{plans: []fakeFFmpegRunPlan{
			hlsRunPlan(fmp4testutil.Fixture{SafeVideo: true, Segments: shortSegments}),
		}}
		app.FFmpeg = fake
		movieID := insertTestHLSMovieFixture(t, app, "h264", 1080)

		session, err := createTestHLSSession(app, context.Background(), movieID, helpers.HLS_PROFILE_REMUX, testIntPtr(0), testPlaybackSessionID, 0, false)
		if err != nil {
			t.Fatalf("createHLSSession returned error: %v", err)
		}
		defer cleanupHLSSession(session)

		if !session.CopyVideo {
			t.Fatal("CopyVideo = false, want the remux kept after a clean short exit")
		}
		if fake.CallCount() != 1 {
			t.Fatalf("RunHLS call count = %d, want 1", fake.CallCount())
		}
		_, err = readVerdict(t, app, movieID)
		if !errors.Is(err, sql.ErrNoRows) {
			t.Fatalf("GetRemuxSafetyVerdict error = %v, want sql.ErrNoRows for a short sample", err)
		}
	})

	t.Run("unsafe output falls back and persists the verdict", func(t *testing.T) {
		app := setupTestApp(t)
		fake := &fakeFFmpeg{plans: []fakeFFmpegRunPlan{
			hlsRunPlan(fmp4testutil.Fixture{SafeVideo: false, Segments: shortSegments}),
			hlsRunPlan(transcodeFixture),
		}}
		app.FFmpeg = fake
		movieID := insertTestHLSMovieFixture(t, app, "h264", 1080)

		session, err := createTestHLSSession(app, context.Background(), movieID, helpers.HLS_PROFILE_REMUX, testIntPtr(0), testPlaybackSessionID, 0, false)
		if err != nil {
			t.Fatalf("createHLSSession returned error: %v", err)
		}
		defer cleanupHLSSession(session)

		if session.CopyVideo {
			t.Fatal("CopyVideo = true, want the transcode fallback")
		}
		if fake.CallCount() != 2 {
			t.Fatalf("RunHLS call count = %d, want 2", fake.CallCount())
		}
		verdict, err := readVerdict(t, app, movieID)
		if err != nil {
			t.Fatalf("GetRemuxSafetyVerdict: %v", err)
		}
		if verdict.Safe {
			t.Fatal("persisted verdict Safe = true, want unsafe")
		}
	})

	t.Run("a clean exit with no segments is still a failed preflight", func(t *testing.T) {
		app := setupTestApp(t)
		fake := &fakeFFmpeg{plans: []fakeFFmpegRunPlan{
			{WriteFiles: func(outDir string) error {
				return os.WriteFile(filepath.Join(outDir, helpers.HLS_INIT_FILENAME), fmp4testutil.BuildInitMP4(), 0o644)
			}},
			hlsRunPlan(transcodeFixture),
		}}
		app.FFmpeg = fake
		movieID := insertTestHLSMovieFixture(t, app, "h264", 1080)

		session, err := createTestHLSSession(app, context.Background(), movieID, helpers.HLS_PROFILE_REMUX, testIntPtr(0), testPlaybackSessionID, 0, false)
		if err != nil {
			t.Fatalf("createHLSSession returned error: %v", err)
		}
		defer cleanupHLSSession(session)

		if session.CopyVideo {
			t.Fatal("CopyVideo = true, want the transcode fallback")
		}
		_, err = readVerdict(t, app, movieID)
		if !errors.Is(err, sql.ErrNoRows) {
			t.Fatalf("GetRemuxSafetyVerdict error = %v, want sql.ErrNoRows for an empty preflight", err)
		}
	})
}

func TestCleanupPersonalHLSSessionsForOwner_KeepsCurrentWindow(t *testing.T) {
	app := setupTestApp(t)

	userID := int64(100)
	audioTrack := 0
	oldKey := HLSSessionKey(movieRef(5), helpers.HLS_PROFILE_REMUX, &audioTrack, nil, helpers.HLSVideoCodecs{}, testPlaybackSessionID, 0, userID)
	keepKey := HLSSessionKey(movieRef(5), helpers.HLS_PROFILE_REMUX, &audioTrack, nil, helpers.HLSVideoCodecs{}, testPlaybackSessionID, 40, userID)
	otherKey := HLSSessionKey(movieRef(5), helpers.HLS_PROFILE_REMUX, &audioTrack, nil, helpers.HLSVideoCodecs{}, testPlaybackSessionID, 80, userID+1)

	app.HLSSessionCache.SetDefault(oldKey, &HLSSession{Media: movieRef(5), FileID: 5, OwnerUserID: userID, PlaybackSession: testPlaybackSessionID, TempDir: t.TempDir()})
	app.HLSSessionCache.SetDefault(keepKey, &HLSSession{Media: movieRef(5), FileID: 5, OwnerUserID: userID, PlaybackSession: testPlaybackSessionID, TempDir: t.TempDir()})
	app.HLSSessionCache.SetDefault(otherKey, &HLSSession{Media: movieRef(5), FileID: 5, OwnerUserID: userID + 1, PlaybackSession: testPlaybackSessionID, TempDir: t.TempDir()})

	removed := app.cleanupPersonalHLSSessionsForOwner(movieRef(5), userID, testPlaybackSessionID, keepKey)
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

	userID := int64(100)
	audioTrack := 0
	key := HLSSessionKey(movieRef(5), helpers.HLS_PROFILE_REMUX, &audioTrack, nil, helpers.HLSVideoCodecs{}, testPlaybackSessionID, 0, userID)
	session := &HLSSession{
		Media:           movieRef(5),
		FileID:          5,
		OwnerUserID:     userID,
		PlaybackSession: testPlaybackSessionID,
		TempDir:         t.TempDir(),
	}
	cleanupStarted, releaseCleanup := blockHLSSessionCleanup(t, session)
	app.HLSSessionCache.Set(key, session, hlsPersonalSessionTTL)
	resultCh := make(chan int, 1)
	go func() {
		resultCh <- app.cleanupPersonalHLSSessionsForOwner(movieRef(5), userID, testPlaybackSessionID, "")
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

	audioTrack := 0
	personalKey := HLSSessionKey(movieRef(5), helpers.HLS_PROFILE_REMUX, &audioTrack, nil, helpers.HLSVideoCodecs{}, testPlaybackSessionID, 0, 100)
	roomKey := RoomHLSSessionKey(9)
	personalSession := &HLSSession{Media: movieRef(5), FileID: 5, OwnerUserID: 100, PlaybackSession: testPlaybackSessionID}

	before := time.Now()
	app.HLSSessionCache.Set(personalKey, personalSession, time.Minute)
	app.RefreshHLSSessionTTL(personalKey, personalSession)
	app.RefreshHLSSessionTTL(roomKey, &HLSSession{Media: movieRef(5), FileID: 5, IsRoom: true})
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

	audioTrack := 0
	key := HLSSessionKey(movieRef(5), helpers.HLS_PROFILE_REMUX, &audioTrack, nil, helpers.HLSVideoCodecs{}, testPlaybackSessionID, 0, 100)
	session := &HLSSession{
		Media:  movieRef(5),
		FileID: 5, OwnerUserID: 100, PlaybackSession: testPlaybackSessionID,
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
	app.HLSMaxPersonalSessionsPerUser = 2

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
				movieRef(movieID),
				helpers.HLS_PROFILE_REMUX,
				testIntPtr(0),
				nil,
				helpers.HLSVideoCodecs{},
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
			movieRef(movieID),
			helpers.HLS_PROFILE_REMUX,
			testIntPtr(0),
			nil,
			helpers.HLSVideoCodecs{},
			playbackSessions[i],
			i*20,
			userID,
		)
		_, atPersonalCapacity := errors.AsType[*hlsPersonalSessionCapacityError](err)
		if !atPersonalCapacity {
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
	app.HLSMaxPersonalSessionsPerUser = 1

	userID := int64(100)
	movieID := int64(5)
	oldKey := HLSSessionKey(movieRef(movieID), helpers.HLS_PROFILE_720P_3MBPS, nil, nil, helpers.HLSVideoCodecs{}, testOtherPlaybackSessionID, 0, userID)
	oldSession := &HLSSession{
		Media:           movieRef(movieID),
		FileID:          movieID,
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
		reservation, err := app.reservePersonalHLSSession(movieRef(movieID), userID, testPlaybackSessionID)
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
	// The victim's output is deleted in the background, off the start path.
	app.Wait.Wait()
	_, err := os.Stat(oldSession.TempDir)
	if !os.IsNotExist(err) {
		t.Fatalf("capacity victim temp dir still exists after cleanup: %v", err)
	}
}

func TestPersonalHLSSessionReservationCommit_UpdatesAccountingBeforeBlockingTeardown(t *testing.T) {
	app := setupTestApp(t)

	userID := int64(100)
	movieID := int64(5)
	oldKey := HLSSessionKey(movieRef(movieID), helpers.HLS_PROFILE_720P_3MBPS, nil, nil, helpers.HLSVideoCodecs{}, testPlaybackSessionID, 0, userID)
	newKey := HLSSessionKey(movieRef(movieID), helpers.HLS_PROFILE_720P_3MBPS, nil, nil, helpers.HLSVideoCodecs{}, testPlaybackSessionID, 30, userID)
	oldSession := &HLSSession{
		Media:           movieRef(movieID),
		FileID:          movieID,
		OwnerUserID:     userID,
		PlaybackSession: testPlaybackSessionID,
		TempDir:         t.TempDir(),
	}
	newSession := &HLSSession{
		Media:           movieRef(movieID),
		FileID:          movieID,
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
		reservation.commit(movieRef(movieID), newKey, newSession)
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
	// The superseded output is deleted in the background, off the start path.
	app.Wait.Wait()
	_, err := os.Stat(oldSession.TempDir)
	if !os.IsNotExist(err) {
		t.Fatalf("superseded session temp dir still exists after cleanup: %v", err)
	}
}

// A start waits for the window it supersedes to stop, since that frees the
// permit and the CPU the new FFmpeg needs, but not for its output to be
// deleted, which the background teardown does with the same single stop.
func TestRetireDisplacedHLSSessions_StopsInLineAndDeletesInTheBackground(t *testing.T) {
	app := setupTestApp(t)

	cmd := exec.Command(os.Args[0], "-test.run=^TestListenForShutdown_HLSProcessHelper$")
	cmd.Env = append(os.Environ(), listenForShutdownHLSProcessHelperEnv+"=1")
	err := cmd.Start()
	if err != nil {
		t.Fatalf("start stand-in FFmpeg: %v", err)
	}
	session := &HLSSession{TempDir: t.TempDir(), Cmd: cmd}
	var cancels atomic.Int32
	session.Cancel = func() {
		cancels.Add(1)
		_ = cmd.Process.Kill()
	}
	go func() {
		waitErr := cmd.Wait()
		session.ExitMu.Lock()
		session.Exited = true
		session.ExitErr = waitErr
		session.ExitMu.Unlock()
	}()

	app.retireDisplacedHLSSessions([]*HLSSession{session, nil})

	exited, _ := session.exitStatus()
	if !exited || !session.stopped() {
		t.Fatalf("retire returned with exited=%v stopped=%v, want a stopped and exited FFmpeg", exited, session.stopped())
	}
	app.Wait.Wait()
	_, err = os.Stat(session.TempDir)
	if !os.IsNotExist(err) {
		t.Fatalf("superseded session temp dir still exists after the background teardown: %v", err)
	}
	if cancels.Load() != 1 {
		t.Fatalf("FFmpeg was canceled %d times, want once", cancels.Load())
	}
}

func TestReclaimIdlePersonalHLSSession_ReleasesLockBeforeTeardown(t *testing.T) {
	app := setupTestApp(t)

	userID := int64(100)
	movieID := int64(5)
	key := HLSSessionKey(movieRef(movieID), helpers.HLS_PROFILE_720P_3MBPS, nil, nil, helpers.HLSVideoCodecs{}, testOtherPlaybackSessionID, 0, userID)
	session := &HLSSession{
		Media:           movieRef(movieID),
		FileID:          movieID,
		OwnerUserID:     userID,
		PlaybackSession: testOtherPlaybackSessionID,
		TempDir:         t.TempDir(),
		TranscodePool:   hlsTranscodePoolCPU,
	}
	cleanupStarted, releaseCleanup := blockHLSSessionCleanup(t, session)
	app.HLSSessionCache.Set(
		key,
		session,
		hlsPersonalSessionTTL-hlsIdlePermitReclaimThreshold-time.Second,
	)
	resultCh := make(chan bool, 1)
	go func() {
		resultCh <- app.reclaimIdlePersonalHLSSessionForOwner(userID, hlsTranscodePoolCPU)
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
	app.HLSMaxPersonalSessionsPerUser = 1
	app.FFmpeg = &fakeFFmpeg{plans: []fakeFFmpegRunPlan{
		{StartErr: errors.New("ffmpeg startup failed")},
		{},
	}}

	movieID := insertTestHLSMovieFixture(t, app, "h264", 1080)
	userID := int64(100)
	_, _, err := app.GetOrCreateHLSSession(
		context.Background(),
		movieRef(movieID),
		helpers.HLS_PROFILE_720P_3MBPS,
		testIntPtr(0),
		nil,
		helpers.HLSVideoCodecs{},
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
		movieRef(movieID),
		helpers.HLS_PROFILE_720P_3MBPS,
		testIntPtr(0),
		nil,
		helpers.HLSVideoCodecs{},
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
	app.HLSMaxPersonalSessionsPerUser = 1

	started := make(chan struct{}, 1)
	continueStart := make(chan struct{})
	app.FFmpeg = &fakeFFmpeg{plans: []fakeFFmpegRunPlan{{
		Started:  started,
		Continue: continueStart,
	}}}

	movieID := insertTestHLSMovieFixture(t, app, "h264", 1080)
	userID := int64(100)
	oldKey := HLSSessionKey(movieRef(movieID), helpers.HLS_PROFILE_720P_3MBPS, testIntPtr(0), nil, helpers.HLSVideoCodecs{}, testOtherPlaybackSessionID, 0, userID)
	otherOwnerKey := HLSSessionKey(movieRef(movieID), helpers.HLS_PROFILE_720P_3MBPS, testIntPtr(0), nil, helpers.HLSVideoCodecs{}, "11111111-1111-4111-8111-111111111111", 0, userID+1)
	roomKey := RoomHLSSessionKey(9)
	app.HLSSessionCache.Set(oldKey, &HLSSession{
		Media:  movieRef(movieID),
		FileID: movieID, OwnerUserID: userID, PlaybackSession: testOtherPlaybackSessionID,
		TempDir: t.TempDir(), Exited: true,
	}, 2*time.Minute)
	app.HLSSessionCache.Set(otherOwnerKey, &HLSSession{
		Media:  movieRef(movieID),
		FileID: movieID, OwnerUserID: userID + 1, PlaybackSession: "11111111-1111-4111-8111-111111111111",
		TempDir: t.TempDir(), Exited: true,
	}, time.Minute)
	app.HLSSessionCache.Set(roomKey, &HLSSession{
		Media:  movieRef(movieID),
		FileID: movieID, IsRoom: true, TempDir: t.TempDir(), Exited: true,
	}, time.Minute)

	type result struct {
		session *HLSSession
		err     error
	}
	resultCh := make(chan result, 1)
	go func() {
		session, _, err := app.GetOrCreateHLSSession(
			context.Background(),
			movieRef(movieID),
			helpers.HLS_PROFILE_720P_3MBPS,
			testIntPtr(0),
			nil,
			helpers.HLSVideoCodecs{},
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
	app.FFmpeg = &fakeFFmpeg{plans: []fakeFFmpegRunPlan{{}}}

	// A completed session sorts first but no longer owns a permit. A later idle,
	// still-running session owns the only permit and must be the reclaim victim.
	app.HLSCPUTranscodeLimiter = newHLSTranscodeLimiter(hlsTranscodePoolCPU, 1)
	release := holdHLSTranscodePermit(t, app, hlsTranscodePoolCPU)
	var releaseOnce sync.Once
	releaseRunningPermit := func() {
		releaseOnce.Do(release)
	}
	defer releaseRunningPermit()

	userID := int64(100)
	movieID := insertTestHLSMovieFixture(t, app, "h264", 1080)
	completedKey := HLSSessionKey(movieRef(movieID), helpers.HLS_PROFILE_720P_3MBPS, testIntPtr(0), nil, helpers.HLSVideoCodecs{}, testOtherPlaybackSessionID, 0, userID)
	runningKey := HLSSessionKey(movieRef(movieID), helpers.HLS_PROFILE_720P_3MBPS, testIntPtr(0), nil, helpers.HLSVideoCodecs{}, "11111111-1111-4111-8111-111111111111", 10, userID)
	completedSession := &HLSSession{
		Media:           movieRef(movieID),
		FileID:          movieID,
		OwnerUserID:     userID,
		PlaybackSession: testOtherPlaybackSessionID,
		TempDir:         t.TempDir(),
		Exited:          true,
	}
	runningSession := &HLSSession{
		Media:           movieRef(movieID),
		FileID:          movieID,
		OwnerUserID:     userID,
		PlaybackSession: "11111111-1111-4111-8111-111111111111",
		TempDir:         t.TempDir(),
		Cancel:          releaseRunningPermit,
		TranscodePool:   hlsTranscodePoolCPU,
	}
	app.HLSSessionCache.Set(completedKey, completedSession, hlsPersonalSessionTTL-hlsIdlePermitReclaimThreshold-2*time.Second)
	app.HLSSessionCache.Set(runningKey, runningSession, hlsPersonalSessionTTL-hlsIdlePermitReclaimThreshold-time.Second)

	session, _, err := app.GetOrCreateHLSSession(context.Background(), movieRef(movieID), helpers.HLS_PROFILE_720P_3MBPS, testIntPtr(0), nil, helpers.HLSVideoCodecs{}, testPlaybackSessionID, 0, userID)
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
	app.FFmpeg = &fakeFFmpeg{plans: []fakeFFmpegRunPlan{{}}}

	// Another device on the same account is actively playing (its TTL is fresh
	// because segment fetches refresh it); a full pool must wait out its budget
	// and then 503 the newcomer instead of killing the active stream.
	withTestHLSTranscodeAcquireWait(t, 50*time.Millisecond)

	app.HLSCPUTranscodeLimiter = newHLSTranscodeLimiter(hlsTranscodePoolCPU, 1)
	holdHLSTranscodePermit(t, app, hlsTranscodePoolCPU)

	userID := int64(100)
	movieID := insertTestHLSMovieFixture(t, app, "h264", 1080)
	activeKey := HLSSessionKey(movieRef(movieID), helpers.HLS_PROFILE_720P_3MBPS, testIntPtr(0), nil, helpers.HLSVideoCodecs{}, testOtherPlaybackSessionID, 0, userID)
	activeSession := &HLSSession{
		Media:           movieRef(movieID),
		FileID:          movieID,
		OwnerUserID:     userID,
		PlaybackSession: testOtherPlaybackSessionID,
		TempDir:         t.TempDir(),
		Exited:          true,
	}
	app.HLSSessionCache.Set(activeKey, activeSession, hlsPersonalSessionTTL)

	_, _, err := app.GetOrCreateHLSSession(context.Background(), movieRef(movieID), helpers.HLS_PROFILE_720P_3MBPS, testIntPtr(0), nil, helpers.HLSVideoCodecs{}, testPlaybackSessionID, 0, userID)
	_, atCapacity := errors.AsType[*hlsTranscodeCapacityError](err)
	if !atCapacity {
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
	app.FFmpeg = &fakeFFmpeg{plans: []fakeFFmpegRunPlan{{}}}

	withTestHLSTranscodeAcquireWait(t, 10*time.Second)

	// The pool is full and nothing is reclaimable: the held permit belongs to a
	// session that is genuinely playing, which is precisely the case reclaim
	// cannot resolve.
	app.HLSCPUTranscodeLimiter = newHLSTranscodeLimiter(hlsTranscodePoolCPU, 1)
	release := holdHLSTranscodePermit(t, app, hlsTranscodePoolCPU)

	userID := int64(100)
	movieID := insertTestHLSMovieFixture(t, app, "h264", 1080)
	activeKey := HLSSessionKey(movieRef(movieID), helpers.HLS_PROFILE_720P_3MBPS, testIntPtr(0), nil, helpers.HLSVideoCodecs{}, testOtherPlaybackSessionID, 0, userID)
	app.HLSSessionCache.Set(activeKey, &HLSSession{
		Media:           movieRef(movieID),
		FileID:          movieID,
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
			context.Background(), movieRef(movieID), helpers.HLS_PROFILE_720P_3MBPS,
			testIntPtr(0), nil, helpers.HLSVideoCodecs{}, testPlaybackSessionID, 0, userID,
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

// The flight runs on the context of the request that started it. A follower
// that joined it while the leader was parked on a full pool used to inherit
// the leader's cancellation when that client went away, and its handler then
// answered a live client with an empty 200.
func TestGetOrCreateHLSSession_FollowerOutlivesCancelledLeader(t *testing.T) {
	app := setupTestApp(t)
	app.FFmpeg = &fakeFFmpeg{plans: []fakeFFmpegRunPlan{{}}}

	withTestHLSTranscodeAcquireWait(t, 10*time.Second)
	app.HLSCPUTranscodeLimiter = newHLSTranscodeLimiter(hlsTranscodePoolCPU, 1)
	release := holdHLSTranscodePermit(t, app, hlsTranscodePoolCPU)

	userID := int64(100)
	movieID := insertTestHLSMovieFixture(t, app, "h264", 1080)
	create := func(ctx context.Context) (*HLSSession, error) {
		session, _, err := app.GetOrCreateHLSSession(
			ctx, movieRef(movieID), helpers.HLS_PROFILE_720P_3MBPS,
			testIntPtr(0), nil, helpers.HLSVideoCodecs{}, testPlaybackSessionID, 0, userID,
		)
		return session, err
	}

	type createResult struct {
		session *HLSSession
		err     error
	}
	leaderCtx, cancelLeader := context.WithCancel(context.Background())
	defer cancelLeader()
	leaderCh := make(chan createResult, 1)
	go func() {
		session, err := create(leaderCtx)
		leaderCh <- createResult{session: session, err: err}
	}()
	// Let the leader park on the full pool before the follower joins its flight.
	time.Sleep(100 * time.Millisecond)

	followerCh := make(chan createResult, 1)
	go func() {
		session, err := create(context.Background())
		followerCh <- createResult{session: session, err: err}
	}()
	time.Sleep(100 * time.Millisecond)

	cancelLeader()
	select {
	case leader := <-leaderCh:
		if !errors.Is(leader.err, context.Canceled) {
			t.Fatalf("leader err = %v, want context.Canceled", leader.err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("the cancelled leader never returned")
	}

	release()

	select {
	case follower := <-followerCh:
		if follower.err != nil {
			t.Fatalf("follower err = %v, want a session", follower.err)
		}
		defer cleanupHLSSession(follower.session)
	case <-time.After(10 * time.Second):
		t.Fatal("the follower never resolved after the permit freed")
	}
}

// Reclaim exists to free a transcode permit. A copy-video session never held
// one, so killing it would interrupt playback and buy nothing.
func TestReclaimIdlePersonalHLSSession_SkipsCopyVideoSessions(t *testing.T) {
	app := setupTestApp(t)

	userID := int64(100)
	key := HLSSessionKey(movieRef(5), helpers.HLS_PROFILE_REMUX, testIntPtr(0), nil, helpers.HLSVideoCodecs{}, testOtherPlaybackSessionID, 0, userID)
	session := &HLSSession{
		Media:           movieRef(5),
		FileID:          5,
		OwnerUserID:     userID,
		PlaybackSession: testOtherPlaybackSessionID,
		TempDir:         t.TempDir(),
		CopyVideo:       true,
		TranscodePool:   hlsTranscodePoolNone,
	}
	app.HLSSessionCache.Set(key, session, hlsPersonalSessionTTL-hlsIdlePermitReclaimThreshold-time.Second)

	if app.reclaimIdlePersonalHLSSessionForOwner(userID, hlsTranscodePoolCPU) {
		t.Fatal("an idle copy-video session was reclaimed")
	}
	if _, cached := app.HLSSessionCache.Get(key); !cached {
		t.Fatal("expected the copy-video session to stay cached")
	}
}

// An audio-only encode holds no permit either, so it is no more a reclaim
// candidate than a pure copy.
func TestReclaimIdlePersonalHLSSession_SkipsCopyVideoAudioEncode(t *testing.T) {
	app := setupTestApp(t)

	userID := int64(100)
	key := HLSSessionKey(movieRef(5), helpers.HLS_PROFILE_REMUX, testIntPtr(0), nil, helpers.HLSVideoCodecs{}, testOtherPlaybackSessionID, 0, userID)
	session := &HLSSession{
		Media:                 movieRef(5),
		FileID:                5,
		OwnerUserID:           userID,
		PlaybackSession:       testOtherPlaybackSessionID,
		TempDir:               t.TempDir(),
		CopyVideo:             true,
		TranscodePool:         hlsTranscodePoolNone,
		EffectiveAudioProfile: &helpers.HLSResolvedAudioProfile{Codec: helpers.HLSAudioCodecAAC, Channels: 2},
	}
	app.HLSSessionCache.Set(key, session, hlsPersonalSessionTTL-hlsIdlePermitReclaimThreshold-time.Second)

	if app.reclaimIdlePersonalHLSSessionForOwner(userID, hlsTranscodePoolCPU) {
		t.Fatal("an idle audio-encoding remux session was reclaimed")
	}
	if _, cached := app.HLSSessionCache.Get(key); !cached {
		t.Fatal("expected the audio-encoding remux session to stay cached")
	}
}

// Reclaim frees a permit for the waiter that asked. Killing an idle hardware
// encode frees nothing a CPU waiter can use, so only the requested pool is
// searched.
func TestReclaimIdlePersonalHLSSession_OnlyReclaimsTheRequestedPool(t *testing.T) {
	app := setupTestApp(t)

	userID := int64(100)
	key := HLSSessionKey(movieRef(5), helpers.HLS_PROFILE_720P_3MBPS, testIntPtr(0), nil, helpers.HLSVideoCodecs{}, testOtherPlaybackSessionID, 0, userID)
	canceled := make(chan struct{})
	session := &HLSSession{
		Media:           movieRef(5),
		FileID:          5,
		OwnerUserID:     userID,
		PlaybackSession: testOtherPlaybackSessionID,
		TempDir:         t.TempDir(),
		TranscodePool:   hlsTranscodePoolHardware,
		Cancel:          func() { close(canceled) },
	}
	app.HLSSessionCache.Set(key, session, hlsPersonalSessionTTL-hlsIdlePermitReclaimThreshold-time.Second)

	if app.reclaimIdlePersonalHLSSessionForOwner(userID, hlsTranscodePoolCPU) {
		t.Fatal("an idle hardware session was reclaimed for a CPU waiter")
	}
	if _, cached := app.HLSSessionCache.Get(key); !cached {
		t.Fatal("expected the hardware session to stay cached")
	}

	if !app.reclaimIdlePersonalHLSSessionForOwner(userID, hlsTranscodePoolHardware) {
		t.Fatal("idle hardware session was not reclaimed for a hardware waiter")
	}
	if _, cached := app.HLSSessionCache.Get(key); cached {
		t.Fatal("reclaimed hardware session remained cached")
	}
	select {
	case <-canceled:
	default:
		t.Fatal("reclaim did not stop the hardware session")
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
		session := &HLSSession{Media: movieRef(5), FileID: 5, TempDir: t.TempDir()}
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
	fake := &fakeFFmpeg{plans: []fakeFFmpegRunPlan{hlsRunPlan(safeRemuxFixture)}}
	app.FFmpeg = fake

	movieID := insertTestHLSMovieFixture(t, app, "h264", 1080)
	userID := int64(100)

	created, key, err := app.GetOrCreateHLSSession(
		context.Background(),
		movieRef(movieID),
		helpers.HLS_PROFILE_REMUX,
		testIntPtr(0),
		nil,
		helpers.HLSVideoCodecs{},
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
		movieRef(movieID),
		helpers.HLS_PROFILE_REMUX,
		testIntPtr(0),
		nil,
		helpers.HLSVideoCodecs{},
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
			context.Background(), movieRef(movieID), helpers.HLS_PROFILE_REMUX,
			testIntPtr(0), nil, helpers.HLSVideoCodecs{}, testPlaybackSessionID, 0, userID,
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
				context.Background(), movieRef(movieID), helpers.HLS_PROFILE_REMUX,
				testIntPtr(0), nil, helpers.HLSVideoCodecs{}, testPlaybackSessionID, 0, userID,
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
		context.Background(), movieRef(movieID), helpers.HLS_PROFILE_720P_3MBPS,
		testIntPtr(0), nil, helpers.HLSVideoCodecs{}, testPlaybackSessionID, 0, userID,
	)
	if err != nil {
		t.Fatalf("legacy creation error: %v", err)
	}
	defer cleanupHLSSession(legacySession)

	// Switching to an explicit profile in the same playback session starts a
	// new FFmpeg process and supersedes the legacy window.
	explicitSession, explicitKey, err := app.GetOrCreateHLSSession(
		context.Background(), movieRef(movieID), helpers.HLS_PROFILE_720P_3MBPS,
		testIntPtr(0), eac3Surround, helpers.HLSVideoCodecs{}, testPlaybackSessionID, 0, userID,
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
		context.Background(), movieRef(movieID), helpers.HLS_PROFILE_720P_3MBPS,
		testIntPtr(0), &helpers.HLSAudioProfileRequest{Codec: helpers.HLSAudioCodecEAC3, MaxChannels: 6},
		helpers.HLSVideoCodecs{},
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
		context.Background(), movieRef(movieID), helpers.HLS_PROFILE_720P_3MBPS,
		testIntPtr(0), eac3Surround, helpers.HLSVideoCodecs{}, testOtherPlaybackSessionID, 0, userID,
	)
	if err != nil {
		t.Fatalf("other playback session creation error: %v", err)
	}
	defer cleanupHLSSession(otherSession)

	if _, cached := app.HLSSessionCache.Get(explicitKey); !cached {
		t.Fatal("explicit session was removed by a different playback session's creation")
	}
}

// With every slot already held by an in-flight reservation, evicting cached
// sessions cannot make room. The eviction loop used to run anyway: it killed
// each of the owner's cached sessions in turn and then refused the request.
func TestReservePersonalHLSSession_ReservationsAtLimitKeepCachedSessions(t *testing.T) {
	app := setupTestApp(t)
	app.HLSMaxPersonalSessionsPerUser = 2

	userID := int64(100)
	keys := []string{
		HLSSessionKey(movieRef(5), helpers.HLS_PROFILE_720P_3MBPS, nil, nil, helpers.HLSVideoCodecs{}, testPlaybackSessionID, 0, userID),
		HLSSessionKey(movieRef(6), helpers.HLS_PROFILE_720P_3MBPS, nil, nil, helpers.HLSVideoCodecs{}, testPlaybackSessionID, 0, userID),
	}
	for i, key := range keys {
		session := &HLSSession{
			Media:           movieRef(int64(5 + i)),
			FileID:          int64(5 + i),
			OwnerUserID:     userID,
			PlaybackSession: testPlaybackSessionID,
			TempDir:         t.TempDir(),
			Exited:          true,
		}
		defer cleanupHLSSession(session)
		app.HLSSessionCache.Set(key, session, hlsPersonalSessionTTL)
	}
	app.PersonalHLSReservations[userID] = 2

	_, err := app.reservePersonalHLSSession(movieRef(7), userID, "11111111-2222-3333-4444-555555555555")
	_, atPersonalCapacity := errors.AsType[*hlsPersonalSessionCapacityError](err)
	if !atPersonalCapacity {
		t.Fatalf("error = %v, want personal-session capacity error", err)
	}

	for _, key := range keys {
		if _, cached := app.HLSSessionCache.Get(key); !cached {
			t.Fatalf("cached session %q was evicted although eviction could not make room", key)
		}
	}
	app.PersonalHLSMu.Lock()
	reserved := app.PersonalHLSReservations[userID]
	app.PersonalHLSMu.Unlock()
	if reserved != 2 {
		t.Fatalf("pending reservations = %d, want 2", reserved)
	}
}

// A failed session answered every retry of its URL with the same failure and
// never expired, because each retry refreshed it. The next manifest request
// must replace it, while a stop Igloo asked for and a clean exit stay cached.
func TestGetOrCreateHLSSession_ReplacesOnlyFailedSessions(t *testing.T) {
	cases := []struct {
		name         string
		exitErr      error
		expectedStop bool
		wantReplaced bool
	}{
		{name: "ffmpeg failed", exitErr: errors.New("exit status 1"), wantReplaced: true},
		{name: "stop igloo asked for", exitErr: errors.New("signal: killed"), expectedStop: true},
		{name: "clean exit", exitErr: nil},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			app := setupTestApp(t)
			fake := &fakeFFmpeg{plans: []fakeFFmpegRunPlan{hlsRunPlan(transcodeFixture)}}
			app.FFmpeg = fake

			movieID := insertTestHLSMovieFixture(t, app, "h264", 1080)
			userID := int64(100)
			key := HLSSessionKey(movieRef(movieID), helpers.HLS_PROFILE_720P_3MBPS, testIntPtr(0), nil, helpers.HLSVideoCodecs{}, testPlaybackSessionID, 0, userID)
			oldDir := t.TempDir()
			old := &HLSSession{
				Media:           movieRef(movieID),
				FileID:          movieID,
				OwnerUserID:     userID,
				PlaybackSession: testPlaybackSessionID,
				TempDir:         oldDir,
				Exited:          true,
				ExitErr:         tc.exitErr,
				ExpectedStop:    tc.expectedStop,
			}
			app.HLSSessionCache.Set(key, old, hlsPersonalSessionTTL)

			session, gotKey, err := app.GetOrCreateHLSSession(
				context.Background(),
				movieRef(movieID),
				helpers.HLS_PROFILE_720P_3MBPS,
				testIntPtr(0),
				nil,
				helpers.HLSVideoCodecs{},
				testPlaybackSessionID,
				0,
				userID,
			)
			if err != nil {
				t.Fatalf("GetOrCreateHLSSession error: %v", err)
			}
			defer cleanupHLSSession(session)
			if gotKey != key {
				t.Fatalf("key = %q, want %q", gotKey, key)
			}

			if !tc.wantReplaced {
				if session != old {
					t.Fatal("cached session was replaced, want it reused")
				}
				if fake.CallCount() != 0 {
					t.Fatalf("RunHLS call count = %d, want 0", fake.CallCount())
				}
				return
			}

			if session == old {
				t.Fatal("failed session was served again, want a replacement")
			}
			if fake.CallCount() != 1 {
				t.Fatalf("RunHLS call count = %d, want 1", fake.CallCount())
			}
			raw, ok := app.HLSSessionCache.Get(key)
			if !ok || raw != session {
				t.Fatal("replacement is not the cached session")
			}
			if _, statErr := os.Stat(oldDir); !os.IsNotExist(statErr) {
				t.Fatalf("failed session temp dir still exists (stat error %v)", statErr)
			}
		})
	}
}

// A seek whose replacement fails to plan keeps the window that was playing:
// the start retires superseded windows only after its plan succeeds. Once a
// replacement does plan, it retires the old window as before.
func TestGetOrCreateHLSSession_FailedReplacementKeepsSupersededWindow(t *testing.T) {
	app := setupTestApp(t)
	app.HLSMaxPersonalSessionsPerUser = 1
	fake := &fakeFFmpeg{plans: []fakeFFmpegRunPlan{
		hlsRunPlan(transcodeFixture),
		hlsRunPlan(transcodeFixture),
	}}
	app.FFmpeg = fake

	movieID := insertTestHLSMovieFixture(t, app, "h264", 1080)
	userID := int64(100)

	oldSession, oldKey, err := app.GetOrCreateHLSSession(
		context.Background(), movieRef(movieID), helpers.HLS_PROFILE_720P_3MBPS,
		testIntPtr(0), nil, helpers.HLSVideoCodecs{}, testPlaybackSessionID, 0, userID,
	)
	if err != nil {
		t.Fatalf("initial creation error: %v", err)
	}
	defer cleanupHLSSession(oldSession)

	// An audio ordinal the movie does not have fails planning.
	_, _, err = app.GetOrCreateHLSSession(
		context.Background(), movieRef(movieID), helpers.HLS_PROFILE_720P_3MBPS,
		testIntPtr(9), nil, helpers.HLSVideoCodecs{}, testPlaybackSessionID, 600, userID,
	)
	if _, invalid := errors.AsType[*hlsInvalidAudioSelectionError](err); !invalid {
		t.Fatalf("replacement error = %v, want an invalid audio selection", err)
	}
	if raw, cached := app.HLSSessionCache.Get(oldKey); !cached || raw != oldSession {
		t.Fatal("a replacement that failed to plan ended the window that was playing")
	}
	if oldSession.stopped() {
		t.Fatal("a replacement that failed to plan stopped the playing window's FFmpeg")
	}
	app.PersonalHLSMu.Lock()
	reserved := app.PersonalHLSReservations[userID]
	app.PersonalHLSMu.Unlock()
	if reserved != 0 {
		t.Fatalf("pending reservations after the failed replacement = %d, want 0", reserved)
	}

	// At a per-user cap of one, a seek that plans replaces the window rather
	// than being refused for it.
	newSession, newKey, err := app.GetOrCreateHLSSession(
		context.Background(), movieRef(movieID), helpers.HLS_PROFILE_720P_3MBPS,
		testIntPtr(0), nil, helpers.HLSVideoCodecs{}, testPlaybackSessionID, 600, userID,
	)
	if err != nil {
		t.Fatalf("replacement creation error: %v", err)
	}
	defer cleanupHLSSession(newSession)
	if newKey == oldKey {
		t.Fatalf("replacement key %q must differ from the original", newKey)
	}
	if _, cached := app.HLSSessionCache.Get(oldKey); cached {
		t.Fatal("the superseded window stayed cached after its replacement started")
	}
	if !oldSession.stopped() {
		t.Fatal("the superseded window's FFmpeg was not stopped")
	}
	if fake.CallCount() != 2 {
		t.Fatalf("RunHLS call count = %d, want 2", fake.CallCount())
	}
}

// Superseded windows do not count against the per-user cap, so admitting a
// seek never evicts the owner's other playback for room the seek frees itself.
func TestReservePersonalHLSSession_SupersededWindowsDoNotCountAgainstCap(t *testing.T) {
	app := setupTestApp(t)
	app.HLSMaxPersonalSessionsPerUser = 2

	userID := int64(100)
	sameKey := HLSSessionKey(movieRef(5), helpers.HLS_PROFILE_720P_3MBPS, nil, nil, helpers.HLSVideoCodecs{}, testPlaybackSessionID, 0, userID)
	otherKey := HLSSessionKey(movieRef(6), helpers.HLS_PROFILE_720P_3MBPS, nil, nil, helpers.HLSVideoCodecs{}, testOtherPlaybackSessionID, 0, userID)
	for key, session := range map[string]*HLSSession{
		sameKey:  {Media: movieRef(5), FileID: 5, OwnerUserID: userID, PlaybackSession: testPlaybackSessionID, TempDir: t.TempDir(), Exited: true},
		otherKey: {Media: movieRef(6), FileID: 6, OwnerUserID: userID, PlaybackSession: testOtherPlaybackSessionID, TempDir: t.TempDir(), Exited: true},
	} {
		defer cleanupHLSSession(session)
		app.HLSSessionCache.Set(key, session, hlsPersonalSessionTTL)
	}

	reservation, err := app.reservePersonalHLSSession(movieRef(5), userID, testPlaybackSessionID)
	if err != nil {
		t.Fatalf("reservePersonalHLSSession returned error: %v", err)
	}
	reservation.release()

	for _, key := range []string{sameKey, otherKey} {
		if _, cached := app.HLSSessionCache.Get(key); !cached {
			t.Fatalf("admission removed %q", key)
		}
	}
}

// Two of one owner's clients seeking at once each leave their own window outside the
// cap, and neither counts the other's superseded window beside the
// reservation that stands in for it, so neither evicts the other's window.
func TestReservePersonalHLSSession_ConcurrentReplacementsKeepEachOthersWindows(t *testing.T) {
	app := setupTestApp(t)
	app.HLSMaxPersonalSessionsPerUser = 2

	userID := int64(100)
	firstKey := HLSSessionKey(movieRef(5), helpers.HLS_PROFILE_720P_3MBPS, nil, nil, helpers.HLSVideoCodecs{}, testPlaybackSessionID, 0, userID)
	secondKey := HLSSessionKey(movieRef(6), helpers.HLS_PROFILE_720P_3MBPS, nil, nil, helpers.HLSVideoCodecs{}, testOtherPlaybackSessionID, 0, userID)
	for key, session := range map[string]*HLSSession{
		firstKey:  {Media: movieRef(5), FileID: 5, OwnerUserID: userID, PlaybackSession: testPlaybackSessionID, TempDir: t.TempDir(), Exited: true},
		secondKey: {Media: movieRef(6), FileID: 6, OwnerUserID: userID, PlaybackSession: testOtherPlaybackSessionID, TempDir: t.TempDir(), Exited: true},
	} {
		defer cleanupHLSSession(session)
		app.HLSSessionCache.Set(key, session, hlsPersonalSessionTTL)
	}

	first, err := app.reservePersonalHLSSession(movieRef(5), userID, testPlaybackSessionID)
	if err != nil {
		t.Fatalf("first replacement reservation error: %v", err)
	}
	second, err := app.reservePersonalHLSSession(movieRef(6), userID, testOtherPlaybackSessionID)
	if err != nil {
		t.Fatalf("second replacement reservation error: %v", err)
	}

	// Both plans fail.
	first.release()
	second.release()

	for _, key := range []string{firstKey, secondKey} {
		if _, cached := app.HLSSessionCache.Get(key); !cached {
			t.Fatalf("overlapping replacements that failed removed %q", key)
		}
	}
	app.PersonalHLSMu.Lock()
	reserved, replacing := len(app.PersonalHLSReservations), len(app.PersonalHLSReplacing)
	app.PersonalHLSMu.Unlock()
	if reserved != 0 || replacing != 0 {
		t.Fatalf("after release: %d owners with reservations, %d replaced windows; want 0 and 0", reserved, replacing)
	}
}

// A seek stops the window it supersedes before its own FFmpeg asks for a
// permit. Retiring the window only at commit would leave the pool full: the
// window is fresh, so reclaim skips it, and the seek would time out with 503.
func TestGetOrCreateHLSSession_SeekFreesSupersededPermitBeforeStarting(t *testing.T) {
	app := setupTestApp(t)
	app.FFmpeg = &fakeFFmpeg{plans: []fakeFFmpegRunPlan{hlsRunPlan(transcodeFixture)}}
	withTestHLSTranscodeAcquireWait(t, 50*time.Millisecond)

	app.HLSCPUTranscodeLimiter = newHLSTranscodeLimiter(hlsTranscodePoolCPU, 1)
	release := holdHLSTranscodePermit(t, app, hlsTranscodePoolCPU)
	var releaseOnce sync.Once
	releasePermit := func() { releaseOnce.Do(release) }
	defer releasePermit()

	userID := int64(100)
	movieID := insertTestHLSMovieFixture(t, app, "h264", 1080)
	oldKey := HLSSessionKey(movieRef(movieID), helpers.HLS_PROFILE_720P_3MBPS, testIntPtr(0), nil, helpers.HLSVideoCodecs{}, testPlaybackSessionID, 0, userID)
	oldSession := &HLSSession{
		Media:           movieRef(movieID),
		FileID:          movieID,
		OwnerUserID:     userID,
		PlaybackSession: testPlaybackSessionID,
		TempDir:         t.TempDir(),
		Cancel:          releasePermit,
		TranscodePool:   hlsTranscodePoolCPU,
	}
	app.HLSSessionCache.Set(oldKey, oldSession, hlsPersonalSessionTTL)

	session, _, err := app.GetOrCreateHLSSession(
		context.Background(), movieRef(movieID), helpers.HLS_PROFILE_720P_3MBPS,
		testIntPtr(0), nil, helpers.HLSVideoCodecs{}, testPlaybackSessionID, 600, userID,
	)
	if err != nil {
		t.Fatalf("seek on a full pool returned error: %v", err)
	}
	defer cleanupHLSSession(session)
	if _, cached := app.HLSSessionCache.Get(oldKey); cached {
		t.Fatal("the superseded window stayed cached after the seek")
	}
}

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

	session, err := app.GetOrCreateRoomHLSSession(context.Background(), roomID, 999, "720p_3mbps", 0, nil, nil)
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

	_, err = app.GetOrCreateRoomHLSSession(context.Background(), 1, movieID, "720p_3mbps", 0, nil, nil)
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

	_, err := app.GetOrCreateRoomHLSSession(context.Background(), roomID, 999, "1080p_8mbps", 0, nil, nil)
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
		context.Background(),
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

	movie, err := app.Queries.GetMovieByID(context.Background(), movieID)
	if err != nil {
		t.Fatalf("load movie: %v", err)
	}
	audioStreams, err := app.Queries.GetAudioStreamsByMovieID(context.Background(), movieID)
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
		context.Background(),
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
	personalKeyA := HLSSessionKey(movieRef(1), "720p_3mbps", nil, nil, helpers.HLSVideoCodecs{}, "session-a", 0, 100)
	personalKeyB := HLSSessionKey(movieRef(2), "720p_3mbps", nil, nil, helpers.HLSVideoCodecs{}, "session-b", 0, 100)

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

	session, err := app.GetOrCreateRoomHLSSession(context.Background(), roomID, movieID, helpers.HLS_PROFILE_720P_3MBPS, 0, nil, nil)
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

func TestIsRemuxCandidate(t *testing.T) {
	hevc := helpers.HLSVideoCodecs{HEVC: true}
	main10 := database.VideoStream{
		Codec:         "hevc",
		CodecProfile:  sql.NullString{String: "Main 10", Valid: true},
		PixelFormat:   sql.NullString{String: "yuv420p10le", Valid: true},
		ColorTransfer: sql.NullString{String: "smpte2084", Valid: true},
	}
	withDolbyVision := func(profile, compatibility int64) database.VideoStream {
		stream := main10
		stream.DvProfile = sql.NullInt64{Int64: profile, Valid: true}
		stream.DvBlSignalCompatibilityID = sql.NullInt64{Int64: compatibility, Valid: true}
		return stream
	}
	with := func(edit func(*database.VideoStream)) database.VideoStream {
		stream := main10
		edit(&stream)
		return stream
	}

	tests := []struct {
		name        string
		stream      database.VideoStream
		videoCodecs helpers.HLSVideoCodecs
		want        bool
	}{
		{name: "H.264 stays on the browser-safe rules", stream: database.VideoStream{Codec: "h264"}, want: true},
		{name: "H.264 High 10 is still refused with hevc", stream: database.VideoStream{Codec: "h264", CodecProfile: sql.NullString{String: "High 10", Valid: true}}, videoCodecs: hevc},
		{name: "HEVC without the request", stream: main10},
		{name: "HEVC Main 8-bit without the request", stream: with(func(s *database.VideoStream) {
			s.CodecProfile.String = "Main"
			s.PixelFormat.String = "yuv420p"
		})},
		{name: "HEVC Main 10 HDR10", stream: main10, videoCodecs: hevc, want: true},
		{name: "HEVC Main 8-bit", stream: with(func(s *database.VideoStream) {
			s.CodecProfile.String = "Main"
			s.PixelFormat.String = "yuv420p"
		}), videoCodecs: hevc, want: true},
		{name: "HEVC with nothing probed", stream: database.VideoStream{Codec: "hevc"}, videoCodecs: hevc, want: true},
		{name: "HEVC Rext", stream: with(func(s *database.VideoStream) { s.CodecProfile.String = "Rext" }), videoCodecs: hevc},
		{name: "HEVC 4:2:2", stream: with(func(s *database.VideoStream) { s.PixelFormat.String = "yuv422p10le" }), videoCodecs: hevc},
		{name: "HEVC 12-bit", stream: with(func(s *database.VideoStream) { s.BitDepth = sql.NullInt64{Int64: 12, Valid: true} }), videoCodecs: hevc},
		{name: "HEVC interlaced", stream: with(func(s *database.VideoStream) { s.FieldOrder = sql.NullString{String: "tt", Valid: true} }), videoCodecs: hevc},
		{name: "Dolby Vision 7 on an HDR10 base layer", stream: withDolbyVision(7, 6), videoCodecs: hevc, want: true},
		{name: "Dolby Vision 8.1", stream: withDolbyVision(8, 1), videoCodecs: hevc, want: true},
		{name: "Dolby Vision 8.4 HLG", stream: withDolbyVision(8, 4), videoCodecs: hevc, want: true},
		{name: "Dolby Vision 5 has no base layer", stream: withDolbyVision(5, 0), videoCodecs: hevc},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, reason := isRemuxCandidate(&tt.stream, tt.videoCodecs)
			if got != tt.want {
				t.Fatalf("isRemuxCandidate = %t (%s), want %t", got, reason, tt.want)
			}
			if !got && reason == "" {
				t.Fatal("a refusal carries no reason")
			}
		})
	}
}

func setTestVideoStreamColumns(t *testing.T, app *Application, movieID int64, profile, pixelFormat string, dvProfile, dvCompatibility sql.NullInt64) {
	t.Helper()
	_, err := app.DB.Exec(`
		UPDATE video_streams
		SET codec_profile = ?, pixel_format = ?, color_transfer = 'smpte2084', dv_profile = ?, dv_bl_signal_compatibility_id = ?
		WHERE movie_id = ?
	`, profile, pixelFormat, dvProfile, dvCompatibility, movieID)
	if err != nil {
		t.Fatalf("update video stream: %v", err)
	}
}

func TestCreateHLSSession_HEVCRemuxCopiesForADeclaringClient(t *testing.T) {
	app := setupTestApp(t)
	fake := &fakeFFmpeg{plans: []fakeFFmpegRunPlan{
		hlsRunPlan(fmp4testutil.Fixture{SafeVideo: true, HEVC: true, Segments: helpers.HLS_REMUX_PREVALIDATE_SEGMENTS}),
	}}
	app.FFmpeg = fake
	movieID := insertTestHLSMovieFixture(t, app, "hevc", 2160)
	setTestVideoStreamColumns(t, app, movieID, "Main 10", "yuv420p10le", sql.NullInt64{Int64: 7, Valid: true}, sql.NullInt64{Int64: 6, Valid: true})

	session, err := createTestHLSSessionWithVideoCodecs(app, context.Background(), movieID, helpers.HLS_PROFILE_REMUX, testIntPtr(0), helpers.HLSVideoCodecs{HEVC: true})
	if err != nil {
		t.Fatalf("createHLSSession returned error: %v", err)
	}
	defer cleanupHLSSession(session)

	if !session.CopyVideo || session.EffectiveProfile != helpers.HLS_PROFILE_REMUX {
		t.Fatalf("CopyVideo = %t, EffectiveProfile = %q; want a remux", session.CopyVideo, session.EffectiveProfile)
	}
	if session.TranscodePool != hlsTranscodePoolNone {
		t.Fatalf("TranscodePool = %s, want none: a copy holds no encode permit", session.TranscodePool)
	}
	calls := fake.Calls()
	if len(calls) != 1 {
		t.Fatalf("RunHLS call count = %d, want 1", len(calls))
	}
	if !calls[0].CopyVideo || calls[0].CopyVideoTag != "hvc1" {
		t.Fatalf("RunHLS CopyVideo = %t, CopyVideoTag = %q; want a copy tagged hvc1", calls[0].CopyVideo, calls[0].CopyVideoTag)
	}
	if calls[0].TonemapHDR {
		t.Fatal("RunHLS TonemapHDR = true; a copy keeps HDR")
	}
}

func TestCreateHLSSession_HEVCRemuxFallsBack(t *testing.T) {
	tests := []struct {
		name            string
		profile         string
		dvCompatibility sql.NullInt64
		videoCodecs     helpers.HLSVideoCodecs
		output          fmp4testutil.Fixture
	}{
		{
			name:    "without video_codecs",
			profile: "Main 10",
		},
		{
			name:        "Rext",
			profile:     "Rext",
			videoCodecs: helpers.HLSVideoCodecs{HEVC: true},
		},
		{
			name:            "Dolby Vision 5",
			profile:         "Main 10",
			dvCompatibility: sql.NullInt64{Int64: 0, Valid: true},
			videoCodecs:     helpers.HLSVideoCodecs{HEVC: true},
		},
		{
			name:        "fragments that start on a trailing picture",
			profile:     "Main 10",
			videoCodecs: helpers.HLSVideoCodecs{HEVC: true},
			output:      fmp4testutil.Fixture{SafeVideo: false, HEVC: true, Segments: helpers.HLS_REMUX_PREVALIDATE_SEGMENTS},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			app := setupTestApp(t)
			plans := []fakeFFmpegRunPlan{hlsRunPlan(transcodeFixture)}
			preflight := tt.output.Segments > 0
			if preflight {
				plans = append([]fakeFFmpegRunPlan{hlsRunPlan(tt.output)}, plans...)
			}
			fake := &fakeFFmpeg{plans: plans}
			app.FFmpeg = fake
			movieID := insertTestHLSMovieFixture(t, app, "hevc", 2160)
			dvProfile := sql.NullInt64{}
			if tt.dvCompatibility.Valid {
				dvProfile = sql.NullInt64{Int64: 5, Valid: true}
			}
			setTestVideoStreamColumns(t, app, movieID, tt.profile, "yuv420p10le", dvProfile, tt.dvCompatibility)

			session, err := createTestHLSSessionWithVideoCodecs(app, context.Background(), movieID, helpers.HLS_PROFILE_REMUX, testIntPtr(0), tt.videoCodecs)
			if err != nil {
				t.Fatalf("createHLSSession returned error: %v", err)
			}
			defer cleanupHLSSession(session)

			if session.CopyVideo {
				t.Fatal("CopyVideo = true, want the transcode fallback")
			}
			calls := fake.Calls()
			last := calls[len(calls)-1]
			if last.CopyVideo || last.Profile == helpers.HLS_PROFILE_REMUX || !last.TonemapHDR {
				t.Fatalf("fallback RunHLS = profile %q, CopyVideo %t, TonemapHDR %t; want a tone-mapped transcode", last.Profile, last.CopyVideo, last.TonemapHDR)
			}
			wantCalls := 1
			if preflight {
				wantCalls = 2
			}
			if len(calls) != wantCalls {
				t.Fatalf("RunHLS call count = %d, want %d", len(calls), wantCalls)
			}
		})
	}
}

func TestIsBrowserSafeH264RemuxCandidate_PixelFormats(t *testing.T) {
	tests := []struct {
		pixelFormat string
		wantSafe    bool
	}{
		{"yuv420p", true},
		{"yuvj420p", true},
		// 8-bit 4:2:0 names that a "contains 10/12" marker list misread.
		{"nv12", true},
		{"nv21", true},
		{"", true},
		{"yuv420p10le", false},
		{"yuv422p", false},
		{"yuv444p", false},
		{"gray", false},
	}

	for _, tt := range tests {
		t.Run(tt.pixelFormat, func(t *testing.T) {
			stream := database.VideoStream{
				Codec:        "h264",
				CodecProfile: sql.NullString{String: "High", Valid: true},
				BitDepth:     sql.NullInt64{Int64: 8, Valid: true},
				PixelFormat:  sql.NullString{String: tt.pixelFormat, Valid: tt.pixelFormat != ""},
			}

			safe, reason := isBrowserSafeH264RemuxCandidate(&stream)
			if safe != tt.wantSafe {
				t.Fatalf("pixel format %q: got safe=%t (%s), want %t", tt.pixelFormat, safe, reason, tt.wantSafe)
			}
		})
	}
}

// The key itself separates media kinds (TestHLSSessionKey); access checks
// must too, so a movie ref never reaches an episode session with the same id.
func TestCanAccessPersonalHLSSession_SeparatesMediaKinds(t *testing.T) {
	session := &HLSSession{Media: episodeRef(9), OwnerUserID: 1}
	if canAccessPersonalHLSSession(session, movieRef(9), 1) {
		t.Fatal("a movie ref must not access an episode session")
	}
	if !canAccessPersonalHLSSession(session, episodeRef(9), 1) {
		t.Fatal("the episode ref must access its own session")
	}
}
