package main

import (
	"database/sql"
	"testing"

	"igloo/cmd/internal/database"
)

func TestMovieContentType(t *testing.T) {
	tests := []struct {
		name       string
		container  string
		storedMime string
		want       string
	}{
		{"pinned map wins", "mp4", "application/octet-stream", "video/mp4"},
		{"unknown container falls back to the stored value", "ogv", "video/ogg", "video/ogg"},
		{"unknown container with no stored value", "ogv", "", ""},
		{"matroska is not video/mp4", "mkv", "video/mp4", "video/x-matroska"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := videoContentType(tt.container, tt.storedMime); got != tt.want {
				t.Errorf("videoContentType(%q, %q) = %q, want %q", tt.container, tt.storedMime, got, tt.want)
			}
		})
	}
}

func TestPrimaryVideoStream(t *testing.T) {
	coverArt := database.VideoStream{StreamIndex: 0, Codec: "MJPEG"}
	feature := database.VideoStream{StreamIndex: 1, Codec: "h264"}

	if got := primaryVideoStream(nil); got != nil {
		t.Errorf("expected nil for no streams, got %+v", got)
	}

	got := primaryVideoStream([]database.VideoStream{coverArt, feature})
	if got == nil || got.Codec != "h264" {
		t.Errorf("expected the h264 feature stream, got %+v", got)
	}

	// Cover art only: fall back to the first row rather than reporting no
	// video at all, so the caller's own rules decide.
	got = primaryVideoStream([]database.VideoStream{coverArt})
	if got == nil || got.Codec != "MJPEG" {
		t.Errorf("expected the only stream as fallback, got %+v", got)
	}
}

// ffprobe frequently omits a per-stream bit_rate for Matroska, and those are
// exactly the sources that fail the remux gate, so the container average has
// to stand in or bitrate-aware fallback selection would never fire for them.
func TestSourceVideoBitRate(t *testing.T) {
	tests := []struct {
		name  string
		size  int64
		dur   sql.NullFloat64
		video *database.VideoStream
		want  int64
	}{
		{
			name:  "a probed stream bitrate wins",
			size:  1_000_000_000,
			dur:   sql.NullFloat64{Float64: 3600, Valid: true},
			video: &database.VideoStream{BitRate: 5_000_000},
			want:  5_000_000,
		},
		{
			name:  "an unprobed stream falls back to the container average",
			size:  900_000_000,
			dur:   sql.NullFloat64{Float64: 3600, Valid: true},
			video: &database.VideoStream{BitRate: 0},
			want:  2_000_000,
		},
		{
			name:  "an unmeasurable duration reports unknown",
			size:  900_000_000,
			dur:   sql.NullFloat64{Valid: false},
			video: &database.VideoStream{BitRate: 0},
			want:  0,
		},
		{
			name:  "a zero-length duration reports unknown",
			size:  900_000_000,
			dur:   sql.NullFloat64{Float64: 0, Valid: true},
			video: &database.VideoStream{BitRate: 0},
			want:  0,
		},
		{
			name:  "an empty file reports unknown",
			size:  0,
			dur:   sql.NullFloat64{Float64: 3600, Valid: true},
			video: &database.VideoStream{BitRate: 0},
			want:  0,
		},
		{
			name:  "no video stream reports unknown",
			size:  0,
			dur:   sql.NullFloat64{Valid: false},
			video: nil,
			want:  0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			source := playbackSource{Size: tt.size, Duration: tt.dur}

			got := sourceVideoBitRate(source, tt.video)
			if got != tt.want {
				t.Fatalf("sourceVideoBitRate() = %d, want %d", got, tt.want)
			}
		})
	}
}
