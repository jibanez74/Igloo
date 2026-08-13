package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"igloo/cmd/internal/helpers"

	"github.com/go-chi/chi/v5"
)

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
		// temp_file writes segments under .tmp names before the rename; a
		// request must never be able to read one mid-write.
		{"segment_0.m4s.tmp", false},
		{"init.mp4.tmp", false},
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

func TestWriteHLSSessionError(t *testing.T) {
	tests := []struct {
		name           string
		err            error
		wantStatus     int
		wantRetryAfter string
	}{
		{
			name:       "a missing session is not found",
			err:        errHLSSessionNotFound,
			wantStatus: http.StatusNotFound,
		},
		{
			name:       "a session that produced nothing is not found",
			err:        errHLSSessionEmpty,
			wantStatus: http.StatusNotFound,
		},
		{
			name:       "a failed session is a server error",
			err:        fmt.Errorf("%w: ffmpeg died", errHLSSessionFailed),
			wantStatus: http.StatusInternalServerError,
		},
		{
			name:           "a full transcode pool is retryable",
			err:            &hlsTranscodeCapacityError{MaxActive: 2},
			wantStatus:     http.StatusServiceUnavailable,
			wantRetryAfter: "5",
		},
		{
			name:           "a user at their session cap is retryable",
			err:            &hlsPersonalSessionCapacityError{MaxActive: 3},
			wantStatus:     http.StatusServiceUnavailable,
			wantRetryAfter: "5",
		},
		{
			name:           "a full transcode disk is retryable",
			err:            &hlsStorageCapacityError{FreeBytes: 1024, RequiredBytes: 2 << 30},
			wantStatus:     http.StatusServiceUnavailable,
			wantRetryAfter: "5",
		},
		{
			// The session is healthy, so the client should come back sooner than
			// it would for a capacity refusal.
			name:           "a playlist that is not published yet retries sooner",
			err:            errHLSPlaylistNotReady,
			wantStatus:     http.StatusServiceUnavailable,
			wantRetryAfter: "1",
		},
		{
			name:       "anything else is a bad request",
			err:        errors.New("movie 7 has no valid duration in the database"),
			wantStatus: http.StatusBadRequest,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			writeHLSSessionError(recorder, tt.err)

			if recorder.Code != tt.wantStatus {
				t.Fatalf("status = %d, want %d", recorder.Code, tt.wantStatus)
			}
			if got := recorder.Header().Get("Retry-After"); got != tt.wantRetryAfter {
				t.Fatalf("Retry-After = %q, want %q", got, tt.wantRetryAfter)
			}
			if !strings.Contains(recorder.Body.String(), tt.err.Error()) {
				t.Fatalf("body = %s, want it to carry %q", recorder.Body.String(), tt.err.Error())
			}
		})
	}
}

