package ffprobe

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

const metadataTimeout = 60 * time.Second

type FfprobeResult struct {
	Streams  []Stream  `json:"streams"`
	Format   Format    `json:"format"`
	Chapters []Chapter `json:"chapters"`
}

type Stream struct {
	Index         int    `json:"index"`
	CodecName     string `json:"codec_name"`
	CodecType     string `json:"codec_type"`
	Profile       string `json:"profile"`
	BitRate       string `json:"bit_rate"`
	SampleRate    string `json:"sample_rate"`
	Channels      int    `json:"channels"`
	ChannelLayout string `json:"channel_layout"`

	// Video-specific fields
	Width          int              `json:"width"`
	Height         int              `json:"height"`
	CodedWidth     int              `json:"coded_width"`
	CodedHeight    int              `json:"coded_height"`
	AspectRatio    string           `json:"display_aspect_ratio"`
	Level          int              `json:"level"`
	AvgFrameRate   string           `json:"avg_frame_rate"`
	FrameRate      string           `json:"r_frame_rate"`
	BitDepth       string           `json:"bits_per_raw_sample"`
	PixelFormat    string           `json:"pix_fmt"`
	ColorRange     string           `json:"color_range"`
	ColorTransfer  string           `json:"color_transfer"`
	ColorPrimaries string           `json:"color_primaries"`
	ColorSpace     string           `json:"color_space"`
	FieldOrder     string           `json:"field_order"`
	SideDataList   []StreamSideData `json:"side_data_list"`

	Tags        StreamTags        `json:"tags"`
	Disposition StreamDisposition `json:"disposition"`
}

type StreamSideData struct {
	SideDataType string `json:"side_data_type"`
	Rotation     int64  `json:"rotation"`
}

// Rotation returns the display-matrix rotation in degrees and whether a
// display matrix was present at all, so callers can distinguish an explicit
// 0-degree matrix from no rotation side data.
func (s Stream) Rotation() (int64, bool) {
	for _, sideData := range s.SideDataList {
		if sideData.SideDataType == "Display Matrix" {
			return sideData.Rotation, true
		}
	}
	return 0, false
}

type StreamDisposition struct {
	AttachedPic     int `json:"attached_pic"`
	Default         int `json:"default"`
	Forced          int `json:"forced"`
	Comment         int `json:"comment"`
	Dub             int `json:"dub"`
	Original        int `json:"original"`
	HearingImpaired int `json:"hearing_impaired"`
	VisualImpaired  int `json:"visual_impaired"`
}

type StreamTags struct {
	Title    string `json:"title"`
	Language string `json:"language"`
}

// UnmarshalJSON normalizes stream tag keys the same way FormatTags does, so
// Matroska muxers that write TITLE/LANGUAGE (or the "lang" alias) still
// produce labelled, preference-matchable streams.
func (t *StreamTags) UnmarshalJSON(data []byte) error {
	values, err := normalizedTagValues(data)
	if err != nil {
		return err
	}

	t.Title = firstTagValue(values, "title")
	t.Language = firstTagValue(values, "language", "lang")

	return nil
}

type Format struct {
	Duration string     `json:"duration"`
	BitRate  string     `json:"bit_rate"`
	Tags     FormatTags `json:"tags"`
}

type FormatTags struct {
	Title       string `json:"title"`
	Artist      string `json:"artist"`
	AlbumArtist string `json:"album_artist"`
	Composer    string `json:"composer"`
	Album       string `json:"album"`
	Genre       string `json:"genre"`
	Track       string `json:"track"`
	Disc        string `json:"disc"`
	Date        string `json:"date"`
	Copyright   string `json:"copyright"`
	SortName    string `json:"sort_name"`
	SortAlbum   string `json:"sort_album"`
	SortArtist  string `json:"sort_artist"`
}

func (t *FormatTags) UnmarshalJSON(data []byte) error {
	values, err := normalizedTagValues(data)
	if err != nil {
		return err
	}

	t.Title = firstTagValue(values, "title")
	t.Artist = firstTagValue(values, "artist")
	t.AlbumArtist = firstTagValue(values, "albumartist")
	t.Composer = firstTagValue(values, "composer")
	t.Album = firstTagValue(values, "album")
	t.Genre = firstTagValue(values, "genre")
	t.Track = firstTagValue(values, "track", "tracknumber")
	t.Disc = firstTagValue(values, "disc", "discnumber")
	t.Date = firstTagValue(values, "date", "year")
	t.Copyright = firstTagValue(values, "copyright")
	t.SortName = firstTagValue(values, "sortname", "titlesort")
	t.SortAlbum = firstTagValue(values, "sortalbum", "albumsort")
	t.SortArtist = firstTagValue(values, "sortartist", "artistsort")

	return nil
}

