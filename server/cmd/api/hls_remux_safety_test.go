package main

import (
	"context"
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

	const baseVersion = "7.0.2-Jellyfin"

	baseKey := remuxSafetyFingerprint(&baseMovie, &baseVideo, baseVersion)
	if got := remuxSafetyFingerprint(&baseMovie, &baseVideo, baseVersion); got != baseKey {
		t.Fatalf("fingerprint not stable: %q vs %q", got, baseKey)
	}

	// The verdict validates FFmpeg-generated fMP4 output, so a different muxer
	// must not inherit it.
	t.Run("ffmpeg version", func(t *testing.T) {
		if got := remuxSafetyFingerprint(&baseMovie, &baseVideo, "7.1-Jellyfin"); got == baseKey {
			t.Fatal("fingerprint unchanged after an ffmpeg version change")
		}
	})

	t.Run("producer revision", func(t *testing.T) {
		want := fmt.Sprintf(":p%d:%s", remuxVerdictProducerRevision, baseVersion)
		if !strings.HasSuffix(baseKey, want) {
			t.Fatalf("fingerprint %q does not end with the producer terms %q", baseKey, want)
		}
	})

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
		// A rescan that newly discovers interlacing must kill a stale safe
		// verdict, since the gate now rejects interlaced streams.
		{"field order", func(_ *database.Movie, v *database.VideoStream) {
			v.FieldOrder = sql.NullString{String: "tt", Valid: true}
		}},
		{"movie size", func(m *database.Movie, _ *database.VideoStream) { m.Size = 2_000_000 }},
		{"updated at", func(m *database.Movie, _ *database.VideoStream) { m.UpdatedAt = "2026-07-02" }},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			movie := baseMovie
			video := baseVideo
			tt.mutate(&movie, &video)
			if got := remuxSafetyFingerprint(&movie, &video, baseVersion); got == baseKey {
				t.Fatalf("fingerprint unchanged after %s change", tt.name)
			}
		})
	}
}

func TestRemuxSafetyVerdictStore(t *testing.T) {
	app := setupTestApp(t)
	defer app.DB.Close()

	ctx := context.Background()
	movieID := insertTestHLSMovieFixture(t, app, "h264", 1080)
	const streamIndex = int64(0)
	const fingerprint = "fingerprint-a"

	t.Run("returns a persisted verdict", func(t *testing.T) {
		app.setRemuxSafetyVerdict(movieID, streamIndex, fingerprint, false, "10-bit H.264")

		verdict, ok := app.getRemuxSafetyVerdict(ctx, movieID, streamIndex, fingerprint)
		if !ok {
			t.Fatal("persisted verdict was not returned")
		}
		if verdict.Safe {
			t.Fatal("Safe = true, want the persisted unsafe verdict")
		}
		if verdict.Reason != "10-bit H.264" {
			t.Fatalf("Reason = %q, want the persisted reason", verdict.Reason)
		}
	})

	// A stale fingerprint means the file or its stream rows changed since the
	// verdict was computed; serving it could remux a stream that is no longer
	// safe.
	t.Run("treats a changed fingerprint as a miss", func(t *testing.T) {
		_, ok := app.getRemuxSafetyVerdict(ctx, movieID, streamIndex, "fingerprint-b")
		if ok {
			t.Fatal("a stale fingerprint was reported as a hit")
		}
	})

	t.Run("reports an absent row as a miss", func(t *testing.T) {
		_, ok := app.getRemuxSafetyVerdict(ctx, movieID, streamIndex+1, fingerprint)
		if ok {
			t.Fatal("an absent verdict was reported as a hit")
		}
	})

	t.Run("upserts over the previous verdict", func(t *testing.T) {
		// Written here rather than relied on from the first subtest: the row
		// count below only proves an upsert when a row already existed, and
		// this subtest must hold under -run on its own.
		app.setRemuxSafetyVerdict(movieID, streamIndex, fingerprint, false, "10-bit H.264")
		app.setRemuxSafetyVerdict(movieID, streamIndex, "fingerprint-b", true, "validated safe remux")

		verdict, ok := app.getRemuxSafetyVerdict(ctx, movieID, streamIndex, "fingerprint-b")
		if !ok {
			t.Fatal("upserted verdict was not returned")
		}
		if !verdict.Safe {
			t.Fatal("Safe = false, want the upserted safe verdict")
		}

		var count int
		err := app.DB.QueryRowContext(ctx, `
			SELECT COUNT(*) FROM remux_safety_verdicts WHERE movie_id = ? AND stream_index = ?
		`, movieID, streamIndex).Scan(&count)
		if err != nil {
			t.Fatalf("count verdict rows: %v", err)
		}
		if count != 1 {
			t.Fatalf("verdict row count = %d, want the upsert to keep a single row", count)
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
