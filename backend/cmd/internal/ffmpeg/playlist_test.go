package ffmpeg

import (
	"os"
	"path/filepath"
	"testing"
)

func TestConvertToVod(t *testing.T) {
	// Create a temporary directory for test files
	tmpDir := t.TempDir()

	// Create a test EVENT playlist with keyframe segments
	eventPlaylist := `#EXTM3U
#EXT-X-VERSION:7
#EXT-X-TARGETDURATION:6
#EXT-X-MEDIA-SEQUENCE:0
#EXT-X-PLAYLIST-TYPE:EVENT
#EXT-X-MAP:URI="init.mp4"
#EXTINF:6.0,
segment_0.mp4#keyframe
#EXTINF:5.8,
segment_1.mp4
#EXTINF:6.2,
segment_2.mp4#keyframe`

	playlistPath := filepath.Join(tmpDir, "playlist.m3u8")
	err := os.WriteFile(playlistPath, []byte(eventPlaylist), 0644)
	if err != nil {
		t.Fatalf("Failed to create test playlist: %v", err)
	}

	// Create ffmpeg instance
	f := &ffmpeg{}

	// Test converting to VOD
	err = f.convertToVod(playlistPath)
	if err != nil {
		t.Errorf("convertToVod failed: %v", err)
	}

	// Read the converted playlist
	content, err := os.ReadFile(playlistPath)
	if err != nil {
		t.Fatalf("Failed to read converted playlist: %v", err)
	}

	// Verify VOD playlist content
	expected := `#EXTM3U
#EXT-X-VERSION:7
#EXT-X-PLAYLIST-TYPE:VOD
#EXT-X-TARGETDURATION:6
#EXT-X-MEDIA-SEQUENCE:0
#EXT-X-MAP:URI="init.mp4"
#EXTINF:6.000,
segment_0.mp4#keyframe
#EXTINF:5.800,
segment_1.mp4
#EXTINF:6.200,
segment_2.mp4#keyframe
#EXT-X-ENDLIST
`

	if string(content) != expected {
		t.Errorf("Converted playlist content doesn't match expected:\nGot:\n%s\nExpected:\n%s", string(content), expected)
	}

	// Test with non-existent file
	err = f.convertToVod("nonexistent.m3u8")
	if err == nil {
		t.Error("Expected error for non-existent file, got nil")
	}
}