func normalizedTagValues(data []byte) (map[string]string, error) {
	var raw map[string]interface{}
	err := json.Unmarshal(data, &raw)
	if err != nil {
		return nil, err
	}

	values := make(map[string]string, len(raw))
	for key, value := range raw {
		text, ok := value.(string)
		if !ok {
			continue
		}

		text = strings.TrimSpace(text)
		if text == "" {
			continue
		}

		normalizedKey := normalizeTagKey(key)
		if _, exists := values[normalizedKey]; exists {
			continue
		}

		values[normalizedKey] = text
	}

	return values, nil
}

func normalizeTagKey(key string) string {
	key = strings.ToLower(strings.TrimSpace(key))
	key = strings.ReplaceAll(key, "_", "")
	key = strings.ReplaceAll(key, "-", "")
	key = strings.ReplaceAll(key, " ", "")
	return key
}

func firstTagValue(values map[string]string, keys ...string) string {
	for _, key := range keys {
		value := values[key]
		if value != "" {
			return value
		}
	}

	return ""
}

type ChapterTags struct {
	Title string `json:"title"`
}

type Chapter struct {
	StartTime string      `json:"start_time"`
	Start     int         `json:"start"`
	Tags      ChapterTags `json:"tags"`
}

func (f *ffprobe) GetMetadata(ctx context.Context, filePath string) (*FfprobeResult, error) {
	return f.runMetadata(ctx, filePath,
		"-v", "quiet",
		"-print_format", "json",
		"-show_streams",
		"-show_format",
		"-show_chapters",
		filePath,
	)
}

func (f *ffprobe) GetAudioMetadata(ctx context.Context, filePath string) (*FfprobeResult, error) {
	return f.runMetadata(ctx, filePath,
		"-v", "quiet",
		"-print_format", "json",
		"-show_format",
		"-show_streams",
		"-show_entries", "format=duration,bit_rate:format_tags:stream=codec_name,codec_type,profile,channels,channel_layout:stream_tags=language",
		filePath,
	)
}

// runMetadata caps every probe at metadataTimeout while still honoring the
// caller's context, so a canceled library scan kills its in-flight ffprobe
// instead of leaving shutdown to wait out the timeout.
func (f *ffprobe) runMetadata(ctx context.Context, filePath string, args ...string) (*FfprobeResult, error) {
	if strings.TrimSpace(filePath) == "" {
		return nil, fmt.Errorf("file path is required")
	}

	probeCtx, cancel := context.WithTimeout(ctx, metadataTimeout)
	defer cancel()

	cmd := exec.CommandContext(probeCtx, f.bin, args...)

	output, err := cmd.Output()
	if err != nil {
		// A caller that went away and a probe that outlasted its own deadline are
		// different failures: only the latter says the file is slow to read.
		if ctx.Err() != nil {
			return nil, fmt.Errorf("ffprobe canceled for %s: %w", filePath, ctx.Err())
		}
		if probeCtx.Err() != nil {
			return nil, fmt.Errorf("ffprobe timed out for %s after %s: %w", filePath, metadataTimeout, probeCtx.Err())
		}
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) && len(exitErr.Stderr) > 0 {
			stderr := strings.TrimSpace(string(exitErr.Stderr))
			if stderr != "" {
				return nil, fmt.Errorf("ffprobe failed for %s: %w: %s", filePath, err, stderr)
			}
		}
		return nil, fmt.Errorf("ffprobe failed for %s: %w", filePath, err)
	}

	var result FfprobeResult

	err = json.Unmarshal(output, &result)
	if err != nil {
		return nil, fmt.Errorf("failed to parse ffprobe output for %s: %w", filePath, err)
	}

	if len(result.Streams) == 0 {
		return nil, fmt.Errorf("no streams found in %s", filePath)
	}

	return &result, nil
}
