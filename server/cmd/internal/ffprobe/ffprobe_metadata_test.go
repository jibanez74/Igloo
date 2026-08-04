package ffprobe

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
)

func TestRunMetadataRejectsEmptyPath(t *testing.T) {
	probe := &ffprobe{bin: writeFakeFFprobe(t, fakeFFprobeSpec{})}

	_, err := probe.GetMetadata("")
	if err == nil || !strings.Contains(err.Error(), "file path is required") {
		t.Fatalf("GetMetadata(\"\") error = %v, want missing file path error", err)
	}

	_, err = probe.GetAudioMetadata("   ")
	if err == nil || !strings.Contains(err.Error(), "file path is required") {
		t.Fatalf("GetAudioMetadata(blank) error = %v, want missing file path error", err)
	}
}

func TestRunMetadataValidJSON(t *testing.T) {
	probe := &ffprobe{bin: writeFakeFFprobe(t, fakeFFprobeSpec{
		stdout: `{"streams":[{"index":1,"codec_name":"aac","codec_type":"audio","sample_rate":"44100"}],"format":{"duration":"12.34","tags":{"title":"Song Title"}},"chapters":[]}`,
	})}

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
	probe := &ffprobe{bin: writeFakeFFprobe(t, fakeFFprobeSpec{
		stdout: `{invalid json`,
	})}

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
	probe := &ffprobe{bin: writeFakeFFprobe(t, fakeFFprobeSpec{
		stderr:   "  fake stderr  ",
		exitCode: 2,
	})}

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
	probe := &ffprobe{bin: writeFakeFFprobe(t, fakeFFprobeSpec{
		stdout: `{"streams":[],"format":{},"chapters":[]}`,
	})}

	_, err := probe.GetMetadata("/tmp/empty.mp3")
	if err == nil {
		t.Fatal("Expected empty streams error")
	}

	if !strings.Contains(err.Error(), "no streams found in /tmp/empty.mp3") {
		t.Fatalf("error = %q, want no streams error with file path", err.Error())
	}
}

func TestGetAudioMetadataRequestsScannerFields(t *testing.T) {
	argsLog := filepath.Join(t.TempDir(), "args.log")
	probe := &ffprobe{bin: writeFakeFFprobe(t, fakeFFprobeSpec{
		stdout:  `{"streams":[{"codec_name":"aac","codec_type":"audio","channels":2}],"format":{"duration":"12.34","bit_rate":"256000","tags":{"title":"Song Title"}}}`,
		argsLog: argsLog,
	})}

	_, err := probe.GetAudioMetadata("/tmp/song.mp3")
	if err != nil {
		t.Fatalf("GetAudioMetadata failed: %v", err)
	}

	args := readArgumentLog(t, argsLog)
	requireArgumentValue(t, args, "-print_format", "json")
	requireArgumentValue(t, args, "-show_entries",
		"format=duration,bit_rate:format_tags:stream=codec_name,codec_type,profile,channels,channel_layout:stream_tags=language")
	if args[len(args)-1] != "/tmp/song.mp3" {
		t.Fatalf("last argument = %q, want the probed file path", args[len(args)-1])
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
