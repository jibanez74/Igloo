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

	"igloo/cmd/internal/helpers"

	"github.com/go-chi/chi/v5"
)

const testPlaybackSessionID = "4a5d0cb7-66f7-45ec-95d9-93fbe6e9eea4"

func TestParseSegmentIndex(t *testing.T) {
	tests := []struct {
		filename string
		wantIdx  int64
		wantErr  bool
	}{
		{helpers.HLS_SEGMENT_FILENAME_PREFIX + "0" + helpers.HLS_SEGMENT_FILENAME_SUFFIX, 0, false},
		{helpers.HLS_SEGMENT_FILENAME_PREFIX + "42" + helpers.HLS_SEGMENT_FILENAME_SUFFIX, 42, false},
		{helpers.HLS_SEGMENT_FILENAME_PREFIX + "900" + helpers.HLS_SEGMENT_FILENAME_SUFFIX, 900, false},
		{"init.mp4", 0, true},
		{"bad_name.m4s", 0, true},
		{helpers.HLS_SEGMENT_FILENAME_PREFIX + "abc" + helpers.HLS_SEGMENT_FILENAME_SUFFIX, 0, true},
	}
	for _, tt := range tests {
		idx, err := parseSegmentIndex(tt.filename)
		if tt.wantErr {
			if err == nil {
				t.Errorf("parseSegmentIndex(%q) expected error, got idx=%d", tt.filename, idx)
			}
			continue
		}
		if err != nil {
			t.Errorf("parseSegmentIndex(%q) unexpected error: %v", tt.filename, err)
			continue
		}
		if idx != tt.wantIdx {
			t.Errorf("parseSegmentIndex(%q) = %d, want %d", tt.filename, idx, tt.wantIdx)
		}
	}
}

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

func TestValidateHLSFilename(t *testing.T) {
	tests := []struct {
		name     string
		filename string
		wantErr  bool
	}{
		{
			name:     "init file is allowed",
			filename: helpers.HLS_INIT_FILENAME,
		},
		{
			name:     "segment filename is allowed",
			filename: "segment_69.m4s",
		},
		{
			name:     "later segment filename is allowed",
			filename: "segment_99.m4s",
		},
		{
			name:     "invalid segment name returns error",
			filename: "bad_name.m4s",
			wantErr:  true,
		},
		{
			name:     "out of range segment index returns error",
			filename: "segment_18446744073709551615.m4s",
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateHLSFilename(tt.filename)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("validateHLSFilename(%q) expected error", tt.filename)
				}
				return
			}
			if err != nil {
				t.Fatalf("validateHLSFilename(%q) unexpected error: %v", tt.filename, err)
			}
		})
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
	session := &HLSSession{
		MovieID:          5,
		OwnerUserID:      userID,
		TempDir:          t.TempDir(),
		DurationSec:      12,
		StartSec:         0,
		RequestedProfile: helpers.HLS_PROFILE_REMUX,
		EffectiveProfile: helpers.HLS_PROFILE_1080P_8MBPS,
		CopyVideo:        false,
	}
	app.HLSSessionCache.SetDefault(HLSSessionKey(5, helpers.HLS_PROFILE_REMUX, &audioTrack, testPlaybackSessionID, 0), session)

	router := chi.NewRouter()
	router.Get("/api/movies/{id}/hls/{profile}/playlist.m3u8", func(w http.ResponseWriter, r *http.Request) {
		app.SessionManager.Put(r.Context(), cookieUserID, userID)
		app.HLSManifest(w, r)
	})

	req := httptest.NewRequest(
		http.MethodGet,
		fmt.Sprintf("/api/movies/5/hls/remux/playlist.m3u8?audio_track=0&playback_session=%s&start=0", testPlaybackSessionID),
		nil,
	)
	recorder := httptest.NewRecorder()

	handler := app.SessionManager.LoadAndSave(router)
	handler.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", recorder.Code, recorder.Body.String())
	}

	body := recorder.Body.String()
	if !strings.Contains(body, fmt.Sprintf(`/api/movies/5/hls/remux/init.mp4?audio_track=0&playback_session=%s&start=0`, testPlaybackSessionID)) {
		t.Fatalf("playlist body missing remux init path: %s", body)
	}
	if !strings.Contains(body, fmt.Sprintf(`/api/movies/5/hls/remux/segment_0.m4s?audio_track=0&playback_session=%s&start=0`, testPlaybackSessionID)) {
		t.Fatalf("playlist body missing remux segment path: %s", body)
	}
	if strings.Contains(body, helpers.HLS_PROFILE_1080P_8MBPS) {
		t.Fatalf("playlist body should not expose effective profile path: %s", body)
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
		MovieID:          5,
		OwnerUserID:      userID,
		TempDir:          dir,
		StartSec:         0,
		RequestedProfile: helpers.HLS_PROFILE_REMUX,
		EffectiveProfile: helpers.HLS_PROFILE_1080P_8MBPS,
		CopyVideo:        false,
		Exited:           true,
		ExitMu:           sync.Mutex{},
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
	roomKey := RoomHLSSessionKey(9)

	app.HLSSessionCache.SetDefault(matchingKey, &HLSSession{MovieID: 5, OwnerUserID: userID, PlaybackSession: testPlaybackSessionID, TempDir: matchingDir})
	app.HLSSessionCache.SetDefault(otherMovieKey, &HLSSession{MovieID: 6, OwnerUserID: userID, PlaybackSession: testPlaybackSessionID, TempDir: t.TempDir()})
	app.HLSSessionCache.SetDefault(otherUserKey, &HLSSession{MovieID: 5, OwnerUserID: userID + 1, PlaybackSession: testPlaybackSessionID, TempDir: t.TempDir()})
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
	for _, key := range []string{otherMovieKey, otherUserKey, roomKey} {
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
