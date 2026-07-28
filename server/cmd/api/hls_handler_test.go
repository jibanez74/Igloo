package main

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"igloo/cmd/internal/database"
	"igloo/cmd/internal/helpers"

	"github.com/go-chi/chi/v5"
)

const (
	testPlaybackSessionID      = "4a5d0cb7-66f7-45ec-95d9-93fbe6e9eea4"
	testOtherPlaybackSessionID = "b3c1f6d2-8a4e-4f0b-9c7d-1e2a3b4c5d6e"
)

func TestHLSSessionKey(t *testing.T) {
	audioTrack := 2
	key := HLSSessionKey(123, "720p_3mbps", &audioTrack, testPlaybackSessionID, 40)
	want := "movie:123:720p_3mbps:audio:2:session:" + testPlaybackSessionID + ":start:40"
	if key != want {
		t.Errorf("HLSSessionKey = %q, want %q", key, want)
	}
}

func TestIsAllowedHLSFilename(t *testing.T) {
	tests := []struct {
		name string
		ok   bool
	}{
		{"init.mp4", true},
		{"segment_0.m4s", true},
		{"segment_999.m4s", true},
		{"segment_.m4s", false},
		{"segment_abc.m4s", false},
		{"other.mp4", false},
		{"../escape.m4s", false},
		{"bad_name.m4s", false},
		// Larger than int64 max: index must stay representable as int64.
		{"segment_18446744073709551615.m4s", false},
	}
	for _, tt := range tests {
		got := isAllowedHLSFilename(tt.name)
		if got != tt.ok {
			t.Errorf("isAllowedHLSFilename(%q) = %v, want %v", tt.name, got, tt.ok)
		}
	}
}

func TestStartSegmentComputation(t *testing.T) {
	segDur := float64(helpers.HLS_SEGMENT_TIME_SEC)
	tests := []struct {
		startSec    float64
		wantSegment int64
	}{
		{0, 0},
		{segDur, 1},
		{segDur * 900, 900},
		{segDur*2 + 1, 2},
		{0.5, 0},
	}
	for _, tt := range tests {
		got := int64(tt.startSec / segDur)
		if got != tt.wantSegment {
			t.Errorf("startSec=%.1f -> segment=%d, want %d", tt.startSec, got, tt.wantSegment)
		}
	}
}

func TestWriteHLSSessionError_PersonalCapacityReturnsRetryable503(t *testing.T) {
	recorder := httptest.NewRecorder()
	writeHLSSessionError(recorder, &hlsPersonalSessionCapacityError{MaxActive: 3})

	if recorder.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want 503", recorder.Code)
	}
	if recorder.Header().Get("Retry-After") != "5" {
		t.Fatalf("Retry-After = %q, want 5", recorder.Header().Get("Retry-After"))
	}
	if !strings.Contains(recorder.Body.String(), "personal HLS sessions") {
		t.Fatalf("response does not distinguish personal-session capacity: %s", recorder.Body.String())
	}
}

