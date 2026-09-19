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
