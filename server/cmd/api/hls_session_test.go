package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestHLSSessionKey(t *testing.T) {
	key := HLSSessionKey(123, "1080p_4mbps", 0)
	if key != "123:1080p_4mbps:0" {
		t.Errorf("HLSSessionKey(123, 1080p_4mbps, 0) = %q, want 123:1080p_4mbps:0", key)
	}
	key2 := HLSSessionKey(1, "720p_3mbps", 1)
	if key2 != "1:720p_3mbps:1" {
		t.Errorf("HLSSessionKey(1, 720p_3mbps, 1) = %q, want 1:720p_3mbps:1", key2)
	}
}

func TestHLSSessionKey_AllProfiles(t *testing.T) {
	tests := []struct {
		movieID    int64
		profile    string
		audioTrack int
		want       string
	}{
		{1, "remux", 0, "1:remux:0"},
		{5, "2160p_16mbps", 2, "5:2160p_16mbps:2"},
		{100, "1080p_8mbps", 0, "100:1080p_8mbps:0"},
		{100, "1080p_6mbps", 1, "100:1080p_6mbps:1"},
		{100, "1080p_4mbps", 3, "100:1080p_4mbps:3"},
		{42, "720p_3mbps", 0, "42:720p_3mbps:0"},
	}
	for _, tt := range tests {
		got := HLSSessionKey(tt.movieID, tt.profile, tt.audioTrack)
		if got != tt.want {
			t.Errorf("HLSSessionKey(%d, %q, %d) = %q, want %q",
				tt.movieID, tt.profile, tt.audioTrack, got, tt.want)
		}
	}
}

func TestHLSSessionKey_Uniqueness(t *testing.T) {
	// Different movie IDs must produce different keys
	k1 := HLSSessionKey(1, "1080p_4mbps", 0)
	k2 := HLSSessionKey(2, "1080p_4mbps", 0)
	if k1 == k2 {
		t.Error("Different movie IDs should produce different keys")
	}

	// Different profiles must produce different keys
	k3 := HLSSessionKey(1, "1080p_4mbps", 0)
	k4 := HLSSessionKey(1, "720p_3mbps", 0)
	if k3 == k4 {
		t.Error("Different profiles should produce different keys")
	}

	// Different audio tracks must produce different keys
	k5 := HLSSessionKey(1, "1080p_4mbps", 0)
	k6 := HLSSessionKey(1, "1080p_4mbps", 1)
	if k5 == k6 {
		t.Error("Different audio tracks should produce different keys")
	}
}

// ---- cleanupHLSSession ----

func TestCleanupHLSSession_NilSession(t *testing.T) {
	// Should not panic on nil session
	cleanupHLSSession(nil)
}

func TestCleanupHLSSession_RemovesTempDir(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a file inside the temp dir to confirm it's non-empty
	f := filepath.Join(tmpDir, "segment_0.m4s")
	if err := os.WriteFile(f, []byte("data"), 0644); err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

	session := &HLSSession{TempDir: tmpDir}
	cleanupHLSSession(session)

	if _, err := os.Stat(tmpDir); !os.IsNotExist(err) {
		t.Errorf("Expected temp dir %s to be removed after cleanup, but it still exists", tmpDir)
	}
}

func TestCleanupHLSSession_EmptyTempDir(t *testing.T) {
	// A session with an empty TempDir string should not attempt RemoveAll("")
	// which would be catastrophic. It should silently skip.
	session := &HLSSession{TempDir: ""}
	// Should not panic or error
	cleanupHLSSession(session)
}

func TestCleanupHLSSession_NoCmdDoesNotPanic(t *testing.T) {
	tmpDir := t.TempDir()
	session := &HLSSession{
		TempDir: tmpDir,
		Cmd:     nil, // no command — should not panic
	}
	cleanupHLSSession(session)
}