func TestSessionPlaylistDurationSec(t *testing.T) {
	tests := []struct {
		name    string
		session *HLSSession
		want    float64
	}{
		{
			name:    "nil session",
			session: nil,
			want:    0,
		},
		{
			name: "full session keeps total duration",
			session: &HLSSession{
				DurationSec: 7200,
				StartSec:    0,
			},
			want: 7200,
		},
		{
			name: "rebased session uses remaining duration",
			session: &HLSSession{
				DurationSec: 7200,
				StartSec:    6591,
			},
			want: 609,
		},
		{
			name: "start beyond duration clamps to zero",
			session: &HLSSession{
				DurationSec: 10,
				StartSec:    20,
			},
			want: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := sessionPlaylistDurationSec(tt.session)
			if got != tt.want {
				t.Fatalf("sessionPlaylistDurationSec() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestBuildHLSPlaylistBody(t *testing.T) {
	session := &HLSSession{DurationSec: 12, CopyVideo: false}
	generated := buildHLSPlaylistBody(session, session.DurationSec, "/api/hls/", "?audio_track=0")
	if !strings.Contains(generated, "segment_0.m4s?audio_track=0") {
		t.Fatalf("generated playlist did not include rewritten segment URL: %s", generated)
	}

	session.FinalPlaylist = "#EXTM3U\n#EXT-X-MAP:URI=\"init.mp4\"\n#EXTINF:4,\nsegment_0.m4s\n"
	finalized := buildHLSPlaylistBody(session, session.DurationSec, "/api/hls/", "?audio_track=1")
	if !strings.Contains(finalized, "/api/hls/init.mp4?audio_track=1") {
		t.Fatalf("final playlist did not rewrite init URL: %s", finalized)
	}
	if !strings.Contains(finalized, "/api/hls/segment_0.m4s?audio_track=1") {
		t.Fatalf("final playlist did not rewrite segment URL: %s", finalized)
	}
}

func TestServeReadyHLSSegment(t *testing.T) {
	t.Run("serves completed segment with shared headers", func(t *testing.T) {
		tempDir := t.TempDir()
		filename := helpers.HLS_SEGMENT_FILENAME_PREFIX + "0" + helpers.HLS_SEGMENT_FILENAME_SUFFIX
		nextFilename := helpers.HLS_SEGMENT_FILENAME_PREFIX + "1" + helpers.HLS_SEGMENT_FILENAME_SUFFIX
		err := os.WriteFile(filepath.Join(tempDir, filename), []byte("segment payload"), 0o600)
		if err != nil {
			t.Fatalf("write segment: %v", err)
		}
		err = os.WriteFile(filepath.Join(tempDir, nextFilename), []byte("next segment"), 0o600)
		if err != nil {
			t.Fatalf("write next segment: %v", err)
		}

		req := httptest.NewRequest(http.MethodGet, "/segment", nil)
		w := httptest.NewRecorder()
		serveReadyHLSSegment(w, req, &HLSSession{TempDir: tempDir}, filename)

		if w.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200: %s", w.Code, w.Body.String())
		}
		if got := w.Header().Get("Content-Type"); got != hlsSegmentHTTPContentType {
			t.Fatalf("Content-Type = %q, want %q", got, hlsSegmentHTTPContentType)
		}
		if got := w.Header().Get("Cache-Control"); got != "no-store" {
			t.Fatalf("Cache-Control = %q, want no-store", got)
		}
		if w.Body.String() != "segment payload" {
			t.Fatalf("body = %q, want segment payload", w.Body.String())
		}
	})

	t.Run("reports exited transcode before waiting", func(t *testing.T) {
		session := &HLSSession{TempDir: t.TempDir(), Exited: true, ExitErr: fmt.Errorf("ffmpeg failed")}
		req := httptest.NewRequest(http.MethodGet, "/segment", nil)
		w := httptest.NewRecorder()
		serveReadyHLSSegment(w, req, session, helpers.HLS_SEGMENT_FILENAME_PREFIX+"0"+helpers.HLS_SEGMENT_FILENAME_SUFFIX)

		if w.Code != http.StatusInternalServerError {
			t.Fatalf("status = %d, want 500: %s", w.Code, w.Body.String())
		}
		if !strings.Contains(w.Body.String(), "transcoding stopped") {
			t.Fatalf("body = %q, want transcode failure", w.Body.String())
		}
	})
}

func TestFileReady(t *testing.T) {
	dir := t.TempDir()

	t.Run("non-existent file returns false", func(t *testing.T) {
		if fileReady(filepath.Join(dir, "missing.m4s")) {
			t.Error("expected false for non-existent file")
		}
	})

	t.Run("empty file returns false", func(t *testing.T) {
		path := filepath.Join(dir, "empty.m4s")
		err := os.WriteFile(path, []byte{}, 0644)
		if err != nil {
			t.Fatal(err)
		}
		if fileReady(path) {
			t.Error("expected false for empty file")
		}
	})

	t.Run("file with content returns true", func(t *testing.T) {
		path := filepath.Join(dir, "ready.m4s")
		err := os.WriteFile(path, []byte{0x00, 0x01, 0x02}, 0644)
		if err != nil {
			t.Fatal(err)
		}
		if !fileReady(path) {
			t.Error("expected true for file with content")
		}
	})
}

func TestSegmentComplete(t *testing.T) {
	prefix := helpers.HLS_SEGMENT_FILENAME_PREFIX
	suffix := helpers.HLS_SEGMENT_FILENAME_SUFFIX
	segName := func(n int) string {
		return fmt.Sprintf("%s%d%s", prefix, n, suffix)
	}

	t.Run("init is complete when segment_0 exists", func(t *testing.T) {
		dir := t.TempDir()
		session := &HLSSession{TempDir: dir}

		if segmentComplete(session, helpers.HLS_INIT_FILENAME) {
			t.Error("init should not be complete before segment_0 exists")
		}

		err := os.WriteFile(filepath.Join(dir, segName(0)), []byte{0x01}, 0644)
		if err != nil {
			t.Fatal(err)
		}

		if !segmentComplete(session, helpers.HLS_INIT_FILENAME) {
			t.Error("init should be complete once segment_0 exists")
		}
	})

	t.Run("segment is complete when next segment exists", func(t *testing.T) {
		dir := t.TempDir()
		session := &HLSSession{TempDir: dir}

		err := os.WriteFile(filepath.Join(dir, segName(0)), []byte{0x01}, 0644)
		if err != nil {
			t.Fatal(err)
		}

		if segmentComplete(session, segName(0)) {
			t.Error("segment_0 should not be complete without segment_1")
		}

		err = os.WriteFile(filepath.Join(dir, segName(1)), []byte{0x01}, 0644)
		if err != nil {
			t.Fatal(err)
		}

		if !segmentComplete(session, segName(0)) {
			t.Error("segment_0 should be complete once segment_1 exists")
		}
	})

	t.Run("last segment is complete when ffmpeg has exited", func(t *testing.T) {
		dir := t.TempDir()
		session := &HLSSession{TempDir: dir, ExitMu: sync.Mutex{}}

		err := os.WriteFile(filepath.Join(dir, segName(5)), []byte{0x01}, 0644)
		if err != nil {
			t.Fatal(err)
		}

		if segmentComplete(session, segName(5)) {
			t.Error("segment should not be complete when ffmpeg is still running and no next segment")
		}

		session.ExitMu.Lock()
		session.Exited = true
		session.ExitMu.Unlock()

		if !segmentComplete(session, segName(5)) {
			t.Error("segment should be complete after ffmpeg exits and file exists")
		}
	})

	t.Run("exited session without file on disk returns false", func(t *testing.T) {
		dir := t.TempDir()
		session := &HLSSession{TempDir: dir, Exited: true, ExitMu: sync.Mutex{}}

		if segmentComplete(session, segName(99)) {
			t.Error("should return false when ffmpeg exited but segment file does not exist")
		}
	})

	t.Run("invalid filename returns false", func(t *testing.T) {
		dir := t.TempDir()
		session := &HLSSession{TempDir: dir, ExitMu: sync.Mutex{}}

		if segmentComplete(session, "garbage.txt") {
			t.Error("should return false for unparseable filename")
		}
	})
}

func TestHLSManifest_UsesRequestedRemuxPathWhenEffectiveProfileFallsBack(t *testing.T) {
	app := setupTestApp(t)
	defer app.DB.Close()

	audioTrack := 0
	app.InitSession()
	userID := int64(42)
	movieID := insertTestHLSMovieFixture(t, app, "h264", 1080)
	session := &HLSSession{
		MovieID:         movieID,
		OwnerUserID:     userID,
		PlaybackSession: testPlaybackSessionID,
		TempDir:         t.TempDir(),
		DurationSec:     12,
		StartSec:        0,
		CopyVideo:       false,
	}
	app.HLSSessionCache.SetDefault(HLSSessionKey(movieID, helpers.HLS_PROFILE_REMUX, &audioTrack, testPlaybackSessionID, 0), session)

	router := chi.NewRouter()
	router.Get("/api/movies/{id}/hls/{profile}/playlist.m3u8", func(w http.ResponseWriter, r *http.Request) {
		app.SessionManager.Put(r.Context(), cookieUserID, userID)
		app.HLSManifest(w, r)
	})

	req := httptest.NewRequest(
		http.MethodGet,
		fmt.Sprintf("/api/movies/%d/hls/remux/playlist.m3u8?audio_track=0&playback_session=%s&start=0", movieID, testPlaybackSessionID),
		nil,
	)
	recorder := httptest.NewRecorder()

	handler := app.SessionManager.LoadAndSave(router)
	handler.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", recorder.Code, recorder.Body.String())
	}

	body := recorder.Body.String()
	if !strings.Contains(body, fmt.Sprintf(`/api/movies/%d/hls/remux/init.mp4?audio_track=0&playback_session=%s&start=0`, movieID, testPlaybackSessionID)) {
		t.Fatalf("playlist body missing remux init path: %s", body)
	}
	if !strings.Contains(body, fmt.Sprintf(`/api/movies/%d/hls/remux/segment_0.m4s?audio_track=0&playback_session=%s&start=0`, movieID, testPlaybackSessionID)) {
		t.Fatalf("playlist body missing remux segment path: %s", body)
	}
	if strings.Contains(body, helpers.HLS_PROFILE_1080P_8MBPS) {
		t.Fatalf("playlist body should not expose effective profile path: %s", body)
	}
}

func TestHLSManifest_PropagatesEffectiveStartToAssetsAndSegmentLookup(t *testing.T) {
	app := setupTestApp(t)
	defer app.DB.Close()
	app.Settings = &database.Setting{}
	app.InitSession()
	app.FFmpeg = &fakeFFmpeg{plans: []fakeFFmpegRunPlan{{
		WriteFiles: func(outDir string) error {
			segmentPath := filepath.Join(outDir, helpers.HLS_SEGMENT_FILENAME_PREFIX+"0"+helpers.HLS_SEGMENT_FILENAME_SUFFIX)
			return os.WriteFile(segmentPath, []byte("effective-segment"), 0o644)
		},
	}}}

	movieID := insertTestHLSMovieFixture(t, app, "h264", 1080)
	userID := int64(42)
	audioTrack := 0
	effectiveStart := 7200 - hlsStartClampTailSec

	router := chi.NewRouter()
	router.Get("/api/movies/{id}/hls/{profile}/playlist.m3u8", func(w http.ResponseWriter, r *http.Request) {
		app.SessionManager.Put(r.Context(), cookieUserID, userID)
		app.HLSManifest(w, r)
	})
	router.Get("/api/movies/{id}/hls/{profile}/{filename}", func(w http.ResponseWriter, r *http.Request) {
		app.SessionManager.Put(r.Context(), cookieUserID, userID)
		app.HLSSegment(w, r)
	})
	handler := app.SessionManager.LoadAndSave(router)

	manifestURL := fmt.Sprintf(
		"/api/movies/%d/hls/%s/playlist.m3u8?audio_track=0&playback_session=%s&start=9000",
		movieID,
		helpers.HLS_PROFILE_720P_3MBPS,
		testPlaybackSessionID,
	)
	manifestRecorder := httptest.NewRecorder()
	handler.ServeHTTP(manifestRecorder, httptest.NewRequest(http.MethodGet, manifestURL, nil))
	if manifestRecorder.Code != http.StatusOK {
		t.Fatalf("manifest status = %d, want 200: %s", manifestRecorder.Code, manifestRecorder.Body.String())
	}
	if !strings.Contains(manifestRecorder.Body.String(), fmt.Sprintf("start=%d", effectiveStart)) {
		t.Fatalf("manifest assets do not use effective start %d: %s", effectiveStart, manifestRecorder.Body.String())
	}
	if strings.Contains(manifestRecorder.Body.String(), "start=9000") {
		t.Fatalf("manifest assets expose invalid requested start: %s", manifestRecorder.Body.String())
	}

	effectiveKey := HLSSessionKey(
		movieID,
		helpers.HLS_PROFILE_720P_3MBPS,
		&audioTrack,
		testPlaybackSessionID,
		effectiveStart,
	)
	_, cached := app.HLSSessionCache.Get(effectiveKey)
	if !cached {
		t.Fatalf("effective key %q was not cached", effectiveKey)
	}

	segmentURL := fmt.Sprintf(
		"/api/movies/%d/hls/%s/segment_0.m4s?audio_track=0&playback_session=%s&start=%d",
		movieID,
		helpers.HLS_PROFILE_720P_3MBPS,
		testPlaybackSessionID,
		effectiveStart,
	)
	segmentRecorder := httptest.NewRecorder()
	handler.ServeHTTP(segmentRecorder, httptest.NewRequest(http.MethodGet, segmentURL, nil))
	if segmentRecorder.Code != http.StatusOK {
		t.Fatalf("segment status = %d, want 200: %s", segmentRecorder.Code, segmentRecorder.Body.String())
	}
	if segmentRecorder.Body.String() != "effective-segment" {
		t.Fatalf("segment body = %q, want effective-segment", segmentRecorder.Body.String())
	}
}

func TestHLSSegment_UsesRequestedRemuxKeyWhenEffectiveProfileFallsBack(t *testing.T) {
	app := setupTestApp(t)
	defer app.DB.Close()

	audioTrack := 0
	app.InitSession()
	userID := int64(43)
	dir := t.TempDir()
	segmentPath := filepath.Join(dir, "segment_0.m4s")
	writeErr := os.WriteFile(segmentPath, []byte("segment-bytes"), 0644)
	if writeErr != nil {
		t.Fatalf("write segment: %v", writeErr)
	}

	session := &HLSSession{
		MovieID:     5,
		OwnerUserID: userID,
		TempDir:     dir,
		StartSec:    0,
		CopyVideo:   false,
		Exited:      true,
		ExitMu:      sync.Mutex{},
	}
	app.HLSSessionCache.SetDefault(HLSSessionKey(5, helpers.HLS_PROFILE_REMUX, &audioTrack, testPlaybackSessionID, 0), session)

	router := chi.NewRouter()
	router.Get("/api/movies/{id}/hls/{profile}/{filename}", func(w http.ResponseWriter, r *http.Request) {
		app.SessionManager.Put(r.Context(), cookieUserID, userID)
		app.HLSSegment(w, r)
	})

	req := httptest.NewRequest(
		http.MethodGet,
		fmt.Sprintf("/api/movies/5/hls/remux/segment_0.m4s?audio_track=0&playback_session=%s&start=0", testPlaybackSessionID),
		nil,
	)
	recorder := httptest.NewRecorder()

	handler := app.SessionManager.LoadAndSave(router)
	handler.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", recorder.Code, recorder.Body.String())
	}
	if recorder.Body.String() != "segment-bytes" {
		t.Fatalf("segment body = %q, want %q", recorder.Body.String(), "segment-bytes")
	}
}

func TestStopPersonalHLSSession_RemovesOnlyMatchingOwnedSession(t *testing.T) {
	app := setupTestApp(t)
	defer app.DB.Close()
	app.InitSession()

	userID := int64(100)
	audioTrack := 0
	matchingDir := t.TempDir()
	matchingKey := HLSSessionKey(5, helpers.HLS_PROFILE_REMUX, &audioTrack, testPlaybackSessionID, 0)
	otherMovieKey := HLSSessionKey(6, helpers.HLS_PROFILE_REMUX, &audioTrack, testPlaybackSessionID, 0)
	otherUserKey := HLSSessionKey(5, helpers.HLS_PROFILE_REMUX, &audioTrack, testPlaybackSessionID, 4)
	otherPlaybackKey := HLSSessionKey(5, helpers.HLS_PROFILE_REMUX, &audioTrack, testOtherPlaybackSessionID, 0)
	roomKey := RoomHLSSessionKey(9)

	app.HLSSessionCache.SetDefault(matchingKey, &HLSSession{MovieID: 5, OwnerUserID: userID, PlaybackSession: testPlaybackSessionID, TempDir: matchingDir})
	app.HLSSessionCache.SetDefault(otherMovieKey, &HLSSession{MovieID: 6, OwnerUserID: userID, PlaybackSession: testPlaybackSessionID, TempDir: t.TempDir()})
	app.HLSSessionCache.SetDefault(otherUserKey, &HLSSession{MovieID: 5, OwnerUserID: userID + 1, PlaybackSession: testPlaybackSessionID, TempDir: t.TempDir()})
	// A late stop from a closing tab must never remove a session the user just
	// created under a different playback_session UUID after reopening.
	app.HLSSessionCache.SetDefault(otherPlaybackKey, &HLSSession{MovieID: 5, OwnerUserID: userID, PlaybackSession: testOtherPlaybackSessionID, TempDir: t.TempDir()})
	app.HLSSessionCache.SetDefault(roomKey, &HLSSession{MovieID: 5, OwnerUserID: userID, PlaybackSession: testPlaybackSessionID, TempDir: t.TempDir(), IsRoom: true})

	router := chi.NewRouter()
	router.Post("/api/movies/{id}/hls/session/stop", func(w http.ResponseWriter, r *http.Request) {
		app.SessionManager.Put(r.Context(), cookieUserID, userID)
		app.StopPersonalHLSSession(w, r)
	})
	handler := app.SessionManager.LoadAndSave(router)
	req := httptest.NewRequest(
		http.MethodPost,
		fmt.Sprintf("/api/movies/5/hls/session/stop?playback_session=%s", testPlaybackSessionID),
		nil,
	)
	recorder := httptest.NewRecorder()

	handler.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", recorder.Code, recorder.Body.String())
	}
	if _, ok := app.HLSSessionCache.Get(matchingKey); ok {
		t.Fatal("expected matching personal HLS session to be removed")
	}
	for _, key := range []string{otherMovieKey, otherUserKey, otherPlaybackKey, roomKey} {
		if _, ok := app.HLSSessionCache.Get(key); !ok {
			t.Fatalf("expected non-matching session %q to remain", key)
		}
	}
	if _, err := os.Stat(matchingDir); !os.IsNotExist(err) {
		t.Fatalf("expected matching temp dir to be removed, err=%v", err)
	}
}

