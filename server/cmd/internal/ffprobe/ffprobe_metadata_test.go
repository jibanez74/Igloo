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

func TestGetAudioMetadataRequestsScannerFields(t *testing.T) {
	argsPath := filepath.Join(t.TempDir(), "args.log")
	t.Setenv("FFPROBE_ARGS_LOG", argsPath)

	binPath := filepath.Join(t.TempDir(), "ffprobe")
	script := `#!/bin/sh
printf '%s\n' "$@" > "$FFPROBE_ARGS_LOG"
printf '%s\n' '{"streams":[{"codec_name":"aac","codec_type":"audio","channels":2}],"format":{"duration":"12.34","bit_rate":"256000","tags":{"title":"Song Title"}}}'
`
	err := os.WriteFile(binPath, []byte(script), 0755)
	if err != nil {
		t.Fatalf("write fake ffprobe: %v", err)
	}

	probe := &ffprobe{bin: binPath}
	_, err = probe.GetAudioMetadata("/tmp/song.mp3")
	if err != nil {
		t.Fatalf("GetAudioMetadata failed: %v", err)
	}

	argsData, err := os.ReadFile(argsPath)
	if err != nil {
		t.Fatalf("read ffprobe arguments: %v", err)
	}
	got := strings.Split(strings.TrimSpace(string(argsData)), "\n")
	want := []string{
		"-v",
		"quiet",
		"-print_format",
		"json",
		"-show_format",
		"-show_streams",
		"-show_entries",
		"format=duration,bit_rate:format_tags:stream=codec_name,codec_type,profile,channels,channel_layout:stream_tags=language",
		"/tmp/song.mp3",
	}
	if len(got) != len(want) {
		t.Fatalf("argument count = %d, want %d: %q", len(got), len(want), got)
	}
	for index, wantArg := range want {
		if got[index] != wantArg {
			t.Fatalf("argument %d = %q, want %q", index, got[index], wantArg)
		}
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

func TestStreamDispositionUnmarshalKeepsAllFlags(t *testing.T) {
	var stream Stream
	err := json.Unmarshal([]byte(`{
		"index": 2,
		"codec_name": "aac",
		"codec_type": "audio",
		"disposition": {
			"default": 1,
			"dub": 1,
			"original": 1,
			"comment": 1,
			"forced": 1,
			"hearing_impaired": 1,
			"visual_impaired": 1,
			"attached_pic": 1
		}
	}`), &stream)
	if err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}

	d := stream.Disposition
	if d.Default != 1 {
		t.Error("Default flag was dropped")
	}
	if d.Forced != 1 {
		t.Error("Forced flag was dropped")
	}
	if d.Comment != 1 {
		t.Error("Comment flag was dropped")
	}
	if d.Dub != 1 {
		t.Error("Dub flag was dropped")
	}
	if d.Original != 1 {
		t.Error("Original flag was dropped")
	}
	if d.HearingImpaired != 1 {
		t.Error("HearingImpaired flag was dropped")
	}
	if d.VisualImpaired != 1 {
		t.Error("VisualImpaired flag was dropped")
	}
	if d.AttachedPic != 1 {
		t.Error("AttachedPic flag was dropped")
	}
}

func TestStreamTagsUnmarshalNormalizesKeys(t *testing.T) {
	cases := []struct {
		name         string
		payload      string
		wantTitle    string
		wantLanguage string
	}{
		{
			name:         "uppercase matroska keys",
			payload:      `{"TITLE": "Director Commentary", "LANGUAGE": "eng"}`,
			wantTitle:    "Director Commentary",
			wantLanguage: "eng",
		},
		{
			name:         "lang alias and padded values",
			payload:      `{"lang": "  fra  ", "title": "  Main  "}`,
			wantTitle:    "Main",
			wantLanguage: "fra",
		},
		{
			name:         "empty values ignored",
			payload:      `{"title": "", "language": ""}`,
			wantTitle:    "",
			wantLanguage: "",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var tags StreamTags
			err := json.Unmarshal([]byte(tc.payload), &tags)
			if err != nil {
				t.Fatalf("Unmarshal failed: %v", err)
			}
			if tags.Title != tc.wantTitle {
				t.Errorf("Title = %q, want %q", tags.Title, tc.wantTitle)
			}
			if tags.Language != tc.wantLanguage {
				t.Errorf("Language = %q, want %q", tags.Language, tc.wantLanguage)
			}
		})
	}
}

func fakeFFprobe(t *testing.T) string {
	t.Helper()

	binPath := filepath.Join(t.TempDir(), "ffprobe")
	script := `#!/bin/sh
case "$FFPROBE_FAKE_MODE" in
valid)
	printf '%s\n' '{"streams":[{"index":1,"codec_name":"aac","codec_type":"audio","sample_rate":"44100"}],"format":{"duration":"12.34","tags":{"title":"Song Title"}},"chapters":[]}'
	;;
invalid)
	printf '%s\n' '{invalid json'
	;;
fail)
	printf '%s\n' '  fake stderr  ' >&2
	exit 2
	;;
empty)
	printf '%s\n' '{"streams":[],"format":{},"chapters":[]}'
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