func TestParseHLSParams(t *testing.T) {
	const validQuery = "playback_session=" + testPlaybackSessionID + "&start=40"

	router := chi.NewRouter()
	router.Get("/api/movies/{id}/hls/{profile}/playlist.m3u8", func(w http.ResponseWriter, r *http.Request) {
		params, ok := parseHLSParams(w, r)
		if !ok {
			return
		}
		audioTrack := "none"
		if params.AudioTrack != nil {
			audioTrack = strconv.Itoa(*params.AudioTrack)
		}
		fmt.Fprintf(w, "%d|%s|%s|%d|%s|%s",
			params.MovieID, params.Profile, params.PlaybackSession, params.StartSec, audioTrack, params.Reload)
	})

	tests := []struct {
		name       string
		target     string
		wantStatus int
		wantBody   string
	}{
		{
			name:       "accepts a full request",
			target:     "/api/movies/7/hls/720p_3mbps/playlist.m3u8?" + validQuery + "&audio_track=2&reload=9",
			wantStatus: http.StatusOK,
			wantBody:   "7|720p_3mbps|" + testPlaybackSessionID + "|40|2|9",
		},
		{
			// An absent audio_track stays nil so the cache key can distinguish
			// "no audio selected" from track 0.
			name:       "omitted audio_track stays unset",
			target:     "/api/movies/7/hls/720p_3mbps/playlist.m3u8?" + validQuery,
			wantStatus: http.StatusOK,
			wantBody:   "7|720p_3mbps|" + testPlaybackSessionID + "|40|none|",
		},
		{
			name:       "rejects a non-numeric movie id",
			target:     "/api/movies/abc/hls/720p_3mbps/playlist.m3u8?" + validQuery,
			wantStatus: http.StatusBadRequest,
			wantBody:   "invalid movie id",
		},
		{
			name:       "rejects a zero movie id",
			target:     "/api/movies/0/hls/720p_3mbps/playlist.m3u8?" + validQuery,
			wantStatus: http.StatusBadRequest,
			wantBody:   "invalid movie id",
		},
		{
			name:       "rejects an unknown quality profile",
			target:     "/api/movies/7/hls/junk/playlist.m3u8?" + validQuery,
			wantStatus: http.StatusBadRequest,
			wantBody:   "invalid quality profile",
		},
		{
			name:       "rejects a missing playback_session",
			target:     "/api/movies/7/hls/720p_3mbps/playlist.m3u8?start=40",
			wantStatus: http.StatusBadRequest,
			wantBody:   "invalid playback_session",
		},
		{
			name:       "rejects a playback_session that is not a UUID",
			target:     "/api/movies/7/hls/720p_3mbps/playlist.m3u8?playback_session=not-a-uuid&start=40",
			wantStatus: http.StatusBadRequest,
			wantBody:   "invalid playback_session",
		},
		{
			// start keys the session cache, so an absent one would silently
			// collapse every seek window onto the same session.
			name:       "rejects a missing start",
			target:     "/api/movies/7/hls/720p_3mbps/playlist.m3u8?playback_session=" + testPlaybackSessionID,
			wantStatus: http.StatusBadRequest,
			wantBody:   "start parameter is required",
		},
		{
			name:       "rejects a negative start",
			target:     "/api/movies/7/hls/720p_3mbps/playlist.m3u8?playback_session=" + testPlaybackSessionID + "&start=-1",
			wantStatus: http.StatusBadRequest,
			wantBody:   "invalid start parameter",
		},
		{
			name:       "rejects a non-numeric start",
			target:     "/api/movies/7/hls/720p_3mbps/playlist.m3u8?playback_session=" + testPlaybackSessionID + "&start=abc",
			wantStatus: http.StatusBadRequest,
			wantBody:   "invalid start parameter",
		},
		{
			name:       "rejects a negative audio_track",
			target:     "/api/movies/7/hls/720p_3mbps/playlist.m3u8?" + validQuery + "&audio_track=-1",
			wantStatus: http.StatusBadRequest,
			wantBody:   "invalid audio_track",
		},
		{
			name:       "rejects a non-numeric audio_track",
			target:     "/api/movies/7/hls/720p_3mbps/playlist.m3u8?" + validQuery + "&audio_track=x",
			wantStatus: http.StatusBadRequest,
			wantBody:   "invalid audio_track",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, tt.target, nil))

			if recorder.Code != tt.wantStatus {
				t.Fatalf("status = %d, want %d: %s", recorder.Code, tt.wantStatus, recorder.Body.String())
			}
			if !strings.Contains(recorder.Body.String(), tt.wantBody) {
				t.Fatalf("body = %s, want it to contain %q", recorder.Body.String(), tt.wantBody)
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

func TestBuildHLSPlaylistBody(t *testing.T) {
	t.Run("synthesizes a transcode playlist and rewrites its URLs", func(t *testing.T) {
		session := &HLSSession{DurationSec: 12, CopyVideo: false}
		generated, err := buildHLSPlaylistBody(t.Context(), session, session.DurationSec, "/api/hls/", "?audio_track=0")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !strings.Contains(generated, "segment_0.m4s?audio_track=0") {
			t.Fatalf("generated playlist did not include rewritten segment URL: %s", generated)
		}

		session.FinalPlaylist = "#EXTM3U\n#EXT-X-MAP:URI=\"init.mp4\"\n#EXTINF:4,\nsegment_0.m4s\n"
		finalized, err := buildHLSPlaylistBody(t.Context(), session, session.DurationSec, "/api/hls/", "?audio_track=1")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !strings.Contains(finalized, "/api/hls/init.mp4?audio_track=1") {
			t.Fatalf("final playlist did not rewrite init URL: %s", finalized)
		}
		if !strings.Contains(finalized, "/api/hls/segment_0.m4s?audio_track=1") {
			t.Fatalf("final playlist did not rewrite segment URL: %s", finalized)
		}
	})

	// A copy-video session's segments split on source keyframes, so the playlist
	// has to come from FFmpeg. Synthesizing one advertises durations FFmpeg never
	// produced and segments that will never exist (audit H1/H2).
	t.Run("copy-video serves FFmpeg's own playlist", func(t *testing.T) {
		tempDir := t.TempDir()
		livePlaylist := "#EXTM3U\n#EXT-X-VERSION:7\n#EXT-X-TARGETDURATION:10\n" +
			"#EXT-X-PLAYLIST-TYPE:EVENT\n#EXT-X-MAP:URI=\"init.mp4\"\n" +
			"#EXTINF:8.466792,\nsegment_0.m4s\n#EXTINF:6.006000,\nsegment_1.m4s\n"

		writeErr := os.WriteFile(filepath.Join(tempDir, helpers.HLS_PLAYLIST_FILENAME), []byte(livePlaylist), 0o644)
		if writeErr != nil {
			t.Fatalf("failed to seed playlist: %v", writeErr)
		}

		session := &HLSSession{DurationSec: 600, CopyVideo: true, TempDir: tempDir}

		playlist, err := buildHLSPlaylistBody(t.Context(), session, session.DurationSec, "/api/hls/", "?start=0")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if !strings.Contains(playlist, "#EXTINF:8.466792,") {
			t.Fatalf("real segment duration was not preserved: %s", playlist)
		}
		if strings.Contains(playlist, "#EXTINF:4.000000,") {
			t.Fatalf("playlist still contains synthesized 4s durations: %s", playlist)
		}
		if strings.Count(playlist, ".m4s?start=0") != 2 {
			t.Fatalf("expected exactly the two real segments, got: %s", playlist)
		}
		if strings.Contains(playlist, "#EXT-X-ENDLIST") {
			t.Fatalf("a still-encoding session must not be advertised as complete: %s", playlist)
		}
	})

	// Without a published playlist the request must not fall back to a synthesized
	// one; it retries instead.
	t.Run("copy-video without a playlist yet is retryable", func(t *testing.T) {
		session := &HLSSession{DurationSec: 600, CopyVideo: true, TempDir: t.TempDir()}

		ctx, cancel := context.WithCancel(t.Context())
		cancel()

		_, err := buildHLSPlaylistBody(ctx, session, session.DurationSec, "/api/hls/", "")
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("expected the abandoned request to stop waiting, got %v", err)
		}
	})

	// A copy-video session that exits cleanly without output must 404 rather than
	// hand back a playlist describing segments that do not exist.
	t.Run("copy-video that exited without a playlist reports an empty session", func(t *testing.T) {
		session := &HLSSession{DurationSec: 600, CopyVideo: true, TempDir: t.TempDir()}
		session.Exited = true

		_, err := buildHLSPlaylistBody(t.Context(), session, session.DurationSec, "/api/hls/", "")
		if !errors.Is(err, errHLSSessionEmpty) {
			t.Fatalf("expected an empty-session error, got %v", err)
		}
	})

	// onExit publishes FinalPlaylist before it marks the session exited, so a
	// session that finishes mid-request still serves its real playlist.
	t.Run("copy-video prefers a published final playlist", func(t *testing.T) {
		session := &HLSSession{DurationSec: 600, CopyVideo: true, TempDir: t.TempDir()}
		session.FinalPlaylist = "#EXTM3U\n#EXT-X-MAP:URI=\"init.mp4\"\n" +
			"#EXTINF:8.466792,\nsegment_0.m4s\n#EXT-X-ENDLIST\n"

		playlist, err := buildHLSPlaylistBody(t.Context(), session, session.DurationSec, "/api/hls/", "?start=0")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !strings.Contains(playlist, "/api/hls/init.mp4?start=0") {
			t.Fatalf("final playlist did not rewrite init URL: %s", playlist)
		}
		if !strings.Contains(playlist, "/api/hls/segment_0.m4s?start=0") {
			t.Fatalf("final playlist did not rewrite segment URL: %s", playlist)
		}
	})

	t.Run("copy-video that exited with an error reports a failed session", func(t *testing.T) {
		session := &HLSSession{DurationSec: 600, CopyVideo: true, TempDir: t.TempDir()}
		session.Exited = true
		session.ExitErr = errors.New("ffmpeg exited 1")

		_, err := buildHLSPlaylistBody(t.Context(), session, session.DurationSec, "/api/hls/", "")
		if !errors.Is(err, errHLSSessionFailed) {
			t.Fatalf("expected a failed-session error, got %v", err)
		}
	})

	// The live playlist file outlives the process that was appending to it. It
	// has no ENDLIST, so serving it to a dead session leaves the client
	// reloading a playlist that will never grow while playback sits stalled
	// with no error reported.
	t.Run("copy-video that died does not serve its unterminated live playlist", func(t *testing.T) {
		tempDir := t.TempDir()
		live := "#EXTM3U\n#EXT-X-PLAYLIST-TYPE:EVENT\n#EXT-X-MAP:URI=\"init.mp4\"\n" +
			"#EXTINF:4.000000,\nsegment_0.m4s\n"
		writeErr := os.WriteFile(filepath.Join(tempDir, helpers.HLS_PLAYLIST_FILENAME), []byte(live), 0o644)
		if writeErr != nil {
			t.Fatalf("failed to write live playlist: %v", writeErr)
		}

		session := &HLSSession{DurationSec: 600, CopyVideo: true, TempDir: tempDir}
		session.Exited = true
		session.ExitErr = errors.New("ffmpeg exited 1")

		_, err := buildHLSPlaylistBody(t.Context(), session, session.DurationSec, "/api/hls/", "")
		if !errors.Is(err, errHLSSessionFailed) {
			t.Fatalf("expected a failed-session error, got %v", err)
		}
	})

	// Synthesizing describes output FFmpeg has not produced yet, which is only
	// true while it is still running. A dead transcode used to be handed back a
	// complete playlist, so the client discovered the failure only by waiting
	// out every segment request in turn.
	t.Run("transcode that exited with an error reports a failed session", func(t *testing.T) {
		session := &HLSSession{DurationSec: 600, TempDir: t.TempDir()}
		session.Exited = true
		session.ExitErr = errors.New("ffmpeg exited 1")

		_, err := buildHLSPlaylistBody(t.Context(), session, session.DurationSec, "/api/hls/", "")
		if !errors.Is(err, errHLSSessionFailed) {
			t.Fatalf("expected a failed-session error, got %v", err)
		}
	})

	t.Run("transcode that exited without a playlist reports an empty session", func(t *testing.T) {
		session := &HLSSession{DurationSec: 600, TempDir: t.TempDir()}
		session.Exited = true

		_, err := buildHLSPlaylistBody(t.Context(), session, session.DurationSec, "/api/hls/", "")
		if !errors.Is(err, errHLSSessionEmpty) {
			t.Fatalf("expected an empty-session error, got %v", err)
		}
	})

	t.Run("running transcode still gets a synthesized playlist", func(t *testing.T) {
		session := &HLSSession{DurationSec: 600, TempDir: t.TempDir()}

		playlist, err := buildHLSPlaylistBody(t.Context(), session, session.DurationSec, "/api/hls/", "?start=0")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !strings.Contains(playlist, "/api/hls/segment_0.m4s?start=0") {
			t.Fatalf("synthesized playlist missing segment URL: %s", playlist)
		}
	})

	t.Run("copy-video serves the playlist as soon as FFmpeg publishes it", func(t *testing.T) {
		tempDir := t.TempDir()
		session := &HLSSession{DurationSec: 600, CopyVideo: true, TempDir: tempDir}

		go func() {
			time.Sleep(50 * time.Millisecond)
			livePlaylist := "#EXTM3U\n#EXT-X-TARGETDURATION:10\n#EXT-X-MAP:URI=\"init.mp4\"\n" +
				"#EXTINF:8.466792,\nsegment_0.m4s\n"
			_ = os.WriteFile(filepath.Join(tempDir, helpers.HLS_PLAYLIST_FILENAME), []byte(livePlaylist), 0o644)
		}()

		playlist, err := buildHLSPlaylistBody(t.Context(), session, session.DurationSec, "/api/hls/", "?start=0")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !strings.Contains(playlist, "/api/hls/segment_0.m4s?start=0") {
			t.Fatalf("late playlist was not served: %s", playlist)
		}
	})

	// A session that seeks past the end of a stream exits cleanly having written
	// one empty segment, which FFmpeg declares as `#EXTINF:0.000000` under an
	// invalid `#EXT-X-TARGETDURATION:0`. Serving that would be a valid-looking
	// manifest for unplayable media.
	t.Run("copy-video rejects a degenerate zero-duration playlist", func(t *testing.T) {
		tempDir := t.TempDir()
		degenerate := "#EXTM3U\n#EXT-X-VERSION:7\n#EXT-X-TARGETDURATION:0\n" +
			"#EXT-X-PLAYLIST-TYPE:EVENT\n#EXT-X-MAP:URI=\"init.mp4\"\n" +
			"#EXTINF:0.000000,\nsegment_0.m4s\n#EXT-X-ENDLIST\n"

		writeErr := os.WriteFile(filepath.Join(tempDir, helpers.HLS_PLAYLIST_FILENAME), []byte(degenerate), 0o644)
		if writeErr != nil {
			t.Fatalf("failed to seed playlist: %v", writeErr)
		}

		session := &HLSSession{DurationSec: 30, CopyVideo: true, TempDir: tempDir}
		session.Exited = true

		_, err := buildHLSPlaylistBody(t.Context(), session, session.DurationSec, "/api/hls/", "")
		if !errors.Is(err, errHLSSessionEmpty) {
			t.Fatalf("expected an empty session to be reported as producing nothing, got %v", err)
		}
	})

	// The same past-the-end exit exists for transcodes: onExit publishes whatever
	// FFmpeg wrote, so a finalized playlist can still describe nothing playable.
	t.Run("transcode rejects a degenerate zero-duration final playlist", func(t *testing.T) {
		session := &HLSSession{DurationSec: 30, CopyVideo: false, TempDir: t.TempDir()}
		session.FinalPlaylist = "#EXTM3U\n#EXT-X-VERSION:7\n#EXT-X-TARGETDURATION:0\n" +
			"#EXT-X-MAP:URI=\"init.mp4\"\n#EXTINF:0.000000,\nsegment_0.m4s\n#EXT-X-ENDLIST\n"
		session.Exited = true

		_, err := buildHLSPlaylistBody(t.Context(), session, session.DurationSec, "/api/hls/", "")
		if !errors.Is(err, errHLSSessionEmpty) {
			t.Fatalf("expected an unplayable final playlist to be reported as empty, got %v", err)
		}
	})
}

func TestServeReadyHLSSegment(t *testing.T) {
	tempDir := t.TempDir()
	filename := helpers.HLS_SEGMENT_FILENAME_PREFIX + "0" + helpers.HLS_SEGMENT_FILENAME_SUFFIX
	payload := []byte("segment payload")
	err := os.WriteFile(filepath.Join(tempDir, filename), payload, 0o600)
	if err != nil {
		t.Fatalf("write segment: %v", err)
	}
	// A temp_file session judges a segment by its own final name, so no
	// successor file is needed for it to serve.
	session := &HLSSession{TempDir: tempDir, TempFileSegments: true}

	t.Run("serves completed segment with shared headers", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/segment", nil)
		w := httptest.NewRecorder()
		serveReadyHLSSegment(w, req, session, filename)

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

	t.Run("serves a requested byte range", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/segment", nil)
		req.Header.Set("Range", "bytes=0-6")
		w := httptest.NewRecorder()
		serveReadyHLSSegment(w, req, session, filename)

		if w.Code != http.StatusPartialContent {
			t.Fatalf("status = %d, want 206: %s", w.Code, w.Body.String())
		}
		if got := w.Header().Get("Accept-Ranges"); got != "bytes" {
			t.Fatalf("Accept-Ranges = %q, want bytes", got)
		}
		wantContentRange := fmt.Sprintf("bytes 0-6/%d", len(payload))
		if got := w.Header().Get("Content-Range"); got != wantContentRange {
			t.Fatalf("Content-Range = %q, want %q", got, wantContentRange)
		}
		if w.Body.String() != "segment" {
			t.Fatalf("body = %q, want segment", w.Body.String())
		}
	})

	t.Run("rejects an unsatisfiable byte range", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/segment", nil)
		req.Header.Set("Range", fmt.Sprintf("bytes=%d-", len(payload)))
		w := httptest.NewRecorder()
		serveReadyHLSSegment(w, req, session, filename)

		if w.Code != http.StatusRequestedRangeNotSatisfiable {
			t.Fatalf("status = %d, want 416: %s", w.Code, w.Body.String())
		}
		wantContentRange := fmt.Sprintf("bytes */%d", len(payload))
		if got := w.Header().Get("Content-Range"); got != wantContentRange {
			t.Fatalf("Content-Range = %q, want %q", got, wantContentRange)
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

	// The init segment is gated on segment_0 existing, so a session that died
	// between writing the two used to satisfy neither the ready check nor the
	// exit check: the request polled for the full hlsSegmentWait and answered
	// 503, even though init.mp4 was on disk and could never change again.
	t.Run("serves a final init segment when FFmpeg died before segment_0", func(t *testing.T) {
		tempDir := t.TempDir()
		payload := []byte("init-bytes")
		writeErr := os.WriteFile(filepath.Join(tempDir, helpers.HLS_INIT_FILENAME), payload, 0o644)
		if writeErr != nil {
			t.Fatalf("failed to write init segment: %v", writeErr)
		}

		session := &HLSSession{TempDir: tempDir, Exited: true, ExitErr: fmt.Errorf("ffmpeg failed")}
		req := httptest.NewRequest(http.MethodGet, "/init", nil)
		w := httptest.NewRecorder()

		done := make(chan struct{})
		go func() {
			defer close(done)
			serveReadyHLSSegment(w, req, session, helpers.HLS_INIT_FILENAME)
		}()

		select {
		case <-done:
		case <-time.After(5 * time.Second):
			t.Fatal("init request waited on a segment a dead session will never write")
		}

		if w.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200: %s", w.Code, w.Body.String())
		}
		if w.Body.String() != string(payload) {
			t.Fatalf("body = %q, want %q", w.Body.String(), payload)
		}
	})

	// A seek abandons the in-flight segment request. Without honouring
	// cancellation the goroutine keeps polling for hlsSegmentWait, so scrubbing
	// accumulates them.
	t.Run("returns as soon as the client goes away", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		req := httptest.NewRequest(http.MethodGet, "/segment", nil).WithContext(ctx)
		w := httptest.NewRecorder()

		done := make(chan struct{})
		go func() {
			defer close(done)
			serveReadyHLSSegment(w, req, &HLSSession{TempDir: t.TempDir()}, helpers.HLS_SEGMENT_FILENAME_PREFIX+"0"+helpers.HLS_SEGMENT_FILENAME_SUFFIX)
		}()

		select {
		case <-done:
		case <-time.After(5 * time.Second):
			t.Fatal("segment request kept polling after the client went away")
		}

		if w.Code != http.StatusOK || w.Body.Len() != 0 {
			t.Fatalf("abandoned request wrote status %d and %d bytes, want nothing", w.Code, w.Body.Len())
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

func TestSegmentReadyTempFile(t *testing.T) {
	prefix := helpers.HLS_SEGMENT_FILENAME_PREFIX
	suffix := helpers.HLS_SEGMENT_FILENAME_SUFFIX
	segName := func(n int) string {
		return fmt.Sprintf("%s%d%s", prefix, n, suffix)
	}
	newSession := func(dir string) *HLSSession {
		return &HLSSession{TempDir: dir, TempFileSegments: true}
	}

	t.Run("a segment is ready by its own final name", func(t *testing.T) {
		dir := t.TempDir()
		session := newSession(dir)

		if segmentReady(session, segName(0)) {
			t.Error("missing segment must not be ready")
		}

		err := os.WriteFile(filepath.Join(dir, segName(0)), []byte{0x01}, 0o644)
		if err != nil {
			t.Fatal(err)
		}

		if !segmentReady(session, segName(0)) {
			t.Error("a renamed segment is complete and must be ready without a successor")
		}
	})

	t.Run("an empty segment file is not ready", func(t *testing.T) {
		dir := t.TempDir()
		err := os.WriteFile(filepath.Join(dir, segName(0)), nil, 0o644)
		if err != nil {
			t.Fatal(err)
		}

		if segmentReady(newSession(dir), segName(0)) {
			t.Error("zero-byte segment must not be ready")
		}
	})

	t.Run("init is not ready on existence alone", func(t *testing.T) {
		dir := t.TempDir()
		err := os.WriteFile(filepath.Join(dir, helpers.HLS_INIT_FILENAME), []byte{0x01}, 0o644)
		if err != nil {
			t.Fatal(err)
		}

		// The hls muxer opens init.mp4 under its final name directly, so
		// existence does not prove it was closed.
		if segmentReady(newSession(dir), helpers.HLS_INIT_FILENAME) {
			t.Error("init must not be ready without evidence FFmpeg moved past it")
		}
	})

	t.Run("init is ready once segment_0's temp file appears", func(t *testing.T) {
		dir := t.TempDir()
		session := newSession(dir)
		err := os.WriteFile(filepath.Join(dir, helpers.HLS_INIT_FILENAME), []byte{0x01}, 0o644)
		if err != nil {
			t.Fatal(err)
		}

		// The muxer opens segment_0's .tmp only after closing init.mp4, so
		// even an empty .tmp is proof init is final.
		err = os.WriteFile(filepath.Join(dir, segName(0)+hlsTempFileSuffix), nil, 0o644)
		if err != nil {
			t.Fatal(err)
		}

		if !segmentReady(session, helpers.HLS_INIT_FILENAME) {
			t.Error("init must be ready once segment_0.m4s.tmp exists")
		}
	})

	t.Run("init is ready once segment_0's final name appears", func(t *testing.T) {
		dir := t.TempDir()
		session := newSession(dir)
		err := os.WriteFile(filepath.Join(dir, helpers.HLS_INIT_FILENAME), []byte{0x01}, 0o644)
		if err != nil {
			t.Fatal(err)
		}
		err = os.WriteFile(filepath.Join(dir, segName(0)), []byte{0x01}, 0o644)
		if err != nil {
			t.Fatal(err)
		}

		if !segmentReady(session, helpers.HLS_INIT_FILENAME) {
			t.Error("init must be ready once segment_0.m4s exists")
		}
	})

	t.Run("an empty init file is never ready", func(t *testing.T) {
		dir := t.TempDir()
		session := newSession(dir)
		err := os.WriteFile(filepath.Join(dir, helpers.HLS_INIT_FILENAME), nil, 0o644)
		if err != nil {
			t.Fatal(err)
		}
		err = os.WriteFile(filepath.Join(dir, segName(0)), []byte{0x01}, 0o644)
		if err != nil {
			t.Fatal(err)
		}

		if segmentReady(session, helpers.HLS_INIT_FILENAME) {
			t.Error("zero-byte init must not be ready even with segment_0 present")
		}
	})

	t.Run("a dead session's init is final", func(t *testing.T) {
		dir := t.TempDir()
		session := newSession(dir)
		session.Exited = true

		if segmentReady(session, helpers.HLS_INIT_FILENAME) {
			t.Error("a missing init must not be ready even after exit")
		}

		err := os.WriteFile(filepath.Join(dir, helpers.HLS_INIT_FILENAME), []byte{0x01}, 0o644)
		if err != nil {
			t.Fatal(err)
		}

		if !segmentReady(session, helpers.HLS_INIT_FILENAME) {
			t.Error("a dead session that wrote init but never opened segment_0 leaves init final")
		}
	})

	t.Run("a fallback session delegates to the successor heuristic", func(t *testing.T) {
		dir := t.TempDir()
		legacy := &HLSSession{TempDir: dir}
		err := os.WriteFile(filepath.Join(dir, segName(0)), []byte{0x01}, 0o644)
		if err != nil {
			t.Fatal(err)
		}

		if segmentReady(legacy, segName(0)) {
			t.Error("without temp_file a segment must wait for its successor")
		}

		err = os.WriteFile(filepath.Join(dir, segName(1)), []byte{0x01}, 0o644)
		if err != nil {
			t.Fatal(err)
		}

		if !segmentReady(legacy, segName(0)) {
			t.Error("legacy readiness must clear once the successor exists")
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

	req := httptest.NewRequest(
		http.MethodGet,
		fmt.Sprintf("/api/movies/%d/hls/remux/playlist.m3u8?audio_track=0&playback_session=%s&start=0", movieID, testPlaybackSessionID),
		nil,
	)
	addOpenAPITestCookie(req)
	recorder := httptest.NewRecorder()

	newHLSTestHandler(t, app, userID).ServeHTTP(recorder, req)

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
	app.FFmpeg = &fakeFFmpeg{plans: []fakeFFmpegRunPlan{{
		WriteFiles: func(outDir string) error {
			// The full fixture first, so the session models a transcode that
			// produced a playlist rather than one that exited having written
			// nothing, then the recognizable segment body this test asserts on.
			err := writeTestHLSFixture(outDir, transcodeFixture)
			if err != nil {
				return err
			}
			segmentPath := filepath.Join(outDir, helpers.HLS_SEGMENT_FILENAME_PREFIX+"0"+helpers.HLS_SEGMENT_FILENAME_SUFFIX)
			return os.WriteFile(segmentPath, []byte("effective-segment"), 0o644)
		},
	}}}

	movieID := insertTestHLSMovieFixture(t, app, "h264", 1080)
	userID := int64(42)
	audioTrack := 0
	effectiveStart := 7200 - hlsStartClampTailSec

	handler := newHLSTestHandler(t, app, userID)

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

func TestHLSManifest_RepeatedRequestsReusePersonalSession(t *testing.T) {
	app := setupTestApp(t)
	defer app.DB.Close()
	ffmpegRunner := &fakeFFmpeg{plans: []fakeFFmpegRunPlan{hlsRunPlan(transcodeFixture)}}
	app.FFmpeg = ffmpegRunner

	movieID := insertTestHLSMovieFixture(t, app, "h264", 1080)
	userID := int64(42)

	handler := newHLSTestHandler(t, app, userID)

	manifestURL := fmt.Sprintf(
		"/api/movies/%d/hls/%s/playlist.m3u8?audio_track=0&playback_session=%s&start=590",
		movieID,
		helpers.HLS_PROFILE_720P_3MBPS,
		testPlaybackSessionID,
	)
	for requestNumber := 1; requestNumber <= 2; requestNumber++ {
		recorder := httptest.NewRecorder()
		handler.ServeHTTP(
			recorder,
			httptest.NewRequest(http.MethodGet, manifestURL, nil),
		)
		if recorder.Code != http.StatusOK {
			t.Fatalf(
				"manifest request %d status = %d, want 200: %s",
				requestNumber,
				recorder.Code,
				recorder.Body.String(),
			)
		}
	}

	if calls := ffmpegRunner.CallCount(); calls != 1 {
		t.Fatalf("FFmpeg calls = %d, want 1 for repeated session URL", calls)
	}
}

func TestHLSSegment_UsesRequestedRemuxKeyWhenEffectiveProfileFallsBack(t *testing.T) {
	app := setupTestApp(t)
	defer app.DB.Close()

	audioTrack := 0
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

	req := httptest.NewRequest(
		http.MethodGet,
		fmt.Sprintf("/api/movies/5/hls/remux/segment_0.m4s?audio_track=0&playback_session=%s&start=0", testPlaybackSessionID),
		nil,
	)
	addOpenAPITestCookie(req)
	recorder := httptest.NewRecorder()

	newHLSTestHandler(t, app, userID).ServeHTTP(recorder, req)

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

	handler := newHLSTestHandler(t, app, userID)
	req := httptest.NewRequest(
		http.MethodPost,
		fmt.Sprintf("/api/movies/5/hls/session/stop?playback_session=%s", testPlaybackSessionID),
		nil,
	)
	addOpenAPITestCookie(req)
	recorder := httptest.NewRecorder()

	handler.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", recorder.Code, recorder.Body.String())
	}
	assertOpenAPIExchange(t, "stopPersonalHlsSession", req, recorder)
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

	handler := newHLSTestHandler(t, app, 100)
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

	handler := newHLSTestHandler(t, app, userID)
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

func TestHLSManifest_RejectsUnauthenticatedRequests(t *testing.T) {
	app := setupTestApp(t)
	defer app.DB.Close()
	app.InitSession()

	router := chi.NewRouter()
	router.Get("/api/movies/{id}/hls/{profile}/"+helpers.HLS_PLAYLIST_FILENAME, app.HLSManifest)
	handler := app.SessionManager.LoadAndSave(router)

	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, httptest.NewRequest(
		http.MethodGet,
		"/api/movies/7/hls/720p_3mbps/playlist.m3u8?playback_session="+testPlaybackSessionID+"&start=0",
		nil,
	))

	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401: %s", recorder.Code, recorder.Body.String())
	}
}

func TestHLSManifest_SurfacesSessionCreationFailure(t *testing.T) {
	app := setupTestApp(t)
	defer app.DB.Close()
	app.FFmpeg = &fakeFFmpeg{}

	result, err := app.DB.Exec(`
		INSERT INTO movies (title, file_path, file_name, size, container, mime_type, adult)
		VALUES ('No Duration', '/tmp/manifest-nodur.mkv', 'manifest-nodur.mkv', 1, 'mkv', 'video/x-matroska', 0)
	`)
	if err != nil {
		t.Fatalf("insert movie: %v", err)
	}
	movieID, err := result.LastInsertId()
	if err != nil {
		t.Fatalf("last insert id: %v", err)
	}

	recorder := httptest.NewRecorder()
	newHLSTestHandler(t, app, 42).ServeHTTP(recorder, httptest.NewRequest(
		http.MethodGet,
		fmt.Sprintf("/api/movies/%d/hls/720p_3mbps/playlist.m3u8?playback_session=%s&start=0", movieID, testPlaybackSessionID),
		nil,
	))

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400: %s", recorder.Code, recorder.Body.String())
	}
	if !strings.Contains(recorder.Body.String(), "no valid duration") {
		t.Fatalf("body = %s, want the session failure reason", recorder.Body.String())
	}
}

func TestHLSSegment_RejectsBadRequests(t *testing.T) {
	app := setupTestApp(t)
	defer app.DB.Close()

	audioTrack := 0
	userID := int64(100)
	handler := newHLSTestHandler(t, app, userID)
	segmentURL := func(filename string) string {
		return fmt.Sprintf(
			"/api/movies/5/hls/remux/%s?audio_track=0&playback_session=%s&start=0",
			filename,
			testPlaybackSessionID,
		)
	}

	t.Run("rejects a filename outside the segment naming scheme", func(t *testing.T) {
		recorder := httptest.NewRecorder()
		handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, segmentURL("garbage.txt"), nil))

		if recorder.Code != http.StatusBadRequest {
			t.Fatalf("status = %d, want 400: %s", recorder.Code, recorder.Body.String())
		}
		if !strings.Contains(recorder.Body.String(), "invalid segment filename") {
			t.Fatalf("body = %s, want the filename rejection", recorder.Body.String())
		}
	})

	t.Run("reports an uncached session as not found", func(t *testing.T) {
		recorder := httptest.NewRecorder()
		handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, segmentURL("segment_0.m4s"), nil))

		if recorder.Code != http.StatusNotFound {
			t.Fatalf("status = %d, want 404: %s", recorder.Code, recorder.Body.String())
		}
	})

	// A cache entry of the wrong type can only come from a bug, but leaving it
	// in place would make every later request for this key fail the same way.
	t.Run("evicts a cache entry that is not a session", func(t *testing.T) {
		key := HLSSessionKey(5, helpers.HLS_PROFILE_REMUX, &audioTrack, testPlaybackSessionID, 0)
		app.HLSSessionCache.SetDefault(key, "not a session")

		recorder := httptest.NewRecorder()
		handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, segmentURL("segment_0.m4s"), nil))

		if recorder.Code != http.StatusNotFound {
			t.Fatalf("status = %d, want 404: %s", recorder.Code, recorder.Body.String())
		}
		if _, cached := app.HLSSessionCache.Get(key); cached {
			t.Fatal("expected the unusable cache entry to be evicted")
		}
	})
}

