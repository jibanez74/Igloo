package helpers

import "testing"

func TestIsCoverArtVideoCodec(t *testing.T) {
	t.Parallel()
	tests := []struct {
		codec string
		want  bool
	}{
		{"mjpeg", true},
		{"MJPEG", true},
		{"png", true},
		{"gif", true},
		{"bmp", true},
		{"h264", false},
		{"hevc", false},
		{"", false},
	}
	for _, tt := range tests {
		t.Run(tt.codec, func(t *testing.T) {
			t.Parallel()
			if got := IsCoverArtVideoCodec(tt.codec); got != tt.want {
				t.Errorf("IsCoverArtVideoCodec(%q) = %v, want %v", tt.codec, got, tt.want)
			}
		})
	}
}
