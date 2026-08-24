package main

import (
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
			if got := movieContentType(tt.container, tt.storedMime); got != tt.want {
				t.Errorf("movieContentType(%q, %q) = %q, want %q", tt.container, tt.storedMime, got, tt.want)
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
