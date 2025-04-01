package ffmpeg

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestCreateHlsStream(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a sample video file for testing
	testVideoPath := filepath.Join(tmpDir, "test.mp4")
	err := createSampleVideo(testVideoPath)
	if err != nil {
		t.Fatalf("Failed to create sample video: %v", err)
	}

	// Create FFmpeg instance with settings
	settings := &Settings{
		FfmpegPath:                 "ffmpeg",
		EnableHardwareAcceleration: false,
		HardwareEncoder:            "software",
	}
	f, err := New(settings)
	if err != nil {
		t.Fatalf("Failed to create FFmpeg instance: %v", err)
	}

	// Set up HLS options
	opts := &HlsOpts{
		InputPath:        testVideoPath,
		OutputDir:        tmpDir,
		StartTime:        0,
		AudioStreamIndex: 0,
		AudioCodec:       AudioCodecAAC,
		AudioBitRate:     DefaultAudioRate,
		AudioChannels:    DefaultAudioChannels,
		VideoStreamIndex: 0,
		VideoCodec:       VideoCodecH264,
		VideoBitrate:     DefaultVideoRate,
		VideoHeight:      DefaultVideoHeight,
		Preset:           DefaultPreset,
		SegmentsUrl:      "http://localhost:8080/segments",
		TotalDuration:    60, // 1 minute test video
	}

	// Create HLS stream
	jobID, err := f.CreateHlsStream(opts)
	if err != nil {
		t.Fatalf("Failed to create HLS stream: %v", err)
	}

	// Wait for the job to complete
	err = waitForJobCompletion(f.(*ffmpeg), jobID, 30*time.Second)
	if err != nil {
		t.Fatalf("Job did not complete in time: %v", err)
	}

	// Verify output files
	expectedFiles := []string{
		DefaultPlaylistName,
		DefaultInitFileName,
	}

	for _, file := range expectedFiles {
		path := filepath.Join(tmpDir, file)
		if _, err := os.Stat(path); os.IsNotExist(err) {
			t.Errorf("Expected file %s was not created", file)
		}
	}

	// Verify VOD playlist content
	vodPath := filepath.Join(tmpDir, DefaultPlaylistName)
	vodContent, err := os.ReadFile(vodPath)
	if err != nil {
		t.Fatalf("Failed to read VOD playlist: %v", err)
	}

	// Print playlist content for debugging
	t.Logf("Playlist content:\n%s", string(vodContent))

	// Check for required tags in VOD playlist
	vodRequiredTags := []string{
		"#EXTM3U",
		"#EXT-X-VERSION:7",
		"#EXT-X-TARGETDURATION:1",
		"#EXT-X-MEDIA-SEQUENCE:0",
		"#EXT-X-PLAYLIST-TYPE:VOD",
		"#EXT-X-INDEPENDENT-SEGMENTS",
		"#EXT-X-MAP:URI=\"init.mp4\"",
	}

	for _, tag := range vodRequiredTags {
		if !strings.Contains(string(vodContent), tag) {
			t.Errorf("VOD playlist missing required tag: %s", tag)
		}
	}

	// Verify segment files exist
	segmentPattern := filepath.Join(tmpDir, "segment_*.m4s")
	segments, err := filepath.Glob(segmentPattern)
	if err != nil {
		t.Fatalf("Failed to find segment files: %v", err)
	}

	if len(segments) == 0 {
		t.Error("No segment files were created")
	}
}

