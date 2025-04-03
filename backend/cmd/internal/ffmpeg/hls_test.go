package ffmpeg

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestCreateHlsStream(t *testing.T) {
	// Create a temporary directory for test output
	tempDir, err := os.MkdirTemp("", "hls_test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Use the existing test video file
	testVideoPath := "test.mp4"
	if _, err := os.Stat(testVideoPath); os.IsNotExist(err) {
		t.Fatalf("test video file not found: %v", err)
	}

	// Create FFmpeg instance
	ffmpeg := NewFFmpeg()

	// Test cases
	tests := []struct {
		name    string
		opts    *HlsOpts
		wantErr bool
	}{
		{
			name: "successful stream creation",
			opts: &HlsOpts{
				InputPath:        testVideoPath,
				OutputDir:        filepath.Join(tempDir, "output"),
				AudioCodec:       AudioCodecAAC,
				AudioBitRate:     DefaultAudioRate,
				AudioChannels:    DefaultAudioChannels,
				VideoCodec:       VideoCodecH264,
				VideoBitrate:     2000,
				VideoHeight:      720,
				SegmentsUrl:      "http://test.com/segments",
				Preset:           DefaultPreset,
				AudioStreamIndex: 1, // Use the audio stream
				VideoStreamIndex: 0, // Use the first video stream
			},
			wantErr: false,
		},
		{
			name: "invalid input path",
			opts: &HlsOpts{
				InputPath:        "nonexistent.mp4",
				OutputDir:        filepath.Join(tempDir, "output"),
				AudioCodec:       AudioCodecAAC,
				AudioBitRate:     DefaultAudioRate,
				AudioChannels:    DefaultAudioChannels,
				VideoCodec:       VideoCodecH264,
				VideoBitrate:     2000,
				VideoHeight:      720,
				SegmentsUrl:      "http://test.com/segments",
				Preset:           DefaultPreset,
				AudioStreamIndex: 1,
				VideoStreamIndex: 0,
			},
			wantErr: true,
		},
		{
			name: "invalid output directory",
			opts: &HlsOpts{
				InputPath:        testVideoPath,
				OutputDir:        "/nonexistent/output",
				AudioCodec:       AudioCodecAAC,
				AudioBitRate:     DefaultAudioRate,
				AudioChannels:    DefaultAudioChannels,
				VideoCodec:       VideoCodecH264,
				VideoBitrate:     2000,
				VideoHeight:      720,
				SegmentsUrl:      "http://test.com/segments",
				Preset:           DefaultPreset,
				AudioStreamIndex: 1,
				VideoStreamIndex: 0,
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create output directory if it doesn't exist
			if !tt.wantErr {
				err := os.MkdirAll(tt.opts.OutputDir, 0755)
				if err != nil {
					t.Fatalf("failed to create output directory: %v", err)
				}
			}

			// Create HLS stream
			jobID, err := ffmpeg.CreateHlsStream(tt.opts)
			if (err != nil) != tt.wantErr {
				t.Errorf("CreateHlsStream() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr {
				// Verify job was created
				if jobID == "" {
					t.Error("CreateHlsStream() returned empty job ID")
				}

				// Wait longer for segments to be generated
				time.Sleep(5 * time.Second)

				// Check if playlist file exists
				playlistPath := filepath.Join(tt.opts.OutputDir, DefaultPlaylistName)
				if _, err := os.Stat(playlistPath); os.IsNotExist(err) {
					t.Errorf("playlist file not created at %s", playlistPath)
				}

				// Wait for job to be cleaned up
				maxRetries := 5
				for i := 0; i < maxRetries; i++ {
					err = ffmpeg.CancelJob(jobID)
					if err == nil {
						break
					}
					time.Sleep(time.Second)
				}
				if err != nil {
					t.Logf("warning: failed to cancel job after %d retries: %v", maxRetries, err)
				}
			}
		})
	}
}
