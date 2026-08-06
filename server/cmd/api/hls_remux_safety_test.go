package main

import (
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"igloo/cmd/internal/database"
	"igloo/cmd/internal/helpers"
)

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

func TestRemuxSafetyFingerprint_ChangesWithStreamProperties(t *testing.T) {
	baseMovie := database.Movie{ID: 7, Size: 1_000_000, UpdatedAt: "2026-07-01"}
	baseVideo := database.VideoStream{
		StreamIndex:  0,
		Codec:        "h264",
		CodecProfile: sql.NullString{String: "High", Valid: true},
		BitDepth:     sql.NullInt64{Int64: 8, Valid: true},
		PixelFormat:  sql.NullString{String: "yuv420p", Valid: true},
	}

	baseKey := remuxSafetyFingerprint(&baseMovie, &baseVideo)
	if got := remuxSafetyFingerprint(&baseMovie, &baseVideo); got != baseKey {
		t.Fatalf("fingerprint not stable: %q vs %q", got, baseKey)
	}

	tests := []struct {
		name   string
		mutate func(m *database.Movie, v *database.VideoStream)
	}{
		{"codec", func(_ *database.Movie, v *database.VideoStream) { v.Codec = "hevc" }},
		{"codec profile", func(_ *database.Movie, v *database.VideoStream) {
			v.CodecProfile = sql.NullString{String: "High 10", Valid: true}
		}},
		{"bit depth", func(_ *database.Movie, v *database.VideoStream) {
			v.BitDepth = sql.NullInt64{Int64: 10, Valid: true}
		}},
		{"pixel format", func(_ *database.Movie, v *database.VideoStream) {
			v.PixelFormat = sql.NullString{String: "yuv420p10le", Valid: true}
		}},
		{"movie size", func(m *database.Movie, _ *database.VideoStream) { m.Size = 2_000_000 }},
		{"updated at", func(m *database.Movie, _ *database.VideoStream) { m.UpdatedAt = "2026-07-02" }},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			movie := baseMovie
			video := baseVideo
			tt.mutate(&movie, &video)
			if got := remuxSafetyFingerprint(&movie, &video); got == baseKey {
				t.Fatalf("fingerprint unchanged after %s change", tt.name)
			}
		})
	}
}

func TestRemuxSafetyVerdictCache(t *testing.T) {
	app := setupTestApp(t)
	defer app.DB.Close()

	t.Run("returns a stored verdict", func(t *testing.T) {
		app.setRemuxSafetyVerdict("stored", false, "10-bit H.264")

		verdict, ok := app.getRemuxSafetyVerdict("stored")
		if !ok {
			t.Fatal("stored verdict was not returned")
		}
		if verdict.Safe {
			t.Fatal("Safe = true, want the stored unsafe verdict")
		}
		if verdict.Reason != "10-bit H.264" {
			t.Fatalf("Reason = %q, want the stored reason", verdict.Reason)
		}
	})

	t.Run("reports an unknown key as a miss", func(t *testing.T) {
		_, ok := app.getRemuxSafetyVerdict("never-stored")
		if ok {
			t.Fatal("an unknown key was reported as a hit")
		}
	})

	// An entry of the wrong type can only come from a bug, but leaving it cached
	// would pin the movie to a permanent miss for the whole 24h TTL.
	t.Run("evicts an entry that is not a verdict", func(t *testing.T) {
		app.RemuxSafetyCache.SetDefault("poisoned", "not a verdict")

		_, ok := app.getRemuxSafetyVerdict("poisoned")
		if ok {
			t.Fatal("a poisoned entry was reported as a hit")
		}
		if _, cached := app.RemuxSafetyCache.Get("poisoned"); cached {
			t.Fatal("expected the poisoned entry to be evicted")
		}
	})
}

