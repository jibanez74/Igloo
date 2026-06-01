package ffprobe

import (
	"encoding/json"
	"testing"
)

func TestGetMetadata_NonExistentFile(t *testing.T) {
	probe, err := New()
	if err != nil {
		t.Fatalf("Failed to create ffprobe instance: %v", err)
	}
	defer Cleanup()

	_, err = probe.GetMetadata("/nonexistent/path/file.mp3")
	if err == nil {
		t.Error("Expected error for non-existent file")
	}
}

func TestGetMetadata_EmptyPath(t *testing.T) {
	probe, err := New()
	if err != nil {
		t.Fatalf("Failed to create ffprobe instance: %v", err)
	}
	defer Cleanup()

	_, err = probe.GetMetadata("")
	if err == nil {
		t.Error("Expected error for empty file path")
	}
}

func TestGetAudioMetadata_EmptyPath(t *testing.T) {
	probe, err := New()
	if err != nil {
		t.Fatalf("Failed to create ffprobe instance: %v", err)
	}
	defer Cleanup()

	_, err = probe.GetAudioMetadata("")
	if err == nil {
		t.Error("Expected error for empty file path")
	}
}

func TestFormatTagsUnmarshalAliases(t *testing.T) {
	var tags FormatTags
	err := json.Unmarshal([]byte(`{
		"title": "Song Title",
		"albumartist": "Album Artist",
		"tracknumber": "2/10",
		"discnumber": "1/2",
		"titlesort": "Song Title Sort",
		"albumsort": "Album Sort",
		"artistsort": "Artist Sort"
	}`), &tags)
	if err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}

	if tags.Title != "Song Title" {
		t.Fatalf("Title = %q, want %q", tags.Title, "Song Title")
	}
	if tags.AlbumArtist != "Album Artist" {
		t.Fatalf("AlbumArtist = %q, want %q", tags.AlbumArtist, "Album Artist")
	}
	if tags.Track != "2/10" {
		t.Fatalf("Track = %q, want %q", tags.Track, "2/10")
	}
	if tags.Disc != "1/2" {
		t.Fatalf("Disc = %q, want %q", tags.Disc, "1/2")
	}
	if tags.SortName != "Song Title Sort" {
		t.Fatalf("SortName = %q, want %q", tags.SortName, "Song Title Sort")
	}
	if tags.SortAlbum != "Album Sort" {
		t.Fatalf("SortAlbum = %q, want %q", tags.SortAlbum, "Album Sort")
	}
	if tags.SortArtist != "Artist Sort" {
		t.Fatalf("SortArtist = %q, want %q", tags.SortArtist, "Artist Sort")
	}
}
