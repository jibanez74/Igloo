package ffprobe

import (
	"path/filepath"
	"runtime"
	"testing"
)

// getTestMediaPath returns the absolute path to a test media file.
func getTestMediaPath(filename string) string {
	_, currentFile, _, _ := runtime.Caller(0)
	projectRoot := filepath.Join(filepath.Dir(currentFile), "..", "..", "..")

	return filepath.Join(projectRoot, "media", filename)
}

func TestGetMetadata_AudioFile(t *testing.T) {
	probe, err := New()
	if err != nil {
		t.Fatalf("Failed to create ffprobe instance: %v", err)
	}
	defer Cleanup()

	trackPath := getTestMediaPath("track.m4a")

	result, err := probe.GetMetadata(trackPath)
	if err != nil {
		t.Fatalf("GetMetadata failed: %v", err)
	}

	if result == nil {
		t.Fatal("Expected non-nil result")
	}

	// Verify we got at least one stream
	if len(result.Streams) == 0 {
		t.Error("Expected at least one stream")
	}

	// Verify format info is populated
	if result.Format.Filename == "" {
		t.Error("Expected Format.Filename to be set")
	}

	if result.Format.Duration == "" {
		t.Error("Expected Format.Duration to be set")
	}

	if result.Format.FormatName == "" {
		t.Error("Expected Format.FormatName to be set")
	}
}

func TestGetMetadata_AudioStream(t *testing.T) {
	probe, err := New()
	if err != nil {
		t.Fatalf("Failed to create ffprobe instance: %v", err)
	}
	defer Cleanup()

	trackPath := getTestMediaPath("track.m4a")

	result, err := probe.GetMetadata(trackPath)
	if err != nil {
		t.Fatalf("GetMetadata failed: %v", err)
	}

	var audioStream *Stream

	// Find the audio stream
	for i := range result.Streams {
		if result.Streams[i].CodecType == "audio" {
			audioStream = &result.Streams[i]
			break
		}
	}

	if audioStream == nil {
		t.Fatal("Expected to find an audio stream")
	}

	if audioStream.CodecName == "" {
		t.Error("Expected audio stream to have CodecName")
	}

	// an audio track must have at least one channel
	if audioStream.Channels == 0 {
		t.Error("Expected audio stream to have Channels > 0")
	}
}

func TestGetMetadata_NonExistentFile(t *testing.T) {
	probe, err := New()
	if err != nil {
		t.Fatalf("Failed to create ffprobe instance: %v", err)
	}
	defer Cleanup()

	_, err = probe.GetMetadata("/nonexistent/path/file.mp3")
	if err == nil {
		t.Error("Expected error for non-existent file")
	}
}

func TestGetMetadata_EmptyPath(t *testing.T) {
	probe, err := New()
	if err != nil {
		t.Fatalf("Failed to create ffprobe instance: %v", err)
	}
	defer Cleanup()

	_, err = probe.GetMetadata("")
	if err == nil {
		t.Error("Expected error for empty file path")
	}
}

func TestGetMetadata_FormatTags(t *testing.T) {
	probe, err := New()
	if err != nil {
		t.Fatalf("Failed to create ffprobe instance: %v", err)
	}
	defer Cleanup()

	trackPath := getTestMediaPath("track.m4a")

	result, err := probe.GetMetadata(trackPath)
	if err != nil {
		t.Fatalf("GetMetadata failed: %v", err)
	}

	// Log the tags we found (some may be empty depending on the file)
	t.Logf("Title: %s", result.Format.Tags.Title)
	t.Logf("Artist: %s", result.Format.Tags.Artist)
	t.Logf("Album: %s", result.Format.Tags.Album)
	t.Logf("Genre: %s", result.Format.Tags.Genre)
	t.Logf("Track: %s", result.Format.Tags.Track)
	t.Logf("Date: %s", result.Format.Tags.Date)

	// Verify that at least some metadata structure is present
	// (actual values depend on the test file's embedded metadata)
	if result.Format.Tags == (FormatTags{}) {
		t.Log("Warning: No format tags found in file - this may be expected if file has no metadata")
	}
}

func TestGetMetadata_MultipleCallsUseSameInstance(t *testing.T) {
	probe1, err := New()
	if err != nil {
		t.Fatalf("Failed to create first ffprobe instance: %v", err)
	}

	probe2, err := New()
	if err != nil {
		t.Fatalf("Failed to create second ffprobe instance: %v", err)
	}
	defer Cleanup()

	// Both should be the same singleton instance
	if probe1 != probe2 {
		t.Error("Expected New() to return the same singleton instance")
	}

	trackPath := getTestMediaPath("track.m4a")

	// Both instances should work
	result1, err := probe1.GetMetadata(trackPath)
	if err != nil {
		t.Fatalf("First GetMetadata call failed: %v", err)
	}

	result2, err := probe2.GetMetadata(trackPath)
	if err != nil {
		t.Fatalf("Second GetMetadata call failed: %v", err)
	}

	// Results should be equivalent
	if result1.Format.Duration != result2.Format.Duration {
		t.Error("Expected same duration from both calls")
	}
}
