package ffprobe

import (
	"errors"
	"fmt"
	"igloo/cmd/internal/database"
	"os/exec"
	"path/filepath"
)

type Ffprobe interface {
	GetMovieMetadata(filePath *string) (*movieMetadataResult, error)
}

type tags struct {
	Title    string `json:"title,omitempty"`
	Language string `json:"language,omitempty"`
}

type mediaStream struct {
	Index          int    `json:"index"`
	CodecName      string `json:"codec_name,omitempty"`
	CodecLongName  string `json:"codec_long_name,omitempty"`
	CodecType      string `json:"codec_type,omitempty"`
	Profile        string `json:"profile,omitempty"`
	Height         uint   `json:"height,omitempty"`
	Width          uint   `json:"width,omitempty"`
	CodedHeight    uint   `json:"coded_height,omitempty"`
	CodedWidth     uint   `json:"coded_width,omitempty"`
	AspectRatio    string `json:"display_aspect_ratio,omitempty"`
	Level          int    `json:"level,omitempty"`
	AvgFrameRate   string `json:"avg_frame_rate,omitempty"`
	FrameRate      string `json:"r_frame_rate,omitempty"`
	BitDepth       string `json:"bits_per_raw_sample,omitempty"`
	BitRate        string `json:"bit_rate,omitempty"`
	ColorRange     string `json:"color_range,omitempty"`
	ColorTransfer  string `json:"color_transfer,omitempty"`
	ColorPrimaries string `json:"color_primaries,omitempty"`
	ColorSpace     string `json:"color_space,omitempty"`
	Channels       int    `json:"channels,omitempty"`
	ChannelLayout  string `json:"channel_layout,omitempty"`
	Tags           tags   `json:"tags,omitempty"`
}

type format struct {
	Filename       string `json:"filename"`
	BitRate        string `json:"bit_rate,omitempty"`
	Size           string `json:"size,omitempty"`
	Duration       string `json:"duration,omitempty"`
	FormatName     string `json:"format_name,omitempty"`
	FormatLongName string `json:"format_long_name,omitempty"`
}

type tmdbChapter struct {
	Start     int    `json:"start"`
	StartTime string `json:"start_time"`
	End       int    `json:"end"`
	EndTime   string `json:"end_time"`
	Title     string `json:"title"`
}

type result struct {
	Streams  []mediaStream `json:"streams"`
	Chapters []tmdbChapter `json:"chapters"`
	Format   format        `json:"format"`
}

type movieMetadataResult struct {
	VideoList    []database.CreateVideoStreamParams
	AudioList    []database.CreateAudioStreamParams
	SubtitleList []database.CreateSubtitleParams
	ChapterList  []database.CreateChapterParams
}

type ffprobe struct {
	bin string
}

func New(ffprobePath string) (Ffprobe, error) {
	if ffprobePath == "" {
		return nil, errors.New("ffprobe path is required")
	}

	f := ffprobe{
		bin: ffprobePath,
	}

	if f.bin == "ffprobe" {
		path, err := exec.LookPath(f.bin)
		if err != nil {
			return nil, fmt.Errorf("ffprobe not found in PATH: %w", err)
		}

		f.bin = path
	} else {
		absPath, err := filepath.Abs(f.bin)
		if err != nil {
			return nil, fmt.Errorf("invalid ffprobe path: %w", err)
		}

		f.bin = absPath

		_, err = exec.LookPath(f.bin)
		if err != nil {
			return nil, fmt.Errorf("ffprobe not found or not executable at %s: %w", f.bin, err)
		}
	}

	return &f, nil
}
