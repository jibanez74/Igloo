package helpers

import (
	"maps"
	"testing"
)

// Expected values are literals on purpose: a bad edit to either MIME table
// must fail here, so do not derive them from the maps. The key sets must also
// match the extension tables exactly, since the scanners accept by extension
// and the stream handlers answer with the MIME type.
func TestMimeTypesPinValuesAndCoverValidExtensions(t *testing.T) {
	wantVideo := map[string]string{
		"mp4":  "video/mp4",
		"m4v":  "video/mp4",
		"mkv":  "video/x-matroska",
		"webm": "video/webm",
		"avi":  "video/x-msvideo",
		"mov":  "video/quicktime",
	}
	if !maps.Equal(VideoMimeTypes, wantVideo) {
		t.Errorf("VideoMimeTypes = %v, want %v", VideoMimeTypes, wantVideo)
	}

	wantAudio := map[string]string{
		"mp3":  "audio/mpeg",
		"flac": "audio/flac",
		"m4a":  "audio/mp4",
	}
	if !maps.Equal(AudioMimeTypes, wantAudio) {
		t.Errorf("AudioMimeTypes = %v, want %v", AudioMimeTypes, wantAudio)
	}

	for ext := range ValidVideoExtensions {
		_, ok := VideoMimeTypes[ext]
		if !ok {
			t.Errorf("VideoMimeTypes is missing entry for valid extension %q", ext)
		}
	}
	for ext := range VideoMimeTypes {
		if !ValidVideoExtensions[ext] {
			t.Errorf("VideoMimeTypes has entry %q that is not a valid video extension", ext)
		}
	}

	for ext := range ValidAudioExtensions {
		_, ok := AudioMimeTypes[ext]
		if !ok {
			t.Errorf("AudioMimeTypes is missing entry for valid extension %q", ext)
		}
	}
	for ext := range AudioMimeTypes {
		if !ValidAudioExtensions[ext] {
			t.Errorf("AudioMimeTypes has entry %q that is not a valid audio extension", ext)
		}
	}
}
