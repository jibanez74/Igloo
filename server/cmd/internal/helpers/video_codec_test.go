package helpers

import "testing"

// Import treats only still-image codecs as embedded cover art; playback also
// skips MJPEG, which docs/ffmpeg.md keeps as an accepted moving-video codec.
func TestCoverArtVideoCodecs(t *testing.T) {
	t.Parallel()
	tests := []struct {
		codec        string
		wantImport   bool
		wantPlayback bool
	}{
		{codec: "mjpeg", wantImport: false, wantPlayback: true},
		{codec: "MJPEG", wantImport: false, wantPlayback: true},
		{codec: " mjpeg ", wantImport: false, wantPlayback: true},
		{codec: "png", wantImport: true, wantPlayback: true},
		{codec: "gif", wantImport: true, wantPlayback: true},
		{codec: "bmp", wantImport: true, wantPlayback: true},
		{codec: "h264", wantImport: false, wantPlayback: false},
		{codec: "hevc", wantImport: false, wantPlayback: false},
		{codec: "", wantImport: false, wantPlayback: false},
	}
	for _, tt := range tests {
		t.Run(tt.codec, func(t *testing.T) {
			t.Parallel()
			gotImport := IsCoverArtVideoCodec(tt.codec)
			if gotImport != tt.wantImport {
				t.Errorf("IsCoverArtVideoCodec(%q) = %v, want %v", tt.codec, gotImport, tt.wantImport)
			}
			gotPlayback := IsPlaybackCoverArtVideoCodec(tt.codec)
			if gotPlayback != tt.wantPlayback {
				t.Errorf("IsPlaybackCoverArtVideoCodec(%q) = %v, want %v", tt.codec, gotPlayback, tt.wantPlayback)
			}
		})
	}
}
