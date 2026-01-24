package ffprobe

import (
	"encoding/json"
	"fmt"
	"os/exec"
)

// FfprobeResult is the unified result from ffprobe for both audio and video files.
// Fields not applicable to the media type will be empty/zero values.
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
	ColorRange     string `json:"color_range"`
	ColorTransfer  string `json:"color_transfer"`
	ColorPrimaries string `json:"color_primaries"`
	ColorSpace     string `json:"color_space"`

	Tags StreamTags `json:"tags"`
}

type StreamTags struct {
	Title    string `json:"title"`
	Language string `json:"language"`
}

type Format struct {
	Filename       string     `json:"filename"`
	Duration       string     `json:"duration"`
	Size           string     `json:"size"`
	BitRate        string     `json:"bit_rate"`
	FormatName     string     `json:"format_name"`
	FormatLongName string     `json:"format_long_name"`
	Tags           FormatTags `json:"tags"`
}

type FormatTags struct {
	Title        string `json:"title"`
	Artist       string `json:"artist"`
	AlbumArtist  string `json:"album_artist"`
	Composer     string `json:"composer"`
	Album        string `json:"album"`
	Genre        string `json:"genre"`
	Track        string `json:"track"`
	Disc         string `json:"disc"`
	Date         string `json:"date"`
	Copyright    string `json:"copyright"`
	PurchaseDate string `json:"purchase_date"`
	SortName     string `json:"sort_name"`
	SortAlbum    string `json:"sort_album"`
	SortArtist   string `json:"sort_artist"`
}

type Chapter struct {
	StartTime string `json:"start_time"`
	Start     int    `json:"start"`
	End       int    `json:"end"`
	EndTime   string `json:"end_time"`

	Tags struct {
		Title string `json:"title"`
	} `json:"tags"`
}

func (f *ffprobe) GetMetadata(filePath string) (*FfprobeResult, error) {
	cmd := exec.Command(f.bin,
		"-v", "quiet",
		"-print_format", "json",
		"-show_streams",
		"-show_format",
		"-show_chapters",
		filePath)

	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("ffprobe failed for %s: %w", filePath, err)
	}

	var result FfprobeResult

	err = json.Unmarshal(output, &result)
	if err != nil {
		return nil, fmt.Errorf("failed to parse ffprobe output: %w", err)
	}

	if len(result.Streams) == 0 {
		return nil, fmt.Errorf("no streams found in %s", filePath)
	}

	return &result, nil
}