func TestStopPersonalHLSSession_InvalidPlaybackSession(t *testing.T) {
	app := setupTestApp(t)
	defer app.DB.Close()
	app.InitSession()

	router := chi.NewRouter()
	router.Post("/api/movies/{id}/hls/session/stop", func(w http.ResponseWriter, r *http.Request) {
		app.SessionManager.Put(r.Context(), cookieUserID, int64(100))
		app.StopPersonalHLSSession(w, r)
	})
	handler := app.SessionManager.LoadAndSave(router)
	req := httptest.NewRequest(http.MethodPost, "/api/movies/5/hls/session/stop?playback_session=not-a-uuid", nil)
	recorder := httptest.NewRecorder()

	handler.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", recorder.Code, recorder.Body.String())
	}
}

func TestHLSSegment_RejectsDifferentOwner(t *testing.T) {
	app := setupTestApp(t)
	defer app.DB.Close()
	app.InitSession()

	audioTrack := 0
	userID := int64(100)
	key := HLSSessionKey(5, helpers.HLS_PROFILE_REMUX, &audioTrack, testPlaybackSessionID, 0)
	app.HLSSessionCache.SetDefault(key, &HLSSession{
		MovieID:         5,
		OwnerUserID:     userID + 1,
		PlaybackSession: testPlaybackSessionID,
		TempDir:         t.TempDir(),
		Exited:          true,
		ExitMu:          sync.Mutex{},
	})

	router := chi.NewRouter()
	router.Get("/api/movies/{id}/hls/{profile}/{filename}", func(w http.ResponseWriter, r *http.Request) {
		app.SessionManager.Put(r.Context(), cookieUserID, userID)
		app.HLSSegment(w, r)
	})
	handler := app.SessionManager.LoadAndSave(router)
	req := httptest.NewRequest(
		http.MethodGet,
		fmt.Sprintf("/api/movies/5/hls/remux/segment_0.m4s?audio_track=0&playback_session=%s&start=0", testPlaybackSessionID),
		nil,
	)
	recorder := httptest.NewRecorder()

	handler.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", recorder.Code, recorder.Body.String())
	}
	if _, ok := app.HLSSessionCache.Get(key); !ok {
		t.Fatal("expected mismatched-owner session to remain cached")
	}
}

func TestCleanupPersonalHLSSessionsForOwner_KeepsCurrentWindow(t *testing.T) {
	app := setupTestApp(t)
	defer app.DB.Close()

	userID := int64(100)
	audioTrack := 0
	oldKey := HLSSessionKey(5, helpers.HLS_PROFILE_REMUX, &audioTrack, testPlaybackSessionID, 0)
	keepKey := HLSSessionKey(5, helpers.HLS_PROFILE_REMUX, &audioTrack, testPlaybackSessionID, 40)
	otherKey := HLSSessionKey(5, helpers.HLS_PROFILE_REMUX, &audioTrack, testPlaybackSessionID, 80)

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
	key := HLSSessionKey(5, helpers.HLS_PROFILE_REMUX, &audioTrack, testPlaybackSessionID, 0)
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
	personalKey := HLSSessionKey(5, helpers.HLS_PROFILE_REMUX, &audioTrack, testPlaybackSessionID, 0)
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
	key := HLSSessionKey(5, helpers.HLS_PROFILE_REMUX, &audioTrack, testPlaybackSessionID, 0)
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
