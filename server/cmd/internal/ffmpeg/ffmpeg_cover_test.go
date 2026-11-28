package ffmpeg

import (
	"slices"
	"testing"
)

func TestAttachedPictureArgsCopyOneStreamIntoOneImage(t *testing.T) {
	args := attachedPictureArgs("/music/a.flac", 2, "/static/local/albums/7/.tmp-x")
	want := []string{
		"-hide_banner", "-nostdin", "-v", "error", "-y",
		"-i", "/music/a.flac",
		"-map", "0:2",
		"-c", "copy", "-frames:v", "1",
		"-f", "image2", "-update", "1",
		"/static/local/albums/7/.tmp-x",
	}
	if !slices.Equal(args, want) {
		t.Fatalf("args = %q, want %q", args, want)
	}
}
