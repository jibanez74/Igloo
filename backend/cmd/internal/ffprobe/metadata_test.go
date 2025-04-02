package ffprobe

import (
	"igloo/cmd/internal/database"
	"os"
	"path/filepath"
	"testing"
)

func TestGetMovieMetadata(t *testing.T) {
	// Get the absolute path to the test video file
	testVideoPath := filepath.Join("..", "..", "..", "test.mp4")
	absPath, err := filepath.Abs(testVideoPath)
	if err != nil {
		t.Fatalf("Failed to get absolute path: %v", err)
	}

	// Check if test file exists
	if _, err := os.Stat(absPath); os.IsNotExist(err) {
		t.Skipf("Test video file not found at %s, skipping tests", absPath)
	}

	// Create ffprobe instance
	settings := &database.GlobalSetting{
		FfprobePath: "ffprobe",
		FfmpegPath:  "ffmpeg",
	}
	f, err := New(settings)
	if err != nil {
		t.Fatalf("Failed to create ffprobe instance: %v", err)
	}

	// Test GetMovieMetadata
	t.Run("GetMovieMetadata", func(t *testing.T) {
		result, err := f.GetMovieMetadata(&absPath)
		if err != nil {
			t.Fatalf("GetMovieMetadata failed: %v", err)
		}

		// Verify video streams
		if len(result.VideoList) == 0 {
			t.Error("Expected at least one video stream")
		}
		videoStream := result.VideoList[0]
		if videoStream.Codec == "" {
			t.Error("Video stream codec should not be empty")
		}
		if videoStream.Width <= 0 {
			t.Error("Video width should be positive")
		}
		if videoStream.Height <= 0 {
			t.Error("Video height should be positive")
		}
		if videoStream.Duration == "" {
			t.Error("Video duration should not be empty")
		}

		// Verify audio streams
		if len(result.AudioList) == 0 {
			t.Error("Expected at least one audio stream")
		}
		audioStream := result.AudioList[0]
		if audioStream.Codec == "" {
			t.Error("Audio stream codec should not be empty")
		}
		if audioStream.Channels <= 0 {
			t.Error("Audio channels should be positive")
		}

		// Verify subtitle streams (if any)
		for _, subtitle := range result.SubtitleList {
			if subtitle.Codec == "" {
				t.Error("Subtitle codec should not be empty")
			}
		}
	})

	// Test ProcessVideoStream
	t.Run("ProcessVideoStream", func(t *testing.T) {
		testStream := mediaStream{
			Index:          0,
			CodecName:      "h264",
			CodecLongName:  "H.264 / AVC / MPEG-4 AVC / MPEG-4 part 10",
			Profile:        "high",
			Width:          1920,
			Height:         1080,
			CodedWidth:     1920,
			CodedHeight:    1080,
			AspectRatio:    "16:9",
			Level:          41,
			ColorTransfer:  "bt709",
			ColorPrimaries: "bt709",
			ColorSpace:     "bt709",
			ColorRange:     "tv",
			AvgFrameRate:   "24000/1001",
			FrameRate:      "24000/1001",
			BitDepth:       "8",
			BitRate:        "5000000",
		}

		result := f.processVideoStream(testStream)
		if result.Index != 0 {
			t.Errorf("Expected index 0, got %d", result.Index)
		}
		if result.Codec != "h264" {
			t.Errorf("Expected codec h264, got %s", result.Codec)
		}
		if result.Width != 1920 {
			t.Errorf("Expected width 1920, got %d", result.Width)
		}
		if result.Height != 1080 {
			t.Errorf("Expected height 1080, got %d", result.Height)
		}
	})

	// Test ProcessAudioStream
	t.Run("ProcessAudioStream", func(t *testing.T) {
		testStream := mediaStream{
			Index:     1,
			CodecName: "aac",
			Tags: tags{
				Title:    "English",
				Language: "eng",
			},
			Channels:      6,
			ChannelLayout: "5.1",
		}

		result := f.processAudioStream(testStream)
		if result.Index != 1 {
			t.Errorf("Expected index 1, got %d", result.Index)
		}
		if result.Codec != "aac" {
			t.Errorf("Expected codec aac, got %s", result.Codec)
		}
		if result.Language != "eng" {
			t.Errorf("Expected language eng, got %s", result.Language)
		}
		if result.Channels != 6 {
			t.Errorf("Expected 6 channels, got %d", result.Channels)
		}
	})

	// Test ProcessSubtitleStream
	t.Run("ProcessSubtitleStream", func(t *testing.T) {
		testStream := mediaStream{
			Index:         2,
			CodecName:     "ass",
			CodecLongName: "ASS (Advanced SubStation Alpha) subtitle",
			Tags: tags{
				Title:    "English",
				Language: "eng",
			},
		}

		result := f.processSubtitleStream(testStream)
		if result.Index != 2 {
			t.Errorf("Expected index 2, got %d", result.Index)
		}
		if result.Codec != "ass" {
			t.Errorf("Expected codec ass, got %s", result.Codec)
		}
		if result.Language != "eng" {
			t.Errorf("Expected language eng, got %s", result.Language)
		}
	})

	// Test ProcessChapters
	t.Run("ProcessChapters", func(t *testing.T) {
		testChapters := []tmdbChapter{
			{
				Start:     0,
				StartTime: "0.0",
				End:       300,
				EndTime:   "300.0",
				Title:     "Chapter 1",
			},
			{
				Start:     300,
				StartTime: "300.0",
				End:       600,
				EndTime:   "600.0",
				Title:     "Chapter 2",
			},
		}

		result := f.processChapters(testChapters)
		if len(result) != 2 {
			t.Errorf("Expected 2 chapters, got %d", len(result))
		}
		if result[0].Title != "Chapter 1" {
			t.Errorf("Expected chapter title 'Chapter 1', got %s", result[0].Title)
		}
		if result[0].StartTimeMs != 0 {
			t.Errorf("Expected start time 0ms, got %d", result[0].StartTimeMs)
		}
		if result[1].Title != "Chapter 2" {
			t.Errorf("Expected chapter title 'Chapter 2', got %s", result[1].Title)
		}
		if result[1].StartTimeMs != 300000 {
			t.Errorf("Expected start time 300000ms, got %d", result[1].StartTimeMs)
		}
	})
}
