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

func TestRunMetadataNonzeroExitWithoutStderr(t *testing.T) {
	probe := &ffprobe{bin: writeFakeFFprobe(t, fakeFFprobeSpec{
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
	if !strings.Contains(errText, "exit status 2") {
		t.Fatalf("error = %q, want the exit status", errText)
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

func TestFormatTagsAliasPrecedence(t *testing.T) {
	var tags FormatTags
	err := json.Unmarshal([]byte(`{
		"track": "3",
		"tracknumber": "9",
		"disc": "1",
		"discnumber": "5",
		"date": "2001",
		"year": "1999",
		"sortname": "Canonical Sort",
		"titlesort": "Alias Sort"
	}`), &tags)
	if err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}

	if tags.Track != "3" {
		t.Fatalf("Track = %q, want canonical %q over alias", tags.Track, "3")
	}
	if tags.Disc != "1" {
		t.Fatalf("Disc = %q, want canonical %q over alias", tags.Disc, "1")
	}
	if tags.Date != "2001" {
		t.Fatalf("Date = %q, want canonical %q over alias", tags.Date, "2001")
	}
	if tags.SortName != "Canonical Sort" {
		t.Fatalf("SortName = %q, want canonical %q over alias", tags.SortName, "Canonical Sort")
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

func TestStreamRotationFromSideDataList(t *testing.T) {
	cases := []struct {
		name       string
		payload    string
		wantDeg    int64
		wantMatrix bool
	}{
		{
			// Captured verbatim from the bundled ffprobe 7.1.4-Jellyfin on an
			// mp4 written with -display_rotation 90; the displaymatrix dump is
			// deliberately unparsed noise.
			name:       "display matrix rotation",
			payload:    `{"field_order": "progressive", "side_data_list": [{"side_data_type": "Display Matrix", "displaymatrix": "\n00000000:            0      -65536           0\n", "rotation": 90}]}`,
			wantDeg:    90,
			wantMatrix: true,
		},
		{
			name:       "explicit zero-degree matrix",
			payload:    `{"side_data_list": [{"side_data_type": "Display Matrix", "rotation": 0}]}`,
			wantMatrix: true,
		},
		{
			name:    "no side data",
			payload: `{"field_order": "tb"}`,
		},
		{
			name:    "other side data types are not a matrix",
			payload: `{"side_data_list": [{"side_data_type": "H.26[45] User Data Unregistered SEI message"}]}`,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var stream Stream
			err := json.Unmarshal([]byte(tc.payload), &stream)
			if err != nil {
				t.Fatalf("Unmarshal failed: %v", err)
			}
			deg, hasMatrix := stream.Rotation()
			if deg != tc.wantDeg || hasMatrix != tc.wantMatrix {
				t.Errorf("Rotation() = (%d, %v), want (%d, %v)", deg, hasMatrix, tc.wantDeg, tc.wantMatrix)
			}
		})
	}
}