func TestCreateHlsStreamErrors(t *testing.T) {
	settings := &Settings{
		FfmpegPath:                 "ffmpeg",
		EnableHardwareAcceleration: false,
		HardwareEncoder:            "software",
	}
	f, err := New(settings)
	if err != nil {
		t.Fatalf("Failed to create FFmpeg instance: %v", err)
	}

	tests := []struct {
		name    string
		opts    *HlsOpts
		wantErr bool
	}{
		{
			name:    "nil options",
			opts:    nil,
			wantErr: true,
		},
		{
			name: "empty input path",
			opts: &HlsOpts{
				InputPath: "",
				OutputDir: t.TempDir(),
			},
			wantErr: true,
		},
		{
			name: "non-existent input file",
			opts: &HlsOpts{
				InputPath: "nonexistent.mp4",
				OutputDir: t.TempDir(),
			},
			wantErr: true,
		},
		{
			name: "empty output directory",
			opts: &HlsOpts{
				InputPath: "test.mp4",
				OutputDir: "",
			},
			wantErr: true,
		},
		{
			name: "empty segments URL",
			opts: &HlsOpts{
				InputPath:   "test.mp4",
				OutputDir:   t.TempDir(),
				SegmentsUrl: "",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := f.CreateHlsStream(tt.opts)
			if (err != nil) != tt.wantErr {
				t.Errorf("CreateHlsStream() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestCreateHlsStreamWithRealVideo(t *testing.T) {
	// Skip if running in CI or short mode
	if testing.Short() {
		t.Skip("Skipping test with real video in short mode")
	}

	// Create output directory
	tmpDir := t.TempDir()

	// Create FFmpeg instance with settings
	settings := &Settings{
		FfmpegPath:                 "ffmpeg",
		EnableHardwareAcceleration: false,
		HardwareEncoder:            "software",
	}
	f, err := New(settings)
	if err != nil {
		t.Fatalf("Failed to create FFmpeg instance: %v", err)
	}

	// Set up HLS options
	opts := &HlsOpts{
		InputPath:        "../test.mkv",
		OutputDir:        tmpDir,
		StartTime:        0,
		AudioStreamIndex: 0,
		AudioCodec:       AudioCodecAAC,
		AudioBitRate:     DefaultAudioRate,
		AudioChannels:    DefaultAudioChannels,
		VideoStreamIndex: 0,
		VideoCodec:       VideoCodecH264,
		VideoBitrate:     DefaultVideoRate,
		VideoHeight:      DefaultVideoHeight,
		Preset:           VideoPresetUltrafast, // Use ultrafast preset for quicker testing
		SegmentsUrl:      "http://localhost:8080/segments",
		TotalDuration:    60, // Process first minute only for testing
	}

	// Create HLS stream
	jobID, err := f.CreateHlsStream(opts)
	if err != nil {
		t.Fatalf("Failed to create HLS stream: %v", err)
	}

	// Wait for the job to complete with a longer timeout due to file size
	err = waitForJobCompletion(f.(*ffmpeg), jobID, 5*time.Minute)
	if err != nil {
		t.Fatalf("Job did not complete in time: %v", err)
	}

	// Verify output files
	expectedFiles := []string{
		DefaultPlaylistName,
		DefaultInitFileName,
	}

	for _, file := range expectedFiles {
		path := filepath.Join(tmpDir, file)
		if _, err := os.Stat(path); os.IsNotExist(err) {
			t.Errorf("Expected file %s was not created", file)
		}
	}

	// Verify VOD playlist content
	vodPath := filepath.Join(tmpDir, DefaultPlaylistName)
	vodContent, err := os.ReadFile(vodPath)
	if err != nil {
		t.Fatalf("Failed to read VOD playlist: %v", err)
	}

	// Print playlist content for debugging
	t.Logf("Playlist content:\n%s", string(vodContent))

	// Check for required tags in VOD playlist
	vodRequiredTags := []string{
		"#EXTM3U",
		"#EXT-X-VERSION:7",
		"#EXT-X-TARGETDURATION:6",
		"#EXT-X-MEDIA-SEQUENCE:0",
		"#EXT-X-PLAYLIST-TYPE:VOD",
		"#EXT-X-INDEPENDENT-SEGMENTS",
		"#EXT-X-MAP:URI=\"init.mp4\"",
	}

	for _, tag := range vodRequiredTags {
		if !strings.Contains(string(vodContent), tag) {
			t.Errorf("VOD playlist missing required tag: %s", tag)
		}
	}

	// Verify segment files exist
	segmentPattern := filepath.Join(tmpDir, "segment_*.m4s")
	segments, err := filepath.Glob(segmentPattern)
	if err != nil {
		t.Fatalf("Failed to find segment files: %v", err)
	}

	if len(segments) == 0 {
		t.Error("No segment files were created")
	}

	// Log number of segments created
	t.Logf("Number of segments created: %d", len(segments))

	// Verify first segment exists and has content
	if len(segments) > 0 {
		firstSegment := segments[0]
		info, err := os.Stat(firstSegment)
		if err != nil {
			t.Errorf("Failed to stat first segment: %v", err)
		} else {
			t.Logf("First segment size: %d bytes", info.Size())
		}
	}
}

// Helper function to create a sample video file for testing
func createSampleVideo(path string) error {
	// Create a 1-second video with a solid color using FFmpeg
	cmd := exec.Command("ffmpeg", "-f", "lavfi", "-i", "color=c=red:s=1280x720:d=1", "-c:v", "libx264", path)
	return cmd.Run()
}

// Helper function to wait for job completion
func waitForJobCompletion(f *ffmpeg, jobID string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			f.mu.Lock()
			job, exists := f.jobs[jobID]
			f.mu.Unlock()

			if !exists {
				return nil // Job completed and was cleaned up
			}

			if job.process.ProcessState != nil && job.process.ProcessState.Exited() {
				return nil // Job completed
			}

			if time.Now().After(deadline) {
				return fmt.Errorf("timeout waiting for job completion")
			}
		}
	}
}
