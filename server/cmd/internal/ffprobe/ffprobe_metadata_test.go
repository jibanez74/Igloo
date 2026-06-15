package ffprobe

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
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

func TestRunMetadataValidJSON(t *testing.T) {
	t.Setenv("FFPROBE_FAKE_MODE", "valid")
	probe := &ffprobe{bin: fakeFFprobe(t)}

	result, err := probe.GetMetadata("/tmp/song.mp3")
	if err != nil {
		t.Fatalf("GetMetadata failed: %v", err)
	}

	if len(result.Streams) != 1 {
		t.Fatalf("len(Streams) = %d, want 1", len(result.Streams))
	}
	if result.Streams[0].CodecName != "aac" {
		t.Fatalf("CodecName = %q, want %q", result.Streams[0].CodecName, "aac")
	}
	if result.Streams[0].CodecType != "audio" {
		t.Fatalf("CodecType = %q, want %q", result.Streams[0].CodecType, "audio")
	}
	if result.Format.Filename != "song.mp3" {
		t.Fatalf("Format.Filename = %q, want %q", result.Format.Filename, "song.mp3")
	}
	if result.Format.Tags.Title != "Song Title" {
		t.Fatalf("Format.Tags.Title = %q, want %q", result.Format.Tags.Title, "Song Title")
	}
}

func TestRunMetadataInvalidJSONIncludesFilePath(t *testing.T) {
	t.Setenv("FFPROBE_FAKE_MODE", "invalid")
	probe := &ffprobe{bin: fakeFFprobe(t)}

	_, err := probe.GetMetadata("/tmp/bad.mp3")
	if err == nil {
		t.Fatal("Expected parse error")
	}

	errText := err.Error()
	if !strings.Contains(errText, "failed to parse ffprobe output for /tmp/bad.mp3") {
		t.Fatalf("error = %q, want parse error with file path", errText)
	}
}

func TestRunMetadataNonzeroExitSurfacesTrimmedStderr(t *testing.T) {
	t.Setenv("FFPROBE_FAKE_MODE", "fail")
	probe := &ffprobe{bin: fakeFFprobe(t)}

	_, err := probe.GetMetadata("/tmp/fail.mp3")
	if err == nil {
		t.Fatal("Expected ffprobe failure")
	}

	errText := err.Error()
	if !strings.Contains(errText, "ffprobe failed for /tmp/fail.mp3") {
		t.Fatalf("error = %q, want ffprobe failure with file path", errText)
	}
	if !strings.Contains(errText, "fake stderr") {
		t.Fatalf("error = %q, want stderr", errText)
	}
	if strings.Contains(errText, "  fake stderr  ") {
		t.Fatalf("error = %q, want trimmed stderr", errText)
	}
}

func TestRunMetadataEmptyStreamsRejected(t *testing.T) {
	t.Setenv("FFPROBE_FAKE_MODE", "empty")
	probe := &ffprobe{bin: fakeFFprobe(t)}

	_, err := probe.GetMetadata("/tmp/empty.mp3")
	if err == nil {
		t.Fatal("Expected empty streams error")
	}

	if !strings.Contains(err.Error(), "no streams found in /tmp/empty.mp3") {
		t.Fatalf("error = %q, want no streams error with file path", err.Error())
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

func fakeFFprobe(t *testing.T) string {
	t.Helper()

	binPath := filepath.Join(t.TempDir(), "ffprobe")
	script := `#!/bin/sh
case "$FFPROBE_FAKE_MODE" in
valid)
	printf '%s\n' '{"streams":[{"index":1,"codec_name":"aac","codec_type":"audio","sample_rate":"44100"}],"format":{"filename":"song.mp3","duration":"12.34","tags":{"title":"Song Title"}},"chapters":[]}'
	;;
invalid)
	printf '%s\n' '{invalid json'
	;;
fail)
	printf '%s\n' '  fake stderr  ' >&2
	exit 2
	;;
empty)
	printf '%s\n' '{"streams":[],"format":{"filename":"empty.mp3"},"chapters":[]}'
	;;
*)
	printf '%s\n' 'unknown fake mode' >&2
	exit 9
	;;
esac
`

	err := os.WriteFile(binPath, []byte(script), 0755)
	if err != nil {
		t.Fatalf("Write fake ffprobe: %v", err)
	}

	return binPath
}
