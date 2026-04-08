package main

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"igloo/cmd/internal/helpers"
)

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
	key := HLSSessionKey(123, "720p_3mbps", 2)
	want := "123:720p_3mbps:2"
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

func TestResolveHLSDiskFilename(t *testing.T) {
	session := &HLSSession{StartSegment: 69}

	tests := []struct {
		name     string
		filename string
		want     string
		wantErr  bool
	}{
		{
			name:     "init file is unchanged",
			filename: helpers.HLS_INIT_FILENAME,
			want:     helpers.HLS_INIT_FILENAME,
		},
		{
			name:     "start segment maps to first disk segment",
			filename: "segment_69.m4s",
			want:     "segment_0.m4s",
		},
		{
			name:     "later segment maps relative to start segment",
			filename: "segment_99.m4s",
			want:     "segment_30.m4s",
		},
		{
			name:     "request before session start returns error",
			filename: "segment_0.m4s",
			wantErr:  true,
		},
		{
			name:     "invalid segment name returns error",
			filename: "bad_name.m4s",
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := resolveHLSDiskFilename(session, tt.filename)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("resolveHLSDiskFilename(%q) expected error", tt.filename)
				}
				return
			}
			if err != nil {
				t.Fatalf("resolveHLSDiskFilename(%q) unexpected error: %v", tt.filename, err)
			}
			if got != tt.want {
				t.Fatalf("resolveHLSDiskFilename(%q) = %q, want %q", tt.filename, got, tt.want)
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
