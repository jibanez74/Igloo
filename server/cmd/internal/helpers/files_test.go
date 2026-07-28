package helpers

import "testing"

func TestVideoMimeTypesCoverValidVideoExtensions(t *testing.T) {
	for ext := range ValidVideoExtensions {
		mimeType, ok := VideoMimeTypes[ext]
		if !ok {
			t.Errorf("VideoMimeTypes is missing entry for valid extension %q", ext)
			continue
		}
		if mimeType == "" {
			t.Errorf("VideoMimeTypes[%q] is empty", ext)
		}
	}

	for ext := range VideoMimeTypes {
		if !ValidVideoExtensions[ext] {
			t.Errorf("VideoMimeTypes has entry %q that is not a valid video extension", ext)
		}
	}
}

func TestIsMovieReleaseNoiseToken(t *testing.T) {
	tests := []struct {
		token string
		want  bool
	}{
		{"1080p", true},
		{"WEB-DL", true},
		{"x265", true},
		{"remastered", true},
		{"extended", true},
		{"mkv", true},
		{"Moneyball", false},
		{"2011", false},
	}

	for _, tt := range tests {
		got := IsMovieReleaseNoiseToken(tt.token)
		if got != tt.want {
			t.Errorf("IsMovieReleaseNoiseToken(%q) = %v, want %v", tt.token, got, tt.want)
		}
	}
}
