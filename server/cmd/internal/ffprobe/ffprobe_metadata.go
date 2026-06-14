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
	Width          int    `json:"width"`
	Height         int    `json:"height"`
	CodedWidth     int    `json:"coded_width"`
	CodedHeight    int    `json:"coded_height"`
	AspectRatio    string `json:"display_aspect_ratio"`
	Level          int    `json:"level"`
	AvgFrameRate   string `json:"avg_frame_rate"`
	FrameRate      string `json:"r_frame_rate"`
	BitDepth       string `json:"bits_per_raw_sample"`
	PixelFormat    string `json:"pix_fmt"`
	ColorRange     string `json:"color_range"`
	ColorTransfer  string `json:"color_transfer"`
	ColorPrimaries string `json:"color_primaries"`
	ColorSpace     string `json:"color_space"`

	Tags        StreamTags        `json:"tags"`
	Disposition StreamDisposition `json:"disposition"`
}

type StreamDisposition struct {
	AttachedPic int `json:"attached_pic"`
}

type StreamTags struct {
	Title    string `json:"title"`
	Language string `json:"language"`
}

type Format struct {
	Filename   string     `json:"filename"`
	Duration   string     `json:"duration"`
	Size       string     `json:"size"`
	BitRate    string     `json:"bit_rate"`
	FormatName string     `json:"format_name"`
	Tags       FormatTags `json:"tags"`
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
	var raw map[string]interface{}
	err := json.Unmarshal(data, &raw)
	if err != nil {
		return err
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

func (f *ffprobe) GetMetadata(filePath string) (*FfprobeResult, error) {
	return f.runMetadata(filePath,
		"-v", "quiet",
		"-print_format", "json",
		"-show_streams",
		"-show_format",
		"-show_chapters",
		filePath,
	)
}

func (f *ffprobe) GetAudioMetadata(filePath string) (*FfprobeResult, error) {
	return f.runMetadata(filePath,
		"-v", "quiet",
		"-print_format", "json",
		"-show_format",
		"-show_streams",
		"-show_entries", "format=filename,duration,size,bit_rate,format_name:format_tags:stream=index,codec_name,codec_type,profile,bit_rate,sample_rate,channels,channel_layout:stream_tags=title,language",
		filePath,
	)
}

func (f *ffprobe) runMetadata(filePath string, args ...string) (*FfprobeResult, error) {
	if strings.TrimSpace(filePath) == "" {
		return nil, fmt.Errorf("file path is required")
	}

	ctx, cancel := context.WithTimeout(context.Background(), metadataTimeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, f.bin, args...)

	output, err := cmd.Output()
	if err != nil {
		if ctx.Err() != nil {
			return nil, fmt.Errorf("ffprobe timed out for %s after %s: %w", filePath, metadataTimeout, ctx.Err())
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