func TestWaitForRemuxPreflight(t *testing.T) {
	const segmentCount = helpers.HLS_REMUX_PREVALIDATE_SEGMENTS

	// segmentComplete treats a segment as done once the next one exists, so
	// proving N segments complete needs N+1 files or an exited session.
	writeSegments := func(t *testing.T, dir string, through int) {
		t.Helper()
		for i := 0; i <= through; i++ {
			name := fmt.Sprintf("%s%d%s", helpers.HLS_SEGMENT_FILENAME_PREFIX, i, helpers.HLS_SEGMENT_FILENAME_SUFFIX)
			err := os.WriteFile(filepath.Join(dir, name), []byte{0x01}, 0o600)
			if err != nil {
				t.Fatalf("write %s: %v", name, err)
			}
		}
	}
	writeInit := func(t *testing.T, dir string) {
		t.Helper()
		err := os.WriteFile(filepath.Join(dir, helpers.HLS_INIT_FILENAME), []byte{0x01}, 0o600)
		if err != nil {
			t.Fatalf("write init: %v", err)
		}
	}

	exitErr := errors.New("ffmpeg exited 1")

	t.Run("accepts a complete preflight", func(t *testing.T) {
		dir := t.TempDir()
		writeInit(t, dir)
		writeSegments(t, dir, segmentCount-1)
		session := &HLSSession{TempDir: dir, Exited: true}

		err := waitForRemuxPreflight(session, segmentCount, time.Second)
		if err != nil {
			t.Fatalf("waitForRemuxPreflight returned error: %v", err)
		}
	})

	t.Run("reports a missing init segment", func(t *testing.T) {
		session := &HLSSession{TempDir: t.TempDir(), Exited: true}

		err := waitForRemuxPreflight(session, segmentCount, time.Second)
		if err == nil || !strings.Contains(err.Error(), "init segment was not generated") {
			t.Fatalf("error = %v, want the missing-init message", err)
		}
	})

	t.Run("wraps the ffmpeg error when init is missing", func(t *testing.T) {
		session := &HLSSession{TempDir: t.TempDir(), Exited: true, ExitErr: exitErr}

		err := waitForRemuxPreflight(session, segmentCount, time.Second)
		if !errors.Is(err, exitErr) {
			t.Fatalf("error = %v, want it to wrap the ffmpeg exit error", err)
		}
	})

	t.Run("names the segment that never completed", func(t *testing.T) {
		dir := t.TempDir()
		writeInit(t, dir)
		writeSegments(t, dir, segmentCount-2)
		session := &HLSSession{TempDir: dir, Exited: true}

		err := waitForRemuxPreflight(session, segmentCount, time.Second)
		wantName := fmt.Sprintf("%s%d%s", helpers.HLS_SEGMENT_FILENAME_PREFIX, segmentCount-1, helpers.HLS_SEGMENT_FILENAME_SUFFIX)
		if err == nil || !strings.Contains(err.Error(), wantName) {
			t.Fatalf("error = %v, want it to name %q", err, wantName)
		}
	})

	t.Run("wraps the ffmpeg error when a segment is missing", func(t *testing.T) {
		dir := t.TempDir()
		writeInit(t, dir)
		writeSegments(t, dir, segmentCount-2)
		session := &HLSSession{TempDir: dir, Exited: true, ExitErr: exitErr}

		err := waitForRemuxPreflight(session, segmentCount, time.Second)
		if !errors.Is(err, exitErr) {
			t.Fatalf("error = %v, want it to wrap the ffmpeg exit error", err)
		}
	})

	// A session that is still running has to be given up on eventually, or a
	// stalled remux would hold the request open indefinitely.
	t.Run("times out on a session that is still running", func(t *testing.T) {
		dir := t.TempDir()
		writeInit(t, dir)
		session := &HLSSession{TempDir: dir}

		err := waitForRemuxPreflight(session, segmentCount, time.Millisecond)
		if err == nil || !strings.Contains(err.Error(), "timed out waiting for") {
			t.Fatalf("error = %v, want a timeout", err)
		}
	})
}