func TestParsePlaylist(t *testing.T) {
	f := &ffmpeg{}

	tests := []struct {
		name    string
		content string
		wantErr bool
	}{
		{
			name: "valid playlist with keyframes",
			content: `#EXTM3U
#EXT-X-VERSION:7
#EXT-X-TARGETDURATION:6
#EXT-X-MEDIA-SEQUENCE:0
#EXT-X-MAP:URI="init.mp4"
#EXTINF:6.0,
segment_0.mp4#keyframe
#EXTINF:5.8,
segment_1.mp4`,
			wantErr: false,
		},
		{
			name: "invalid version",
			content: `#EXTM3U
#EXT-X-VERSION:invalid
#EXT-X-TARGETDURATION:6`,
			wantErr: true,
		},
		{
			name: "invalid target duration",
			content: `#EXTM3U
#EXT-X-VERSION:7
#EXT-X-TARGETDURATION:invalid`,
			wantErr: true,
		},
		{
			name: "missing segment URI",
			content: `#EXTM3U
#EXT-X-VERSION:7
#EXTINF:6.0,`,
			wantErr: true,
		},
		{
			name: "segment duration too long",
			content: `#EXTM3U
#EXT-X-VERSION:7
#EXT-X-TARGETDURATION:6
#EXTINF:11.0,
segment_0.mp4`,
			wantErr: true,
		},
		{
			name: "segment duration too short",
			content: `#EXTM3U
#EXT-X-VERSION:7
#EXT-X-TARGETDURATION:6
#EXTINF:1.0,
segment_0.mp4`,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			info, err := f.parsePlaylist(tt.content)
			if tt.wantErr {
				if err == nil {
					t.Error("Expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Errorf("parsePlaylist failed: %v", err)
				return
			}
			if info.Version != 7 {
				t.Errorf("Expected version 7, got %d", info.Version)
			}
			if info.TargetDuration != 6 {
				t.Errorf("Expected target duration 6, got %f", info.TargetDuration)
			}
			if info.InitSegment != "init.mp4" {
				t.Errorf("Expected init segment 'init.mp4', got '%s'", info.InitSegment)
			}
			if len(info.Segments) != 2 {
				t.Errorf("Expected 2 segments, got %d", len(info.Segments))
			}
			if info.Segments[0].IsKeyframe != true {
				t.Error("Expected first segment to be a keyframe")
			}
			if info.Segments[1].IsKeyframe != false {
				t.Error("Expected second segment to not be a keyframe")
			}
			if info.Segments[0].StartTime != 0 {
				t.Errorf("Expected first segment start time to be 0, got %f", info.Segments[0].StartTime)
			}
			if info.Segments[1].StartTime != 6.0 {
				t.Errorf("Expected second segment start time to be 6.0, got %f", info.Segments[1].StartTime)
			}
		})
	}
}

func TestGenerateVodPlaylist(t *testing.T) {
	f := &ffmpeg{}

	info := &playlistInfo{
		Version:        7,
		TargetDuration: 6,
		MediaSequence:  0,
		InitSegment:    "init.mp4",
		Segments: []segmentInfo{
			{
				Duration:   6.0,
				URI:        "segment_0.mp4",
				IsKeyframe: true,
				StartTime:  0.0,
				EndTime:    6.0,
			},
			{
				Duration:   5.8,
				URI:        "segment_1.mp4",
				IsKeyframe: false,
				StartTime:  6.0,
				EndTime:    11.8,
			},
		},
	}

	got := f.generateVodPlaylist(info)
	expected := `#EXTM3U
#EXT-X-VERSION:7
#EXT-X-PLAYLIST-TYPE:VOD
#EXT-X-TARGETDURATION:6
#EXT-X-MEDIA-SEQUENCE:0
#EXT-X-MAP:URI="init.mp4"
#EXTINF:6.000,
segment_0.mp4#keyframe
#EXTINF:5.800,
segment_1.mp4
#EXT-X-ENDLIST
`

	if got != expected {
		t.Errorf("generateVodPlaylist output doesn't match expected:\nGot:\n%s\nExpected:\n%s", got, expected)
	}
}

func TestExtractUriFromMap(t *testing.T) {
	f := &ffmpeg{}

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "valid URI",
			input:    `#EXT-X-MAP:URI="init.mp4"`,
			expected: "init.mp4",
		},
		{
			name:     "URI with additional attributes",
			input:    `#EXT-X-MAP:URI="init.mp4",BYTERANGE="1234@5678"`,
			expected: "init.mp4",
		},
		{
			name:     "no URI",
			input:    `#EXT-X-MAP:BYTERANGE="1234@5678"`,
			expected: "",
		},
		{
			name:     "empty input",
			input:    "",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := f.extractUriFromMap(tt.input)
			if got != tt.expected {
				t.Errorf("extractUriFromMap(%q) = %q, want %q", tt.input, got, tt.expected)
			}
		})
	}
}

func TestGetSegmentCount(t *testing.T) {
	tmpDir := t.TempDir()
	f := &ffmpeg{}

	// Create a test playlist with keyframe segments
	playlist := `#EXTM3U
#EXT-X-VERSION:7
#EXT-X-TARGETDURATION:6
#EXT-X-MEDIA-SEQUENCE:0
#EXT-X-MAP:URI="init.mp4"
#EXTINF:6.0,
segment_0.mp4#keyframe
#EXTINF:5.8,
segment_1.mp4
#EXTINF:6.2,
segment_2.mp4#keyframe`

	playlistPath := filepath.Join(tmpDir, "playlist.m3u8")
	err := os.WriteFile(playlistPath, []byte(playlist), 0644)
	if err != nil {
		t.Fatalf("Failed to create test playlist: %v", err)
	}

	// Test with valid playlist
	count, err := f.getSegmentCount(playlistPath)
	if err != nil {
		t.Errorf("getSegmentCount failed: %v", err)
	}
	if count != 3 {
		t.Errorf("Expected 3 segments, got %d", count)
	}

	// Test with non-existent file
	_, err = f.getSegmentCount("nonexistent.m3u8")
	if err == nil {
		t.Error("Expected error for non-existent file, got nil")
	}
}
