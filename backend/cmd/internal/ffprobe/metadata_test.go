package ffprobe

import (
	"igloo/cmd/internal/database"
	"testing"
)

// mockFfprobe is a mock implementation of the Ffprobe interface for testing
type mockFfprobe struct {
	ffprobe
}

func (f *mockFfprobe) GetMovieMetadata(filePath *string) (*movieMetadataResult, error) {
	// Mock result with test data
	mockResult := &result{
		Streams: []mediaStream{
			{
				Index:          0,
				CodecName:      "h264",
				CodecLongName:  "H.264 / AVC / MPEG-4 AVC / MPEG-4 part 10",
				CodecType:      "video",
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
			},
			{
				Index:         1,
				CodecName:     "aac",
				CodecLongName: "AAC (Advanced Audio Coding)",
				CodecType:     "audio",
				Profile:       "LC",
				Channels:      6,
				ChannelLayout: "5.1",
				BitRate:       "384000",
			},
			{
				Index:         2,
				CodecName:     "subrip",
				CodecLongName: "SubRip subtitle",
				CodecType:     "subtitle",
				Tags: tags{
					Language: "eng",
					Title:    "English",
				},
			},
		},
		Chapters: []tmdbChapter{
			{
				Start:     0,
				StartTime: "0.000000",
				End:       300,
				EndTime:   "5.000000",
				Title:     "Chapter 1",
			},
		},
		Format: format{
			Duration: "6784.875000",
		},
	}

	videoStreams, audioStreams, subtitleStreams := f.processStreams(mockResult.Streams)
	result := &movieMetadataResult{
		VideoList:    videoStreams,
		AudioList:    audioStreams,
		SubtitleList: subtitleStreams,
		ChapterList:  f.processChapters(mockResult.Chapters),
	}

	// Set duration from format
	if mockResult.Format.Duration != "" {
		result.VideoList[0].Duration = mockResult.Format.Duration
	} else {
		result.VideoList[0].Duration = "0"
	}

	return result, nil
}

func TestGetMovieMetadata(t *testing.T) {
	// Create mock ffprobe instance
	settings := &database.GlobalSetting{
		FfprobePath: "ffprobe",
	}
	f := &mockFfprobe{}
	f.bin = settings.FfprobePath

	// Test GetMovieMetadata
	t.Run("GetMovieMetadata", func(t *testing.T) {
		testPath := "test.mkv"
		result, err := f.GetMovieMetadata(&testPath)
		if err != nil {
			t.Fatalf("GetMovieMetadata failed: %v", err)
		}

		// Verify video streams
		if len(result.VideoList) == 0 {
			t.Error("Expected at least one video stream")
		}
		videoStream := result.VideoList[0]
		if videoStream.Codec != "h264" {
			t.Errorf("Expected codec h264, got %s", videoStream.Codec)
		}
		if videoStream.Width != 1920 {
			t.Errorf("Expected width 1920, got %d", videoStream.Width)
		}
		if videoStream.Height != 1080 {
			t.Errorf("Expected height 1080, got %d", videoStream.Height)
		}
		if videoStream.Duration != "6784.875000" {
			t.Errorf("Expected duration 6784.875000, got %s", videoStream.Duration)
		}

		// Verify audio streams
		if len(result.AudioList) == 0 {
			t.Error("Expected at least one audio stream")
		}
		audioStream := result.AudioList[0]
		if audioStream.Codec != "aac" {
			t.Errorf("Expected codec aac, got %s", audioStream.Codec)
		}
		if audioStream.Channels != 6 {
			t.Errorf("Expected channels 6, got %d", audioStream.Channels)
		}

		// Verify subtitle streams
		if len(result.SubtitleList) == 0 {
			t.Error("Expected at least one subtitle stream")
		}
		subtitleStream := result.SubtitleList[0]
		if subtitleStream.Codec != "subrip" {
			t.Errorf("Expected codec subrip, got %s", subtitleStream.Codec)
		}
		if subtitleStream.Language != "eng" {
			t.Errorf("Expected language eng, got %s", subtitleStream.Language)
		}

		// Verify chapters
		if len(result.ChapterList) == 0 {
			t.Error("Expected at least one chapter")
		}
		chapter := result.ChapterList[0]
		if chapter.Title != "Chapter 1" {
			t.Errorf("Expected title Chapter 1, got %s", chapter.Title)
		}
	})

	// Test processVideoStream
	t.Run("processVideoStream", func(t *testing.T) {
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

		videoStream := f.processVideoStream(testStream)
		if videoStream.Codec != testStream.CodecName {
			t.Errorf("Expected codec %s, got %s", testStream.CodecName, videoStream.Codec)
		}
		if videoStream.Width != int32(testStream.Width) {
			t.Errorf("Expected width %d, got %d", testStream.Width, videoStream.Width)
		}
		if videoStream.Height != int32(testStream.Height) {
			t.Errorf("Expected height %d, got %d", testStream.Height, videoStream.Height)
		}
	})

	// Test processAudioStream
	t.Run("processAudioStream", func(t *testing.T) {
		testStream := mediaStream{
			Index:         1,
			CodecName:     "aac",
			CodecLongName: "AAC (Advanced Audio Coding)",
			Profile:       "LC",
			Channels:      6,
			ChannelLayout: "5.1",
			BitRate:       "384000",
		}

		audioStream := f.processAudioStream(testStream)
		if audioStream.Codec != testStream.CodecName {
			t.Errorf("Expected codec %s, got %s", testStream.CodecName, audioStream.Codec)
		}
		if audioStream.Channels != int32(testStream.Channels) {
			t.Errorf("Expected channels %d, got %d", testStream.Channels, audioStream.Channels)
		}
	})

	// Test processSubtitleStream
	t.Run("processSubtitleStream", func(t *testing.T) {
		testStream := mediaStream{
			Index:         2,
			CodecName:     "subrip",
			CodecLongName: "SubRip subtitle",
			Tags: tags{
				Language: "eng",
				Title:    "English",
			},
		}

		subtitleStream := f.processSubtitleStream(testStream)
		if subtitleStream.Codec != testStream.CodecName {
			t.Errorf("Expected codec %s, got %s", testStream.CodecName, subtitleStream.Codec)
		}
		if subtitleStream.Language != testStream.Tags.Language {
			t.Errorf("Expected language %s, got %s", testStream.Tags.Language, subtitleStream.Language)
		}
	})

	// Test processChapters
	t.Run("processChapters", func(t *testing.T) {
		testChapters := []tmdbChapter{
			{
				Start:     0,
				StartTime: "0.000000",
				End:       300,
				EndTime:   "5.000000",
				Title:     "Chapter 1",
			},
		}

		chapters := f.processChapters(testChapters)
		if len(chapters) != len(testChapters) {
			t.Errorf("Expected %d chapters, got %d", len(testChapters), len(chapters))
		}
		if chapters[0].Title != testChapters[0].Title {
			t.Errorf("Expected title %s, got %s", testChapters[0].Title, chapters[0].Title)
		}
	})
}
