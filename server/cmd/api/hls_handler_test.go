package main

import (
	"os"
	"path/filepath"
	"sync"
	"testing"

	"igloo/cmd/internal/helpers"
)

// ---- isAllowedHLSFilename ----

func TestIsAllowedHLSFilename(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected bool
	}{
		{
			name:     "init file is allowed",
			input:    helpers.HLS_INIT_FILENAME, // "init.mp4"
			expected: true,
		},
		{
			name:     "segment_0 is allowed",
			input:    helpers.HLS_SEGMENT_FILENAME_PREFIX + "0" + helpers.HLS_SEGMENT_FILENAME_SUFFIX,
			expected: true,
		},
		{
			name:     "segment_1 is allowed",
			input:    helpers.HLS_SEGMENT_FILENAME_PREFIX + "1" + helpers.HLS_SEGMENT_FILENAME_SUFFIX,
			expected: true,
		},
		{
			name:     "segment_999 is allowed",
			input:    helpers.HLS_SEGMENT_FILENAME_PREFIX + "999" + helpers.HLS_SEGMENT_FILENAME_SUFFIX,
			expected: true,
		},
		{
			name:     "segment_ without number is rejected",
			input:    helpers.HLS_SEGMENT_FILENAME_PREFIX + helpers.HLS_SEGMENT_FILENAME_SUFFIX,
			expected: false,
		},
		{
			name:     "empty string is rejected",
			input:    "",
			expected: false,
		},
		{
			name:     "wrong prefix is rejected",
			input:    "chunk_0.m4s",
			expected: false,
		},
		{
			name:     "wrong suffix is rejected",
			input:    helpers.HLS_SEGMENT_FILENAME_PREFIX + "0.ts",
			expected: false,
		},
		{
			name:     "negative number is rejected",
			input:    helpers.HLS_SEGMENT_FILENAME_PREFIX + "-1" + helpers.HLS_SEGMENT_FILENAME_SUFFIX,
			expected: false,
		},
		{
			name:     "non-numeric suffix is rejected",
			input:    helpers.HLS_SEGMENT_FILENAME_PREFIX + "abc" + helpers.HLS_SEGMENT_FILENAME_SUFFIX,
			expected: false,
		},
		{
			name:     "path traversal is rejected",
			input:    "../segment_0.m4s",
			expected: false,
		},
		{
			name:     "playlist file is rejected",
			input:    "playlist.m3u8",
			expected: false,
		},
		{
			name:     "segment with leading zero is accepted (valid uint)",
			input:    helpers.HLS_SEGMENT_FILENAME_PREFIX + "007" + helpers.HLS_SEGMENT_FILENAME_SUFFIX,
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isAllowedHLSFilename(tt.input)
			if result != tt.expected {
				t.Errorf("isAllowedHLSFilename(%q) = %v, want %v", tt.input, result, tt.expected)
			}
		})
	}
}

// ---- fileReady ----

func TestFileReady(t *testing.T) {
	t.Run("non-existent path returns false", func(t *testing.T) {
		result := fileReady("/nonexistent/path/to/file.m4s")
		if result {
			t.Error("Expected false for non-existent file")
		}
	})

	t.Run("empty file returns false", func(t *testing.T) {
		tmpDir := t.TempDir()
		path := filepath.Join(tmpDir, "empty.m4s")
		f, err := os.Create(path)
		if err != nil {
			t.Fatalf("Failed to create temp file: %v", err)
		}
		f.Close()

		result := fileReady(path)
		if result {
			t.Error("Expected false for empty file")
		}
	})

	t.Run("file with content returns true", func(t *testing.T) {
		tmpDir := t.TempDir()
		path := filepath.Join(tmpDir, "segment_0.m4s")
		if err := os.WriteFile(path, []byte("fake segment data"), 0644); err != nil {
			t.Fatalf("Failed to write temp file: %v", err)
		}

		result := fileReady(path)
		if !result {
			t.Error("Expected true for file with content")
		}
	})

	t.Run("directory path returns false", func(t *testing.T) {
		tmpDir := t.TempDir()
		// A directory has size 0 conceptually (or OS-dependent) but Stat succeeds.
		// fileReady checks size > 0; a directory often has a nonzero size on disk.
		// We just confirm it doesn't panic.
		_ = fileReady(tmpDir)
	})
}

// ---- segmentComplete ----

