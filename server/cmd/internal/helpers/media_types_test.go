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

// Expected values are literals on purpose: a bad edit to VideoMimeTypes must
// fail here, so do not assert against the map.
func TestVideoMimeTypesPinPerContainerValues(t *testing.T) {
	cases := map[string]string{
		"mp4":  "video/mp4",
		"m4v":  "video/mp4",
		"mkv":  "video/x-matroska",
		"webm": "video/webm",
		"avi":  "video/x-msvideo",
		"mov":  "video/quicktime",
	}
	for ext, want := range cases {
		if VideoMimeTypes[ext] != want {
			t.Errorf("VideoMimeTypes[%q] = %q, want %q", ext, VideoMimeTypes[ext], want)
		}
	}
}

func TestAudioMimeTypesCoverValidAudioExtensions(t *testing.T) {
	for ext := range ValidAudioExtensions {
		mimeType, ok := AudioMimeTypes[ext]
		if !ok {
			t.Errorf("AudioMimeTypes is missing entry for valid extension %q", ext)
			continue
		}
		if mimeType == "" {
			t.Errorf("AudioMimeTypes[%q] is empty", ext)
		}
	}

	for ext := range AudioMimeTypes {
		if !ValidAudioExtensions[ext] {
			t.Errorf("AudioMimeTypes has entry %q that is not a valid audio extension", ext)
		}
	}
}