func TestStopPersonalHLSSession_RejectsBadRequests(t *testing.T) {
	app := setupTestApp(t)
	defer app.DB.Close()

	t.Run("rejects an unauthenticated caller", func(t *testing.T) {
		app.InitSession()
		router := chi.NewRouter()
		router.Post("/api/movies/{id}/hls/session/stop", app.StopPersonalHLSSession)

		recorder := httptest.NewRecorder()
		app.SessionManager.LoadAndSave(router).ServeHTTP(recorder, httptest.NewRequest(
			http.MethodPost,
			"/api/movies/5/hls/session/stop?playback_session="+testPlaybackSessionID,
			nil,
		))

		if recorder.Code != http.StatusUnauthorized {
			t.Fatalf("status = %d, want 401: %s", recorder.Code, recorder.Body.String())
		}
	})

	t.Run("rejects a non-numeric movie id", func(t *testing.T) {
		recorder := httptest.NewRecorder()
		newHLSTestHandler(t, app, 100).ServeHTTP(recorder, httptest.NewRequest(
			http.MethodPost,
			"/api/movies/abc/hls/session/stop?playback_session="+testPlaybackSessionID,
			nil,
		))

		if recorder.Code != http.StatusBadRequest {
			t.Fatalf("status = %d, want 400: %s", recorder.Code, recorder.Body.String())
		}
	})
}

