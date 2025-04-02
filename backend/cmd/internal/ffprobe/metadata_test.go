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
	// Mock result with test data based on test.mp4
	mockResult := &result{
		Streams: []mediaStream{
			{
				Index:          1,
				CodecName:      "h264",
				CodecLongName:  "H.264 / AVC / MPEG-4 AVC / MPEG-4 part 10",
				CodecType:      "video",
				Profile:        "High",
				Width:          1920,
				Height:         800,
				CodedWidth:     1920,
				CodedHeight:    800,
				AspectRatio:    "12:5",
				Level:          40,
				ColorTransfer:  "bt709",
				ColorPrimaries: "bt709",
				ColorSpace:     "bt709",
				ColorRange:     "tv",
				AvgFrameRate:   "83029800/3463037",
				FrameRate:      "24000/1001",
				BitDepth:       "8",
				BitRate:        "4498650",
				Disposition: &disposition{
					Default:         1,
					AttachedPic:     0,
					Forced:          0,
					HearingImpaired: 0,
					VisualImpaired:  0,
				},
			},
			{
				Index:         0,
				CodecName:     "aac",
				CodecLongName: "AAC (Advanced Audio Coding)",
				CodecType:     "audio",
				Profile:       "LC",
				Channels:      2,
				ChannelLayout: "stereo",
				BitRate:       "125643",
				Tags: tags{
					Language: "eng",
				},
				Disposition: &disposition{
					Default:         1,
					AttachedPic:     0,
					Forced:          0,
					HearingImpaired: 0,
					VisualImpaired:  0,
				},
			},
			{
				Index:         5,
				CodecName:     "mov_text",
				CodecLongName: "MOV text",
				CodecType:     "subtitle",
				Tags: tags{
					Language: "eng",
				},
				Disposition: &disposition{
					Default:         1,
					AttachedPic:     0,
					Forced:          0,
					HearingImpaired: 0,
					VisualImpaired:  0,
				},
			},
		},
		Chapters: []tmdbChapter{
			{
				Start:     0,
				StartTime: "0.000000",
				End:       459000,
				EndTime:   "459.000000",
				Title:     "A Christmas Carol",
			},
			{
				Start:     459000,
				StartTime: "459.000000",
				End:       965000,
				EndTime:   "965.000000",
				Title:     "Bah, Humbug!",
			},
		},
		Format: format{
			Duration: "5771.771666",
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
		testPath := "test.mp4"
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
		if videoStream.Height != 800 {
			t.Errorf("Expected height 800, got %d", videoStream.Height)
		}
		if videoStream.Duration != "5771.771666" {
			t.Errorf("Expected duration 5771.771666, got %s", videoStream.Duration)
		}

		// Verify audio streams
		if len(result.AudioList) == 0 {
			t.Error("Expected at least one audio stream")
		}
		audioStream := result.AudioList[0]
		if audioStream.Codec != "aac" {
			t.Errorf("Expected codec aac, got %s", audioStream.Codec)
		}
		if audioStream.Channels != 2 {
			t.Errorf("Expected channels 2, got %d", audioStream.Channels)
		}
		if audioStream.Language != "eng" {
			t.Errorf("Expected language eng, got %s", audioStream.Language)
		}

		// Verify subtitle streams
		if len(result.SubtitleList) == 0 {
			t.Error("Expected at least one subtitle stream")
		}
		subtitleStream := result.SubtitleList[0]
		if subtitleStream.Codec != "mov_text" {
			t.Errorf("Expected codec mov_text, got %s", subtitleStream.Codec)
		}
		if subtitleStream.Language != "eng" {
			t.Errorf("Expected language eng, got %s", subtitleStream.Language)
		}

		// Verify chapters
		if len(result.ChapterList) == 0 {
			t.Error("Expected at least one chapter")
		}
		chapter := result.ChapterList[0]
		if chapter.Title != "A Christmas Carol" {
			t.Errorf("Expected title 'A Christmas Carol', got %s", chapter.Title)
		}
	})

	// Test processVideoStream
	t.Run("processVideoStream", func(t *testing.T) {
		testStream := mediaStream{
			Index:          1,
			CodecName:      "h264",
			CodecLongName:  "H.264 / AVC / MPEG-4 AVC / MPEG-4 part 10",
			Profile:        "High",
			Width:          1920,
			Height:         800,
			CodedWidth:     1920,
			CodedHeight:    800,
			AspectRatio:    "12:5",
			Level:          40,
			ColorTransfer:  "bt709",
			ColorPrimaries: "bt709",
			ColorSpace:     "bt709",
			ColorRange:     "tv",
			AvgFrameRate:   "83029800/3463037",
			FrameRate:      "24000/1001",
			BitDepth:       "8",
			BitRate:        "4498650",
			Disposition: &disposition{
				Default:         1,
				AttachedPic:     0,
				Forced:          0,
				HearingImpaired: 0,
				VisualImpaired:  0,
			},
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
			Index:         0,
			CodecName:     "aac",
			CodecLongName: "AAC (Advanced Audio Coding)",
			Profile:       "LC",
			Channels:      2,
			ChannelLayout: "stereo",
			BitRate:       "125643",
			Tags: tags{
				Language: "eng",
			},
			Disposition: &disposition{
				Default:         1,
				AttachedPic:     0,
				Forced:          0,
				HearingImpaired: 0,
				VisualImpaired:  0,
			},
		}

		audioStream := f.processAudioStream(testStream)
		if audioStream.Codec != testStream.CodecName {
			t.Errorf("Expected codec %s, got %s", testStream.CodecName, audioStream.Codec)
		}
		if audioStream.Channels != int32(testStream.Channels) {
			t.Errorf("Expected channels %d, got %d", testStream.Channels, audioStream.Channels)
		}
		if audioStream.Language != testStream.Tags.Language {
			t.Errorf("Expected language %s, got %s", testStream.Tags.Language, audioStream.Language)
		}
	})

	// Test processSubtitleStream
	t.Run("processSubtitleStream", func(t *testing.T) {
		testStream := mediaStream{
			Index:         5,
			CodecName:     "mov_text",
			CodecLongName: "MOV text",
			Tags: tags{
				Language: "eng",
			},
			Disposition: &disposition{
				Default:         1,
				AttachedPic:     0,
				Forced:          0,
				HearingImpaired: 0,
				VisualImpaired:  0,
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
				End:       459000,
				EndTime:   "459.000000",
				Title:     "A Christmas Carol",
			},
			{
				Start:     459000,
				StartTime: "459.000000",
				End:       965000,
				EndTime:   "965.000000",
				Title:     "Bah, Humbug!",
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
