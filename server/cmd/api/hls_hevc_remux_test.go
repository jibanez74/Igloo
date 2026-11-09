package main

import (
	"context"
	"database/sql"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"igloo/cmd/internal/database"
	"igloo/cmd/internal/ffmpeg/fmp4testutil"
	"igloo/cmd/internal/helpers"
)

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

// The manifest flow for a declaring client: video_codecs reaches planning,
// the HEVC source is copied, every rewritten asset URL carries the parameter,
// and a segment request built from those URLs finds the same session.
func TestHLSManifest_HEVCRemux(t *testing.T) {
	app := setupTestApp(t)
	app.FFmpeg = &fakeFFmpeg{plans: []fakeFFmpegRunPlan{
		hlsRunPlan(fmp4testutil.Fixture{SafeVideo: true, HEVC: true, Segments: helpers.HLS_REMUX_PREVALIDATE_SEGMENTS}),
	}}
	movieID := insertTestHLSMovieFixture(t, app, "hevc", 2160)
	setTestVideoStreamColumns(t, app, movieID, "Main 10", "yuv420p10le", sql.NullInt64{Int64: 8, Valid: true}, sql.NullInt64{Int64: 1, Valid: true})
	handler := authenticatedRouter(t, app, 42)

	manifestURL := fmt.Sprintf(
		"/api/movies/%d/hls/%s/playlist.m3u8?audio_track=0&video_codecs=hevc&playback_session=%s&start=0",
		movieID, helpers.HLS_PROFILE_REMUX, testPlaybackSessionID,
	)
	request := httptest.NewRequest(http.MethodGet, manifestURL, nil)
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("manifest status = %d, want 200: %s", recorder.Code, recorder.Body.String())
	}
	if got := recorder.Header().Get(hlsEffectiveProfileHeader); got != helpers.HLS_PROFILE_REMUX {
		t.Fatalf("effective profile = %q, want remux", got)
	}
	assertOpenAPIExchange(t, "hlsManifest", request, recorder)

	wantAssetQuery := fmt.Sprintf("?audio_track=0&playback_session=%s&start=0&video_codecs=hevc", testPlaybackSessionID)
	segment := helpers.HLS_SEGMENT_FILENAME_PREFIX + "0" + helpers.HLS_SEGMENT_FILENAME_SUFFIX
	body := recorder.Body.String()
	if !strings.Contains(body, helpers.HLS_INIT_FILENAME+wantAssetQuery) || !strings.Contains(body, segment+wantAssetQuery) {
		t.Fatalf("asset URLs do not carry video_codecs: %s", body)
	}

	segmentURL := fmt.Sprintf("/api/movies/%d/hls/%s/%s%s", movieID, helpers.HLS_PROFILE_REMUX, segment, wantAssetQuery)
	segmentRequest := httptest.NewRequest(http.MethodGet, segmentURL, nil)
	segmentRecorder := httptest.NewRecorder()
	handler.ServeHTTP(segmentRecorder, segmentRequest)
	if segmentRecorder.Code != http.StatusOK {
		t.Fatalf("segment status = %d, want 200: %s", segmentRecorder.Code, segmentRecorder.Body.String())
	}
	assertOpenAPIExchange(t, "hlsSegment", segmentRequest, segmentRecorder)

	// The H.264-only key names a different session.
	withoutCodecs := strings.Replace(segmentURL, "&video_codecs=hevc", "", 1)
	missRecorder := httptest.NewRecorder()
	handler.ServeHTTP(missRecorder, httptest.NewRequest(http.MethodGet, withoutCodecs, nil))
	if missRecorder.Code != http.StatusNotFound {
		t.Fatalf("segment without video_codecs status = %d, want 404", missRecorder.Code)
	}
}