func TestHasPlayableSegment(t *testing.T) {
	cases := []struct {
		name     string
		playlist string
		want     bool
	}{
		{"real segment", "#EXTINF:8.466792,\nsegment_0.m4s\n", true},
		{"zero duration only", "#EXTINF:0.000000,\nsegment_0.m4s\n", false},
		{"no segments", "#EXTM3U\n#EXT-X-TARGETDURATION:4\n", false},
		{"zero then real", "#EXTINF:0.000000,\nseg0\n#EXTINF:4.000000,\nseg1\n", true},
		{"malformed duration", "#EXTINF:abc,\nsegment_0.m4s\n", false},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			got := hasPlayableSegment(testCase.playlist)
			if got != testCase.want {
				t.Fatalf("hasPlayableSegment = %v, want %v", got, testCase.want)
			}
		})
	}
}

func TestWriteHLSPlaylistHeaders_PublishesEffectiveProfileAndStart(t *testing.T) {
	session := &HLSSession{
		EffectiveProfile: "1080p_8mbps",
		ActualStartSec:   591.174,
	}

	recorder := httptest.NewRecorder()
	writeHLSPlaylistHeaders(recorder, session)

	if got := recorder.Header().Get(hlsEffectiveProfileHeader); got != "1080p_8mbps" {
		t.Fatalf("effective profile header = %q, want 1080p_8mbps", got)
	}

	start, err := strconv.ParseFloat(recorder.Header().Get(hlsActualStartHeader), 64)
	if err != nil {
		t.Fatalf("actual start header did not parse: %v", err)
	}
	if start != 591.174 {
		t.Fatalf("actual start header = %v, want 591.174", start)
	}
}