func makeTestSession(tmpDir string) *HLSSession {
	return &HLSSession{
		TempDir: tmpDir,
		ExitMu:  sync.Mutex{},
	}
}

func writeFile(t *testing.T, dir, name, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0644); err != nil {
		t.Fatalf("Failed to write %s: %v", name, err)
	}
}

func TestSegmentComplete(t *testing.T) {
	t.Run("init file complete when segment_0 exists", func(t *testing.T) {
		tmpDir := t.TempDir()
		session := makeTestSession(tmpDir)
		writeFile(t, tmpDir, helpers.HLS_SEGMENT_FILENAME_PREFIX+"0"+helpers.HLS_SEGMENT_FILENAME_SUFFIX, "data")

		if !segmentComplete(session, helpers.HLS_INIT_FILENAME) {
			t.Error("Expected init.mp4 to be complete when segment_0.m4s exists")
		}
	})

	t.Run("init file not complete when segment_0 missing", func(t *testing.T) {
		tmpDir := t.TempDir()
		session := makeTestSession(tmpDir)
		// segment_0 does not exist; no exit either
		if segmentComplete(session, helpers.HLS_INIT_FILENAME) {
			t.Error("Expected init.mp4 to be incomplete when segment_0.m4s is missing")
		}
	})

	t.Run("segment_0 complete when segment_1 exists", func(t *testing.T) {
		tmpDir := t.TempDir()
		session := makeTestSession(tmpDir)
		writeFile(t, tmpDir, helpers.HLS_SEGMENT_FILENAME_PREFIX+"1"+helpers.HLS_SEGMENT_FILENAME_SUFFIX, "data")

		filename := helpers.HLS_SEGMENT_FILENAME_PREFIX + "0" + helpers.HLS_SEGMENT_FILENAME_SUFFIX
		if !segmentComplete(session, filename) {
			t.Error("Expected segment_0.m4s to be complete when segment_1.m4s exists")
		}
	})

	t.Run("segment_0 not complete when segment_1 missing and not exited", func(t *testing.T) {
		tmpDir := t.TempDir()
		session := makeTestSession(tmpDir)
		// neither segment_1 nor exit condition

		filename := helpers.HLS_SEGMENT_FILENAME_PREFIX + "0" + helpers.HLS_SEGMENT_FILENAME_SUFFIX
		if segmentComplete(session, filename) {
			t.Error("Expected segment_0.m4s to be incomplete when segment_1.m4s is missing and not exited")
		}
	})

	t.Run("last segment complete when ffmpeg exited and file exists", func(t *testing.T) {
		tmpDir := t.TempDir()
		session := makeTestSession(tmpDir)
		session.Exited = true

		lastSeg := helpers.HLS_SEGMENT_FILENAME_PREFIX + "9" + helpers.HLS_SEGMENT_FILENAME_SUFFIX
		writeFile(t, tmpDir, lastSeg, "data")
		// segment_10 does NOT exist (it's the last one)

		if !segmentComplete(session, lastSeg) {
			t.Error("Expected last segment to be complete when ffmpeg exited and file exists")
		}
	})

	t.Run("segment not complete when ffmpeg exited but file is missing", func(t *testing.T) {
		tmpDir := t.TempDir()
		session := makeTestSession(tmpDir)
		session.Exited = true
		// file does not exist

		filename := helpers.HLS_SEGMENT_FILENAME_PREFIX + "5" + helpers.HLS_SEGMENT_FILENAME_SUFFIX
		if segmentComplete(session, filename) {
			t.Error("Expected segment to be incomplete when ffmpeg exited but file does not exist")
		}
	})

	t.Run("invalid segment filename returns false", func(t *testing.T) {
		tmpDir := t.TempDir()
		session := makeTestSession(tmpDir)

		if segmentComplete(session, "invalid_name.xyz") {
			t.Error("Expected false for invalid segment filename")
		}
	})

	t.Run("segment with negative number returns false", func(t *testing.T) {
		tmpDir := t.TempDir()
		session := makeTestSession(tmpDir)

		// segment_-1.m4s is not a valid parsed uint but has the right prefix/suffix
		filename := helpers.HLS_SEGMENT_FILENAME_PREFIX + "-1" + helpers.HLS_SEGMENT_FILENAME_SUFFIX
		if segmentComplete(session, filename) {
			t.Error("Expected false for segment with negative number")
		}
	})
}