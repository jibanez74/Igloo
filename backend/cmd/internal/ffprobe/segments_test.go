package ffprobe

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestComputeSegments(t *testing.T) {
	f := &ffprobe{
		bin: "ffprobe",
	}

	tests := []struct {
		name             string
		keyframeData     *KeyframeData
		segmentLength    time.Duration
		expectedSegments []time.Duration
		expectedError    bool
	}{
		{
			name: "empty keyframe data",
			keyframeData: &KeyframeData{
				TotalDuration: 0,
				Keyframes:     []time.Duration{},
			},
			segmentLength:    6 * time.Second,
			expectedSegments: nil,
			expectedError:    false,
		},
		{
			name: "single keyframe",
			keyframeData: &KeyframeData{
				TotalDuration: 10 * time.Second,
				Keyframes:     []time.Duration{5 * time.Second},
			},
			segmentLength:    6 * time.Second,
			expectedSegments: []time.Duration{6 * time.Second, 4 * time.Second},
			expectedError:    false,
		},
		{
			name: "multiple keyframes",
			keyframeData: &KeyframeData{
				TotalDuration: 20 * time.Second,
				Keyframes: []time.Duration{
					5 * time.Second,
					10 * time.Second,
					15 * time.Second,
				},
			},
			segmentLength:    6 * time.Second,
			expectedSegments: []time.Duration{6 * time.Second, 6 * time.Second, 6 * time.Second, 2 * time.Second},
			expectedError:    false,
		},
		{
			name: "keyframes not aligned with segment length",
			keyframeData: &KeyframeData{
				TotalDuration: 15 * time.Second,
				Keyframes: []time.Duration{
					3 * time.Second,
					7 * time.Second,
					12 * time.Second,
				},
			},
			segmentLength:    6 * time.Second,
			expectedSegments: []time.Duration{6 * time.Second, 6 * time.Second, 3 * time.Second},
			expectedError:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			segments := f.ComputeSegments(tt.keyframeData, tt.segmentLength)

			if tt.expectedError {
				if segments != nil {
					t.Errorf("expected error but got segments: %v", segments)
				}
				return
			}

			if len(segments) != len(tt.expectedSegments) {
				t.Errorf("expected %d segments, got %d", len(tt.expectedSegments), len(segments))
				return
			}

			for i, segment := range segments {
				if segment != tt.expectedSegments[i] {
					t.Errorf("segment %d: expected %v, got %v", i, tt.expectedSegments[i], segment)
				}
			}
		})
	}
}

func TestExtractKeyframes(t *testing.T) {
	// Get the path to the test video file
	testVideoPath := filepath.Join("..", "..", "..", "test.mkv")

	// Verify the test file exists
	if _, err := os.Stat(testVideoPath); os.IsNotExist(err) {
		t.Fatalf("test video file not found at %s", testVideoPath)
	}

	// Create a mock ffprobe struct for testing
	f := &ffprobe{
		bin: "ffprobe",
	}

	tests := []struct {
		name         string
		filePath     string
		expectError  bool
		validateData func(*testing.T, *KeyframeData)
	}{
		{
			name:        "non-existent file",
			filePath:    "non_existent_file.mkv",
			expectError: true,
		},
		{
			name:        "valid video file",
			filePath:    testVideoPath,
			expectError: false,
			validateData: func(t *testing.T, data *KeyframeData) {
				if data == nil {
					t.Error("expected non-nil KeyframeData")
					return
				}
				if data.TotalDuration <= 0 {
					t.Errorf("expected positive duration, got %v", data.TotalDuration)
				}
				if len(data.Keyframes) == 0 {
					t.Error("expected at least one keyframe")
				}
				// Verify keyframes are in ascending order
				for i := 1; i < len(data.Keyframes); i++ {
					if data.Keyframes[i] <= data.Keyframes[i-1] {
						t.Errorf("keyframes not in ascending order: %v <= %v", data.Keyframes[i], data.Keyframes[i-1])
					}
				}
				// Verify last keyframe is not beyond total duration
				if len(data.Keyframes) > 0 {
					lastKeyframe := data.Keyframes[len(data.Keyframes)-1]
					if lastKeyframe > data.TotalDuration {
						t.Errorf("last keyframe (%v) beyond total duration (%v)", lastKeyframe, data.TotalDuration)
					}
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data, err := f.ExtractKeyframes(tt.filePath)

			if tt.expectError {
				if err == nil {
					t.Error("expected error but got nil")
				}
				return
			}

			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}

			if tt.validateData != nil {
				tt.validateData(t, data)
			}
		})
	}
}

func TestComputeEqualLengthSegments(t *testing.T) {
	tests := []struct {
		name             string
		totalDuration    time.Duration
		segmentLength    time.Duration
		expectedSegments []time.Duration
	}{
		{
			name:             "exact segments",
			totalDuration:    12 * time.Second,
			segmentLength:    6 * time.Second,
			expectedSegments: []time.Duration{6 * time.Second, 6 * time.Second},
		},
		{
			name:             "with remainder",
			totalDuration:    15 * time.Second,
			segmentLength:    6 * time.Second,
			expectedSegments: []time.Duration{6 * time.Second, 6 * time.Second, 3 * time.Second},
		},
		{
			name:             "single segment",
			totalDuration:    5 * time.Second,
			segmentLength:    6 * time.Second,
			expectedSegments: []time.Duration{5 * time.Second},
		},
		{
			name:             "zero duration",
			totalDuration:    0,
			segmentLength:    6 * time.Second,
			expectedSegments: nil,
		},
		{
			name:             "zero segment length",
			totalDuration:    12 * time.Second,
			segmentLength:    0,
			expectedSegments: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			segments := computeEqualLengthSegments(tt.totalDuration, tt.segmentLength)

			if len(segments) != len(tt.expectedSegments) {
				t.Errorf("expected %d segments, got %d", len(tt.expectedSegments), len(segments))
				return
			}

			for i, segment := range segments {
				if segment != tt.expectedSegments[i] {
					t.Errorf("segment %d: expected %v, got %v", i, tt.expectedSegments[i], segment)
				}
			}
		})
	}
}