func TestWriteHLSPlaylistHeaders_OmitsUnknownStart(t *testing.T) {
	session := &HLSSession{EffectiveProfile: "remux", ActualStartSec: hlsUnknownActualStart}

	recorder := httptest.NewRecorder()
	writeHLSPlaylistHeaders(recorder, session)

	if got := recorder.Header().Get(hlsActualStartHeader); got != "" {
		t.Fatalf("unknown start must not be published, got %q", got)
	}
}

func TestLogFirstHLSSegmentServed(t *testing.T) {
	var buf bytes.Buffer
	session := &HLSSession{
		TempDir:          "/transcode/igloo-hls-abc",
		MovieID:          7,
		Logger:           slog.New(slog.NewTextHandler(&buf, nil)),
		StartedAt:        time.Now(),
		EffectiveProfile: helpers.HLS_PROFILE_720P_3MBPS,
	}

	logFirstHLSSegmentServed(session, "init.mp4", time.Now())
	logFirstHLSSegmentServed(session, "segment_0.m4s", time.Now())

	if got := strings.Count(buf.String(), "hls first segment served"); got != 1 {
		t.Fatalf("first-serve log emitted %d times, want exactly once:\n%s", got, buf.String())
	}
	if !strings.Contains(buf.String(), "filename=init.mp4") {
		t.Fatalf("log should name the first served file:\n%s", buf.String())
	}

	// Bare sessions, as tests build them, must neither log nor panic.
	logFirstHLSSegmentServed(&HLSSession{TempDir: "x"}, "init.mp4", time.Now())
	logFirstHLSSegmentServed(&HLSSession{TempDir: "x", Logger: session.Logger}, "init.mp4", time.Now())
	if strings.Count(buf.String(), "hls first segment served") != 1 {
		t.Fatalf("bare sessions must not log:\n%s", buf.String())
	}
}
